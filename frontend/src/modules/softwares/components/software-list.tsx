import { useMemo } from "react"
import { useNavigate } from "react-router"
import {
  createColumnHelper,
  type ColumnDef,
} from "@tanstack/react-table"
import {
  ArrowUpCircle,
  Download,
  ExternalLink,
  MoreHorizontal,
  Play,
  RotateCcw,
  ScrollText,
  Square,
} from "lucide-react"

import { Button } from "@/components/ui/button"
import { DataTable } from "@/components/ui/data-table"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { cn } from "@/lib/utils"

import {
  softwareInstallStatus,
  type SoftwareListItem,
  type SoftwareServiceAction,
} from "../pages/list/api"
import { SoftwareGlyph } from "./software-glyph"
import { SourceBadge } from "./source-badge"
import { StatusPill } from "./status-pill"

const columnHelper = createColumnHelper<SoftwareListItem>()

function serviceDotClass(overall?: string) {
  switch (overall) {
    case "running":
      return "bg-emerald-500"
    case "partial":
      return "bg-amber-500"
    case "failed":
      return "bg-red-500"
    case "stopped":
      return "bg-muted-foreground"
    default:
      return "bg-muted-foreground/60"
  }
}

export function SoftwareList({
  items,
  selectedIds,
  allSelected,
  someSelected,
  onToggleSelect,
  onToggleSelectAll,
  onClick,
  onUpdate,
  onServiceAction,
  busyAction,
  queueBusyById,
}: {
  items: SoftwareListItem[]
  selectedIds: Set<string>
  allSelected: boolean
  someSelected: boolean
  onToggleSelect: (id: string, next?: boolean) => void
  onToggleSelectAll: () => void
  onClick: (s: SoftwareListItem) => void
  onUpdate: (s: SoftwareListItem) => void
  onServiceAction?: (id: string, action: SoftwareServiceAction) => void
  busyAction?: string | null
  queueBusyById?: Map<string, "installing" | "updating">
}) {
  const navigate = useNavigate()
  const columns = useMemo(
    () =>
      [
        columnHelper.display({
          id: "select",
          header: () => (
            <div className="flex items-center justify-center">
              <input
                type="checkbox"
                className="size-3.5 accent-foreground"
                checked={allSelected}
                ref={(el) => {
                  if (el) el.indeterminate = someSelected && !allSelected
                }}
                onChange={onToggleSelectAll}
                aria-label="Select all on this page"
              />
            </div>
          ),
          cell: ({ row }) => {
            const item = row.original
            const selected = selectedIds.has(item.id)
            return (
              <div className="flex items-center justify-center">
                <input
                  type="checkbox"
                  className="size-3.5 accent-foreground"
                  checked={selected}
                  onChange={(e) => onToggleSelect(item.id, e.target.checked)}
                  onClick={(e) => e.stopPropagation()}
                  aria-label={`Select ${item.name}`}
                />
              </div>
            )
          },
          meta: { className: "w-9 !px-1.5", width: 36 },
          size: 36,
        }),
        columnHelper.accessor("name", {
          header: "Software",
          meta: { className: "!pl-1" },
          cell: ({ row }) => {
            const item = row.original
            const status = softwareInstallStatus(
              item,
              queueBusyById?.get(item.id) ?? null
            )
            const accent = item.color || "var(--primary)"
            return (
              <button
                type="button"
                onClick={() => onClick(item)}
                className="group flex min-w-0 max-w-sm items-center gap-2 text-start"
              >
                <div className="relative shrink-0">
                  {item.image?.trim() ? (
                    <div className="flex size-7 items-center justify-center overflow-hidden rounded border border-border/60 bg-background">
                      <SoftwareGlyph
                        name={item.icon}
                        image={item.image}
                        className="h-3.5 w-3.5"
                        imgClassName="size-7 object-cover"
                      />
                    </div>
                  ) : (
                    <div
                      className="flex size-7 items-center justify-center rounded text-white"
                      style={{ backgroundColor: accent }}
                    >
                      <SoftwareGlyph name={item.icon} className="h-3.5 w-3.5" />
                    </div>
                  )}
                  {status === "update_available" ? (
                    <span className="absolute -top-0.5 -right-0.5 size-1.5 rounded-full bg-amber-500 ring-1 ring-background" />
                  ) : null}
                </div>
                <div className="min-w-0">
                  <div className="flex min-w-0 flex-wrap items-center gap-1.5">
                    <div className="truncate text-sm font-medium group-hover:underline">
                      {item.name}
                    </div>
                    <SourceBadge software={item} />
                  </div>
                  {item.details ? (
                    <p className="max-w-[18rem] truncate text-[11px] leading-tight text-muted-foreground">
                      {item.details}
                    </p>
                  ) : null}
                </div>
              </button>
            )
          },
        }),
        columnHelper.accessor("category", {
          header: "Category",
          cell: ({ row }) => {
            const item = row.original
            if (!item.category) {
              return <span className="text-xs text-muted-foreground">—</span>
            }
            return (
              <div className="min-w-0 max-w-36">
                <div className="truncate text-xs text-foreground/90">
                  {item.category}
                </div>
                {item.sub_category ? (
                  <div className="truncate text-[11px] text-muted-foreground">
                    {item.sub_category}
                  </div>
                ) : null}
              </div>
            )
          },
        }),
        columnHelper.display({
          id: "installed",
          header: "Installed",
          cell: ({ row }) => {
            const ver = row.original.installed_version?.version
            return (
              <span
                className={cn(
                  "font-mono text-xs tabular-nums",
                  ver ? "text-foreground" : "text-muted-foreground"
                )}
              >
                {ver ? `v${ver}` : "—"}
              </span>
            )
          },
        }),
        columnHelper.display({
          id: "latest",
          header: "Latest",
          cell: ({ row }) => {
            const item = row.original
            const ver = item.latest_version?.version
            const hasUpdate = Boolean(item.has_update)
            return (
              <div className="flex items-center gap-1.5">
                <span
                  className={cn(
                    "font-mono text-xs tabular-nums",
                    ver ? "text-foreground" : "text-muted-foreground"
                  )}
                >
                  {ver ? `v${ver}` : "—"}
                </span>
                {hasUpdate ? (
                  <span className="text-[10px] font-medium text-amber-700 dark:text-amber-300">
                    New
                  </span>
                ) : null}
              </div>
            )
          },
        }),
        columnHelper.display({
          id: "status",
          header: "Status",
          cell: ({ row }) => {
            const item = row.original
            const status = softwareInstallStatus(
              item,
              queueBusyById?.get(item.id) ?? null
            )
            const managed =
              Boolean(item.can_control) || Boolean(item.service_status?.managed)
            const overall = item.service_status?.overall
            return (
              <div className="flex flex-col items-start gap-1">
                <StatusPill status={status} compact />
                {managed ? (
                  <span className="inline-flex items-center gap-1 text-[11px] text-muted-foreground">
                    <span
                      className={cn("size-1.5 rounded-full", serviceDotClass(overall))}
                      aria-hidden
                    />
                    {overall || "unknown"}
                  </span>
                ) : null}
              </div>
            )
          },
        }),
        columnHelper.display({
          id: "actions",
          header: "",
          cell: ({ row }) => {
            const item = row.original
            const status = softwareInstallStatus(item)
            const managed =
              Boolean(item.can_control) || Boolean(item.service_status?.managed)
            const overall = item.service_status?.overall
            const canControl =
              managed && Boolean(item.is_installed) && onServiceAction
            const busy = busyAction?.startsWith(`${item.id}:`)
            const currentAction = busy ? busyAction!.split(":")[1] : null

            return (
              <div className="flex justify-end">
                <DropdownMenu>
                  <DropdownMenuTrigger asChild>
                    <Button
                      variant="ghost"
                      size="icon-sm"
                      className="size-7"
                      aria-label={`${item.name} actions`}
                      onClick={(e) => e.stopPropagation()}
                    >
                      <MoreHorizontal className="size-4" />
                    </Button>
                  </DropdownMenuTrigger>
                  <DropdownMenuContent align="end" className="min-w-40">
                    <DropdownMenuItem onClick={() => onClick(item)}>
                      <ExternalLink className="size-3.5" />
                      Open
                    </DropdownMenuItem>
                    {status === "not_installed" ? (
                      <DropdownMenuItem onClick={() => onClick(item)}>
                        <Download className="size-3.5" />
                        Install
                      </DropdownMenuItem>
                    ) : null}
                    {status === "update_available" ? (
                      <DropdownMenuItem onClick={() => onUpdate(item)}>
                        <ArrowUpCircle className="size-3.5" />
                        Update
                      </DropdownMenuItem>
                    ) : null}
                    {canControl ? (
                      <>
                        <DropdownMenuSeparator />
                        <DropdownMenuItem
                          disabled={Boolean(busy) || overall === "running"}
                          onClick={() => onServiceAction!(item.id, "start")}
                        >
                          <Play className="size-3.5" />
                          {currentAction === "start" ? "Starting…" : "Start"}
                        </DropdownMenuItem>
                        <DropdownMenuItem
                          disabled={Boolean(busy) || overall === "stopped"}
                          onClick={() => onServiceAction!(item.id, "stop")}
                        >
                          <Square className="size-3.5" />
                          {currentAction === "stop" ? "Stopping…" : "Stop"}
                        </DropdownMenuItem>
                        <DropdownMenuItem
                          disabled={Boolean(busy)}
                          onClick={() => onServiceAction!(item.id, "restart")}
                        >
                          <RotateCcw className="size-3.5" />
                          {currentAction === "restart" ? "Restarting…" : "Restart"}
                        </DropdownMenuItem>
                      </>
                    ) : null}
                    {managed ? (
                      <>
                        {!canControl ? <DropdownMenuSeparator /> : null}
                        <DropdownMenuItem
                          onClick={() =>
                            navigate(`/softwares/${item.id}?tab=service`)
                          }
                        >
                          <ScrollText className="size-3.5" />
                          Live logs
                        </DropdownMenuItem>
                      </>
                    ) : null}
                  </DropdownMenuContent>
                </DropdownMenu>
              </div>
            )
          },
        }),
      ] as ColumnDef<SoftwareListItem, unknown>[],
    [
      allSelected,
      someSelected,
      selectedIds,
      onToggleSelect,
      onToggleSelectAll,
      onClick,
      onUpdate,
      onServiceAction,
      busyAction,
      queueBusyById,
      navigate,
    ]
  )

  return (
    <DataTable
      dense
      headerAlign="center"
      columns={columns}
      data={items}
      emptyMessage="No software matches your filters."
      className="gap-0"
    />
  )
}
