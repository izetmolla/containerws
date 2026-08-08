import { useMemo, useState } from "react"
import { Link, useNavigate } from "react-router"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import {
  createColumnHelper,
  type ColumnDef,
  type RowSelectionState,
} from "@tanstack/react-table"
import { MoreHorizontal, Plus, Search } from "lucide-react"
import { toast } from "sonner"

import ContentLoader from "@/components/content-loader"
import { Button } from "@/components/ui/button"
import { ButtonGroup } from "@/components/ui/button-group"
import { DataTable } from "@/components/ui/data-table"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { Input } from "@/components/ui/input"
import { toastRequestError } from "@/lib/network"

import { EngineBanner } from "../_shared/engine-status"
import { useEngineDown } from "../_shared/use-engine-status"
import { EnvironmentSelector } from "../_shared/environment-selector"
import {
  DOCKER_PAGE_DESCRIPTIONS,
  DockerRefreshButton,
  SummaryChip,
} from "../_shared/page-chrome"
import { selectColumn } from "../_shared/select-column"
import {
  DOCKER_NETWORKS_KEY,
  listNetworks,
  removeNetwork,
  type NetworkRow,
} from "./api"
import { asArray } from "@/lib/as-array"

const columnHelper = createColumnHelper<NetworkRow>()

const PROTECTED_NETWORKS = new Set(["bridge", "host", "none"])

