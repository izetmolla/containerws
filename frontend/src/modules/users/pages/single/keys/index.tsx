import { useMemo, useState } from "react"
import { Link, useOutletContext } from "react-router"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import {
  Check,
  Copy,
  Download,
  KeyRound,
  Plus,
  RefreshCw,
  Trash2,
  Unplug,
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
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { Separator } from "@/components/ui/separator"
import { getRequestErrorMessage } from "@/lib/network"
import { cn } from "@/lib/utils"

import type { UserSingleOutletContext } from "../types"
import {
  addAuthorizedKey,
  deleteIdentityKey,
  generateIdentityKey,
  getUserKeys,
  killSSHSession,
  listSSHSessions,
  removeAuthorizedKey,
  USER_KEYS_FETCH_KEY,
  USER_SSH_SESSIONS_FETCH_KEY,
  type IdentityKey,
  type SSHConnection,
  type SSHKeysStatus,
} from "./api"

async function copyText(text: string, label = "Copied") {
  const value = text.trim()
  if (!value) {
    toast.error("Nothing to copy")
    return false
  }
  try {
    await navigator.clipboard.writeText(value)
    toast.success(label)
    return true
  } catch {
    toast.error("Could not copy to clipboard")
    return false
  }
}

function downloadText(filename: string, content: string) {
  const blob = new Blob([content], { type: "text/plain;charset=utf-8" })
  const url = URL.createObjectURL(blob)
  const a = document.createElement("a")
  a.href = url
  a.download = filename
  a.click()
  URL.revokeObjectURL(url)
}

function formatRemoteEndpoint(session: SSHConnection) {
  const host = session.remote_host?.trim()
  if (!host) return "Local console"
  if (session.remote_port && session.remote_port > 0) {
    return `${host}:${session.remote_port}`
  }
  return host
}

function typeBadge(type?: string) {
  const t = (type || "").toLowerCase()
  if (t.includes("ed25519")) {
    return "bg-emerald-500/12 text-emerald-800 ring-emerald-500/20 dark:text-emerald-300"
  }
  if (t.includes("rsa")) {
    return "bg-sky-500/12 text-sky-800 ring-sky-500/20 dark:text-sky-300"
  }
  if (t.includes("ecdsa")) {
    return "bg-amber-500/12 text-amber-900 ring-amber-500/20 dark:text-amber-300"
  }
  return "bg-muted text-muted-foreground ring-border"
}

export default function UserKeysPage() {
  const { user, id } = useOutletContext<UserSingleOutletContext>()
  const queryClient = useQueryClient()
  const linuxExists = Boolean(user.linux?.exists)
  const username = user.username || user.linux?.username || ""

  const [addOpen, setAddOpen] = useState(false)
  const [genOpen, setGenOpen] = useState(false)
  const [removeIndex, setRemoveIndex] = useState<number | null>(null)
  const [deleteIdentityOpen, setDeleteIdentityOpen] = useState(false)
  const [killSession, setKillSession] = useState<SSHConnection | null>(null)
  const [freshPrivate, setFreshPrivate] = useState<string>("")
  const [copiedPub, setCopiedPub] = useState(false)

  const keysQuery = useQuery({
    queryKey: [USER_KEYS_FETCH_KEY, id],
    queryFn: () => getUserKeys(id),
    enabled: !!id && linuxExists,
  })

  const sessionsQuery = useQuery({
    queryKey: [USER_SSH_SESSIONS_FETCH_KEY, id],
    queryFn: () => listSSHSessions(id),
    enabled: !!id && linuxExists,
    refetchInterval: 10_000,
  })

  const status = keysQuery.data?.data
  const identity = status?.identity
  const sessions = sessionsQuery.data?.data ?? []

  const invalidate = () => {
    void queryClient.invalidateQueries({ queryKey: [USER_KEYS_FETCH_KEY, id] })
  }

  const invalidateSessions = () => {
    void queryClient.invalidateQueries({
      queryKey: [USER_SSH_SESSIONS_FETCH_KEY, id],
    })
  }

  const addMutation = useMutation({
    mutationFn: (body: { key: string; comment?: string }) =>
      addAuthorizedKey(id, body),
    onSuccess: (res) => {
      toast.success(res.message || "Key added")
      setAddOpen(false)
      invalidate()
    },
    onError: (err) =>
      toast.error(getRequestErrorMessage(err, "Failed to add key")),
  })

  const removeMutation = useMutation({
    mutationFn: (index: number) => removeAuthorizedKey(id, index),
    onSuccess: (res) => {
      toast.success(res.message || "Key removed")
      setRemoveIndex(null)
      invalidate()
    },
    onError: (err) =>
      toast.error(getRequestErrorMessage(err, "Failed to remove key")),
  })

  const genMutation = useMutation({
    mutationFn: (body: {
      type?: "ed25519" | "rsa"
      comment?: string
      passphrase?: string
      overwrite?: boolean
    }) => generateIdentityKey(id, body),
    onSuccess: (res) => {
      toast.success(res.message || "Keypair generated")
      setGenOpen(false)
      setFreshPrivate(res.data?.identity?.private_key || "")
      invalidate()
    },
    onError: (err) =>
      toast.error(getRequestErrorMessage(err, "Failed to generate keypair")),
  })

  const deleteIdentityMutation = useMutation({
    mutationFn: () => deleteIdentityKey(id),
    onSuccess: (res) => {
      toast.success(res.message || "Identity deleted")
      setDeleteIdentityOpen(false)
      setFreshPrivate("")
      invalidate()
    },
    onError: (err) =>
      toast.error(getRequestErrorMessage(err, "Failed to delete identity")),
  })

  const killSessionMutation = useMutation({
    mutationFn: (sessionId: string) => killSSHSession(id, sessionId),
    onSuccess: (res) => {
      toast.success(res.message || "Session terminated")
      setKillSession(null)
      invalidateSessions()
    },
    onError: (err) =>
      toast.error(getRequestErrorMessage(err, "Failed to terminate session")),
  })

  const privateMaterial = useMemo(() => {
    if (freshPrivate.trim()) return freshPrivate
    return ""
  }, [freshPrivate])

  if (!linuxExists) {
    return (
      <section className="flex min-h-[280px] flex-col items-center justify-center rounded-xl border border-dashed bg-muted/20 px-6 py-16 text-center">
        <div className="grid size-12 place-items-center rounded-2xl bg-muted">
          <KeyRound className="size-5 text-muted-foreground" />
        </div>
        <h2 className="mt-4 text-base font-semibold tracking-tight">SSH Keys</h2>
        <p className="mt-1 max-w-md text-sm text-muted-foreground">
          Provision a Linux account for{" "}
          <span className="font-medium text-foreground">
            {user.full_name || user.username || "this user"}
          </span>{" "}
          before managing{" "}
          <code className="rounded bg-muted px-1 py-0.5 text-xs">
            ~/.ssh/authorized_keys
          </code>{" "}
          and identity keypairs.
        </p>
        <Button className="mt-5" asChild>
          <Link to={`/users/${id}`}>Open General</Link>
        </Button>
      </section>
    )
  }

  return (
    <div className="flex w-full flex-col gap-5">
      <section className="overflow-hidden rounded-xl border bg-card shadow-sm">
        <div className="flex flex-col gap-3 border-b bg-gradient-to-br from-muted/50 via-muted/30 to-transparent p-5 md:flex-row md:items-center md:justify-between md:p-6">
          <div className="min-w-0 space-y-1">
            <div className="flex flex-wrap items-center gap-2">
              <h2 className="text-base font-semibold tracking-tight">SSH Keys</h2>
              <Badge variant="outline" className="font-normal">
                {username}
              </Badge>
            </div>
            <p className="text-sm text-muted-foreground">
              Manage login keys in{" "}
              <code className="rounded bg-muted/80 px-1 py-0.5 text-xs">
                {status?.authorized_keys_path || "~/.ssh/authorized_keys"}
              </code>{" "}
              and the user&apos;s identity keypair.
            </p>
          </div>
          <div className="flex flex-wrap gap-2">
            <Button
              type="button"
              variant="outline"
              size="sm"
              disabled={keysQuery.isFetching}
              onClick={() => void keysQuery.refetch()}
            >
              <RefreshCw
                data-icon="inline-start"
                className={cn(keysQuery.isFetching && "animate-spin")}
              />
              Refresh
            </Button>
            <Button type="button" size="sm" onClick={() => setAddOpen(true)}>
              <Plus data-icon="inline-start" />
              Add authorized key
            </Button>
          </div>
        </div>

        <div className="p-5 md:p-6">
          {keysQuery.isLoading ? (
            <p className="text-sm text-muted-foreground">Loading keys…</p>
          ) : keysQuery.isError ? (
            <p className="text-sm text-destructive">
              {getRequestErrorMessage(keysQuery.error, "Failed to load keys")}
            </p>
          ) : (
            <AuthorizedKeysList
              status={status}
              onRemove={(index) => setRemoveIndex(index)}
              onCopy={async (line) => {
                await copyText(line, "Public key copied")
              }}
            />
          )}
        </div>
      </section>

      <IdentitySection
        identity={identity}
        username={username}
        privateMaterial={privateMaterial}
        copiedPub={copiedPub}
        onCopyPublic={async () => {
          const ok = await copyText(
            identity?.public_key || "",
            "Public key copied"
          )
          if (ok) {
            setCopiedPub(true)
            window.setTimeout(() => setCopiedPub(false), 1600)
          }
        }}
        onCopyPrivate={async () => {
          if (!privateMaterial) {
            toast.message("Private key is only shown right after generation")
            return
          }
          await copyText(privateMaterial, "Private key copied")
        }}
        onDownloadPublic={() => {
          if (!identity?.public_key) return
          const name =
            identity.public_path?.split("/").pop() || `${username}.pub`
          downloadText(name, identity.public_key.endsWith("\n")
            ? identity.public_key
            : identity.public_key + "\n")
        }}
        onDownloadPrivate={() => {
          if (!privateMaterial) {
            toast.message("Private key is only available right after generation")
            return
          }
          const name =
            identity?.private_path?.split("/").pop() || `id_ed25519`
          downloadText(name, privateMaterial)
        }}
        onGenerate={() => setGenOpen(true)}
        onDelete={() => setDeleteIdentityOpen(true)}
      />

      <SessionsSection
        sessions={sessions}
        loading={sessionsQuery.isLoading}
        refreshing={sessionsQuery.isFetching}
        error={
          sessionsQuery.isError
            ? getRequestErrorMessage(
                sessionsQuery.error,
                "Failed to load sessions"
              )
            : null
        }
        onRefresh={() => void sessionsQuery.refetch()}
        onKill={(session) => setKillSession(session)}
      />

      <AddAuthorizedKeyDialog
        open={addOpen}
        onOpenChange={setAddOpen}
        pending={addMutation.isPending}
        onSubmit={(key, comment) => addMutation.mutate({ key, comment })}
      />

      <GenerateIdentityDialog
        open={genOpen}
        onOpenChange={setGenOpen}
        pending={genMutation.isPending}
        hasExisting={Boolean(identity?.exists)}
        defaultComment={`${username}@containerws`}
        onSubmit={(body) => genMutation.mutate(body)}
      />

      <AlertDialog
        open={removeIndex !== null}
        onOpenChange={(open) => {
          if (!open) setRemoveIndex(null)
        }}
      >
        <AlertDialogContent size="default">
          <AlertDialogHeader>
            <AlertDialogTitle>Remove authorized key?</AlertDialogTitle>
            <AlertDialogDescription>
              This deletes the selected line from{" "}
              <code className="text-xs">authorized_keys</code>. SSH clients using
              that key will no longer be able to log in as {username}.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction
              variant="destructive"
              disabled={removeMutation.isPending}
              onClick={() => {
                if (removeIndex !== null) removeMutation.mutate(removeIndex)
              }}
            >
              Remove
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      <AlertDialog
        open={deleteIdentityOpen}
        onOpenChange={setDeleteIdentityOpen}
      >
        <AlertDialogContent size="default">
          <AlertDialogHeader>
            <AlertDialogTitle>Delete identity keypair?</AlertDialogTitle>
            <AlertDialogDescription>
              Removes{" "}
              <code className="text-xs">id_ed25519</code> /{" "}
              <code className="text-xs">id_rsa</code> and matching{" "}
              <code className="text-xs">.pub</code> files from{" "}
              <code className="text-xs">~/.ssh</code>. Authorized keys are not
              changed.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction
              variant="destructive"
              disabled={deleteIdentityMutation.isPending}
              onClick={() => deleteIdentityMutation.mutate()}
            >
              Delete keypair
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      <AlertDialog
        open={killSession !== null}
        onOpenChange={(open) => {
          if (!open) setKillSession(null)
        }}
      >
        <AlertDialogContent size="default">
          <AlertDialogHeader>
            <AlertDialogTitle>Terminate SSH session?</AlertDialogTitle>
            <AlertDialogDescription>
              This disconnects{" "}
              <span className="font-medium text-foreground">
                {killSession
                  ? formatRemoteEndpoint(killSession)
                  : "the remote client"}
              </span>
              {killSession?.tty ? (
                <>
                  {" "}
                  on <code className="text-xs">{killSession.tty}</code>
                </>
              ) : null}
              {killSession?.pid ? (
                <>
                  {" "}
                  (pid {killSession.pid})
                </>
              ) : null}
              . Unsaved work in that session may be lost.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction
              variant="destructive"
              disabled={killSessionMutation.isPending || !killSession}
              onClick={() => {
                if (killSession) killSessionMutation.mutate(killSession.id)
              }}
            >
              Terminate session
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}

function AuthorizedKeysList({
  status,
  onRemove,
  onCopy,
}: {
  status?: SSHKeysStatus
  onRemove: (index: number) => void
  onCopy: (line: string) => void | Promise<void>
}) {
  const keys = status?.authorized_keys ?? []
  if (keys.length === 0) {
    return (
      <div className="rounded-lg border border-dashed bg-muted/15 px-4 py-10 text-center">
        <p className="text-sm font-medium">No authorized keys yet</p>
        <p className="mt-1 text-sm text-muted-foreground">
          Paste a public key (one at a time) to allow SSH login for this user.
        </p>
      </div>
    )
  }

  return (
    <ul className="flex flex-col gap-3">
      {keys.map((key) => (
        <li
          key={`${key.index}-${key.fingerprint || key.line}`}
          className="rounded-lg border bg-background/60 p-4"
        >
          <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
            <div className="min-w-0 space-y-1.5">
              <div className="flex flex-wrap items-center gap-2">
                <span
                  className={cn(
                    "inline-flex items-center rounded-full px-2 py-0.5 text-[11px] font-medium uppercase tracking-wide ring-1 ring-inset",
                    typeBadge(key.type)
                  )}
                >
                  {key.type || "key"}
                </span>
                <span className="truncate text-sm font-medium">
                  {key.comment || "Unnamed key"}
                </span>
              </div>
              {key.fingerprint ? (
                <p className="font-mono text-xs text-muted-foreground break-all">
                  {key.fingerprint}
                </p>
              ) : null}
            </div>
            <div className="flex shrink-0 gap-1.5">
              <Button
                type="button"
                size="sm"
                variant="outline"
                onClick={() => void onCopy(key.line)}
              >
                <Copy data-icon="inline-start" />
                Copy
              </Button>
              <Button
                type="button"
                size="sm"
                variant="ghost"
                className="text-destructive hover:bg-destructive/10 hover:text-destructive"
                onClick={() => onRemove(key.index)}
              >
                <Trash2 data-icon="inline-start" />
                Remove
              </Button>
            </div>
          </div>
        </li>
      ))}
    </ul>
  )
}

function IdentitySection({
  identity,
  username,
  privateMaterial,
  copiedPub,
  onCopyPublic,
  onCopyPrivate,
  onDownloadPublic,
  onDownloadPrivate,
  onGenerate,
  onDelete,
}: {
  identity?: IdentityKey
  username: string
  privateMaterial: string
  copiedPub: boolean
  onCopyPublic: () => void | Promise<void>
  onCopyPrivate: () => void | Promise<void>
  onDownloadPublic: () => void
  onDownloadPrivate: () => void
  onGenerate: () => void
  onDelete: () => void
}) {
  const exists = Boolean(identity?.exists)

  return (
    <section className="overflow-hidden rounded-xl border bg-card shadow-sm">
      <div className="flex flex-col gap-3 border-b bg-gradient-to-br from-muted/40 via-muted/20 to-transparent p-5 md:flex-row md:items-center md:justify-between md:p-6">
        <div className="min-w-0 space-y-1">
          <h2 className="text-base font-semibold tracking-tight">
            Host identity
          </h2>
          <p className="max-w-xl text-sm text-muted-foreground">
            SSH client credentials for this account. Generate a keypair, then
            copy the public key wherever remote access is required.
          </p>
        </div>
        <div className="flex shrink-0 flex-wrap gap-2">
          <Button type="button" size="sm" onClick={onGenerate}>
            <KeyRound data-icon="inline-start" />
            {exists ? "Regenerate" : "Generate keypair"}
          </Button>
          {exists ? (
            <Button
              type="button"
              size="sm"
              variant="outline"
              className="text-destructive hover:bg-destructive/10 hover:text-destructive"
              onClick={onDelete}
            >
              <Trash2 data-icon="inline-start" />
              Delete
            </Button>
          ) : null}
        </div>
      </div>

      <div className="p-5 md:p-6">
        {!exists ? (
          <div className="rounded-lg border border-dashed bg-muted/15 px-4 py-10 text-center">
            <p className="text-sm font-medium">No host identity</p>
            <p className="mt-1 text-sm text-muted-foreground">
              Generate an Ed25519 keypair (recommended) for {username}.
            </p>
          </div>
        ) : (
          <div className="space-y-4">
            <div className="flex flex-wrap items-center gap-2">
              <span
                className={cn(
                  "inline-flex items-center rounded-full px-2 py-0.5 text-[11px] font-medium uppercase tracking-wide ring-1 ring-inset",
                  typeBadge(identity?.type)
                )}
              >
                {identity?.type || "key"}
              </span>
              {identity?.comment ? (
                <span className="text-sm font-medium">{identity.comment}</span>
              ) : null}
              {identity?.fingerprint ? (
                <span className="font-mono text-xs text-muted-foreground">
                  {identity.fingerprint}
                </span>
              ) : null}
            </div>

            <div className="space-y-2">
              <div className="flex flex-wrap items-center justify-between gap-2">
                <Label>Public key</Label>
                <div className="flex gap-1.5">
                  <Button
                    type="button"
                    size="sm"
                    variant="outline"
                    onClick={() => void onCopyPublic()}
                  >
                    {copiedPub ? (
                      <Check data-icon="inline-start" />
                    ) : (
                      <Copy data-icon="inline-start" />
                    )}
                    {copiedPub ? "Copied" : "Copy public key"}
                  </Button>
                  <Button
                    type="button"
                    size="sm"
                    variant="ghost"
                    onClick={onDownloadPublic}
                  >
                    <Download data-icon="inline-start" />
                    Download
                  </Button>
                </div>
              </div>
              <pre className="max-h-36 overflow-auto rounded-lg border bg-muted/30 p-3 font-mono text-[11px] leading-relaxed break-all whitespace-pre-wrap">
                {identity?.public_key || "—"}
              </pre>
            </div>

            {privateMaterial ? (
              <>
                <Separator />
                <div className="space-y-2">
                  <div className="flex flex-wrap items-center justify-between gap-2">
                    <div>
                      <Label>Private key</Label>
                      <p className="text-xs text-amber-700 dark:text-amber-300">
                        Shown once after generation. Store it securely — it is
                        not kept in the browser.
                      </p>
                    </div>
                    <div className="flex gap-1.5">
                      <Button
                        type="button"
                        size="sm"
                        variant="outline"
                        onClick={() => void onCopyPrivate()}
                      >
                        <Copy data-icon="inline-start" />
                        Copy private key
                      </Button>
                      <Button
                        type="button"
                        size="sm"
                        variant="ghost"
                        onClick={onDownloadPrivate}
                      >
                        <Download data-icon="inline-start" />
                        Download
                      </Button>
                    </div>
                  </div>
                  <pre className="max-h-48 overflow-auto rounded-lg border border-amber-500/30 bg-amber-500/5 p-3 font-mono text-[11px] leading-relaxed whitespace-pre-wrap">
                    {privateMaterial}
                  </pre>
                </div>
              </>
            ) : null}

            <p className="text-xs text-muted-foreground">
              Files:{" "}
              <code className="rounded bg-muted px-1 py-0.5">
                {identity?.private_path}
              </code>
              {identity?.has_public ? (
                <>
                  {" "}
                  ·{" "}
                  <code className="rounded bg-muted px-1 py-0.5">
                    {identity?.public_path}
                  </code>
                </>
              ) : null}
            </p>
          </div>
        )}
      </div>
    </section>
  )
}

function SessionsSection({
  sessions,
  loading,
  refreshing,
  error,
  onRefresh,
  onKill,
}: {
  sessions: SSHConnection[]
  loading: boolean
  refreshing: boolean
  error: string | null
  onRefresh: () => void
  onKill: (session: SSHConnection) => void
}) {
  return (
    <section className="overflow-hidden rounded-xl border bg-card shadow-sm">
      <div className="flex flex-col gap-3 border-b bg-gradient-to-br from-muted/40 via-muted/20 to-transparent p-5 md:flex-row md:items-center md:justify-between md:p-6">
        <div className="min-w-0 space-y-1">
          <div className="flex flex-wrap items-center gap-2">
            <h2 className="text-base font-semibold tracking-tight">
              Active sessions
            </h2>
            <Badge variant="outline" className="font-normal tabular-nums">
              {sessions.length}
            </Badge>
          </div>
          <p className="max-w-xl text-sm text-muted-foreground">
            Live SSH and terminal logins for this account. Terminate a session
            to disconnect the remote client immediately.
          </p>
        </div>
        <Button
          type="button"
          size="sm"
          variant="outline"
          disabled={refreshing}
          onClick={onRefresh}
        >
          <RefreshCw
            data-icon="inline-start"
            className={cn(refreshing && "animate-spin")}
          />
          Refresh
        </Button>
      </div>

      <div className="p-5 md:p-6">
        {loading ? (
          <p className="text-sm text-muted-foreground">Loading sessions…</p>
        ) : error ? (
          <p className="text-sm text-destructive">{error}</p>
        ) : sessions.length === 0 ? (
          <div className="rounded-lg border border-dashed bg-muted/15 px-4 py-10 text-center">
            <p className="text-sm font-medium">No active sessions</p>
            <p className="mt-1 text-sm text-muted-foreground">
              Connected SSH clients for this user will appear here.
            </p>
          </div>
        ) : (
          <ul className="flex flex-col gap-3">
            {sessions.map((session) => (
              <li
                key={session.id}
                className="rounded-lg border bg-background/60 p-4"
              >
                <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
                  <div className="min-w-0 space-y-2">
                    <div className="flex flex-wrap items-center gap-2">
                      <span
                        className={cn(
                          "inline-flex items-center rounded-full px-2 py-0.5 text-[11px] font-medium capitalize ring-1 ring-inset",
                          session.kind === "interactive" ||
                            (session.via_ssh && session.tty)
                            ? "bg-emerald-500/12 text-emerald-800 ring-emerald-500/20 dark:text-emerald-300"
                            : session.via_ssh
                              ? "bg-sky-500/12 text-sky-800 ring-sky-500/20 dark:text-sky-300"
                              : "bg-muted text-muted-foreground ring-border"
                        )}
                      >
                        {session.kind || (session.via_ssh ? "ssh" : "tty")}
                      </span>
                      <span className="truncate font-mono text-sm font-semibold tracking-tight">
                        {formatRemoteEndpoint(session)}
                      </span>
                      {session.tty ? (
                        <code className="rounded bg-muted px-1.5 py-0.5 text-[11px] text-muted-foreground">
                          {session.tty}
                        </code>
                      ) : session.via_ssh ? (
                        <code className="rounded bg-muted px-1.5 py-0.5 text-[11px] text-muted-foreground">
                          notty
                        </code>
                      ) : null}
                    </div>
                    <dl className="grid gap-1 text-xs text-muted-foreground sm:grid-cols-2">
                      {session.local_addr ? (
                        <div>
                          <span className="text-muted-foreground/80">Server · </span>
                          <span className="font-mono text-foreground/80">
                            {session.local_addr}
                            {session.local_port ? `:${session.local_port}` : ""}
                          </span>
                        </div>
                      ) : null}
                      {session.remote_port ? (
                        <div>
                          <span className="text-muted-foreground/80">
                            Client port ·{" "}
                          </span>
                          <span className="font-mono text-foreground/80">
                            {session.remote_port}
                          </span>
                        </div>
                      ) : null}
                      {session.pid > 0 ? (
                        <div>
                          <span className="text-muted-foreground/80">
                            Session PID ·{" "}
                          </span>
                          <span className="font-mono text-foreground/80">
                            {session.pid}
                          </span>
                        </div>
                      ) : null}
                      {session.shell_command ? (
                        <div className="min-w-0 sm:col-span-2">
                          <span className="text-muted-foreground/80">
                            Shell ·{" "}
                          </span>
                          <span className="font-mono text-foreground/80 break-all">
                            {session.shell_command}
                            {session.shell_pid
                              ? ` (pid ${session.shell_pid})`
                              : ""}
                          </span>
                        </div>
                      ) : null}
                      {session.started_at ? (
                        <div className="sm:col-span-2">
                          <span className="text-muted-foreground/80">
                            Since ·{" "}
                          </span>
                          <span className="text-foreground/80">
                            {session.started_at}
                          </span>
                          {session.idle && session.idle !== "." ? (
                            <span className="text-foreground/80">
                              {" "}
                              · idle {session.idle}
                            </span>
                          ) : null}
                        </div>
                      ) : null}
                    </dl>
                  </div>
                  <Button
                    type="button"
                    size="sm"
                    variant="outline"
                    className="text-destructive hover:bg-destructive/10 hover:text-destructive"
                    onClick={() => onKill(session)}
                  >
                    <Unplug data-icon="inline-start" />
                    Disconnect
                  </Button>
                </div>
              </li>
            ))}
          </ul>
        )}
      </div>
    </section>
  )
}

function AddAuthorizedKeyDialog({
  open,
  onOpenChange,
  pending,
  onSubmit,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  pending: boolean
  onSubmit: (key: string, comment?: string) => void
}) {
  const [key, setKey] = useState("")
  const [comment, setComment] = useState("")

  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        onOpenChange(next)
        if (!next) {
          setKey("")
          setComment("")
        }
      }}
    >
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>Add authorized key</DialogTitle>
          <DialogDescription>
            Paste one OpenSSH public key (e.g.{" "}
            <code className="text-xs">ssh-ed25519 AAAA… comment</code>). Keys are
            appended to{" "}
            <code className="text-xs">~/.ssh/authorized_keys</code> one at a
            time.
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-3">
          <div className="space-y-1.5">
            <Label htmlFor="auth-key">Public key</Label>
            <textarea
              id="auth-key"
              value={key}
              onChange={(e) => setKey(e.target.value)}
              rows={5}
              placeholder="ssh-ed25519 AAAA... user@host"
              className="flex w-full rounded-lg border border-input bg-transparent px-2.5 py-2 font-mono text-xs outline-none transition-colors placeholder:text-muted-foreground focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50 dark:bg-input/30"
            />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="auth-comment">Comment override (optional)</Label>
            <Input
              id="auth-comment"
              value={comment}
              onChange={(e) => setComment(e.target.value)}
              placeholder="laptop-2026"
            />
          </div>
        </div>
        <DialogFooter>
          <Button
            type="button"
            variant="outline"
            onClick={() => onOpenChange(false)}
          >
            Cancel
          </Button>
          <Button
            type="button"
            disabled={pending || !key.trim()}
            onClick={() => onSubmit(key.trim(), comment.trim() || undefined)}
          >
            Add key
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

function GenerateIdentityDialog({
  open,
  onOpenChange,
  pending,
  hasExisting,
  defaultComment,
  onSubmit,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  pending: boolean
  hasExisting: boolean
  defaultComment: string
  onSubmit: (body: {
    type?: "ed25519" | "rsa"
    comment?: string
    passphrase?: string
    overwrite?: boolean
  }) => void
}) {
  const [type, setType] = useState<"ed25519" | "rsa">("ed25519")
  const [comment, setComment] = useState(defaultComment)
  const [passphrase, setPassphrase] = useState("")

  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        onOpenChange(next)
        if (next) {
          setComment(defaultComment)
          setType("ed25519")
          setPassphrase("")
        }
      }}
    >
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>
            {hasExisting ? "Regenerate identity keypair" : "Generate identity keypair"}
          </DialogTitle>
          <DialogDescription>
            Creates an OpenSSH keypair under{" "}
            <code className="text-xs">~/.ssh</code>. Ed25519 is recommended.
            {hasExisting
              ? " Existing id_ed25519 / id_rsa files will be overwritten."
              : null}
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-3">
          <div className="space-y-1.5">
            <Label>Key type</Label>
            <Select
              value={type}
              onValueChange={(v) => setType(v as "ed25519" | "rsa")}
            >
              <SelectTrigger className="w-full">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="ed25519">Ed25519 (recommended)</SelectItem>
                <SelectItem value="rsa">RSA 4096</SelectItem>
              </SelectContent>
            </Select>
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="id-comment">Comment</Label>
            <Input
              id="id-comment"
              value={comment}
              onChange={(e) => setComment(e.target.value)}
              placeholder="user@host"
            />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="id-pass">Passphrase (optional)</Label>
            <Input
              id="id-pass"
              type="password"
              autoComplete="new-password"
              value={passphrase}
              onChange={(e) => setPassphrase(e.target.value)}
              placeholder="Leave empty for no passphrase"
            />
          </div>
        </div>
        <DialogFooter>
          <Button
            type="button"
            variant="outline"
            onClick={() => onOpenChange(false)}
          >
            Cancel
          </Button>
          <Button
            type="button"
            disabled={pending}
            onClick={() =>
              onSubmit({
                type,
                comment: comment.trim() || undefined,
                passphrase: passphrase || undefined,
                overwrite: hasExisting,
              })
            }
          >
            {hasExisting ? "Overwrite & generate" : "Generate"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
