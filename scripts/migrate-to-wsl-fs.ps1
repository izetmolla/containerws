#Requires -Version 5.1
<#
.SYNOPSIS
  Move ContainerWS workspace off Windows NTFS (C:) onto the WSL Linux filesystem
  so Docker bind-mounts get real inotify — fixing air / Vite HMR / git refresh.

.DESCRIPTION
  Your container currently mounts:
    C:\Users\...\WORKSPACE\containerws\content  ->  /workspace
  via 9p/drvfs. That mount does NOT deliver file-change events into Linux.

  This script:
    1. Picks a WSL distro (not docker-desktop*)
    2. Copies content/, config/, keys.ssh/ into ~/WORKSPACE/containerws on Linux ext4
    3. Writes CONTAINERWS_HOST_ROOT for docker compose
    4. Optionally recreates the containerws container with the new mounts

  Run from Windows PowerShell (Admin not required):
    powershell -ExecutionPolicy Bypass -File .\scripts\migrate-to-wsl-fs.ps1

.PARAMETER WindowsRoot
  Host folder that contains content\, config\, keys.ssh\

.PARAMETER WslDistro
  WSL distro name. Empty = auto-pick (Ubuntu preferred).

.PARAMETER WslDest
  Destination inside WSL (tilde allowed), e.g. ~/WORKSPACE/containerws

.PARAMETER SkipCopy
  Only rewrite env + recreate container (files already on WSL).

.PARAMETER NoRecreate
  Copy + write env, but do not docker compose up.

.PARAMETER IncludeNodeModules
  Also copy node_modules (much slower). Default skips them — run pnpm install after.
#>

[CmdletBinding()]
param(
  [string]$WindowsRoot = "C:\Users\imolla\WORKSPACE\containerws",
  [string]$WslDistro = "",
  [string]$WslDest = "~/WORKSPACE/containerws",
  [switch]$SkipCopy,
  [switch]$NoRecreate,
  [switch]$IncludeNodeModules,
  [switch]$Force
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

function Write-Step([string]$Message) {
  Write-Host ""
  Write-Host "==> $Message" -ForegroundColor Cyan
}

function Write-Ok([string]$Message) {
  Write-Host "    OK: $Message" -ForegroundColor Green
}

function Write-Warn([string]$Message) {
  Write-Host "    WARN: $Message" -ForegroundColor Yellow
}

function Assert-Command([string]$Name) {
  if (-not (Get-Command $Name -ErrorAction SilentlyContinue)) {
    throw "Required command not found: $Name"
  }
}

function Get-WslDistros {
  $prev = [Console]::OutputEncoding
  try {
    [Console]::OutputEncoding = [System.Text.Encoding]::Unicode
    $lines = & wsl.exe -l -q 2>$null
  } finally {
    [Console]::OutputEncoding = $prev
  }
  $names = @()
  foreach ($line in @($lines)) {
    $n = ("$line" -replace "\(Default\)", "").Trim()
    $n = ($n -replace "\u0000", "").Trim()
    if ($n) { $names += $n }
  }
  return $names
}

function Convert-ToWslPath([string]$WinPath) {
  $full = [System.IO.Path]::GetFullPath($WinPath)
  if ($full -notmatch '^[A-Za-z]:\\') {
    throw "Not a Windows drive path: $WinPath"
  }
  $drive = $full.Substring(0, 1).ToLowerInvariant()
  $rest = $full.Substring(2) -replace '\\', '/'
  return "/mnt/$drive$rest"
}

function Invoke-Wsl {
  param(
    [Parameter(Mandatory = $true)][string]$Distro,
    [Parameter(Mandatory = $true)][string]$BashCommand
  )
  & wsl.exe -d $Distro -- bash -lc $BashCommand
  if ($LASTEXITCODE -ne 0) {
    throw "WSL command failed (exit $LASTEXITCODE): $BashCommand"
  }
}

Write-Host @"

ContainerWS — migrate workspace to WSL Linux filesystem
=======================================================
Fixes missing hot-reload / git refresh caused by C: 9p bind-mounts.

Windows root : $WindowsRoot
WSL dest     : $WslDest

"@

Assert-Command "wsl.exe"

# --- Validate Windows layout ---
Write-Step "Checking Windows workspace layout"
$required = @("content")
$optional = @("config", "keys.ssh")
foreach ($name in $required) {
  $p = Join-Path $WindowsRoot $name
  if (-not (Test-Path -LiteralPath $p)) {
    throw "Missing required folder: $p"
  }
  Write-Ok $p
}
foreach ($name in $optional) {
  $p = Join-Path $WindowsRoot $name
  if (Test-Path -LiteralPath $p) { Write-Ok $p }
  else { Write-Warn "Optional folder missing (skipped): $p" }
}

# --- Pick WSL distro ---
Write-Step "Selecting WSL distro"
$distros = @(Get-WslDistros | Where-Object {
  $_ -and
  $_ -notmatch '^docker-desktop' -and
  $_ -notmatch '^docker-desktop-data'
})
if (-not $distros -or $distros.Count -eq 0) {
  throw "No usable WSL distro found. Install Ubuntu from Microsoft Store, then re-run."
}

if ($WslDistro) {
  if ($distros -notcontains $WslDistro) {
    throw "Distro '$WslDistro' not found. Available: $($distros -join ', ')"
  }
  $distro = $WslDistro
} else {
  $ubuntu = $distros | Where-Object { $_ -match 'Ubuntu' } | Select-Object -First 1
  $distro = if ($ubuntu) { $ubuntu } else { $distros[0] }
}
Write-Ok "Using distro: $distro (candidates: $($distros -join ', '))"

# Resolve ~ and abs path inside WSL
Write-Step "Resolving Linux destination path"
$resolveCmd = @"
set -e
DEST='$WslDest'
DEST=`${DEST/#\~/`$HOME}
mkdir -p "`$DEST"
realpath "`$DEST"
"@
$wslAbsDest = (& wsl.exe -d $distro -- bash -lc $resolveCmd).Trim()
if (-not $wslAbsDest) { throw "Could not resolve WSL destination path." }
Write-Ok "Linux path: $wslAbsDest"

