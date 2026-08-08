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
  createIngress,
  deleteIngress,
  formatAge,
  getStoredNamespace,
  K8S_INGRESSES_KEY,
  listIngresses,
  type IngressRow,
  type IngressRule,
  type IngressTLS,
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
import {
  emptyIngressRule,
  IngressAdvancedEditor,
} from "./ingress-form"

const columnHelper = createColumnHelper<IngressRow>()

export default function IngressesPage() {
  const queryClient = useQueryClient()
  const [createOpen, setCreateOpen] = useState(false)
  const [removeTarget, setRemoveTarget] = useState<IngressRow | null>(null)
  const [name, setName] = useState("")
  const [namespace, setNamespace] = useState(getStoredNamespace() || "default")
  const [ingressClass, setIngressClass] = useState("")
  const [rules, setRules] = useState<IngressRule[]>([emptyIngressRule()])
  const [tls, setTls] = useState<IngressTLS[]>([])
  const [labels, setLabels] = useState<Record<string, string>>({})
  const [annotations, setAnnotations] = useState<Record<string, string>>({})
  const ns = useNamespaceFilter()

  const listQuery = useQuery({
    queryKey: [K8S_INGRESSES_KEY, ns || "all"],
    queryFn: () => listIngresses(ns || undefined),
    refetchInterval: 15_000,
  })

  const invalidate = () =>
    queryClient.invalidateQueries({ queryKey: [K8S_INGRESSES_KEY] })

  const resetCreate = () => {
    setName("")
    setNamespace(getStoredNamespace() || "default")
    setIngressClass("")
    setRules([emptyIngressRule()])
    setTls([])
    setLabels({})
    setAnnotations({})
  }

  const createMutation = useMutation({
    mutationFn: () =>
      createIngress({
        name: name.trim(),
        namespace: namespace.trim() || "default",
        ingress_class: ingressClass.trim() || undefined,
        rules: rules.map((rule) => ({
          host: rule.host.trim(),
          paths: rule.paths.map((p) => ({
            path: p.path.trim() || "/",
            path_type: p.path_type || "Prefix",
            service_name: p.service_name.trim(),
            service_port: Number(p.service_port) || 0,
            service_port_name: p.service_port_name?.trim() || undefined,
          })),
        })),
        tls: tls
          .filter((t) => t.secret_name.trim() || t.hosts.some((h) => h.trim()))
          .map((t) => ({
            secret_name: t.secret_name.trim(),
            hosts: t.hosts.map((h) => h.trim()).filter(Boolean),
          })),
        labels,
        annotations,
      }),
    onSuccess: (res) => {
      toast.success(res.message || "Ingress created")
      setCreateOpen(false)
      resetCreate()
      invalidate()
    },
    onError: (err) => toastRequestError(err, "Create failed"),
  })

  const deleteMutation = useMutation({
    mutationFn: (row: IngressRow) => deleteIngress(row.namespace, row.name),
    onSuccess: (res) => {
      toast.success(res.message || "Ingress deleted")
      setRemoveTarget(null)
      invalidate()
    },
    onError: (err) => toastRequestError(err, "Delete failed"),
  })

  const rows = useK8sNamespacedRows(asArray(listQuery.data?.data))
  const nsOptions = useK8sNamespaceOptions(rows.map((r) => r.namespace))
  const classOptions = optionsFromValues(rows.map((r) => r.class))

  const canCreate =
    Boolean(name.trim()) &&
    rules.some((rule) =>
      rule.paths.some((p) => p.service_name.trim() && Number(p.service_port) > 0),
    )

  const columns = [
    columnHelper.accessor("name", {
      ...k8sNameFilter(),
      cell: ({ row, getValue }) => (
        <Link
          className="font-medium hover:underline"
          to={`/kubernetes/ingresses/${encodeURIComponent(row.original.namespace)}/${encodeURIComponent(row.original.name)}`}
        >
          {getValue()}
        </Link>
      ),
    }),
    columnHelper.accessor("namespace", { ...k8sNamespaceFilter(nsOptions) }),
    columnHelper.accessor("class", {
      ...k8sMultiSelectFilter("Class", classOptions),
      cell: ({ getValue }) => getValue() || "—",
    }),
    columnHelper.accessor("hosts", {
      ...k8sSortable("Hosts"),
      cell: ({ getValue }) => getValue() || "—",
    }),
    columnHelper.accessor("address", {
      ...k8sSortable("Address"),
      cell: ({ getValue }) => getValue() || "—",
    }),
    columnHelper.accessor("created_at", {
      ...k8sSortable("Age"),
      cell: ({ getValue }) => formatAge(getValue()),
    }),
    columnHelper.display({
      id: "actions",
      cell: ({ row }) => {
        const item = row.original
        const href = `/kubernetes/ingresses/${encodeURIComponent(item.namespace)}/${encodeURIComponent(item.name)}`
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
  ] as ColumnDef<IngressRow, unknown>[]

  return (
    <ContentLoader
      title="Ingresses"
      breadcrumb={[
        { label: "Kubernetes", to: "/kubernetes" },
        { label: "Ingresses" },
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
                  confirm: "Delete {n} ingress(s)? This cannot be undone.",
                  run: (selected) =>
                    runForEachSelected(selected, (row) =>
                      deleteIngress(row.namespace, row.name),
                    ),
                },
              ]}
            />
          )}
        />
      </div>

      <Dialog
        open={createOpen}
        onOpenChange={(open) => {
          setCreateOpen(open)
          if (!open) resetCreate()
        }}
      >
        <DialogContent className="flex max-h-[min(90vh,56rem)] max-w-[calc(100%-2rem)] flex-col gap-4 overflow-hidden sm:max-w-4xl">
          <DialogHeader>
            <DialogTitle>Create ingress</DialogTitle>
            <DialogDescription>
              Build rules from Services and TLS secrets in the target namespace.
              You can still type custom values when something is missing.
            </DialogDescription>
          </DialogHeader>
          <div className="min-h-0 flex-1 space-y-4 overflow-y-auto pe-1">
            <div className="grid gap-3 sm:grid-cols-2">
              <div className="space-y-2">
                <Label>Name</Label>
                <Input
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  placeholder="my-ingress"
                />
              </div>
              <div className="space-y-2">
                <Label>Namespace</Label>
                <Input
                  value={namespace}
                  onChange={(e) => setNamespace(e.target.value)}
                  placeholder="default"
                />
              </div>
            </div>
            <IngressAdvancedEditor
              compact
              namespace={namespace.trim() || "default"}
              ingressClass={ingressClass}
              onIngressClassChange={setIngressClass}
              rules={rules}
              onRulesChange={setRules}
              tls={tls}
              onTlsChange={setTls}
              labels={labels}
              onLabelsChange={setLabels}
              annotations={annotations}
              onAnnotationsChange={setAnnotations}
              metadataResetKey={createOpen ? "create-open" : "create-closed"}
            />
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setCreateOpen(false)}>
              Cancel
            </Button>
            <Button
              disabled={!canCreate || createMutation.isPending}
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
            <DialogTitle>Delete ingress?</DialogTitle>
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
