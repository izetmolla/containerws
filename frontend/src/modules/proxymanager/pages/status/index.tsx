import { useState } from "react"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { FileSearch, Loader2, Play } from "lucide-react"
import { toast } from "sonner"

import ContentLoader from "@/components/content-loader"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Separator } from "@/components/ui/separator"
import { asArray } from "@/lib/as-array"
import { toastRequestError } from "@/lib/network"

import {
  getProxyStatus,
  listProxyRuns,
  previewProxy,
  PROXY_RUNS_KEY,
  PROXY_STATUS_KEY,
} from "../_shared/api"
import { runProxyApply } from "../_shared/apply"
import {
  DirtyBanner,
  PROXY_PAGE_DESCRIPTIONS,
  ProxyRefreshButton,
  ProxySubNav,
  SummaryChip,
} from "../_shared/page-chrome"

function formatWhen(value?: string | null) {
  if (!value) return "—"
  const d = new Date(value)
  if (Number.isNaN(d.getTime())) return value
  return new Intl.DateTimeFormat(undefined, {
    dateStyle: "medium",
    timeStyle: "short",
  }).format(d)
}

export default function ProxyStatusPage() {
  const queryClient = useQueryClient()
  const [preview, setPreview] = useState<Record<string, string> | null>(null)

  const statusQuery = useQuery({
    queryKey: [PROXY_STATUS_KEY],
    queryFn: getProxyStatus,
    refetchInterval: 15_000,
  })
  const runsQuery = useQuery({
    queryKey: [PROXY_RUNS_KEY],
    queryFn: listProxyRuns,
  })

  const applyMutation = useMutation({
    mutationFn: () => runProxyApply(queryClient),
  })

  const previewMutation = useMutation({
    mutationFn: previewProxy,
    onSuccess: (res) => {
      setPreview(res.data?.contents || {})
      if (res.data?.error) toast.warning(res.data.error)
      else toast.success("Preview generated")
    },
    onError: (err) => toastRequestError(err, "Preview failed"),
  })

  const data = statusQuery.data?.data
  const settings = data?.settings
  const runtime = data?.runtime
  const runs = asArray(runsQuery.data?.data)

  return (
    <ContentLoader
      title="Proxy Status"
      description={PROXY_PAGE_DESCRIPTIONS.status}
      breadcrumb={[
        { label: "Proxy Manager", to: "/proxymanager" },
        { label: "Status" },
      ]}
      isLoading={statusQuery.isLoading}
      error={statusQuery.error}
      rightComponent={
        <div className="flex flex-wrap gap-2">
          <ProxyRefreshButton
            isFetching={statusQuery.isFetching}
            onClick={() => {
              void statusQuery.refetch()
              void runsQuery.refetch()
            }}
          />
          <Button
            variant="outline"
            size="sm"
            onClick={() => previewMutation.mutate()}
            disabled={previewMutation.isPending}
          >
            <FileSearch className="size-3.5" />
            Preview
          </Button>
          <Button
            size="sm"
            onClick={() => applyMutation.mutate()}
            disabled={applyMutation.isPending}
          >
            {applyMutation.isPending ? (
              <Loader2 className="size-3.5 animate-spin" />
            ) : (
              <Play className="size-3.5" />
            )}
            {applyMutation.isPending ? "Applying…" : "Apply"}
          </Button>
        </div>
      }
    >
      <ProxySubNav />
      <DirtyBanner
        dirty={settings?.dirty}
        lastError={settings?.last_apply_error}
        onApply={() => applyMutation.mutate()}
        applying={applyMutation.isPending}
      />

      <div className="mb-6 grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
        <SummaryChip label="Engine" value={settings?.active_engine || "—"} />
        <SummaryChip
          label="Components"
          value={
            data?.components?.ready === false ? (
              <span className="text-destructive">
                Missing: {(data.components.missing || []).join(", ") || "yes"}
              </span>
            ) : data?.module_enabled === false ? (
              "Module disabled"
            ) : (
              "Ready"
            )
          }
        />
        <SummaryChip
          label="Dirty"
          value={settings?.dirty ? "Yes — needs apply" : "Clean"}
        />
        <SummaryChip
          label="Docker"
          value={
            runtime?.docker_available
              ? "Available"
              : runtime?.docker_error || "Unavailable"
          }
        />
      </div>

      {data?.last_run?.id ? (
        <div className="mb-6 rounded-xl border bg-muted/20 p-4">
          <div className="mb-2 flex items-center gap-2">
            <h2 className="text-sm font-semibold">Last apply run</h2>
            <Badge
              variant={
                data.last_run.status === "success" ? "default" : "destructive"
              }
            >
              {data.last_run.status}
            </Badge>
            <span className="text-xs text-muted-foreground">
              {data.last_run.engine} · {formatWhen(data.last_run.started_at)}
            </span>
          </div>
          {data.last_run.error_text ? (
            <p className="mb-2 text-sm text-destructive">
              {data.last_run.error_text}
            </p>
          ) : null}
          <pre className="max-h-48 overflow-auto rounded-lg bg-background p-3 text-xs">
            {data.last_run.log_text || "—"}
          </pre>
        </div>
      ) : null}

      <Separator className="my-6" />

      <h2 className="mb-3 text-base font-semibold">Apply history</h2>
      <div className="space-y-2">
        {runs.length === 0 ? (
          <p className="text-sm text-muted-foreground">No apply runs yet.</p>
        ) : (
          runs.map((run) => (
            <div
              key={run.id}
              className="flex flex-wrap items-center justify-between gap-2 rounded-lg border px-3 py-2 text-sm"
            >
              <div className="flex items-center gap-2">
                <Badge variant="secondary">{run.engine}</Badge>
                <Badge
                  variant={run.status === "success" ? "default" : "outline"}
                >
                  {run.status}
                </Badge>
                <span className="text-muted-foreground">
                  {formatWhen(run.started_at)}
                </span>
              </div>
              {run.error_text ? (
                <span className="truncate text-xs text-destructive">
                  {run.error_text}
                </span>
              ) : null}
            </div>
          ))
        )}
      </div>

      {preview ? (
        <>
          <Separator className="my-6" />
          <h2 className="mb-3 text-base font-semibold">Generated config preview</h2>
          <div className="space-y-4">
            {Object.keys(preview).length === 0 ? (
              <p className="text-sm text-muted-foreground">No files generated.</p>
            ) : (
              Object.entries(preview).map(([path, content]) => (
                <div key={path} className="rounded-xl border">
                  <div className="border-b bg-muted/30 px-3 py-2 font-mono text-xs">
                    {path}
                  </div>
                  <pre className="max-h-64 overflow-auto p-3 text-xs">{content}</pre>
                </div>
              ))
            )}
          </div>
        </>
      ) : null}
    </ContentLoader>
  )
}
