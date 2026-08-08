import { useState } from "react"
import { Link, useNavigate } from "react-router"
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
import { asArray } from "@/lib/as-array"
import { toastRequestError } from "@/lib/network"

import {
  deleteSecret,
  formatAge,
  K8S_SECRETS_KEY,
  listSecrets,
  type SecretRow,
} from "../_shared/api"
import { k8sNameFilter, k8sInitialState, k8sTextFilter, k8sNamespaceFilter, k8sSortable, useK8sNamespaceOptions } from "../_shared/client-table"
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

const columnHelper = createColumnHelper<SecretRow>()

export default function SecretsPage() {
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const [removeTarget, setRemoveTarget] = useState<SecretRow | null>(null)
  const ns = useNamespaceFilter()

  const listQuery = useQuery({
    queryKey: [K8S_SECRETS_KEY, ns || "all"],
    queryFn: () => listSecrets(ns || undefined),
    refetchInterval: 15_000,
  })

  const invalidate = () =>
    queryClient.invalidateQueries({ queryKey: [K8S_SECRETS_KEY] })

  const deleteMutation = useMutation({
    mutationFn: (row: SecretRow) => deleteSecret(row.namespace, row.name),
    onSuccess: (res) => {
      toast.success(res.message || "Secret deleted")
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
        <Link className="font-medium hover:underline" to={`/kubernetes/secrets/${encodeURIComponent(row.original.namespace)}/${encodeURIComponent(row.original.name)}`}>
          {getValue()}
        </Link>
      ),
    }),
    columnHelper.accessor("namespace", { ...k8sNamespaceFilter(nsOptions) }),
    columnHelper.accessor("type", { ...k8sTextFilter("Type") }),
    columnHelper.accessor("keys", { ...k8sSortable("Keys") }),
    columnHelper.accessor("created_at", {
      ...k8sSortable("Age"),
      cell: ({ getValue }) => formatAge(getValue()),
    }),
    columnHelper.display({
      id: "actions",
      cell: ({ row }) => {
        const item = row.original
        const href = `/kubernetes/secrets/${encodeURIComponent(item.namespace)}/${encodeURIComponent(item.name)}`
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
  ] as ColumnDef<SecretRow, unknown>[]

  return (
    <ContentLoader
      title="Secrets"
      breadcrumb={[
        { label: "Kubernetes", to: "/kubernetes" },
        { label: "Secrets" },
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
            onClick={() => navigate("/kubernetes/secrets/create")}
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
                  confirm: "Delete {n} secret(s)? This cannot be undone.",
                  run: (selected) =>
                    runForEachSelected(selected, (row) =>
                      deleteSecret(row.namespace, row.name),
                    ),
                },
              ]}
            />
          )}
        />
      </div>

      <Dialog
        open={!!removeTarget}
        onOpenChange={(o) => !o && setRemoveTarget(null)}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Delete secret?</DialogTitle>
            <DialogDescription>
              Delete {removeTarget?.namespace}/{removeTarget?.name}? Secret
              values are never stored in the app database.
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
