import { useMemo } from "react"
import { Link } from "react-router"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import {
  createColumnHelper,
  type ColumnDef,
} from "@tanstack/react-table"
import {
  ArrowLeft,
  Loader2,
  Package,
  RefreshCw,
  RotateCcw,
} from "lucide-react"
import { toast } from "sonner"

import ContentLoader from "@/components/content-loader"
import { Button } from "@/components/ui/button"
import { DataTable } from "@/components/ui/data-table"
import { getRequestErrorMessage, withError } from "@/lib/network"
import { cn } from "@/lib/utils"

import { SoftwareGlyph } from "../../components/software-glyph"
import {
  getSoftwareQueue,
  retrySoftwareQueueItem,
  SOFTWARES_FETCH_KEY,
  type SoftwareQueueItem,
} from "../list/api"

const columnHelper = createColumnHelper<SoftwareQueueItem>()

function actionLabel(action: string) {
  switch (action) {
    case "update":
      return "Update"
    case "uninstall":
      return "Uninstall"
    default:
      return "Install"
  }
}

function InstallingActionCell({
  item,
  onRetry,
  retrying,
}: {
  item: SoftwareQueueItem
  onRetry: (item: SoftwareQueueItem) => void
  retrying: boolean
}) {
  if (item.status === "error") {
    return (
      <Button
        size="sm"
        variant="outline"
        className="min-w-[7.5rem] border-destructive/40 text-destructive hover:bg-destructive/10 hover:text-destructive"
        disabled={retrying}
        onClick={() => onRetry(item)}
      >
        {retrying ? (
          <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" />
        ) : (
          <RotateCcw className="mr-1.5 h-3.5 w-3.5" />
        )}
        Retry
      </Button>
    )
  }

  const pending = item.status === "pending"
  return (
    <Button
      size="sm"
      disabled
      className={cn(
        "min-w-[7.5rem] cursor-default border-transparent",
        "bg-sky-600 text-white shadow-sm",
        "disabled:opacity-100",
        !pending && "animate-pulse"
      )}
    >
      <Loader2
        className={cn(
          "mr-1.5 h-3.5 w-3.5",
          pending ? "animate-spin opacity-70" : "animate-spin"
        )}
      />
      {pending ? "Queued" : "Installing"}
    </Button>
  )
}

