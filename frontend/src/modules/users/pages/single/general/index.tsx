import { useMemo, useState } from "react"
import { Link, useNavigate, useOutletContext } from "react-router"
import { useMutation, useQuery } from "@tanstack/react-query"
import {
  Lock,
  Monitor,
  Shield,
  Terminal,
  Unlock,
  User as UserIcon,
} from "lucide-react"
import { toast } from "sonner"

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
import { ReactSelect } from "@/components/ui/reactselect"
import { getRequestErrorMessage } from "@/lib/network"
import { cn } from "@/lib/utils"

import {
  getUserFormOptions,
  lockLinux,
  novncClientURL,
  openNovnc,
  provisionLinux,
  setLinuxGroups,
  setLinuxPassword,
  setPanelPassword,
  unlockLinux,
  updateLinux,
  updateUser,
  USERS_FETCH_KEY,
  type UserDetail,
} from "../../list/api"
import type { UserSingleOutletContext } from "../types"

type Option = { value: string; label: string }

export default function UserGeneralPage() {
  const { user, id, invalidate } = useOutletContext<UserSingleOutletContext>()
  const navigate = useNavigate()
  const [panelPassOpen, setPanelPassOpen] = useState(false)
  const [linuxPassOpen, setLinuxPassOpen] = useState(false)
  const [linuxProvisionOpen, setLinuxProvisionOpen] = useState(false)

  const optionsQuery = useQuery({
    queryKey: [USERS_FETCH_KEY, "options"],
    queryFn: getUserFormOptions,
  })
  const formOpts = optionsQuery.data?.data

  const updateMutation = useMutation({
    mutationFn: (body: Parameters<typeof updateUser>[1]) =>
      updateUser(id, body),
    onSuccess: (res) => {
      toast.success(res.message || "Saved")
      invalidate()
    },
    onError: (err) =>
      toast.error(getRequestErrorMessage(err, "Failed to save")),
  })

  const panelPassMutation = useMutation({
    mutationFn: (password: string) => setPanelPassword(id, password),
    onSuccess: (res) => {
      toast.success(res.message || "Password updated")
      setPanelPassOpen(false)
    },
    onError: (err) =>
      toast.error(getRequestErrorMessage(err, "Failed to set password")),
  })

  const linuxPassMutation = useMutation({
    mutationFn: (password: string) => setLinuxPassword(id, password),
    onSuccess: (res) => {
      toast.success(res.message || "Linux password updated")
      setLinuxPassOpen(false)
      invalidate()
    },
    onError: (err) =>
      toast.error(getRequestErrorMessage(err, "Failed to set Linux password")),
  })

  const groupsMutation = useMutation({
    mutationFn: (groups: string[]) => setLinuxGroups(id, groups),
    onSuccess: (res) => {
      toast.success(res.message || "Groups updated")
      invalidate()
    },
    onError: (err) =>
      toast.error(getRequestErrorMessage(err, "Failed to update groups")),
  })

  const shellMutation = useMutation({
    mutationFn: (shell: string) => updateLinux(id, { shell }),
    onSuccess: () => {
      toast.success("Shell updated")
      invalidate()
    },
    onError: (err) =>
      toast.error(getRequestErrorMessage(err, "Failed to update shell")),
  })

  const lockMutation = useMutation({
    mutationFn: () =>
      user.linux?.locked ? unlockLinux(id) : lockLinux(id),
    onSuccess: (res) => {
      toast.success(res.message || "Updated")
      invalidate()
    },
    onError: (err) =>
      toast.error(getRequestErrorMessage(err, "Failed to lock/unlock")),
  })

  const provisionMutation = useMutation({
    mutationFn: (body: {
      password: string
      shell: string
      groups: string[]
    }) =>
      provisionLinux(id, {
        password: body.password,
        shell: body.shell,
        groups: body.groups,
        create_home: true,
      }),
    onSuccess: (res) => {
      toast.success(res.message || "Linux user created")
      setLinuxProvisionOpen(false)
      invalidate()
    },
    onError: (err) =>
      toast.error(getRequestErrorMessage(err, "Failed to create Linux user")),
  })

  const groupOptions: Option[] = useMemo(() => {
    const names = formOpts?.groups?.length
      ? formOpts.groups
      : [...(formOpts?.common_groups || []), ...(user.linux?.groups || [])]
    const seen = new Set<string>()
    const out: Option[] = []
    for (const g of names) {
      if (!g || seen.has(g)) continue
      seen.add(g)
      out.push({ value: g, label: g })
    }
    for (const g of user.linux?.groups || []) {
      if (!g || seen.has(g)) continue
      seen.add(g)
      out.push({ value: g, label: g })
    }
    return out
  }, [formOpts, user.linux?.groups])

  const shellOptions: Option[] = (formOpts?.shells || ["/bin/bash"]).map(
    (s) => ({ value: s, label: s })
  )

  return (
    <>
      <div className="grid gap-6 lg:grid-cols-[minmax(0,1fr)_320px]">
        <div className="grid min-w-0 gap-6">
          <ProfileCard
            user={user}
            pending={updateMutation.isPending}
            onSave={(body) => updateMutation.mutate(body)}
            onChangePassword={() => setPanelPassOpen(true)}
          />

          <LinuxCard
            user={user}
            groupOptions={groupOptions}
            shellOptions={shellOptions}
            onProvision={() => setLinuxProvisionOpen(true)}
            onSaveGroups={(groups) => groupsMutation.mutate(groups)}
            onSaveShell={(shell) => shellMutation.mutate(shell)}
            onPassword={() => setLinuxPassOpen(true)}
            onToggleLock={() => lockMutation.mutate()}
            pending={
              groupsMutation.isPending ||
              shellMutation.isPending ||
              lockMutation.isPending
            }
          />
        </div>

        <aside className="grid content-start gap-4">
          <div className="grid gap-3 rounded-xl border bg-card p-4">
            <h2 className="text-sm font-semibold">Quick connect</h2>
            <p className="text-xs text-muted-foreground">
              Jump to a shell as this Linux user, or open their noVNC desktop.
            </p>
            {user.username ? (
              <Button variant="outline" className="justify-start" asChild>
                <Link to={`/users/${id}/shell`}>
                  <Terminal data-icon="inline-start" />
                  Terminal as {user.username}
                </Link>
              </Button>
            ) : (
              <Button variant="outline" className="justify-start" disabled>
                <Terminal data-icon="inline-start" />
                Terminal
              </Button>
            )}
            <Button
              variant="outline"
              className="justify-start"
              disabled={!user.vnc?.live}
              onClick={() => {
                const url =
                  user.novnc_url ||
                  (user.vnc ? novncClientURL(user.vnc.id) : "")
                if (url) openNovnc(url)
              }}
            >
              <Monitor data-icon="inline-start" />
              {user.vnc?.live ? "Open noVNC" : "Desktop not running"}
            </Button>
            <Button
              variant="ghost"
              className="justify-start"
              asChild
            >
              <Link to={`/users/${id}/vnc`}>Manage VNC</Link>
            </Button>
            <Button
              variant="ghost"
              className="justify-start"
              onClick={() => navigate("/users")}
            >
              Back to list
            </Button>
          </div>

          <div className="grid gap-2 rounded-xl border bg-card p-4 text-xs text-muted-foreground">
            <div className="flex justify-between gap-2">
              <span>ID</span>
              <span className="truncate font-mono text-foreground">
                {user.id}
              </span>
            </div>
            <div className="flex justify-between gap-2">
              <span>Created</span>
              <span>{user.created_at}</span>
            </div>
            <div className="flex justify-between gap-2">
              <span>Updated</span>
              <span>{user.updated_at}</span>
            </div>
          </div>
        </aside>
      </div>

      <PasswordDialog
        open={panelPassOpen}
        title="Panel password"
        description="Updates the hashed password used for panel sign-in."
        pending={panelPassMutation.isPending}
        onCancel={() => setPanelPassOpen(false)}
        onSubmit={(p) => panelPassMutation.mutate(p)}
      />
      <PasswordDialog
        open={linuxPassOpen}
        title="Linux password"
        description="Updates /etc/shadow for this OS account via chpasswd."
        pending={linuxPassMutation.isPending}
        onCancel={() => setLinuxPassOpen(false)}
        onSubmit={(p) => linuxPassMutation.mutate(p)}
      />
      <LinuxProvisionDialog
        open={linuxProvisionOpen}
        username={user.username || ""}
        groupOptions={groupOptions}
        shellOptions={shellOptions}
        pending={provisionMutation.isPending}
        onCancel={() => setLinuxProvisionOpen(false)}
        onSubmit={(body) => provisionMutation.mutate(body)}
      />
    </>
  )
}

