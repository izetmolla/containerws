import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useOutletContext } from "react-router";

import { applyJobYaml, getJobYaml, K8S_JOBS_KEY } from "../../_shared/api";
import { ResourceYamlEditor } from "../../_shared/resource-ui";
import type { JobOutletContext } from "./layout";

export default function JobYamlPage() {
  const { namespace, name, invalidate } = useOutletContext<JobOutletContext>();
  const queryClient = useQueryClient();
  const query = useQuery({
    queryKey: [K8S_JOBS_KEY, namespace, name, "yaml"],
    queryFn: () => getJobYaml(namespace, name),
  });
  return (
    <ResourceYamlEditor
      yaml={query.data?.data.yaml}
      loading={query.isLoading}
      onReload={() => void query.refetch()}
      onApply={async (yaml) => {
        const res = await applyJobYaml(namespace, name, yaml);
        return { message: res.message, yaml: res.data?.yaml };
      }}
      onApplied={() => {
        invalidate();
        void queryClient.invalidateQueries({
          queryKey: [K8S_JOBS_KEY, namespace, name, "yaml"],
        });
      }}
    />
  );
}
