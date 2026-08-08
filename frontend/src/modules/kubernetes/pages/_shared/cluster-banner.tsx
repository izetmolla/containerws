import { Link } from "react-router"
import { AlertTriangle, Settings } from "lucide-react"
import { useQuery } from "@tanstack/react-query"

import { Button } from "@/components/ui/button"
import { cn } from "@/lib/utils"

import { getClusterStatus, K8S_CLUSTER_KEY } from "./api"
import { SystemResourcesToggle } from "./system-toggle"

export function useClusterStatus() {
  return useQuery({
    queryKey: [K8S_CLUSTER_KEY, "status"],
    queryFn: getClusterStatus,
    refetchInterval: 15_000,
  })
}

export function ClusterBanner({ className }: { className?: string }) {
  const q = useClusterStatus()
  const data = q.data?.data
  const unreachable = !q.isLoading && data && !data.reachable

  return (
    <div className={cn("space-y-3", className)}>
      {unreachable ? (
        <div className="flex flex-wrap items-center justify-between gap-3 rounded-xl border border-amber-500/30 bg-amber-500/10 px-4 py-3 text-sm">
          <div className="flex min-w-0 items-start gap-2.5">
            <AlertTriangle className="mt-0.5 size-4 shrink-0 text-amber-700 dark:text-amber-300" />
            <div className="min-w-0">
              <p className="font-medium text-amber-900 dark:text-amber-200">
                Kubernetes cluster is not reachable
              </p>
              <p className="mt-0.5 text-xs text-amber-800/80 dark:text-amber-300/80">
                {data?.error ||
                  "Set a valid kubeconfig path under Kubernetes → Settings."}
              </p>
            </div>
          </div>
          <Button size="sm" variant="outline" asChild>
            <Link to="/kubernetes/settings">
              <Settings data-icon="inline-start" />
              Settings
            </Link>
          </Button>
        </div>
      ) : null}

      <div className="flex justify-end">
        <SystemResourcesToggle />
      </div>
    </div>
  )
}
