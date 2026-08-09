import { useState } from "react"
import { Link, useParams, useSearchParams } from "react-router"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import {
  ArrowRightLeft,
  Box,
  Download,
  ExternalLink,
  Package,
  RefreshCw,
  Terminal,
  Trash2,
} from "lucide-react"
import { toast } from "sonner"

import ContentLoader from "@/components/content-loader"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { getRequestErrorMessage, withError } from "@/lib/network"
import { cn } from "@/lib/utils"
import { SOFTWARES_FETCH_KEY } from "@/modules/softwares/pages/list/api"

import {
  BREW_FORMULA_KEY,
  BREW_FORMULAE_KEY,
  BREW_INSTALLED_KEY,
  getBrewFormula,
  getBrewJob,
  runBrewAction,
  switchPackageManager,
  type BrewFormulaVersion,
} from "./api"
import { FormulaGlyph, HomebrewMark } from "./formula-glyph"

async function waitBrewJob(id: string) {
  for (let i = 0; i < 600; i++) {
    await new Promise((r) => setTimeout(r, 1000))
    const snap = await getBrewJob(id)
    const st = snap.data?.status
    if (st === "success" || st === "error") {
      if (st === "error") {
        throw new Error(snap.data?.error || "Brew action failed")
      }
      return snap
    }
  }
  throw new Error("Timed out waiting for brew job")
}

function formatCount(n?: number) {
  if (n == null || n <= 0) return "—"
  return n.toLocaleString()
}

function versionKindLabel(kind: string) {
  switch (kind) {
    case "stable":
      return "Stable"
    case "head":
      return "HEAD"
    case "versioned":
      return "Versioned"
    case "installed":
      return "Installed keg"
    default:
      return kind
  }
}

function VersionsPanel({
  versions,
  currentName,
}: {
  versions: BrewFormulaVersion[]
  currentName: string
}) {
  if (!versions.length) {
    return (
      <p className="text-sm text-muted-foreground">No version history listed.</p>
    )
  }

  return (
    <div className="overflow-hidden rounded-xl border">
      <table className="w-full text-sm">
        <thead className="bg-muted/50 text-left text-xs uppercase tracking-wide text-muted-foreground">
          <tr>
            <th className="px-4 py-2.5 font-medium">Formula</th>
            <th className="px-4 py-2.5 font-medium">Version</th>
            <th className="px-4 py-2.5 font-medium">Type</th>
            <th className="px-4 py-2.5 font-medium">Status</th>
          </tr>
        </thead>
        <tbody>
          {versions.map((row, i) => {
            const isSelf =
              row.formula === currentName &&
              (row.kind === "stable" || row.kind === "installed" || row.kind === "head")
            const href =
              row.href ||
              (row.formula ? `/brew/${encodeURIComponent(row.formula)}` : undefined)
            return (
              <tr
                key={`${row.formula}-${row.version}-${row.kind}-${i}`}
                className={cn(
                  "border-t",
                  row.current ? "bg-primary/5" : "bg-background"
                )}
              >
                <td className="px-4 py-2.5 font-medium">
                  {href && !isSelf ? (
                    <Link
                      to={href}
                      className="text-foreground underline-offset-2 hover:underline"
                    >
                      {row.formula}
                    </Link>
                  ) : (
                    row.formula
                  )}
                </td>
                <td className="px-4 py-2.5 font-mono text-xs">
                  {row.version ? `v${row.version}` : "—"}
                </td>
                <td className="px-4 py-2.5">
                  <Badge variant="outline" className="font-normal">
                    {versionKindLabel(row.kind)}
                  </Badge>
                </td>
                <td className="px-4 py-2.5">
                  <div className="flex flex-wrap gap-1.5">
                    {row.current ? (
                      <Badge variant="secondary">Current</Badge>
                    ) : null}
                    {row.installed ? (
                      <Badge variant="default">Installed</Badge>
                    ) : (
                      <span className="text-xs text-muted-foreground">
                        Not installed
                      </span>
                    )}
                  </div>
                </td>
              </tr>
            )
          })}
        </tbody>
      </table>
    </div>
  )
}