$fstype = (& wsl.exe -d $distro -- bash -lc "df -T '$wslAbsDest' | awk 'NR==2{print `$2}'").Trim()
if ($fstype -match '9p|drvfs|cifs|fuse') {
  throw "Destination is still on a remote/Windows-style FS ($fstype). Pick a path under `$HOME on the Linux disk."
}
Write-Ok "Filesystem type: $fstype (good — not 9p)"

if (-not $Force) {
  Write-Host ""
  Write-Host "This will copy workspace data into WSL and (unless -NoRecreate) recreate container 'containerws'." -ForegroundColor Yellow
  Write-Host "Your current Cursor/SSH session into the container will disconnect during recreate." -ForegroundColor Yellow
  $answer = Read-Host "Continue? [y/N]"
  if ($answer -notmatch '^(y|yes)$') {
    Write-Host "Aborted."
    exit 1
  }
}

# --- Copy ---
if (-not $SkipCopy) {
  Write-Step "Copying folders into WSL (this can take a while)"
  $winContent = Convert-ToWslPath (Join-Path $WindowsRoot "content")
  $exclude = @()
  if (-not $IncludeNodeModules) {
    $exclude += "--exclude=node_modules"
    $exclude += "--exclude=.pnpm-store"
  }
  $exclude += "--exclude=tmp"
  $exclude += "--exclude=.git/objects/tmp_*"
  $excludeArgs = ($exclude -join " ")

  $hasRsync = (& wsl.exe -d $distro -- bash -lc "command -v rsync >/dev/null 2>&1 && echo yes || echo no").Trim()

  $folders = @("content")
  foreach ($name in $optional) {
    if (Test-Path -LiteralPath (Join-Path $WindowsRoot $name)) { $folders += $name }
  }

  foreach ($name in $folders) {
    $srcWin = Join-Path $WindowsRoot $name
    $srcWsl = Convert-ToWslPath $srcWin
    $dstWsl = "$wslAbsDest/$name"
    Write-Host "    Copying $name ..."
    if ($hasRsync -eq "yes") {
      $copyCmd = "mkdir -p '$dstWsl' && rsync -a --info=stats2 $excludeArgs '$srcWsl/' '$dstWsl/'"
    } else {
      Write-Warn "rsync not found — installing rsync in WSL"
      Invoke-Wsl -Distro $distro -BashCommand "sudo apt-get update -qq && sudo DEBIAN_FRONTEND=noninteractive apt-get install -y -qq rsync >/dev/null"
      $copyCmd = "mkdir -p '$dstWsl' && rsync -a --info=stats2 $excludeArgs '$srcWsl/' '$dstWsl/'"
    }
    Invoke-Wsl -Distro $distro -BashCommand $copyCmd
    Write-Ok "$name -> $dstWsl"
  }

  if (-not $IncludeNodeModules) {
    Write-Warn "Skipped node_modules / .pnpm-store. Inside the container run: cd <app>/frontend && pnpm install"
  }
} else {
  Write-Step "Skipping copy (-SkipCopy)"
}

# --- Env files for compose ---
Write-Step "Writing CONTAINERWS_HOST_ROOT env files"
$envBody = @"
# Generated by scripts/migrate-to-wsl-fs.ps1
# Linux filesystem path used by docker-compose-windows.yaml bind mounts.
CONTAINERWS_HOST_ROOT=$wslAbsDest
"@

