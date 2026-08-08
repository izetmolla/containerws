import { useQuery } from "@tanstack/react-query"
import { Link } from "react-router"
import { RefreshCw } from "lucide-react"
import { createColumnHelper, type ColumnDef } from "@tanstack/react-table"

import ContentLoader from "@/components/content-loader"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { DataTable } from "@/components/datatable"
import { asArray } from "@/lib/as-array"

import {
  formatAge,
  K8S_EVENTS_KEY,
  K8S_NODES_KEY,
  listEvents,
  listNodes,
  type EventRow,
  type NodeRow,
} from "../_shared/api"
import { k8sNameFilter, k8sClientTableProps, k8sInitialState, k8sTextFilter, k8sNamespaceFilter, k8sMultiSelectFilter, k8sSortable, optionsFromValues, useK8sNamespaceOptions } from "../_shared/client-table"
import { ClusterBanner, useClusterStatus } from "../_shared/cluster-banner"
import {
  useK8sNamespacedRows,
} from "../_shared/system-resources"

const nodeHelper = createColumnHelper<NodeRow>()
const eventHelper = createColumnHelper<EventRow>()

export default function KubernetesClusterPage() {
  const statusQuery = useClusterStatus()
  const nodesQuery = useQuery({
    queryKey: [K8S_NODES_KEY],
    queryFn: listNodes,
    refetchInterval: 20_000,
  })
  const eventsQuery = useQuery({
    queryKey: [K8S_EVENTS_KEY],
    queryFn: () => listEvents(),
    refetchInterval: 20_000,
  })

  const status = statusQuery.data?.data
  const nodes = asArray(nodesQuery.data?.data)
  const events = useK8sNamespacedRows(asArray(eventsQuery.data?.data))
  const nsOptions = useK8sNamespaceOptions(events.map((r) => r.namespace))
  const eventTypeOptions = optionsFromValues(events.map((r) => r.type))
  const eventReasonOptions = optionsFromValues(events.map((r) => r.reason))
  const nodeRoleOptions = optionsFromValues(nodes.map((r) => r.roles))

  const nodeColumns = [
    nodeHelper.accessor("name", {
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
    nodeHelper.accessor("status", {
      ...k8sSortable("Status"),
      cell: ({ getValue }) => (
        <Badge variant={getValue() === "Ready" ? "default" : "destructive"}>
          {getValue()}
        </Badge>
      ),
    }),
    nodeHelper.accessor("roles", { ...k8sMultiSelectFilter("Roles", nodeRoleOptions) }),
    nodeHelper.accessor("pod_count", { ...k8sSortable("Pods") }),
    nodeHelper.accessor("version", { ...k8sSortable("Version") }),
    nodeHelper.accessor("cpu", { ...k8sSortable("CPU") }),
    nodeHelper.accessor("memory", { ...k8sSortable("Memory") }),
    nodeHelper.accessor("os_image", { ...k8sTextFilter("OS") }),
  ] as ColumnDef<NodeRow, unknown>[]

  const eventColumns = [
    eventHelper.accessor("type", {
      ...k8sMultiSelectFilter("Type", eventTypeOptions),
      cell: ({ getValue }) => (
        <Badge variant={getValue() === "Warning" ? "destructive" : "secondary"}>
          {getValue()}
        </Badge>
      ),
    }),
    eventHelper.accessor("namespace", { ...k8sNamespaceFilter(nsOptions) }),
    eventHelper.accessor("object", { ...k8sSortable("Object") }),
    eventHelper.accessor("reason", {
      ...k8sMultiSelectFilter("Reason", eventReasonOptions),
    }),
    eventHelper.accessor("message", { ...k8sTextFilter("Message") }),
    eventHelper.accessor("count", { ...k8sSortable("Count") }),
    eventHelper.accessor("last_seen", {
      ...k8sSortable("Age"),
      cell: ({ getValue }) => formatAge(getValue()),
    }),
  ] as ColumnDef<EventRow, unknown>[]

  return (
    <ContentLoader
      title="Cluster"
      breadcrumb={[
        { label: "Kubernetes", to: "/kubernetes" },
        { label: "Cluster" },
      ]}
      isLoading={statusQuery.isLoading}
      error={statusQuery.error}
      rightComponent={
        <div className="flex flex-wrap items-center gap-2">
          <Button size="sm" variant="outline" asChild>
            <Link to="/kubernetes/nodes">Manage nodes</Link>
          </Button>
          <Button
            size="sm"
            variant="outline"
            onClick={() => {
              void statusQuery.refetch()
              void nodesQuery.refetch()
              void eventsQuery.refetch()
            }}
          >
            <RefreshCw className="size-3.5" />
          </Button>
        </div>
      }
    >
      <div className="space-y-6">
        <ClusterBanner />

        <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
          <Stat
            label="API"
            value={status?.reachable ? status.version || "OK" : "Offline"}
          />
          <Stat
            label="Nodes"
            value={
              status?.nodes != null
                ? `${status.nodes_ready ?? 0}/${status.nodes}`
                : "—"
            }
          />
          <Stat label="Namespaces" value={status?.namespaces ?? "—"} />
          <Stat
            label="Pods"
            value={
              status?.pods != null
                ? `${status.pods_running ?? 0}/${status.pods}`
                : "—"
            }
          />
        </div>

        <div className="space-y-2">
          <div className="flex items-center justify-between">
            <h2 className="text-sm font-medium">Nodes</h2>
            <Button size="sm" variant="ghost" asChild>
              <Link to="/kubernetes/nodes">View all</Link>
            </Button>
          </div>
          <DataTable
            columns={nodeColumns}
            source={{ type: "client", data: nodes }}
            getRowId={(row) => row.name}
            {...k8sClientTableProps}
            initialState={k8sInitialState(10, false)}
          />
        </div>

        <div className="space-y-2">
          <h2 className="text-sm font-medium">Recent events</h2>
          <DataTable
            columns={eventColumns}
            source={{ type: "client", data: events }}
            getRowId={(row) =>
              `${row.namespace}/${row.name}/${row.object}/${row.reason}/${row.last_seen}`
            }
            {...k8sClientTableProps}
            initialState={k8sInitialState(10, false)}
          />
        </div>
      </div>
    </ContentLoader>
  )
}

function Stat({ label, value }: { label: string; value: string | number }) {
  return (
    <div className="rounded-xl border bg-muted/20 px-4 py-3">
      <p className="text-xs text-muted-foreground">{label}</p>
      <p className="mt-1 truncate font-mono text-sm font-medium">{value}</p>
    </div>
  )
}
