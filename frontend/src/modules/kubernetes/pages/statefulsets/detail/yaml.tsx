import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useOutletContext } from "react-router";

import {
  applyStatefulSetYaml,
  getStatefulSetYaml,
  K8S_STATEFULSETS_KEY,
} from "../../_shared/api";
import { ResourceYamlEditor } from "../../_shared/resource-ui";
import type { StatefulSetOutletContext } from "./layout";

export default function StatefulSetYamlPage() {
  const { namespace, name, invalidate } =
    useOutletContext<StatefulSetOutletContext>();
  const queryClient = useQueryClient();
  const query = useQuery({
    queryKey: [K8S_STATEFULSETS_KEY, namespace, name, "yaml"],
    queryFn: () => getStatefulSetYaml(namespace, name),
  });
  return (
    <ResourceYamlEditor
      yaml={query.data?.data.yaml}
      loading={query.isLoading}
      onReload={() => void query.refetch()}
      onApply={async (yaml) => {
        const res = await applyStatefulSetYaml(namespace, name, yaml);
        return { message: res.message, yaml: res.data?.yaml };
      }}
      onApplied={() => {
        invalidate();
        void queryClient.invalidateQueries({
          queryKey: [K8S_STATEFULSETS_KEY, namespace, name, "yaml"],
        });
      }}
    />
  );
}
