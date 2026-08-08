import { useMemo, useState } from "react"
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
import { Label } from "@/components/ui/label"
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
  createVolume,
  DOCKER_VOLUMES_KEY,
  inspectVolume,
  listVolumes,
  removeVolume,
  type VolumeRow,
} from "./api"
import { asArray } from "@/lib/as-array"

const columnHelper = createColumnHelper<VolumeRow>()

export default function DockerVolumesPage() {
  const queryClient = useQueryClient()
  const engineDown = useEngineDown()
  const [search, setSearch] = useState("")
  const [rowSelection, setRowSelection] = useState<RowSelectionState>({})
  const [createOpen, setCreateOpen] = useState(false)
  const [name, setName] = useState("")
  const [driver, setDriver] = useState("local")
  const [inspectOpen, setInspectOpen] = useState(false)
  const [inspectJson, setInspectJson] = useState("")
  const [removeTarget, setRemoveTarget] = useState<VolumeRow | null>(null)
  const [bulkRemoveIds, setBulkRemoveIds] = useState<string[] | null>(null)

  const listQuery = useQuery({
    queryKey: [DOCKER_VOLUMES_KEY],
    queryFn: listVolumes,
    refetchInterval: 15_000,
  })

  const invalidate = () =>
    queryClient.invalidateQueries({ queryKey: [DOCKER_VOLUMES_KEY] })

  const createMutation = useMutation({
    mutationFn: () =>
      createVolume({
        name: name.trim() || undefined,
        driver: driver.trim() || "local",
      }),
    onSuccess: (res) => {
      toast.success(res.message || "Volume created")
      setCreateOpen(false)
      setName("")
      invalidate()
    },
    onError: (err) => toastRequestError(err, "Create failed"),
  })

  const removeMutation = useMutation({
    mutationFn: (n: string) => removeVolume(n, true),
    onSuccess: (res) => {
      toast.success(res.message || "Volume removed")
      setRemoveTarget(null)
      invalidate()
    },
    onError: (err) => toastRequestError(err, "Remove failed"),
  })

  const bulkRemoveMutation = useMutation({
    mutationFn: async (ids: string[]) => {
      const results = await Promise.allSettled(
        ids.map((id) => removeVolume(id, true))
      )
      const failed = results.filter((r) => r.status === "rejected").length
      return { ok: results.length - failed, failed, total: results.length }
    },
    onSuccess: (res) => {
      if (res.failed === 0) toast.success(`Removed ${res.ok} volume(s)`)
      else toast.warning(`Removed ${res.ok}, ${res.failed} failed of ${res.total}`)
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
        r.mountpoint.toLowerCase().includes(q)
    )
  }, [rows, search])

  const selectedIds = useMemo(
    () => Object.keys(rowSelection).filter((id) => rowSelection[id]),
    [rowSelection]
  )
  const hasSelection = selectedIds.length > 0
  const busy = removeMutation.isPending || bulkRemoveMutation.isPending

  const columns = useMemo(
    () =>
      [
        selectColumn(columnHelper, "volume"),
        columnHelper.accessor("name", {
          header: "Name",
          enableSorting: true,
          cell: ({ getValue }) => {
            const text = getValue()
            return (
              <span title={text} className="block max-w-[14rem] truncate font-medium">
                {text}
              </span>
            )
          },
          meta: { className: "max-w-[14rem]" },
        }),
        columnHelper.accessor("driver", {
          header: "Driver",
          enableSorting: true,
        }),
        columnHelper.accessor("mountpoint", {
          header: "Mountpoint",
          enableSorting: true,
          cell: ({ getValue }) => {
            const text = getValue()
            return (
              <span
                title={text}
                className="block max-w-[18rem] truncate font-mono text-xs text-muted-foreground"
              >
                {text}
              </span>
            )
          },
          meta: { className: "max-w-[18rem]" },
        }),
        columnHelper.accessor("scope", {
          header: "Scope",
          enableSorting: true,
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
                  onClick={async () => {
                    try {
                      const res = await inspectVolume(row.original.name)
                      setInspectJson(JSON.stringify(res.data, null, 2))
                      setInspectOpen(true)
                    } catch (err) {
                      toastRequestError(err, "Inspect failed")
                    }
                  }}
                >
                  Inspect
                </DropdownMenuItem>
                <DropdownMenuSeparator />
                <DropdownMenuItem
                  className="text-destructive"
                  onClick={() => setRemoveTarget(row.original)}
                >
                  Remove
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          ),
          meta: { width: 48, className: "w-12" },
        }),
      ] as ColumnDef<VolumeRow, unknown>[],
    []
  )

  return (
    <ContentLoader
      title="Volumes"
      description={DOCKER_PAGE_DESCRIPTIONS.volumes}
      showHeaderSeparator
      breadcrumb={[
        { label: "Docker", to: "/docker" },
        { label: "Volumes" },
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
          getRowId={(row) => row.name}
          emptyMessage="No volumes found. Create a volume to persist container data."
          toolbarStart={
            <div className="flex flex-wrap items-center gap-3">
              <div className="relative w-full max-w-xs min-w-[12rem]">
                <Search className="pointer-events-none absolute top-1/2 left-2.5 size-3.5 -translate-y-1/2 text-muted-foreground" />
                <Input
                  className="h-8 pl-8"
                  placeholder="Search volumes…"
                  value={search}
                  onChange={(e) => setSearch(e.target.value)}
                />
              </div>
              <SummaryChip>{rows.length} volumes</SummaryChip>
              {hasSelection ? (
                <span className="text-xs text-muted-foreground">
                  {selectedIds.length} selected
                </span>
              ) : null}
            </div>
          }
          toolbar={
            <div className="flex flex-wrap items-center gap-2">
              <ButtonGroup aria-label="Volume actions">
                <Button
                  size="sm"
                  variant="destructive"
                  disabled={!hasSelection || busy}
                  onClick={() => setBulkRemoveIds(selectedIds)}
                >
                  Remove
                </Button>
              </ButtonGroup>
              <Button size="sm" onClick={() => setCreateOpen(true)}>
                <Plus data-icon="inline-start" />
                Create volume
              </Button>
            </div>
          }
        />
      </div>

      <Dialog open={createOpen} onOpenChange={setCreateOpen}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>Create volume</DialogTitle>
            <DialogDescription>
              Create a Docker volume. Leave name empty to auto-generate.
            </DialogDescription>
          </DialogHeader>
          <div className="grid gap-3 py-2">
            <div className="grid gap-1.5">
              <Label>Name</Label>
              <Input
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="optional"
              />
            </div>
            <div className="grid gap-1.5">
              <Label>Driver</Label>
              <Input
                value={driver}
                onChange={(e) => setDriver(e.target.value)}
                placeholder="local"
              />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setCreateOpen(false)}>
              Cancel
            </Button>
            <Button
              disabled={createMutation.isPending}
              onClick={() => createMutation.mutate()}
            >
              {createMutation.isPending ? "Creating…" : "Create"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={inspectOpen} onOpenChange={setInspectOpen}>
        <DialogContent className="sm:max-w-2xl">
          <DialogHeader>
            <DialogTitle>Inspect volume</DialogTitle>
          </DialogHeader>
          <pre className="max-h-[60vh] overflow-auto rounded-md border bg-muted/30 p-3 font-mono text-xs">
            {inspectJson}
          </pre>
        </DialogContent>
      </Dialog>

      <Dialog
        open={Boolean(removeTarget)}
        onOpenChange={(v) => !v && setRemoveTarget(null)}
      >
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>Remove volume</DialogTitle>
            <DialogDescription>
              Remove volume {removeTarget?.name}?
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
                removeTarget && removeMutation.mutate(removeTarget.name)
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
            <DialogTitle>Remove volumes</DialogTitle>
            <DialogDescription>
              Remove {bulkRemoveIds?.length} selected volume(s)?
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
