import { useState } from "react"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import {
  CheckCircle2,
  Copy,
  KeyRound,
  Loader2,
  Plus,
  Save,
  Trash2,
} from "lucide-react"
import { toast } from "sonner"

import ContentLoader from "@/components/content-loader"
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
import { ReactSelect } from "@/components/ui/reactselect"
import { Separator } from "@/components/ui/separator"
import { getRequestErrorMessage, withError } from "@/lib/network"
import { cn } from "@/lib/utils"

import {
  createMcpKey,
  deleteMcpKey,
  getMcpKey,
  getMcpStandalone,
  listMcpKeys,
  revokeMcpKey,
  SETTINGS_MCP_FETCH_KEY,
  updateMcpStandalone,
  type McpKey,
} from "./api"

type AddressOption = { value: string; label: string }

function formatWhen(value: string | null | undefined) {
  if (!value) return "—"
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return new Intl.DateTimeFormat(undefined, {
    dateStyle: "medium",
    timeStyle: "short",
  }).format(date)
}

function tokenHint(key: Pick<McpKey, "key_prefix" | "key_suffix" | "key">) {
  if (key.key) return key.key
  const prefix = key.key_prefix || ""
  const suffix = key.key_suffix || ""
  if (prefix && suffix) return `${prefix}…${suffix}`
  if (prefix) return `${prefix}…`
  return "—"
}

async function copyText(text: string, label: string) {
  try {
    await navigator.clipboard.writeText(text)
    toast.success(`${label} copied`)
  } catch {
    toast.error("Could not copy to clipboard")
  }
}

