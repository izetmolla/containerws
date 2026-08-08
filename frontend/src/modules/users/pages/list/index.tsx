import { useMemo, useState } from "react"
import { Link, useNavigate } from "react-router"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import {
  createColumnHelper,
  type ColumnDef,
} from "@tanstack/react-table"
import {
  MoreHorizontal,
  Monitor,
  Plus,
  Search,
  Terminal,
} from "lucide-react"
import { toast } from "sonner"

import ContentLoader from "@/components/content-loader"
import { Avatar, AvatarFallback } from "@/components/ui/avatar"
import { Button } from "@/components/ui/button"
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
import { ReactSelect } from "@/components/ui/reactselect"
import { Label } from "@/components/ui/label"
import { getRequestErrorMessage, withError } from "@/lib/network"
import { cn, generateAvatarFallback } from "@/lib/utils"

import {
  createUser,
  deleteUser,
  getUserFormOptions,
  getUsersList,
  novncClientURL,
  openNovnc,
  startVncProfile,
  USERS_FETCH_KEY,
  type CreateUserInput,
  type UserListItem,
} from "./api"
import { asArray } from "@/lib/as-array"

type Option = { value: string; label: string }
type ActionKind = "delete" | "open-vnc" | "terminal" | null

const columnHelper = createColumnHelper<UserListItem>()

function statusClass(status: string) {
  if (status === "active") {
    return "bg-emerald-500/15 text-emerald-700 dark:text-emerald-300"
  }
  if (status === "disabled" || status === "suspended") {
    return "bg-amber-500/15 text-amber-800 dark:text-amber-300"
  }
  return "bg-muted text-muted-foreground"
}