function ProfileCard({
  user,
  pending,
  onSave,
  onChangePassword,
}: {
  user: UserDetail
  pending: boolean
  onSave: (body: {
    first_name: string
    last_name: string
    email: string
    username: string
    status: string
    roles: string[]
  }) => void
  onChangePassword: () => void
}) {
  const [firstName, setFirstName] = useState(user.first_name || "")
  const [lastName, setLastName] = useState(user.last_name || "")
  const [email, setEmail] = useState(user.email || "")
  const [username, setUsername] = useState(user.username || "")
  const [status, setStatus] = useState(user.status || "active")
  const [roles, setRoles] = useState((user.roles || []).join(", "))
  const [prevUser, setPrevUser] = useState(user)

  if (user !== prevUser) {
    setPrevUser(user)
    setFirstName(user.first_name || "")
    setLastName(user.last_name || "")
    setEmail(user.email || "")
    setUsername(user.username || "")
    setStatus(user.status || "active")
    setRoles((user.roles || []).join(", "))
  }

  return (
    <section className="grid gap-4 rounded-xl border bg-card p-5">
      <div className="flex items-start justify-between gap-3">
        <div className="flex items-center gap-2">
          <div className="grid size-9 place-items-center rounded-lg bg-muted">
            <UserIcon className="size-4" />
          </div>
          <div>
            <h2 className="font-semibold">Panel profile</h2>
            <p className="text-xs text-muted-foreground">
              Application account used for sign-in and authorization.
            </p>
          </div>
        </div>
        <Button variant="outline" size="sm" onClick={onChangePassword}>
          Change password
        </Button>
      </div>

      <div className="grid gap-3 sm:grid-cols-2">
        <Field label="First name" value={firstName} onChange={setFirstName} />
        <Field label="Last name" value={lastName} onChange={setLastName} />
        <Field label="Username" value={username} onChange={setUsername} />
        <Field label="Email" value={email} onChange={setEmail} />
        <Field label="Status" value={status} onChange={setStatus} />
        <Field
          label="Roles (comma-separated)"
          value={roles}
          onChange={setRoles}
        />
      </div>

      <div className="flex justify-end">
        <Button
          disabled={pending}
          onClick={() =>
            onSave({
              first_name: firstName,
              last_name: lastName,
              email,
              username,
              status,
              roles: roles
                .split(",")
                .map((r) => r.trim())
                .filter(Boolean),
            })
          }
        >
          {pending ? "Saving…" : "Save profile"}
        </Button>
      </div>
    </section>
  )
}

