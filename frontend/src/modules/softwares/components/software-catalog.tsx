import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { ReactSelect } from "@/components/ui/reactselect"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import {
  ArrowUpCircle,
  ChevronLeft,
  ChevronRight,
  Download,
  LayoutGrid,
  List,
  Search,
  Trash2,
  X,
} from "lucide-react"
import { useMemo, useState } from "react"
import { createPortal } from "react-dom"
import { toast } from "sonner"
import type {
  SoftwaresListFacets,
  SoftwaresListParams,
  SoftwareListItem,
  SoftwareServiceAction,
} from "../pages/list/api"
import { SoftwareCard } from "./software-card"
import { SoftwareList } from "./software-list"
import { asArray } from "@/lib/as-array"

// Catalog URL filters omit "uninstalled" (item status only; not a list facet).
type StatusFilter = Exclude<
  NonNullable<SoftwaresListParams["status"]>,
  "uninstalled"
>
type SortKey = NonNullable<SoftwaresListParams["sort"]>
type SourceFilter = NonNullable<SoftwaresListParams["source"]>
type CategoryOption = { value: string; label: string }

export type SoftwareCatalogFilters = {
  q: string
  category: string
  status: StatusFilter
  sort: SortKey
  source: SourceFilter
  page: number
  limit: number
}