function statusDotClass(status: string) {
  if (status === "active") return "bg-emerald-500"
  if (status === "disabled" || status === "suspended") return "bg-amber-500"
  return "bg-muted-foreground"
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

function userMatchesQuery(item: UserListItem, query: string) {
  if (!query) return true
  const haystack = [
    item.full_name,
    item.username,
    item.email,
    item.status,
    ...(item.roles || []),
    ...(item.linux_groups || []),
  ]
    .filter(Boolean)
    .join(" ")
    .toLowerCase()
  return haystack.includes(query)
}

export default function UsersListPage() {
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const [addOpen, setAddOpen] = useState(false)
  const [action, setAction] = useState<ActionKind>(null)
  const [selected, setSelected] = useState<UserListItem | null>(null)
  const [deleteLinux, setDeleteLinux] = useState(false)
  const [search, setSearch] = useState("")

  const listQuery = useQuery({
    queryKey: [USERS_FETCH_KEY, "list"],
    queryFn: getUsersList,
  })

  const optionsQuery = useQuery({
    queryKey: [USERS_FETCH_KEY, "options"],
    queryFn: getUserFormOptions,
    enabled: addOpen,
  })

  const invalidate = () => {
    void queryClient.invalidateQueries({ queryKey: [USERS_FETCH_KEY] })
  }

  const createMutation = useMutation({
    mutationFn: createUser,
    onSuccess: (res) => {
      toast.success(res.message || "User created")
      if (res.warnings?.length) {
        toast.message(res.warnings.join("; "))
      }
      setAddOpen(false)
      invalidate()
    },
    onError: (err) =>
      toast.error(getRequestErrorMessage(err, "Failed to create user")),
  })

  const deleteMutation = useMutation({
    mutationFn: (row: UserListItem) =>
      deleteUser(row.id, { deleteLinux, removeHome: deleteLinux }),
    onSuccess: (res) => {
      toast.success(res.message || "User deleted")
      closeAction()
      invalidate()
    },
    onError: (err) =>
      toast.error(getRequestErrorMessage(err, "Failed to delete user")),
  })

  const startVncMutation = useMutation({
    mutationFn: (row: UserListItem) => startVncProfile(row.id),
    onSuccess: (res) => {
      toast.success(res.message || "Opening desktop")
      closeAction()
      const url =
        res.novnc_url ||
        (selected?.vnc_session_id
          ? novncClientURL(selected.vnc_session_id)
          : "/novnc")
      openNovnc(url)
    },
    onError: (err) =>
      toast.error(getRequestErrorMessage(err, "Failed to open VNC")),
  })

  const closeAction = () => {
    setAction(null)
    setSelected(null)
    setDeleteLinux(false)
  }

  const openAction = (kind: ActionKind, row: UserListItem) => {
    setSelected(row)
    setAction(kind)
  }

  const rows = asArray(listQuery.data?.data)
  const formOpts = optionsQuery.data?.data
  const normalizedSearch = search.trim().toLowerCase()
  const filteredRows = useMemo(
    () =>
      rows.filter((row) => userMatchesQuery(row, normalizedSearch)),
    [rows, normalizedSearch]
  )

  const columns = useMemo(
    () =>
      [
        columnHelper.accessor("full_name", {
          header: "User",
          cell: ({ row }) => {
            const item = row.original
            const title =
              item.full_name?.trim() ||
              item.username ||
              item.email ||
              item.id
            const subtitle = [item.username, item.email]
              .filter(Boolean)
              .filter((v, i, arr) => arr.indexOf(v) === i && v !== title)
              .join(" · ")
            return (
              <Link
                to={`/users/${item.id}`}
                className="group flex min-w-0 items-center gap-3"
              >
                <Avatar size="default" className="rounded-lg">
                  <AvatarFallback className="rounded-lg bg-primary/10 text-xs font-medium text-primary">
                    {generateAvatarFallback(title)}
                  </AvatarFallback>
                </Avatar>
                <div className="min-w-0">
                  <div className="truncate font-medium tracking-tight group-hover:underline">
                    {title}
                  </div>
                  {subtitle ? (
                    <div className="truncate text-xs text-muted-foreground">
                      {subtitle}
                    </div>
                  ) : null}
                </div>
              </Link>
            )
          },
        }),
        columnHelper.accessor("status", {
          header: "Status",
          cell: ({ getValue }) => {
            const status = getValue() || "unknown"
            return (
              <span
                className={cn(
                  "inline-flex items-center gap-1.5 rounded-md px-2 py-1 text-xs font-medium capitalize",
                  statusClass(status)
                )}
              >
                <span
                  className={cn("size-1.5 rounded-full", statusDotClass(status))}
                  aria-hidden
                />
                {status}
              </span>
            )
          },
        }),
        columnHelper.accessor("linux_exists", {
          header: "Linux",
          cell: ({ row }) => {
            const item = row.original
            if (!item.linux_exists) {
              return (
                <span className="inline-flex items-center gap-1.5 text-xs text-muted-foreground">
                  <Terminal className="size-3.5 opacity-50" aria-hidden />
                  Not linked
                </span>
              )
            }
            return (
              <div className="min-w-0">
                <div className="flex items-center gap-1.5 text-xs font-medium">
                  <Terminal className="size-3.5 text-muted-foreground" aria-hidden />
                  {item.linux_locked ? "Locked" : "Active"}
                </div>
                <div className="mt-0.5 truncate text-[11px] text-muted-foreground max-w-44">
                  {(item.linux_groups || []).slice(0, 3).join(", ") ||
                    item.linux_shell ||
                    "—"}
                </div>
              </div>
            )
          },
        }),
        columnHelper.accessor("has_vnc", {
          header: "VNC",
          cell: ({ row }) => {
            const item = row.original
            if (!item.has_vnc) {
              return (
                <span className="inline-flex items-center gap-1.5 text-xs text-muted-foreground">
                  <Monitor className="size-3.5 opacity-50" aria-hidden />
                  None
                </span>
              )
            }
            const live =
              item.vnc_status === "active" || item.vnc_status === "running"
            return (
              <div className="min-w-0">
                <div className="flex items-center gap-1.5 text-xs font-medium capitalize">
                  <Monitor
                    className={cn(
                      "size-3.5",
                      live ? "text-emerald-500" : "text-muted-foreground"
                    )}
                    aria-hidden
                  />
                  {item.vnc_status || "Configured"}
                </div>
                <div className="mt-0.5 font-mono text-[11px] text-muted-foreground">
                  {item.vnc_port ? `:${item.vnc_port}` : "—"}
                </div>
              </div>
            )
          },
        }),
        columnHelper.accessor("roles", {
          header: "Roles",
          cell: ({ getValue }) => {
            const roles = getValue() || []
            if (!roles.length) {
              return <span className="text-xs text-muted-foreground">—</span>
            }
            const visible = roles.slice(0, 2)
            const remaining = roles.length - visible.length
            return (
              <div className="flex flex-wrap gap-1 max-w-44">
                {visible.map((r) => (
                  <span
                    key={r}
                    className="rounded-md border border-border/70 bg-muted/50 px-1.5 py-0.5 text-[11px] text-foreground/90"
                  >
                    {r}
                  </span>
                ))}
                {remaining > 0 ? (
                  <span className="rounded-md px-1.5 py-0.5 text-[11px] text-muted-foreground">
                    +{remaining}
                  </span>
                ) : null}
              </div>
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
              <div className="flex justify-end">
                <DropdownMenu>
                  <DropdownMenuTrigger asChild>
                    <Button
                      variant="ghost"
                      size="icon-sm"
                      aria-label="User actions"
                    >
                      <MoreHorizontal />
                    </Button>
                  </DropdownMenuTrigger>
                  <DropdownMenuContent align="end" className="min-w-44">
                    <DropdownMenuItem
                      onClick={() => navigate(`/users/${item.id}`)}
                    >
                      Open profile
                    </DropdownMenuItem>
                    {item.username ? (
                      <DropdownMenuItem
                        onClick={() =>
                          window.open(
                            `/shell?as_user=${encodeURIComponent(item.username)}`,
                            "_blank"
                          )
                        }
                      >
                        Open terminal
                      </DropdownMenuItem>
                    ) : null}
                    {item.has_vnc ? (
                      <DropdownMenuItem
                        onClick={() => {
                          if (item.vnc_session_id) {
                            openNovnc(novncClientURL(item.vnc_session_id))
                          } else {
                            openAction("open-vnc", item)
                          }
                        }}
                      >
                        Open noVNC
                      </DropdownMenuItem>
                    ) : null}
                    <DropdownMenuSeparator />
                    <DropdownMenuItem
                      variant="destructive"
                      onClick={() => openAction("delete", item)}
                    >
                      Delete
                    </DropdownMenuItem>
                  </DropdownMenuContent>
                </DropdownMenu>
              </div>
            )
          },
        }),
      ] as ColumnDef<UserListItem, unknown>[],
    [navigate]
  )

  return (
    <ContentLoader
      title="Users"
      description="Manage panel accounts, Linux access, and desktop sessions."
      breadcrumb={[{ label: "Users", to: "/users" }]}
      isLoading={listQuery.isLoading}
      error={withError(listQuery.error, listQuery.data)}
      showHeaderSeparator
      rightComponent={
        <Button size="sm" onClick={() => setAddOpen(true)}>
          <Plus data-icon="inline-start" />
          Add user
        </Button>
      }
    >
      <DataTable
        columns={columns}
        data={filteredRows}
        emptyMessage={
          rows.length === 0
            ? "No users yet. Add a panel user (optionally with Linux + VNC)."
            : "No users match your search."
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
                placeholder="Search users…"
                className="h-8 pl-8"
                aria-label="Search users"
              />
            </div>
            <p className="text-xs text-muted-foreground tabular-nums">
              {filteredRows.length === rows.length
                ? `${rows.length} user${rows.length === 1 ? "" : "s"}`
                : `${filteredRows.length} of ${rows.length}`}
            </p>
          </div>
        }
      />

      <AddUserDialog
        open={addOpen}
        onOpenChange={setAddOpen}
        options={formOpts}
        loadingOptions={optionsQuery.isLoading}
        pending={createMutation.isPending}
        onSubmit={(values) => createMutation.mutate(values)}
      />

      <Dialog
        open={action === "delete" && !!selected}
        onOpenChange={(next) => !next && closeAction()}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Delete user</DialogTitle>
            <DialogDescription>
              Soft-delete{" "}
              {selected?.full_name || selected?.username || "this user"} from
              the panel. Optionally remove the Linux account too.
            </DialogDescription>
          </DialogHeader>
          <label className="flex items-center gap-2 text-sm">
            <input
              type="checkbox"
              checked={deleteLinux}
              onChange={(e) => setDeleteLinux(e.target.checked)}
            />
            Also delete Linux account and home directory
          </label>
          <DialogFooter>
            <Button variant="outline" onClick={closeAction}>
              Cancel
            </Button>
            <Button
              variant="destructive"
              disabled={deleteMutation.isPending}
              onClick={() => selected && deleteMutation.mutate(selected)}
            >
              {deleteMutation.isPending ? "Deleting…" : "Delete"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog
        open={action === "open-vnc" && !!selected}
        onOpenChange={(next) => !next && closeAction()}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Start & open noVNC</DialogTitle>
            <DialogDescription>
              Start the desktop for this user and open{" "}
              <code className="text-xs">/novnc/vnc.html?session_id=…</code>
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={closeAction}>
              Cancel
            </Button>
            <Button
              disabled={startVncMutation.isPending}
              onClick={() => selected && startVncMutation.mutate(selected)}
            >
              {startVncMutation.isPending ? "Starting…" : "Open desktop"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </ContentLoader>
  )
}

function AddUserDialog({
  open,
  onOpenChange,
  options,
  loadingOptions,
  pending,
  onSubmit,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  options?: {
    groups: string[]
    common_groups: string[]
    shells: string[]
    statuses: string[]
    panel_roles: string[]
  }
  loadingOptions: boolean
  pending: boolean
  onSubmit: (values: CreateUserInput) => void
}) {
  const [username, setUsername] = useState("")
  const [email, setEmail] = useState("")
  const [firstName, setFirstName] = useState("")
  const [lastName, setLastName] = useState("")
  const [password, setPassword] = useState("")
  const [status, setStatus] = useState<Option | null>({
    value: "active",
    label: "active",
  })
  const [roles, setRoles] = useState<Option[]>([])
  const [createLinux, setCreateLinux] = useState(true)
  const [linuxGroups, setLinuxGroups] = useState<Option[]>([])
  const [linuxShell, setLinuxShell] = useState<Option | null>({
    value: "/bin/bash",
    label: "/bin/bash",
  })
  const [createVnc, setCreateVnc] = useState(true)
  const [startVnc, setStartVnc] = useState(false)
  const [vncPassword, setVncPassword] = useState("")

  const reset = () => {
    setUsername("")
    setEmail("")
    setFirstName("")
    setLastName("")
    setPassword("")
    setStatus({ value: "active", label: "active" })
    setRoles([])
    setCreateLinux(true)
    setLinuxGroups([])
    setLinuxShell({ value: "/bin/bash", label: "/bin/bash" })
    setCreateVnc(true)
    setStartVnc(false)
    setVncPassword("")
  }

  const groupOptions: Option[] = (options?.groups?.length
    ? options.groups
    : options?.common_groups || []
  ).map((g) => ({ value: g, label: g }))

  const shellOptions: Option[] = (options?.shells || ["/bin/bash"]).map(
    (s) => ({ value: s, label: s })
  )
  const statusOptions: Option[] = (options?.statuses || ["active"]).map(
    (s) => ({ value: s, label: s })
  )
  const roleOptions: Option[] = (options?.panel_roles || ["user"]).map(
    (s) => ({ value: s, label: s })
  )

  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        onOpenChange(next)
        if (!next) reset()
      }}
    >
      <DialogContent className="max-h-[90vh] overflow-y-auto sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>Add user</DialogTitle>
          <DialogDescription>
            Create a panel account, optionally provision a full Linux user
            (groups like docker/sudo), and a VNC desktop profile.
          </DialogDescription>
        </DialogHeader>

        <div className="grid gap-3">
          <div className="grid grid-cols-2 gap-3">
            <div className="grid gap-1.5">
              <Label htmlFor="u-username">Username</Label>
              <Input
                id="u-username"
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                placeholder="alice"
                autoComplete="off"
              />
            </div>
            <div className="grid gap-1.5">
              <Label htmlFor="u-email">Email</Label>
              <Input
                id="u-email"
                type="email"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                placeholder="alice@example.com"
              />
            </div>
          </div>

          <div className="grid grid-cols-2 gap-3">
            <div className="grid gap-1.5">
              <Label htmlFor="u-fn">First name</Label>
              <Input
                id="u-fn"
                value={firstName}
                onChange={(e) => setFirstName(e.target.value)}
              />
            </div>
            <div className="grid gap-1.5">
              <Label htmlFor="u-ln">Last name</Label>
              <Input
                id="u-ln"
                value={lastName}
                onChange={(e) => setLastName(e.target.value)}
              />
            </div>
          </div>

          <div className="grid gap-1.5">
            <Label htmlFor="u-pass">Panel password</Label>
            <Input
              id="u-pass"
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              autoComplete="new-password"
            />
          </div>

          <div className="grid grid-cols-2 gap-3">
            <div className="grid gap-1.5">
              <Label>Status</Label>
              <ReactSelect<Option, false>
                size="sm"
                isLoading={loadingOptions}
                options={statusOptions}
                value={status}
                onChange={(o) => setStatus(o)}
              />
            </div>
            <div className="grid gap-1.5">
              <Label>Panel roles</Label>
              <ReactSelect<Option, true>
                size="sm"
                isMulti
                isLoading={loadingOptions}
                options={roleOptions}
                value={roles}
                onChange={(o) => setRoles([...(o || [])])}
              />
            </div>
          </div>

          <div className="rounded-lg border p-3 grid gap-3">
            <label className="flex items-center gap-2 text-sm font-medium">
              <input
                type="checkbox"
                checked={createLinux}
                onChange={(e) => setCreateLinux(e.target.checked)}
              />
              Create Linux OS user
            </label>
            {createLinux ? (
              <>
                <div className="grid gap-1.5">
                  <Label>Login shell</Label>
                  <ReactSelect<Option, false>
                    size="sm"
                    options={shellOptions}
                    value={linuxShell}
                    onChange={(o) => setLinuxShell(o)}
                  />
                </div>
                <div className="grid gap-1.5">
                  <Label>Linux groups</Label>
                  <ReactSelect<Option, true>
                    size="sm"
                    isMulti
                    isLoading={loadingOptions}
                    options={groupOptions}
                    value={linuxGroups}
                    onChange={(o) => setLinuxGroups([...(o || [])])}
                    placeholder="Select from host groups…"
                  />
                  <p className="text-[11px] text-muted-foreground">
                    From this host&apos;s groups. <code>docker</code> /{" "}
                    <code>sudo</code> are created if missing.
                  </p>
                </div>
              </>
            ) : null}
          </div>

          <div className="rounded-lg border p-3 grid gap-3">
            <label className="flex items-center gap-2 text-sm font-medium">
              <input
                type="checkbox"
                checked={createVnc}
                onChange={(e) => setCreateVnc(e.target.checked)}
              />
              Create VNC / noVNC profile
            </label>
            {createVnc ? (
              <>
                <div className="grid gap-1.5">
                  <Label htmlFor="vnc-pass">VNC password</Label>
                  <Input
                    id="vnc-pass"
                    type="password"
                    value={vncPassword}
                    onChange={(e) => setVncPassword(e.target.value)}
                    placeholder="Defaults to panel password (max 8 chars)"
                    autoComplete="new-password"
                  />
                </div>
                <label className="flex items-center gap-2 text-sm">
                  <input
                    type="checkbox"
                    checked={startVnc}
                    onChange={(e) => setStartVnc(e.target.checked)}
                  />
                  Start desktop now
                </label>
              </>
            ) : null}
          </div>
        </div>

        <DialogFooter>
          <Button
            variant="outline"
            onClick={() => {
              onOpenChange(false)
              reset()
            }}
          >
            Cancel
          </Button>
          <Button
            disabled={(!username.trim() && !email.trim()) || !password || pending}
            onClick={() =>
              onSubmit({
                username: username.trim(),
                email: email.trim(),
                first_name: firstName.trim(),
                last_name: lastName.trim(),
                password,
                status: status?.value || "active",
                roles: roles.map((r) => r.value),
                is_confirmed: true,
                create_linux: createLinux,
                linux_shell: linuxShell?.value,
                linux_groups: linuxGroups.map((g) => g.value),
                linux_create_home: true,
                create_vnc: createVnc,
                vnc_password: vncPassword || undefined,
                start_vnc: startVnc,
              })
            }
          >
            {pending ? "Creating…" : "Create user"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
