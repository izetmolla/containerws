import { Link } from "react-router"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { createColumnHelper, type ColumnDef } from "@tanstack/react-table"
import {
  Ban,
  CirclePlay,
  Eye,
  RefreshCw,
} from "lucide-react"
import { toast } from "sonner"

import ContentLoader from "@/components/content-loader"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { DataTable } from "@/components/datatable"
import {
  DropdownMenuItem,
  DropdownMenuSeparator,
} from "@/components/ui/dropdown-menu"
import { asArray } from "@/lib/as-array"
import { toastRequestError } from "@/lib/network"

import {
  cordonNode,
  formatAge,
  K8S_NODES_KEY,
  listNodes,
  uncordonNode,
  type NodeRow,
} from "../_shared/api"
import { k8sNameFilter, k8sInitialState, k8sMultiSelectFilter, k8sSortable, optionsFromValues } from "../_shared/client-table"
import {
  K8sBulkActionBar,
  k8sSelectableTableProps,
  runForEachSelected,
} from "../_shared/bulk-actions"
import { ClusterBanner } from "../_shared/cluster-banner"
import { RowActionsMenu } from "../_shared/row-actions"

const columnHelper = createColumnHelper<NodeRow>()

export default function NodesPage() {
  const queryClient = useQueryClient()

  const listQuery = useQuery({
    queryKey: [K8S_NODES_KEY],
    queryFn: listNodes,
    refetchInterval: 15_000,
  })

  const invalidate = () =>
    queryClient.invalidateQueries({ queryKey: [K8S_NODES_KEY] })

  const cordonMutation = useMutation({
    mutationFn: (name: string) => cordonNode(name),
    onSuccess: (res) => {
      toast.success(res.message || "Node cordoned")
      invalidate()
    },
    onError: (err) => toastRequestError(err, "Cordon failed"),
  })

  const uncordonMutation = useMutation({
    mutationFn: (name: string) => uncordonNode(name),
    onSuccess: (res) => {
      toast.success(res.message || "Node uncordoned")
      invalidate()
    },
    onError: (err) => toastRequestError(err, "Uncordon failed"),
  })

  const rows = asArray(listQuery.data?.data)
  const statusOptions = optionsFromValues(rows.map((r) => r.status))
  const roleOptions = optionsFromValues(rows.map((r) => r.roles))

  const ready = rows.filter((r) => r.status === "Ready").length

  const columns = [
    columnHelper.accessor("name", {
      ...k8sNameFilter(),
      cell: ({ row }) => (
        <Link
          to={`/kubernetes/nodes/${encodeURIComponent(row.original.name)}`}
          className="font-medium hover:underline"
        >
          {row.original.name}
        </Link>
      ),
    }),
    columnHelper.accessor("status", {
      ...k8sMultiSelectFilter("Status", statusOptions),
      cell: ({ row }) => (
        <div className="flex flex-wrap gap-1">
          <Badge
            variant={row.original.status === "Ready" ? "default" : "destructive"}
          >
            {row.original.status}
          </Badge>
          {row.original.unschedulable ? (
            <Badge variant="secondary">SchedulingDisabled</Badge>
          ) : null}
        </div>
      ),
    }),
    columnHelper.accessor("roles", { ...k8sMultiSelectFilter("Roles", roleOptions) }),
    columnHelper.accessor("internal_ip", {
      ...k8sSortable("Internal IP"),
      cell: ({ getValue }) => getValue() || "—",
    }),
    columnHelper.accessor("pod_count", { ...k8sSortable("Pods") }),
    columnHelper.accessor("cpu", { ...k8sSortable("CPU") }),
    columnHelper.accessor("memory", { ...k8sSortable("Memory") }),
    columnHelper.accessor("version", { ...k8sSortable("Version") }),
    columnHelper.accessor("created_at", {
      ...k8sSortable("Age"),
      cell: ({ getValue }) => formatAge(getValue()),
    }),
    columnHelper.display({
      id: "actions",
      cell: ({ row }) => {
        const item = row.original
        return (
          <RowActionsMenu label={`Actions for ${item.name}`}>
            <DropdownMenuItem asChild>
              <Link to={`/kubernetes/nodes/${encodeURIComponent(item.name)}`}>
                <Eye />
                View details
              </Link>
            </DropdownMenuItem>
            <DropdownMenuSeparator />
            {item.unschedulable ? (
              <DropdownMenuItem
                onClick={() => uncordonMutation.mutate(item.name)}
              >
                <CirclePlay />
                Uncordon
              </DropdownMenuItem>
            ) : (
              <DropdownMenuItem onClick={() => cordonMutation.mutate(item.name)}>
                <Ban />
                Cordon
              </DropdownMenuItem>
            )}
          </RowActionsMenu>
        )
      },
    }),
  ] as ColumnDef<NodeRow, unknown>[]

  return (
    <ContentLoader
      title="Nodes"
      breadcrumb={[
        { label: "Kubernetes", to: "/kubernetes" },
        { label: "Nodes" },
      ]}
      isLoading={listQuery.isLoading}
      error={listQuery.error}
      rightComponent={
        <div className="flex flex-wrap items-center gap-2">
          <Button size="sm" variant="outline" onClick={() => invalidate()}>
            <RefreshCw className="size-3.5" />
          </Button>
        </div>
      }
    >
      <div className="space-y-4">
        <ClusterBanner />
        <div className="grid gap-3 sm:grid-cols-3">
          <Stat label="Nodes" value={rows.length} />
          <Stat label="Ready" value={`${ready}/${rows.length || 0}`} />
          <Stat
            label="Pods on nodes"
            value={rows.reduce((a, r) => a + (r.pod_count || 0), 0)}
          />
        </div>
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
                  key: "cordon",
                  label: "Cordon",
                  icon: <Ban />,
                  tooltip: "Mark selected nodes unschedulable",
                  run: (selected) =>
                    runForEachSelected(selected, (row) => cordonNode(row.name)),
                },
                {
                  key: "uncordon",
                  label: "Uncordon",
                  icon: <CirclePlay />,
                  tooltip: "Mark selected nodes schedulable",
                  run: (selected) =>
                    runForEachSelected(selected, (row) =>
                      uncordonNode(row.name),
                    ),
                },
              ]}
            />
          )}
        />
      </div>
    </ContentLoader>
  )
}

function Stat({ label, value }: { label: string; value: string | number }) {
  return (
    <div className="rounded-xl border bg-muted/20 px-4 py-3">
      <p className="text-xs text-muted-foreground">{label}</p>
      <p className="mt-1 font-mono text-sm font-medium">{value}</p>
    </div>
  )
}