export default function McpSettingsPage() {
  const queryClient = useQueryClient()

  const [enabled, setEnabled] = useState(false)
  const [address, setAddress] = useState("0.0.0.0")
  const [port, setPort] = useState("8090")

  const [createOpen, setCreateOpen] = useState(false)
  const [keyName, setKeyName] = useState("")
  const [keyDescription, setKeyDescription] = useState("")
  const [expiresDays, setExpiresDays] = useState("0")
  const [createdSecret, setCreatedSecret] = useState<string | null>(null)

  const [deleteTarget, setDeleteTarget] = useState<McpKey | null>(null)
  const [copyingId, setCopyingId] = useState<string | null>(null)

  const standaloneQuery = useQuery({
    queryKey: [SETTINGS_MCP_FETCH_KEY, "standalone"],
    queryFn: getMcpStandalone,
    refetchInterval: 15_000,
  })

  const keysQuery = useQuery({
    queryKey: [SETTINGS_MCP_FETCH_KEY, "keys"],
    queryFn: listMcpKeys,
  })

  const standaloneData = standaloneQuery.data?.data
  const [prevStandaloneData, setPrevStandaloneData] = useState(standaloneData)
  if (standaloneData !== prevStandaloneData) {
    setPrevStandaloneData(standaloneData)
    if (standaloneData) {
      setEnabled(Boolean(standaloneData.enabled))
      setAddress(standaloneData.address || "0.0.0.0")
      setPort(String(standaloneData.port > 0 ? standaloneData.port : 8090))
    }
  }

  const saveStandalone = useMutation({
    mutationFn: () => {
      const portNum = Number.parseInt(port, 10)
      if (!Number.isFinite(portNum) || portNum < 1 || portNum > 65535) {
        return Promise.reject(new Error("Port must be between 1 and 65535"))
      }
      return updateMcpStandalone({
        enabled,
        address: address.trim() || "0.0.0.0",
        port: portNum,
      })
    },
    onSuccess: (res) => {
      toast.success(res.message || "Standalone MCP updated")
      void queryClient.invalidateQueries({
        queryKey: [SETTINGS_MCP_FETCH_KEY, "standalone"],
      })
    },
    onError: (err) => {
      toast.error(
        getRequestErrorMessage(err, "Failed to update standalone MCP")
      )
    },
  })

  const createKey = useMutation({
    mutationFn: () => {
      const days = Number.parseInt(expiresDays, 10)
      return createMcpKey({
        name: keyName.trim() || "MCP key",
        description: keyDescription.trim(),
        expires_in_days: Number.isFinite(days) && days > 0 ? days : 0,
      })
    },
    onSuccess: (res) => {
      toast.success(res.message || "MCP key created")
      setCreatedSecret(res.data?.key || null)
      setKeyName("")
      setKeyDescription("")
      setExpiresDays("0")
      void queryClient.invalidateQueries({
        queryKey: [SETTINGS_MCP_FETCH_KEY, "keys"],
      })
    },
    onError: (err) => {
      toast.error(getRequestErrorMessage(err, "Failed to create key"))
    },
  })

  const revokeKey = useMutation({
    mutationFn: (id: string) => revokeMcpKey(id),
    onSuccess: (res) => {
      toast.success(res.message || "Key revoked")
      void queryClient.invalidateQueries({
        queryKey: [SETTINGS_MCP_FETCH_KEY, "keys"],
      })
    },
    onError: (err) => {
      toast.error(getRequestErrorMessage(err, "Failed to revoke key"))
    },
  })

  const removeKey = useMutation({
    mutationFn: (id: string) => deleteMcpKey(id),
    onSuccess: (res) => {
      toast.success(res.message || "Key deleted")
      setDeleteTarget(null)
      void queryClient.invalidateQueries({
        queryKey: [SETTINGS_MCP_FETCH_KEY, "keys"],
      })
    },
    onError: (err) => {
      toast.error(getRequestErrorMessage(err, "Failed to delete key"))
    },
  })

  const status = standaloneQuery.data?.data
  const keys = keysQuery.data?.data || []
  const addressOptions: AddressOption[] = (() => {
    // Host IPs come from standalone status (bind_addresses) — no extra request.
    const rows = status?.bind_addresses ?? []
    const opts = rows.map((opt) => ({
      value: opt.address,
      label: opt.label,
    }))
    if (address && !opts.some((o) => o.value === address)) {
      opts.push({ value: address, label: `${address} · saved` })
    }
    if (opts.length === 0) {
      opts.push(
        { value: "0.0.0.0", label: "0.0.0.0 · all interfaces" },
        { value: "127.0.0.1", label: "127.0.0.1 · localhost" }
      )
    } else if (!opts.some((o) => o.value === "0.0.0.0")) {
      opts.unshift({
        value: "0.0.0.0",
        label: "0.0.0.0 · all interfaces",
      })
    }
    return opts
  })()
  const loading = standaloneQuery.isLoading || keysQuery.isLoading
  const pageError =
    withError(standaloneQuery.error, standaloneQuery.data) ||
    withError(keysQuery.error, keysQuery.data)

  return (
    <ContentLoader
      title="MCP"
      description="API keys and standalone MCP listener for external clients."
      breadcrumb={[
        { label: "Settings", to: "/settings" },
        { label: "MCP" },
      ]}
      isLoading={loading}
      error={pageError}
      showHeaderSeparator
    >
      <div className="w-full space-y-10">
        <section className="space-y-4">
          <div className="flex flex-wrap items-start justify-between gap-3">
            <div>
              <h2 className="text-sm font-medium">Standalone listener</h2>
              <p className="mt-1 text-sm text-muted-foreground">
                Bind a dedicated MCP HTTP endpoint on a host interface and
                port. MCP is also available on the main API
                {status?.main_api_mcp ? ` (${status.main_api_mcp})` : ""}.
              </p>
            </div>
            <div className="flex flex-wrap items-center gap-2">
              {status?.running ? (
                <Badge variant="default">Running</Badge>
              ) : status?.enabled ? (
                <Badge variant="secondary">Enabled</Badge>
              ) : (
                <Badge variant="outline">Off</Badge>
              )}
              {status?.source ? (
                <Badge variant="outline">{status.source}</Badge>
              ) : null}
            </div>
          </div>
          <Separator />

          <label className="flex cursor-pointer items-start gap-3 rounded-lg border border-border/70 bg-muted/20 px-3 py-3">
            <input
              type="checkbox"
              className="mt-1 size-4 accent-primary"
              checked={enabled}
              onChange={(e) => setEnabled(e.target.checked)}
            />
            <span>
              <span className="block text-sm font-medium">
                Enable standalone MCP
              </span>
              <span className="mt-0.5 block text-sm text-muted-foreground">
                Start a separate Fiber listener for MCP clients (Cursor, CLI,
                etc.).
              </span>
            </span>
          </label>

          <div className="grid gap-4 sm:grid-cols-2">
            <div className="space-y-2">
              <Label htmlFor="mcp-address">Interface IP</Label>
              <ReactSelect<AddressOption, false>
                inputId="mcp-address"
                options={addressOptions}
                value={address || "0.0.0.0"}
                onValueChange={(v) => setAddress(v || "0.0.0.0")}
                placeholder="Search host IPs…"
                isSearchable
                isDisabled={!enabled}
                isLoading={standaloneQuery.isFetching}
                wrapOptionText
                className="w-full"
              />
              <p className="text-xs text-muted-foreground">
                Live addresses on this host. Search by IP or interface name.
              </p>
            </div>
            <div className="space-y-2">
              <Label htmlFor="mcp-port">Port</Label>
              <Input
                id="mcp-port"
                type="number"
                min={1}
                max={65535}
                value={port}
                disabled={!enabled}
                onChange={(e) => setPort(e.target.value)}
              />
            </div>
          </div>

          {status?.public_url ? (
            <div className="flex flex-wrap items-center gap-2 rounded-lg border border-border/60 bg-muted/15 px-3 py-2 text-sm">
              <span className="text-muted-foreground">Public URL</span>
              <code className="truncate font-mono text-xs">
                {status.public_url}
              </code>
              <Button
                type="button"
                size="sm"
                variant="ghost"
                onClick={() => void copyText(status.public_url, "URL")}
              >
                <Copy className="size-3.5" />
                Copy
              </Button>
            </div>
          ) : null}

          {status?.last_error ? (
            <p className="text-sm text-destructive">{status.last_error}</p>
          ) : null}

          <div className="flex justify-end">
            <Button
              type="button"
              disabled={saveStandalone.isPending}
              onClick={() => saveStandalone.mutate()}
            >
              {saveStandalone.isPending ? (
                <Loader2 className="size-4 animate-spin" />
              ) : (
                <Save className="size-4" />
              )}
              Save listener
            </Button>
          </div>
        </section>

        <section className="space-y-4">
          <div className="flex flex-wrap items-start justify-between gap-3">
            <div>
              <h2 className="text-sm font-medium">Access tokens</h2>
              <p className="mt-1 text-sm text-muted-foreground">
                Generate keys for Bearer / X-Api-Key / ?token= auth. The full
                secret is shown only once at creation.
              </p>
            </div>
            <Button
              type="button"
              size="sm"
              onClick={() => {
                setCreatedSecret(null)
                setCreateOpen(true)
              }}
            >
              <Plus className="size-4" />
              Generate key
            </Button>
          </div>
          <Separator />

          {keys.length === 0 ? (
            <div className="flex flex-col items-center gap-2 rounded-lg border border-dashed border-border/80 px-4 py-10 text-center">
              <KeyRound className="size-8 text-muted-foreground/70" />
              <p className="text-sm text-muted-foreground">
                No MCP keys yet. Generate one to connect external clients.
              </p>
            </div>
          ) : (
            <ul className="divide-y divide-border/70 rounded-lg border border-border/70">
              {keys.map((key) => {
                const active = key.status === "active"
                const hint = tokenHint(key)
                return (
                  <li
                    key={key.id}
                    className="flex flex-col gap-3 px-3 py-3 sm:flex-row sm:items-center sm:justify-between"
                  >
                    <div className="min-w-0 space-y-1">
                      <div className="flex flex-wrap items-center gap-2">
                        <span className="truncate text-sm font-medium">
                          {key.name}
                        </span>
                        <Badge variant={active ? "default" : "secondary"}>
                          {key.status}
                        </Badge>
                      </div>
                      <div className="flex min-w-0 flex-wrap items-center gap-1.5">
                        <code
                          className="max-w-full truncate rounded bg-muted px-1.5 py-0.5 font-mono text-xs"
                          title={hint}
                        >
                          {hint}
                        </code>
                        <Button
                          type="button"
                          size="sm"
                          variant="ghost"
                          className="h-7 px-2"
                          title="Copy full MCP key"
                          disabled={copyingId === key.id}
                          onClick={() => {
                            void (async () => {
                              setCopyingId(key.id)
                              try {
                                if (key.key) {
                                  await copyText(key.key, "MCP key")
                                  return
                                }
                                const res = await getMcpKey(key.id)
                                const full = res.data?.key?.trim()
                                if (!full) {
                                  toast.error("Full key unavailable")
                                  return
                                }
                                await copyText(full, "MCP key")
                              } catch (err) {
                                toast.error(
                                  getRequestErrorMessage(
                                    err,
                                    "Could not load MCP key"
                                  )
                                )
                              } finally {
                                setCopyingId(null)
                              }
                            })()
                          }}
                        >
                          {copyingId === key.id ? (
                            <Loader2 className="size-3.5 animate-spin" />
                          ) : (
                            <Copy className="size-3.5" />
                          )}
                          <span className="sr-only sm:not-sr-only sm:ml-1">
                            Copy
                          </span>
                        </Button>
                      </div>
                      {key.description ? (
                        <p className="text-sm text-muted-foreground">
                          {key.description}
                        </p>
                      ) : null}
                      <p className="text-xs text-muted-foreground">
                        Created {formatWhen(key.created_at)}
                        {key.expires_at
                          ? ` · Expires ${formatWhen(key.expires_at)}`
                          : " · No expiry"}
                        {key.last_used_at
                          ? ` · Last used ${formatWhen(key.last_used_at)}`
                          : " · Never used"}
                        {key.last_used_ip
                          ? ` · ${key.last_used_ip}`
                          : ""}
                      </p>
                    </div>
                    <div className="flex shrink-0 flex-wrap gap-1.5">
                      {active ? (
                        <Button
                          type="button"
                          size="sm"
                          variant="outline"
                          disabled={revokeKey.isPending}
                          onClick={() => revokeKey.mutate(key.id)}
                        >
                          Revoke
                        </Button>
                      ) : null}
                      <Button
                        type="button"
                        size="sm"
                        variant="ghost"
                        className="text-destructive hover:text-destructive"
                        onClick={() => setDeleteTarget(key)}
                      >
                        <Trash2 className="size-3.5" />
                        Delete
                      </Button>
                    </div>
                  </li>
                )
              })}
            </ul>
          )}
        </section>
      </div>

      <Dialog
        open={createOpen}
        onOpenChange={(open) => {
          setCreateOpen(open)
          if (!open) setCreatedSecret(null)
        }}
      >
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>
              {createdSecret ? "Copy your MCP key" : "Generate MCP key"}
            </DialogTitle>
            <DialogDescription>
              {createdSecret
                ? "Store this secret securely. You can copy it again later from the keys list."
                : "Create a token for MCP clients. Name it so you can revoke it later."}
            </DialogDescription>
          </DialogHeader>

          {createdSecret ? (
            <div className="space-y-3">
              <div className="flex items-start gap-2 rounded-lg border border-emerald-500/30 bg-emerald-500/10 px-3 py-2 text-sm">
                <CheckCircle2 className="mt-0.5 size-4 shrink-0 text-emerald-600" />
                <span>Key created. Copy it before closing this dialog.</span>
              </div>
              <div className="flex gap-2">
                <Input
                  readOnly
                  value={createdSecret}
                  className="font-mono text-xs"
                />
                <Button
                  type="button"
                  variant="outline"
                  onClick={() => void copyText(createdSecret, "Key")}
                >
                  <Copy className="size-4" />
                </Button>
              </div>
              <DialogFooter>
                <Button type="button" onClick={() => setCreateOpen(false)}>
                  Done
                </Button>
              </DialogFooter>
            </div>
          ) : (
            <div className="space-y-4">
              <div className="space-y-2">
                <Label htmlFor="mcp-key-name">Name</Label>
                <Input
                  id="mcp-key-name"
                  value={keyName}
                  placeholder="Cursor desktop"
                  onChange={(e) => setKeyName(e.target.value)}
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="mcp-key-desc">Description</Label>
                <textarea
                  id="mcp-key-desc"
                  value={keyDescription}
                  rows={2}
                  placeholder="Optional"
                  onChange={(e) => setKeyDescription(e.target.value)}
                  className={cn(
                    "w-full min-w-0 rounded-lg border border-input bg-transparent px-2.5 py-2 text-sm outline-none",
                    "placeholder:text-muted-foreground focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50",
                    "dark:bg-input/30"
                  )}
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="mcp-key-expires">Expires in (days)</Label>
                <Input
                  id="mcp-key-expires"
                  type="number"
                  min={0}
                  value={expiresDays}
                  onChange={(e) => setExpiresDays(e.target.value)}
                />
                <p className="text-xs text-muted-foreground">
                  Use 0 for no expiry.
                </p>
              </div>
              <DialogFooter>
                <Button
                  type="button"
                  variant="outline"
                  onClick={() => setCreateOpen(false)}
                >
                  Cancel
                </Button>
                <Button
                  type="button"
                  disabled={createKey.isPending}
                  onClick={() => createKey.mutate()}
                >
                  {createKey.isPending ? (
                    <Loader2 className="size-4 animate-spin" />
                  ) : (
                    <KeyRound className="size-4" />
                  )}
                  Generate
                </Button>
              </DialogFooter>
            </div>
          )}
        </DialogContent>
      </Dialog>

      <Dialog
        open={Boolean(deleteTarget)}
        onOpenChange={(open) => {
          if (!open) setDeleteTarget(null)
        }}
      >
        <DialogContent className="sm:max-w-sm">
          <DialogHeader>
            <DialogTitle>Delete MCP key</DialogTitle>
            <DialogDescription>
              Permanently remove{" "}
              <span className="font-medium text-foreground">
                {deleteTarget?.name}
              </span>
              ? Connected clients will stop working immediately.
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              onClick={() => setDeleteTarget(null)}
            >
              Cancel
            </Button>
            <Button
              type="button"
              variant="destructive"
              disabled={removeKey.isPending || !deleteTarget}
              onClick={() => {
                if (deleteTarget) removeKey.mutate(deleteTarget.id)
              }}
            >
              {removeKey.isPending ? (
                <Loader2 className="size-4 animate-spin" />
              ) : (
                <Trash2 className="size-4" />
              )}
              Delete
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </ContentLoader>
  )
}
