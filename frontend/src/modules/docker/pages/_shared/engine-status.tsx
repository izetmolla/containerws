import { Link } from "react-router"
import { useMutation, useQueryClient } from "@tanstack/react-query"
import {
  AlertTriangle,
  Box,
  Loader2,
  Play,
  Power,
  RotateCcw,
} from "lucide-react"
import { toast } from "sonner"

import { Button } from "@/components/ui/button"
import { toastRequestError } from "@/lib/network"
import { cn } from "@/lib/utils"

import {
  controlDockerEngine,
  DOCKER_ENGINE_KEY,
  type EngineControlAction,
} from "./engine-api"
import { useEngineStatus } from "./use-engine-status"
import { DOCKER_ENV_KEY } from "../environments/api"

export function EngineBanner({
  className,
  showControls = true,
}: {
  className?: string
  /** When false, only links are shown (no start/stop/restart). */
  showControls?: boolean
}) {
  const queryClient = useQueryClient()
  const q = useEngineStatus()
  const data = q.data?.data

  const controlMutation = useMutation({
    mutationFn: (action: EngineControlAction) => controlDockerEngine(action),
    onSuccess: (res, action) => {
      toast.success(res.message || `Docker Engine ${action} requested`)
      void queryClient.invalidateQueries({ queryKey: [DOCKER_ENGINE_KEY] })
      void queryClient.invalidateQueries({ queryKey: [DOCKER_ENV_KEY] })
    },
    onError: (err) => toastRequestError(err, "Docker Engine action failed"),
  })

  if (q.isLoading || data?.reachable) return null

  const envName = data?.environment?.name
  const engine = data?.engine
  const canControl = Boolean(engine?.can_control && engine?.software_id)
  const running = Boolean(engine?.running)
  const installed = Boolean(engine?.installed || engine?.binary_present)
  const busy = controlMutation.isPending

  return (
    <div
      className={cn(
        "flex flex-wrap items-center justify-between gap-3 rounded-xl border border-amber-500/30 bg-amber-500/10 px-4 py-3 text-sm",
        className
      )}
    >
      <div className="flex min-w-0 items-start gap-2.5">
        <AlertTriangle className="mt-0.5 size-4 shrink-0 text-amber-700 dark:text-amber-300" />
        <div className="min-w-0">
          <p className="font-medium text-amber-900 dark:text-amber-200">
            {data?.env_disabled
              ? "Docker environment is disabled"
              : "Docker Engine is not reachable"}
            {envName ? (
              <span className="font-normal text-amber-800/80 dark:text-amber-300/80">
                {" "}
                · {envName}
              </span>
            ) : null}
          </p>
          <p className="mt-0.5 text-xs text-amber-800/80 dark:text-amber-300/80">
            {data?.error ||
              (data?.env_disabled
                ? "Enable the environment below, or start Docker Engine if it is stopped."
                : "Start Docker Engine, enable the Local environment, then refresh.")}
          </p>
          {data?.sock ? (
            <p className="mt-1 font-mono text-[11px] text-amber-800/70 dark:text-amber-300/70">
              Endpoint: {data.sock}
            </p>
          ) : null}
        </div>
      </div>
      <div className="flex flex-wrap items-center gap-2">
        {showControls && canControl && installed ? (
          <>
            {!running ? (
              <Button
                size="sm"
                disabled={busy}
                onClick={() => controlMutation.mutate("start")}
              >
                {busy && controlMutation.variables === "start" ? (
                  <Loader2 data-icon="inline-start" className="animate-spin" />
                ) : (
                  <Play data-icon="inline-start" />
                )}
                Start engine
              </Button>
            ) : null}
            <Button
              size="sm"
              variant="outline"
              disabled={busy}
              onClick={() => controlMutation.mutate("restart")}
            >
              {busy && controlMutation.variables === "restart" ? (
                <Loader2 data-icon="inline-start" className="animate-spin" />
              ) : (
                <RotateCcw data-icon="inline-start" />
              )}
              Restart
            </Button>
            {running ? (
              <Button
                size="sm"
                variant="outline"
                disabled={busy}
                onClick={() => controlMutation.mutate("stop")}
              >
                {busy && controlMutation.variables === "stop" ? (
                  <Loader2 data-icon="inline-start" className="animate-spin" />
                ) : (
                  <Power data-icon="inline-start" />
                )}
                Stop
              </Button>
            ) : null}
          </>
        ) : null}
        <Button size="sm" variant="outline" asChild>
          <Link to="/docker/environments">Environments</Link>
        </Button>
        <Button size="sm" variant="outline" asChild>
          <Link to="/softwares">
            <Box data-icon="inline-start" />
            Softwares
          </Link>
        </Button>
      </div>
    </div>
  )
}
