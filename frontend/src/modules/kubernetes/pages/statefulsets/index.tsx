import { useState } from "react";
import { Link, useNavigate } from "react-router";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { createColumnHelper, type ColumnDef } from "@tanstack/react-table";
import { Eye, Plus, RefreshCw, RotateCcw, Scaling, Trash2 } from "lucide-react";
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
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { asArray } from "@/lib/as-array";
import { toastRequestError } from "@/lib/network";

import {
  deleteStatefulSet,
  formatAge,
  K8S_STATEFULSETS_KEY,
  listStatefulSets,
  restartStatefulSet,
  scaleStatefulSet,
  type StatefulSetRow,
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

const helper = createColumnHelper<StatefulSetRow>();

export default function StatefulSetsPage() {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const ns = useNamespaceFilter();
  const [removeTarget, setRemoveTarget] = useState<StatefulSetRow | null>(null);
  const [scaleTarget, setScaleTarget] = useState<StatefulSetRow | null>(null);
  const [replicas, setReplicas] = useState("1");
  const query = useQuery({
    queryKey: [K8S_STATEFULSETS_KEY, ns || "all"],
    queryFn: () => listStatefulSets(ns || undefined),
    refetchInterval: 10_000,
  });
  const invalidate = () =>
    queryClient.invalidateQueries({ queryKey: [K8S_STATEFULSETS_KEY] });
  const remove = useMutation({
    mutationFn: (row: StatefulSetRow) =>
      deleteStatefulSet(row.namespace, row.name),
    onSuccess: (res) => {
      toast.success(res.message || "StatefulSet deleted");
      setRemoveTarget(null);
      invalidate();
    },
    onError: (error) => toastRequestError(error, "Delete failed"),
  });
  const scale = useMutation({
    mutationFn: () => {
      if (!scaleTarget) throw new Error("No target");
      return scaleStatefulSet(
        scaleTarget.namespace,
        scaleTarget.name,
        Math.max(0, Number(replicas) || 0),
      );
    },
    onSuccess: (res) => {
      toast.success(res.message || "StatefulSet scaled");
      setScaleTarget(null);
      invalidate();
    },
    onError: (error) => toastRequestError(error, "Scale failed"),
  });
  const restart = useMutation({
    mutationFn: (row: StatefulSetRow) =>
      restartStatefulSet(row.namespace, row.name),
    onSuccess: (res) => {
      toast.success(res.message || "StatefulSet restarted");
      invalidate();
    },
    onError: (error) => toastRequestError(error, "Restart failed"),
  });
  const rows = useK8sNamespacedRows(asArray(query.data?.data))
  const namespaceOptions = useK8sNamespaceOptions(
    rows.map((row) => row.namespace),
  );
  const columns = [
    helper.accessor("name", {
      ...k8sNameFilter(),
      cell: ({ row, getValue }) => (
        <Link
          className="font-medium hover:underline"
          to={`/kubernetes/statefulsets/${encodeURIComponent(row.original.namespace)}/${encodeURIComponent(row.original.name)}`}
        >
          {getValue()}
        </Link>
      ),
    }),
    helper.accessor("namespace", { ...k8sNamespaceFilter(namespaceOptions) }),
    helper.accessor("ready", { ...k8sSortable("Ready") }),
    helper.accessor("replicas", { ...k8sSortable("Replicas") }),
    helper.accessor("service_name", { ...k8sTextFilter("Service") }),
    helper.accessor("images", { ...k8sTextFilter("Images") }),
    helper.accessor("created_at", {
      ...k8sSortable("Age"),
      cell: ({ getValue }) => formatAge(getValue()),
    }),
    helper.display({
      id: "actions",
      cell: ({ row }) => {
        const item = row.original;
        const href = `/kubernetes/statefulsets/${encodeURIComponent(item.namespace)}/${encodeURIComponent(item.name)}`;
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
              onClick={() => {
                setScaleTarget(item);
                setReplicas(String(item.replicas));
              }}
            >
              <Scaling />
              Scale…
            </DropdownMenuItem>
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
  ] as ColumnDef<StatefulSetRow, unknown>[];

  return (
    <ContentLoader
      title="StatefulSets"
      breadcrumb={[
        { label: "Kubernetes", to: "/kubernetes" },
        { label: "StatefulSets" },
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
            onClick={() => navigate("/kubernetes/statefulsets/create")}
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
                  tooltip: "Restart selected StatefulSets",
                  run: (selected) =>
                    runForEachSelected(selected, (row) =>
                      restartStatefulSet(row.namespace, row.name),
                    ),
                },
                {
                  key: "delete",
                  label: "Delete",
                  icon: <Trash2 />,
                  variant: "destructive",
                  confirm: "Delete {n} StatefulSet(s)? This cannot be undone.",
                  run: (selected) =>
                    runForEachSelected(selected, (row) =>
                      deleteStatefulSet(row.namespace, row.name),
                    ),
                },
              ]}
            />
          )}
        />
      </div>
      <Dialog
        open={!!scaleTarget}
        onOpenChange={(open) => !open && setScaleTarget(null)}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Scale StatefulSet</DialogTitle>
            <DialogDescription>
              {scaleTarget?.namespace}/{scaleTarget?.name}
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-2">
            <Label htmlFor="replicas">Replicas</Label>
            <Input
              id="replicas"
              type="number"
              min={0}
              value={replicas}
              onChange={(event) => setReplicas(event.target.value)}
            />
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setScaleTarget(null)}>
              Cancel
            </Button>
            <Button disabled={scale.isPending} onClick={() => scale.mutate()}>
              Scale
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
      <Dialog
        open={!!removeTarget}
        onOpenChange={(open) => !open && setRemoveTarget(null)}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Delete StatefulSet?</DialogTitle>
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
