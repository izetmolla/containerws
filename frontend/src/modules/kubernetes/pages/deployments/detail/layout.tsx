import { useQuery, useQueryClient } from "@tanstack/react-query"
import { RefreshCw } from "lucide-react"
import { Outlet, useParams } from "react-router"

import ContentLoader from "@/components/content-loader"
import { Button } from "@/components/ui/button"

import {
  getDeployment,
  K8S_DEPLOYMENTS_KEY,
  type DeploymentDetail,
} from "../../_shared/api"
import { ClusterBanner } from "../../_shared/cluster-banner"
import { ResourceSubNav } from "../../_shared/resource-ui"

export type DeploymentOutletContext = {
  namespace: string
  name: string
  deployment: DeploymentDetail
  invalidate: () => void
}

export default function DeploymentDetailLayout() {
  const params = useParams()
  const namespace = decodeURIComponent(params.namespace || "")
  const name = decodeURIComponent(params.name || "")
  const queryClient = useQueryClient()
  const query = useQuery({
    queryKey: [K8S_DEPLOYMENTS_KEY, namespace, name],
    queryFn: () => getDeployment(namespace, name),
    enabled: Boolean(namespace && name),
    refetchInterval: 8_000,
  })
  const invalidate = () => {
    void queryClient.invalidateQueries({ queryKey: [K8S_DEPLOYMENTS_KEY] })
  }
  const base = `/kubernetes/deployments/${encodeURIComponent(namespace)}/${encodeURIComponent(name)}`

  return (
    <ContentLoader
      title={name}
      breadcrumb={[
        { label: "Kubernetes", to: "/kubernetes" },
        { label: "Deployments", to: "/kubernetes/deployments" },
        { label: name },
      ]}
      isLoading={query.isLoading}
      error={query.error}
      rightComponent={
        <Button size="sm" variant="outline" onClick={() => void query.refetch()}>
          <RefreshCw className={query.isFetching ? "size-3.5 animate-spin" : "size-3.5"} />
        </Button>
      }
    >
      <div className="space-y-4">
        <ClusterBanner />
        <ResourceSubNav
          base={base}
          tabs={[
            { to: ".", end: true, label: "Overview" },
            { to: "pods", label: "Pods" },
            { to: "events", label: "Events" },
            { to: "yaml", label: "YAML" },
          ]}
        />
        {query.data?.data ? (
          <Outlet
            context={{
              namespace,
              name,
              deployment: query.data.data,
              invalidate,
            } satisfies DeploymentOutletContext}
          />
        ) : null}
      </div>
    </ContentLoader>
  )
}
