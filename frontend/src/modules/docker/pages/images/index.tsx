import { useMemo, useState } from "react"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import {
  createColumnHelper,
  type ColumnDef,
  type RowSelectionState,
} from "@tanstack/react-table"
import { MoreHorizontal, Plus, Search, Trash2, Download } from "lucide-react"
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
import { ReactSelect } from "@/components/ui/reactselect"
import { Switch } from "@/components/ui/switch"
import { toastRequestError } from "@/lib/network"
import { cn } from "@/lib/utils"

import { EngineBanner } from "../_shared/engine-status"
import { useEngineDown } from "../_shared/use-engine-status"
import { formatBytes } from "../_shared/engine-format"
import { EnvironmentSelector } from "../_shared/environment-selector"
import {
  DOCKER_PAGE_DESCRIPTIONS,
  DockerRefreshButton,
  SummaryChip,
} from "../_shared/page-chrome"
import { selectColumn } from "../_shared/select-column"
import {
  DOCKER_IMAGES_KEY,
  inspectImage,
  listImages,
  pruneImages,
  pullImage,
  removeImage,
  type ImageRow,
} from "./api"
import { asArray } from "@/lib/as-array"

const columnHelper = createColumnHelper<ImageRow>()

type UsageFilter = "all" | "used" | "unused"
type UsageOption = { value: UsageFilter; label: string }

const USAGE_FILTER_OPTIONS: UsageOption[] = [
  { value: "all", label: "All images" },
  { value: "used", label: "Used" },
  { value: "unused", label: "Unused" },
]

function isImageInUse(row: ImageRow) {
  return Boolean(row.in_use || (row.containers ?? 0) > 0)
}

