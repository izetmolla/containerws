import { useQuery, useQueryClient } from "@tanstack/react-query"
import { useOutletContext } from "react-router"

import {
  applyConfigMapYaml,
  getConfigMapYaml,
  K8S_CONFIGMAPS_KEY,
} from "../../_shared/api"
import { ResourceYamlEditor } from "../../_shared/resource-ui"
import type { ConfigMapOutletContext } from "./layout"

export default function ConfigMapYamlPage() {
  const { namespace, name, invalidate } =
    useOutletContext<ConfigMapOutletContext>()
  const queryClient = useQueryClient()
  const query = useQuery({
    queryKey: [K8S_CONFIGMAPS_KEY, namespace, name, "yaml"],
    queryFn: () => getConfigMapYaml(namespace, name),
  })

  return (
    <ResourceYamlEditor
      yaml={query.data?.data.yaml}
      loading={query.isLoading}
      onReload={() => void query.refetch()}
      onApply={async (yaml) => {
        const res = await applyConfigMapYaml(namespace, name, yaml)
        return { message: res.message, yaml: res.data?.yaml }
      }}
      onApplied={() => {
        invalidate()
        void queryClient.invalidateQueries({
          queryKey: [K8S_CONFIGMAPS_KEY, namespace, name, "yaml"],
        })
      }}
    />
  )
}