export default function DockerNetworksPage() {
  const navigate = useNavigate()
  const engineDown = useEngineDown()
  const queryClient = useQueryClient()
  const [search, setSearch] = useState("")
  const [rowSelection, setRowSelection] = useState<RowSelectionState>({})
  const [removeTarget, setRemoveTarget] = useState<NetworkRow | null>(null)
  const [bulkRemoveIds, setBulkRemoveIds] = useState<string[] | null>(null)

  const listQuery = useQuery({
    queryKey: [DOCKER_NETWORKS_KEY],
    queryFn: listNetworks,
    refetchInterval: 15_000,
  })

  const invalidate = () =>
    queryClient.invalidateQueries({ queryKey: [DOCKER_NETWORKS_KEY] })

  const removeMutation = useMutation({
    mutationFn: (id: string) => removeNetwork(id),
    onSuccess: (res) => {
      toast.success(res.message || "Network removed")
      setRemoveTarget(null)
      invalidate()
    },
    onError: (err) => toastRequestError(err, "Remove failed"),
  })

  const bulkRemoveMutation = useMutation({
    mutationFn: async (ids: string[]) => {
      const results = await Promise.allSettled(ids.map((id) => removeNetwork(id)))
      const failed = results.filter((r) => r.status === "rejected").length
      return { ok: results.length - failed, failed, total: results.length }
    },
    onSuccess: (res) => {
      if (res.failed === 0) toast.success(`Removed ${res.ok} network(s)`)
      else
        toast.warning(
          `Removed ${res.ok}, ${res.failed} failed of ${res.total}`
        )
      setBulkRemoveIds(null)
      setRowSelection({})
      invalidate()
    },
    onError: (err) => toastRequestError(err, "Bulk remove failed"),
  })

  const rows = asArray(listQuery.data?.data)
  const filtered = useMemo(() => {
    const list = rows
    const q = search.trim().toLowerCase()
    if (!q) return list
    return list.filter(
      (r) =>
        r.name.toLowerCase().includes(q) ||
        r.driver.toLowerCase().includes(q) ||
        r.short_id.includes(q)
    )
  }, [rows, search])

  const selectedIds = useMemo(
    () => Object.keys(rowSelection).filter((id) => rowSelection[id]),
    [rowSelection]
  )
  const removableSelected = useMemo(
    () =>
      filtered.filter(
        (r) => selectedIds.includes(r.id) && !PROTECTED_NETWORKS.has(r.name)
      ),
    [filtered, selectedIds]
  )
  const hasRemovable = removableSelected.length > 0
  const busy = removeMutation.isPending || bulkRemoveMutation.isPending

  const columns = useMemo(
    () =>
      [
        selectColumn(columnHelper, "network"),
        columnHelper.accessor("name", {
          header: "Name",
          enableSorting: true,
          cell: ({ row }) => {
            const text = row.original.name
            return (
              <Link
                to={`/docker/networks/edit?id=${encodeURIComponent(row.original.id)}`}
                title={text}
                className="block max-w-[14rem] truncate font-medium text-sky-600 hover:underline dark:text-sky-400"
              >
                {text}
              </Link>
            )
          },
          meta: { className: "max-w-[14rem]" },
        }),
        columnHelper.accessor("driver", {
          header: "Driver",
          enableSorting: true,
        }),
        columnHelper.accessor("scope", {
          header: "Scope",
          enableSorting: true,
        }),
        columnHelper.accessor("short_id", {
          header: "ID",
          enableSorting: true,
          cell: ({ getValue }) => (
            <span className="font-mono text-xs text-muted-foreground">
              {getValue()}
            </span>
          ),
        }),
        columnHelper.display({
          id: "actions",
          enableSorting: false,
          cell: ({ row }) => (
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button variant="ghost" size="icon-sm">
                  <MoreHorizontal className="size-4" />
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end">
                <DropdownMenuItem
                  onClick={() =>
                    navigate(
                      `/docker/networks/edit?id=${encodeURIComponent(row.original.id)}`
                    )
                  }
                >
                  Open
                </DropdownMenuItem>
                <DropdownMenuSeparator />
                <DropdownMenuItem
                  className="text-destructive"
                  disabled={PROTECTED_NETWORKS.has(row.original.name)}
                  onClick={() => setRemoveTarget(row.original)}
                >
                  Remove
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          ),
          meta: { width: 48, className: "w-12" },
        }),
      ] as ColumnDef<NetworkRow, unknown>[],
    [navigate]
  )

  return (
    <ContentLoader
      title="Networks"
      description={DOCKER_PAGE_DESCRIPTIONS.networks}
      showHeaderSeparator
      breadcrumb={[
        { label: "Docker", to: "/docker" },
        { label: "Networks" },
      ]}
      isLoading={listQuery.isLoading}
      error={engineDown ? undefined : listQuery.error}
      rightComponent={
        <div className="flex flex-wrap items-center gap-2">
          <EnvironmentSelector />
          <DockerRefreshButton
            onClick={() => invalidate()}
            isFetching={listQuery.isFetching}
          />
        </div>
      }
    >
      <div className="flex flex-col gap-4">
        <EngineBanner />
        <DataTable
          columns={columns}
          data={filtered}
          dense
          enableRowSelection
          enableSorting
          enablePagination
          pageSize={10}
          paginationResetKey={search}
          rowSelection={rowSelection}
          onRowSelectionChange={setRowSelection}
          getRowId={(row) => row.id}
          emptyMessage="No networks found. Create a network to isolate services."
          toolbarStart={
            <div className="flex flex-wrap items-center gap-3">
              <div className="relative w-full max-w-xs min-w-[12rem]">
                <Search className="pointer-events-none absolute top-1/2 left-2.5 size-3.5 -translate-y-1/2 text-muted-foreground" />
                <Input
                  className="h-8 pl-8"
                  placeholder="Search networks…"
                  value={search}
                  onChange={(e) => setSearch(e.target.value)}
                />
              </div>
              <SummaryChip>{rows.length} networks</SummaryChip>
              {selectedIds.length ? (
                <span className="text-xs text-muted-foreground">
                  {selectedIds.length} selected
                </span>
              ) : null}
            </div>
          }
          toolbar={
            <div className="flex flex-wrap items-center gap-2">
              <ButtonGroup aria-label="Network actions">
                <Button
                  size="sm"
                  variant="destructive"
                  disabled={!hasRemovable || busy}
                  onClick={() =>
                    setBulkRemoveIds(removableSelected.map((r) => r.id))
                  }
                >
                  Remove
                </Button>
              </ButtonGroup>
              <Button
                size="sm"
                onClick={() => navigate("/docker/networks/edit")}
              >
                <Plus data-icon="inline-start" />
                Create network
              </Button>
            </div>
          }
        />
      </div>

      <Dialog
        open={Boolean(removeTarget)}
        onOpenChange={(v) => !v && setRemoveTarget(null)}
      >
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>Remove network</DialogTitle>
            <DialogDescription>
              Remove network {removeTarget?.name}?
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setRemoveTarget(null)}>
              Cancel
            </Button>
            <Button
              variant="destructive"
              disabled={removeMutation.isPending}
              onClick={() =>
                removeTarget && removeMutation.mutate(removeTarget.id)
              }
            >
              Remove
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog
        open={Boolean(bulkRemoveIds?.length)}
        onOpenChange={(v) => !v && setBulkRemoveIds(null)}
      >
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>Remove networks</DialogTitle>
            <DialogDescription>
              Remove {bulkRemoveIds?.length} selected network(s)? Built-in
              bridge/host/none networks are skipped.
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setBulkRemoveIds(null)}>
              Cancel
            </Button>
            <Button
              variant="destructive"
              disabled={bulkRemoveMutation.isPending}
              onClick={() =>
                bulkRemoveIds && bulkRemoveMutation.mutate(bulkRemoveIds)
              }
            >
              {bulkRemoveMutation.isPending ? "Removing…" : "Remove"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </ContentLoader>
  )
}
