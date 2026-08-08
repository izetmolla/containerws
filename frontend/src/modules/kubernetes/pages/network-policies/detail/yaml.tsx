import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useOutletContext } from "react-router";

import {
  applyNetworkPolicyYaml,
  getNetworkPolicyYaml,
  K8S_NETWORK_POLICIES_KEY,
} from "../../_shared/api";
import { ResourceYamlEditor } from "../../_shared/resource-ui";
import type { NetworkPolicyOutletContext } from "./layout";

export default function NetworkPolicyYamlPage() {
  const { namespace, name, invalidate } =
    useOutletContext<NetworkPolicyOutletContext>();
  const queryClient = useQueryClient();
  const query = useQuery({
    queryKey: [K8S_NETWORK_POLICIES_KEY, namespace, name, "yaml"],
    queryFn: () => getNetworkPolicyYaml(namespace, name),
  });
  return (
    <ResourceYamlEditor
      yaml={query.data?.data.yaml}
      loading={query.isLoading}
      onReload={() => void query.refetch()}
      onApply={async (yaml) => {
        const res = await applyNetworkPolicyYaml(namespace, name, yaml);
        return { message: res.message, yaml: res.data?.yaml };
      }}
      onApplied={() => {
        invalidate();
        void queryClient.invalidateQueries({
          queryKey: [K8S_NETWORK_POLICIES_KEY, namespace, name, "yaml"],
        });
      }}
    />
  );
}
