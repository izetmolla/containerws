import { useEffect, useMemo, useState } from "react"
import { useQuery } from "@tanstack/react-query"
import {
  ChevronRight,
  Folder,
  FolderOpen,
  Home,
  Loader2,
} from "lucide-react"

import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { ScrollArea } from "@/components/ui/scroll-area"
import { useDebounce } from "@/components/layouts/dashboard1/hooks/use-debounce"
import { cn } from "@/lib/utils"

import {
  FILEMANAGER_FETCH_KEY,
  listDirectory,
  type FileEntry,
  type FileRoot,
} from "./api"

function normalizePath(raw: string): string {
  const trimmed = raw.trim()
  if (!trimmed) return "/"
  const withSlash = trimmed.startsWith("/") ? trimmed : `/${trimmed}`
  const clean = withSlash.replace(/\/+/g, "/")
  if (clean.length > 1 && clean.endsWith("/")) return clean.slice(0, -1)
  return clean || "/"
}

function splitCrumbs(path: string) {
  const clean = path === "/" ? "/" : path.replace(/\/+$/, "")
  if (clean === "/") return [{ label: "/", path: "/" }]
  const parts = clean.split("/").filter(Boolean)
  const crumbs: { label: string; path: string }[] = [{ label: "/", path: "/" }]
  let acc = ""
  for (const part of parts) {
    acc += `/${part}`
    crumbs.push({ label: part, path: acc })
  }
  return crumbs
}

function parentOf(path: string) {
  const clean = normalizePath(path)
  if (clean === "/") return "/"
  const i = clean.lastIndexOf("/")
  if (i <= 0) return "/"
  return clean.slice(0, i) || "/"
}

