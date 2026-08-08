"use client"

import { useState, type ReactNode } from "react"
import {
  ChevronDown,
  ExternalLink,
  Keyboard,
  Loader2,
  Maximize2,
  Minimize2,
  MoreVertical,
  Pencil,
  Plus,
  SquareTerminal,
  X,
} from "lucide-react"
import { toast } from "sonner"

import { Button } from "@/components/ui/button"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { getRequestErrorMessage } from "@/lib/network"
import { cn } from "@/lib/utils"
import { openCodeserverEditor } from "@/modules/vscode/pages/list/api"

import type { CloudShellTab } from "./store"

type CloudShellHeaderProps = {
  tabs: CloudShellTab[]
  activeTabId: string | null
  maximized: boolean
  minimized: boolean
  fullscreen?: boolean
  /** Preferred folder for Open Editor (active shell cwd when known). */
  editorPath?: string | null
  /** Whether the special-keys accessory bar is open. */
  specialKeysOpen?: boolean
  onToggleSpecialKeys?: () => void
  onSelectTab: (id: string) => void
  onCloseTab: (id: string) => void
  onAddTab: () => void
  onMinimize: () => void
  onToggleMaximize: () => void
  onClose: () => void
  onPopOut: () => void
  onReconnect: () => void
  onNewShell: () => void
}

function IconButton({
  label,
  onClick,
  children,
  className,
}: {
  label: string
  onClick?: () => void
  children: ReactNode
  className?: string
}) {
  return (
    <button
      type="button"
      title={label}
      aria-label={label}
      onClick={onClick}
      className={cn(
        "inline-flex size-8 shrink-0 items-center justify-center rounded-sm text-zinc-300 transition-colors",
        "hover:bg-white/10 hover:text-white",
        "focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-sky-500",
        className
      )}
    >
      {children}
    </button>
  )
}

