import { Button } from "@/components/ui/button"
import { cn } from "@/lib/utils"
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
import { useNavigate } from "react-router"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import {
  softwareInstallStatus,
  type SoftwareListItem,
  type SoftwareServiceAction,
} from "../pages/list/api"
import { SoftwareGlyph } from "./software-glyph"
import { SourceBadge } from "./source-badge"
import { StatusPill } from "./status-pill"

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

export function SoftwareCard({
  software,
  onClick,
  onUpdate,
  onServiceAction,
  busyAction,
  queueBusy,
}: {
  software: SoftwareListItem
  onClick: (s: SoftwareListItem) => void
  onUpdate: (s: SoftwareListItem) => void
  onServiceAction?: (id: string, action: SoftwareServiceAction) => void
  busyAction?: string | null
  queueBusy?: "installing" | "updating" | null
}) {
  const navigate = useNavigate()
  const status = softwareInstallStatus(software, queueBusy ?? null)
  const hasUpdate = status === "update_available"
  const accent = software.color || "var(--primary)"
  const managed =
    Boolean(software.can_control) || Boolean(software.service_status?.managed)
  const overall = software.service_status?.overall
  const canControl =
    managed && Boolean(software.is_installed) && onServiceAction
  const showServiceMenu = managed
  const busy = busyAction?.startsWith(`${software.id}:`)
  const currentAction = busy ? busyAction!.split(":")[1] : null

  const versionLabel = software.installed_version
    ? `v${software.installed_version.version}`
    : software.latest_version
      ? `v${software.latest_version.version}`
      : "—"

  return (
    <div
      role="button"
      tabIndex={0}
      onClick={() => onClick(software)}
      onKeyDown={(e) => {
        if (e.key === "Enter" || e.key === " ") {
          e.preventDefault()
          onClick(software)
        }
      }}
      className={cn(
        "group flex h-full cursor-pointer flex-col border border-border/70 bg-card/40 p-4 transition-colors",
        "rounded-lg hover:bg-muted/25 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
      )}
    >
      <div className="flex items-start gap-3">
        <div className="relative shrink-0">
          {software.image?.trim() ? (
            <div className="flex size-10 items-center justify-center overflow-hidden rounded-md border border-border/60 bg-background">
              <SoftwareGlyph
                name={software.icon}
                image={software.image}
                className="h-5 w-5"
                imgClassName="size-10 object-cover"
              />
            </div>
          ) : (
            <div
              className="flex size-10 items-center justify-center rounded-md text-white"
              style={{ backgroundColor: accent }}
            >
              <SoftwareGlyph name={software.icon} className="h-5 w-5" />
            </div>
          )}
          {hasUpdate ? (
            <span className="absolute -top-0.5 -right-0.5 size-2 rounded-full bg-amber-500 ring-2 ring-background" />
          ) : null}
        </div>

        <div className="min-w-0 flex-1">
          <div className="flex items-start justify-between gap-2">
            <div className="min-w-0">
              <div className="flex min-w-0 flex-wrap items-center gap-1.5">
                <h3 className="truncate text-base font-medium tracking-tight group-hover:underline">
                  {software.name}
                </h3>
                <SourceBadge software={software} />
              </div>
              <p className="mt-0.5 truncate text-xs text-muted-foreground">
                {[software.category, software.sub_category]
                  .filter(Boolean)
                  .join(" · ") || "Uncategorized"}
              </p>
            </div>
            <div
              className="shrink-0"
              onClick={(e) => e.stopPropagation()}
              onKeyDown={(e) => e.stopPropagation()}
            >
              <DropdownMenu>
                <DropdownMenuTrigger asChild>
                  <Button
                    variant="ghost"
                    size="icon-sm"
                    className="size-8 opacity-0 transition-opacity group-hover:opacity-100 focus-visible:opacity-100 data-[state=open]:opacity-100"
                    aria-label={`${software.name} actions`}
                  >
                    <MoreHorizontal className="size-4" />
                  </Button>
                </DropdownMenuTrigger>
                <DropdownMenuContent align="end" className="min-w-40">
                  <DropdownMenuItem onClick={() => onClick(software)}>
                    <ExternalLink className="size-3.5" />
                    Open
                  </DropdownMenuItem>
                  {status === "not_installed" ? (
                    <DropdownMenuItem onClick={() => onClick(software)}>
                      <Download className="size-3.5" />
                      Install
                    </DropdownMenuItem>
                  ) : null}
                  {hasUpdate ? (
                    <DropdownMenuItem onClick={() => onUpdate(software)}>
                      <ArrowUpCircle className="size-3.5" />
                      Update
                    </DropdownMenuItem>
                  ) : null}
                  {canControl ? (
                    <>
                      <DropdownMenuSeparator />
                      <DropdownMenuItem
                        disabled={Boolean(busy) || overall === "running"}
                        onClick={() => onServiceAction!(software.id, "start")}
                      >
                        <Play className="size-3.5" />
                        {currentAction === "start" ? "Starting…" : "Start"}
                      </DropdownMenuItem>
                      <DropdownMenuItem
                        disabled={Boolean(busy) || overall === "stopped"}
                        onClick={() => onServiceAction!(software.id, "stop")}
                      >
                        <Square className="size-3.5" />
                        {currentAction === "stop" ? "Stopping…" : "Stop"}
                      </DropdownMenuItem>
                      <DropdownMenuItem
                        disabled={Boolean(busy)}
                        onClick={() => onServiceAction!(software.id, "restart")}
                      >
                        <RotateCcw className="size-3.5" />
                        {currentAction === "restart" ? "Restarting…" : "Restart"}
                      </DropdownMenuItem>
                    </>
                  ) : null}
                  {showServiceMenu ? (
                    <>
                      {!canControl ? <DropdownMenuSeparator /> : null}
                      <DropdownMenuItem
                        onClick={() =>
                          navigate(`/softwares/${software.id}?tab=service`)
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
          </div>
        </div>
      </div>

      <p className="mt-3 line-clamp-2 flex-1 text-sm leading-relaxed text-muted-foreground">
        {software.details || "No description"}
      </p>

      <div className="mt-4 flex items-center justify-between gap-2 border-t border-border/50 pt-3">
        <div className="flex min-w-0 flex-wrap items-center gap-2">
          <StatusPill status={status} />
          {managed ? (
            <span className="inline-flex items-center gap-1.5 text-xs text-muted-foreground">
              <span
                className={cn("size-1.5 rounded-full", serviceDotClass(overall))}
                aria-hidden
              />
              {overall || "unknown"}
            </span>
          ) : null}
        </div>
        <span className="shrink-0 font-mono text-xs tabular-nums text-muted-foreground">
          {versionLabel}
        </span>
      </div>
    </div>
  )
}