export function DestinationPickerDialog({
  open,
  mode,
  source,
  sources,
  initialPath,
  roots,
  busy,
  onOpenChange,
  onConfirm,
}: {
  open: boolean
  mode: "copy" | "move"
  source: FileEntry | null
  /** When moving/copying multiple items. */
  sources?: FileEntry[]
  initialPath: string
  roots: FileRoot[]
  busy?: boolean
  onOpenChange: (open: boolean) => void
  onConfirm: (destinationFolder: string) => void
}) {
  const [browsePath, setBrowsePath] = useState(normalizePath(initialPath || "/"))
  const [pathDraft, setPathDraft] = useState(normalizePath(initialPath || "/"))
  const [prevOpen, setPrevOpen] = useState(open)
  const debouncedDraft = useDebounce(pathDraft, 500)

  if (open !== prevOpen) {
    setPrevOpen(open)
    if (open) {
      const start = normalizePath(initialPath || "/")
      setBrowsePath(start)
      setPathDraft(start)
    }
  }

  useEffect(() => {
    if (!open) return
    const next = normalizePath(debouncedDraft)
    setBrowsePath((prev) => (prev === next ? prev : next))
  }, [debouncedDraft, open])

  const listQuery = useQuery({
    queryKey: [FILEMANAGER_FETCH_KEY, "dest-picker", browsePath],
    queryFn: async () => {
      const res = await listDirectory(browsePath, false)
      return res.data
    },
    enabled: open,
    staleTime: 5_000,
    placeholderData: (prev) => prev,
    retry: false,
  })

  const folders = useMemo(() => {
    const entries = listQuery.data?.entries ?? []
    return entries.filter(
      (e) =>
        e.type === "directory" ||
        (e.type === "symlink" && e.executable),
    )
  }, [listQuery.data?.entries])

  const crumbs = splitCrumbs(browsePath)
  const resolvedPath = normalizePath(pathDraft)
  const bulkSources = sources && sources.length > 1 ? sources : null
  const previewName = bulkSources
    ? `${bulkSources.length} items`
    : source?.name || "item"
  const finalPath = bulkSources
    ? resolvedPath === "/"
      ? `/${previewName}`
      : `${resolvedPath}/…`
    : resolvedPath === "/"
      ? `/${previewName}`
      : `${resolvedPath}/${previewName}`
  const pathPending =
    normalizePath(pathDraft) !== browsePath &&
    normalizePath(debouncedDraft) !== normalizePath(pathDraft)

  const goTo = (next: string) => {
    const p = normalizePath(next)
    setBrowsePath(p)
    setPathDraft(p)
  }

  const commitTypedPath = () => {
    const p = normalizePath(pathDraft)
    setPathDraft(p)
    setBrowsePath(p)
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="flex max-h-[min(90vh,44rem)] max-w-[calc(100%-2rem)] flex-col gap-0 overflow-hidden p-0 sm:max-w-3xl">
        <div className="space-y-1 border-b px-5 py-4">
          <DialogHeader className="gap-1 text-left">
            <DialogTitle>
              {mode === "move" ? "Move to folder" : "Copy to folder"}
            </DialogTitle>
            <DialogDescription className="text-left">
              Choose where to place{" "}
              <span className="font-mono text-foreground">
                {bulkSources
                  ? `${bulkSources.length} selected items`
                  : source?.name || "item"}
              </span>
              . Missing folders are created automatically.
            </DialogDescription>
          </DialogHeader>
        </div>

        <div className="grid min-h-0 flex-1 gap-0 sm:grid-cols-[180px_minmax(0,1fr)]">
          <aside className="hidden border-e bg-muted/20 p-3 sm:block">
            <p className="mb-2 px-1 text-[11px] font-medium tracking-wide text-muted-foreground uppercase">
              Shortcuts
            </p>
            <div className="space-y-0.5">
              {(roots.length ? roots : [{ path: "/", label: "Root", icon: "Home" }]).map(
                (root) => {
                  const active =
                    browsePath === root.path ||
                    browsePath.startsWith(
                      root.path === "/" ? "/" : `${root.path}/`,
                    )
                  return (
                    <button
                      key={root.path}
                      type="button"
                      onClick={() => goTo(root.path)}
                      className={cn(
                        "flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-left text-sm transition-colors",
                        active
                          ? "bg-muted font-medium text-foreground"
                          : "text-muted-foreground hover:bg-muted/70 hover:text-foreground",
                      )}
                    >
                      {root.path === "/" || root.icon === "Home" ? (
                        <Home className="size-3.5 shrink-0" />
                      ) : (
                        <Folder className="size-3.5 shrink-0" />
                      )}
                      <span className="truncate">{root.label}</span>
                    </button>
                  )
                },
              )}
            </div>
          </aside>

          <div className="flex min-h-0 flex-col">
            <div className="space-y-3 border-b px-4 py-3">
              <div className="space-y-1.5">
                <Label htmlFor="fm-dest" className="text-xs">
                  Destination folder
                </Label>
                <div className="flex gap-2">
                  <Input
                    id="fm-dest"
                    value={pathDraft}
                    onChange={(e) => setPathDraft(e.target.value)}
                    onKeyDown={(e) => {
                      if (e.key === "Enter") {
                        e.preventDefault()
                        commitTypedPath()
                      }
                    }}
                    placeholder="/path/to/folder"
                    className="font-mono text-xs"
                    autoFocus
                  />
                  <Button
                    type="button"
                    size="sm"
                    variant="outline"
                    className="shrink-0"
                    onClick={commitTypedPath}
                  >
                    Go
                  </Button>
                </div>
                <p className="text-[11px] text-muted-foreground">
                  {pathPending
                    ? "Checking path after you stop typing…"
                    : listQuery.isError
                      ? "This path does not exist yet."
                      : null}
                </p>
              </div>

              <nav className="flex flex-wrap items-center gap-0.5 text-xs">
                {crumbs.map((c, i) => (
                  <span key={c.path} className="flex items-center gap-0.5">
                    {i > 0 ? (
                      <ChevronRight className="size-3 text-muted-foreground" />
                    ) : null}
                    <button
                      type="button"
                      onClick={() => goTo(c.path)}
                      className={cn(
                        "rounded px-1 py-0.5 font-mono hover:bg-muted",
                        i === crumbs.length - 1
                          ? "font-medium text-foreground"
                          : "text-muted-foreground",
                      )}
                    >
                      {c.label}
                    </button>
                  </span>
                ))}
              </nav>
            </div>

            <div className="flex items-center justify-between gap-2 border-b bg-muted/15 px-4 py-2 text-xs text-muted-foreground">
              <span>
                {pathPending ? (
                  <span className="inline-flex items-center gap-1.5">
                    <Loader2 className="size-3 animate-spin" />
                    Waiting for typing…
                  </span>
                ) : listQuery.isFetching ? (
                  <span className="inline-flex items-center gap-1.5">
                    <Loader2 className="size-3 animate-spin" />
                    Loading…
                  </span>
                ) : listQuery.isError ? (
                  <span className="text-amber-700 dark:text-amber-300">
                    Folder not found yet — it will be created on{" "}
                    {mode === "move" ? "move" : "copy"}
                  </span>
                ) : (
                  `${folders.length} folder${folders.length === 1 ? "" : "s"}`
                )}
              </span>
              {browsePath !== "/" ? (
                <Button
                  type="button"
                  size="sm"
                  variant="ghost"
                  className="h-7 text-xs"
                  onClick={() => goTo(parentOf(browsePath))}
                >
                  Up one level
                </Button>
              ) : null}
            </div>

            <ScrollArea className="min-h-[14rem] flex-1">
              <div className="p-2">
                {folders.length === 0 && !listQuery.isFetching && !listQuery.isError ? (
                  <p className="px-2 py-8 text-center text-sm text-muted-foreground">
                    No subfolders here. Use this folder as the destination, or
                    type a new path above.
                  </p>
                ) : (
                  <ul className="space-y-0.5">
                    {folders.map((folder) => (
                      <li key={folder.path}>
                        <button
                          type="button"
                          onClick={() => goTo(folder.path)}
                          onDoubleClick={() => goTo(folder.path)}
                          className="flex w-full items-center gap-2 rounded-lg px-2.5 py-2 text-left text-sm transition-colors hover:bg-muted"
                        >
                          <FolderOpen className="size-4 shrink-0 text-amber-600 dark:text-amber-400" />
                          <span className="min-w-0 flex-1 truncate font-medium">
                            {folder.name}
                          </span>
                          <ChevronRight className="size-3.5 shrink-0 text-muted-foreground" />
                        </button>
                      </li>
                    ))}
                  </ul>
                )}
              </div>
            </ScrollArea>

            <div className="space-y-2 border-t bg-muted/20 px-4 py-3">
              <div className="flex flex-wrap items-center gap-2 text-xs">
                <span className="text-muted-foreground">Will place as</span>
                <Badge
                  variant="outline"
                  className="max-w-full truncate font-mono text-[11px] font-normal"
                >
                  {finalPath}
                </Badge>
              </div>
            </div>
          </div>
        </div>

        <DialogFooter className="m-0 shrink-0 rounded-none border-t px-5 py-3 sm:justify-between">
          <p className="hidden text-xs text-muted-foreground sm:block">
            Select a folder or type a path. New folders are created silently.
          </p>
          <div className="flex w-full gap-2 sm:w-auto">
            <Button
              type="button"
              variant="outline"
              className="flex-1 sm:flex-none"
              onClick={() => onOpenChange(false)}
            >
              Cancel
            </Button>
            <Button
              type="button"
              className="flex-1 sm:flex-none"
              disabled={!resolvedPath || busy}
              onClick={() => onConfirm(resolvedPath)}
            >
              {mode === "move" ? "Move here" : "Copy here"}
            </Button>
          </div>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
