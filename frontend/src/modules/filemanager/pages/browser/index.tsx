import { useEffect, useMemo, useRef, useState } from "react"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import {
  ArrowUp,
  ArrowDown,
  ArrowUpDown,
  Check,
  ChevronRight,
  Code2,
  Copy,
  Download,
  Eye,
  EyeOff,
  File,
  FileCode,
  FileImage,
  FilePlus,
  FileText,
  Folder,
  FolderOpen,
  FolderPlus,
  Grid2X2,
  HardDrive,
  Home,
  List as ListIcon,
  MoreHorizontal,
  Move,
  Package,
  Pencil,
  RefreshCw,
  RotateCcw,
  Search,
  Settings,
  Shield,
  SquareTerminal,
  Trash2,
  Upload,
  Users,
  Database,
  Thermometer,
  X,
} from "lucide-react"
import { toast } from "sonner"
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog"
import { Badge } from "@/components/ui/badge"
import {
  Breadcrumb,
  BreadcrumbItem,
  BreadcrumbLink,
  BreadcrumbList,
  BreadcrumbPage,
  BreadcrumbSeparator,
} from "@/components/ui/breadcrumb"
import { Button } from "@/components/ui/button"
import { ButtonGroup } from "@/components/ui/button-group"
import { Checkbox } from "@/components/ui/checkbox"
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
import { ScrollArea } from "@/components/ui/scroll-area"
import { Separator } from "@/components/ui/separator"
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group"
import { toastRequestError } from "@/lib/network"
import { useEvent } from "@/lib/use-event"
import { cn } from "@/lib/utils"
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip"
import {
  getCodeserverStatus,
  VSCODE_SESSIONS_FETCH_KEY,
} from "@/modules/vscode/pages/list/api"

import {
  chmodPath,
  copyPath,
  createFile,
  downloadEntries,
  downloadFile,
  duplicatePath,
  emptyTrash,
  FILEMANAGER_FETCH_KEY,
  formatBytes,
  formatModTime,
  isTrashPath,
  listDirectory,
  mkdir,
  movePath,
  moveToTrash,
  permanentlyDeleteTrashItem,
  readFile,
  renamePath,
  restoreTrashItem,
  trashIdFromEntry,
  unzipPath,
  uploadFile,
  writeFile,
  zipPaths,
  type FileEntry,
  type FileRoot,
} from "./api"
import { useFileClipboard } from "./clipboard"
import { DestinationPickerDialog } from "./destination-picker"
import {
  EntryContextMenu,
  PaneContextMenu,
} from "./entry-context-menu"
import {
  openFolderInTerminal,
  openFolderInVSCode,
} from "./open-helpers"
import {
  FileSelectionBar,
  type SelectionAction,
} from "./selection-bar"
import { SessionTabs } from "./session-tabs"
import { useFileSessions } from "./sessions"
import { isTextEditableFile, TextFileEditor } from "./text-file-editor"

type ViewMode = "list" | "grid"
type SortKey = "name" | "size" | "modified" | "owner" | "mode"
type SortDir = "asc" | "desc"

type DialogKind =
  | null
  | "mkdir"
  | "create"
  | "rename"
  | "move"
  | "copy"
  | "chmod"
  | "edit"
  | "delete"
  | "empty-trash"

type MetaColKey = "size" | "modified" | "owner" | "mode"

const NAME_COL_MIN = 250
const META_COL_MAX = 480
const META_COL_MIN: Record<MetaColKey, number> = {
  size: 56,
  modified: 100,
  owner: 72,
  mode: 56,
}
const DEFAULT_COL_WIDTHS: Record<MetaColKey, number> = {
  size: 80,
  modified: 160,
  owner: 112,
  mode: 72,
}

const rootIconMap: Record<string, typeof Home> = {
  Home,
  FolderKanban: FolderOpen,
  Thermometer,
  HardDrive,
  Users,
  Package,
  Database,
  Settings,
  Trash2,
}

function entryIcon(entry: FileEntry, className?: string) {
  if (entry.mime_hint?.startsWith("trash:")) {
    if (entry.type === "directory") {
      return <Folder className={cn("text-amber-600 dark:text-amber-400", className)} />
    }
  }
  if (entry.type === "directory") {
    return <Folder className={cn("text-amber-600 dark:text-amber-400", className)} />
  }
  switch (entry.mime_hint) {
    case "text":
      return <FileText className={cn("text-sky-600 dark:text-sky-400", className)} />
    case "image":
      return <FileImage className={cn("text-violet-600 dark:text-violet-400", className)} />
    case "archive":
      return <Package className={cn("text-orange-600 dark:text-orange-400", className)} />
    default:
      if (/\.(go|ts|tsx|js|py|rs|sh)$/i.test(entry.name)) {
        return <FileCode className={cn("text-emerald-600 dark:text-emerald-400", className)} />
      }
      return <File className={cn("text-muted-foreground", className)} />
  }
}

