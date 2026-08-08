import { Link, useParams } from "react-router"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { createColumnHelper, type ColumnDef } from "@tanstack/react-table"
import { ArrowLeft, RefreshCw } from "lucide-react"
import { toast } from "sonner"

import ContentLoader from "@/components/content-loader"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { DataTable } from "@/components/datatable"
import { asArray } from "@/lib/as-array"
import { toastRequestError } from "@/lib/network"

import {
  cordonNode,
  formatAge,
  getNode,
  K8S_NODE_DETAIL_KEY,
  K8S_NODES_KEY,
  listNodePods,
  uncordonNode,
  type NodePodRow,
} from "../_shared/api"
import { k8sNameFilter, k8sClientTableProps, k8sInitialState, k8sNamespaceFilter, k8sMultiSelectFilter, k8sSortable, optionsFromValues, useK8sNamespaceOptions } from "../_shared/client-table"
import { ClusterBanner } from "../_shared/cluster-banner"

const podHelper = createColumnHelper<NodePodRow>()

export default function NodeDetailPage() {
  const { name: rawName } = useParams()
  const name = decodeURIComponent(rawName || "")
  const queryClient = useQueryClient()

  const nodeQuery = useQuery({
    queryKey: [K8S_NODE_DETAIL_KEY, name],
    queryFn: () => getNode(name),
    enabled: !!name,
    refetchInterval: 15_000,
  })

  const podsQuery = useQuery({
    queryKey: [K8S_NODE_DETAIL_KEY, name, "pods"],
    queryFn: () => listNodePods(name),
    enabled: !!name,
    refetchInterval: 15_000,
  })

  const invalidate = () => {
    void queryClient.invalidateQueries({ queryKey: [K8S_NODE_DETAIL_KEY, name] })
    void queryClient.invalidateQueries({ queryKey: [K8S_NODES_KEY] })
  }

  const cordonMutation = useMutation({
    mutationFn: () => cordonNode(name),
    onSuccess: (res) => {
      toast.success(res.message || "Node cordoned")
      invalidate()
    },
    onError: (err) => toastRequestError(err, "Cordon failed"),
  })

  const uncordonMutation = useMutation({
    mutationFn: () => uncordonNode(name),
    onSuccess: (res) => {
      toast.success(res.message || "Node uncordoned")
      invalidate()
    },
    onError: (err) => toastRequestError(err, "Uncordon failed"),
  })

  const node = nodeQuery.data?.data
  const pods = asArray(podsQuery.data?.data)
  const nsOptions = useK8sNamespaceOptions(pods.map((r) => r.namespace))
  const statusOptions = optionsFromValues(pods.map((r) => r.status))

  const podColumns = [
    podHelper.accessor("name", { ...k8sNameFilter() }),
    podHelper.accessor("namespace", { ...k8sNamespaceFilter(nsOptions) }),
    podHelper.accessor("status", { ...k8sMultiSelectFilter("Status", statusOptions) }),
    podHelper.accessor("ready", { ...k8sSortable("Ready") }),
    podHelper.accessor("restarts", { ...k8sSortable("Restarts") }),
    podHelper.accessor("ip", { ...k8sSortable("IP") }),
    podHelper.accessor("created_at", {
      ...k8sSortable("Age"),
      cell: ({ getValue }) => formatAge(getValue()),
    }),
  ] as ColumnDef<NodePodRow, unknown>[]

  return (
    <ContentLoader
      title={name || "Node"}
      breadcrumb={[
        { label: "Kubernetes", to: "/kubernetes" },
        { label: "Nodes", to: "/kubernetes/nodes" },
        { label: name || "…" },
      ]}
      isLoading={nodeQuery.isLoading}
      error={nodeQuery.error}
      rightComponent={
        <div className="flex flex-wrap items-center gap-2">
          <Button size="sm" variant="outline" asChild>
            <Link to="/kubernetes/nodes">
              <ArrowLeft className="size-3.5" />
              Nodes
            </Link>
          </Button>
          {node?.unschedulable ? (
            <Button
              size="sm"
              variant="outline"
              disabled={uncordonMutation.isPending}
              onClick={() => uncordonMutation.mutate()}
            >
              Uncordon
            </Button>
          ) : (
            <Button
              size="sm"
              variant="outline"
              disabled={cordonMutation.isPending}
              onClick={() => cordonMutation.mutate()}
            >
              Cordon
            </Button>
          )}
          <Button size="sm" variant="outline" onClick={() => invalidate()}>
            <RefreshCw className="size-3.5" />
          </Button>
        </div>
      }
    >
      <div className="space-y-6">
        <ClusterBanner />

        {node ? (
          <>
            <div className="flex flex-wrap items-center gap-2">
              <Badge
                variant={node.status === "Ready" ? "default" : "destructive"}
              >
                {node.status}
              </Badge>
              <Badge variant="secondary">{node.roles}</Badge>
              {node.unschedulable ? (
                <Badge variant="secondary">SchedulingDisabled</Badge>
              ) : null}
              <span className="text-xs text-muted-foreground">
                Age {formatAge(node.created_at)}
              </span>
            </div>

            <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
              <Stat label="Internal IP" value={node.internal_ip || "—"} />
              <Stat label="Hostname" value={node.hostname || "—"} />
              <Stat label="Pods" value={`${node.pod_count}${node.pods_capacity ? ` / ${node.pods_capacity}` : ""}`} />
              <Stat label="Kubelet" value={node.version || "—"} />
              <Stat label="CPU (cap / alloc)" value={`${node.cpu} / ${node.cpu_allocatable || "—"}`} />
              <Stat
                label="Memory (cap / alloc)"
                value={`${node.memory} / ${node.memory_allocatable || "—"}`}
              />
              <Stat label="OS" value={node.os_image || "—"} />
              <Stat label="Runtime" value={node.container_runtime || "—"} />
            </div>

            <div className="space-y-2">
              <h2 className="text-sm font-medium">Conditions</h2>
              <div className="overflow-x-auto rounded-xl border">
                <table className="w-full text-sm">
                  <thead>
                    <tr className="border-b text-left text-xs text-muted-foreground">
                      <th className="px-3 py-2">Type</th>
                      <th className="px-3 py-2">Status</th>
                      <th className="px-3 py-2">Reason</th>
                      <th className="px-3 py-2">Message</th>
                    </tr>
                  </thead>
                  <tbody>
                    {(node.conditions || []).map((c) => (
                      <tr key={c.type} className="border-b border-border/60">
                        <td className="px-3 py-1.5 font-medium">{c.type}</td>
                        <td className="px-3 py-1.5">
                          <Badge
                            variant={
                              c.status === "True" && c.type === "Ready"
                                ? "default"
                                : c.status === "True" && c.type !== "Ready"
                                  ? "secondary"
                                  : "outline"
                            }
                          >
                            {c.status}
                          </Badge>
                        </td>
                        <td className="px-3 py-1.5 text-muted-foreground">
                          {c.reason || "—"}
                        </td>
                        <td className="max-w-md truncate px-3 py-1.5 text-muted-foreground">
                          {c.message || "—"}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </div>

            {(node.taints || []).length > 0 ? (
              <div className="space-y-2">
                <h2 className="text-sm font-medium">Taints</h2>
                <div className="flex flex-wrap gap-2">
                  {node.taints!.map((t) => (
                    <Badge key={`${t.key}:${t.effect}`} variant="outline" className="font-mono text-[11px]">
                      {t.key}{t.value ? `=${t.value}` : ""}:{t.effect}
                    </Badge>
                  ))}
                </div>
              </div>
            ) : null}

            <div className="space-y-2">
              <h2 className="text-sm font-medium">Pods on this node</h2>
              <DataTable
                columns={podColumns}
                source={{ type: "client", data: pods }}
                getRowId={(row) => `${row.namespace}/${row.name}`}
                {...k8sClientTableProps}
                initialState={k8sInitialState(20, false)}
              />
            </div>
          </>
        ) : null}
      </div>
    </ContentLoader>
  )
}

function Stat({ label, value }: { label: string; value: string | number }) {
  return (
    <div className="rounded-xl border bg-muted/20 px-4 py-3">
      <p className="text-xs text-muted-foreground">{label}</p>
      <p className="mt-1 truncate font-mono text-sm font-medium" title={String(value)}>
        {value}
      </p>
    </div>
  )
}
