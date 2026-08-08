import { useState } from "react";
import { Link, useNavigate } from "react-router";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { createColumnHelper, type ColumnDef } from "@tanstack/react-table";
import { Eye, Plus, RefreshCw, Trash2 } from "lucide-react";
import { toast } from "sonner";

import ContentLoader from "@/components/content-loader";
import { DataTable } from "@/components/datatable";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  DropdownMenuItem,
  DropdownMenuSeparator,
} from "@/components/ui/dropdown-menu";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { asArray } from "@/lib/as-array";
import { toastRequestError } from "@/lib/network";

import {
  deleteCronJob,
  deleteJob,
  formatAge,
  K8S_CRONJOBS_KEY,
  K8S_JOBS_KEY,
  listCronJobs,
  listJobs,
  type CronJobRow,
  type JobRow,
} from "../_shared/api";
import {
  K8sBulkActionBar,
  k8sSelectableTableProps,
  runForEachSelected,
} from "../_shared/bulk-actions";
import {
  k8sInitialState,
  k8sNameFilter,
  k8sNamespaceFilter,
  k8sSortable,
  k8sTextFilter,
  useK8sNamespaceOptions,
} from "../_shared/client-table";
import { ClusterBanner } from "../_shared/cluster-banner"
import {
  useK8sNamespacedRows,
} from "../_shared/system-resources";
import {
  NamespaceSelector,
  useNamespaceFilter,
} from "../_shared/namespace-selector";
import { RowActionsMenu } from "../_shared/row-actions";

const jobHelper = createColumnHelper<JobRow>();
const cronHelper = createColumnHelper<CronJobRow>();
type RemoveTarget =
  { kind: "job"; row: JobRow } | { kind: "cronjob"; row: CronJobRow };

