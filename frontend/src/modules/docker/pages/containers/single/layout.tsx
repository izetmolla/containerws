import { useLocation, useNavigate, useParams, Outlet } from "react-router"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { Play, RotateCcw, Square, Trash2 } from "lucide-react"
import { toast } from "sonner"

import ContentLoader from "@/components/content-loader"
import { Button } from "@/components/ui/button"
import { toastRequestError } from "@/lib/network"
import { cn } from "@/lib/utils"

import { EngineBanner } from "../../_shared/engine-status"
import { stateBadgeClass } from "../../_shared/engine-format"
import { EnvironmentSelector } from "../../_shared/environment-selector"
import { DockerRefreshButton } from "../../_shared/page-chrome"
import { DockerSubNav } from "../../_shared/resource-ui"
import {
  DOCKER_CONTAINERS_KEY,
  getContainer,
  removeContainer,
  restartContainer,
  startContainer,
  stopContainer,
  type ContainerDetail,
} from "../list/api"

export type ContainerOutletContext = {
  id: string
  container: ContainerDetail
  invalidate: () => void
}

const SUB_TABS = [
  { to: ".", end: true, label: "Overview" },
  { to: "logs", label: "Logs" },
  { to: "inspect", label: "Inspect" },
  { to: "stats", label: "Stats" },
  { to: "console", label: "Console" },
]

export default function ContainerDetailLayout() {
  const { id = "" } = useParams()
  const location = useLocation()
  const navigate = useNavigate()
  const queryClient = useQueryClient()

  const detailQuery = useQuery({
    queryKey: [DOCKER_CONTAINERS_KEY, id],
    queryFn: () => getContainer(id),
    enabled: Boolean(id),
    refetchInterval: 8_000,
  })

  const invalidate = () => {
    void queryClient.invalidateQueries({ queryKey: [DOCKER_CONTAINERS_KEY, id] })
    void queryClient.invalidateQueries({ queryKey: [DOCKER_CONTAINERS_KEY] })
  }

  const actionMutation = useMutation({
    mutationFn: async (action: "start" | "stop" | "restart" | "remove") => {
      if (action === "start") return startContainer(id)
      if (action === "stop") return stopContainer(id)
      if (action === "restart") return restartContainer(id)
      return removeContainer(id, { force: true, volumes: true })
    },
    onSuccess: (res, action) => {
      toast.success(res.message || `${action} ok`)
      if (action === "remove") {
        navigate("/docker/containers")
        return
      }
      invalidate()
    },
    onError: (err) => toastRequestError(err, "Container action failed"),
  })

  const data = detailQuery.data?.data
  const name = data?.name || data?.short_id || id
  const base = `/docker/containers/${id}`
  const state = (data?.state || "").toLowerCase()
  const running = state === "running"
  const paused = state === "paused"

  const onLogs = location.pathname.endsWith("/logs")
  const onInspect = location.pathname.endsWith("/inspect")
  const onStats = location.pathname.endsWith("/stats")
  const onConsole = location.pathname.endsWith("/console")

  const breadcrumb = [
    { label: "Docker", to: "/docker" },
    { label: "Containers", to: "/docker/containers" },
    { label: name, to: base },
    ...(onLogs ? [{ label: "Logs" }] : []),
    ...(onInspect ? [{ label: "Inspect" }] : []),
    ...(onStats ? [{ label: "Stats" }] : []),
    ...(onConsole ? [{ label: "Console" }] : []),
  ]

  const busy = actionMutation.isPending

  return (
    <ContentLoader
      title={name}
      description={
        data
          ? `${data.short_id || data.id.slice(0, 12)} · ${data.status || data.state}`
          : "Container details"
      }
      showHeaderSeparator
      breadcrumb={breadcrumb}
      isLoading={detailQuery.isLoading}
      error={detailQuery.error}
      rightComponent={
        <div className="flex flex-wrap items-center gap-2">
          <EnvironmentSelector />
          <DockerRefreshButton
            onClick={() => void detailQuery.refetch()}
            isFetching={detailQuery.isFetching}
          />
        </div>
      }
    >
      <div className="flex w-full min-w-0 flex-col gap-4">
        <EngineBanner />

        {data ? (
          <div className="flex flex-col gap-3 rounded-xl border bg-muted/15 px-4 py-3">
            <div className="flex flex-wrap items-center gap-2">
              <h2 className="text-base font-semibold tracking-tight">{name}</h2>
              <span
                className={cn(
                  "inline-flex rounded-full px-2 py-0.5 text-[11px] font-medium capitalize",
                  stateBadgeClass(data.state),
                )}
              >
                {data.state}
              </span>
              <span className="font-mono text-xs text-muted-foreground">
                {data.short_id || data.id.slice(0, 12)}
              </span>
              <span className="text-xs text-muted-foreground">
                {data.status}
              </span>
            </div>
            <div className="flex flex-wrap items-center gap-2">
              {!running && !paused ? (
                <Button
                  size="sm"
                  disabled={busy}
                  onClick={() => actionMutation.mutate("start")}
                >
                  <Play data-icon="inline-start" />
                  Start
                </Button>
              ) : null}
              {running || paused ? (
                <Button
                  size="sm"
                  variant="outline"
                  disabled={busy}
                  onClick={() => actionMutation.mutate("stop")}
                >
                  <Square data-icon="inline-start" />
                  Stop
                </Button>
              ) : null}
              <Button
                size="sm"
                variant="outline"
                disabled={busy}
                onClick={() => actionMutation.mutate("restart")}
              >
                <RotateCcw data-icon="inline-start" />
                Restart
              </Button>
              <Button
                size="sm"
                variant="destructive"
                disabled={busy}
                onClick={() => {
                  if (
                    window.confirm(
                      `Remove container ${name}? Linked anonymous volumes will also be removed.`,
                    )
                  ) {
                    actionMutation.mutate("remove")
                  }
                }}
              >
                <Trash2 data-icon="inline-start" />
                Remove
              </Button>
            </div>
          </div>
        ) : null}

        <DockerSubNav base={base} tabs={[...SUB_TABS]} />

        {data ? (
          <Outlet
            context={
              {
                id,
                container: data,
                invalidate,
              } satisfies ContainerOutletContext
            }
          />
        ) : null}
      </div>
    </ContentLoader>
  )
}
