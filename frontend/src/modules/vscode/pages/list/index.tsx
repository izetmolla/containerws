import { useCallback, useEffect, useMemo, useRef, useState } from "react"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import {
  createColumnHelper,
  type ColumnDef,
} from "@tanstack/react-table"
import {
  Code2,
  ExternalLink,
  FolderGit2,
  FolderOpen,
  MoreHorizontal,
  Plus,
  Search,
} from "lucide-react"
import { toast } from "sonner"

import ContentLoader from "@/components/content-loader"
import { Avatar, AvatarFallback } from "@/components/ui/avatar"
import { Button } from "@/components/ui/button"
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
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
import { getRequestErrorMessage, withError } from "@/lib/network"
import { cn, generateAvatarFallback } from "@/lib/utils"
import {
  InstallTerminal,
  type InstallTerminalStatus,
} from "@/modules/softwares/pages/single/components/install-terminal"
import type { InstallTerminalLine } from "@/modules/softwares/pages/single/api"
import { useCurrentUser } from "@/store/authorization"

import { PathPicker } from "./path-picker"
import {
  deleteCodeserverSession,
  disableCodeserverSession,
  enableCodeserverSession,
  getAvailableUsers,
  getCodeserverSessionsList,
  getCodeserverStatus,
  openCodeserverSession,
  streamCreateCodeserverSession,
  streamReactivateCodeserverSession,
  updateCodeserverSession,
  VSCODE_SESSIONS_FETCH_KEY,
  type AvailableUser,
  type CodeserverSessionListItem,
} from "./api"
import { VsCodeInstallGate } from "./install-gate"
import { asArray } from "@/lib/as-array"

type UserOption = { value: string; label: string }

type ActionKind =
  | "disable"
  | "enable"
  | "delete"
  | "edit"
  | "restart"
  | null

const columnHelper = createColumnHelper<CodeserverSessionListItem>()

function isAdminUser(roles: string[] | undefined) {
  if (!roles?.length) return false
  return roles.some((r) => {
    const base = r.includes(":") ? r.slice(0, r.indexOf(":")) : r
    return base.toLowerCase() === "admin"
  })
}

function formatUpdatedAt(value: string) {
  if (!value) return "—"
  const date = new Date(value.includes("T") ? value : value.replace(" ", "T"))
  if (Number.isNaN(date.getTime())) return value
  return new Intl.DateTimeFormat(undefined, {
    year: "numeric",
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  }).format(date)
}

function workspaceTitle(item: CodeserverSessionListItem) {
  return item.name?.trim() || folderBasename(item.path) || "workspace"
}

function folderBasename(path: string) {
  const clean = (path || "").replace(/\/+$/, "")
  if (!clean || clean === "/") return "workspace"
  const parts = clean.split("/")
  return parts[parts.length - 1] || "workspace"
}

function sessionMatchesQuery(item: CodeserverSessionListItem, query: string) {
  if (!query) return true
  const haystack = [
    item.name,
    item.full_name,
    item.username,
    item.email,
    item.status,
    item.path,
  ]
    .filter(Boolean)
    .join(" ")
    .toLowerCase()
  return haystack.includes(query)
}

