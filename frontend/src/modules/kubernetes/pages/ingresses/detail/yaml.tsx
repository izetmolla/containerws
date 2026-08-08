import { useQuery, useQueryClient } from "@tanstack/react-query"
import { useOutletContext } from "react-router"

import {
  applyIngressYaml,
  getIngressYaml,
  K8S_INGRESSES_KEY,
} from "../../_shared/api"
import { ResourceYamlEditor } from "../../_shared/resource-ui"
import type { IngressOutletContext } from "./layout"

export default function IngressYamlPage() {
  const { namespace, name, invalidate } =
    useOutletContext<IngressOutletContext>()
  const queryClient = useQueryClient()
  const query = useQuery({
    queryKey: [K8S_INGRESSES_KEY, namespace, name, "yaml"],
    queryFn: () => getIngressYaml(namespace, name),
  })

  return (
    <ResourceYamlEditor
      yaml={query.data?.data.yaml}
      loading={query.isLoading}
      onReload={() => void query.refetch()}
      onApply={async (yaml) => {
        const res = await applyIngressYaml(namespace, name, yaml)
        return { message: res.message, yaml: res.data?.yaml }
      }}
      onApplied={() => {
        invalidate()
        void queryClient.invalidateQueries({
          queryKey: [K8S_INGRESSES_KEY, namespace, name, "yaml"],
        })
      }}
    />
  )
}
