import { useState } from "react"
import { Link } from "react-router"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { createColumnHelper, type ColumnDef } from "@tanstack/react-table"
import { Eye, Plus, RefreshCw, Trash2 } from "lucide-react"
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
import { Textarea } from "@/components/ui/textarea"
import { asArray } from "@/lib/as-array"
import { toastRequestError } from "@/lib/network"

import {
  createConfigMap,
  deleteConfigMap,
  formatAge,
  getStoredNamespace,
  K8S_CONFIGMAPS_KEY,
  listConfigMaps,
  type ConfigMapRow,
} from "../_shared/api"
import { k8sNameFilter, k8sInitialState, k8sNamespaceFilter, k8sSortable, useK8sNamespaceOptions } from "../_shared/client-table"
import {
  K8sBulkActionBar,
  k8sSelectableTableProps,
  runForEachSelected,
} from "../_shared/bulk-actions"
import { RowActionsMenu } from "../_shared/row-actions"
import { ClusterBanner } from "../_shared/cluster-banner"
import {
  useK8sNamespacedRows,
} from "../_shared/system-resources"
import { NamespaceSelector, useNamespaceFilter } from "../_shared/namespace-selector"

const columnHelper = createColumnHelper<ConfigMapRow>()

export default function ConfigMapsPage() {
  const queryClient = useQueryClient()
  const [createOpen, setCreateOpen] = useState(false)
  const [removeTarget, setRemoveTarget] = useState<ConfigMapRow | null>(null)
  const [name, setName] = useState("")
  const [namespace, setNamespace] = useState(getStoredNamespace() || "default")
  const [dataText, setDataText] = useState("key=value")
  const ns = useNamespaceFilter()

  const listQuery = useQuery({
    queryKey: [K8S_CONFIGMAPS_KEY, ns || "all"],
    queryFn: () => listConfigMaps(ns || undefined),
    refetchInterval: 15_000,
  })

  const invalidate = () =>
    queryClient.invalidateQueries({ queryKey: [K8S_CONFIGMAPS_KEY] })

  const createMutation = useMutation({
    mutationFn: () => {
      const data: Record<string, string> = {}
      for (const line of dataText.split("\n")) {
        const i = line.indexOf("=")
        if (i <= 0) continue
        data[line.slice(0, i).trim()] = line.slice(i + 1)
      }
      return createConfigMap({
        name: name.trim(),
        namespace: namespace.trim() || "default",
        data,
      })
    },
    onSuccess: (res) => {
      toast.success(res.message || "ConfigMap created")
      setCreateOpen(false)
      setName("")
      invalidate()
    },
    onError: (err) => toastRequestError(err, "Create failed"),
  })

  const deleteMutation = useMutation({
    mutationFn: (row: ConfigMapRow) => deleteConfigMap(row.namespace, row.name),
    onSuccess: (res) => {
      toast.success(res.message || "ConfigMap deleted")
      setRemoveTarget(null)
      invalidate()
    },
    onError: (err) => toastRequestError(err, "Delete failed"),
  })

  const rows = useK8sNamespacedRows(asArray(listQuery.data?.data))
  const nsOptions = useK8sNamespaceOptions(rows.map((r) => r.namespace))

  const columns = [
    columnHelper.accessor("name", {
      ...k8sNameFilter(),
      cell: ({ row, getValue }) => (
        <Link className="font-medium hover:underline" to={`/kubernetes/configmaps/${encodeURIComponent(row.original.namespace)}/${encodeURIComponent(row.original.name)}`}>
          {getValue()}
        </Link>
      ),
    }),
    columnHelper.accessor("namespace", { ...k8sNamespaceFilter(nsOptions) }),
    columnHelper.accessor("keys", { ...k8sSortable("Keys") }),
    columnHelper.accessor("created_at", {
      ...k8sSortable("Age"),
      cell: ({ getValue }) => formatAge(getValue()),
    }),
    columnHelper.display({
      id: "actions",
      cell: ({ row }) => {
        const item = row.original
        const href = `/kubernetes/configmaps/${encodeURIComponent(item.namespace)}/${encodeURIComponent(item.name)}`
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
        )
      },
    }),
  ] as ColumnDef<ConfigMapRow, unknown>[]

  return (
    <ContentLoader
      title="ConfigMaps"
      breadcrumb={[
        { label: "Kubernetes", to: "/kubernetes" },
        { label: "ConfigMaps" },
      ]}
      isLoading={listQuery.isLoading}
      error={listQuery.error}
      rightComponent={
        <div className="flex flex-wrap items-center gap-2">
          <NamespaceSelector />
          <Button size="sm" variant="outline" onClick={() => invalidate()}>
            <RefreshCw className="size-3.5" />
          </Button>
          <Button size="sm" onClick={() => setCreateOpen(true)}>
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
                  confirm: "Delete {n} ConfigMap(s)? This cannot be undone.",
                  run: (selected) =>
                    runForEachSelected(selected, (row) =>
                      deleteConfigMap(row.namespace, row.name),
                    ),
                },
              ]}
            />
          )}
        />
      </div>

      <Dialog open={createOpen} onOpenChange={setCreateOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Create ConfigMap</DialogTitle>
            <DialogDescription>
              One key=value per line. Stored on the cluster only.
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-3">
            <div className="space-y-2">
              <Label>Name</Label>
              <Input value={name} onChange={(e) => setName(e.target.value)} />
            </div>
            <div className="space-y-2">
              <Label>Namespace</Label>
              <Input
                value={namespace}
                onChange={(e) => setNamespace(e.target.value)}
              />
            </div>
            <div className="space-y-2">
              <Label>Data</Label>
              <Textarea
                value={dataText}
                onChange={(e) => setDataText(e.target.value)}
                className="min-h-28 font-mono text-xs"
              />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setCreateOpen(false)}>
              Cancel
            </Button>
            <Button
              disabled={!name.trim() || createMutation.isPending}
              onClick={() => createMutation.mutate()}
            >
              Create
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
            <DialogTitle>Delete ConfigMap?</DialogTitle>
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
