import { Folder, Home, Plus, X } from "lucide-react"

import { Button } from "@/components/ui/button"
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip"
import { cn } from "@/lib/utils"

import type { FileSession } from "./sessions"

function sessionIcon(session: FileSession) {
  const path = session.path?.trim()
  if (!path || path === "/" || session.label === "Home") {
    return Home
  }
  return Folder
}

export function SessionTabs({
  sessions,
  activeId,
  dirtyIds,
  onSelect,
  onClose,
  onAdd,
}: {
  sessions: FileSession[]
  activeId: string
  dirtyIds?: Set<string>
  onSelect: (id: string) => void
  onClose: (id: string) => void
  onAdd: () => void
}) {
  return (
    <div className="flex h-9 min-w-0 items-stretch border-b bg-muted/40">
      <div
        role="tablist"
        aria-label="File sessions"
        className="flex min-w-0 flex-1 items-stretch overflow-x-auto [scrollbar-width:none] [&::-webkit-scrollbar]:hidden"
      >
        {sessions.map((session) => {
          const active = session.id === activeId
          const dirty = dirtyIds?.has(session.id)
          const Icon = sessionIcon(session)
          const label = session.label || "Home"
          const pathHint = session.path || "Home"

          return (
            <div
              key={session.id}
              role="tab"
              aria-selected={active}
              className={cn(
                "group relative flex max-w-[11.5rem] min-w-[6.5rem] shrink-0 items-stretch border-r border-border/60 transition-colors",
                active
                  ? "z-[1] -mb-px border-b border-b-background bg-background text-foreground"
                  : "bg-transparent text-muted-foreground hover:bg-muted/70 hover:text-foreground",
              )}
            >
              {active ? (
                <span
                  aria-hidden
                  className="absolute inset-x-0 bottom-0 h-0.5 bg-primary"
                />
              ) : null}
              <button
                type="button"
                className="flex min-w-0 flex-1 items-center gap-1.5 px-2.5 text-left text-[12px] leading-none"
                onClick={() => onSelect(session.id)}
                title={pathHint}
              >
                <Icon
                  className={cn(
                    "size-3.5 shrink-0",
                    active ? "text-primary" : "text-muted-foreground/80",
                  )}
                />
                <span className="min-w-0 flex-1 truncate font-medium tracking-tight">
                  {label}
                </span>
                {dirty ? (
                  <span
                    className="size-1.5 shrink-0 rounded-full bg-amber-500"
                    title="Updated elsewhere — will refresh"
                  />
                ) : null}
              </button>
              {sessions.length > 1 ? (
                <button
                  type="button"
                  className={cn(
                    "me-1 self-center rounded-sm p-0.5 text-muted-foreground transition-opacity hover:bg-muted hover:text-foreground",
                    active
                      ? "opacity-70"
                      : "opacity-0 group-hover:opacity-100 focus-visible:opacity-100",
                  )}
                  onClick={(e) => {
                    e.stopPropagation()
                    onClose(session.id)
                  }}
                  aria-label={`Close ${label}`}
                >
                  <X className="size-3" />
                </button>
              ) : null}
            </div>
          )
        })}
      </div>

      <div className="flex shrink-0 items-center border-l border-border/60 px-1">
        <Tooltip>
          <TooltipTrigger asChild>
            <Button
              type="button"
              size="icon-sm"
              variant="ghost"
              className="size-7 text-muted-foreground hover:text-foreground"
              onClick={onAdd}
              aria-label="New session tab"
            >
              <Plus className="size-3.5" />
            </Button>
          </TooltipTrigger>
          <TooltipContent side="bottom">New session</TooltipContent>
        </Tooltip>
      </div>
    </div>
  )
}