export function SoftwareCatalog({
  items,
  facets,
  filters,
  pagination,
  isFetching,
  onFiltersChange,
  onOpen,
  onUpdate,
  onBulkAction,
  bulkBusy,
  onServiceAction,
  busyAction,
  queueBusyById,
}: {
  items: SoftwareListItem[]
  facets?: SoftwaresListFacets | null
  filters: SoftwareCatalogFilters
  pagination: {
    page: number
    limit: number
    total: number
    total_pages: number
  }
  isFetching?: boolean
  onFiltersChange: (patch: Partial<SoftwareCatalogFilters>) => void
  onOpen: (s: SoftwareListItem) => void
  onUpdate: (s: SoftwareListItem) => void
  onBulkAction?: (
    action: "install" | "update" | "uninstall",
    items: SoftwareListItem[]
  ) => void
  bulkBusy?: boolean
  onServiceAction?: (id: string, action: SoftwareServiceAction) => void
  busyAction?: string | null
  queueBusyById?: Map<string, "installing" | "updating">
}) {
  const [view, setView] = useState<"grid" | "list">(() => {
    if (typeof window === "undefined") return "grid"
    return (localStorage.getItem("sw-view") as "grid" | "list") ?? "grid"
  })
  const [selectedIds, setSelectedIds] = useState<Set<string>>(() => new Set())
  const itemsKey = items.map((i) => i.id).join("\0")
  const [prevItemsKey, setPrevItemsKey] = useState(itemsKey)

  const setViewPersist = (v: "grid" | "list") => {
    setView(v)
    if (typeof window !== "undefined") localStorage.setItem("sw-view", v)
    if (v === "grid") setSelectedIds(new Set())
  }

  // Drop selections that left the current page / filter set.
  if (itemsKey !== prevItemsKey) {
    setPrevItemsKey(itemsKey)
    const visible = new Set(items.map((i) => i.id))
    setSelectedIds((prev) => {
      let changed = false
      const next = new Set<string>()
      for (const id of prev) {
        if (visible.has(id)) next.add(id)
        else changed = true
      }
      return changed || next.size !== prev.size ? next : prev
    })
  }

  const updateCount = facets?.update_count ?? 0
  const categories = asArray(facets?.categories)
  const totalActive = facets?.total_active ?? pagination.total

  const categoryOptions = useMemo<CategoryOption[]>(
    () => {
      const list = categories
      return [
        { value: "all", label: `All categories (${totalActive})` },
        ...list.map((c) => ({
          value: c.name,
          label: `${c.name} (${c.count})`,
        })),
      ]
    },
    [categories, totalActive]
  )

  const clearFilters = () => {
    onFiltersChange({
      q: "",
      category: "all",
      status: "all",
      source: "local",
      page: 1,
    })
  }

  const filtersActive =
    filters.q !== "" ||
    filters.category !== "all" ||
    filters.status !== "all" ||
    filters.source !== "local"

  const totalPages = Math.max(1, pagination.total_pages || 1)
  const page = Math.min(Math.max(1, pagination.page || 1), totalPages)

  const allSelected =
    items.length > 0 && items.every((item) => selectedIds.has(item.id))
  const someSelected = items.some((item) => selectedIds.has(item.id))

  const toggleSelected = (id: string, next?: boolean) => {
    setSelectedIds((prev) => {
      const checked = next ?? !prev.has(id)
      const copy = new Set(prev)
      if (checked) copy.add(id)
      else copy.delete(id)
      return copy
    })
  }

  const toggleSelectAll = () => {
    if (allSelected) {
      setSelectedIds(new Set())
      return
    }
    setSelectedIds(new Set(items.map((i) => i.id)))
  }

  const selectedItems = items.filter((i) => selectedIds.has(i.id))
  const selectedNotInstalled = selectedItems.filter((i) => !i.is_installed)
  const selectedInstalled = selectedItems.filter((i) => i.is_installed)
  const selectedRemovable = selectedInstalled.filter(
    (i) => i.can_uninstall && i.package_manager !== "brew"
  )
  const selectedWithUpdate = selectedItems.filter((i) => i.has_update)

  return (
    <div className="space-y-6">
      <div className="space-y-4">
        <div className="flex flex-wrap items-center gap-2">
          <div className="relative min-w-[240px] flex-1">
            <Search className="pointer-events-none absolute left-2.5 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
            <Input
              value={filters.q}
              onChange={(e) =>
                onFiltersChange({ q: e.target.value, page: 1 })
              }
              placeholder="Search software…"
              className="pl-8"
            />
          </div>

          <Select
            value={filters.status}
            onValueChange={(v) =>
              onFiltersChange({ status: v as StatusFilter, page: 1 })
            }
          >
            <SelectTrigger className="w-[180px]">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">All statuses</SelectItem>
              <SelectItem value="installed">Installed</SelectItem>
              <SelectItem value="update_available">Update available</SelectItem>
              <SelectItem value="not_installed">Not installed</SelectItem>
            </SelectContent>
          </Select>

          <Select
            value={filters.source}
            onValueChange={(v) =>
              onFiltersChange({ source: v as SourceFilter, page: 1 })
            }
          >
            <SelectTrigger className="w-[150px]">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="local">Local</SelectItem>
              <SelectItem value="all">All sources</SelectItem>
              <SelectItem value="remote">
                Remote
                {facets?.remote_count
                  ? ` (${facets.remote_count})`
                  : ""}
              </SelectItem>
            </SelectContent>
          </Select>

          <div className="w-[220px]">
            <ReactSelect<CategoryOption, false>
              options={categoryOptions}
              value={filters.category}
              onValueChange={(v) =>
                onFiltersChange({ category: v || "all", page: 1 })
              }
              placeholder="Category…"
              isSearchable
              isClearable={filters.category !== "all"}
              className="w-full"
            />
          </div>

          <Select
            value={filters.sort}
            onValueChange={(v) =>
              onFiltersChange({ sort: v as SortKey, page: 1 })
            }
          >
            <SelectTrigger className="w-[180px]">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="order">Default order</SelectItem>
              <SelectItem value="name">Name A–Z</SelectItem>
              <SelectItem value="recent">Recently updated</SelectItem>
              <SelectItem value="category">Category</SelectItem>
            </SelectContent>
          </Select>

          <div className="flex overflow-hidden rounded-md border">
            <button
              type="button"
              onClick={() => setViewPersist("grid")}
              className={`p-2 ${view === "grid" ? "bg-muted text-foreground" : "text-muted-foreground hover:bg-muted/50"}`}
              aria-label="Grid view"
            >
              <LayoutGrid className="h-4 w-4" />
            </button>
            <button
              type="button"
              onClick={() => setViewPersist("list")}
              className={`border-l p-2 ${view === "list" ? "bg-muted text-foreground" : "text-muted-foreground hover:bg-muted/50"}`}
              aria-label="List view"
            >
              <List className="h-4 w-4" />
            </button>
          </div>

          {updateCount > 0 && (
            <Button
              className="bg-amber-500 text-black hover:bg-amber-400"
              onClick={() =>
                onFiltersChange({ status: "update_available", page: 1 })
              }
            >
              Updates ({updateCount})
            </Button>
          )}
        </div>
      </div>

      <div className={isFetching ? "opacity-60 transition-opacity" : undefined}>
        {items.length === 0 ? (
          <div className="flex flex-col items-center justify-center rounded-lg border border-dashed py-16 text-center">
            <Search className="mb-3 h-8 w-8 text-muted-foreground" />
            <h3 className="text-base font-medium">No software matches your filters</h3>
            <p className="mt-1 text-sm text-muted-foreground">
              Try adjusting your search or clearing filters.
            </p>
            {filtersActive && (
              <Button variant="outline" size="sm" className="mt-4" onClick={clearFilters}>
                <X className="mr-1.5 h-3.5 w-3.5" />
                Clear filters
              </Button>
            )}
          </div>
        ) : view === "grid" ? (
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
            {items.map((s) => (
              <SoftwareCard
                key={s.id}
                software={s}
                onClick={onOpen}
                onUpdate={onUpdate}
                onServiceAction={onServiceAction}
                busyAction={busyAction}
                queueBusy={queueBusyById?.get(s.id) ?? null}
              />
            ))}
          </div>
        ) : (
          <SoftwareList
            items={items}
            selectedIds={selectedIds}
            allSelected={allSelected}
            someSelected={someSelected}
            onToggleSelect={toggleSelected}
            onToggleSelectAll={toggleSelectAll}
            onClick={onOpen}
            onUpdate={onUpdate}
            onServiceAction={onServiceAction}
            busyAction={busyAction}
            queueBusyById={queueBusyById}
          />
        )}
      </div>

      {pagination.total > 0 && (
        <div className="flex flex-col items-center justify-between gap-3 border-t pt-4 sm:flex-row">
          <p className="text-sm text-muted-foreground">
            Showing{" "}
            <span className="font-medium text-foreground">
              {Math.min((page - 1) * pagination.limit + 1, pagination.total)}
              –
              {Math.min(page * pagination.limit, pagination.total)}
            </span>{" "}
            of{" "}
            <span className="font-medium text-foreground">{pagination.total}</span>
          </p>
          <div className="flex items-center gap-2">
            <Select
              value={String(filters.limit)}
              onValueChange={(v) =>
                onFiltersChange({ limit: Number(v), page: 1 })
              }
            >
              <SelectTrigger className="w-[110px]">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="8">8 / page</SelectItem>
                <SelectItem value="12">12 / page</SelectItem>
                <SelectItem value="24">24 / page</SelectItem>
                <SelectItem value="48">48 / page</SelectItem>
              </SelectContent>
            </Select>
            <Button
              type="button"
              variant="outline"
              size="sm"
              disabled={page <= 1}
              onClick={() => onFiltersChange({ page: page - 1 })}
            >
              <ChevronLeft className="mr-1 h-4 w-4" />
              Prev
            </Button>
            <span className="min-w-[5rem] text-center text-sm tabular-nums text-muted-foreground">
              {page} / {totalPages}
            </span>
            <Button
              type="button"
              variant="outline"
              size="sm"
              disabled={page >= totalPages}
              onClick={() => onFiltersChange({ page: page + 1 })}
            >
              Next
              <ChevronRight className="ml-1 h-4 w-4" />
            </Button>
          </div>
        </div>
      )}

      {selectedIds.size > 0 && typeof document !== "undefined"
        ? createPortal(
            <div className="pointer-events-none fixed inset-x-0 bottom-5 z-50 flex justify-center px-4">
              <div className="pointer-events-auto flex items-center gap-1.5 rounded-lg border bg-background/95 px-2.5 py-1.5 shadow-md backdrop-blur supports-backdrop-filter:bg-background/80">
                <span className="hidden px-2 text-sm text-muted-foreground sm:inline">
                  {selectedIds.size} selected
                </span>
                <Button
                  size="sm"
                  disabled={
                    selectedNotInstalled.length === 0 || Boolean(bulkBusy)
                  }
                  onClick={() => {
                    if (onBulkAction) {
                      onBulkAction("install", selectedNotInstalled)
                      setSelectedIds(new Set())
                      return
                    }
                    const target = selectedNotInstalled[0]
                    if (target) onOpen(target)
                  }}
                >
                  <Download className="mr-1.5 h-3.5 w-3.5" />
                  Install
                  {selectedNotInstalled.length > 1
                    ? ` (${selectedNotInstalled.length})`
                    : ""}
                </Button>
                <Button
                  size="sm"
                  className="bg-amber-500 text-black hover:bg-amber-400"
                  disabled={
                    selectedWithUpdate.length === 0 || Boolean(bulkBusy)
                  }
                  onClick={() => {
                    if (onBulkAction) {
                      onBulkAction("update", selectedWithUpdate)
                      setSelectedIds(new Set())
                      return
                    }
                    const target = selectedWithUpdate[0]
                    if (target) onUpdate(target)
                  }}
                >
                  <ArrowUpCircle className="mr-1.5 h-3.5 w-3.5" />
                  Update
                  {selectedWithUpdate.length > 1
                    ? ` (${selectedWithUpdate.length})`
                    : ""}
                </Button>
                <Button
                  size="sm"
                  variant="outline"
                  disabled={
                    selectedRemovable.length === 0 || Boolean(bulkBusy)
                  }
                  onClick={() => {
                    if (onBulkAction) {
                      onBulkAction("uninstall", selectedRemovable)
                      setSelectedIds(new Set())
                      return
                    }
                    toast.message(
                      "Uninstall is not available yet for catalog software."
                    )
                  }}
                  title={
                    selectedInstalled.length > 0 &&
                    selectedRemovable.length === 0
                      ? "Selected items have no uninstall script"
                      : undefined
                  }
                >
                  <Trash2 className="mr-1.5 h-3.5 w-3.5" />
                  Uninstall
                  {selectedRemovable.length > 1
                    ? ` (${selectedRemovable.length})`
                    : ""}
                </Button>
                <Button
                  size="sm"
                  variant="ghost"
                  className="px-2"
                  onClick={() => setSelectedIds(new Set())}
                  aria-label="Clear selection"
                >
                  <X className="h-4 w-4" />
                </Button>
              </div>
            </div>,
            document.body
          )
        : null}
    </div>
  )
}