export default function SoftwaresInstallingPage() {
  const queryClient = useQueryClient()

  const queueQuery = useQuery({
    queryKey: [SOFTWARES_FETCH_KEY, "queue"],
    queryFn: getSoftwareQueue,
    refetchInterval: (q) => {
      const items = q.state.data?.data?.items ?? []
      const busy = items.some(
        (i) => i.status === "pending" || i.status === "running"
      )
      return busy || Boolean(q.state.data?.data?.running) ? 1500 : 4000
    },
  })

  const retryMutation = useMutation({
    mutationFn: (item: SoftwareQueueItem) =>
      retrySoftwareQueueItem({
        id: item.id.startsWith("job-") ? undefined : item.id,
        softwareId: item.software_id,
        action: (item.action as "install" | "update" | "uninstall") || "install",
      }),
    onSuccess: (res) => {
      toast.success(res.message || "Retry queued")
      void queryClient.invalidateQueries({
        queryKey: [SOFTWARES_FETCH_KEY, "queue"],
      })
      void queryClient.invalidateQueries({
        queryKey: [SOFTWARES_FETCH_KEY, "list"],
      })
    },
    onError: (err) => {
      toast.error(getRequestErrorMessage(err, "Retry failed"))
    },
  })

  const items = queueQuery.data?.data?.items ?? []
  const pendingCount = queueQuery.data?.data?.pending ?? 0

  const columns = useMemo(() => {
    return [
      columnHelper.display({
        id: "software",
        header: "Software",
        cell: ({ row }) => {
          const item = row.original
          const accent = item.color || "var(--primary)"
          const subtitle =
            [item.category, item.version ? `v${item.version}` : ""]
              .filter(Boolean)
              .join(" · ") || actionLabel(item.action)
          return (
            <div className="flex min-w-0 items-center gap-3 py-0.5">
              {item.image?.trim() ? (
                <div className="flex h-9 w-9 shrink-0 items-center justify-center overflow-hidden rounded-lg border border-border/60 bg-background shadow-sm">
                  <SoftwareGlyph
                    name={item.icon}
                    image={item.image}
                    className="h-4 w-4"
                    imgClassName="h-9 w-9 object-cover"
                  />
                </div>
              ) : (
                <div
                  className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg text-white shadow-sm"
                  style={{ backgroundColor: accent }}
                >
                  <SoftwareGlyph name={item.icon} className="h-4 w-4" />
                </div>
              )}
              <div className="min-w-0">
                <Link
                  to={`/softwares/${item.software_id}`}
                  className="block truncate font-medium hover:underline"
                >
                  {item.software_name || item.software_id}
                </Link>
                <p className="truncate text-xs text-muted-foreground">
                  {subtitle}
                </p>
              </div>
            </div>
          )
        },
      }),
      columnHelper.accessor("action", {
        header: "Action",
        cell: ({ getValue }) => (
          <span className="text-sm text-muted-foreground">
            {actionLabel(String(getValue() || "install"))}
          </span>
        ),
      }),
      columnHelper.accessor("message", {
        header: "Status",
        cell: ({ row }) => {
          const item = row.original
          const statusText =
            item.message ||
            (item.status === "pending"
              ? "Waiting in queue..."
              : item.status === "running"
                ? "Running..."
                : item.status)
          return (
            <p
              className={cn(
                "max-w-md truncate text-sm",
                item.status === "error"
                  ? "text-destructive"
                  : "text-muted-foreground"
              )}
              title={item.message || undefined}
            >
              {statusText}
            </p>
          )
        },
      }),
      columnHelper.display({
        id: "progress",
        header: () => <span className="sr-only">Progress</span>,
        meta: { align: "right" as const },
        cell: ({ row }) => (
          <div className="flex justify-end">
            <InstallingActionCell
              item={row.original}
              onRetry={(item) => retryMutation.mutate(item)}
              retrying={
                retryMutation.isPending &&
                retryMutation.variables?.id === row.original.id
              }
            />
          </div>
        ),
      }),
    ] as ColumnDef<SoftwareQueueItem, unknown>[]
  }, [retryMutation])

  return (
    <ContentLoader
      title="Installing"
      breadcrumb={[
        { label: "Softwares", to: "/softwares" },
        { label: "Installing", to: "/softwares/installing" },
      ]}
      isLoading={queueQuery.isLoading}
      error={withError(queueQuery.error, queueQuery.data)}
      showHeaderSeparator
      customTitle={
        <div className="flex items-center gap-3">
          <div className="flex h-11 w-11 items-center justify-center rounded-xl bg-sky-600/15 text-sky-700 dark:text-sky-300">
            <Package className="h-5 w-5" />
          </div>
          <div>
            <h1 className="text-2xl font-semibold tracking-tight">Installing</h1>
            <p className="text-sm text-muted-foreground">
              {pendingCount > 0
                ? `${pendingCount} in progress · updates every few seconds`
                : items.length > 0
                  ? `${items.length} item${items.length === 1 ? "" : "s"} need attention`
                  : "Queue is empty"}
            </p>
          </div>
        </div>
      }
      rightComponent={
        <div className="flex items-center gap-2">
          <Button
            variant="outline"
            size="sm"
            onClick={() => void queueQuery.refetch()}
            disabled={queueQuery.isFetching}
          >
            <RefreshCw
              className={cn(
                "mr-1.5 h-3.5 w-3.5",
                queueQuery.isFetching && "animate-spin"
              )}
            />
            Refresh
          </Button>
          <Button variant="ghost" size="sm" asChild>
            <Link to="/softwares">
              <ArrowLeft className="mr-1.5 h-3.5 w-3.5" />
              Catalog
            </Link>
          </Button>
        </div>
      }
    >
      <DataTable
        columns={columns}
        data={items}
        emptyMessage="No installs in progress. Queue software from the catalog to see them here."
      />
    </ContentLoader>
  )
}