export function CloudShellHeader({
  tabs,
  activeTabId,
  maximized,
  minimized,
  fullscreen = false,
  editorPath,
  specialKeysOpen = false,
  onToggleSpecialKeys,
  onSelectTab,
  onCloseTab,
  onAddTab,
  onMinimize,
  onToggleMaximize,
  onClose,
  onPopOut,
  onReconnect,
  onNewShell,
}: CloudShellHeaderProps) {
  const activeTab = tabs.find((t) => t.id === activeTabId) ?? tabs[0]
  const [openingEditor, setOpeningEditor] = useState(false)

  const handleOpenEditor = async () => {
    if (openingEditor) return
    setOpeningEditor(true)
    // Open synchronously so popup blockers allow the tab after the async start.
    const tab = window.open("about:blank", "_blank")
    try {
      const res = await openCodeserverEditor({
        path: editorPath || undefined,
        shellSessionId: activeTab?.sessionId || activeTabId || undefined,
      })
      const url =
        res.connect_url ||
        res.data?.connect_url ||
        (res.data?.id ? `/codeserver/${res.data.id}/` : "")
      if (!url) {
        throw new Error("VS Code connect URL missing")
      }
      if (tab && !tab.closed) {
        tab.location.href = url
      } else {
        window.open(url, "_blank", "noopener,noreferrer")
      }
      toast.success(res.message || "Opening VS Code")
    } catch (err) {
      if (tab && !tab.closed) tab.close()
      toast.error(getRequestErrorMessage(err, "Could not open VS Code editor"))
    } finally {
      setOpeningEditor(false)
    }
  }

  return (
    <div className="relative flex h-10 shrink-0 items-stretch border-b border-white/10 bg-[#202124] text-zinc-200">
      <div
        className="pointer-events-none absolute left-1/2 top-1.5 z-10 hidden h-1 w-10 -translate-x-1/2 rounded-full bg-white/20 sm:block"
        aria-hidden
      />

      {/* Branding + tabs */}
      <div className="flex min-w-0 flex-1 items-center gap-1 pl-2 sm:gap-2 sm:pl-3">
        <div className="flex shrink-0 items-center gap-1.5 sm:gap-2 sm:pr-2">
          <SquareTerminal className="size-4 shrink-0 text-zinc-400" />
          {/* Full label on sm+; icon-only on phones */}
          <div className="hidden leading-none sm:block">
            <div className="text-[9px] font-medium tracking-[0.14em] text-zinc-400 uppercase">
              Cloud Shell
            </div>
            <div className="text-[13px] font-semibold text-white">Terminal</div>
          </div>
        </div>

        {/* Mobile: compact active-session switcher (avoids cramped tab strip) */}
        <div className="flex min-w-0 flex-1 items-center gap-0.5 sm:hidden">
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <button
                type="button"
                className={cn(
                  "flex min-w-0 flex-1 items-center gap-1 rounded-sm border border-white/10 bg-[#2b2c2f] px-2 py-1 text-left",
                  "focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-sky-500"
                )}
              >
                <span className="truncate text-[12px] font-medium text-white">
                  {activeTab?.title ?? "Session"}
                </span>
                <ChevronDown className="size-3.5 shrink-0 text-zinc-400" />
              </button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="start" className="min-w-48">
              {tabs.map((tab) => (
                <DropdownMenuItem
                  key={tab.id}
                  className="flex items-center justify-between gap-3"
                  onClick={() => onSelectTab(tab.id)}
                >
                  <span className={cn(tab.id === activeTabId && "font-semibold")}>
                    {tab.title}
                    {tab.sessionId ? (
                      <span className="ml-1 text-[10px] text-muted-foreground">•</span>
                    ) : null}
                  </span>
                  {tabs.length > 1 ? (
                    <button
                      type="button"
                      aria-label={`Close ${tab.title}`}
                      className="rounded-sm p-0.5 text-muted-foreground hover:bg-muted hover:text-foreground"
                      onClick={(e) => {
                        e.stopPropagation()
                        onCloseTab(tab.id)
                      }}
                    >
                      <X className="size-3.5" />
                    </button>
                  ) : null}
                </DropdownMenuItem>
              ))}
              <DropdownMenuSeparator />
              <DropdownMenuItem onClick={onAddTab}>
                <Plus className="size-3.5" />
                New tab
              </DropdownMenuItem>
              <DropdownMenuItem onClick={onReconnect}>Reconnect</DropdownMenuItem>
              <DropdownMenuItem onClick={onNewShell}>New shell</DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>

          <IconButton label="New terminal" onClick={onAddTab} className="size-7">
            <Plus className="size-3.5" />
          </IconButton>
        </div>

        {/* Desktop / tablet: horizontal tabs */}
        <div className="hidden min-w-0 flex-1 items-end gap-0 self-stretch overflow-x-auto sm:flex">
          {tabs.map((tab) => {
            const active = tab.id === activeTabId
            return (
              <div
                key={tab.id}
                className={cn(
                  "group relative flex h-full max-w-[160px] items-center gap-1 border-b-2 px-2.5 text-[12px] md:max-w-[220px] md:px-3",
                  active
                    ? "border-sky-500 bg-[#2b2c2f] text-white"
                    : "border-transparent text-zinc-400 hover:bg-white/5 hover:text-zinc-200"
                )}
              >
                <button
                  type="button"
                  className="min-w-0 truncate focus-visible:outline-none"
                  onClick={() => onSelectTab(tab.id)}
                  title={tab.title}
                >
                  {tab.title}
                </button>
                <button
                  type="button"
                  aria-label={`Close ${tab.title}`}
                  className={cn(
                    "inline-flex size-4 shrink-0 items-center justify-center rounded-sm hover:bg-white/15",
                    active ? "opacity-70" : "opacity-0 group-hover:opacity-100"
                  )}
                  onClick={(e) => {
                    e.stopPropagation()
                    onCloseTab(tab.id)
                  }}
                >
                  <X className="size-3" />
                </button>
              </div>
            )
          })}

          <IconButton label="New terminal" onClick={onAddTab} className="size-7 self-center">
            <Plus className="size-3.5" />
          </IconButton>

          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <button
                type="button"
                aria-label="Terminal menu"
                className="inline-flex size-7 items-center justify-center self-center rounded-sm text-zinc-400 hover:bg-white/10 hover:text-white"
              >
                <ChevronDown className="size-3.5" />
              </button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="start" className="min-w-44">
              <DropdownMenuItem onClick={onAddTab}>New tab</DropdownMenuItem>
              <DropdownMenuItem onClick={onReconnect}>Reconnect</DropdownMenuItem>
              <DropdownMenuItem onClick={onNewShell}>New shell</DropdownMenuItem>
              <DropdownMenuItem onClick={onPopOut}>Open in new window</DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      </div>

      {/* Actions */}
      <div className="flex shrink-0 items-center gap-0 pr-1 pl-1 sm:gap-0.5 sm:pl-2">
        <Button
          type="button"
          variant="outline"
          size="sm"
          disabled={openingEditor}
          className={cn(
            "mr-1 hidden h-7 gap-1.5 border-sky-500/70 bg-transparent px-2.5 text-[12px] text-sky-400 hover:bg-sky-500/10 hover:text-sky-300 md:inline-flex",
            fullscreen && "hidden"
          )}
          onClick={() => void handleOpenEditor()}
        >
          {openingEditor ? (
            <Loader2 className="size-3 animate-spin" />
          ) : (
            <Pencil className="size-3" />
          )}
          {openingEditor ? "Opening…" : "Open Editor"}
        </Button>

        <IconButton
          label={
            specialKeysOpen
              ? "Hide special keys"
              : "Special keys (Ctrl+C, Esc, arrows…)"
          }
          onClick={() => {
            onToggleSpecialKeys?.()
            window.dispatchEvent(new Event("cloudshell:focus"))
          }}
          className={cn(specialKeysOpen && "bg-sky-600/80 text-white hover:bg-sky-500")}
        >
          <Keyboard className="size-3.5" />
        </IconButton>

        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <button
              type="button"
              aria-label="More"
              className="inline-flex size-8 items-center justify-center rounded-sm text-zinc-300 hover:bg-white/10 hover:text-white"
            >
              <MoreVertical className="size-3.5" />
            </button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end" className="min-w-48">
            <DropdownMenuItem
              className={cn("md:hidden", fullscreen && "hidden")}
              disabled={openingEditor}
              onClick={() => void handleOpenEditor()}
            >
              {openingEditor ? (
                <Loader2 className="size-3.5 animate-spin" />
              ) : (
                <Pencil className="size-3.5" />
              )}
              {openingEditor ? "Opening…" : "Open Editor"}
            </DropdownMenuItem>
            <DropdownMenuItem className="sm:hidden" onClick={onPopOut}>
              <ExternalLink className="size-3.5" />
              Open in new window
            </DropdownMenuItem>
            <DropdownMenuItem onClick={onReconnect}>Reconnect session</DropdownMenuItem>
            <DropdownMenuItem onClick={onNewShell}>New shell (kill old)</DropdownMenuItem>
            <DropdownMenuItem onClick={onAddTab}>New session tab</DropdownMenuItem>
            <DropdownMenuSeparator />
            <DropdownMenuItem
              onClick={() => {
                onToggleSpecialKeys?.()
                window.dispatchEvent(new Event("cloudshell:focus"))
              }}
            >
              <Keyboard className="size-3.5" />
              {specialKeysOpen ? "Hide special keys" : "Special keys bar"}
            </DropdownMenuItem>
            <DropdownMenuItem
              onClick={() => {
                window.dispatchEvent(new Event("cloudshell:focus"))
              }}
            >
              <Keyboard className="size-3.5" />
              Focus / show keyboard
            </DropdownMenuItem>
            <DropdownMenuSeparator />
            <DropdownMenuItem onClick={onClose}>
              {fullscreen ? "Exit full screen" : "Close Cloud Shell"}
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>

        <div className="mx-0.5 hidden h-5 w-px bg-white/10 sm:mx-1 sm:block" />

        <IconButton
          label={minimized ? "Restore" : "Minimize"}
          onClick={onMinimize}
          className={cn(fullscreen ? "hidden" : "hidden sm:inline-flex")}
        >
          <span className="text-base leading-none">─</span>
        </IconButton>
        <IconButton
          label={maximized ? "Restore size" : "Maximize"}
          onClick={onToggleMaximize}
          className={cn(fullscreen && "hidden")}
        >
          {maximized ? (
            <Minimize2 className="size-3.5" />
          ) : (
            <Maximize2 className="size-3.5" />
          )}
        </IconButton>
        <IconButton
          label="Open in new window"
          onClick={onPopOut}
          className={cn("hidden sm:inline-flex", fullscreen && "hidden")}
        >
          <ExternalLink className="size-3.5" />
        </IconButton>
        <IconButton label={fullscreen ? "Exit full screen" : "Close"} onClick={onClose}>
          <X className="size-3.5" />
        </IconButton>
      </div>
    </div>
  )
}
