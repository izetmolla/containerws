import { useEffect, useMemo, type ReactNode } from "react"
import { AnimatePresence, motion } from "motion/react"
import {
  Copy,
  Download,
  FileCode,
  File,
  FolderOpen,
  Move,
  Pencil,
  RotateCcw,
  Shield,
  Trash2,
  X,
} from "lucide-react"

import { Button } from "@/components/ui/button"
import { Separator } from "@/components/ui/separator"
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip"
import { cn } from "@/lib/utils"

import type { FileEntry } from "./api"
import { isTextEditableFile } from "./text-file-editor"

export type SelectionAction =
  | "open"
  | "edit"
  | "open-editor"
  | "download"
  | "rename"
  | "move"
  | "copy"
  | "chmod"
  | "delete"
  | "restore"
  | "clear"

type ActionDef = {
  key: SelectionAction
  label: string
  icon: ReactNode
  variant?: "default" | "destructive"
  iconOnly?: boolean
}

function ActionButton({
  action,
  disabled,
  pending,
  onClick,
}: {
  action: ActionDef
  disabled?: boolean
  pending?: boolean
  onClick: () => void
}) {
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <Button
          type="button"
          size="sm"
          variant="secondary"
          aria-label={action.label}
          disabled={disabled || pending}
          className={cn(
            "border border-secondary bg-secondary/50 hover:bg-secondary/70 [&>svg]:size-3.5",
            action.iconOnly
              ? "size-8 gap-0 px-0"
              : "h-8 gap-1.5 px-2.5",
            action.variant === "destructive" &&
              "border-destructive/30 bg-destructive/10 text-destructive hover:bg-destructive/20 hover:text-destructive",
          )}
          onClick={onClick}
        >
          {action.icon}
          {!action.iconOnly ? (
            <span className="hidden sm:inline">{action.label}</span>
          ) : null}
        </Button>
      </TooltipTrigger>
      <TooltipContent sideOffset={6}>{action.label}</TooltipContent>
    </Tooltip>
  )
}

export function FileSelectionBar({
  selected,
  entries,
  busy,
  onAction,
  className,
  inTrash,
}: {
  selected: Set<string>
  entries: FileEntry[]
  busy?: boolean
  onAction: (action: SelectionAction, items: FileEntry[]) => void
  className?: string
  inTrash?: boolean
}) {
  const items = useMemo(
    () => entries.filter((e) => selected.has(e.path)),
    [entries, selected],
  )

  const visible = items.length > 0

  useEffect(() => {
    if (!visible) return
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onAction("clear", items)
    }
    window.addEventListener("keydown", onKey)
    return () => window.removeEventListener("keydown", onKey)
  }, [visible, items, onAction])

  const onlyOne = items.length === 1
  const single = onlyOne ? items[0] : null
  const allFiles = items.every((e) => e.type !== "directory")
  const hasDir = items.some((e) => e.type === "directory")
  const canEdit = onlyOne && single != null && isTextEditableFile(single)
  const canDownload = allFiles && items.length > 0
  const folderCount = items.filter((e) => e.type === "directory").length
  const fileCount = items.length - folderCount

  const actions: ActionDef[] = []

  if (inTrash) {
    actions.push(
      {
        key: "restore",
        label: "Restore",
        icon: <RotateCcw />,
      },
      {
        key: "delete",
        label: "Delete permanently",
        icon: <Trash2 />,
        variant: "destructive",
      },
    )
  } else {
    if (onlyOne && single) {
      actions.push({
        key: "open",
        label: single.type === "directory" ? "Open" : "Open",
        icon: single.type === "directory" ? <FolderOpen /> : <File />,
      })
      if (single.type !== "directory") {
        actions.push({
          key: "open-editor",
          label: "Open At Editor",
          icon: <FileCode />,
        })
      }
      if (canEdit) {
        actions.push({
          key: "edit",
          label: "Edit",
          icon: <File />,
        })
      }
      if (single.type !== "directory") {
        actions.push({
          key: "download",
          label: "Download",
          icon: <Download />,
        })
      }
      actions.push({
        key: "rename",
        label: "Rename",
        icon: <Pencil />,
      })
    } else if (canDownload) {
      actions.push({
        key: "download",
        label: "Download",
        icon: <Download />,
      })
    }

    actions.push(
      {
        key: "move",
        label: "Move",
        icon: <Move />,
      },
      {
        key: "copy",
        label: "Copy",
        icon: <Copy />,
      },
      {
        key: "chmod",
        label: "Permissions",
        icon: <Shield />,
      },
      {
        key: "delete",
        label: "Move to trash",
        icon: <Trash2 />,
        variant: "destructive",
        iconOnly: true,
      },
    )
  }

  const summary =
    folderCount > 0 && fileCount > 0
      ? `${fileCount} file${fileCount === 1 ? "" : "s"}, ${folderCount} folder${folderCount === 1 ? "" : "s"}`
      : folderCount > 0
        ? `${folderCount} folder${folderCount === 1 ? "" : "s"}`
        : `${fileCount} file${fileCount === 1 ? "" : "s"}`

  return (
    <AnimatePresence>
      {visible ? (
        <motion.div
          role="toolbar"
          aria-label="Selection actions"
          aria-orientation="horizontal"
          initial={{ opacity: 0, y: 16 }}
          animate={{ opacity: 1, y: 0 }}
          exit={{ opacity: 0, y: 16 }}
          transition={{ duration: 0.18, ease: "easeOut" }}
          className={cn(
            "pointer-events-auto absolute inset-x-3 bottom-3 z-20 mx-auto flex w-fit max-w-[calc(100%-1.5rem)] flex-wrap items-center justify-center gap-2 rounded-xl border bg-background/95 p-2 text-foreground shadow-lg backdrop-blur-md supports-backdrop-filter:bg-background/80",
            className,
          )}
        >
          <div className="flex h-8 items-center rounded-lg border bg-muted/40 pr-1 pl-2.5">
            <span className="whitespace-nowrap text-xs font-medium">
              {items.length} selected
              <span className="hidden text-muted-foreground sm:inline">
                {" "}
                · {summary}
              </span>
            </span>
            <Separator
              orientation="vertical"
              className="mx-2 data-[orientation=vertical]:h-4"
            />
            <Tooltip>
              <TooltipTrigger asChild>
                <Button
                  type="button"
                  variant="ghost"
                  size="icon"
                  className="size-6"
                  onClick={() => onAction("clear", items)}
                >
                  <X className="size-3.5" />
                </Button>
              </TooltipTrigger>
              <TooltipContent sideOffset={8}>
                Clear selection (Esc)
              </TooltipContent>
            </Tooltip>
          </div>

          <Separator
            orientation="vertical"
            className="hidden h-6 data-[orientation=vertical]:h-6 sm:block"
          />

          <div className="flex flex-wrap items-center justify-center gap-1.5">
            {actions.map((action) => (
              <ActionButton
                key={action.key}
                action={action}
                disabled={busy || (action.key === "download" && hasDir)}
                pending={busy}
                onClick={() => onAction(action.key, items)}
              />
            ))}
          </div>
        </motion.div>
      ) : null}
    </AnimatePresence>
  )
}
