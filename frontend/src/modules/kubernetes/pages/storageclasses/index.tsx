import { useQuery, useQueryClient } from "@tanstack/react-query"
import { createColumnHelper, type ColumnDef } from "@tanstack/react-table"
import { RefreshCw } from "lucide-react"

import ContentLoader from "@/components/content-loader"
import { DataTable } from "@/components/datatable"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { asArray } from "@/lib/as-array"

import {
  K8S_STORAGE_CLASSES_KEY,
  listStorageClasses,
  type StorageClassRow,
} from "../_shared/api"
import {
  k8sInitialState,
  k8sNameFilter,
  k8sSortable,
  k8sTextFilter,
} from "../_shared/client-table"
import { ClusterBanner } from "../_shared/cluster-banner"

const helper = createColumnHelper<StorageClassRow>()

export default function StorageClassesPage() {
  const queryClient = useQueryClient()
  const query = useQuery({
    queryKey: [K8S_STORAGE_CLASSES_KEY],
    queryFn: listStorageClasses,
    refetchInterval: 30_000,
  })
  const invalidate = () =>
    queryClient.invalidateQueries({ queryKey: [K8S_STORAGE_CLASSES_KEY] })
  const rows = asArray(query.data?.data)

  const columns = [
    helper.accessor("name", {
      ...k8sNameFilter(),
      cell: ({ row }) => (
        <div className="flex items-center gap-2">
          <span className="font-medium">{row.original.name}</span>
          {row.original.default ? (
            <Badge variant="secondary" className="text-[10px]">
              default
            </Badge>
          ) : null}
        </div>
      ),
    }),
    helper.accessor("provisioner", {
      ...k8sTextFilter("Provisioner", "Filter provisioner…"),
      cell: ({ getValue }) => (
        <span className="font-mono text-xs text-muted-foreground">
          {getValue()}
        </span>
      ),
    }),
    helper.accessor("reclaim_policy", {
      ...k8sSortable("Reclaim"),
      cell: ({ getValue }) => getValue() || "—",
    }),
    helper.accessor("volume_binding_mode", {
      ...k8sSortable("Binding"),
      cell: ({ getValue }) => getValue() || "—",
    }),
    helper.accessor("allow_volume_expansion", {
      ...k8sSortable("Expandable"),
      cell: ({ getValue }) => (getValue() ? "Yes" : "No"),
    }),
  ] as ColumnDef<StorageClassRow, unknown>[]

  return (
    <ContentLoader
      title="StorageClasses"
      breadcrumb={[
        { label: "Kubernetes", to: "/kubernetes" },
        { label: "StorageClasses" },
      ]}
      isLoading={query.isLoading}
      error={query.error}
      rightComponent={
        <Button size="sm" variant="outline" onClick={() => invalidate()}>
          <RefreshCw className="size-3.5" />
        </Button>
      }
    >
      <div className="space-y-4">
        <ClusterBanner />
        <p className="text-sm text-muted-foreground">
          Cluster storage provisioners available for PersistentVolumeClaims.
        </p>
        <DataTable
          columns={columns}
          source={{ type: "client", data: rows }}
          getRowId={(row) => row.name}
          initialState={k8sInitialState(20)}
        />
      </div>
    </ContentLoader>
  )
}