function MetaChip({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-lg border bg-card px-3 py-2">
      <div className="text-[10px] font-medium uppercase tracking-wide text-muted-foreground">
        {label}
      </div>
      <div className="mt-0.5 truncate text-sm font-medium">{value}</div>
    </div>
  )
}

export default function BrewFormulaDetailPage() {
  const { name = "" } = useParams()
  const [searchParams] = useSearchParams()
  const kindParam = searchParams.get("kind") || undefined
  const queryClient = useQueryClient()
  const [busy, setBusy] = useState(false)

  const query = useQuery({
    queryKey: [BREW_FORMULA_KEY, name, kindParam],
    queryFn: () => getBrewFormula(name, kindParam),
    enabled: Boolean(name),
  })

  const formula = query.data?.data
  const isCask = formula?.kind === "cask"
  const displayName = formula?.display_name || formula?.name || name
  const ownedLocal =
    formula?.ownership === "local" || formula?.package_manager === "local"

  const actionMutation = useMutation({
    mutationFn: async (action: "install" | "upgrade" | "uninstall") => {
      setBusy(true)
      const job = await runBrewAction(action, [name], formula?.kind || kindParam)
      const id = job.data?.id
      if (!id) return job
      return waitBrewJob(id)
    },
    onSuccess: (_res, action) => {
      toast.success(`${action} ${name} completed`)
      void queryClient.invalidateQueries({ queryKey: [BREW_FORMULA_KEY, name] })
      void queryClient.invalidateQueries({ queryKey: [BREW_FORMULAE_KEY] })
      void queryClient.invalidateQueries({ queryKey: [BREW_INSTALLED_KEY] })
    },
    onError: (err) => {
      toast.error(getRequestErrorMessage(err, "Brew action failed"))
    },
    onSettled: () => setBusy(false),
  })

  const switchMutation = useMutation({
    mutationFn: (target: "local" | "brew") => {
      if (!formula?.software_id) {
        return Promise.reject(new Error("No matching Softwares entry"))
      }
      return switchPackageManager(formula.software_id, target)
    },
    onSuccess: (res) => {
      toast.success(res.message || "Switched package manager")
      void queryClient.invalidateQueries({ queryKey: [BREW_FORMULA_KEY, name] })
      void queryClient.invalidateQueries({ queryKey: [SOFTWARES_FETCH_KEY] })
    },
    onError: (err) => {
      toast.error(getRequestErrorMessage(err, "Switch failed"))
    },
  })

  const shortDesc =
    formula?.desc?.trim() ||
    "No short description is available for this package."

  return (
    <ContentLoader
      title={displayName}
      description={shortDesc}
      breadcrumb={[
        { label: "Brew Manager", to: "/brew" },
        { label: displayName },
      ]}
      isLoading={query.isLoading}
      error={withError(query.error, query.data)}
      showHeaderSeparator
    >
      {formula ? (
        <div className="mx-auto max-w-4xl space-y-8">
          <section className="overflow-hidden rounded-2xl border bg-card shadow-xs">
            <div className="flex flex-wrap items-center justify-between gap-3 border-b bg-muted/30 px-5 py-3">
              <div className="flex min-w-0 items-center gap-2 text-sm text-muted-foreground">
                <HomebrewMark className="size-5 shrink-0" />
                <span className="font-medium text-foreground">
                  {isCask ? "Homebrew cask" : "Homebrew formula"}
                </span>
                {formula.tap ? (
                  <span className="truncate font-mono text-xs">{formula.tap}</span>
                ) : null}
                {formula.deprecated ? (
                  <Badge variant="destructive">Deprecated</Badge>
                ) : null}
                {formula.disabled ? (
                  <Badge variant="destructive">Disabled</Badge>
                ) : null}
              </div>
              <div className="flex flex-wrap items-center gap-2">
                {!formula.installed ? (
                  <Button
                    size="sm"
                    disabled={busy || ownedLocal}
                    onClick={() => actionMutation.mutate("install")}
                  >
                    <Download className="size-4" />
                    Install
                    {formula.version ? ` v${formula.version}` : ""}
                  </Button>
                ) : formula.outdated ? (
                  <Button
                    size="sm"
                    variant="secondary"
                    disabled={busy || ownedLocal}
                    onClick={() => actionMutation.mutate("upgrade")}
                  >
                    <RefreshCw className="size-4" />
                    Update
                    {formula.version ? ` to v${formula.version}` : ""}
                  </Button>
                ) : (
                  <Badge variant="default" className="h-8 px-3 text-xs">
                    Installed
                    {formula.installed_version
                      ? ` · v${formula.installed_version}`
                      : ""}
                  </Badge>
                )}
              </div>
            </div>

            <div className="flex flex-col gap-6 p-5 sm:flex-row sm:items-start">
              <FormulaGlyph
                name={displayName}
                icon={formula.icon}
                logo={formula.logo}
                category={formula.category}
                size="xl"
                className="ring-1 ring-border/60"
              />
              <div className="min-w-0 flex-1 space-y-3">
                <div className="space-y-1.5">
                  <h2 className="text-2xl font-semibold tracking-tight">
                    {displayName}
                  </h2>
                  {formula.display_name &&
                  formula.display_name !== formula.name ? (
                    <p className="font-mono text-xs text-muted-foreground">
                      {formula.name}
                    </p>
                  ) : null}
                  <p className="max-w-2xl text-sm leading-relaxed text-muted-foreground">
                    {shortDesc}
                  </p>
                </div>
                <div className="flex flex-wrap gap-2">
                  <Badge variant="secondary">
                    {isCask ? "Desktop App" : "Formula"}
                  </Badge>
                  {formula.category ? (
                    <Badge variant="outline">{formula.category}</Badge>
                  ) : null}
                  {ownedLocal ? <Badge variant="outline">Softwares</Badge> : null}
                  {formula.package_manager === "brew" ? (
                    <Badge variant="secondary">Brew-managed</Badge>
                  ) : null}
                  {formula.aliases?.slice(0, 4).map((alias) => (
                    <Badge key={alias} variant="outline" className="font-mono">
                      {alias}
                    </Badge>
                  ))}
                </div>
                <div className="grid gap-2 sm:grid-cols-2 lg:grid-cols-4">
                  <MetaChip
                    label="Stable"
                    value={formula.version ? `v${formula.version}` : "—"}
                  />
                  <MetaChip
                    label="Installed"
                    value={
                      formula.installed_version
                        ? `v${formula.installed_version}`
                        : "—"
                    }
                  />
                  <MetaChip label="License" value={formula.license || "—"} />
                  <MetaChip
                    label="Installs (30d)"
                    value={formatCount(formula.analytics?.install_30d)}
                  />
                </div>
                {formula.homepage ? (
                  <a
                    href={formula.homepage}
                    target="_blank"
                    rel="noreferrer"
                    className="inline-flex items-center gap-1.5 text-sm text-muted-foreground underline-offset-2 hover:text-foreground hover:underline"
                  >
                    {formula.homepage.replace(/^https?:\/\//, "")}
                    <ExternalLink className="size-3.5" />
                  </a>
                ) : null}
              </div>
            </div>
          </section>

          <section className="space-y-3">
            <div className="flex items-center gap-2">
              <Package className="size-4 text-muted-foreground" />
              <h3 className="text-sm font-semibold tracking-tight">Versions</h3>
            </div>
            <p className="text-sm text-muted-foreground">
              Stable release, related versioned formulae, and any kegs already
              installed on this host — review these before installing or
              upgrading.
            </p>
            <VersionsPanel
              versions={formula.versions ?? []}
              currentName={formula.name}
            />
          </section>

          {(formula.installed ||
            formula.can_switch_to_brew ||
            formula.can_switch_to_local ||
            formula.software_id ||
            ownedLocal) ? (
            <section className="space-y-3">
              <div className="flex items-center gap-2">
                <Box className="size-4 text-muted-foreground" />
                <h3 className="text-sm font-semibold tracking-tight">
                  Manage
                </h3>
              </div>
              <div className="flex flex-wrap gap-2 rounded-xl border bg-card p-4">
                {formula.installed ? (
                  <Button
                    variant="outline"
                    disabled={busy || ownedLocal}
                    onClick={() => actionMutation.mutate("uninstall")}
                  >
                    <Trash2 className="size-4" />
                    Uninstall
                  </Button>
                ) : null}
                {formula.can_switch_to_brew && formula.software_id ? (
                  <Button
                    variant="outline"
                    disabled={switchMutation.isPending}
                    onClick={() => switchMutation.mutate("brew")}
                  >
                    <ArrowRightLeft className="size-4" />
                    Switch to Brew
                  </Button>
                ) : null}
                {formula.can_switch_to_local && formula.software_id ? (
                  <Button
                    variant="outline"
                    disabled={switchMutation.isPending}
                    onClick={() => switchMutation.mutate("local")}
                  >
                    <ArrowRightLeft className="size-4" />
                    Switch to Local
                  </Button>
                ) : null}
                {formula.software_id ? (
                  <Button variant="ghost" asChild>
                    <Link to={`/softwares/${formula.software_id}`}>
                      Open in Softwares
                    </Link>
                  </Button>
                ) : null}
              </div>
              {ownedLocal ? (
                <p className="rounded-lg border border-dashed p-3 text-sm text-muted-foreground">
                  This formula matches a Softwares catalog entry managed by the
                  local package manager. Switch to Brew to manage it here, or
                  use Softwares actions instead.
                </p>
              ) : null}
            </section>
          ) : null}

          {(formula.dependencies?.length ||
            formula.build_dependencies?.length ||
            formula.executables?.length) ? (
            <section className="grid gap-4 md:grid-cols-3">
              <div className="space-y-2 rounded-xl border bg-card p-4">
                <h4 className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">
                  Dependencies
                </h4>
                <FormulaNameList names={formula.dependencies} />
              </div>
              <div className="space-y-2 rounded-xl border bg-card p-4">
                <h4 className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">
                  Build dependencies
                </h4>
                <FormulaNameList names={formula.build_dependencies} />
              </div>
              <div className="space-y-2 rounded-xl border bg-card p-4">
                <div className="flex items-center gap-1.5">
                  <Terminal className="size-3.5 text-muted-foreground" />
                  <h4 className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">
                    Executables
                  </h4>
                </div>
                {formula.executables?.length ? (
                  <ul className="space-y-1 font-mono text-xs">
                    {formula.executables.map((bin) => (
                      <li key={bin}>{bin}</li>
                    ))}
                  </ul>
                ) : (
                  <p className="text-sm text-muted-foreground">None listed</p>
                )}
              </div>
            </section>
          ) : null}

          {(formula.analytics?.install_90d ||
            formula.analytics?.install_365d) ? (
            <section className="grid gap-2 sm:grid-cols-3">
              <MetaChip
                label="Installs · 30 days"
                value={formatCount(formula.analytics?.install_30d)}
              />
              <MetaChip
                label="Installs · 90 days"
                value={formatCount(formula.analytics?.install_90d)}
              />
              <MetaChip
                label="Installs · 365 days"
                value={formatCount(formula.analytics?.install_365d)}
              />
            </section>
          ) : null}
        </div>
      ) : null}
    </ContentLoader>
  )
}

function FormulaNameList({ names }: { names?: string[] }) {
  if (!names?.length) {
    return <p className="text-sm text-muted-foreground">None</p>
  }
  return (
    <ul className="flex flex-wrap gap-1.5">
      {names.map((n) => (
        <li key={n}>
          <Link
            to={`/brew/${encodeURIComponent(n)}`}
            className="inline-flex rounded-md border bg-background px-2 py-0.5 font-mono text-xs hover:border-foreground/30"
          >
            {n}
          </Link>
        </li>
      ))}
    </ul>
  )
}
