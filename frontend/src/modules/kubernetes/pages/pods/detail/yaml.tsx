import { useQuery, useQueryClient } from "@tanstack/react-query"
import { useOutletContext } from "react-router"

import {
  applyPodYaml,
  getPodYaml,
  K8S_PODS_KEY,
} from "../../_shared/api"
import { ResourceYamlEditor } from "../../_shared/resource-ui"
import type { PodOutletContext } from "./layout"

export default function PodYamlPage() {
  const { namespace, name, invalidate } = useOutletContext<PodOutletContext>()
  const queryClient = useQueryClient()
  const query = useQuery({
    queryKey: [K8S_PODS_KEY, namespace, name, "yaml"],
    queryFn: () => getPodYaml(namespace, name),
  })

  return (
    <ResourceYamlEditor
      yaml={query.data?.data.yaml}
      loading={query.isLoading}
      onReload={() => void query.refetch()}
      onApply={async (yaml) => {
        const res = await applyPodYaml(namespace, name, yaml)
        return { message: res.message, yaml: res.data?.yaml }
      }}
      onApplied={() => {
        invalidate()
        void queryClient.invalidateQueries({
          queryKey: [K8S_PODS_KEY, namespace, name, "yaml"],
        })
      }}
    />
  )
}
