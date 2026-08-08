import { useState } from "react"
import { Link, useNavigate } from "react-router"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { createColumnHelper, type ColumnDef } from "@tanstack/react-table"
import {
  Download,
  Eye,
  Plus,
  RefreshCw,
  RotateCcw,
  Scaling,
  Trash2,
} from "lucide-react"
import { toast } from "sonner"

import ContentLoader from "@/components/content-loader"
import { Button } from "@/components/ui/button"
import { DataTable } from "@/components/datatable"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import {
  DropdownMenuItem,
  DropdownMenuSeparator,
} from "@/components/ui/dropdown-menu"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { asArray } from "@/lib/as-array"
import { toastRequestError } from "@/lib/network"

import {
  deleteDeployment,
  formatAge,
  K8S_DEPLOYMENTS_KEY,
  listDeployments,
  pullRestartDeployment,
  restartDeployment,
  scaleDeployment,
  type DeploymentRow,
} from "../_shared/api"
import {
  k8sInitialState,
  k8sNameFilter,
  k8sNamespaceFilter,
  k8sSortable,
  k8sTextFilter,
  useK8sNamespaceOptions,
} from "../_shared/client-table"
import {
  K8sBulkActionBar,
  k8sSelectableTableProps,
  runForEachSelected,
} from "../_shared/bulk-actions"
import { ClusterBanner } from "../_shared/cluster-banner"
import {
  useK8sNamespacedRows,
} from "../_shared/system-resources"
import { NamespaceSelector, useNamespaceFilter } from "../_shared/namespace-selector"
import { RowActionsMenu } from "../_shared/row-actions"

const columnHelper = createColumnHelper<DeploymentRow>()