$envTargets = @(
  (Join-Path $WindowsRoot ".env"),
  (Join-Path $WindowsRoot "content\containerws\.env.containerws-host")
)

foreach ($t in $envTargets) {
  $dir = Split-Path -Parent $t
  if (Test-Path -LiteralPath $dir) {
    Set-Content -LiteralPath $t -Value $envBody -Encoding UTF8
    Write-Ok $t
  }
}

Invoke-Wsl -Distro $distro -BashCommand "cat > '$wslAbsDest/.env' <<'EOF'
$envBody
EOF
mkdir -p '$wslAbsDest/content/containerws'
cp '$wslAbsDest/.env' '$wslAbsDest/content/containerws/.env.containerws-host' 2>/dev/null || true
"
Write-Ok "$wslAbsDest/.env"

# Patch compose default if the checked-in file still has hardcoded /c/Users path —
# compose already supports ${CONTAINERWS_HOST_ROOT:-...}; ensure .env is used at up time.

# --- Recreate container ---
if ($NoRecreate) {
  Write-Step "Skipping container recreate (-NoRecreate)"
} else {
  Write-Step "Recreating Docker container with Linux bind mounts"
  Assert-Command "docker.exe"

  $composeCandidates = @(
    "$wslAbsDest/content/containerws/docker-compose-windows.yaml",
    "$wslAbsDest/docker-compose-windows.yaml"
  )
  $composeFind = ($composeCandidates | ForEach-Object { "test -f '$_' && echo '$_'" }) -join "; "
  $composePath = (& wsl.exe -d $distro -- bash -lc $composeFind).Trim().Split("`n") | Where-Object { $_ } | Select-Object -First 1
  if (-not $composePath) {
    throw "docker-compose-windows.yaml not found under $wslAbsDest"
  }
  Write-Ok "Compose file: $composePath"

  # Prefer docker inside WSL (correct mount namespace for Linux paths)
  $dockerInWsl = (& wsl.exe -d $distro -- bash -lc "command -v docker >/dev/null 2>&1 && echo yes || echo no").Trim()

  $upScript = @"
set -euo pipefail
export CONTAINERWS_HOST_ROOT='$wslAbsDest'
COMPOSE='$composePath'
cd "`$(dirname "`$COMPOSE")"
if [ -f '$wslAbsDest/.env' ]; then
  set -a
  . '$wslAbsDest/.env'
  set +a
fi
echo "CONTAINERWS_HOST_ROOT=`$CONTAINERWS_HOST_ROOT"
docker compose -f "`$COMPOSE" --env-file '$wslAbsDest/.env' down || true
docker compose -f "`$COMPOSE" --env-file '$wslAbsDest/.env' up -d
docker inspect -f '{{range .Mounts}}{{println .Source "->" .Destination}}{{end}}' containerws | head -20
"@

  if ($dockerInWsl -eq "yes") {
    Write-Ok "Using docker inside WSL distro '$distro'"
    Invoke-Wsl -Distro $distro -BashCommand $upScript
  } else {
    Write-Warn "No docker in WSL — using Windows docker.exe (ensure Docker Desktop WSL integration is enabled for '$distro')"
    $env:CONTAINERWS_HOST_ROOT = $wslAbsDest
    $winCompose = Join-Path $WindowsRoot "content\containerws\docker-compose-windows.yaml"
    if (-not (Test-Path -LiteralPath $winCompose)) {
      throw "Compose not found at $winCompose and docker is unavailable inside WSL."
    }
    & docker.exe compose -f $winCompose --env-file (Join-Path $WindowsRoot ".env") down
    & docker.exe compose -f $winCompose --env-file (Join-Path $WindowsRoot ".env") up -d
  }

  Write-Step "Verifying mount is no longer Windows 9p"
  Start-Sleep -Seconds 3
  $verify = & docker.exe exec containerws sh -lc "findmnt -no FSTYPE,SOURCE /workspace 2>/dev/null || df -T /workspace | tail -1"
  Write-Host "    /workspace => $verify"
  if ("$verify" -match '9p|drvfs|C:\\|C:/') {
    Write-Warn "Mount still looks like Windows/9p. Check Docker Desktop > Settings > Resources > WSL Integration for '$distro', then re-run with -SkipCopy."
  } else {
    Write-Ok "Workspace mount looks like a Linux path (inotify should work)"
  }
}

Write-Host @"

Done.
-----
Next steps:
  1. Reconnect Cursor / SSH to the containerws container.
  2. If frontend deps were skipped:  cd /workspace/containerws/frontend && pnpm install
  3. Restart watchers:  air   and   pnpm dev
  4. Optional: rename the old Windows folders as backup, e.g.
       ren `"$WindowsRoot\content`" content.ntfs-backup

WSL project path:  \\wsl$\$distro$($wslAbsDest -replace '/','\')
Linux path:        $wslAbsDest

"@
