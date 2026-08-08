import { useQuery, useQueryClient } from "@tanstack/react-query";
import { RefreshCw } from "lucide-react";
import { Outlet, useParams } from "react-router";

import ContentLoader from "@/components/content-loader";
import { Button } from "@/components/ui/button";

import {
  getCronJob,
  K8S_CRONJOBS_KEY,
  type CronJobDetail,
} from "../../_shared/api";
import { ClusterBanner } from "../../_shared/cluster-banner";
import { ResourceSubNav } from "../../_shared/resource-ui";

export type CronJobOutletContext = {
  namespace: string;
  name: string;
  cronJob: CronJobDetail;
  invalidate: () => void;
};

export default function CronJobDetailLayout() {
  const params = useParams();
  const namespace = decodeURIComponent(params.namespace || "");
  const name = decodeURIComponent(params.name || "");
  const queryClient = useQueryClient();
  const query = useQuery({
    queryKey: [K8S_CRONJOBS_KEY, namespace, name],
    queryFn: () => getCronJob(namespace, name),
    enabled: Boolean(namespace && name),
    refetchInterval: 10_000,
  });
  const invalidate = () => {
    void queryClient.invalidateQueries({ queryKey: [K8S_CRONJOBS_KEY] });
  };
  const base = `/kubernetes/jobs/cron/${encodeURIComponent(namespace)}/${encodeURIComponent(name)}`;
  return (
    <ContentLoader
      title={name}
      breadcrumb={[
        { label: "Kubernetes", to: "/kubernetes" },
        { label: "Jobs", to: "/kubernetes/jobs" },
        { label: "CronJobs" },
        { label: name },
      ]}
      isLoading={query.isLoading}
      error={query.error}
      rightComponent={
        <Button
          size="sm"
          variant="outline"
          onClick={() => void query.refetch()}
        >
          <RefreshCw
            className={query.isFetching ? "size-3.5 animate-spin" : "size-3.5"}
          />
        </Button>
      }
    >
      <div className="space-y-4">
        <ClusterBanner />
        <ResourceSubNav
          base={base}
          tabs={[
            { to: ".", end: true, label: "Overview" },
            { to: "yaml", label: "YAML" },
          ]}
        />
        {query.data?.data ? (
          <Outlet
            context={
              {
                namespace,
                name,
                cronJob: query.data.data,
                invalidate,
              } satisfies CronJobOutletContext
            }
          />
        ) : null}
      </div>
    </ContentLoader>
  );
}