export default function DeploymentsPage() {
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const [removeTarget, setRemoveTarget] = useState<DeploymentRow | null>(null)
  const [scaleTarget, setScaleTarget] = useState<DeploymentRow | null>(null)
  const [replicas, setReplicas] = useState("1")
  const ns = useNamespaceFilter()

  const listQuery = useQuery({
    queryKey: [K8S_DEPLOYMENTS_KEY, ns || "all"],
    queryFn: () => listDeployments(ns || undefined),
    refetchInterval: 10_000,
  })

  const invalidate = () =>
    queryClient.invalidateQueries({ queryKey: [K8S_DEPLOYMENTS_KEY] })

  const deleteMutation = useMutation({
    mutationFn: (row: DeploymentRow) => deleteDeployment(row.namespace, row.name),
    onSuccess: (res) => {
      toast.success(res.message || "Deployment deleted")
      setRemoveTarget(null)
      invalidate()
    },
    onError: (err) => toastRequestError(err, "Delete failed"),
  })

  const scaleMutation = useMutation({
    mutationFn: () => {
      if (!scaleTarget) throw new Error("No target")
      return scaleDeployment(
        scaleTarget.namespace,
        scaleTarget.name,
        Number(replicas) || 0
      )
    },
    onSuccess: (res) => {
      toast.success(res.message || "Scaled")
      setScaleTarget(null)
      invalidate()
    },
    onError: (err) => toastRequestError(err, "Scale failed"),
  })

  const restartMutation = useMutation({
    mutationFn: (row: DeploymentRow) =>
      restartDeployment(row.namespace, row.name),
    onSuccess: (res) => {
      toast.success(res.message || "Restarted")
      invalidate()
    },
    onError: (err) => toastRequestError(err, "Restart failed"),
  })

  const pullRestartMutation = useMutation({
    mutationFn: (row: DeploymentRow) =>
      pullRestartDeployment(row.namespace, row.name),
    onSuccess: (res) => {
      toast.success(res.message || "Pulling image and restarting")
      invalidate()
    },
    onError: (err) => toastRequestError(err, "Pull & restart failed"),
  })

  const rows = useK8sNamespacedRows(asArray(listQuery.data?.data))
  const nsOptions = useK8sNamespaceOptions(rows.map((r) => r.namespace))

  const columns = [
    columnHelper.accessor("name", {
      ...k8sNameFilter(),
      cell: ({ row, getValue }) => (
        <Link className="font-medium hover:underline" to={`/kubernetes/deployments/${encodeURIComponent(row.original.namespace)}/${encodeURIComponent(row.original.name)}`}>
          {getValue()}
        </Link>
      ),
    }),
    columnHelper.accessor("namespace", { ...k8sNamespaceFilter(nsOptions) }),
    columnHelper.accessor("ready", { ...k8sSortable("Ready") }),
    columnHelper.accessor("up_to_date", { ...k8sSortable("Up-to-date") }),
    columnHelper.accessor("available", { ...k8sSortable("Available") }),
    columnHelper.accessor("images", { ...k8sTextFilter("Images") }),
    columnHelper.accessor("created_at", {
      ...k8sSortable("Age"),
      cell: ({ getValue }) => formatAge(getValue()),
    }),
    columnHelper.display({
      id: "actions",
      cell: ({ row }) => {
        const item = row.original
        const href = `/kubernetes/deployments/${encodeURIComponent(item.namespace)}/${encodeURIComponent(item.name)}`
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
                setScaleTarget(item)
                setReplicas(String(item.replicas))
              }}
            >
              <Scaling />
              Scale…
            </DropdownMenuItem>
            <DropdownMenuItem onClick={() => restartMutation.mutate(item)}>
              <RotateCcw />
              Restart
            </DropdownMenuItem>
            <DropdownMenuItem
              disabled={pullRestartMutation.isPending}
              onClick={() => pullRestartMutation.mutate(item)}
            >
              <Download />
              Pull image & restart
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
        )
      },
    }),
  ] as ColumnDef<DeploymentRow, unknown>[]

  return (
    <ContentLoader
      title="Deployments"
      breadcrumb={[
        { label: "Kubernetes", to: "/kubernetes" },
        { label: "Deployments" },
      ]}
      isLoading={listQuery.isLoading}
      error={listQuery.error}
      rightComponent={
        <div className="flex flex-wrap items-center gap-2">
          <NamespaceSelector />
          <Button size="sm" variant="outline" onClick={() => invalidate()}>
            <RefreshCw className="size-3.5" />
          </Button>
          <Button
            size="sm"
            onClick={() => navigate("/kubernetes/deployments/create")}
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
                  tooltip: "Restart selected deployments",
                  run: (selected) =>
                    runForEachSelected(selected, (row) =>
                      restartDeployment(row.namespace, row.name),
                    ),
                },
                {
                  key: "pull-restart",
                  label: "Pull & restart",
                  icon: <Download />,
                  tooltip: "Pull latest images and restart",
                  run: (selected) =>
                    runForEachSelected(selected, (row) =>
                      pullRestartDeployment(row.namespace, row.name),
                    ),
                },
                {
                  key: "delete",
                  label: "Delete",
                  icon: <Trash2 />,
                  variant: "destructive",
                  confirm: "Delete {n} deployment(s)? This cannot be undone.",
                  run: (selected) =>
                    runForEachSelected(selected, (row) =>
                      deleteDeployment(row.namespace, row.name),
                    ),
                },
              ]}
            />
          )}
        />
      </div>

      <Dialog
        open={!!scaleTarget}
        onOpenChange={(o) => !o && setScaleTarget(null)}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Scale deployment</DialogTitle>
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
              onChange={(e) => setReplicas(e.target.value)}
            />
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setScaleTarget(null)}>
              Cancel
            </Button>
            <Button
              disabled={scaleMutation.isPending}
              onClick={() => scaleMutation.mutate()}
            >
              Scale
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog
        open={!!removeTarget}
        onOpenChange={(o) => !o && setRemoveTarget(null)}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Delete deployment?</DialogTitle>
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
              disabled={deleteMutation.isPending}
              onClick={() =>
                removeTarget && deleteMutation.mutate(removeTarget)
              }
            >
              Delete
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </ContentLoader>
  )
}