function LinuxCard({
  user,
  groupOptions,
  shellOptions,
  onProvision,
  onSaveGroups,
  onSaveShell,
  onPassword,
  onToggleLock,
  pending,
}: {
  user: UserDetail
  groupOptions: Option[]
  shellOptions: Option[]
  onProvision: () => void
  onSaveGroups: (groups: string[]) => void
  onSaveShell: (shell: string) => void
  onPassword: () => void
  onToggleLock: () => void
  pending: boolean
}) {
  const linux = user.linux
  const [groups, setGroups] = useState<Option[]>(
    (linux?.groups || []).map((g) => ({ value: g, label: g }))
  )
  const [shell, setShell] = useState<Option | null>(
    linux?.shell ? { value: linux.shell, label: linux.shell } : null
  )
  const [prevLinux, setPrevLinux] = useState(linux)

  if (linux !== prevLinux) {
    setPrevLinux(linux)
    setGroups((linux?.groups || []).map((g) => ({ value: g, label: g })))
    setShell(linux?.shell ? { value: linux.shell, label: linux.shell } : null)
  }

  if (!user.username) {
    return (
      <section className="rounded-xl border bg-card p-5">
        <h2 className="mb-1 font-semibold">Linux account</h2>
        <p className="text-sm text-muted-foreground">
          Set a username on the panel profile before provisioning an OS user.
        </p>
      </section>
    )
  }

  if (!linux?.exists) {
    return (
      <section className="grid gap-3 rounded-xl border bg-card p-5">
        <div className="flex items-center gap-2">
          <Shield className="size-4" />
          <h2 className="font-semibold">Linux account</h2>
        </div>
        <p className="text-sm text-muted-foreground">
          No OS user named <code>{user.username}</code> yet. Create a full Linux
          account with home, shell, and groups.
        </p>
        <div>
          <Button onClick={onProvision}>Provision Linux user</Button>
        </div>
      </section>
    )
  }

  return (
    <section className="grid gap-4 rounded-xl border bg-card p-5">
      <div className="flex items-start justify-between gap-3">
        <div className="flex items-center gap-2">
          <Shield className="size-4" />
          <div>
            <h2 className="font-semibold">Linux account</h2>
            <p className="text-xs text-muted-foreground">
              uid {linux.uid} · {linux.home_dir} ·{" "}
              <span
                className={cn(
                  linux.locked ? "text-amber-600" : "text-emerald-600"
                )}
              >
                {linux.locked ? "locked" : "unlocked"}
              </span>
            </p>
          </div>
        </div>
        <div className="flex gap-2">
          <Button variant="outline" size="sm" onClick={onPassword}>
            Password
          </Button>
          <Button
            variant="outline"
            size="sm"
            onClick={onToggleLock}
            disabled={pending}
          >
            {linux.locked ? (
              <>
                <Unlock data-icon="inline-start" /> Unlock
              </>
            ) : (
              <>
                <Lock data-icon="inline-start" /> Lock
              </>
            )}
          </Button>
        </div>
      </div>

      <div className="grid gap-3 sm:grid-cols-2">
        <div className="grid gap-1.5">
          <Label>Shell</Label>
          <ReactSelect<Option, false>
            size="sm"
            options={shellOptions}
            value={shell}
            onChange={(o) => setShell(o)}
          />
        </div>
        <div className="grid gap-1.5 sm:col-span-2">
          <Label>Groups</Label>
          <ReactSelect<Option, true>
            size="sm"
            isMulti
            options={groupOptions}
            value={groups}
            onChange={(o) => setGroups([...(o || [])])}
          />
          <p className="text-[11px] text-muted-foreground">
            Groups are read from this host&apos;s <code>/etc/group</code>.
            Suggested groups like <code>docker</code> / <code>sudo</code> are
            created automatically if missing when you save.
          </p>
        </div>
      </div>

      <div className="flex justify-end gap-2">
        <Button
          variant="outline"
          disabled={!shell || pending}
          onClick={() => shell && onSaveShell(shell.value)}
        >
          Save shell
        </Button>
        <Button
          disabled={pending}
          onClick={() => onSaveGroups(groups.map((g) => g.value))}
        >
          Save groups
        </Button>
      </div>
    </section>
  )
}

