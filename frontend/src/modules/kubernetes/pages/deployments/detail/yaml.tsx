import { useQuery, useQueryClient } from "@tanstack/react-query"
import { useOutletContext } from "react-router"

import {
  applyDeploymentYaml,
  getDeploymentYaml,
  K8S_DEPLOYMENTS_KEY,
} from "../../_shared/api"
import { ResourceYamlEditor } from "../../_shared/resource-ui"
import type { DeploymentOutletContext } from "./layout"

export default function DeploymentYamlPage() {
  const { namespace, name, invalidate } =
    useOutletContext<DeploymentOutletContext>()
  const queryClient = useQueryClient()
  const query = useQuery({
    queryKey: [K8S_DEPLOYMENTS_KEY, namespace, name, "yaml"],
    queryFn: () => getDeploymentYaml(namespace, name),
  })

  return (
    <ResourceYamlEditor
      yaml={query.data?.data.yaml}
      loading={query.isLoading}
      onReload={() => void query.refetch()}
      onApply={async (yaml) => {
        const res = await applyDeploymentYaml(namespace, name, yaml)
        return { message: res.message, yaml: res.data?.yaml }
      }}
      onApplied={() => {
        invalidate()
        void queryClient.invalidateQueries({
          queryKey: [K8S_DEPLOYMENTS_KEY, namespace, name, "yaml"],
        })
      }}
    />
  )
}
