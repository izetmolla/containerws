import { useQuery } from "@tanstack/react-query";
import { createColumnHelper, type ColumnDef } from "@tanstack/react-table";
import { Link, useOutletContext } from "react-router";

import { DataTable } from "@/components/datatable";
import { asArray } from "@/lib/as-array";

import {
  formatAge,
  K8S_DAEMONSETS_KEY,
  listDaemonSetPods,
  type PodRow,
} from "../../_shared/api";
import {
  k8sClientTableProps,
  k8sInitialState,
  k8sMultiSelectFilter,
  k8sNameFilter,
  k8sSortable,
  optionsFromValues,
} from "../../_shared/client-table";
import type { DaemonSetOutletContext } from "./layout";

const helper = createColumnHelper<PodRow>();

export default function DaemonSetPodsPage() {
  const { namespace, name } = useOutletContext<DaemonSetOutletContext>();
  const query = useQuery({
    queryKey: [K8S_DAEMONSETS_KEY, namespace, name, "pods"],
    queryFn: () => listDaemonSetPods(namespace, name),
    refetchInterval: 8_000,
  });
  const rows = asArray(query.data?.data);
  const columns = [
    helper.accessor("name", {
      ...k8sNameFilter(),
      cell: ({ row, getValue }) => (
        <Link
          className="font-medium hover:underline"
          to={`/kubernetes/pods/${encodeURIComponent(row.original.namespace)}/${encodeURIComponent(row.original.name)}`}
        >
          {getValue()}
        </Link>
      ),
    }),
    helper.accessor("status", {
      ...k8sMultiSelectFilter(
        "Status",
        optionsFromValues(rows.map((row) => row.status)),
      ),
    }),
    helper.accessor("ready", { ...k8sSortable("Ready") }),
    helper.accessor("restarts", { ...k8sSortable("Restarts") }),
    helper.accessor("node", { ...k8sSortable("Node") }),
    helper.accessor("ip", { ...k8sSortable("IP") }),
    helper.accessor("created_at", {
      ...k8sSortable("Age"),
      cell: ({ getValue }) => formatAge(getValue()),
    }),
  ] as ColumnDef<PodRow, unknown>[];
  return (
    <DataTable
      columns={columns}
      source={{ type: "client", data: rows }}
      getRowId={(row) => `${row.namespace}/${row.name}`}
      {...k8sClientTableProps}
      initialState={k8sInitialState(20)}
    />
  );
}
