import { useState } from "react"
import { Link, useNavigate } from "react-router"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { createColumnHelper, type ColumnDef } from "@tanstack/react-table"
import {
  CheckCircle2,
  Copy,
  Download,
  Eye,
  Pencil,
  Plus,
  RefreshCw,
  Rocket,
  Trash2,
  Unplug,
} from "lucide-react"
import { toast } from "sonner"

import ContentLoader from "@/components/content-loader"
import { DataTable } from "@/components/datatable"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
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
import { asArray } from "@/lib/as-array"
import { toastRequestError } from "@/lib/network"

import {
  applySavedApplication,
  deleteApplication,
  duplicateApplication,
  formatAge,
  getApplication,
  K8S_APPLICATIONS_KEY,
  listApplications,
  removeApplicationFromCluster,
  type K8sApplicationRow,
} from "../_shared/api"
import {
  K8sBulkActionBar,
  k8sSelectableTableProps,
  runForEachSelected,
} from "../_shared/bulk-actions"
import {
  k8sInitialState,
  k8sNameFilter,
  k8sNamespaceFilter,
  k8sSortable,
  useK8sNamespaceOptions,
} from "../_shared/client-table"
import { ClusterBanner } from "../_shared/cluster-banner"
import {
  useK8sNamespacedRows,
} from "../_shared/system-resources"
import { RowActionsMenu } from "../_shared/row-actions"

const columnHelper = createColumnHelper<K8sApplicationRow>()

type ConfirmAction =
  | { kind: "delete-store"; row: K8sApplicationRow }
  | { kind: "remove-cluster"; row: K8sApplicationRow }

function StatusBadge({ row }: { row: K8sApplicationRow }) {
  const status = row.status || "unknown"
  const label =
    status === "healthy"
      ? "Healthy"
      : status === "partial"
        ? "Partial"
        : status === "missing"
          ? "Missing"
          : status === "empty"
            ? "Empty"
            : "Unknown"
  const variant =
    status === "healthy"
      ? "default"
      : status === "partial"
        ? "secondary"
        : status === "missing"
          ? "destructive"
          : "outline"
  const detail =
    row.resource_count > 0 &&
    (row.ready_count != null || row.missing_count != null)
      ? ` ${row.ready_count ?? 0}/${row.resource_count}`
      : ""
  return (
    <Badge variant={variant}>
      {label}
      {detail}
    </Badge>
  )
}

async function downloadYaml(row: K8sApplicationRow) {
  const res = await getApplication(row.id)
  const blob = new Blob([res.data.yaml || ""], {
    type: "application/x-yaml;charset=utf-8",
  })
  const url = URL.createObjectURL(blob)
  const a = document.createElement("a")
  a.href = url
  a.download = `${row.name || "application"}.yaml`
  a.click()
  URL.revokeObjectURL(url)
}

