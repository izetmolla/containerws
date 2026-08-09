import {
  Archive,
  ArchiveRestore,
  ClipboardCopy,
  ClipboardPaste,
  Code2,
  Copy,
  Download,
  FileCode,
  FileText,
  FolderOpen,
  Move,
  Pencil,
  RotateCcw,
  Scissors,
  Shield,
  SquareTerminal,
  Trash2,
} from "lucide-react"
import type { ReactNode } from "react"

import {
  ContextMenu,
  ContextMenuContent,
  ContextMenuItem,
  ContextMenuSeparator,
  ContextMenuShortcut,
  ContextMenuTrigger,
} from "@/components/ui/context-menu"

import type { FileEntry } from "./api"
import { isTextEditableFile } from "./text-file-editor"

export type EntryContextActions = {
  onOpen: () => void
  onOpenAtEditor?: () => void
  onOpenInTerminal?: () => void
  onOpenInVSCode?: () => void
  onEdit?: () => void
  onRename?: () => void
  onDuplicate?: () => void
  onMove?: () => void
  onCopyTo?: () => void
  onCopyClipboard?: () => void
  onCutClipboard?: () => void
  onPasteClipboard?: () => void
  onChmod?: () => void
  onDownload?: () => void
  onZip?: () => void
  onUnzip?: () => void
  onDelete?: () => void
  onRestore?: () => void
  onDeletePermanent?: () => void
  canPaste?: boolean
  isTrash?: boolean
}

export function EntryContextMenu({
  entry,
  children,
  actions,
}: {
  entry: FileEntry
  children: ReactNode
  actions: EntryContextActions
}) {
  const canEdit = isTextEditableFile(entry)
  const canOpenEditor = entry.type !== "directory"
  const isDirectory = entry.type === "directory"
  const isTrash = actions.isTrash || Boolean(entry.mime_hint?.startsWith("trash:"))

  return (
    <ContextMenu>
      <ContextMenuTrigger asChild>{children}</ContextMenuTrigger>
      <ContextMenuContent className="w-56">
        {isTrash ? (
          <>
            <ContextMenuItem onClick={actions.onRestore}>
              <RotateCcw className="size-4" />
              Restore
            </ContextMenuItem>
            <ContextMenuSeparator />
            <ContextMenuItem
              variant="destructive"
              onClick={actions.onDeletePermanent}
            >
              <Trash2 className="size-4" />
              Delete permanently
            </ContextMenuItem>
          </>
        ) : (
          <>
            <ContextMenuItem onClick={actions.onOpen}>
              <FolderOpen className="size-4" />
              Open
            </ContextMenuItem>
            {canOpenEditor && actions.onOpenAtEditor ? (
              <ContextMenuItem onClick={actions.onOpenAtEditor}>
                <FileCode className="size-4" />
                Open in editor
              </ContextMenuItem>
            ) : null}
            {isDirectory && actions.onOpenInTerminal ? (
              <ContextMenuItem onClick={actions.onOpenInTerminal}>
                <SquareTerminal className="size-4" />
                Open in terminal
              </ContextMenuItem>
            ) : null}
            {isDirectory && actions.onOpenInVSCode ? (
              <ContextMenuItem onClick={actions.onOpenInVSCode}>
                <Code2 className="size-4" />
                Open in VS Code
              </ContextMenuItem>
            ) : null}
            {canEdit && actions.onEdit ? (
              <ContextMenuItem onClick={actions.onEdit}>
                <FileText className="size-4" />
                Edit file
              </ContextMenuItem>
            ) : null}
            <ContextMenuSeparator />
            <ContextMenuItem onClick={actions.onCopyClipboard}>
              <ClipboardCopy className="size-4" />
              Copy
              <ContextMenuShortcut>⌘C</ContextMenuShortcut>
            </ContextMenuItem>
            <ContextMenuItem onClick={actions.onCutClipboard}>
              <Scissors className="size-4" />
              Cut
              <ContextMenuShortcut>⌘X</ContextMenuShortcut>
            </ContextMenuItem>
            <ContextMenuItem
              disabled={!actions.canPaste}
              onClick={actions.onPasteClipboard}
            >
              <ClipboardPaste className="size-4" />
              Paste
              <ContextMenuShortcut>⌘V</ContextMenuShortcut>
            </ContextMenuItem>
            <ContextMenuSeparator />
            {actions.onRename ? (
              <ContextMenuItem onClick={actions.onRename}>
                <Pencil className="size-4" />
                Rename
              </ContextMenuItem>
            ) : null}
            {actions.onDuplicate ? (
              <ContextMenuItem onClick={actions.onDuplicate}>
                <Copy className="size-4" />
                Duplicate
              </ContextMenuItem>
            ) : null}
            {actions.onMove ? (
              <ContextMenuItem onClick={actions.onMove}>
                <Move className="size-4" />
                Move…
              </ContextMenuItem>
            ) : null}
            {actions.onCopyTo ? (
              <ContextMenuItem onClick={actions.onCopyTo}>
                <Copy className="size-4" />
                Copy to…
              </ContextMenuItem>
            ) : null}
            {actions.onChmod ? (
              <ContextMenuItem onClick={actions.onChmod}>
                <Shield className="size-4" />
                Permissions…
              </ContextMenuItem>
            ) : null}
            {actions.onDownload ? (
              <ContextMenuItem onClick={actions.onDownload}>
                <Download className="size-4" />
                Download
              </ContextMenuItem>
            ) : null}
            {actions.onZip ? (
              <ContextMenuItem onClick={actions.onZip}>
                <Archive className="size-4" />
                Compress to zip
              </ContextMenuItem>
            ) : null}
            {actions.onUnzip ? (
              <ContextMenuItem onClick={actions.onUnzip}>
                <ArchiveRestore className="size-4" />
                Extract zip
              </ContextMenuItem>
            ) : null}
            <ContextMenuSeparator />
            <ContextMenuItem variant="destructive" onClick={actions.onDelete}>
              <Trash2 className="size-4" />
              Move to trash
            </ContextMenuItem>
          </>
        )}
      </ContextMenuContent>
    </ContextMenu>
  )
}

