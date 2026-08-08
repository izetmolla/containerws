import { useState } from "react"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import {
  ArrowUpCircle,
  CheckCircle2,
  ExternalLink,
  Loader2,
  RefreshCw,
} from "lucide-react"
import { toast } from "sonner"

import ContentLoader from "@/components/content-loader"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Separator } from "@/components/ui/separator"
import { getRequestErrorMessage, withError } from "@/lib/network"
import { cn } from "@/lib/utils"

import {
  applyUpdate,
  checkForUpdates,
  getUpdateStatus,
  listUpdateReleases,
  SETTINGS_UPDATE_FETCH_KEY,
  type UpdateRelease,
} from "./api"

function formatBytes(n?: number) {
  if (!n || n <= 0) return ""
  if (n < 1024) return `${n} B`
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`
  return `${(n / (1024 * 1024)).toFixed(1)} MB`
}

function formatWhen(iso?: string) {
  if (!iso) return "Never"
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return iso
  return d.toLocaleString()
}

/** Poll until the API is back after binary restart, then reload the UI. */
async function waitForRestartThenReload() {
  const started = Date.now()
  // Brief grace period while the process exits / systemd restarts.
  await new Promise((r) => window.setTimeout(r, 1500))
  for (let i = 0; i < 60; i++) {
    try {
      const res = await getUpdateStatus()
      if (res?.data && !res.error) {
        window.location.reload()
        return
      }
    } catch {
      // Server still down — keep waiting.
    }
    if (Date.now() - started > 90_000) break
    await new Promise((r) => window.setTimeout(r, 1000))
  }
  window.location.reload()
}

export default function UpdateSettingsPage() {
  const queryClient = useQueryClient()
  const [confirmTarget, setConfirmTarget] = useState<UpdateRelease | null>(null)
  const [applyPhase, setApplyPhase] = useState<
    "idle" | "downloading" | "restarting"
  >("idle")

  const statusQuery = useQuery({
    queryKey: [SETTINGS_UPDATE_FETCH_KEY, "status"],
    queryFn: getUpdateStatus,
  })

  const releasesQuery = useQuery({
    queryKey: [SETTINGS_UPDATE_FETCH_KEY, "releases"],
    queryFn: listUpdateReleases,
  })

  const invalidate = () => {
    void queryClient.invalidateQueries({
      queryKey: [SETTINGS_UPDATE_FETCH_KEY],
    })
  }

  const checkMutation = useMutation({
    mutationFn: checkForUpdates,
    onSuccess: (res) => {
      toast.success(res.message || "Checked for updates")
      invalidate()
    },
    onError: (err) => {
      toast.error(getRequestErrorMessage(err, "Could not check for updates"))
      invalidate()
    },
  })

  const applyMutation = useMutation({
    mutationFn: async (rel: UpdateRelease) => {
      setApplyPhase("downloading")
      return applyUpdate(rel.tag, false)
    },
    onSuccess: (res) => {
      setApplyPhase("restarting")
      toast.success(res.message || "Update installed — restarting…")
      setConfirmTarget(null)
      void waitForRestartThenReload()
    },
    onError: (err) => {
      setApplyPhase("idle")
      toast.error(getRequestErrorMessage(err, "Update failed"))
    },
  })

  const status = statusQuery.data?.data
  const releases = releasesQuery.data?.data?.releases ?? []

  const busy =
    checkMutation.isPending ||
    applyMutation.isPending ||
    applyPhase === "restarting" ||
    applyPhase === "downloading"

  return (
    <ContentLoader
      title="Update"
      description="Check GitHub Releases and update this machine’s Container Workspace binary."
      breadcrumb={[
        { label: "Settings", to: "/settings" },
        { label: "Update" },
      ]}
      isLoading={statusQuery.isLoading}
      error={withError(statusQuery.error, statusQuery.data)}
      showHeaderSeparator
    >
      <div className="w-full space-y-6">
        <section className="rounded-xl border border-border/70 bg-card/40 p-4 sm:p-5">
          <div className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
            <div className="min-w-0 space-y-2">
              <div className="flex flex-wrap items-center gap-2">
                <h2 className="text-base font-semibold tracking-tight">
                  Current installation
                </h2>
                {status?.update_available ? (
                  <Badge className="bg-sky-600 text-white hover:bg-sky-600/90">
                    Update available
                  </Badge>
                ) : (
                  <Badge variant="outline">Up to date</Badge>
                )}
              </div>
              <dl className="grid gap-1.5 text-sm sm:grid-cols-2">
                <div>
                  <dt className="text-xs text-muted-foreground">Version</dt>
                  <dd className="font-mono">{status?.current_version || "—"}</dd>
                </div>
                <div>
                  <dt className="text-xs text-muted-foreground">Commit</dt>
                  <dd className="font-mono text-xs">
                    {status?.commit_sha || "—"}
                  </dd>
                </div>
                <div>
                  <dt className="text-xs text-muted-foreground">Platform</dt>
                  <dd className="font-mono text-xs">
                    {status?.goos}/{status?.goarch}
                  </dd>
                </div>
                <div>
                  <dt className="text-xs text-muted-foreground">Latest release</dt>
                  <dd className="font-mono text-xs">
                    {status?.latest_tag || "—"}
                  </dd>
                </div>
                <div className="sm:col-span-2">
                  <dt className="text-xs text-muted-foreground">Binary</dt>
                  <dd className="truncate font-mono text-xs text-muted-foreground">
                    {status?.binary_path || "—"}
                  </dd>
                </div>
                <div className="sm:col-span-2">
                  <dt className="text-xs text-muted-foreground">Repository</dt>
                  <dd className="font-mono text-xs">
                    {status?.repo ? (
                      <a
                        className="inline-flex items-center gap-1 text-foreground underline-offset-2 hover:underline"
                        href={`https://github.com/${status.repo}/releases`}
                        target="_blank"
                        rel="noreferrer"
                      >
                        {status.repo}
                        <ExternalLink className="size-3" />
                      </a>
                    ) : (
                      "—"
                    )}
                  </dd>
                </div>
                <div>
                  <dt className="text-xs text-muted-foreground">Last checked</dt>
                  <dd className="text-xs">{formatWhen(status?.last_check)}</dd>
                </div>
                <div>
                  <dt className="text-xs text-muted-foreground">Expected asset</dt>
                  <dd className="truncate font-mono text-[11px] text-muted-foreground">
                    {status?.expected_asset || "—"}
                  </dd>
                </div>
              </dl>
              {status?.last_error ? (
                <p className="rounded-md border border-amber-500/30 bg-amber-500/10 px-3 py-2 text-xs text-amber-900 dark:text-amber-200">
                  {status.last_error}
                </p>
              ) : null}
            </div>
            <Button
              type="button"
              size="sm"
              disabled={busy}
              onClick={() => checkMutation.mutate()}
            >
              {checkMutation.isPending ? (
                <Loader2 className="size-4 animate-spin" />
              ) : (
                <RefreshCw className="size-4" />
              )}
              Check for updates
            </Button>
          </div>
        </section>

        <Separator />

        <section className="space-y-3">
          <div className="flex items-center justify-between gap-2">
            <h2 className="text-sm font-semibold tracking-wide text-muted-foreground uppercase">
              Releases
            </h2>
            {releasesQuery.isFetching ? (
              <Loader2 className="size-4 animate-spin text-muted-foreground" />
            ) : null}
          </div>

          {releasesQuery.isLoading ? (
            <div className="flex items-center gap-2 py-8 text-sm text-muted-foreground">
              <Loader2 className="size-4 animate-spin" />
              Loading releases…
            </div>
          ) : releases.length === 0 ? (
            <div className="rounded-lg border border-dashed border-border/80 px-4 py-10 text-center text-sm text-muted-foreground">
              No releases cached yet. Click <strong>Check for updates</strong>.
            </div>
          ) : (
            <ul className="divide-y divide-border/70 rounded-lg border border-border/70">
              {releases.map((rel) => (
                <li
                  key={rel.tag}
                  className={cn(
                    "flex flex-col gap-3 px-3 py-3 sm:flex-row sm:items-center sm:justify-between",
                    rel.prerelease && "opacity-80"
                  )}
                >
                  <div className="min-w-0 space-y-1">
                    <div className="flex flex-wrap items-center gap-2">
                      <code className="font-mono text-sm font-medium">
                        {rel.tag}
                      </code>
                      {rel.newer ? (
                        <Badge variant="secondary">Newer</Badge>
                      ) : null}
                      {rel.prerelease ? (
                        <Badge variant="outline">Pre-release</Badge>
                      ) : null}
                      {!rel.has_asset ? (
                        <Badge variant="outline" className="text-amber-700">
                          No asset for this OS
                        </Badge>
                      ) : null}
                    </div>
                    <p className="text-xs text-muted-foreground">
                      {formatWhen(rel.published_at)}
                      {rel.asset_name
                        ? ` · ${rel.asset_name}${
                            rel.asset_size
                              ? ` (${formatBytes(rel.asset_size)})`
                              : ""
                          }`
                        : ""}
                    </p>
                  </div>
                  <div className="flex shrink-0 flex-wrap gap-1.5">
                    {rel.html_url ? (
                      <Button type="button" size="sm" variant="ghost" asChild>
                        <a href={rel.html_url} target="_blank" rel="noreferrer">
                          <ExternalLink className="size-3.5" />
                          Notes
                        </a>
                      </Button>
                    ) : null}
                    <Button
                      type="button"
                      size="sm"
                      disabled={busy || !rel.has_asset}
                      onClick={() => setConfirmTarget(rel)}
                    >
                      <ArrowUpCircle className="size-3.5" />
                      Update
                    </Button>
                  </div>
                </li>
              ))}
            </ul>
          )}
        </section>
      </div>

      <Dialog
        open={Boolean(confirmTarget)}
        onOpenChange={(open) => {
          if (!open && applyPhase === "idle") setConfirmTarget(null)
        }}
      >
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>Install update {confirmTarget?.tag}?</DialogTitle>
            <DialogDescription>
              Downloads the matching binary for{" "}
              <code className="font-mono text-xs">
                {status?.goos}/{status?.goarch}
              </code>
              , replaces{" "}
              <code className="font-mono text-xs">
                {status?.binary_path || "the running binary"}
              </code>
              , and restarts the process. A backup is kept as{" "}
              <code className="font-mono text-xs">.bak</code>.
            </DialogDescription>
          </DialogHeader>
          {applyPhase !== "idle" ? (
            <div className="flex items-center gap-2 rounded-md border border-border/70 bg-muted/40 px-3 py-2 text-sm">
              <Loader2 className="size-4 animate-spin" />
              {applyPhase === "downloading"
                ? "Downloading and installing…"
                : "Restarting — the page will reload…"}
            </div>
          ) : (
            <div className="space-y-1 text-sm text-muted-foreground">
              <p className="flex items-center gap-1.5">
                <CheckCircle2 className="size-3.5 text-emerald-600" />
                Asset:{" "}
                <code className="font-mono text-xs text-foreground">
                  {confirmTarget?.asset_name || "—"}
                </code>
              </p>
            </div>
          )}
          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              disabled={applyMutation.isPending || applyPhase !== "idle"}
              onClick={() => setConfirmTarget(null)}
            >
              Cancel
            </Button>
            <Button
              type="button"
              disabled={
                !confirmTarget ||
                applyMutation.isPending ||
                applyPhase !== "idle"
              }
              onClick={() => {
                if (confirmTarget) applyMutation.mutate(confirmTarget)
              }}
            >
              {applyMutation.isPending ? (
                <Loader2 className="size-4 animate-spin" />
              ) : (
                <ArrowUpCircle className="size-4" />
              )}
              Install & restart
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </ContentLoader>
  )
}