export default function ApplicationsListPage() {
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const [confirm, setConfirm] = useState<ConfirmAction | null>(null)

  const listQuery = useQuery({
    queryKey: [K8S_APPLICATIONS_KEY, "list"],
    queryFn: listApplications,
    refetchInterval: 15_000,
  })

  const invalidate = () =>
    void queryClient.invalidateQueries({ queryKey: [K8S_APPLICATIONS_KEY] })

  const deleteMutation = useMutation({
    mutationFn: (row: K8sApplicationRow) => deleteApplication(row.id),
    onSuccess: (res) => {
      toast.success(res.message || "Application deleted")
      setConfirm(null)
      invalidate()
    },
    onError: (err) => toastRequestError(err, "Delete failed"),
  })

  const removeMutation = useMutation({
    mutationFn: (row: K8sApplicationRow) =>
      removeApplicationFromCluster(row.id),
    onSuccess: (res) => {
      const s = res.data.summary
      if (s.failed > 0 && s.applied === 0) {
        toast.error(res.message || "Remove failed")
      } else if (s.failed > 0) {
        toast.warning(res.message || "Completed with errors")
      } else {
        toast.success(res.message || "Removed from cluster")
      }
      setConfirm(null)
      invalidate()
    },
    onError: (err) => toastRequestError(err, "Remove failed"),
  })

  const applyMutation = useMutation({
    mutationFn: (row: K8sApplicationRow) => applySavedApplication(row.id),
    onSuccess: (res) => {
      toast.success(res.message || "Applied")
      invalidate()
    },
    onError: (err) => toastRequestError(err, "Apply failed"),
  })

  const dryRunMutation = useMutation({
    mutationFn: (row: K8sApplicationRow) =>
      applySavedApplication(row.id, { dry_run: true }),
    onSuccess: (res) => {
      toast.success(res.message || "Dry-run OK")
      invalidate()
    },
    onError: (err) => toastRequestError(err, "Dry-run failed"),
  })

  const duplicateMutation = useMutation({
    mutationFn: (row: K8sApplicationRow) => duplicateApplication(row.id),
    onSuccess: (res) => {
      toast.success(res.message || "Duplicated")
      invalidate()
      navigate(
        `/kubernetes/applications/edit?id=${encodeURIComponent(res.data.id)}`,
      )
    },
    onError: (err) => toastRequestError(err, "Duplicate failed"),
  })

  const busy =
    applyMutation.isPending ||
    dryRunMutation.isPending ||
    duplicateMutation.isPending ||
    deleteMutation.isPending ||
    removeMutation.isPending

  const rows = useK8sNamespacedRows(asArray(listQuery.data?.data))
  const nsOptions = useK8sNamespaceOptions(rows.map((r) => r.namespace))

  const columns = [
    columnHelper.accessor("name", {
      ...k8sNameFilter(),
      cell: ({ row, getValue }) => (
        <Link
          className="font-medium hover:underline"
          to={`/kubernetes/applications/edit?id=${encodeURIComponent(row.original.id)}`}
        >
          {getValue()}
        </Link>
      ),
    }),
    columnHelper.accessor("namespace", {
      ...k8sNamespaceFilter(nsOptions),
      cell: ({ getValue }) => {
        const ns = getValue()
        if (!ns) return "—"
        return (
          <Link
            className="hover:underline"
            to={`/kubernetes/namespaces/${encodeURIComponent(ns)}`}
          >
            {ns}
          </Link>
        )
      },
    }),
    columnHelper.accessor("version", {
      ...k8sSortable("Version"),
      cell: ({ getValue }) => (
        <Badge variant="outline" className="font-mono">
          v{getValue() || 1}
        </Badge>
      ),
    }),
    columnHelper.accessor("status", {
      ...k8sSortable("Status"),
      cell: ({ row }) => <StatusBadge row={row.original} />,
    }),
    columnHelper.accessor("resource_count", {
      ...k8sSortable("Resources"),
    }),
    columnHelper.accessor("updated_at", {
      ...k8sSortable("Updated"),
      cell: ({ getValue }) => formatAge(getValue()),
    }),
    columnHelper.accessor("created_at", {
      ...k8sSortable("Created"),
      cell: ({ getValue }) => formatAge(getValue()),
    }),
    columnHelper.display({
      id: "actions",
      cell: ({ row }) => {
        const item = row.original
        const href = `/kubernetes/applications/edit?id=${encodeURIComponent(item.id)}`
        return (
          <RowActionsMenu label={`Actions for ${item.name}`}>
            <DropdownMenuItem asChild>
              <Link to={href}>
                <Eye />
                Open
              </Link>
            </DropdownMenuItem>
            <DropdownMenuItem asChild>
              <Link to={href}>
                <Pencil />
                Edit
              </Link>
            </DropdownMenuItem>
            <DropdownMenuSeparator />
            <DropdownMenuItem
              disabled={busy}
              onClick={() => applyMutation.mutate(item)}
            >
              <Rocket />
              Apply
            </DropdownMenuItem>
            <DropdownMenuItem
              disabled={busy}
              onClick={() => dryRunMutation.mutate(item)}
            >
              <CheckCircle2 />
              Dry run
            </DropdownMenuItem>
            <DropdownMenuItem
              disabled={busy}
              onClick={() => duplicateMutation.mutate(item)}
            >
              <Copy />
              Duplicate
            </DropdownMenuItem>
            <DropdownMenuItem
              disabled={busy}
              onClick={() => {
                void downloadYaml(item).catch((err) =>
                  toastRequestError(err, "Download failed"),
                )
              }}
            >
              <Download />
              Download YAML
            </DropdownMenuItem>
            <DropdownMenuSeparator />
            <DropdownMenuItem
              variant="destructive"
              disabled={busy}
              onClick={() => setConfirm({ kind: "remove-cluster", row: item })}
            >
              <Unplug />
              Remove from cluster
            </DropdownMenuItem>
            <DropdownMenuItem
              variant="destructive"
              disabled={busy}
              onClick={() => setConfirm({ kind: "delete-store", row: item })}
            >
              <Trash2 />
              Delete from store
            </DropdownMenuItem>
          </RowActionsMenu>
        )
      },
    }),
  ] as ColumnDef<K8sApplicationRow, unknown>[]

  const confirmTitle =
    confirm?.kind === "remove-cluster"
      ? "Remove from cluster?"
      : "Delete from store?"
  const confirmDescription =
    confirm?.kind === "remove-cluster"
      ? `Deletes Kubernetes resources for “${confirm.row.name}”. The saved application stays in SQLite.`
      : `Removes “${confirm?.row.name}” from SQLite. Cluster resources are not deleted.`

  return (
    <ContentLoader
      title="Applications"
      breadcrumb={[
        { label: "Kubernetes", to: "/kubernetes" },
        { label: "Applications" },
      ]}
      isLoading={listQuery.isLoading}
      error={listQuery.error}
      rightComponent={
        <div className="flex flex-wrap items-center gap-2">
          <Button size="sm" variant="outline" onClick={() => invalidate()}>
            <RefreshCw
              className={
                listQuery.isFetching ? "size-3.5 animate-spin" : "size-3.5"
              }
            />
          </Button>
          <Button
            size="sm"
            onClick={() => navigate("/kubernetes/applications/edit")}
          >
            <Plus className="size-3.5" />
            Create
          </Button>
        </div>
      }
    >
      <div className="space-y-4">
        <ClusterBanner />
        <p className="text-sm text-muted-foreground">
          Saved application manifests (YAML + namespace) in SQLite. Status is
          probed live from the cluster. Use Remove from cluster to uninstall
          resources; Delete from store only drops the saved entry.
        </p>
        <DataTable
          columns={columns}
          source={{ type: "client", data: rows }}
          getRowId={(row) => row.id}
          {...k8sSelectableTableProps}
          initialState={k8sInitialState(20)}
          actionBar={(table) => (
            <K8sBulkActionBar
              table={table}
              onDone={invalidate}
              actions={[
                {
                  key: "apply",
                  label: "Apply",
                  icon: <Rocket />,
                  run: (selected) =>
                    runForEachSelected(selected, (row) =>
                      applySavedApplication(row.id),
                    ),
                },
                {
                  key: "remove-cluster",
                  label: "Remove from cluster",
                  icon: <Unplug />,
                  variant: "destructive",
                  confirm:
                    "Remove Kubernetes resources for {n} application(s)? Saved entries stay in SQLite.",
                  run: (selected) =>
                    runForEachSelected(selected, (row) =>
                      removeApplicationFromCluster(row.id),
                    ),
                },
                {
                  key: "delete-store",
                  label: "Delete from store",
                  icon: <Trash2 />,
                  variant: "destructive",
                  confirm:
                    "Delete {n} application(s) from SQLite? Cluster resources are not removed.",
                  run: (selected) =>
                    runForEachSelected(selected, (row) =>
                      deleteApplication(row.id),
                    ),
                },
              ]}
            />
          )}
        />
      </div>

      <Dialog
        open={!!confirm}
        onOpenChange={(o) => !o && setConfirm(null)}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{confirmTitle}</DialogTitle>
            <DialogDescription>{confirmDescription}</DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setConfirm(null)}>
              Cancel
            </Button>
            <Button
              variant="destructive"
              disabled={busy}
              onClick={() => {
                if (!confirm) return
                if (confirm.kind === "remove-cluster") {
                  removeMutation.mutate(confirm.row)
                } else {
                  deleteMutation.mutate(confirm.row)
                }
              }}
            >
              {confirm?.kind === "remove-cluster"
                ? "Remove from cluster"
                : "Delete from store"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </ContentLoader>
  )
}
