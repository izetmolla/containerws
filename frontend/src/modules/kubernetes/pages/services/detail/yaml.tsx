import { useQuery, useQueryClient } from "@tanstack/react-query"
import { useOutletContext } from "react-router"

import {
  applyServiceYaml,
  getServiceYaml,
  K8S_SERVICES_KEY,
} from "../../_shared/api"
import { ResourceYamlEditor } from "../../_shared/resource-ui"
import type { ServiceOutletContext } from "./layout"

export default function ServiceYamlPage() {
  const { namespace, name, invalidate } =
    useOutletContext<ServiceOutletContext>()
  const queryClient = useQueryClient()
  const query = useQuery({
    queryKey: [K8S_SERVICES_KEY, namespace, name, "yaml"],
    queryFn: () => getServiceYaml(namespace, name),
  })

  return (
    <ResourceYamlEditor
      yaml={query.data?.data.yaml}
      loading={query.isLoading}
      onReload={() => void query.refetch()}
      onApply={async (yaml) => {
        const res = await applyServiceYaml(namespace, name, yaml)
        return { message: res.message, yaml: res.data?.yaml }
      }}
      onApplied={() => {
        invalidate()
        void queryClient.invalidateQueries({
          queryKey: [K8S_SERVICES_KEY, namespace, name, "yaml"],
        })
      }}
    />
  )
}
