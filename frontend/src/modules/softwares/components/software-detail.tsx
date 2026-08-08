import { useState } from "react"
import { Link } from "react-router"
import {
  ArrowLeft,
  ArrowUpCircle,
  CheckCircle2,
  Download,
  Loader2,
  Pencil,
  RefreshCw,
  Trash2,
} from "lucide-react"

import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { cn } from "@/lib/utils"

import {
  softwareInstallStatus,
  type ServiceStatus,
  type Software,
  type SoftwareVersion,
} from "../pages/list/api"
import { SoftwareGlyph } from "./software-glyph"
import { StatusPill } from "./status-pill"
import { SoftwareServicePanel } from "./software-service-panel"
import {
  InstallTerminal,
  type InstallTerminalStatus,
} from "../pages/single/components/install-terminal"
import type { InstallTerminalLine } from "../pages/single/api"

export function SoftwareDetail({
  software,
  versions,
  latest,
  installedVersion,
  isInstalled,
  hasUpdate,
  osMissing = false,
  uninstalled = false,
  canInstall,
  canUninstall = false,
  canControl = false,
  serviceStatus = null,
  installing,
  uninstalling = false,
  onInstall,
  onUninstall,
  onServiceStatusChange,
  terminal,
  initialTab = "overview",
}: {
  software: Software
  versions: SoftwareVersion[]
  latest: SoftwareVersion | null | undefined
  installedVersion: SoftwareVersion | null | undefined
  isInstalled: boolean
  hasUpdate: boolean
  osMissing?: boolean
  uninstalled?: boolean
  canInstall: boolean
  canUninstall?: boolean
  canControl?: boolean
  serviceStatus?: ServiceStatus | null
  installing: boolean
  uninstalling?: boolean
  onInstall: () => void
  onUninstall?: () => void
  onServiceStatusChange?: (status: ServiceStatus) => void
  terminal: {
    open: boolean
    status: InstallTerminalStatus
    lines: InstallTerminalLine[]
    jobId: string | null
    failureReason?: string | null
    cancelling: boolean
    onCancel: () => void
    onRetry: () => void
    onClear: () => void
    onClose: () => void
  }
  initialTab?: string
}) {
  const [tab, setTab] = useState(initialTab)
  const accent = software.color || "var(--primary)"
  const status = softwareInstallStatus(
    {
      is_installed: isInstalled,
      has_update: hasUpdate,
      os_missing: osMissing,
      uninstalled,
    },
    installing ? (hasUpdate ? "updating" : "installing") : null
  )

  const actionLabel = (() => {
    if (installing) return "Installing…"
    if (!latest) return "No version"
    if (uninstalled) return `Reinstall v${latest.version}`
    if (hasUpdate) return `Update to v${latest.version}`
    if (osMissing) return `Restore v${latest.version}`
    if (isInstalled) return `Reinstall v${latest.version}`
    return `Install v${latest.version}`
  })()

  return (
    <div className="">
      <Button variant="ghost" size="sm" className="mb-4 -ml-2" asChild>
        <Link to="/softwares">
          <ArrowLeft className="mr-1.5 h-4 w-4" />
          Back to catalog
        </Link>
      </Button>

      <div className="flex flex-col gap-6 lg:flex-row lg:items-start lg:justify-between">
        <div className="flex gap-4">
          {software.image?.trim() ? (
            <div className="flex h-20 w-20 shrink-0 items-center justify-center overflow-hidden rounded-xl border border-border/60 bg-background shadow-sm">
              <SoftwareGlyph
                name={software.icon}
                image={software.image}
                className="h-9 w-9"
                imgClassName="h-20 w-20 object-cover"
              />
            </div>
          ) : (
            <div
              className="flex h-20 w-20 shrink-0 items-center justify-center rounded-xl text-white shadow-sm"
              style={{ backgroundColor: accent }}
            >
              <SoftwareGlyph name={software.icon} className="h-9 w-9" />
            </div>
          )}
          <div>
            <div className="flex flex-wrap items-center gap-2">
              <h1 className="text-2xl font-semibold">{software.name}</h1>
              {software.category ? (
                <Badge variant="outline">{software.category}</Badge>
              ) : null}
              <StatusPill status={status} />
            </div>
            {software.sub_category ? (
              <p className="mt-1 text-sm text-muted-foreground">
                {software.sub_category}
              </p>
            ) : null}
            <p className="mt-3 max-w-xl text-sm">
              {software.details || "No description available."}
            </p>
          </div>
        </div>

        <div className="flex flex-col items-stretch gap-2 lg:min-w-[240px]">
          <Button size="sm" variant="outline" asChild>
            <Link to={`/softwares/${software.id}/package`}>
              <Pencil className="mr-1.5 h-3.5 w-3.5" />
              Edit package
            </Link>
          </Button>
          {(!isInstalled || uninstalled) && (
            <>
              <Button
                size="lg"
                disabled={!canInstall || installing}
                onClick={onInstall}
                style={
                  canInstall
                    ? { backgroundColor: accent, borderColor: accent }
                    : undefined
                }
              >
                {installing ? (
                  <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                ) : (
                  <Download className="mr-2 h-4 w-4" />
                )}
                {actionLabel}
              </Button>
              <button
                type="button"
                onClick={() => setTab("versions")}
                className="text-center text-xs text-muted-foreground underline-offset-4 hover:text-foreground hover:underline"
              >
                Browse versions
              </button>
            </>
          )}
          {hasUpdate && (
            <>
              <Button
                size="lg"
                className="bg-amber-500 text-black hover:bg-amber-400"
                disabled={!canInstall || installing}
                onClick={onInstall}
              >
                {installing ? (
                  <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                ) : (
                  <ArrowUpCircle className="mr-2 h-4 w-4" />
                )}
                {actionLabel}
              </Button>
            </>
          )}
          {isInstalled && !hasUpdate && (
            <Button
              variant="secondary"
              size="lg"
              disabled={!canInstall || installing || uninstalling}
              onClick={onInstall}
            >
              {installing ? (
                <Loader2 className="mr-2 h-4 w-4 animate-spin" />
              ) : (
                <RefreshCw className="mr-2 h-4 w-4" />
              )}
              {actionLabel}
            </Button>
          )}

          {isInstalled && (
            <Button
              variant="outline"
              size="lg"
              className="border-destructive/40 text-destructive hover:bg-destructive/10 hover:text-destructive"
              disabled={!canUninstall || installing || uninstalling}
              onClick={() => onUninstall?.()}
              title={
                canUninstall
                  ? "Run full uninstall script and clean this software"
                  : "No uninstall script configured for this software"
              }
            >
              {uninstalling ? (
                <Loader2 className="mr-2 h-4 w-4 animate-spin" />
              ) : (
                <Trash2 className="mr-2 h-4 w-4" />
              )}
              {uninstalling ? "Uninstalling…" : "Uninstall completely"}
            </Button>
          )}

          {(isInstalled || installedVersion || latest) && (
            <div className="rounded-lg border bg-muted/30 p-3 text-xs text-muted-foreground">
              <div className="flex justify-between">
                <span>Installed</span>
                <span className="font-mono text-foreground">
                  {installedVersion
                    ? `v${installedVersion.version}`
                    : isInstalled
                      ? "Yes"
                      : "—"}
                </span>
              </div>
              <div className="mt-1 flex justify-between">
                <span>Latest</span>
                <span className="font-mono text-foreground">
                  {latest ? `v${latest.version}` : "—"}
                </span>
              </div>
            </div>
          )}

          {!canInstall && (
            <p className="text-xs text-amber-700 dark:text-amber-400">
              {latest
                ? "Latest version has no install script."
                : "No version available to install yet."}
            </p>
          )}
        </div>
      </div>

      <InstallTerminal
        open={terminal.open}
        status={terminal.status}
        lines={terminal.lines}
        title={`${software.name} ${hasUpdate ? "update" : isInstalled ? "reinstall" : "install"}`}
        subtitle={
          latest
            ? `v${latest.version}${terminal.jobId ? ` · job ${terminal.jobId.slice(0, 8)}` : ""}`
            : undefined
        }
        failureReason={terminal.failureReason}
        cancelling={terminal.cancelling}
        onCancel={terminal.onCancel}
        onRetry={terminal.onRetry}
        onClear={terminal.onClear}
        onClose={terminal.onClose}
        className="mt-6"
      />

      <Tabs value={tab} onValueChange={setTab} className="mt-8">
        <TabsList>
          <TabsTrigger value="overview">Overview</TabsTrigger>
          {canControl ? (
            <TabsTrigger value="service">Service</TabsTrigger>
          ) : null}
          <TabsTrigger value="versions">Versions</TabsTrigger>
        </TabsList>

        <TabsContent value="overview" className="mt-4 space-y-6">
          {canControl ? (
            <SoftwareServicePanel
              softwareId={software.id}
              softwareName={software.name}
              units={software.service_units || []}
              initialStatus={serviceStatus}
              isInstalled={isInstalled}
              onStatusChange={onServiceStatusChange}
            />
          ) : null}
          <div className="rounded-lg border p-5">
            <h2 className="mb-2 text-sm font-semibold tracking-wide text-muted-foreground uppercase">
              About
            </h2>
            <p className="text-sm leading-relaxed whitespace-pre-wrap">
              {software.details || "No description available."}
            </p>
          </div>
          {software.tags?.length ? (
            <div>
              <h2 className="mb-2 text-sm font-semibold tracking-wide text-muted-foreground uppercase">
                Tags
              </h2>
              <div className="flex flex-wrap gap-1.5">
                {software.tags.map((t) => (
                  <Badge key={t} variant="secondary" className="font-mono text-xs">
                    {t}
                  </Badge>
                ))}
              </div>
            </div>
          ) : null}
        </TabsContent>

        {canControl ? (
          <TabsContent value="service" className="mt-4 space-y-6">
            <SoftwareServicePanel
              softwareId={software.id}
              softwareName={software.name}
              units={software.service_units || []}
              initialStatus={serviceStatus}
              isInstalled={isInstalled}
              onStatusChange={onServiceStatusChange}
            />
          </TabsContent>
        ) : null}

        <TabsContent value="versions" className="mt-4 space-y-2">
          {versions.length === 0 ? (
            <p className="text-sm text-muted-foreground">No versions yet.</p>
          ) : (
            versions.map((ver) => {
              const isLatest = latest?.id === ver.id
              return (
                <div key={ver.id} className="rounded-lg border p-4">
                  <div className="flex items-center justify-between gap-4">
                    <div className="flex flex-wrap items-center gap-2">
                      <span
                        className={cn(
                          "font-mono text-sm font-semibold",
                          isLatest && "text-foreground"
                        )}
                      >
                        v{ver.version}
                      </span>
                      {isLatest && (
                        <Badge className="bg-primary/10 text-primary hover:bg-primary/20">
                          Latest
                        </Badge>
                      )}
                      {ver.is_installed && (
                        <Badge className="bg-emerald-500/10 text-emerald-600 hover:bg-emerald-500/20 dark:text-emerald-400">
                          <CheckCircle2 className="mr-1 h-3 w-3" />
                          Current
                        </Badge>
                      )}
                      {ver.has_update && (
                        <Badge className="bg-amber-500/10 text-amber-600 hover:bg-amber-500/20 dark:text-amber-400">
                          Update
                        </Badge>
                      )}
                    </div>
                    <span className="text-xs text-muted-foreground">
                      {ver.install_script ? "Script ready" : "No script"}
                    </span>
                  </div>
                  {ver.updated_at || ver.created_at ? (
                    <p className="mt-2 text-xs text-muted-foreground">
                      {new Date(ver.updated_at || ver.created_at || "").toLocaleString()}
                    </p>
                  ) : null}
                </div>
              )
            })
          )}
        </TabsContent>
      </Tabs>
    </div>
  )
}