export default function VsCodeSessionsListPage() {
  const queryClient = useQueryClient()
  const currentUser = useCurrentUser()
  const clientAdmin = isAdminUser(currentUser?.roles)
  const [addOpen, setAddOpen] = useState(false)
  const [action, setAction] = useState<ActionKind>(null)
  const [selected, setSelected] = useState<CodeserverSessionListItem | null>(
    null
  )
  const [search, setSearch] = useState("")
  const [openingId, setOpeningId] = useState<string | null>(null)

  const statusQuery = useQuery({
    queryKey: [VSCODE_SESSIONS_FETCH_KEY, "status"],
    queryFn: getCodeserverStatus,
    refetchInterval: (query) => {
      const data = query.state.data?.data
      if (!data) return 2_000
      if (data.installed) return false
      const queue = data.software_queue
      const queueBusy =
        Boolean(queue?.running) || (queue?.pending ?? 0) > 0
      if (queueBusy || !data.softwaresync_ready) return 2_000
      return 4_000
    },
  })
  const runtime = statusQuery.data?.data
  const isInstalled = Boolean(runtime?.installed)

  const listQuery = useQuery({
    queryKey: [VSCODE_SESSIONS_FETCH_KEY, "list"],
    queryFn: () => getCodeserverSessionsList(),
    enabled: isInstalled,
  })

  const isAdmin = Boolean(listQuery.data?.is_admin ?? clientAdmin)

  const availableQuery = useQuery({
    queryKey: [VSCODE_SESSIONS_FETCH_KEY, "available-users"],
    queryFn: getAvailableUsers,
    enabled: addOpen && isInstalled && isAdmin,
  })

  const invalidate = useCallback(() => {
    void queryClient.invalidateQueries({ queryKey: [VSCODE_SESSIONS_FETCH_KEY] })
  }, [queryClient])

  const refreshAfterInstall = () => {
    void queryClient.invalidateQueries({
      queryKey: [VSCODE_SESSIONS_FETCH_KEY, "status"],
    })
    void queryClient.invalidateQueries({
      queryKey: [VSCODE_SESSIONS_FETCH_KEY, "list"],
    })
  }

  const updateMutation = useMutation({
    mutationFn: ({
      id,
      body,
    }: {
      id: string
      body: Parameters<typeof updateCodeserverSession>[1]
    }) => updateCodeserverSession(id, body),
    onSuccess: (res) => {
      toast.success(res.message || "Workspace updated")
      closeAction()
      invalidate()
    },
    onError: (err) =>
      toast.error(getRequestErrorMessage(err, "Failed to update workspace")),
  })

  const deleteMutation = useMutation({
    mutationFn: (id: string) => deleteCodeserverSession(id),
    onSuccess: (res) => {
      toast.success(res.message || "Workspace deleted")
      closeAction()
      invalidate()
    },
    onError: (err) =>
      toast.error(getRequestErrorMessage(err, "Failed to delete workspace")),
  })

  const disableMutation = useMutation({
    mutationFn: (id: string) => disableCodeserverSession(id),
    onSuccess: (res) => {
      toast.success(res.message || "Workspace disabled")
      closeAction()
      invalidate()
    },
    onError: (err) =>
      toast.error(getRequestErrorMessage(err, "Failed to disable workspace")),
  })

  const enableMutation = useMutation({
    mutationFn: (id: string) => enableCodeserverSession(id),
    onSuccess: (res) => {
      toast.success(res.message || "Workspace enabled")
      closeAction()
      invalidate()
    },
    onError: (err) =>
      toast.error(getRequestErrorMessage(err, "Failed to enable workspace")),
  })

  const closeAction = () => {
    setAction(null)
    setSelected(null)
  }

  const openAction = (kind: ActionKind, row: CodeserverSessionListItem) => {
    setSelected(row)
    setAction(kind)
  }

  const openWorkspace = useCallback(
    async (item: CodeserverSessionListItem) => {
      if (openingId) return
      setOpeningId(item.id)
      try {
        let connectURL = item.connect_url
        if (!item.live) {
          let successURL = ""
          let ok = false
          await streamReactivateCodeserverSession(item.id, {
            onEvent: (event) => {
              if (event.type === "done") {
                ok = Boolean(event.success)
                if (event.connect_url) successURL = event.connect_url
              }
              if (event.type === "error" && event.message) {
                toast.error(event.message)
              }
            },
          })
          if (!ok) {
            toast.error("Failed to start workspace")
            return
          }
          connectURL = successURL || connectURL
          invalidate()
        }
        const res = await openCodeserverSession(item.id)
        connectURL = res.connect_url || connectURL
        if (!connectURL) {
          toast.error("No connect URL available")
          return
        }
        toast.success(res.message || "Opening VS Code")
        window.open(connectURL, "_blank", "noopener,noreferrer")
        invalidate()
      } catch (err) {
        toast.error(getRequestErrorMessage(err, "Failed to open workspace"))
      } finally {
        setOpeningId(null)
      }
    },
    [openingId, invalidate]
  )

  const rows = asArray(listQuery.data?.data)
  const myRows = useMemo(
    () => {
      const list = rows
      return currentUser?.id
        ? list.filter((r) => r.user_id === currentUser.id)
        : list
    },
    [rows, currentUser?.id]
  )
  const normalizedSearch = search.trim().toLowerCase()
  const filteredAdminRows = useMemo(
    () =>
      rows.filter((row) =>
        sessionMatchesQuery(row, normalizedSearch)
      ),
    [rows, normalizedSearch]
  )
  const rowCount = rows.length

  const columns = useMemo(
    () =>
      [
        columnHelper.accessor("name", {
          header: "Workspace",
          cell: ({ row }) => {
            const item = row.original
            return (
              <div className="min-w-0">
                <div className="truncate font-medium tracking-tight">
                  {workspaceTitle(item)}
                </div>
                <div className="truncate font-mono text-[11px] text-muted-foreground">
                  {item.path || "—"}
                </div>
              </div>
            )
          },
        }),
        columnHelper.accessor("full_name", {
          header: "User",
          cell: ({ row }) => {
            const item = row.original
            const title =
              item.full_name?.trim() ||
              item.username ||
              item.email ||
              item.user_id
            return (
              <div className="flex min-w-0 items-center gap-2">
                <Avatar size="sm" className="rounded-md">
                  <AvatarFallback className="rounded-md text-[10px]">
                    {generateAvatarFallback(title)}
                  </AvatarFallback>
                </Avatar>
                <span className="truncate text-sm">{title}</span>
              </div>
            )
          },
        }),
        columnHelper.accessor("status", {
          header: "Status",
          cell: ({ row }) => {
            const item = row.original
            return item.live ? (
              <span className="inline-flex items-center gap-1 text-[11px] text-emerald-600 dark:text-emerald-400">
                <span
                  className="size-1.5 rounded-full bg-emerald-500"
                  aria-hidden
                />
                Live
              </span>
            ) : (
              <span className="text-[11px] text-muted-foreground">Offline</span>
            )
          },
        }),
        columnHelper.accessor("updated_at", {
          header: "Updated",
          cell: ({ getValue }) => (
            <span className="whitespace-nowrap text-xs text-muted-foreground tabular-nums">
              {formatUpdatedAt(getValue())}
            </span>
          ),
        }),
        columnHelper.display({
          id: "actions",
          header: "",
          cell: ({ row }) => {
            const item = row.original
            return (
              <WorkspaceActions
                item={item}
                opening={openingId === item.id}
                onOpen={() => void openWorkspace(item)}
                onEdit={() => openAction("edit", item)}
                onRestart={() => openAction("restart", item)}
                onDisable={() => openAction("disable", item)}
                onEnable={() => openAction("enable", item)}
                onDelete={() => openAction("delete", item)}
              />
            )
          },
        }),
      ] as ColumnDef<CodeserverSessionListItem, unknown>[],
    [openingId, openWorkspace]
  )

  const userOptions: UserOption[] = (availableQuery.data?.data ?? []).map(
    (u: AvailableUser) => ({
      value: u.id,
      label: u.label,
    })
  )

  const busy =
    updateMutation.isPending ||
    deleteMutation.isPending ||
    disableMutation.isPending ||
    enableMutation.isPending

  if (statusQuery.isLoading) {
    return (
      <ContentLoader
        title="VS Code"
        description="Create and open cloud workspaces, Codespaces-style."
        breadcrumb={[{ label: "VS Code", to: "/vscode" }]}
        isLoading
        showHeaderSeparator
      >
        <div />
      </ContentLoader>
    )
  }

  if (statusQuery.error || withError(null, statusQuery.data)) {
    return (
      <ContentLoader
        title="VS Code"
        description="Create and open cloud workspaces, Codespaces-style."
        breadcrumb={[{ label: "VS Code", to: "/vscode" }]}
        error={withError(statusQuery.error, statusQuery.data)}
        showHeaderSeparator
      >
        <div />
      </ContentLoader>
    )
  }

  if (!isInstalled && runtime) {
    return (
      <ContentLoader
        title="VS Code"
        description="Create and open cloud workspaces, Codespaces-style."
        breadcrumb={[{ label: "VS Code", to: "/vscode" }]}
        showHeaderSeparator
      >
        <VsCodeInstallGate
          status={runtime}
          queue={runtime.software_queue}
          onInstalled={refreshAfterInstall}
        />
      </ContentLoader>
    )
  }

  return (
    <ContentLoader
      title="VS Code"
      description="Create workspaces from a folder or git repo, then open them in the browser."
      breadcrumb={[{ label: "VS Code", to: "/vscode" }]}
      isLoading={listQuery.isLoading}
      error={withError(listQuery.error, listQuery.data)}
      showHeaderSeparator
      rightComponent={
        <Button size="sm" onClick={() => setAddOpen(true)}>
          <Plus data-icon="inline-start" />
          New workspace
        </Button>
      }
    >
      <div className="space-y-8">
        <section className="space-y-4">
          <div className="flex items-end justify-between gap-3">
            <div>
              <h2 className="text-sm font-semibold tracking-tight">
                Your workspaces
              </h2>
              <p className="text-xs text-muted-foreground">
                Each workspace points at a folder on this machine.
              </p>
            </div>
            {myRows.length > 0 ? (
              <p className="text-xs text-muted-foreground tabular-nums">
                {myRows.length} workspace{myRows.length === 1 ? "" : "s"}
              </p>
            ) : null}
          </div>

          {myRows.length === 0 ? (
            <EmptyWorkspaces onCreate={() => setAddOpen(true)} />
          ) : (
            <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
              {myRows.map((item) => (
                <WorkspaceCard
                  key={item.id}
                  item={item}
                  opening={openingId === item.id}
                  onOpen={() => void openWorkspace(item)}
                  onEdit={() => openAction("edit", item)}
                  onRestart={() => openAction("restart", item)}
                  onDisable={() => openAction("disable", item)}
                  onEnable={() => openAction("enable", item)}
                  onDelete={() => openAction("delete", item)}
                />
              ))}
            </div>
          )}
        </section>

        {isAdmin ? (
          <section className="space-y-3">
            <div>
              <h2 className="text-sm font-semibold tracking-tight">
                All workspaces
              </h2>
              <p className="text-xs text-muted-foreground">
                Admin view across every user on this host.
              </p>
            </div>
            <DataTable
              columns={columns}
              data={filteredAdminRows}
              emptyMessage={
                rowCount === 0
                  ? "No workspaces yet."
                  : "No workspaces match your search."
              }
              toolbarStart={
                <div className="flex flex-wrap items-center gap-3">
                  <div className="relative w-full max-w-xs">
                    <Search
                      className="pointer-events-none absolute top-1/2 left-2.5 size-3.5 -translate-y-1/2 text-muted-foreground"
                      aria-hidden
                    />
                    <Input
                      value={search}
                      onChange={(e) => setSearch(e.target.value)}
                      placeholder="Search workspaces…"
                      className="h-8 pl-8"
                      aria-label="Search workspaces"
                    />
                  </div>
                  <p className="text-xs text-muted-foreground tabular-nums">
                    {filteredAdminRows.length === rowCount
                      ? `${rowCount} total`
                      : `${filteredAdminRows.length} of ${rowCount}`}
                  </p>
                </div>
              }
            />
          </section>
        ) : null}
      </div>

      <NewWorkspaceDialog
        open={addOpen}
        onOpenChange={setAddOpen}
        isAdmin={isAdmin}
        options={userOptions}
        loadingUsers={availableQuery.isLoading}
        currentUserId={currentUser?.id}
        onCreated={invalidate}
      />

      <EditWorkspaceDialog
        open={action === "edit" && !!selected}
        session={selected}
        pending={updateMutation.isPending}
        onClose={closeAction}
        onSubmit={(body) =>
          selected && updateMutation.mutate({ id: selected.id, body })
        }
      />

      <ReactivateWorkspaceDialog
        open={action === "restart" && !!selected}
        session={selected}
        onClose={closeAction}
        onDone={invalidate}
      />

      <ConfirmActionDialog
        open={action === "disable" && !!selected}
        title="Disable workspace"
        description="Disabled workspaces cannot be opened until enabled again."
        confirmLabel={disableMutation.isPending ? "Disabling…" : "Disable"}
        pending={busy}
        onClose={closeAction}
        onConfirm={() => selected && disableMutation.mutate(selected.id)}
      />

      <ConfirmActionDialog
        open={action === "enable" && !!selected}
        title="Enable workspace"
        description="Mark this workspace as active?"
        confirmLabel={enableMutation.isPending ? "Enabling…" : "Enable"}
        pending={busy}
        onClose={closeAction}
        onConfirm={() => selected && enableMutation.mutate(selected.id)}
      />

      <ConfirmActionDialog
        open={action === "delete" && !!selected}
        title="Delete workspace"
        description="Removes this workspace record and stops its editor process. The folder on disk is kept."
        confirmLabel={deleteMutation.isPending ? "Deleting…" : "Delete"}
        pending={busy}
        destructive
        onClose={closeAction}
        onConfirm={() => selected && deleteMutation.mutate(selected.id)}
      />
    </ContentLoader>
  )
}

