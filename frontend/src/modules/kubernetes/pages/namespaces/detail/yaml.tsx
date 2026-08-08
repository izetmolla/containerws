import { useQuery, useQueryClient } from "@tanstack/react-query"
import { useOutletContext } from "react-router"

import {
  applyNamespaceYaml,
  getNamespaceYaml,
  K8S_NAMESPACES_KEY,
} from "../../_shared/api"
import { ResourceYamlEditor } from "../../_shared/resource-ui"
import type { NamespaceOutletContext } from "./layout"

export default function NamespaceYamlPage() {
  const { name, invalidate } = useOutletContext<NamespaceOutletContext>()
  const queryClient = useQueryClient()
  const query = useQuery({
    queryKey: [K8S_NAMESPACES_KEY, name, "yaml"],
    queryFn: () => getNamespaceYaml(name),
  })

  return (
    <ResourceYamlEditor
      yaml={query.data?.data.yaml}
      loading={query.isLoading}
      onReload={() => void query.refetch()}
      onApply={async (yaml) => {
        const res = await applyNamespaceYaml(name, yaml)
        return { message: res.message, yaml: res.data?.yaml }
      }}
      onApplied={() => {
        invalidate()
        void queryClient.invalidateQueries({
          queryKey: [K8S_NAMESPACES_KEY, name, "yaml"],
        })
      }}
    />
  )
}
