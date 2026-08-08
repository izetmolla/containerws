import { useMutation, useQueryClient } from "@tanstack/react-query"
import { Loader2, Play, Power, RotateCcw } from "lucide-react"
import { toast } from "sonner"

import { Button } from "@/components/ui/button"
import { ButtonGroup } from "@/components/ui/button-group"
import { toastRequestError } from "@/lib/network"
import { cn } from "@/lib/utils"

import {
  controlDockerEngine,
  DOCKER_ENGINE_KEY,
  type EngineControlAction,
} from "./engine-api"
import { useEngineStatus } from "./use-engine-status"
import { DOCKER_ENV_KEY } from "../environments/api"

/** Start / restart / stop Docker Engine (via Softwares docker.service). */
export function EngineControls({ className }: { className?: string }) {
  const queryClient = useQueryClient()
  const q = useEngineStatus()
  const data = q.data?.data
  const engine = data?.engine
  const canControl = Boolean(engine?.can_control && engine?.software_id)
  const running = Boolean(engine?.running || data?.reachable)
  const installed = Boolean(engine?.installed || engine?.binary_present)

  const controlMutation = useMutation({
    mutationFn: (action: EngineControlAction) => controlDockerEngine(action),
    onSuccess: (res, action) => {
      toast.success(res.message || `Docker Engine ${action} requested`)
      void queryClient.invalidateQueries({ queryKey: [DOCKER_ENGINE_KEY] })
      void queryClient.invalidateQueries({ queryKey: [DOCKER_ENV_KEY] })
    },
    onError: (err) => toastRequestError(err, "Docker Engine action failed"),
  })

  if (!canControl || !installed) return null

  const busy = controlMutation.isPending
  const overall = engine?.service?.overall || (running ? "running" : "stopped")

  return (
    <div
      className={cn(
        "flex flex-wrap items-center justify-between gap-3 rounded-xl border bg-card px-4 py-3 text-sm shadow-xs",
        className
      )}
    >
      <div className="min-w-0">
        <p className="font-medium">Docker Engine</p>
        <p className="mt-0.5 text-xs text-muted-foreground">
          Service{" "}
          <span className="font-mono">{overall}</span>
          {data?.reachable ? " · API reachable" : " · API not reachable"}
          {data?.sock ? (
            <>
              {" "}
              · <span className="font-mono">{data.sock}</span>
            </>
          ) : null}
        </p>
      </div>
      <ButtonGroup aria-label="Docker Engine actions">
        <Button
          size="sm"
          variant={running ? "outline" : "default"}
          disabled={busy || running}
          onClick={() => controlMutation.mutate("start")}
        >
          {busy && controlMutation.variables === "start" ? (
            <Loader2 data-icon="inline-start" className="animate-spin" />
          ) : (
            <Play data-icon="inline-start" />
          )}
          Start
        </Button>
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
        <Button
          size="sm"
          variant="outline"
          disabled={busy || !running}
          onClick={() => controlMutation.mutate("stop")}
        >
          {busy && controlMutation.variables === "stop" ? (
            <Loader2 data-icon="inline-start" className="animate-spin" />
          ) : (
            <Power data-icon="inline-start" />
          )}
          Stop
        </Button>
      </ButtonGroup>
    </div>
  )
}