function EmptyWorkspaces({ onCreate }: { onCreate: () => void }) {
  return (
    <div className="flex flex-col items-center justify-center rounded-xl border border-dashed bg-muted/20 px-6 py-14 text-center">
      <span className="mb-4 grid size-12 place-items-center rounded-2xl bg-sky-500/15 text-sky-600 dark:text-sky-300">
        <Code2 className="size-5" aria-hidden />
      </span>
      <h3 className="text-base font-semibold tracking-tight">
        Create your first workspace
      </h3>
      <p className="mt-1.5 max-w-md text-sm text-muted-foreground">
        Pick a folder on this machine — or clone a repository — then open VS
        Code in the browser.
      </p>
      <Button className="mt-5" onClick={onCreate}>
        <Plus data-icon="inline-start" />
        New workspace
      </Button>
    </div>
  )
}

function WorkspaceCard({
  item,
  opening,
  onOpen,
  onEdit,
  onRestart,
  onDisable,
  onEnable,
  onDelete,
}: {
  item: CodeserverSessionListItem
  opening: boolean
  onOpen: () => void
  onEdit: () => void
  onRestart: () => void
  onDisable: () => void
  onEnable: () => void
  onDelete: () => void
}) {
  return (
    <Card className="overflow-hidden">
      <CardHeader className="gap-3 border-b bg-muted/20 pb-4">
        <div className="flex items-start justify-between gap-2">
          <div className="min-w-0">
            <CardTitle className="truncate text-base">
              {workspaceTitle(item)}
            </CardTitle>
            <CardDescription className="mt-1 flex items-center gap-1.5 truncate font-mono text-[11px]">
              <FolderOpen className="size-3 shrink-0" aria-hidden />
              <span className="truncate" title={item.path}>
                {item.path || "—"}
              </span>
            </CardDescription>
          </div>
          {item.live ? (
            <span className="inline-flex shrink-0 items-center gap-1 rounded-full bg-emerald-500/10 px-2 py-0.5 text-[11px] font-medium text-emerald-700 dark:text-emerald-400">
              <span className="size-1.5 rounded-full bg-emerald-500" />
              Live
            </span>
          ) : (
            <span className="shrink-0 rounded-full bg-muted px-2 py-0.5 text-[11px] text-muted-foreground">
              Offline
            </span>
          )}
        </div>
      </CardHeader>
      <CardContent className="pt-4">
        <p className="text-xs text-muted-foreground">
          Updated {formatUpdatedAt(item.updated_at)}
        </p>
      </CardContent>
      <CardFooter className="justify-between gap-2 border-t bg-muted/10">
        <Button size="sm" disabled={opening} onClick={onOpen}>
          <ExternalLink data-icon="inline-start" />
          {opening ? "Opening…" : "Open"}
        </Button>
        <WorkspaceActions
          item={item}
          opening={opening}
          onOpen={onOpen}
          onEdit={onEdit}
          onRestart={onRestart}
          onDisable={onDisable}
          onEnable={onEnable}
          onDelete={onDelete}
          iconOnly
        />
      </CardFooter>
    </Card>
  )
}

