@echo off
setlocal
title ContainerWS — migrate workspace to WSL (fix file watching)
cd /d "%~dp0"

echo.
echo Run this from Windows (PowerShell/CMD), not inside the Linux container.
echo.

powershell.exe -NoProfile -ExecutionPolicy Bypass -File "%~dp0migrate-to-wsl-fs.ps1" %*
set ERR=%ERRORLEVEL%

echo.
if %ERR% neq 0 (
  echo Migration failed with exit code %ERR%.
) else (
  echo Migration script finished.
)
pause
exit /b %ERR%
