import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useOutletContext } from "react-router";

import { applyPvcYaml, getPvcYaml, K8S_PVCS_KEY } from "../../_shared/api";
import { ResourceYamlEditor } from "../../_shared/resource-ui";
import type { PvcOutletContext } from "./layout";

export default function PvcYamlPage() {
  const { namespace, name, invalidate } = useOutletContext<PvcOutletContext>();
  const queryClient = useQueryClient();
  const query = useQuery({
    queryKey: [K8S_PVCS_KEY, namespace, name, "yaml"],
    queryFn: () => getPvcYaml(namespace, name),
  });
  return (
    <ResourceYamlEditor
      yaml={query.data?.data.yaml}
      loading={query.isLoading}
      onReload={() => void query.refetch()}
      onApply={async (yaml) => {
        const res = await applyPvcYaml(namespace, name, yaml);
        return { message: res.message, yaml: res.data?.yaml };
      }}
      onApplied={() => {
        invalidate();
        void queryClient.invalidateQueries({
          queryKey: [K8S_PVCS_KEY, namespace, name, "yaml"],
        });
      }}
    />
  );
}
