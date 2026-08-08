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
import { asArray } from "@/lib/as-array"
import { toastRequestError } from "@/lib/network"

import {
  createNamespace,
  deleteNamespace,
  formatAge,
  K8S_NAMESPACES_KEY,
  listNamespaces,
  type NamespaceRow,
} from "../_shared/api"
import { k8sInitialState, k8sNameFilter, k8sMultiSelectFilter, k8sSortable, optionsFromValues } from "../_shared/client-table"
import {
  K8sBulkActionBar,
  k8sSelectableTableProps,
  runForEachSelected,
} from "../_shared/bulk-actions"
import { RowActionsMenu } from "../_shared/row-actions"
import { ClusterBanner } from "../_shared/cluster-banner"
import {
  useK8sNamespaceListRows,
} from "../_shared/system-resources"

const columnHelper = createColumnHelper<NamespaceRow>()

export default function NamespacesPage() {
  const queryClient = useQueryClient()
  const [createOpen, setCreateOpen] = useState(false)
  const [name, setName] = useState("")
  const [removeTarget, setRemoveTarget] = useState<NamespaceRow | null>(null)

  const listQuery = useQuery({
    queryKey: [K8S_NAMESPACES_KEY],
    queryFn: listNamespaces,
    refetchInterval: 15_000,
  })

  const invalidate = () =>
    queryClient.invalidateQueries({ queryKey: [K8S_NAMESPACES_KEY] })

  const createMutation = useMutation({
    mutationFn: () => createNamespace(name.trim()),
    onSuccess: (res) => {
      toast.success(res.message || "Namespace created")
      setCreateOpen(false)
      setName("")
      invalidate()
    },
    onError: (err) => toastRequestError(err, "Create failed"),
  })

  const deleteMutation = useMutation({
    mutationFn: (n: string) => deleteNamespace(n),
    onSuccess: (res) => {
      toast.success(res.message || "Namespace deleted")
      setRemoveTarget(null)
      invalidate()
    },
    onError: (err) => toastRequestError(err, "Delete failed"),
  })

  const rows = useK8sNamespaceListRows(asArray(listQuery.data?.data))
  const statusOptions = optionsFromValues(rows.map((r) => r.status))

  const columns = [
    columnHelper.accessor("name", {
      ...k8sNameFilter(),
      cell: ({ getValue }) => (
        <Link className="font-medium hover:underline" to={`/kubernetes/namespaces/${encodeURIComponent(getValue())}`}>
          {getValue()}
        </Link>
      ),
    }),
    columnHelper.accessor("status", {
      ...k8sMultiSelectFilter("Status", statusOptions),
      size: 120,
    }),
    columnHelper.accessor("created_at", {
      ...k8sSortable("Age"),
      size: 100,
      cell: ({ getValue }) => formatAge(getValue()),
    }),
    columnHelper.display({
      id: "actions",
      size: 56,
      cell: ({ row }) => {
        const item = row.original
        const href = `/kubernetes/namespaces/${encodeURIComponent(item.name)}`
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
  ] as ColumnDef<NamespaceRow, unknown>[]

  return (
    <ContentLoader
      title="Namespaces"
      breadcrumb={[
        { label: "Kubernetes", to: "/kubernetes" },
        { label: "Namespaces" },
      ]}
      isLoading={listQuery.isLoading}
      error={listQuery.error}
      rightComponent={
        <div className="flex flex-wrap items-center gap-2">
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
          getRowId={(row) => row.name}
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
                  confirm: "Delete {n} namespace(s)? This cannot be undone.",
                  run: (selected) =>
                    runForEachSelected(selected, (row) =>
                      deleteNamespace(row.name),
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
            <DialogTitle>Create namespace</DialogTitle>
            <DialogDescription>
              Creates a namespace on the connected cluster (not stored in the app DB).
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-2">
            <Label htmlFor="ns-name">Name</Label>
            <Input
              id="ns-name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="my-namespace"
            />
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
            <DialogTitle>Delete namespace?</DialogTitle>
            <DialogDescription>
              Delete “{removeTarget?.name}”? This removes it from the cluster.
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
                removeTarget && deleteMutation.mutate(removeTarget.name)
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
