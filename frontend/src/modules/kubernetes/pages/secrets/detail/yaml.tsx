import { useQuery, useQueryClient } from "@tanstack/react-query"
import { useOutletContext } from "react-router"

import {
  applySecretYaml,
  getSecretYaml,
  K8S_SECRETS_KEY,
} from "../../_shared/api"
import { ResourceYamlEditor } from "../../_shared/resource-ui"
import type { SecretOutletContext } from "./layout"

export default function SecretYamlPage() {
  const { namespace, name, invalidate } =
    useOutletContext<SecretOutletContext>()
  const queryClient = useQueryClient()
  const query = useQuery({
    queryKey: [K8S_SECRETS_KEY, namespace, name, "yaml"],
    queryFn: () => getSecretYaml(namespace, name),
  })

  return (
    <ResourceYamlEditor
      yaml={query.data?.data.yaml}
      loading={query.isLoading}
      onReload={() => void query.refetch()}
      onApply={async (yaml) => {
        const res = await applySecretYaml(namespace, name, yaml)
        return { message: res.message, yaml: res.data?.yaml }
      }}
      onApplied={() => {
        invalidate()
        void queryClient.invalidateQueries({
          queryKey: [K8S_SECRETS_KEY, namespace, name, "yaml"],
        })
      }}
    />
  )
}