function Field({
  label,
  value,
  onChange,
}: {
  label: string
  value: string
  onChange: (v: string) => void
}) {
  const id = label.toLowerCase().replace(/\s+/g, "-")
  return (
    <div className="grid gap-1.5">
      <Label htmlFor={id}>{label}</Label>
      <Input id={id} value={value} onChange={(e) => onChange(e.target.value)} />
    </div>
  )
}

function PasswordDialog({
  open,
  title,
  description,
  pending,
  onCancel,
  onSubmit,
}: {
  open: boolean
  title: string
  description: string
  pending: boolean
  onCancel: () => void
  onSubmit: (password: string) => void
}) {
  const [password, setPassword] = useState("")
  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        if (!next) {
          setPassword("")
          onCancel()
        }
      }}
    >
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
          <DialogDescription>{description}</DialogDescription>
        </DialogHeader>
        <Input
          type="password"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          autoComplete="new-password"
        />
        <DialogFooter>
          <Button variant="outline" onClick={onCancel}>
            Cancel
          </Button>
          <Button
            disabled={!password.trim() || pending}
            onClick={() => onSubmit(password)}
          >
            {pending ? "Saving…" : "Save"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

function LinuxProvisionDialog({
  open,
  username,
  groupOptions,
  shellOptions,
  pending,
  onCancel,
  onSubmit,
}: {
  open: boolean
  username: string
  groupOptions: Option[]
  shellOptions: Option[]
  pending: boolean
  onCancel: () => void
  onSubmit: (body: {
    password: string
    shell: string
    groups: string[]
  }) => void
}) {
  const [password, setPassword] = useState("")
  const [shell, setShell] = useState<Option | null>({
    value: "/bin/bash",
    label: "/bin/bash",
  })
  const [groups, setGroups] = useState<Option[]>([])
  return (
    <Dialog open={open} onOpenChange={(next) => !next && onCancel()}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Provision Linux user</DialogTitle>
          <DialogDescription>
            Runs <code>useradd</code> for <strong>{username}</strong> with home
            directory and selected groups.
          </DialogDescription>
        </DialogHeader>
        <div className="grid gap-3">
          <div className="grid gap-1.5">
            <Label>Password</Label>
            <Input
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              autoComplete="new-password"
            />
          </div>
          <div className="grid gap-1.5">
            <Label>Shell</Label>
            <ReactSelect<Option, false>
              size="sm"
              options={shellOptions}
              value={shell}
              onChange={(o) => setShell(o)}
            />
          </div>
          <div className="grid gap-1.5">
            <Label>Groups</Label>
            <ReactSelect<Option, true>
              size="sm"
              isMulti
              options={groupOptions}
              value={groups}
              onChange={(o) => setGroups([...(o || [])])}
            />
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={onCancel}>
            Cancel
          </Button>
          <Button
            disabled={!password.trim() || !shell || pending}
            onClick={() =>
              shell &&
              onSubmit({
                password,
                shell: shell.value,
                groups: groups.map((g) => g.value),
              })
            }
          >
            {pending ? "Creating…" : "Create"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
