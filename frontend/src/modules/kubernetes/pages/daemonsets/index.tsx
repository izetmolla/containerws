import { useState } from "react";
import { Link, useNavigate } from "react-router";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { createColumnHelper, type ColumnDef } from "@tanstack/react-table";
import { Eye, Plus, RefreshCw, RotateCcw, Trash2 } from "lucide-react";
import { toast } from "sonner";

import ContentLoader from "@/components/content-loader";
import { DataTable } from "@/components/datatable";
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
import { asArray } from "@/lib/as-array";
import { toastRequestError } from "@/lib/network";

import {
  deleteDaemonSet,
  formatAge,
  K8S_DAEMONSETS_KEY,
  listDaemonSets,
  restartDaemonSet,
  type DaemonSetRow,
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

const helper = createColumnHelper<DaemonSetRow>();

export default function DaemonSetsPage() {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const ns = useNamespaceFilter();
  const [removeTarget, setRemoveTarget] = useState<DaemonSetRow | null>(null);
  const query = useQuery({
    queryKey: [K8S_DAEMONSETS_KEY, ns || "all"],
    queryFn: () => listDaemonSets(ns || undefined),
    refetchInterval: 10_000,
  });
  const invalidate = () =>
    queryClient.invalidateQueries({ queryKey: [K8S_DAEMONSETS_KEY] });
  const remove = useMutation({
    mutationFn: (row: DaemonSetRow) => deleteDaemonSet(row.namespace, row.name),
    onSuccess: (res) => {
      toast.success(res.message || "DaemonSet deleted");
      setRemoveTarget(null);
      invalidate();
    },
    onError: (error) => toastRequestError(error, "Delete failed"),
  });
  const restart = useMutation({
    mutationFn: (row: DaemonSetRow) =>
      restartDaemonSet(row.namespace, row.name),
    onSuccess: (res) => {
      toast.success(res.message || "DaemonSet restarted");
      invalidate();
    },
    onError: (error) => toastRequestError(error, "Restart failed"),
  });
  const rows = useK8sNamespacedRows(asArray(query.data?.data))
  const columns = [
    helper.accessor("name", {
      ...k8sNameFilter(),
      cell: ({ row, getValue }) => (
        <Link
          className="font-medium hover:underline"
          to={`/kubernetes/daemonsets/${encodeURIComponent(row.original.namespace)}/${encodeURIComponent(row.original.name)}`}
        >
          {getValue()}
        </Link>
      ),
    }),
    helper.accessor("namespace", {
      ...k8sNamespaceFilter(
        useK8sNamespaceOptions(rows.map((row) => row.namespace)),
      ),
    }),
    helper.accessor("desired", { ...k8sSortable("Desired") }),
    helper.accessor("current", { ...k8sSortable("Current") }),
    helper.accessor("ready", { ...k8sSortable("Ready") }),
    helper.accessor("up_to_date", { ...k8sSortable("Up-to-date") }),
    helper.accessor("available", { ...k8sSortable("Available") }),
    helper.accessor("images", { ...k8sTextFilter("Images") }),
    helper.accessor("created_at", {
      ...k8sSortable("Age"),
      cell: ({ getValue }) => formatAge(getValue()),
    }),
    helper.display({
      id: "actions",
      cell: ({ row }) => {
        const item = row.original;
        const href = `/kubernetes/daemonsets/${encodeURIComponent(item.namespace)}/${encodeURIComponent(item.name)}`;
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
              disabled={restart.isPending}
              onClick={() => restart.mutate(item)}
            >
              <RotateCcw />
              Restart
            </DropdownMenuItem>
            <DropdownMenuSeparator />
            <DropdownMenuItem
              variant="destructive"
              onClick={() => setRemoveTarget(item)}
            >
              <Trash2 />
              Delete
            </DropdownMenuItem>
          </RowActionsMenu>
        );
      },
    }),
  ] as ColumnDef<DaemonSetRow, unknown>[];
  return (
    <ContentLoader
      title="DaemonSets"
      breadcrumb={[
        { label: "Kubernetes", to: "/kubernetes" },
        { label: "DaemonSets" },
      ]}
      isLoading={query.isLoading}
      error={query.error}
      rightComponent={
        <div className="flex flex-wrap items-center gap-2">
          <NamespaceSelector />
          <Button size="sm" variant="outline" onClick={() => invalidate()}>
            <RefreshCw className="size-3.5" />
          </Button>
          <Button
            size="sm"
            onClick={() => navigate("/kubernetes/daemonsets/create")}
          >
            <Plus className="size-3.5" />
            Create
          </Button>
        </div>
      }
    >
      <div className="space-y-4">
        <ClusterBanner />
        <DataTable
          columns={columns}
          source={{ type: "client", data: rows }}
          getRowId={(row) => `${row.namespace}/${row.name}`}
          {...k8sSelectableTableProps}
          initialState={k8sInitialState(20)}
          actionBar={(table) => (
            <K8sBulkActionBar
              table={table}
              onDone={invalidate}
              actions={[
                {
                  key: "restart",
                  label: "Restart",
                  icon: <RotateCcw />,
                  tooltip: "Restart selected DaemonSets",
                  run: (selected) =>
                    runForEachSelected(selected, (row) =>
                      restartDaemonSet(row.namespace, row.name),
                    ),
                },
                {
                  key: "delete",
                  label: "Delete",
                  icon: <Trash2 />,
                  variant: "destructive",
                  confirm: "Delete {n} DaemonSet(s)? This cannot be undone.",
                  run: (selected) =>
                    runForEachSelected(selected, (row) =>
                      deleteDaemonSet(row.namespace, row.name),
                    ),
                },
              ]}
            />
          )}
        />
      </div>
      <Dialog
        open={!!removeTarget}
        onOpenChange={(open) => !open && setRemoveTarget(null)}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Delete DaemonSet?</DialogTitle>
            <DialogDescription>
              Delete {removeTarget?.namespace}/{removeTarget?.name}?
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
