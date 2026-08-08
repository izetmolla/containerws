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
import { asArray } from "@/lib/as-array";
import { toastRequestError } from "@/lib/network";

import {
  deletePvc,
  formatAge,
  K8S_PVCS_KEY,
  listPvcs,
  type PvcRow,
} from "../_shared/api";
import {
  K8sBulkActionBar,
  k8sSelectableTableProps,
  runForEachSelected,
} from "../_shared/bulk-actions";
import {
  k8sInitialState,
  k8sMultiSelectFilter,
  k8sNameFilter,
  k8sNamespaceFilter,
  k8sSortable,
  k8sTextFilter,
  optionsFromValues,
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

const helper = createColumnHelper<PvcRow>();

export default function PersistentVolumeClaimsPage() {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const ns = useNamespaceFilter();
  const [removeTarget, setRemoveTarget] = useState<PvcRow | null>(null);
  const query = useQuery({
    queryKey: [K8S_PVCS_KEY, ns || "all"],
    queryFn: () => listPvcs(ns || undefined),
    refetchInterval: 15_000,
  });
  const invalidate = () =>
    queryClient.invalidateQueries({ queryKey: [K8S_PVCS_KEY] });
  const remove = useMutation({
    mutationFn: (row: PvcRow) => deletePvc(row.namespace, row.name),
    onSuccess: (res) => {
      toast.success(res.message || "PersistentVolumeClaim deleted");
      setRemoveTarget(null);
      invalidate();
    },
    onError: (error) => toastRequestError(error, "Delete failed"),
  });
  const rows = useK8sNamespacedRows(asArray(query.data?.data))
  const columns = [
    helper.accessor("name", {
      ...k8sNameFilter(),
      cell: ({ row, getValue }) => (
        <Link
          className="font-medium hover:underline"
          to={`/kubernetes/persistentvolumeclaims/${encodeURIComponent(row.original.namespace)}/${encodeURIComponent(row.original.name)}`}
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
    helper.accessor("status", {
      ...k8sMultiSelectFilter(
        "Status",
        optionsFromValues(rows.map((row) => row.status)),
      ),
      cell: ({ getValue }) => (
        <Badge variant={getValue() === "Bound" ? "default" : "secondary"}>
          {getValue()}
        </Badge>
      ),
    }),
    helper.accessor("capacity", { ...k8sSortable("Capacity") }),
    helper.accessor("access_modes", { ...k8sTextFilter("Access modes") }),
    helper.accessor("storage_class", { ...k8sTextFilter("Storage class") }),
    helper.accessor("volume", { ...k8sTextFilter("Volume") }),
    helper.accessor("created_at", {
      ...k8sSortable("Age"),
      cell: ({ getValue }) => formatAge(getValue()),
    }),
    helper.display({
      id: "actions",
      cell: ({ row }) => {
        const item = row.original;
        const href = `/kubernetes/persistentvolumeclaims/${encodeURIComponent(item.namespace)}/${encodeURIComponent(item.name)}`;
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
              onClick={() => setRemoveTarget(item)}
            >
              <Trash2 />
              Delete
            </DropdownMenuItem>
          </RowActionsMenu>
        );
      },
    }),
  ] as ColumnDef<PvcRow, unknown>[];
  return (
    <ContentLoader
      title="PersistentVolumeClaims"
      breadcrumb={[
        { label: "Kubernetes", to: "/kubernetes" },
        { label: "PersistentVolumeClaims" },
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
            onClick={() =>
              navigate("/kubernetes/persistentvolumeclaims/create")
            }
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
                  key: "delete",
                  label: "Delete",
                  icon: <Trash2 />,
                  variant: "destructive",
                  confirm:
                    "Delete {n} PersistentVolumeClaim(s)? This cannot be undone.",
                  run: (selected) =>
                    runForEachSelected(selected, (row) =>
                      deletePvc(row.namespace, row.name),
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
            <DialogTitle>Delete PersistentVolumeClaim?</DialogTitle>
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