export function PaneContextMenu({
  children,
  canPaste,
  onPaste,
  onNewFolder,
  onNewFile,
  onRefresh,
  onOpenInTerminal,
  onOpenInVSCode,
}: {
  children: ReactNode
  canPaste?: boolean
  onPaste?: () => void
  onNewFolder?: () => void
  onNewFile?: () => void
  onRefresh?: () => void
  onOpenInTerminal?: () => void
  onOpenInVSCode?: () => void
}) {
  return (
    <ContextMenu>
      <ContextMenuTrigger asChild>{children}</ContextMenuTrigger>
      <ContextMenuContent className="w-56">
        {onOpenInTerminal || onOpenInVSCode ? (
          <>
            {onOpenInTerminal ? (
              <ContextMenuItem onClick={onOpenInTerminal}>
                <SquareTerminal className="size-4" />
                Open in terminal
              </ContextMenuItem>
            ) : null}
            {onOpenInVSCode ? (
              <ContextMenuItem onClick={onOpenInVSCode}>
                <Code2 className="size-4" />
                Open in VS Code
              </ContextMenuItem>
            ) : null}
            <ContextMenuSeparator />
          </>
        ) : null}
        <ContextMenuItem disabled={!canPaste} onClick={onPaste}>
          <ClipboardPaste className="size-4" />
          Paste
          <ContextMenuShortcut>⌘V</ContextMenuShortcut>
        </ContextMenuItem>
        <ContextMenuSeparator />
        <ContextMenuItem onClick={onNewFolder}>New folder</ContextMenuItem>
        <ContextMenuItem onClick={onNewFile}>New file</ContextMenuItem>
        <ContextMenuSeparator />
        <ContextMenuItem onClick={onRefresh}>Refresh</ContextMenuItem>
      </ContextMenuContent>
    </ContextMenu>
  )
}
