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
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { asArray } from "@/lib/as-array"
import { toastRequestError } from "@/lib/network"

import {
  createService,
  deleteService,
  formatAge,
  getStoredNamespace,
  K8S_SERVICES_KEY,
  listServices,
  type ServiceRow,
} from "../_shared/api"
import { k8sNameFilter, k8sInitialState, k8sNamespaceFilter, k8sMultiSelectFilter, k8sSortable, optionsFromValues, useK8sNamespaceOptions } from "../_shared/client-table"
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

const columnHelper = createColumnHelper<ServiceRow>()

export default function ServicesPage() {
  const queryClient = useQueryClient()
  const [createOpen, setCreateOpen] = useState(false)
  const [removeTarget, setRemoveTarget] = useState<ServiceRow | null>(null)
  const [name, setName] = useState("")
  const [namespace, setNamespace] = useState(getStoredNamespace() || "default")
  const [port, setPort] = useState("80")
  const [targetPort, setTargetPort] = useState("80")
  const [type, setType] = useState("ClusterIP")
  const [selector, setSelector] = useState("app=")
  const ns = useNamespaceFilter()

  const listQuery = useQuery({
    queryKey: [K8S_SERVICES_KEY, ns || "all"],
    queryFn: () => listServices(ns || undefined),
    refetchInterval: 15_000,
  })

  const invalidate = () =>
    queryClient.invalidateQueries({ queryKey: [K8S_SERVICES_KEY] })

  const createMutation = useMutation({
    mutationFn: () => {
      const sel: Record<string, string> = {}
      for (const part of selector.split(",")) {
        const [k, v] = part.split("=").map((s) => s.trim())
        if (k && v) sel[k] = v
      }
      return createService({
        name: name.trim(),
        namespace: namespace.trim() || "default",
        type,
        port: Number(port) || 80,
        target_port: Number(targetPort) || Number(port) || 80,
        selector: sel,
      })
    },
    onSuccess: (res) => {
      toast.success(res.message || "Service created")
      setCreateOpen(false)
      setName("")
      invalidate()
    },
    onError: (err) => toastRequestError(err, "Create failed"),
  })

  const deleteMutation = useMutation({
    mutationFn: (row: ServiceRow) => deleteService(row.namespace, row.name),
    onSuccess: (res) => {
      toast.success(res.message || "Service deleted")
      setRemoveTarget(null)
      invalidate()
    },
    onError: (err) => toastRequestError(err, "Delete failed"),
  })

  const rows = useK8sNamespacedRows(asArray(listQuery.data?.data))
  const nsOptions = useK8sNamespaceOptions(rows.map((r) => r.namespace))
  const typeOptions = optionsFromValues(rows.map((r) => r.type))

  const columns = [
    columnHelper.accessor("name", {
      ...k8sNameFilter(),
      cell: ({ row, getValue }) => (
        <Link className="font-medium hover:underline" to={`/kubernetes/services/${encodeURIComponent(row.original.namespace)}/${encodeURIComponent(row.original.name)}`}>
          {getValue()}
        </Link>
      ),
    }),
    columnHelper.accessor("namespace", { ...k8sNamespaceFilter(nsOptions) }),
    columnHelper.accessor("type", { ...k8sMultiSelectFilter("Type", typeOptions) }),
    columnHelper.accessor("cluster_ip", { ...k8sSortable("Cluster IP") }),
    columnHelper.accessor("external_ip", { ...k8sSortable("External") }),
    columnHelper.accessor("ports", { ...k8sSortable("Ports") }),
    columnHelper.accessor("created_at", {
      ...k8sSortable("Age"),
      cell: ({ getValue }) => formatAge(getValue()),
    }),
    columnHelper.display({
      id: "actions",
      cell: ({ row }) => {
        const item = row.original
        const href = `/kubernetes/services/${encodeURIComponent(item.namespace)}/${encodeURIComponent(item.name)}`
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
  ] as ColumnDef<ServiceRow, unknown>[]

  return (
    <ContentLoader
      title="Services"
      breadcrumb={[
        { label: "Kubernetes", to: "/kubernetes" },
        { label: "Services" },
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
                  confirm: "Delete {n} service(s)? This cannot be undone.",
                  run: (selected) =>
                    runForEachSelected(selected, (row) =>
                      deleteService(row.namespace, row.name),
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
            <DialogTitle>Create service</DialogTitle>
            <DialogDescription>
              Creates a Service on the cluster via the Kubernetes API.
            </DialogDescription>
          </DialogHeader>
          <div className="grid gap-3 sm:grid-cols-2">
            <div className="space-y-2 sm:col-span-2">
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
              <Label>Type</Label>
              <Select value={type} onValueChange={setType}>
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="ClusterIP">ClusterIP</SelectItem>
                  <SelectItem value="NodePort">NodePort</SelectItem>
                  <SelectItem value="LoadBalancer">LoadBalancer</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-2">
              <Label>Port</Label>
              <Input value={port} onChange={(e) => setPort(e.target.value)} />
            </div>
            <div className="space-y-2">
              <Label>Target port</Label>
              <Input
                value={targetPort}
                onChange={(e) => setTargetPort(e.target.value)}
              />
            </div>
            <div className="space-y-2 sm:col-span-2">
              <Label>Selector (key=value, …)</Label>
              <Input
                value={selector}
                onChange={(e) => setSelector(e.target.value)}
                className="font-mono text-sm"
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
            <DialogTitle>Delete service?</DialogTitle>
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
