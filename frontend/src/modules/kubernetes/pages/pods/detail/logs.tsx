import { useQuery } from "@tanstack/react-query"
import { RefreshCw } from "lucide-react"
import { useEffect, useState } from "react"
import { useOutletContext } from "react-router"

import { Button } from "@/components/ui/button"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"

import { getPodLogs, K8S_PODS_KEY } from "../../_shared/api"
import { LogsViewer } from "../../_shared/resource-ui"
import type { PodOutletContext } from "./layout"

export default function PodLogsPage() {
  const { namespace, name, pod } = useOutletContext<PodOutletContext>()
  const [container, setContainer] = useState(pod.containers[0]?.name || "")
  useEffect(() => {
    if (!pod.containers.some((item) => item.name === container)) {
      setContainer(pod.containers[0]?.name || "")
    }
  }, [container, pod.containers])
  const query = useQuery({
    queryKey: [K8S_PODS_KEY, namespace, name, "logs", container],
    queryFn: () => getPodLogs(namespace, name, { container }),
    enabled: Boolean(container),
    refetchInterval: 5_000,
  })

  return (
    <div className="space-y-3">
      <div className="flex items-center gap-2">
        <Select value={container} onValueChange={setContainer}>
          <SelectTrigger className="w-64"><SelectValue placeholder="Container" /></SelectTrigger>
          <SelectContent>
            {pod.containers.map((item) => <SelectItem key={item.name} value={item.name}>{item.name}</SelectItem>)}
          </SelectContent>
        </Select>
        <Button size="sm" variant="outline" onClick={() => void query.refetch()} disabled={query.isFetching}>
          <RefreshCw className={query.isFetching ? "size-3.5 animate-spin" : "size-3.5"} />
          Refresh
        </Button>
      </div>
      <LogsViewer logs={query.data?.data.logs} loading={query.isLoading} />
    </div>
  )
}