export default function JobsPage() {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const ns = useNamespaceFilter();
  const [removeTarget, setRemoveTarget] = useState<RemoveTarget | null>(null);
  const jobsQuery = useQuery({
    queryKey: [K8S_JOBS_KEY, ns || "all"],
    queryFn: () => listJobs(ns || undefined),
    refetchInterval: 10_000,
  });
  const cronJobsQuery = useQuery({
    queryKey: [K8S_CRONJOBS_KEY, ns || "all"],
    queryFn: () => listCronJobs(ns || undefined),
    refetchInterval: 15_000,
  });
  const invalidate = () => {
    void queryClient.invalidateQueries({ queryKey: [K8S_JOBS_KEY] });
    void queryClient.invalidateQueries({ queryKey: [K8S_CRONJOBS_KEY] });
  };
  const remove = useMutation({
    mutationFn: (target: RemoveTarget) =>
      target.kind === "job"
        ? deleteJob(target.row.namespace, target.row.name)
        : deleteCronJob(target.row.namespace, target.row.name),
    onSuccess: (res) => {
      toast.success(res.message || "Resource deleted");
      setRemoveTarget(null);
      invalidate();
    },
    onError: (error) => toastRequestError(error, "Delete failed"),
  });
  const jobs = useK8sNamespacedRows(asArray(jobsQuery.data?.data));
  const cronJobs = useK8sNamespacedRows(asArray(cronJobsQuery.data?.data));
  const namespaceOptions = useK8sNamespaceOptions([
    ...jobs.map((row) => row.namespace),
    ...cronJobs.map((row) => row.namespace),
  ]);
  const jobColumns = [
    jobHelper.accessor("name", {
      ...k8sNameFilter(),
      cell: ({ row, getValue }) => (
        <Link
          className="font-medium hover:underline"
          to={`/kubernetes/jobs/${encodeURIComponent(row.original.namespace)}/${encodeURIComponent(row.original.name)}`}
        >
          {getValue()}
        </Link>
      ),
    }),
    jobHelper.accessor("namespace", {
      ...k8sNamespaceFilter(namespaceOptions),
    }),
    jobHelper.accessor("completions", { ...k8sSortable("Completions") }),
    jobHelper.accessor("succeeded", { ...k8sSortable("Succeeded") }),
    jobHelper.accessor("failed", { ...k8sSortable("Failed") }),
    jobHelper.accessor("active", { ...k8sSortable("Active") }),
    jobHelper.accessor("duration", { ...k8sSortable("Duration") }),
    jobHelper.accessor("images", { ...k8sTextFilter("Images") }),
    jobHelper.accessor("created_at", {
      ...k8sSortable("Age"),
      cell: ({ getValue }) => formatAge(getValue()),
    }),
    jobHelper.display({
      id: "actions",
      cell: ({ row }) => {
        const item = row.original;
        const href = `/kubernetes/jobs/${encodeURIComponent(item.namespace)}/${encodeURIComponent(item.name)}`;
        return (
          <RowActionsMenu label={`Actions for ${item.name}`}>
            <DropdownMenuItem asChild>
              <Link to={href}>
                <Eye />
                View details
              </Link>
            </DropdownMenuItem>
            <DropdownMenuSeparator />
            <DropdownMenuItem
              variant="destructive"
              onClick={() => setRemoveTarget({ kind: "job", row: item })}
            >
              <Trash2 />
              Delete
            </DropdownMenuItem>
          </RowActionsMenu>
        );
      },
    }),
  ] as ColumnDef<JobRow, unknown>[];
  const cronColumns = [
    cronHelper.accessor("name", {
      ...k8sNameFilter(),
      cell: ({ row, getValue }) => (
        <Link
          className="font-medium hover:underline"
          to={`/kubernetes/jobs/cron/${encodeURIComponent(row.original.namespace)}/${encodeURIComponent(row.original.name)}`}
        >
          {getValue()}
        </Link>
      ),
    }),
    cronHelper.accessor("namespace", {
      ...k8sNamespaceFilter(namespaceOptions),
    }),
    cronHelper.accessor("schedule", { ...k8sTextFilter("Schedule") }),
    cronHelper.accessor("suspend", {
      ...k8sSortable("Suspend"),
      cell: ({ getValue }) => (
        <Badge variant={getValue() ? "secondary" : "outline"}>
          {getValue() ? "Yes" : "No"}
        </Badge>
      ),
    }),
    cronHelper.accessor("active", { ...k8sSortable("Active") }),
    cronHelper.accessor("last_schedule", {
      ...k8sSortable("Last schedule"),
      cell: ({ getValue }) => (getValue() ? formatAge(getValue()!) : "—"),
    }),
    cronHelper.accessor("images", { ...k8sTextFilter("Images") }),
    cronHelper.accessor("created_at", {
      ...k8sSortable("Age"),
      cell: ({ getValue }) => formatAge(getValue()),
    }),
    cronHelper.display({
      id: "actions",
      cell: ({ row }) => {
        const item = row.original;
        const href = `/kubernetes/jobs/cron/${encodeURIComponent(item.namespace)}/${encodeURIComponent(item.name)}`;
        return (
          <RowActionsMenu label={`Actions for ${item.name}`}>
            <DropdownMenuItem asChild>
              <Link to={href}>
                <Eye />
                View details
              </Link>
            </DropdownMenuItem>
            <DropdownMenuSeparator />
            <DropdownMenuItem
              variant="destructive"
              onClick={() => setRemoveTarget({ kind: "cronjob", row: item })}
            >
              <Trash2 />
              Delete
            </DropdownMenuItem>
          </RowActionsMenu>
        );
      },
    }),
  ] as ColumnDef<CronJobRow, unknown>[];

  return (
    <ContentLoader
      title="Jobs"
      breadcrumb={[
        { label: "Kubernetes", to: "/kubernetes" },
        { label: "Jobs" },
      ]}
      isLoading={jobsQuery.isLoading || cronJobsQuery.isLoading}
      error={jobsQuery.error || cronJobsQuery.error}
      rightComponent={
        <div className="flex flex-wrap items-center gap-2">
          <NamespaceSelector />
          <Button size="sm" variant="outline" onClick={invalidate}>
            <RefreshCw className="size-3.5" />
          </Button>
          <Button size="sm" onClick={() => navigate("/kubernetes/jobs/create")}>
            <Plus className="size-3.5" />
            Create
          </Button>
        </div>
      }
    >
      <div className="space-y-4">
        <ClusterBanner />
        <Tabs defaultValue="jobs">
          <TabsList>
            <TabsTrigger value="jobs">Jobs</TabsTrigger>
            <TabsTrigger value="cronjobs">CronJobs</TabsTrigger>
          </TabsList>
          <TabsContent value="jobs">
            <DataTable
              columns={jobColumns}
              source={{ type: "client", data: jobs }}
              getRowId={(row) => `${row.namespace}/${row.name}`}
              {...k8sSelectableTableProps}
              initialState={k8sInitialState(20)}
              actionBar={(table) => (
                <K8sBulkActionBar
                  table={table}
                  onDone={invalidate}
                  actions={[
                    {
                      key: "delete",
                      label: "Delete",
                      icon: <Trash2 />,
                      variant: "destructive",
                      confirm: "Delete {n} Job(s)? This cannot be undone.",
                      run: (selected) =>
                        runForEachSelected(selected, (row) =>
                          deleteJob(row.namespace, row.name),
                        ),
                    },
                  ]}
                />
              )}
            />
          </TabsContent>
          <TabsContent value="cronjobs">
            <DataTable
              columns={cronColumns}
              source={{ type: "client", data: cronJobs }}
              getRowId={(row) => `${row.namespace}/${row.name}`}
              {...k8sSelectableTableProps}
              initialState={k8sInitialState(20)}
              actionBar={(table) => (
                <K8sBulkActionBar
                  table={table}
                  onDone={invalidate}
                  actions={[
                    {
                      key: "delete",
                      label: "Delete",
                      icon: <Trash2 />,
                      variant: "destructive",
                      confirm: "Delete {n} CronJob(s)? This cannot be undone.",
                      run: (selected) =>
                        runForEachSelected(selected, (row) =>
                          deleteCronJob(row.namespace, row.name),
                        ),
                    },
                  ]}
                />
              )}
            />
          </TabsContent>
        </Tabs>
      </div>
      <Dialog
        open={!!removeTarget}
        onOpenChange={(open) => !open && setRemoveTarget(null)}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>
              Delete {removeTarget?.kind === "cronjob" ? "CronJob" : "Job"}?
            </DialogTitle>
            <DialogDescription>
              Delete {removeTarget?.row.namespace}/{removeTarget?.row.name}?
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setRemoveTarget(null)}>
              Cancel
            </Button>
            <Button
              variant="destructive"
              disabled={remove.isPending}
              onClick={() => removeTarget && remove.mutate(removeTarget)}
            >
              Delete
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </ContentLoader>
  );
}
