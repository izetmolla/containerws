import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useOutletContext } from "react-router";

import {
  applyDaemonSetYaml,
  getDaemonSetYaml,
  K8S_DAEMONSETS_KEY,
} from "../../_shared/api";
import { ResourceYamlEditor } from "../../_shared/resource-ui";
import type { DaemonSetOutletContext } from "./layout";

export default function DaemonSetYamlPage() {
  const { namespace, name, invalidate } =
    useOutletContext<DaemonSetOutletContext>();
  const queryClient = useQueryClient();
  const query = useQuery({
    queryKey: [K8S_DAEMONSETS_KEY, namespace, name, "yaml"],
    queryFn: () => getDaemonSetYaml(namespace, name),
  });
  return (
    <ResourceYamlEditor
      yaml={query.data?.data.yaml}
      loading={query.isLoading}
      onReload={() => void query.refetch()}
      onApply={async (yaml) => {
        const res = await applyDaemonSetYaml(namespace, name, yaml);
        return { message: res.message, yaml: res.data?.yaml };
      }}
      onApplied={() => {
        invalidate();
        void queryClient.invalidateQueries({
          queryKey: [K8S_DAEMONSETS_KEY, namespace, name, "yaml"],
        });
      }}
    />
  );
}
