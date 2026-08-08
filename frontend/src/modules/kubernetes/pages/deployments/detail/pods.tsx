import { useQuery } from "@tanstack/react-query"
import { createColumnHelper, type ColumnDef } from "@tanstack/react-table"
import { Link, useOutletContext } from "react-router"

import { DataTable } from "@/components/datatable"
import { asArray } from "@/lib/as-array"

import {
  formatAge, K8S_DEPLOYMENTS_KEY, listDeploymentPods, type PodRow,
} from "../../_shared/api"
import { k8sNameFilter, k8sClientTableProps, k8sInitialState, k8sMultiSelectFilter, k8sSortable, optionsFromValues } from "../../_shared/client-table"
import type { DeploymentOutletContext } from "./layout"

const helper = createColumnHelper<PodRow>()

export default function DeploymentPodsPage() {
  const { namespace, name } = useOutletContext<DeploymentOutletContext>()
  const query = useQuery({
    queryKey: [K8S_DEPLOYMENTS_KEY, namespace, name, "pods"],
    queryFn: () => listDeploymentPods(namespace, name),
    refetchInterval: 8_000,
  })
  const rows = asArray(query.data?.data)
  const statusOptions = optionsFromValues(rows.map((r) => r.status))
  const columns = [
    helper.accessor("name", {
      ...k8sNameFilter(),
      cell: ({ row, getValue }) => (
        <Link className="font-medium hover:underline" to={`/kubernetes/pods/${encodeURIComponent(row.original.namespace)}/${encodeURIComponent(row.original.name)}`}>
          {getValue()}
        </Link>
      ),
    }),
    helper.accessor("status", { ...k8sMultiSelectFilter("Status", statusOptions) }),
    helper.accessor("ready", { ...k8sSortable("Ready") }),
    helper.accessor("restarts", { ...k8sSortable("Restarts") }),
    helper.accessor("ip", { ...k8sSortable("IP") }),
    helper.accessor("created_at", { ...k8sSortable("Age"), cell: ({ getValue }) => formatAge(getValue()) }),
  ] as ColumnDef<PodRow, unknown>[]
  return <DataTable
      columns={columns}
      source={{ type: "client", data: rows }}
      getRowId={(row) => `${row.namespace}/${row.name}`}
      {...k8sClientTableProps}
      initialState={k8sInitialState(20)}
    />
}