function WorkspaceActions({
  item,
  opening,
  onOpen,
  onEdit,
  onRestart,
  onDisable,
  onEnable,
  onDelete,
  iconOnly,
}: {
  item: CodeserverSessionListItem
  opening?: boolean
  onOpen: () => void
  onEdit: () => void
  onRestart: () => void
  onDisable: () => void
  onEnable: () => void
  onDelete: () => void
  iconOnly?: boolean
}) {
  const isActive = item.status === "active"
  return (
    <div className="flex items-center justify-end gap-1">
      {!iconOnly ? (
        <Button
          variant="ghost"
          size="icon-sm"
          aria-label="Open VS Code"
          title="Open VS Code"
          disabled={opening}
          onClick={onOpen}
        >
          <ExternalLink />
        </Button>
      ) : null}
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button
            variant="ghost"
            size="icon-sm"
            aria-label="Workspace actions"
          >
            <MoreHorizontal />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end" className="min-w-48">
          <DropdownMenuItem disabled={opening} onClick={onOpen}>
            Open VS Code
          </DropdownMenuItem>
          <DropdownMenuItem onClick={onRestart}>
            {item.live ? "Restart" : "Start"}
          </DropdownMenuItem>
          <DropdownMenuItem onClick={onEdit}>
            Edit name &amp; folder
          </DropdownMenuItem>
          {isActive ? (
            <DropdownMenuItem onClick={onDisable}>Disable</DropdownMenuItem>
          ) : (
            <DropdownMenuItem onClick={onEnable}>Enable</DropdownMenuItem>
          )}
          <DropdownMenuSeparator />
          <DropdownMenuItem variant="destructive" onClick={onDelete}>
            Delete
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
    </div>
  )
}