export default function DockerImagesPage() {
  const queryClient = useQueryClient()
  const engineDown = useEngineDown()
  const [search, setSearch] = useState("")
  const [usageFilter, setUsageFilter] = useState<UsageFilter>("all")
  const [rowSelection, setRowSelection] = useState<RowSelectionState>({})
  const [pullOpen, setPullOpen] = useState(false)
  const [image, setImage] = useState("nginx")
  const [tag, setTag] = useState("alpine")
  const [rePull, setRePull] = useState(false)
  const [inspectOpen, setInspectOpen] = useState(false)
  const [inspectJson, setInspectJson] = useState("")
  const [removeTarget, setRemoveTarget] = useState<ImageRow | null>(null)
  const [bulkRemoveIds, setBulkRemoveIds] = useState<string[] | null>(null)

  const listQuery = useQuery({
    queryKey: [DOCKER_IMAGES_KEY],
    queryFn: listImages,
    refetchInterval: 15_000,
  })

  const invalidate = () =>
    queryClient.invalidateQueries({ queryKey: [DOCKER_IMAGES_KEY] })

  const pullMutation = useMutation({
    mutationFn: () =>
      pullImage(image.trim(), tag.trim() || undefined, { force: rePull }),
    onSuccess: (res) => {
      toast.success(res.message || "Image pulled")
      setPullOpen(false)
      setRePull(false)
      invalidate()
    },
    onError: (err) => toastRequestError(err, "Pull failed"),
  })

  const pullRef = useMemo(() => {
    const name = image.trim()
    if (!name) return ""
    const t = tag.trim()
    if (!t || name.includes(":") || name.includes("@")) return name
    return `${name}:${t}`
  }, [image, tag])

  const openPull = () => {
    setRePull(false)
    setPullOpen(true)
  }

  const removeMutation = useMutation({
    mutationFn: (id: string) => removeImage(id, true),
    onSuccess: (res) => {
      toast.success(res.message || "Image removed")
      setRemoveTarget(null)
      invalidate()
    },
    onError: (err) => toastRequestError(err, "Remove failed"),
  })

  const bulkRemoveMutation = useMutation({
    mutationFn: async (ids: string[]) => {
      const results = await Promise.allSettled(
        ids.map((id) => removeImage(id, true))
      )
      const failed = results.filter((r) => r.status === "rejected").length
      return { ok: results.length - failed, failed, total: results.length }
    },
    onSuccess: (res) => {
      if (res.failed === 0) toast.success(`Removed ${res.ok} image(s)`)
      else toast.warning(`Removed ${res.ok}, ${res.failed} failed of ${res.total}`)
      setBulkRemoveIds(null)
      setRowSelection({})
      invalidate()
    },
    onError: (err) => toastRequestError(err, "Bulk remove failed"),
  })

  const pruneMutation = useMutation({
    mutationFn: pruneImages,
    onSuccess: (res) => {
      toast.success(res.message || "Pruned")
      invalidate()
    },
    onError: (err) => toastRequestError(err, "Prune failed"),
  })

  const rows = asArray(listQuery.data?.data)
  const filtered = useMemo(() => {
    const list = rows
    const q = search.trim().toLowerCase()
    return list.filter((r) => {
      if (usageFilter === "used" && !isImageInUse(r)) return false
      if (usageFilter === "unused" && isImageInUse(r)) return false
      if (!q) return true
      return (
        r.short_id.includes(q) ||
        r.repo_tags.some((t) => t.toLowerCase().includes(q))
      )
    })
  }, [rows, search, usageFilter])

  const usageCounts = useMemo(() => {
    const list = rows
    let used = 0
    let unused = 0
    for (const r of list) {
      if (isImageInUse(r)) used += 1
      else unused += 1
    }
    return { used, unused, all: list.length }
  }, [rows])

  const selectedIds = useMemo(
    () => Object.keys(rowSelection).filter((id) => rowSelection[id]),
    [rowSelection]
  )
  const hasSelection = selectedIds.length > 0
  const busy = removeMutation.isPending || bulkRemoveMutation.isPending

  const columns = useMemo(
    () =>
      [
        selectColumn(columnHelper, "image"),
        columnHelper.accessor(
          (row) => row.repo_tags.join(", ") || "<none>",
          {
            id: "tags",
            header: "Tags",
            enableSorting: true,
            cell: ({ getValue }) => {
              const text = getValue()
              return (
                <span
                  title={text}
                  className="block max-w-[20rem] truncate font-mono text-xs"
                >
                  {text}
                </span>
              )
            },
            meta: { className: "max-w-[20rem]" },
          }
        ),
        columnHelper.accessor("short_id", {
          header: "ID",
          enableSorting: true,
          cell: ({ getValue }) => (
            <span className="font-mono text-xs text-muted-foreground">
              {getValue()}
            </span>
          ),
        }),
        columnHelper.accessor(
          (row) => (isImageInUse(row) ? 1 : 0),
          {
            id: "usage",
            header: "Usage",
            enableSorting: true,
            cell: ({ row }) => {
              const used = isImageInUse(row.original)
              const count = row.original.containers ?? 0
              return (
                <span
                  title={
                    used
                      ? `Used by ${count} container${count === 1 ? "" : "s"}`
                      : "Not used by any container"
                  }
                  className={cn(
                    "inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[11px] font-medium",
                    used
                      ? "bg-emerald-500/15 text-emerald-700 dark:text-emerald-300"
                      : "bg-muted text-muted-foreground"
                  )}
                >
                  {used ? "Used" : "Unused"}
                  {used && count > 0 ? (
                    <span className="opacity-70">· {count}</span>
                  ) : null}
                </span>
              )
            },
          }
        ),
        columnHelper.accessor("size", {
          header: "Size",
          enableSorting: true,
          cell: ({ getValue }) => formatBytes(getValue()),
        }),
        columnHelper.accessor("created", {
          header: "Created",
          enableSorting: true,
          cell: ({ getValue }) => {
            const ts = getValue()
            if (!ts) return "—"
            return (
              <span className="whitespace-nowrap font-mono text-xs text-muted-foreground">
                {new Date(ts * 1000).toLocaleString()}
              </span>
            )
          },
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
                      const res = await inspectImage(row.original.id)
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
      ] as ColumnDef<ImageRow, unknown>[],
    []
  )

  return (
    <ContentLoader
      title="Images"
      description={DOCKER_PAGE_DESCRIPTIONS.images}
      showHeaderSeparator
      breadcrumb={[
        { label: "Docker", to: "/docker" },
        { label: "Images" },
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
          paginationResetKey={`${search}:${usageFilter}`}
          rowSelection={rowSelection}
          onRowSelectionChange={setRowSelection}
          getRowId={(row) => row.id}
          emptyMessage="No images found. Pull an image to get started."
          toolbarStart={
            <div className="flex flex-wrap items-center gap-3">
              <div className="relative w-full max-w-xs min-w-[12rem]">
                <Search className="pointer-events-none absolute top-1/2 left-2.5 size-3.5 -translate-y-1/2 text-muted-foreground" />
                <Input
                  className="h-8 pl-8"
                  placeholder="Search images…"
                  value={search}
                  onChange={(e) => setSearch(e.target.value)}
                />
              </div>
              <div className="w-[10.5rem]">
                <ReactSelect<UsageOption, false>
                  size="sm"
                  options={USAGE_FILTER_OPTIONS}
                  value={usageFilter}
                  onValueChange={(v) => v && setUsageFilter(v)}
                  isSearchable={false}
                />
              </div>
              <div className="flex flex-wrap items-center gap-1.5">
                <SummaryChip
                  active={usageFilter === "used"}
                  onClick={() => setUsageFilter("used")}
                >
                  {usageCounts.used} used
                </SummaryChip>
                <SummaryChip
                  active={usageFilter === "unused"}
                  onClick={() => setUsageFilter("unused")}
                >
                  {usageCounts.unused} unused
                </SummaryChip>
              </div>
              {hasSelection ? (
                <span className="text-xs text-muted-foreground">
                  {selectedIds.length} selected
                </span>
              ) : null}
            </div>
          }
          toolbar={
            <div className="flex flex-wrap items-center gap-2">
              <ButtonGroup aria-label="Image actions">
                <Button
                  size="sm"
                  variant="destructive"
                  disabled={!hasSelection || busy}
                  onClick={() => setBulkRemoveIds(selectedIds)}
                >
                  Remove
                </Button>
                <Button
                  size="sm"
                  variant="outline"
                  disabled={pruneMutation.isPending}
                  onClick={() => pruneMutation.mutate()}
                >
                  <Trash2 data-icon="inline-start" />
                  Prune unused
                </Button>
              </ButtonGroup>
              <Button size="sm" onClick={openPull}>
                <Plus data-icon="inline-start" />
                Pull image
              </Button>
            </div>
          }
        />
      </div>

      <Dialog
        open={pullOpen}
        onOpenChange={(v) => {
          if (!v) {
            setPullOpen(false)
            setRePull(false)
          }
        }}
      >
        <DialogContent className="flex max-h-[90vh] flex-col gap-0 overflow-hidden p-0 sm:max-w-lg">
          <DialogHeader className="border-b px-6 py-4">
            <DialogTitle className="flex items-center gap-2">
              <span className="flex size-8 items-center justify-center rounded-md bg-muted">
                <Download className="size-4 text-muted-foreground" />
              </span>
              Pull image
            </DialogTitle>
            <DialogDescription>
              Download an image from a registry into the selected Docker
              environment. Leave re-pull off to skip when the tag is already
              local.
            </DialogDescription>
          </DialogHeader>

          <div className="min-h-0 flex-1 space-y-5 overflow-y-auto px-6 py-5">
            <div className="grid gap-4 sm:grid-cols-[1fr_8rem]">
              <div className="grid gap-1.5">
                <Label htmlFor="pull-image">Repository</Label>
                <Input
                  id="pull-image"
                  value={image}
                  onChange={(e) => setImage(e.target.value)}
                  placeholder="nginx"
                  className="font-mono text-sm"
                  autoFocus
                  onKeyDown={(e) => {
                    if (e.key === "Enter" && image.trim() && !pullMutation.isPending) {
                      pullMutation.mutate()
                    }
                  }}
                />
                <p className="text-xs text-muted-foreground">
                  Name only, or full ref like{" "}
                  <span className="font-mono">ghcr.io/org/app</span>
                </p>
              </div>
              <div className="grid gap-1.5">
                <Label htmlFor="pull-tag">Tag</Label>
                <Input
                  id="pull-tag"
                  value={tag}
                  onChange={(e) => setTag(e.target.value)}
                  placeholder="latest"
                  className="font-mono text-sm"
                  onKeyDown={(e) => {
                    if (e.key === "Enter" && image.trim() && !pullMutation.isPending) {
                      pullMutation.mutate()
                    }
                  }}
                />
              </div>
            </div>

            {pullRef ? (
              <div className="rounded-lg border bg-muted/40 px-3 py-2.5">
                <p className="text-[11px] font-medium tracking-wide text-muted-foreground uppercase">
                  Resolved reference
                </p>
                <p className="mt-1 break-all font-mono text-sm">{pullRef}</p>
              </div>
            ) : null}

            <div
              className={cn(
                "flex items-start justify-between gap-4 rounded-lg border px-3 py-3 transition-colors",
                rePull ? "border-primary/30 bg-primary/5" : "bg-background"
              )}
            >
              <div className="min-w-0 space-y-1">
                <Label htmlFor="pull-force" className="cursor-pointer">
                  Re-pull from registry
                </Label>
                <p className="text-xs leading-relaxed text-muted-foreground">
                  Always contact the registry and refresh this tag, even if the
                  image already exists locally.
                </p>
              </div>
              <Switch
                id="pull-force"
                checked={rePull}
                onCheckedChange={setRePull}
                className="mt-0.5"
              />
            </div>
          </div>

          <DialogFooter className="border-t px-6 py-4">
            <Button
              variant="outline"
              onClick={() => {
                setPullOpen(false)
                setRePull(false)
              }}
              disabled={pullMutation.isPending}
            >
              Cancel
            </Button>
            <Button
              disabled={pullMutation.isPending || !image.trim()}
              onClick={() => pullMutation.mutate()}
            >
              <Download data-icon="inline-start" />
              {pullMutation.isPending
                ? rePull
                  ? "Re-pulling…"
                  : "Pulling…"
                : rePull
                  ? "Re-pull image"
                  : "Pull image"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={inspectOpen} onOpenChange={setInspectOpen}>
        <DialogContent className="sm:max-w-2xl">
          <DialogHeader>
            <DialogTitle>Inspect image</DialogTitle>
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
            <DialogTitle>Remove image</DialogTitle>
            <DialogDescription>
              Remove {removeTarget?.repo_tags?.[0] || removeTarget?.short_id}?
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
            <DialogTitle>Remove images</DialogTitle>
            <DialogDescription>
              Force-remove {bulkRemoveIds?.length} selected image(s)?
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