function splitPath(path: string) {
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

function normalizePathInput(raw: string): string {
  const trimmed = raw.trim()
  if (!trimmed) return "/"
  const withSlash = trimmed.startsWith("/") ? trimmed : `/${trimmed}`
  const clean = withSlash.replace(/\/+/g, "/")
  if (clean.length > 1 && clean.endsWith("/")) return clean.slice(0, -1)
  return clean || "/"
}

function usePersistedView(): [ViewMode, (v: ViewMode) => void] {
  const [view, setView] = useState<ViewMode>(() => {
    try {
      const v = localStorage.getItem("filemanager.view")
      return v === "grid" ? "grid" : "list"
    } catch {
      return "list"
    }
  })
  const update = (v: ViewMode) => {
    setView(v)
    try {
      localStorage.setItem("filemanager.view", v)
    } catch {
      /* ignore */
    }
  }
  return [view, update]
}

export default function FileManagerPage() {
  const queryClient = useQueryClient()
  const fileInputRef = useRef<HTMLInputElement>(null)
  const pathInputRef = useRef<HTMLInputElement>(null)
  const sessions = useFileSessions()
  const { clipboard, copyPaths, cutPaths, clearClipboard } = useFileClipboard()
  const [path, setPath] = useState(sessions.active?.path ?? "")
  const [showHidden, setShowHidden] = useState(false)
  const [search, setSearch] = useState("")
  const [view, setView] = usePersistedView()
  const [selected, setSelected] = useState<Set<string>>(new Set())
  const [dialog, setDialog] = useState<DialogKind>(null)
  const [target, setTarget] = useState<FileEntry | null>(null)
  /** Explicit paths for move/copy/chmod when not using current selection alone. */
  const [opPaths, setOpPaths] = useState<string[] | null>(null)
  const [nameInput, setNameInput] = useState("")
  const [destInput, setDestInput] = useState("")
  const [modeInput, setModeInput] = useState("0644")
  const [previewContent, setPreviewContent] = useState("")
  const [previewBaseline, setPreviewBaseline] = useState("")
  const [previewTruncated, setPreviewTruncated] = useState(false)
  const [previewEditable, setPreviewEditable] = useState(false)
  const [pathEditing, setPathEditing] = useState(false)
  const [pathDraft, setPathDraft] = useState("")
  const [sortKey, setSortKey] = useState<SortKey>("name")
  const [sortDir, setSortDir] = useState<SortDir>("asc")
  const [colWidths, setColWidths] = useState(DEFAULT_COL_WIDTHS)
  const colResizeRef = useRef<{
    key: MetaColKey
    startX: number
    startW: number
  } | null>(null)

  const toggleSort = (key: SortKey) => {
    if (sortKey === key) {
      setSortDir((d) => (d === "asc" ? "desc" : "asc"))
    } else {
      setSortKey(key)
      setSortDir(key === "name" ? "asc" : "desc")
    }
  }

  const onColResizeStart = (key: MetaColKey, clientX: number) => {
    colResizeRef.current = {
      key,
      startX: clientX,
      startW: colWidths[key],
    }
    document.body.style.cursor = "col-resize"
    document.body.style.userSelect = "none"
  }

  useEffect(() => {
    const onMove = (ev: PointerEvent) => {
      const start = colResizeRef.current
      if (!start) return
      const next = Math.min(
        META_COL_MAX,
        Math.max(
          META_COL_MIN[start.key],
          start.startW + (ev.clientX - start.startX),
        ),
      )
      setColWidths((prev) =>
        prev[start.key] === next ? prev : { ...prev, [start.key]: next },
      )
    }
    const onUp = () => {
      colResizeRef.current = null
      document.body.style.removeProperty("cursor")
      document.body.style.removeProperty("user-select")
    }
    window.addEventListener("pointermove", onMove)
    window.addEventListener("pointerup", onUp)
    window.addEventListener("pointercancel", onUp)
    return () => {
      window.removeEventListener("pointermove", onMove)
      window.removeEventListener("pointerup", onUp)
      window.removeEventListener("pointercancel", onUp)
    }
  }, [])

  const listQuery = useQuery({
    queryKey: [FILEMANAGER_FETCH_KEY, path || "__home__", showHidden],
    queryFn: async () => {
      const res = await listDirectory(path, showHidden)
      return res.data
    },
    staleTime: 3_000,
    // Keep previous folder visible while the next path loads — no full remount/flash.
    placeholderData: (prev) => prev,
  })

  const vscodeStatusQuery = useQuery({
    queryKey: [VSCODE_SESSIONS_FETCH_KEY, "status"],
    queryFn: async () => {
      const res = await getCodeserverStatus()
      return res.data
    },
    staleTime: 60_000,
  })
  const vscodeInstalled = Boolean(vscodeStatusQuery.data?.installed)

  const data = listQuery.data
  const currentPath = path || data?.path || "/"
  const homeDir = data?.user?.home_dir
  const inTrash = isTrashPath(currentPath, homeDir)
  const entries = useMemo(() => data?.entries ?? [], [data?.entries])
  const roots = useMemo(() => data?.roots ?? [], [data?.roots])
  const placeRoots = useMemo(
    () => roots.filter((r) => (r.group || "places") === "places"),
    [roots],
  )
  const diskRoots = useMemo(
    () => roots.filter((r) => r.group === "disks"),
    [roots],
  )
  const trashRoot = useMemo(
    () => roots.find((r) => r.group === "trash") ?? null,
    [roots],
  )
  const user = data?.user
  const isInitialLoad = listQuery.isLoading && !data
  const isRefreshing = listQuery.isFetching && !!data

  const navigateTo = useEvent((next: string) => {
    const normalized = normalizePathInput(next)
    setPath(normalized)
    sessions.setActivePath(normalized)
    setSelected(new Set())
    setSearch("")
    setPathEditing(false)
  })

  const switchToSession = useEvent((id: string) => {
    const session = sessions.sessions.find((s) => s.id === id)
    if (!session) return
    sessions.switchSession(id)
    setPath(session.path)
    setSelected(new Set())
    setSearch("")
    setPathEditing(false)
    if (session.seenRevision < sessions.revision) {
      void queryClient.invalidateQueries({ queryKey: [FILEMANAGER_FETCH_KEY] })
      sessions.markSeen()
    }
  })

  const startPathEdit = useEvent(() => {
    setPathDraft(currentPath)
    setPathEditing(true)
  })

  const commitPathEdit = useEvent(() => {
    navigateTo(pathDraft)
  })

  const cancelPathEdit = useEvent(() => {
    setPathEditing(false)
    setPathDraft(currentPath)
  })

  useEffect(() => {
    if (!pathEditing) return
    const t = window.setTimeout(() => {
      pathInputRef.current?.focus()
      pathInputRef.current?.select()
    }, 0)
    return () => window.clearTimeout(t)
  }, [pathEditing])

  // When another session bumps revision and this tab is active, refresh.
  useEffect(() => {
    if (!sessions.needsRefresh) return
    void queryClient.invalidateQueries({
      queryKey: [FILEMANAGER_FETCH_KEY, path || "__home__", showHidden],
    })
    sessions.markSeen()
    // eslint-disable-next-line react-hooks/exhaustive-deps -- only react to revision/seen flags
  }, [sessions.needsRefresh, sessions.revision, queryClient, path, showHidden])

  const sortedEntries = useMemo(() => {
    const q = search.trim().toLowerCase()
    const list = q
      ? entries.filter(
          (e) =>
            e.name.toLowerCase().includes(q) ||
            e.owner?.toLowerCase().includes(q) ||
            e.mode.toLowerCase().includes(q),
        )
      : entries.slice()

    const dirRank = (e: FileEntry) => (e.type === "directory" ? 0 : 1)
    const cmpText = (a: string, b: string) =>
      a.localeCompare(b, undefined, { sensitivity: "base", numeric: true })
    const mul = sortDir === "asc" ? 1 : -1

    list.sort((a, b) => {
      // Name sort keeps folders first (file-manager convention).
      if (sortKey === "name") {
        const byType = dirRank(a) - dirRank(b)
        if (byType !== 0) return byType
        return mul * cmpText(a.name, b.name)
      }

      let primary = 0
      switch (sortKey) {
        case "size": {
          const as = a.type === "directory" ? -1 : a.size
          const bs = b.type === "directory" ? -1 : b.size
          primary = as - bs
          break
        }
        case "modified": {
          const at = Date.parse(a.mod_time) || 0
          const bt = Date.parse(b.mod_time) || 0
          primary = at - bt
          break
        }
        case "owner":
          primary = cmpText(
            `${a.owner || ""}:${a.group || ""}`,
            `${b.owner || ""}:${b.group || ""}`,
          )
          break
        case "mode":
          primary = cmpText(a.mode_octal || a.mode, b.mode_octal || b.mode)
          break
      }
      if (primary !== 0) return mul * primary
      // Stable tie-breakers
      const byType = dirRank(a) - dirRank(b)
      if (byType !== 0) return byType
      return cmpText(a.name, b.name)
    })
    return list
  }, [entries, search, sortKey, sortDir])

  const parentPath = data?.parent || ""
  const filtered = sortedEntries

  const invalidate = () => {
    sessions.bumpRevision()
    return queryClient.invalidateQueries({ queryKey: [FILEMANAGER_FETCH_KEY] })
  }

  const closeDialog = () => {
    setDialog(null)
    setTarget(null)
    setOpPaths(null)
    setNameInput("")
    setDestInput("")
    setPreviewContent("")
    setPreviewBaseline("")
    setPreviewEditable(false)
  }

  const openEntry = (entry: FileEntry) => {
    if (inTrash || trashIdFromEntry(entry)) {
      // Trash is a flat list — opening restores context via menu.
      return
    }
    if (entry.type === "directory") {
      navigateTo(entry.path)
      return
    }
    if (entry.type === "symlink") {
      // Navigate; backend follows dir symlinks via Stat.
      navigateTo(entry.path)
      return
    }
    if (isTextEditableFile(entry)) {
      void openEdit(entry)
      return
    }
    void downloadFile(entry.path).catch((err) =>
      toastRequestError(err, "Download failed")
    )
  }

  const openEdit = async (entry: FileEntry) => {
    try {
      const res = await readFile(entry.path)
      setTarget(entry)
      setPreviewContent(res.data.content)
      setPreviewBaseline(res.data.content)
      setPreviewTruncated(!!res.data.truncated)
      setPreviewEditable(
        !res.data.truncated && entry.writable !== false,
      )
      setDialog("edit")
    } catch (err) {
      toastRequestError(err, "Could not open file")
    }
  }

  const mkdirMutation = useMutation({
    mutationFn: () => mkdir(joinPath(currentPath, nameInput.trim())),
    onSuccess: (res) => {
      toast.success(res.message || "Folder created")
      closeDialog()
      invalidate()
    },
    onError: (err) => toastRequestError(err, "Create folder failed"),
  })

  const createMutation = useMutation({
    mutationFn: () => createFile(joinPath(currentPath, nameInput.trim())),
    onSuccess: (res) => {
      toast.success(res.message || "File created")
      closeDialog()
      invalidate()
    },
    onError: (err) => toastRequestError(err, "Create file failed"),
  })

  const renameMutation = useMutation({
    mutationFn: () => {
      if (!target) throw new Error("No target")
      return renamePath(target.path, nameInput.trim())
    },
    onSuccess: (res) => {
      toast.success(res.message || "Renamed")
      closeDialog()
      invalidate()
    },
    onError: (err) => toastRequestError(err, "Rename failed"),
  })

  const moveMutation = useMutation({
    mutationFn: async (destination: string) => {
      const paths =
        opPaths?.length
          ? opPaths
          : selected.size > 0
            ? Array.from(selected)
            : target
              ? [target.path]
              : []
      if (!paths.length) throw new Error("No target")
      const results = await Promise.allSettled(
        paths.map((p) => movePath(p, destination.trim())),
      )
      const failed = results.filter((r) => r.status === "rejected").length
      return { ok: results.length - failed, failed, total: results.length }
    },
    onSuccess: (res) => {
      if (res.failed === 0) toast.success(`Moved ${res.ok} item(s)`)
      else toast.warning(`Moved ${res.ok}, ${res.failed} failed`)
      closeDialog()
      setSelected(new Set())
      invalidate()
    },
    onError: (err) => toastRequestError(err, "Move failed"),
  })

  const copyMutation = useMutation({
    mutationFn: async (destination: string) => {
      const paths =
        opPaths?.length
          ? opPaths
          : selected.size > 0
            ? Array.from(selected)
            : target
              ? [target.path]
              : []
      if (!paths.length) throw new Error("No target")
      const results = await Promise.allSettled(
        paths.map((p) => copyPath(p, destination.trim())),
      )
      const failed = results.filter((r) => r.status === "rejected").length
      return { ok: results.length - failed, failed, total: results.length }
    },
    onSuccess: (res) => {
      if (res.failed === 0) toast.success(`Copied ${res.ok} item(s)`)
      else toast.warning(`Copied ${res.ok}, ${res.failed} failed`)
      closeDialog()
      invalidate()
    },
    onError: (err) => toastRequestError(err, "Copy failed"),
  })

  const chmodMutation = useMutation({
    mutationFn: async () => {
      const paths =
        opPaths?.length
          ? opPaths
          : selected.size > 0
            ? Array.from(selected)
            : target
              ? [target.path]
              : []
      if (!paths.length) throw new Error("No target")
      const mode = modeInput.trim()
      const results = await Promise.allSettled(
        paths.map((p) => chmodPath(p, mode)),
      )
      const failed = results.filter((r) => r.status === "rejected").length
      return { ok: results.length - failed, failed, total: results.length }
    },
    onSuccess: (res) => {
      if (res.failed === 0) toast.success(`Updated permissions on ${res.ok} item(s)`)
      else toast.warning(`Updated ${res.ok}, ${res.failed} failed`)
      closeDialog()
      setSelected(new Set())
      invalidate()
    },
    onError: (err) => toastRequestError(err, "chmod failed"),
  })

  const deleteMutation = useMutation({
    mutationFn: async () => {
      const paths =
        opPaths?.length
          ? opPaths
          : selected.size > 0
            ? Array.from(selected)
            : target
              ? [target.path]
              : []
      if (inTrash) {
        const results = await Promise.allSettled(
          paths.map((p) => {
            const entry = entries.find((e) => e.path === p)
            const id = entry ? trashIdFromEntry(entry) : null
            if (!id) return Promise.reject(new Error("Not a trash item"))
            return permanentlyDeleteTrashItem(id)
          }),
        )
        const failed = results.filter((r) => r.status === "rejected").length
        return {
          ok: results.length - failed,
          failed,
          total: results.length,
          mode: "permanent" as const,
        }
      }
      const results = await Promise.allSettled(
        paths.map((p) => moveToTrash(p)),
      )
      const failed = results.filter((r) => r.status === "rejected").length
      return {
        ok: results.length - failed,
        failed,
        total: results.length,
        mode: "trash" as const,
      }
    },
    onSuccess: (res) => {
      if (res.mode === "permanent") {
        if (res.failed === 0)
          toast.success(`Permanently deleted ${res.ok} item(s)`)
        else toast.warning(`Deleted ${res.ok}, ${res.failed} failed`)
      } else if (res.failed === 0) {
        toast.success(`Moved ${res.ok} item(s) to trash`)
      } else {
        toast.warning(`Trashed ${res.ok}, ${res.failed} failed`)
      }
      closeDialog()
      setSelected(new Set())
      invalidate()
    },
    onError: (err) => toastRequestError(err, "Delete failed"),
  })

  const restoreMutation = useMutation({
    mutationFn: async (ids: string[]) => {
      const results = await Promise.allSettled(
        ids.map((id) => restoreTrashItem(id)),
      )
      const failed = results.filter((r) => r.status === "rejected").length
      return { ok: results.length - failed, failed }
    },
    onSuccess: (res) => {
      if (res.failed === 0) toast.success(`Restored ${res.ok} item(s)`)
      else toast.warning(`Restored ${res.ok}, ${res.failed} failed`)
      setSelected(new Set())
      invalidate()
    },
    onError: (err) => toastRequestError(err, "Restore failed"),
  })

  const emptyTrashMutation = useMutation({
    mutationFn: () => emptyTrash(),
    onSuccess: (res) => {
      toast.success(res.message || `Emptied ${res.data?.deleted ?? 0} item(s)`)
      closeDialog()
      invalidate()
    },
    onError: (err) => toastRequestError(err, "Empty trash failed"),
  })

  const pasteMutation = useMutation({
    mutationFn: async () => {
      if (!clipboard?.paths.length) throw new Error("Clipboard empty")
      const dest = currentPath
      const results = await Promise.allSettled(
        clipboard.paths.map((p) =>
          clipboard.mode === "cut"
            ? movePath(p, dest)
            : copyPath(p, dest),
        ),
      )
      const failed = results.filter((r) => r.status === "rejected").length
      return {
        ok: results.length - failed,
        failed,
        mode: clipboard.mode,
      }
    },
    onSuccess: (res) => {
      if (res.failed === 0) {
        toast.success(
          res.mode === "cut"
            ? `Moved ${res.ok} item(s)`
            : `Pasted ${res.ok} item(s)`,
        )
      } else {
        toast.warning(`Done ${res.ok}, ${res.failed} failed`)
      }
      if (res.mode === "cut") clearClipboard()
      invalidate()
    },
    onError: (err) => toastRequestError(err, "Paste failed"),
  })

  const duplicateMutation = useMutation({
    mutationFn: (p: string) => duplicatePath(p),
    onSuccess: (res) => {
      toast.success(res.message || "Duplicated")
      invalidate()
    },
    onError: (err) => toastRequestError(err, "Duplicate failed"),
  })

  const uploadMutation = useMutation({
    mutationFn: (file: File) => uploadFile(currentPath, file),
    onSuccess: (res) => {
      toast.success(res.message || "Uploaded")
      invalidate()
    },
    onError: (err) => toastRequestError(err, "Upload failed"),
  })

  const saveMutation = useMutation({
    mutationFn: () => {
      if (!target) throw new Error("No target")
      return writeFile(target.path, previewContent)
    },
    onSuccess: (res) => {
      toast.success(res.message || "Saved")
      setPreviewBaseline(previewContent)
      invalidate()
    },
    onError: (err) => toastRequestError(err, "Save failed"),
  })

  const resolveActionPaths = (entry?: FileEntry | null) => {
    if (selected.size > 0) {
      if (entry && selected.has(entry.path)) return Array.from(selected)
      if (!entry) return Array.from(selected)
    }
    if (entry) return [entry.path]
    return []
  }

  const clipboardCopy = useEvent((entry?: FileEntry | null) => {
    const paths = resolveActionPaths(entry)
    if (!paths.length) return
    copyPaths(paths)
    toast.success(`Copied ${paths.length} item(s)`)
  })

  const clipboardCut = useEvent((entry?: FileEntry | null) => {
    const paths = resolveActionPaths(entry)
    if (!paths.length) return
    cutPaths(paths)
    toast.success(`Cut ${paths.length} item(s)`)
  })

  const clipboardPaste = useEvent(() => {
    if (!clipboard?.paths.length || inTrash) return
    pasteMutation.mutate()
  })

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      const targetEl = e.target as HTMLElement | null
      if (
        targetEl &&
        (targetEl.tagName === "INPUT" ||
          targetEl.tagName === "TEXTAREA" ||
          targetEl.isContentEditable)
      ) {
        return
      }
      const meta = e.metaKey || e.ctrlKey
      if (!meta) return
      const key = e.key.toLowerCase()
      if (key === "c") {
        e.preventDefault()
        clipboardCopy()
      } else if (key === "x") {
        e.preventDefault()
        clipboardCut()
      } else if (key === "v") {
        e.preventDefault()
        clipboardPaste()
      }
    }
    window.addEventListener("keydown", onKey)
    return () => window.removeEventListener("keydown", onKey)
  }, [clipboardCopy, clipboardCut, clipboardPaste])

  const toggleSelect = (p: string) => {
    setSelected((prev) => {
      const next = new Set(prev)
      if (next.has(p)) next.delete(p)
      else next.add(p)
      return next
    })
  }

  const handleSelectionAction = (
    action: SelectionAction,
    items: FileEntry[],
  ) => {
    if (action === "clear") {
      setSelected(new Set())
      return
    }
    if (!items.length) return
    const first = items[0]

    if (action === "open") {
      openEntry(first)
      setSelected(new Set())
      return
    }
    if (action === "edit" || action === "open-editor") {
      void openEdit(first)
      return
    }
    if (action === "download") {
      void downloadEntries(items).catch((err) =>
        toastRequestError(err, "Download failed")
      )
      return
    }
    if (action === "zip") {
      void (async () => {
        try {
          const res = await zipPaths(items.map((i) => i.path))
          toast.success(res.message || `Created ${res.data?.name || "archive"}`)
          setSelected(new Set())
          await invalidate()
        } catch (err) {
          toastRequestError(err, "Zip failed")
        }
      })()
      return
    }
    if (action === "unzip") {
      const archive = items[0]
      if (!archive) return
      void (async () => {
        try {
          const res = await unzipPath(archive.path)
          toast.success(
            res.message ||
              `Extracted ${res.data?.extracted ?? 0} file(s) to ${res.data?.path || "folder"}`
          )
          setSelected(new Set())
          await invalidate()
        } catch (err) {
          toastRequestError(err, "Unzip failed")
        }
      })()
      return
    }
    if (action === "rename") {
      setTarget(first)
      setOpPaths([first.path])
      setNameInput(first.name)
      setDialog("rename")
      return
    }
    if (action === "move") {
      setTarget(first)
      setOpPaths(items.map((i) => i.path))
      setDestInput(data?.parent || currentPath)
      setDialog("move")
      return
    }
    if (action === "copy") {
      setTarget(first)
      setOpPaths(items.map((i) => i.path))
      setDestInput(currentPath)
      setDialog("copy")
      return
    }
    if (action === "chmod") {
      setTarget(first)
      setOpPaths(items.map((i) => i.path))
      setModeInput(first.mode_octal || "0644")
      setDialog("chmod")
      return
    }
    if (action === "delete") {
      setTarget(null)
      setOpPaths(items.map((i) => i.path))
      setDialog("delete")
    }
    if (action === "restore") {
      const ids = items
        .map((i) => trashIdFromEntry(i))
        .filter((id): id is string => Boolean(id))
      if (ids.length) restoreMutation.mutate(ids)
    }
  }

  const allFilteredSelected =
    filtered.length > 0 && filtered.every((e) => selected.has(e.path))

  const crumbs = splitPath(currentPath)

  const busy =
    mkdirMutation.isPending ||
    createMutation.isPending ||
    renameMutation.isPending ||
    moveMutation.isPending ||
    copyMutation.isPending ||
    chmodMutation.isPending ||
    deleteMutation.isPending ||
    uploadMutation.isPending ||
    saveMutation.isPending ||
    pasteMutation.isPending ||
    restoreMutation.isPending ||
    emptyTrashMutation.isPending

  const dirtyIds = useMemo(() => {
    const set = new Set<string>()
    for (const s of sessions.sessions) {
      if (s.id !== sessions.activeId && s.seenRevision < sessions.revision) {
        set.add(s.id)
      }
    }
    return set
  }, [sessions.sessions, sessions.activeId, sessions.revision])

  const canPaste = Boolean(clipboard?.paths.length) && !inTrash

  const entryActions = (entry: FileEntry) => {
    const trashId = trashIdFromEntry(entry)
    const isDir = entry.type === "directory"
    return {
      onOpen: () => openEntry(entry),
      onOpenAtEditor: () => void openEdit(entry),
      onOpenInTerminal: isDir
        ? () => openFolderInTerminal(entry.path)
        : undefined,
      onOpenInVSCode:
        isDir && vscodeInstalled
          ? () => void openFolderInVSCode(entry.path)
          : undefined,
      onEdit: () => void openEdit(entry),
      onRename: () => {
        setTarget(entry)
        setOpPaths([entry.path])
        setNameInput(entry.name)
        setDialog("rename")
      },
      onDuplicate: () => duplicateMutation.mutate(entry.path),
      onMove: () => {
        setTarget(entry)
        setOpPaths(resolveActionPaths(entry))
        setDestInput(data?.parent || currentPath)
        setDialog("move")
      },
      onCopyTo: () => {
        setTarget(entry)
        setOpPaths(resolveActionPaths(entry))
        setDestInput(currentPath)
        setDialog("copy")
      },
      onCopyClipboard: () => clipboardCopy(entry),
      onCutClipboard: () => clipboardCut(entry),
      onPasteClipboard: () => clipboardPaste(),
      onChmod: () => {
        setTarget(entry)
        setOpPaths(resolveActionPaths(entry))
        setModeInput(entry.mode_octal || "0644")
        setDialog("chmod")
      },
      onDownload: () => {
        const paths = resolveActionPaths(entry)
        const items = entries.filter((e) => paths.includes(e.path))
        void downloadEntries(items.length ? items : [entry]).catch((err) =>
          toastRequestError(err, "Download failed"),
        )
      },
      onZip: () => {
        const paths = resolveActionPaths(entry)
        void (async () => {
          try {
            const res = await zipPaths(paths)
            toast.success(res.message || `Created ${res.data?.name || "archive"}`)
            setSelected(new Set())
            await invalidate()
          } catch (err) {
            toastRequestError(err, "Zip failed")
          }
        })()
      },
      onUnzip:
        entry.type !== "directory" &&
        entry.name.toLowerCase().endsWith(".zip")
          ? () => {
              void (async () => {
                try {
                  const res = await unzipPath(entry.path)
                  toast.success(
                    res.message ||
                      `Extracted ${res.data?.extracted ?? 0} file(s)`,
                  )
                  setSelected(new Set())
                  await invalidate()
                } catch (err) {
                  toastRequestError(err, "Unzip failed")
                }
              })()
            }
          : undefined,
      onDelete: () => {
        setTarget(entry)
        setOpPaths(resolveActionPaths(entry))
        setDialog("delete")
      },
      onRestore: () => {
        if (trashId) restoreMutation.mutate([trashId])
      },
      onDeletePermanent: () => {
        setTarget(entry)
        setOpPaths([entry.path])
        setDialog("delete")
      },
      canPaste,
      isTrash: inTrash || Boolean(trashId),
    }
  }

  return (
    <div className="flex h-[calc(100dvh-5.5rem)] min-h-[480px] flex-col gap-3">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="min-w-0 space-y-1">
          <h1 className="text-xl font-semibold tracking-tight">Files</h1>
          <p className="text-sm text-muted-foreground">
            Browse this machine as{" "}
            <span className="font-medium text-foreground">
              {user?.shell_user || "…"}
            </span>
            {user ? (
              <>
                {" "}
                <Badge variant="secondary" className="align-middle font-mono text-[10px]">
                  uid {user.uid}
                </Badge>
              </>
            ) : null}
            {clipboard?.paths.length ? (
              <>
                {" "}
                <Badge variant="outline" className="align-middle text-[10px]">
                  {clipboard.mode === "cut" ? "Cut" : "Copied"}{" "}
                  {clipboard.paths.length}
                </Badge>
              </>
            ) : null}
          </p>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          {inTrash ? (
            <Button
              variant="outline"
              size="sm"
              onClick={() => setDialog("empty-trash")}
              disabled={busy || filtered.length === 0}
            >
              <Trash2 className="size-4" />
              Empty trash
            </Button>
          ) : (
            <ButtonGroup>
              <Button
                variant="outline"
                size="sm"
                onClick={() => {
                  setDialog("mkdir")
                  setNameInput("")
                }}
              >
                <FolderPlus className="size-4" />
                New folder
              </Button>
              <Button
                variant="outline"
                size="sm"
                onClick={() => {
                  setDialog("create")
                  setNameInput("")
                }}
              >
                <FilePlus className="size-4" />
                New file
              </Button>
              <Button
                variant="outline"
                size="sm"
                onClick={() => fileInputRef.current?.click()}
                disabled={uploadMutation.isPending}
              >
                <Upload className="size-4" />
                Upload
              </Button>
            </ButtonGroup>
          )}
          <input
            ref={fileInputRef}
            type="file"
            className="hidden"
            multiple
            onChange={(e) => {
              const files = Array.from(e.target.files ?? [])
              e.target.value = ""
              for (const f of files) uploadMutation.mutate(f)
            }}
          />
        </div>
      </div>

      <div className="flex min-h-0 flex-1 overflow-hidden rounded-xl border bg-background">
        <aside className="hidden w-52 shrink-0 border-r bg-muted/20 md:flex md:flex-col">
          <div className="px-3 py-2 text-xs font-medium tracking-wide text-muted-foreground uppercase">
            Places
          </div>
          <ScrollArea className="min-h-0 flex-1 px-2">
            <div className="space-y-3 pb-2">
              <div className="space-y-0.5">
                {placeRoots.map((root) => (
                  <RootButton
                    key={root.path}
                    root={root}
                    active={currentPath === root.path}
                    onClick={() => navigateTo(root.path)}
                  />
                ))}
              </div>
              {diskRoots.length > 0 ? (
                <div className="space-y-0.5">
                  <div className="px-2 py-1 text-[10px] font-medium tracking-wide text-muted-foreground uppercase">
                    Disks
                  </div>
                  {diskRoots.map((root) => (
                    <RootButton
                      key={root.path}
                      root={root}
                      active={
                        currentPath === root.path ||
                        currentPath.startsWith(
                          root.path.endsWith("/") ? root.path : `${root.path}/`,
                        )
                      }
                      onClick={() => navigateTo(root.path)}
                    />
                  ))}
                </div>
              ) : null}
            </div>
          </ScrollArea>
          {trashRoot ? (
            <div className="shrink-0 border-t px-2 py-2">
              <RootButton
                root={trashRoot}
                active={inTrash || currentPath === trashRoot.path}
                onClick={() => navigateTo(trashRoot.path)}
              />
            </div>
          ) : null}
        </aside>

        <div className="relative flex min-w-0 flex-1 flex-col">
          <SessionTabs
            sessions={sessions.sessions}
            activeId={sessions.activeId}
            dirtyIds={dirtyIds}
            onSelect={(id) => switchToSession(id)}
            onClose={(id) => {
              const closing = sessions.sessions.find((s) => s.id === id)
              const idx = sessions.sessions.findIndex((s) => s.id === id)
              const wasActive = id === sessions.activeId
              sessions.closeSession(id)
              if (!wasActive) return
              const remaining = sessions.sessions.filter((s) => s.id !== id)
              const next =
                remaining[Math.max(0, idx - 1)] ?? remaining[0] ?? closing
              if (next && next.id !== id) {
                setPath(next.path)
                setSelected(new Set())
              }
            }}
            onAdd={() => {
              // New tabs open at home/root, not the current folder.
              sessions.addSession("")
              setPath("")
              setSelected(new Set())
              setSearch("")
              setPathEditing(false)
            }}
          />
          <div className="flex flex-wrap items-center gap-2 border-b px-3 py-2">
            <Button
              variant="ghost"
              size="icon-sm"
              disabled={!data?.parent}
              onClick={() => data?.parent && navigateTo(data.parent)}
              title="Up"
            >
              <ArrowUp className="size-4" />
            </Button>
            <Button
              variant="ghost"
              size="icon-sm"
              onClick={() => invalidate()}
              title="Refresh"
            >
              <RefreshCw
                className={cn("size-4", listQuery.isFetching && "animate-spin")}
              />
            </Button>
            <Separator orientation="vertical" className="mx-1 h-5" />

            {pathEditing ? (
              <div className="flex min-w-0 flex-1 items-center gap-1">
                <input
                  ref={pathInputRef}
                  value={pathDraft}
                  onChange={(e) => setPathDraft(e.target.value)}
                  onKeyDown={(e) => {
                    if (e.key === "Enter") {
                      e.preventDefault()
                      commitPathEdit()
                    } else if (e.key === "Escape") {
                      e.preventDefault()
                      cancelPathEdit()
                    }
                  }}
                  placeholder="/path/to/folder"
                  className={cn(
                    "h-8 min-w-0 flex-1 rounded-lg border border-input bg-transparent px-2.5 py-1 font-mono text-xs outline-none",
                    "focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50"
                  )}
                  aria-label="Path"
                />
                <Button
                  variant="ghost"
                  size="icon-sm"
                  title="Go"
                  onClick={() => commitPathEdit()}
                >
                  <Check className="size-4" />
                </Button>
                <Button
                  variant="ghost"
                  size="icon-sm"
                  title="Cancel"
                  onClick={() => cancelPathEdit()}
                >
                  <X className="size-4" />
                </Button>
              </div>
            ) : (
              <div className="flex min-w-0 flex-1 items-center gap-1 overflow-hidden">
                <button
                  type="button"
                  className="min-w-0 flex-1 rounded-md px-1.5 py-1 text-left hover:bg-muted/60"
                  onClick={() => startPathEdit()}
                  title="Click to edit path"
                >
                  <Breadcrumb className="min-w-0 overflow-hidden">
                    <BreadcrumbList className="flex-nowrap overflow-x-auto">
                      {crumbs.map((c, i) => (
                        <BreadcrumbItem key={c.path} className="shrink-0">
                          {i > 0 ? <BreadcrumbSeparator /> : null}
                          {i === crumbs.length - 1 ? (
                            <BreadcrumbPage className="font-mono text-xs">
                              {c.label}
                            </BreadcrumbPage>
                          ) : (
                            <BreadcrumbLink
                              className="cursor-pointer font-mono text-xs"
                              onClick={(e) => {
                                e.preventDefault()
                                e.stopPropagation()
                                navigateTo(c.path)
                              }}
                            >
                              {c.label}
                            </BreadcrumbLink>
                          )}
                        </BreadcrumbItem>
                      ))}
                    </BreadcrumbList>
                  </Breadcrumb>
                </button>
                <Button
                  variant="ghost"
                  size="icon-sm"
                  title="Edit path"
                  onClick={() => startPathEdit()}
                >
                  <Pencil className="size-3.5" />
                </Button>
              </div>
            )}

            <div className="relative w-full max-w-[200px] sm:w-48">
              <Search className="pointer-events-none absolute top-1/2 left-2 size-3.5 -translate-y-1/2 text-muted-foreground" />
              <Input
                value={search}
                onChange={(e) => setSearch(e.target.value)}
                placeholder="Filter…"
                className="h-8 pl-7"
              />
            </div>
            <Button
              variant="ghost"
              size="icon-sm"
              onClick={() => setShowHidden((v) => !v)}
              title={showHidden ? "Hide hidden files" : "Show hidden files"}
            >
              {showHidden ? <Eye className="size-4" /> : <EyeOff className="size-4" />}
            </Button>
            <ToggleGroup
              type="single"
              value={view}
              onValueChange={(v) => v && setView(v as ViewMode)}
              size="sm"
            >
              <ToggleGroupItem value="list" aria-label="List view">
                <ListIcon className="size-4" />
              </ToggleGroupItem>
              <ToggleGroupItem value="grid" aria-label="Grid view">
                <Grid2X2 className="size-4" />
              </ToggleGroupItem>
            </ToggleGroup>
          </div>

          <PaneContextMenu
            canPaste={canPaste}
            onPaste={() => clipboardPaste()}
            onNewFolder={() => {
              setDialog("mkdir")
              setNameInput("")
            }}
            onNewFile={() => {
              setDialog("create")
              setNameInput("")
            }}
            onRefresh={() => invalidate()}
            onOpenInTerminal={
              !inTrash ? () => openFolderInTerminal(currentPath) : undefined
            }
            onOpenInVSCode={
              !inTrash && vscodeInstalled
                ? () => void openFolderInVSCode(currentPath)
                : undefined
            }
          >
            <div
              className={cn(
                "relative min-h-0 flex-1 overflow-auto",
                isRefreshing && "opacity-70 transition-opacity",
              )}
            >
            {isInitialLoad ? (
              <div className="flex h-full items-center justify-center p-8 text-sm text-muted-foreground">
                Loading…
              </div>
            ) : listQuery.isError && !data ? (
              <div className="flex h-full flex-col items-center justify-center gap-2 p-8 text-center">
                <p className="text-sm text-destructive">
                  {(listQuery.error as Error)?.message || "Failed to list directory"}
                </p>
                <Button size="sm" variant="outline" onClick={() => invalidate()}>
                  Retry
                </Button>
              </div>
            ) : data && !data.exists && currentPath === data.path ? (
              <div className="flex h-full flex-col items-center justify-center gap-2 p-8 text-center text-sm text-muted-foreground">
                Path does not exist or is not accessible.
                <Button size="sm" variant="outline" onClick={() => startPathEdit()}>
                  Edit path
                </Button>
              </div>
            ) : filtered.length === 0 && !parentPath ? (
              <div className="flex h-full flex-col items-center justify-center gap-2 p-8 text-center text-sm text-muted-foreground">
                {inTrash ? "Trash is empty. Items are kept for 30 days." : "This folder is empty."}
              </div>
            ) : view === "list" ? (
              <table className="w-full table-fixed text-sm">
                <thead className="sticky top-0 z-10 bg-background/95 backdrop-blur">
                  <tr className="border-b text-left text-xs text-muted-foreground">
                    <th className="w-10 px-3 py-2">
                      <Checkbox
                        checked={allFilteredSelected}
                        onCheckedChange={(v) => {
                          if (v) {
                            setSelected(new Set(filtered.map((e) => e.path)))
                          } else {
                            setSelected(new Set())
                          }
                        }}
                        aria-label="Select all"
                      />
                    </th>
                    <th
                      className="min-w-[250px] px-2 py-2 font-medium"
                      style={{ width: "100%", minWidth: NAME_COL_MIN }}
                    >
                      <SortHeader
                        label="Name"
                        active={sortKey === "name"}
                        dir={sortDir}
                        onClick={() => toggleSort("name")}
                      />
                    </th>
                    <th
                      className="relative hidden px-2 py-2 font-medium md:table-cell"
                      style={{ width: colWidths.size, minWidth: META_COL_MIN.size }}
                    >
                      <SortHeader
                        label="Size"
                        active={sortKey === "size"}
                        dir={sortDir}
                        onClick={() => toggleSort("size")}
                      />
                      <ColumnResizeHandle
                        label="Size"
                        onResizeStart={(x) => onColResizeStart("size", x)}
                      />
                    </th>
                    <th
                      className="relative hidden px-2 py-2 font-medium lg:table-cell"
                      style={{
                        width: colWidths.modified,
                        minWidth: META_COL_MIN.modified,
                      }}
                    >
                      <SortHeader
                        label="Modified"
                        active={sortKey === "modified"}
                        dir={sortDir}
                        onClick={() => toggleSort("modified")}
                      />
                      <ColumnResizeHandle
                        label="Modified"
                        onResizeStart={(x) => onColResizeStart("modified", x)}
                      />
                    </th>
                    <th
                      className="relative hidden px-2 py-2 font-medium xl:table-cell"
                      style={{
                        width: colWidths.owner,
                        minWidth: META_COL_MIN.owner,
                      }}
                    >
                      <SortHeader
                        label="Owner"
                        active={sortKey === "owner"}
                        dir={sortDir}
                        onClick={() => toggleSort("owner")}
                      />
                      <ColumnResizeHandle
                        label="Owner"
                        onResizeStart={(x) => onColResizeStart("owner", x)}
                      />
                    </th>
                    <th
                      className="relative hidden px-2 py-2 font-medium sm:table-cell"
                      style={{ width: colWidths.mode, minWidth: META_COL_MIN.mode }}
                    >
                      <SortHeader
                        label="Mode"
                        active={sortKey === "mode"}
                        dir={sortDir}
                        onClick={() => toggleSort("mode")}
                      />
                      <ColumnResizeHandle
                        label="Mode"
                        onResizeStart={(x) => onColResizeStart("mode", x)}
                      />
                    </th>
                    <th className="w-10 px-2 py-2" />
                  </tr>
                </thead>
                <tbody>
                  {parentPath ? (
                    <tr
                      className="border-b border-border/60 hover:bg-muted/40"
                      onDoubleClick={() => navigateTo(parentPath)}
                    >
                      <td className="px-3 py-1.5" />
                      <td className="px-2 py-1.5" colSpan={5}>
                        <button
                          type="button"
                          className="flex max-w-full items-center gap-2 text-left hover:underline"
                          onClick={() => navigateTo(parentPath)}
                        >
                          <FolderOpen className="size-4 shrink-0 text-muted-foreground" />
                          <span className="truncate font-medium text-muted-foreground">
                            …
                          </span>
                        </button>
                      </td>
                      <td className="px-2 py-1.5" />
                    </tr>
                  ) : null}
                  {filtered.map((entry) => (
                    <EntryContextMenu
                      key={entry.path}
                      entry={entry}
                      actions={entryActions(entry)}
                    >
                      <tr
                        className={cn(
                          "border-b border-border/60 hover:bg-muted/40",
                          selected.has(entry.path) && "bg-muted/50",
                          clipboard?.mode === "cut" &&
                            clipboard.paths.includes(entry.path) &&
                            "opacity-50",
                        )}
                        onDoubleClick={() => openEntry(entry)}
                      >
                        <td className="px-3 py-1.5">
                          <Checkbox
                            checked={selected.has(entry.path)}
                            onCheckedChange={() => toggleSelect(entry.path)}
                            aria-label={`Select ${entry.name}`}
                          />
                        </td>
                        <td
                          className="min-w-0 px-2 py-1.5"
                          style={{ minWidth: NAME_COL_MIN }}
                        >
                          <button
                            type="button"
                            className="flex w-full min-w-0 max-w-full items-center gap-2 text-left hover:underline"
                            onClick={() => openEntry(entry)}
                          >
                            {entryIcon(entry, "size-4 shrink-0")}
                            <TruncatedName name={entry.name} />
                            {entry.hidden ? (
                              <Badge
                                variant="outline"
                                className="shrink-0 text-[10px]"
                              >
                                hidden
                              </Badge>
                            ) : null}
                            {trashIdFromEntry(entry) ? (
                              <Badge
                                variant="secondary"
                                className="shrink-0 text-[10px]"
                              >
                                trash
                              </Badge>
                            ) : null}
                          </button>
                        </td>
                        <td className="hidden whitespace-nowrap px-2 py-1.5 font-mono text-xs text-muted-foreground md:table-cell">
                          {entry.type === "directory" ? "—" : formatBytes(entry.size)}
                        </td>
                        <td className="hidden whitespace-nowrap px-2 py-1.5 text-xs text-muted-foreground lg:table-cell">
                          {formatModTime(entry.mod_time)}
                        </td>
                        <td className="hidden truncate whitespace-nowrap px-2 py-1.5 font-mono text-xs text-muted-foreground xl:table-cell">
                          {entry.owner || "—"}
                          {entry.group ? `:${entry.group}` : ""}
                        </td>
                        <td className="hidden whitespace-nowrap px-2 py-1.5 font-mono text-xs text-muted-foreground sm:table-cell">
                          {entry.mode_octal}
                        </td>
                        <td className="px-2 py-1.5">
                          <EntryMenu
                            entry={entry}
                            inTrash={inTrash || Boolean(trashIdFromEntry(entry))}
                            onOpen={() => openEntry(entry)}
                            onOpenAtEditor={() => void openEdit(entry)}
                            onOpenInTerminal={
                              entry.type === "directory"
                                ? () => openFolderInTerminal(entry.path)
                                : undefined
                            }
                            onOpenInVSCode={
                              entry.type === "directory" && vscodeInstalled
                                ? () => void openFolderInVSCode(entry.path)
                                : undefined
                            }
                            onRename={() => {
                              setTarget(entry)
                              setOpPaths([entry.path])
                              setNameInput(entry.name)
                              setDialog("rename")
                            }}
                            onDuplicate={() => duplicateMutation.mutate(entry.path)}
                            onMove={() => {
                              setTarget(entry)
                              setOpPaths([entry.path])
                              setDestInput(data?.parent || currentPath)
                              setDialog("move")
                            }}
                            onCopy={() => {
                              setTarget(entry)
                              setOpPaths([entry.path])
                              setDestInput(currentPath)
                              setDialog("copy")
                            }}
                            onCopyClipboard={() => clipboardCopy(entry)}
                            onCutClipboard={() => clipboardCut(entry)}
                            onChmod={() => {
                              setTarget(entry)
                              setOpPaths([entry.path])
                              setModeInput(entry.mode_octal || "0644")
                              setDialog("chmod")
                            }}
                            onDownload={() => {
                              const paths = resolveActionPaths(entry)
                              const items = entries.filter((e) =>
                                paths.includes(e.path),
                              )
                              void downloadEntries(
                                items.length ? items : [entry],
                              ).catch((err) =>
                                toastRequestError(err, "Download failed"),
                              )
                            }}
                            onDelete={() => {
                              setTarget(entry)
                              setOpPaths([entry.path])
                              setSelected(new Set())
                              setDialog("delete")
                            }}
                            onRestore={() => {
                              const id = trashIdFromEntry(entry)
                              if (id) restoreMutation.mutate([id])
                            }}
                            onEdit={() => void openEdit(entry)}
                          />
                        </td>
                      </tr>
                    </EntryContextMenu>
                  ))}
                </tbody>
              </table>
            ) : (
              <div className="grid grid-cols-2 gap-2 p-3 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6">
                {parentPath ? (
                  <button
                    type="button"
                    className="flex flex-col items-center gap-2 rounded-lg border border-dashed border-border/80 p-3 text-center hover:bg-muted/40"
                    onClick={() => navigateTo(parentPath)}
                    onDoubleClick={() => navigateTo(parentPath)}
                  >
                    <FolderOpen className="size-10 text-muted-foreground" />
                    <span className="text-xs font-medium text-muted-foreground">
                      …
                    </span>
                  </button>
                ) : null}
                {filtered.map((entry) => (
                  <EntryContextMenu
                    key={entry.path}
                    entry={entry}
                    actions={entryActions(entry)}
                  >
                    <div
                      className={cn(
                        "group relative flex flex-col items-center gap-2 rounded-lg border border-transparent p-3 text-center hover:border-border hover:bg-muted/40",
                        selected.has(entry.path) && "border-border bg-muted/50",
                        clipboard?.mode === "cut" &&
                          clipboard.paths.includes(entry.path) &&
                          "opacity-50",
                      )}
                      onDoubleClick={() => openEntry(entry)}
                    >
                      <div className="absolute top-2 left-2">
                        <Checkbox
                          checked={selected.has(entry.path)}
                          onCheckedChange={() => toggleSelect(entry.path)}
                          aria-label={`Select ${entry.name}`}
                        />
                      </div>
                      <div className="absolute top-1 right-1 opacity-0 group-hover:opacity-100">
                        <EntryMenu
                          entry={entry}
                          inTrash={inTrash || Boolean(trashIdFromEntry(entry))}
                          onOpen={() => openEntry(entry)}
                          onOpenAtEditor={() => void openEdit(entry)}
                          onOpenInTerminal={
                            entry.type === "directory"
                              ? () => openFolderInTerminal(entry.path)
                              : undefined
                          }
                          onOpenInVSCode={
                            entry.type === "directory" && vscodeInstalled
                              ? () => void openFolderInVSCode(entry.path)
                              : undefined
                          }
                          onRename={() => {
                            setTarget(entry)
                            setOpPaths([entry.path])
                            setNameInput(entry.name)
                            setDialog("rename")
                          }}
                          onDuplicate={() => duplicateMutation.mutate(entry.path)}
                          onMove={() => {
                            setTarget(entry)
                            setOpPaths([entry.path])
                            setDestInput(data?.parent || currentPath)
                            setDialog("move")
                          }}
                          onCopy={() => {
                            setTarget(entry)
                            setOpPaths([entry.path])
                            setDestInput(currentPath)
                            setDialog("copy")
                          }}
                          onCopyClipboard={() => clipboardCopy(entry)}
                          onCutClipboard={() => clipboardCut(entry)}
                          onChmod={() => {
                            setTarget(entry)
                            setOpPaths([entry.path])
                            setModeInput(entry.mode_octal || "0644")
                            setDialog("chmod")
                          }}
                          onDownload={() => {
                            const paths = resolveActionPaths(entry)
                            const items = entries.filter((e) =>
                              paths.includes(e.path),
                            )
                            void downloadEntries(
                              items.length ? items : [entry],
                            ).catch((err) =>
                              toastRequestError(err, "Download failed"),
                            )
                          }}
                          onDelete={() => {
                            setTarget(entry)
                            setOpPaths([entry.path])
                            setSelected(new Set())
                            setDialog("delete")
                          }}
                          onRestore={() => {
                            const id = trashIdFromEntry(entry)
                            if (id) restoreMutation.mutate([id])
                          }}
                          onEdit={() => void openEdit(entry)}
                        />
                      </div>
                      <button
                        type="button"
                        className="mt-2 flex flex-col items-center gap-2"
                        onClick={() => openEntry(entry)}
                      >
                        {entryIcon(entry, "size-10")}
                        <span className="line-clamp-2 w-full text-xs font-medium break-all">
                          {entry.name}
                        </span>
                      </button>
                    </div>
                  </EntryContextMenu>
                ))}
              </div>
            )}
            </div>
          </PaneContextMenu>

          <div className="flex items-center justify-between border-t px-3 py-1.5 text-xs text-muted-foreground">
            <span>
              {filtered.length}
              {data?.truncated ? "+" : ""} item{filtered.length === 1 ? "" : "s"}
              {data?.truncated ? " (truncated)" : ""}
              {selected.size > 0 ? ` · ${selected.size} selected` : ""}
            </span>
            <span className="font-mono">{currentPath}</span>
          </div>

          <FileSelectionBar
            selected={selected}
            entries={entries}
            busy={busy}
            inTrash={inTrash}
            onAction={handleSelectionAction}
          />
        </div>
      </div>

      {/* Create / rename dialogs */}
      <Dialog
        open={dialog === "mkdir" || dialog === "create" || dialog === "rename"}
        onOpenChange={(o) => !o && closeDialog()}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>
              {dialog === "mkdir"
                ? "New folder"
                : dialog === "create"
                  ? "New file"
                  : "Rename"}
            </DialogTitle>
            <DialogDescription>
              {dialog === "rename"
                ? `Rename ${target?.name ?? ""}`
                : `Created under ${currentPath}`}
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-2">
            <Label htmlFor="fm-name">Name</Label>
            <Input
              id="fm-name"
              value={nameInput}
              onChange={(e) => setNameInput(e.target.value)}
              placeholder={dialog === "mkdir" ? "folder-name" : "file.txt"}
              autoFocus
              onKeyDown={(e) => {
                if (e.key === "Enter" && nameInput.trim()) {
                  if (dialog === "mkdir") mkdirMutation.mutate()
                  else if (dialog === "create") createMutation.mutate()
                  else if (dialog === "rename") renameMutation.mutate()
                }
              }}
            />
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={closeDialog}>
              Cancel
            </Button>
            <Button
              disabled={!nameInput.trim() || busy}
              onClick={() => {
                if (dialog === "mkdir") mkdirMutation.mutate()
                else if (dialog === "create") createMutation.mutate()
                else if (dialog === "rename") renameMutation.mutate()
              }}
            >
              {dialog === "rename" ? "Rename" : "Create"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <DestinationPickerDialog
        open={dialog === "move" || dialog === "copy"}
        mode={dialog === "move" ? "move" : "copy"}
        source={target}
        sources={
          opPaths && opPaths.length > 1
            ? entries.filter((e) => opPaths.includes(e.path))
            : undefined
        }
        initialPath={destInput || currentPath}
        roots={roots}
        busy={busy}
        onOpenChange={(o) => !o && closeDialog()}
        onConfirm={(destinationFolder) => {
          setDestInput(destinationFolder)
          if (dialog === "move") moveMutation.mutate(destinationFolder)
          else copyMutation.mutate(destinationFolder)
        }}
      />

      <Dialog open={dialog === "chmod"} onOpenChange={(o) => !o && closeDialog()}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Permissions</DialogTitle>
            <DialogDescription>
              Change mode for{" "}
              {(opPaths?.length ?? 0) > 1
                ? `${opPaths!.length} selected items`
                : target?.name}{" "}
              (octal, e.g. 0644 or 0755)
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-2">
            <Label htmlFor="fm-mode">Mode</Label>
            <Input
              id="fm-mode"
              value={modeInput}
              onChange={(e) => setModeInput(e.target.value)}
              placeholder="0644"
              className="font-mono"
              autoFocus
            />
            <div className="flex flex-wrap gap-2 pt-1">
              {["0644", "0755", "0600", "0700", "0664", "0775"].map((m) => (
                <Button
                  key={m}
                  type="button"
                  size="sm"
                  variant={modeInput === m ? "default" : "outline"}
                  className="font-mono"
                  onClick={() => setModeInput(m)}
                >
                  {m}
                </Button>
              ))}
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={closeDialog}>
              Cancel
            </Button>
            <Button
              disabled={!modeInput.trim() || busy}
              onClick={() => chmodMutation.mutate()}
            >
              <Check className="size-4" />
              Apply
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog
        open={dialog === "edit"}
        onOpenChange={(o) => !o && closeDialog()}
      >
        <DialogContent className="flex max-h-[min(90vh,52rem)] max-w-[calc(100%-2rem)] flex-col gap-4 overflow-hidden sm:max-w-4xl">
          <DialogHeader>
            <DialogTitle className="flex flex-wrap items-center gap-2 font-mono text-base">
              {target?.name}
              {previewContent !== previewBaseline ? (
                <Badge variant="secondary">unsaved</Badge>
              ) : null}
            </DialogTitle>
            <DialogDescription className="break-all font-mono text-xs">
              {target?.path}
              {previewTruncated
                ? " — truncated (too large to edit safely; open read-only)"
                : ""}
              {!previewEditable && !previewTruncated
                ? " — read-only"
                : ""}
            </DialogDescription>
          </DialogHeader>
          <TextFileEditor
            filename={target?.name || "file.txt"}
            value={previewContent}
            onChange={setPreviewContent}
            readOnly={!previewEditable}
          />
          <DialogFooter>
            <Button variant="outline" onClick={closeDialog}>
              Close
            </Button>
            {previewEditable ? (
              <Button
                disabled={
                  busy ||
                  previewContent === previewBaseline ||
                  !target?.path
                }
                onClick={() => saveMutation.mutate()}
              >
                Save
              </Button>
            ) : null}
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <AlertDialog
        open={dialog === "delete"}
        onOpenChange={(o) => !o && closeDialog()}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              {inTrash ? "Delete permanently?" : "Move to trash?"}
            </AlertDialogTitle>
            <AlertDialogDescription>
              {inTrash
                ? (opPaths?.length ?? selected.size) > 1
                  ? `Permanently delete ${opPaths?.length || selected.size} selected item(s)? This cannot be undone.`
                  : `Permanently delete “${target?.name}”? This cannot be undone.`
                : (opPaths?.length ?? selected.size) > 1
                  ? `Move ${opPaths?.length || selected.size} selected item(s) to trash? Items are kept for 30 days.`
                  : `Move “${target?.name}” to trash? Items are kept for 30 days.`}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction
              className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
              onClick={() => deleteMutation.mutate()}
            >
              {inTrash ? "Delete permanently" : "Move to trash"}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      <AlertDialog
        open={dialog === "empty-trash"}
        onOpenChange={(o) => !o && closeDialog()}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Empty trash?</AlertDialogTitle>
            <AlertDialogDescription>
              Permanently delete all items in trash? This cannot be undone.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction
              className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
              onClick={() => emptyTrashMutation.mutate()}
            >
              Empty trash
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}

function ColumnResizeHandle({
  label,
  onResizeStart,
}: {
  label: string
  onResizeStart: (clientX: number) => void
}) {
  return (
    <span
      role="separator"
      aria-orientation="vertical"
      aria-label={`Resize ${label} column`}
      tabIndex={-1}
      className="absolute inset-y-0 -end-1 z-10 w-2 cursor-col-resize touch-none"
      onPointerDown={(e) => {
        e.preventDefault()
        e.stopPropagation()
        onResizeStart(e.clientX)
      }}
      onClick={(e) => {
        e.preventDefault()
        e.stopPropagation()
      }}
    />
  )
}

function TruncatedName({ name }: { name: string }) {
  const ref = useRef<HTMLSpanElement>(null)
  const [truncated, setTruncated] = useState(false)
  const [open, setOpen] = useState(false)

  useEffect(() => {
    const el = ref.current
    if (!el) return
    const check = () => {
      setTruncated(el.scrollWidth > el.clientWidth + 1)
    }
    check()
    const ro = new ResizeObserver(check)
    ro.observe(el)
    return () => ro.disconnect()
  }, [name])

  return (
    <Tooltip open={truncated ? open : false} onOpenChange={setOpen}>
      <TooltipTrigger asChild>
        <span ref={ref} className="min-w-0 flex-1 truncate font-medium">
          {name}
        </span>
      </TooltipTrigger>
      <TooltipContent side="top" sideOffset={6} className="max-w-sm break-all">
        {name}
      </TooltipContent>
    </Tooltip>
  )
}

function SortHeader({
  label,
  active,
  dir,
  onClick,
}: {
  label: string
  active: boolean
  dir: SortDir
  onClick: () => void
}) {
  return (
    <button
      type="button"
      className={cn(
        "inline-flex items-center gap-1 transition-colors hover:text-foreground",
        active && "text-foreground",
      )}
      onClick={onClick}
    >
      {label}
      {active ? (
        dir === "asc" ? (
          <ArrowUp className="size-3.5" />
        ) : (
          <ArrowDown className="size-3.5" />
        )
      ) : (
        <ArrowUpDown className="size-3.5 opacity-40" />
      )}
    </button>
  )
}

function joinPath(dir: string, name: string) {
  if (dir === "/") return `/${name}`
  return `${dir.replace(/\/+$/, "")}/${name}`
}

function RootButton({
  root,
  active,
  onClick,
}: {
  root: FileRoot
  active: boolean
  onClick: () => void
}) {
  const Icon = rootIconMap[root.icon || ""] || Folder
  return (
    <button
      type="button"
      onClick={onClick}
      className={cn(
        "flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-left text-sm hover:bg-muted",
        active && "bg-muted font-medium"
      )}
    >
      <Icon className="size-4 shrink-0 text-muted-foreground" />
      <span className="truncate">{root.label}</span>
    </button>
  )
}

function EntryMenu({
  entry,
  inTrash,
  onOpen,
  onRename,
  onDuplicate,
  onMove,
  onCopy,
  onCopyClipboard,
  onCutClipboard,
  onChmod,
  onDownload,
  onDelete,
  onRestore,
  onEdit,
  onOpenAtEditor,
  onOpenInTerminal,
  onOpenInVSCode,
}: {
  entry: FileEntry
  inTrash?: boolean
  onOpen: () => void
  onRename: () => void
  onDuplicate: () => void
  onMove: () => void
  onCopy: () => void
  onCopyClipboard: () => void
  onCutClipboard: () => void
  onChmod: () => void
  onDownload: () => void
  onDelete: () => void
  onRestore: () => void
  onEdit: () => void
  onOpenAtEditor: () => void
  onOpenInTerminal?: () => void
  onOpenInVSCode?: () => void
}) {
  const canEdit = isTextEditableFile(entry)
  const canOpenEditor = entry.type !== "directory"
  const isDirectory = entry.type === "directory"
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="ghost" size="icon-sm">
          <MoreHorizontal className="size-4" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-52">
        {inTrash ? (
          <>
            <DropdownMenuItem onClick={onRestore}>
              <RotateCcw className="size-4" />
              Restore
            </DropdownMenuItem>
            <DropdownMenuSeparator />
            <DropdownMenuItem variant="destructive" onClick={onDelete}>
              <Trash2 className="size-4" />
              Delete permanently
            </DropdownMenuItem>
          </>
        ) : (
          <>
            <DropdownMenuItem onClick={onOpen}>
              <ChevronRight className="size-4" />
              Open
            </DropdownMenuItem>
            {canOpenEditor ? (
              <DropdownMenuItem onClick={onOpenAtEditor}>
                <FileCode className="size-4" />
                Open At Editor
              </DropdownMenuItem>
            ) : null}
            {isDirectory && onOpenInTerminal ? (
              <DropdownMenuItem onClick={onOpenInTerminal}>
                <SquareTerminal className="size-4" />
                Open in terminal
              </DropdownMenuItem>
            ) : null}
            {isDirectory && onOpenInVSCode ? (
              <DropdownMenuItem onClick={onOpenInVSCode}>
                <Code2 className="size-4" />
                Open in VS Code
              </DropdownMenuItem>
            ) : null}
            {canEdit ? (
              <DropdownMenuItem onClick={onEdit}>
                <FileText className="size-4" />
                Edit file
              </DropdownMenuItem>
            ) : null}
            <DropdownMenuSeparator />
            <DropdownMenuItem onClick={onCopyClipboard}>
              <Copy className="size-4" />
              Copy
            </DropdownMenuItem>
            <DropdownMenuItem onClick={onCutClipboard}>
              <Move className="size-4" />
              Cut
            </DropdownMenuItem>
            <DropdownMenuSeparator />
            <DropdownMenuItem onClick={onRename}>
              <Pencil className="size-4" />
              Rename
            </DropdownMenuItem>
            <DropdownMenuItem onClick={onDuplicate}>
              <Copy className="size-4" />
              Duplicate
            </DropdownMenuItem>
            <DropdownMenuItem onClick={onMove}>
              <Move className="size-4" />
              Move…
            </DropdownMenuItem>
            <DropdownMenuItem onClick={onCopy}>
              <Copy className="size-4" />
              Copy to…
            </DropdownMenuItem>
            <DropdownMenuItem onClick={onChmod}>
              <Shield className="size-4" />
              Permissions…
            </DropdownMenuItem>
            <DropdownMenuItem onClick={onDownload}>
              <Download className="size-4" />
              Download
            </DropdownMenuItem>
            <DropdownMenuSeparator />
            <DropdownMenuItem variant="destructive" onClick={onDelete}>
              <Trash2 className="size-4" />
              Move to trash
            </DropdownMenuItem>
          </>
        )}
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