function ConfirmActionDialog({
  open,
  title,
  description,
  confirmLabel,
  pending,
  destructive,
  onClose,
  onConfirm,
}: {
  open: boolean
  title: string
  description: string
  confirmLabel: string
  pending: boolean
  destructive?: boolean
  onClose: () => void
  onConfirm: () => void
}) {
  return (
    <Dialog open={open} onOpenChange={(next) => !next && onClose()}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
          <DialogDescription>{description}</DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <Button variant="outline" onClick={onClose}>
            Cancel
          </Button>
          <Button
            variant={destructive ? "destructive" : "default"}
            disabled={pending}
            onClick={onConfirm}
          >
            {confirmLabel}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

let reactivateLineSeq = 0
function nextReactivateLineId() {
  reactivateLineSeq += 1
  return `cs-reactivate-${reactivateLineSeq}`
}

function ReactivateWorkspaceDialog({
  open,
  session,
  onClose,
  onDone,
}: {
  open: boolean
  session: CodeserverSessionListItem | null
  onClose: () => void
  onDone: () => void
}) {
  const [phase, setPhase] = useState<"idle" | "running" | "success" | "error">(
    "idle"
  )
  const [terminalStatus, setTerminalStatus] =
    useState<InstallTerminalStatus>("idle")
  const [terminalLines, setTerminalLines] = useState<InstallTerminalLine[]>([])
  const [connectURL, setConnectURL] = useState("")
  const abortRef = useRef<AbortController | null>(null)
  const startedForId = useRef<string | null>(null)

  const title = session?.live ? "Restart workspace" : "Start workspace"
  const label = session ? workspaceTitle(session) : "workspace"

  const reset = () => {
    abortRef.current?.abort()
    abortRef.current = null
    startedForId.current = null
    setPhase("idle")
    setTerminalStatus("idle")
    setTerminalLines([])
    setConnectURL("")
  }

  const appendLine = (
    text: string,
    stream: InstallTerminalLine["stream"] = "stdout"
  ) => {
    setTerminalLines((prev) => [
      ...prev,
      { id: nextReactivateLineId(), text, stream, at: Date.now() },
    ])
  }

  const onDoneRef = useRef(onDone)

  useEffect(() => {
    onDoneRef.current = onDone
  }, [onDone])

  useEffect(() => {
    if (!open || !session) return
    if (startedForId.current === session.id) return
    startedForId.current = session.id

    const controller = new AbortController()
    abortRef.current = controller
    setPhase("running")
    setTerminalStatus("running")
    setTerminalLines([])
    setConnectURL("")
    appendLine("Connecting…", "system")

    void streamReactivateCodeserverSession(session.id, {
      signal: controller.signal,
      onEvent: (event) => {
        switch (event.type) {
          case "start":
            appendLine(event.message || "Starting…", "system")
            break
          case "system":
            if (event.line) appendLine(event.line, "system")
            break
          case "log":
            if (event.line) {
              appendLine(
                event.line,
                event.stream === "stderr" ? "stderr" : "stdout"
              )
            }
            break
          case "error":
            setPhase("error")
            setTerminalStatus("error")
            appendLine(event.message || "Failed", "stderr")
            toast.error(event.message || "Failed to start workspace")
            break
          case "done": {
            const ok = Boolean(event.success)
            if (ok) {
              setPhase("success")
              setTerminalStatus("success")
              if (event.connect_url) setConnectURL(event.connect_url)
              appendLine(event.message || "Ready", "system")
              toast.success(event.message || "Workspace ready")
              onDoneRef.current()
            } else {
              setPhase("error")
              setTerminalStatus("error")
              appendLine(event.message || "Failed", "stderr")
              toast.error(event.message || "Failed to start workspace")
            }
            break
          }
        }
      },
    }).catch((err) => {
      if (controller.signal.aborted) return
      setPhase("error")
      setTerminalStatus("error")
      const message = getRequestErrorMessage(err, "Failed to start workspace")
      appendLine(message, "stderr")
      toast.error(message)
    })

    return () => {
      controller.abort()
    }
  }, [open, session])

  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        if (phase === "running") return
        if (!next) {
          onClose()
          reset()
        }
      }}
    >
      <DialogContent className="flex max-h-[90vh] flex-col gap-0 overflow-hidden p-0 sm:max-w-2xl">
        <div className="border-b px-6 py-4">
          <DialogHeader className="text-left">
            <DialogTitle>{title}</DialogTitle>
            <DialogDescription>
              {label}
              {session?.path ? ` · ${session.path}` : ""}
            </DialogDescription>
          </DialogHeader>
        </div>
        <div className="min-h-0 flex-1 overflow-y-auto px-6 py-4">
          <InstallTerminal
            open
            status={terminalStatus}
            lines={terminalLines}
            title="Workspace log"
            className="min-h-[320px]"
          />
        </div>
        <DialogFooter className="border-t px-6 py-4">
          <Button
            variant="outline"
            disabled={phase === "running"}
            onClick={() => {
              onClose()
              reset()
            }}
          >
            Close
          </Button>
          {phase === "success" && connectURL ? (
            <Button
              onClick={() =>
                window.open(connectURL, "_blank", "noopener,noreferrer")
              }
            >
              <ExternalLink data-icon="inline-start" />
              Open in VS Code
            </Button>
          ) : null}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

function EditWorkspaceDialog({
  open,
  session,
  pending,
  onClose,
  onSubmit,
}: {
  open: boolean
  session: CodeserverSessionListItem | null
  pending: boolean
  onClose: () => void
  onSubmit: (body: { name?: string; path?: string }) => void
}) {
  const [name, setName] = useState("")
  const [path, setPath] = useState("/workspace")
  const sessionKey =
    open && session
      ? `${session.id}:${session.name ?? ""}:${session.path ?? ""}`
      : ""
  const [prevSessionKey, setPrevSessionKey] = useState(sessionKey)
  if (sessionKey !== prevSessionKey) {
    setPrevSessionKey(sessionKey)
    if (open && session) {
      setName(session.name || folderBasename(session.path))
      setPath(session.path || "/workspace")
    }
  }

  return (
    <Dialog open={open} onOpenChange={(next) => !next && onClose()}>
      <DialogContent className="sm:max-w-xl">
        <DialogHeader>
          <DialogTitle>Edit workspace</DialogTitle>
          <DialogDescription>
            Rename this workspace or point it at a different folder.
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-4 py-1">
          <div className="grid gap-1.5">
            <Label htmlFor="ws-edit-name">Name</Label>
            <Input
              id="ws-edit-name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder={folderBasename(path)}
            />
          </div>
          <div className="grid gap-1.5">
            <Label>Folder</Label>
            <PathPicker
              value={path}
              onChange={setPath}
              userId={session?.user_id}
              browseTitle="Select workspace folder"
            />
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={onClose}>
            Cancel
          </Button>
          <Button
            disabled={pending || !path.trim()}
            onClick={() =>
              onSubmit({
                name: name.trim() || folderBasename(path),
                path: path.trim() || "/workspace",
              })
            }
          >
            {pending ? "Saving…" : "Save"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

let createLineSeq = 0
function nextCreateLineId() {
  createLineSeq += 1
  return `cs-create-${createLineSeq}`
}

function NewWorkspaceDialog({
  open,
  onOpenChange,
  isAdmin,
  options,
  loadingUsers,
  currentUserId,
  onCreated,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  isAdmin: boolean
  options: UserOption[]
  loadingUsers: boolean
  currentUserId?: string
  onCreated: () => void
}) {
  const [user, setUser] = useState<UserOption | null>(null)
  const [name, setName] = useState("")
  const [path, setPath] = useState("/workspace")
  const [mode, setMode] = useState<"folder" | "clone">("folder")
  const [gitRepo, setGitRepo] = useState("")
  const [gitBranch, setGitBranch] = useState("main")
  const [gitToken, setGitToken] = useState("")
  const [phase, setPhase] = useState<"form" | "running" | "success" | "error">(
    "form"
  )
  const [terminalStatus, setTerminalStatus] =
    useState<InstallTerminalStatus>("idle")
  const [terminalLines, setTerminalLines] = useState<InstallTerminalLine[]>([])
  const [created, setCreated] = useState<CodeserverSessionListItem | null>(null)
  const [connectURL, setConnectURL] = useState("")
  const abortRef = useRef<AbortController | null>(null)
  const nameTouched = useRef(false)

  const reset = () => {
    abortRef.current?.abort()
    abortRef.current = null
    setUser(null)
    setName("")
    setPath("/workspace")
    setMode("folder")
    setGitRepo("")
    setGitBranch("main")
    setGitToken("")
    setPhase("form")
    setTerminalStatus("idle")
    setTerminalLines([])
    setCreated(null)
    setConnectURL("")
    nameTouched.current = false
  }

  const userPickKey =
    open && !user && currentUserId
      ? `${currentUserId}:${isAdmin}:${options.map((o) => o.value).join(",")}`
      : ""
  const [prevUserPickKey, setPrevUserPickKey] = useState("")
  if (!open && prevUserPickKey) {
    setPrevUserPickKey("")
  } else if (userPickKey && userPickKey !== prevUserPickKey) {
    setPrevUserPickKey(userPickKey)
    if (isAdmin) {
      const match = options.find((o) => o.value === currentUserId)
      if (match) setUser(match)
    } else if (currentUserId) {
      setUser({ value: currentUserId, label: "You" })
    }
  }

  useEffect(() => {
    if (nameTouched.current) return
    setName(folderBasename(path))
  }, [path])

  useEffect(() => {
    return () => {
      abortRef.current?.abort()
    }
  }, [])

  const appendLine = (
    text: string,
    stream: InstallTerminalLine["stream"] = "stdout"
  ) => {
    setTerminalLines((prev) => [
      ...prev,
      { id: nextCreateLineId(), text, stream, at: Date.now() },
    ])
  }

  const startCreate = async () => {
    const targetUserId = isAdmin ? user?.value : currentUserId
    if (!targetUserId || phase === "running") return
    if (mode === "clone" && !gitRepo.trim()) {
      toast.error("Enter a git repository URL")
      return
    }

    abortRef.current?.abort()
    const controller = new AbortController()
    abortRef.current = controller

    setPhase("running")
    setTerminalStatus("running")
    setTerminalLines([])
    setCreated(null)
    setConnectURL("")
    appendLine("Connecting…", "system")

    try {
      await streamCreateCodeserverSession(
        {
          user_id: targetUserId,
          name: name.trim() || folderBasename(path),
          path: path.trim() || "/workspace",
          status: "active",
          ...(mode === "clone" && gitRepo.trim()
            ? {
                git_repo: gitRepo.trim(),
                git_branch: gitBranch.trim() || undefined,
                git_token: gitToken.trim() || undefined,
              }
            : {}),
        },
        {
          signal: controller.signal,
          onEvent: (event) => {
            switch (event.type) {
              case "start":
                appendLine(event.message || "Starting…", "system")
                break
              case "system":
                if (event.line) appendLine(event.line, "system")
                break
              case "log":
                if (event.line) {
                  appendLine(
                    event.line,
                    event.stream === "stderr" ? "stderr" : "stdout"
                  )
                }
                break
              case "error":
                setPhase("error")
                setTerminalStatus("error")
                appendLine(event.message || "Create failed", "stderr")
                toast.error(event.message || "Failed to create workspace")
                break
              case "done": {
                const ok = Boolean(event.success)
                if (ok) {
                  setPhase("success")
                  setTerminalStatus("success")
                  if (event.data) setCreated(event.data)
                  if (event.connect_url) setConnectURL(event.connect_url)
                  appendLine(
                    event.message || "Workspace created successfully",
                    "system"
                  )
                  toast.success(event.message || "Workspace created")
                  onCreated()
                } else {
                  setPhase("error")
                  setTerminalStatus("error")
                  appendLine(event.message || "Create failed", "stderr")
                  toast.error(event.message || "Failed to create workspace")
                }
                break
              }
            }
          },
        }
      )
    } catch (err) {
      if (controller.signal.aborted) {
        setPhase("form")
        setTerminalStatus("cancelled")
        return
      }
      setPhase("error")
      setTerminalStatus("error")
      const message = getRequestErrorMessage(err, "Failed to create workspace")
      appendLine(message, "stderr")
      toast.error(message)
    }
  }

  const openSession = () => {
    const url =
      connectURL ||
      created?.connect_url ||
      (created?.id ? `/codeserver/${created.id}/` : "")
    if (!url) {
      toast.error("No connect URL available")
      return
    }
    window.open(url, "_blank", "noopener,noreferrer")
  }

  const formLocked = phase === "running" || phase === "success"
  const showLogs = phase !== "form"
  const canCreate =
    Boolean(isAdmin ? user?.value : currentUserId) &&
    Boolean(path.trim()) &&
    (mode === "folder" || Boolean(gitRepo.trim())) &&
    !formLocked

  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        if (phase === "running") return
        onOpenChange(next)
        if (!next) reset()
      }}
    >
      <DialogContent
        className={cn(
          "flex h-[min(94vh,920px)] max-h-[min(96vh,960px)] w-[calc(100%-1.5rem)] flex-col gap-0 overflow-hidden p-0",
          "sm:max-w-3xl"
        )}
        showCloseButton={phase !== "running"}
      >
        <div className="border-b bg-gradient-to-br from-sky-500/10 via-background to-background px-6 py-5">
          <DialogHeader className="gap-2 text-left">
            <DialogTitle className="flex items-center gap-2.5 text-lg">
              <span className="grid size-9 place-items-center rounded-xl bg-sky-500/15 text-sky-600 dark:text-sky-300">
                <Code2 className="size-4" aria-hidden />
              </span>
              New workspace
            </DialogTitle>
            <DialogDescription className="text-sm leading-relaxed">
              {showLogs
                ? `${name.trim() || folderBasename(path)} · ${path.trim() || "/workspace"}`
                : "Choose a folder on this host, or clone a repository into one."}
            </DialogDescription>
          </DialogHeader>
        </div>

        <div
          className={cn(
            "min-h-0 flex-1 overflow-y-auto px-6 py-5",
            showLogs ? "flex flex-col" : "space-y-5"
          )}
        >
          {showLogs ? (
            <InstallTerminal
              open
              status={terminalStatus}
              lines={terminalLines}
              title="Create log"
              subtitle={`${name.trim() || folderBasename(path)} · ${path.trim() || "/workspace"}`}
              className="min-h-[420px] flex-1"
              bodyClassName="h-auto min-h-0 flex-1"
              onClear={
                phase === "running" ? undefined : () => setTerminalLines([])
              }
              onRetry={
                phase === "error"
                  ? () => {
                      setPhase("form")
                      setTerminalStatus("idle")
                      setTerminalLines([])
                    }
                  : undefined
              }
            />
          ) : (
            <>
              {isAdmin ? (
                <div className="grid gap-1.5">
                  <Label>Owner</Label>
                  <ReactSelect<UserOption, false>
                    size="sm"
                    isDisabled={loadingUsers || formLocked}
                    isLoading={loadingUsers}
                    options={options}
                    value={user}
                    onChange={(o) => setUser(o)}
                    placeholder={
                      loadingUsers ? "Loading…" : "Select a panel user"
                    }
                  />
                </div>
              ) : null}

              <div className="grid gap-1.5">
                <Label htmlFor="ws-name">Name</Label>
                <Input
                  id="ws-name"
                  value={name}
                  disabled={formLocked}
                  onChange={(e) => {
                    nameTouched.current = true
                    setName(e.target.value)
                  }}
                  placeholder={folderBasename(path)}
                />
                <p className="text-xs text-muted-foreground">
                  Defaults to the folder name. You can rename anytime.
                </p>
              </div>

              <div className="grid gap-1.5">
                <Label>Source</Label>
                <div className="grid grid-cols-2 gap-2">
                  <Button
                    type="button"
                    variant={mode === "folder" ? "default" : "outline"}
                    className="justify-start"
                    disabled={formLocked}
                    onClick={() => setMode("folder")}
                  >
                    <FolderOpen data-icon="inline-start" />
                    Existing folder
                  </Button>
                  <Button
                    type="button"
                    variant={mode === "clone" ? "default" : "outline"}
                    className="justify-start"
                    disabled={formLocked}
                    onClick={() => setMode("clone")}
                  >
                    <FolderGit2 data-icon="inline-start" />
                    Clone repository
                  </Button>
                </div>
              </div>

              <div className="grid gap-1.5">
                <Label>Workspace folder</Label>
                <PathPicker
                  value={path}
                  onChange={setPath}
                  userId={isAdmin ? user?.value : currentUserId}
                  disabled={formLocked}
                  browseTitle={
                    mode === "clone"
                      ? "Select clone destination folder"
                      : "Select workspace folder"
                  }
                />
              </div>

              {mode === "clone" ? (
                <div className="space-y-3 rounded-lg border bg-muted/20 p-4">
                  <div className="grid gap-1.5">
                    <Label htmlFor="ws-git-repo">Repository URL</Label>
                    <Input
                      id="ws-git-repo"
                      value={gitRepo}
                      disabled={formLocked}
                      onChange={(e) => setGitRepo(e.target.value)}
                      placeholder="https://github.com/org/repo.git"
                    />
                  </div>
                  <div className="grid gap-3 sm:grid-cols-2">
                    <div className="grid gap-1.5">
                      <Label htmlFor="ws-git-branch">Branch</Label>
                      <Input
                        id="ws-git-branch"
                        value={gitBranch}
                        disabled={formLocked}
                        onChange={(e) => setGitBranch(e.target.value)}
                        placeholder="main"
                      />
                    </div>
                    <div className="grid gap-1.5">
                      <Label htmlFor="ws-git-token">Token (optional)</Label>
                      <Input
                        id="ws-git-token"
                        type="password"
                        value={gitToken}
                        disabled={formLocked}
                        onChange={(e) => setGitToken(e.target.value)}
                        placeholder="For private repos"
                        autoComplete="off"
                      />
                    </div>
                  </div>
                </div>
              ) : null}
            </>
          )}
        </div>

        <DialogFooter className="border-t px-6 py-4">
          {phase === "success" ? (
            <>
              <Button
                variant="outline"
                onClick={() => {
                  onOpenChange(false)
                  reset()
                }}
              >
                Done
              </Button>
              <Button onClick={openSession}>
                <ExternalLink data-icon="inline-start" />
                Open in VS Code
              </Button>
            </>
          ) : (
            <>
              <Button
                variant="outline"
                disabled={phase === "running"}
                onClick={() => {
                  onOpenChange(false)
                  reset()
                }}
              >
                Cancel
              </Button>
              <Button disabled={!canCreate} onClick={() => void startCreate()}>
                {phase === "running" ? "Creating…" : "Create workspace"}
              </Button>
            </>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
