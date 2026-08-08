import { useQuery, useQueryClient } from "@tanstack/react-query"
import { RefreshCw } from "lucide-react"
import { Outlet, useParams } from "react-router"

import ContentLoader from "@/components/content-loader"
import { Button } from "@/components/ui/button"

import {
  getNamespace,
  K8S_NAMESPACES_KEY,
  type NamespaceDetail,
} from "../../_shared/api"
import { ClusterBanner } from "../../_shared/cluster-banner"
import { ResourceSubNav } from "../../_shared/resource-ui"

export type NamespaceOutletContext = {
  name: string
  namespace: NamespaceDetail
  invalidate: () => void
}

export default function NamespaceDetailLayout() {
  const { name: rawName } = useParams()
  const name = decodeURIComponent(rawName || "")
  const queryClient = useQueryClient()
  const query = useQuery({
    queryKey: [K8S_NAMESPACES_KEY, name],
    queryFn: () => getNamespace(name),
    enabled: Boolean(name),
    refetchInterval: 8_000,
  })
  const invalidate = () => {
    void queryClient.invalidateQueries({ queryKey: [K8S_NAMESPACES_KEY] })
  }
  const base = `/kubernetes/namespaces/${encodeURIComponent(name)}`

  return (
    <ContentLoader
      title={name}
      breadcrumb={[
        { label: "Kubernetes", to: "/kubernetes" },
        { label: "Namespaces", to: "/kubernetes/namespaces" },
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
            { to: "yaml", label: "YAML" },
          ]}
        />
        {query.data?.data ? (
          <Outlet
            context={{
              name,
              namespace: query.data.data,
              invalidate,
            } satisfies NamespaceOutletContext}
          />
        ) : null}
      </div>
    </ContentLoader>
  )
}
