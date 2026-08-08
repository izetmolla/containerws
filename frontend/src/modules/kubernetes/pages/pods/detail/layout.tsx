import { Outlet, useParams } from "react-router"
import { useQuery, useQueryClient } from "@tanstack/react-query"
import { RefreshCw } from "lucide-react"

import ContentLoader from "@/components/content-loader"
import { Button } from "@/components/ui/button"

import {
  getPod,
  K8S_PODS_KEY,
  type PodDetail,
} from "../../_shared/api"
import { ClusterBanner } from "../../_shared/cluster-banner"
import { ResourceSubNav } from "../../_shared/resource-ui"

export type PodOutletContext = {
  namespace: string
  name: string
  pod: PodDetail
  invalidate: () => void
}

const TABS = [
  { to: ".", end: true, label: "Overview" },
  { to: "logs", label: "Logs" },
  { to: "exec", label: "Exec" },
  { to: "events", label: "Events" },
  { to: "yaml", label: "YAML" },
]

export default function PodDetailLayout() {
  const { namespace = "", name = "" } = useParams()
  const queryClient = useQueryClient()
  const ns = decodeURIComponent(namespace)
  const podName = decodeURIComponent(name)
  const base = `/kubernetes/pods/${encodeURIComponent(ns)}/${encodeURIComponent(podName)}`

  const detailQuery = useQuery({
    queryKey: [K8S_PODS_KEY, ns, podName],
    queryFn: () => getPod(ns, podName),
    enabled: Boolean(ns && podName),
    refetchInterval: 8_000,
  })

  const invalidate = () => {
    void queryClient.invalidateQueries({ queryKey: [K8S_PODS_KEY, ns, podName] })
    void queryClient.invalidateQueries({ queryKey: [K8S_PODS_KEY] })
  }

  const pod = detailQuery.data?.data

  return (
    <ContentLoader
      title={podName}
      breadcrumb={[
        { label: "Kubernetes", to: "/kubernetes" },
        { label: "Pods", to: "/kubernetes/pods" },
        { label: podName },
      ]}
      isLoading={detailQuery.isLoading}
      error={detailQuery.error}
      rightComponent={
        <Button
          size="sm"
          variant="outline"
          onClick={() => void detailQuery.refetch()}
          disabled={detailQuery.isFetching}
        >
          <RefreshCw
            className={`size-3.5 ${detailQuery.isFetching ? "animate-spin" : ""}`}
          />
        </Button>
      }
    >
      <div className="space-y-4">
        <ClusterBanner />
        <ResourceSubNav base={base} tabs={TABS} />
        {pod ? (
          <Outlet
            context={
              {
                namespace: ns,
                name: podName,
                pod,
                invalidate,
              } satisfies PodOutletContext
            }
          />
        ) : null}
      </div>
    </ContentLoader>
  )
}
