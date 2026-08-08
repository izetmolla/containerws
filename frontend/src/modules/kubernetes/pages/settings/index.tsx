import { useEffect, useEffectEvent, useMemo, useRef, useState } from "react"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import {
  CheckCircle2,
  MoreHorizontal,
  Plus,
  RefreshCw,
  Trash2,
  XCircle,
  Zap,
} from "lucide-react"
import { toast } from "sonner"

import ContentLoader from "@/components/content-loader"
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
import { asArray } from "@/lib/as-array"
import { toastRequestError } from "@/lib/network"
import { cn } from "@/lib/utils"

import { KubeconfigYamlEditor } from "./kubeconfig-yaml-editor"

import {
  activateKubeConfigFile,
  createKubeConfigFile,
  deleteKubeConfigFile,
  getKubeConfig,
  getKubeConfigFile,
  K8S_CLUSTER_KEY,
  K8S_CONFIG_KEY,
  testKubeConfig,
  updateKubeConfig,
  updateKubeConfigFile,
  type KubeConfigFile,
  type KubeContext,
  type KubeSecretMapEntry,
} from "../_shared/api"
import { SystemResourcesToggle } from "../_shared/system-toggle"

const SIDEBAR_WIDTH_KEY = "k8s.settings.secretsSidebarWidth"
const SIDEBAR_MIN = 200
const SIDEBAR_MAX = 480
const SIDEBAR_DEFAULT = 240

function usePersistedSidebarWidth() {
  const [width, setWidth] = useState(() => {
    try {
      const raw = localStorage.getItem(SIDEBAR_WIDTH_KEY)
      const n = raw ? Number(raw) : SIDEBAR_DEFAULT
      if (!Number.isFinite(n)) return SIDEBAR_DEFAULT
      return Math.min(SIDEBAR_MAX, Math.max(SIDEBAR_MIN, n))
    } catch {
      return SIDEBAR_DEFAULT
    }
  })

  const persist = useEffectEvent((next: number) => {
    try {
      localStorage.setItem(SIDEBAR_WIDTH_KEY, String(next))
    } catch {
      /* ignore */
    }
  })

  useEffect(() => {
    persist(width)
  }, [width])

  return [width, setWidth] as const
}

export default function KubernetesSettingsPage() {
  const queryClient = useQueryClient()
  const [selectedId, setSelectedId] = useState<string | null>(null)
  const [sidebarWidth, setSidebarWidth] = usePersistedSidebarWidth()
  const resizeRef = useRef<{ startX: number; startW: number } | null>(null)
  const [name, setName] = useState("")
  const [content, setContent] = useState("")
  const [context, setContext] = useState("")
  const [contexts, setContexts] = useState<KubeContext[]>([])
  const [dirty, setDirty] = useState(false)
  const [createOpen, setCreateOpen] = useState(false)
  const [newName, setNewName] = useState("")
  const [newContent, setNewContent] = useState("")
  const [removeTarget, setRemoveTarget] = useState<KubeConfigFile | null>(null)
  const [testResult, setTestResult] = useState<{
    ok: boolean
    message: string
  } | null>(null)

  const configQuery = useQuery({
    queryKey: [K8S_CONFIG_KEY],
    queryFn: getKubeConfig,
  })

  const files = asArray(configQuery.data?.data?.files)
  const activeId = configQuery.data?.data?.active_id || ""
  const activeContextName = configQuery.data?.data?.context || ""
  const secretMap = asArray(
    configQuery.data?.data?.secret_map,
  ) as KubeSecretMapEntry[]
  const mapRows =
    secretMap.length > 0
      ? secretMap
      : files.map((f) => ({
          ...f,
          contexts: f.id === selectedId ? contexts : [],
        }))

  useEffect(() => {
    if (!files.length) {
      setSelectedId(null)
      return
    }
    if (selectedId && files.some((f) => f.id === selectedId)) return
    setSelectedId(activeId || files[0]?.id || null)
  }, [files, activeId, selectedId])

  const fileQuery = useQuery({
    queryKey: [K8S_CONFIG_KEY, "file", selectedId],
    queryFn: () => getKubeConfigFile(selectedId!),
    enabled: !!selectedId,
  })

  useEffect(() => {
    const data = fileQuery.data?.data
    if (!data) return
    setName(data.file.name || "")
    setContent(data.content || "")
    setContexts(asArray(data.contexts))
    setContext(data.current || "")
    setDirty(false)
    setTestResult(null)
  }, [fileQuery.data])

  const invalidate = async () => {
    await queryClient.invalidateQueries({ queryKey: [K8S_CONFIG_KEY] })
    await queryClient.invalidateQueries({ queryKey: [K8S_CLUSTER_KEY] })
  }

  const saveMutation = useMutation({
    mutationFn: () =>
      updateKubeConfigFile(selectedId!, {
        name: name.trim(),
        content,
      }),
    onSuccess: async (res) => {
      toast.success(res.message || "Saved")
      setDirty(false)
      await invalidate()
      if (selectedId) {
        await queryClient.invalidateQueries({
          queryKey: [K8S_CONFIG_KEY, "file", selectedId],
        })
      }
    },
    onError: (err) => toastRequestError(err, "Save failed"),
  })

  const activateMutation = useMutation({
    mutationFn: (vars?: { id?: string; context?: string }) => {
      const id = vars?.id || selectedId
      if (!id) throw new Error("No kubeconfig selected")
      return activateKubeConfigFile(id, {
        context: vars?.context ?? (context || undefined),
      })
    },
    onSuccess: async (res) => {
              toast.success(res.message || "Set as default")
              await invalidate()
            },
            onError: (err) => toastRequestError(err, "Set default failed"),
          })

  const contextMutation = useMutation({
    mutationFn: () =>
      updateKubeConfig({
        active_id: selectedId || undefined,
        context,
      }),
    onSuccess: async (res) => {
      toast.success(res.message || "Context saved")
      await invalidate()
    },
    onError: (err) => toastRequestError(err, "Save context failed"),
  })

  const createMutation = useMutation({
    mutationFn: () =>
      createKubeConfigFile({
        name: newName.trim(),
        content: newContent,
      }),
    onSuccess: async (res) => {
      toast.success(res.message || "Added")
      setCreateOpen(false)
      setNewName("")
      setNewContent("")
      const id = res.data?.file?.id
      await invalidate()
      if (id) setSelectedId(id)
    },
    onError: (err) => toastRequestError(err, "Create failed"),
  })

  const deleteMutation = useMutation({
    mutationFn: (id: string) => deleteKubeConfigFile(id),
    onSuccess: async (res) => {
      toast.success(res.message || "Removed")
      setRemoveTarget(null)
      const next = res.data?.files?.[0]?.id || null
      setSelectedId(next)
      await invalidate()
    },
    onError: (err) => toastRequestError(err, "Delete failed"),
  })

  const testMutation = useMutation({
    mutationFn: (vars?: { id?: string; context?: string }) =>
      testKubeConfig({
        active_id: vars?.id || selectedId || undefined,
        context: vars?.context ?? (context || undefined),
      }),
    onSuccess: (res) => {
      const d = res.data
      if (d.ok) {
        setTestResult({
          ok: true,
          message: `Connected — ${d.version || "ok"} (${d.namespace_count ?? 0} namespaces)`,
        })
      } else {
        setTestResult({ ok: false, message: d.error || "Connection failed" })
      }
    },
    onError: (err) => {
      setTestResult({ ok: false, message: "Test request failed" })
      toastRequestError(err, "Test failed")
    },
  })

  const selected = useMemo(
    () => files.find((f) => f.id === selectedId) || null,
    [files, selectedId],
  )

  return (
    <ContentLoader
      title="Settings"
      breadcrumb={[
        { label: "Kubernetes", to: "/kubernetes" },
        { label: "Settings" },
      ]}
      isLoading={configQuery.isLoading}
      error={configQuery.error}
      rightComponent={
        <div className="flex items-center gap-2">
          <Button
            size="sm"
            variant="outline"
            onClick={() => void configQuery.refetch()}
          >
            <RefreshCw className="size-3.5" />
          </Button>
          <Button size="sm" onClick={() => setCreateOpen(true)}>
            <Plus className="size-3.5" />
            Add config
          </Button>
        </div>
      }
    >
      <div className="space-y-4">
        <div className="flex flex-wrap items-center justify-between gap-3 rounded-xl border px-4 py-3">
          <div className="min-w-0">
            <h3 className="text-sm font-medium">System resources</h3>
            <p className="text-xs text-muted-foreground">
              When off, hide system namespaces (kube-system, kube-public,
              kube-node-lease, and common platform namespaces) and their
              workloads across Kubernetes lists.
            </p>
          </div>
          <SystemResourcesToggle id="k8s-settings-show-system" />
        </div>

        <div className="rounded-xl border bg-muted/20 p-4 text-sm text-muted-foreground">
          Each kubeconfig is stored as a <strong className="text-foreground">secret</strong>{" "}
          in SQLite (<code className="text-xs">k8s_keys</code>: name, path, secret).
          On startup, missing secrets are seeded from the host profile file{" "}
          <code className="text-xs">/root/.kube/config</code> (or{" "}
          <code className="text-xs">$KUBECONFIG</code>). Each{" "}
          <strong className="text-foreground">context</strong> inside a secret
          points at a <strong className="text-foreground">cluster</strong>. Pick
          Pick which secret and context are the default for the UI and MCP tools.
          Managed copies also live under{" "}
          <code className="text-xs">/root/.kube/containerws/configs</code>.
        </div>

        {mapRows.length > 0 ? (
          <div className="rounded-xl border p-4">
            <div className="mb-3 flex items-baseline justify-between gap-2">
              <div>
                <h3 className="text-sm font-medium">Secret → cluster map</h3>
                <p className="text-xs text-muted-foreground">
                  Which kubeconfig secret is used for which cluster. Activate the
                  pair MCP and the Kubernetes UI should use.
                </p>
              </div>
            </div>
            <div className="overflow-x-auto">
              <table className="w-full min-w-[520px] text-left text-sm">
                <thead>
                  <tr className="border-b text-xs text-muted-foreground">
                    <th className="pb-2 pr-3 font-medium">Secret</th>
                    <th className="pb-2 pr-3 font-medium">Context</th>
                    <th className="pb-2 pr-3 font-medium">Cluster</th>
                    <th className="pb-2 pr-3 font-medium">User</th>
                    <th className="pb-2 font-medium">Status</th>
                  </tr>
                </thead>
                <tbody>
                  {mapRows.flatMap((f) => {
                    const rows =
                      f.contexts?.length > 0
                        ? f.contexts
                        : [
                            {
                              name: f.exists ? "(no contexts)" : "(file missing)",
                              cluster: "",
                              user: "",
                              current: false,
                            } satisfies KubeContext,
                          ]
                    return rows.map((cx, idx) => {
                      const isActivePair =
                        f.active &&
                        !!cx.name &&
                        !cx.name.startsWith("(") &&
                        (activeContextName
                          ? activeContextName === cx.name
                          : cx.current)
                      return (
                        <tr
                          key={`${f.id}-${cx.name}-${idx}`}
                          className={cn(
                            "border-b border-border/60 last:border-0",
                            selectedId === f.id && "bg-muted/40",
                          )}
                        >
                          <td className="py-2 pr-3 align-top">
                            {idx === 0 ? (
                              <button
                                type="button"
                                className="text-left font-medium hover:underline"
                                onClick={() => setSelectedId(f.id)}
                              >
                                {f.name}
                                {f.active ? (
                                  <Badge
                                    variant="default"
                                    className="ml-2 align-middle text-[10px]"
                                  >
                                    default secret
                                  </Badge>
                                ) : null}
                              </button>
                            ) : (
                              <span className="text-muted-foreground">↳</span>
                            )}
                          </td>
                          <td className="py-2 pr-3 font-mono text-xs">{cx.name}</td>
                          <td className="py-2 pr-3 font-mono text-xs">
                            {cx.cluster || "—"}
                          </td>
                          <td className="py-2 pr-3 font-mono text-xs">
                            {cx.user || "—"}
                          </td>
                          <td className="py-2">
                            {isActivePair ? (
                              <Badge variant="default" className="text-[10px]">
                                default for MCP/UI
                              </Badge>
                            ) : null}
                          </td>
                        </tr>
                      )
                    })
                  })}
                </tbody>
              </table>
            </div>
            {selected && contexts.length > 1 ? (
              <p className="mt-3 text-xs text-muted-foreground">
                This secret has {contexts.length} clusters/contexts. Choose one
                below, then use{" "}
                <span className="font-medium text-foreground">
                  Set as default
                </span>{" "}
                or{" "}
                <span className="font-medium text-foreground">Save context</span>.
              </p>
            ) : null}
          </div>
        ) : null}

        <div className="flex min-h-[28rem] gap-0 overflow-hidden rounded-xl border lg:min-h-[32rem]">
          <aside
            className="relative flex shrink-0 flex-col border-e bg-muted/10"
            style={{ width: sidebarWidth }}
          >
            <div className="flex items-center justify-between gap-2 border-b px-2 py-2">
              <p className="truncate px-1 text-xs font-medium text-muted-foreground">
                Secrets (kubeconfigs)
              </p>
              <Button
                type="button"
                size="icon-sm"
                variant="ghost"
                title="Add kubeconfig"
                onClick={() => setCreateOpen(true)}
              >
                <Plus className="size-3.5" />
              </Button>
            </div>

            <div className="min-h-0 flex-1 overflow-y-auto p-2">
              {files.length === 0 ? (
                <p className="px-2 py-6 text-center text-xs text-muted-foreground">
                  No configs yet. Add one by pasting YAML.
                </p>
              ) : (
                <ul className="space-y-1">
                  {files.map((f) => (
                    <li key={f.id}>
                      <div
                        className={cn(
                          "group flex items-start gap-0.5 rounded-lg transition-colors",
                          selectedId === f.id
                            ? "bg-muted"
                            : "hover:bg-muted/50",
                        )}
                      >
                        <button
                          type="button"
                          onClick={() => setSelectedId(f.id)}
                          className="min-w-0 flex-1 px-2 py-2 text-left text-sm"
                        >
                          <span className="block truncate font-medium">
                            {f.name}
                          </span>
                          <span className="block truncate font-mono text-[10px] text-muted-foreground">
                            {f.path}
                          </span>
                          <span className="mt-1 flex flex-wrap gap-1">
                            {f.active ? (
                              <Badge
                                variant="default"
                                className="text-[10px]"
                              >
                                default
                              </Badge>
                            ) : null}
                            {!f.exists ? (
                              <Badge
                                variant="destructive"
                                className="text-[10px]"
                              >
                                missing
                              </Badge>
                            ) : null}
                          </span>
                        </button>
                        <DropdownMenu>
                          <DropdownMenuTrigger asChild>
                            <Button
                              type="button"
                              size="icon-sm"
                              variant="ghost"
                              className="mt-1.5 me-1 shrink-0 opacity-70 group-hover:opacity-100 data-[state=open]:opacity-100"
                              aria-label={`Actions for ${f.name}`}
                            >
                              <MoreHorizontal className="size-3.5" />
                            </Button>
                          </DropdownMenuTrigger>
                          <DropdownMenuContent align="end" className="w-44">
                            <DropdownMenuItem
                              onClick={() => setSelectedId(f.id)}
                            >
                              Open
                            </DropdownMenuItem>
                            <DropdownMenuItem
                              disabled={
                                f.active ||
                                !f.exists ||
                                activateMutation.isPending
                              }
                              onClick={() => {
                                setSelectedId(f.id)
                                activateMutation.mutate({ id: f.id })
                              }}
                            >
                              <Zap className="size-3.5" />
                              Set as default
                            </DropdownMenuItem>
                            <DropdownMenuItem
                              disabled={!f.exists || testMutation.isPending}
                              onClick={() => {
                                setSelectedId(f.id)
                                testMutation.mutate({ id: f.id })
                              }}
                            >
                              Test connection
                            </DropdownMenuItem>
                            <DropdownMenuSeparator />
                            <DropdownMenuItem
                              variant="destructive"
                              disabled={deleteMutation.isPending}
                              onClick={() => setRemoveTarget(f)}
                            >
                              <Trash2 className="size-3.5" />
                              Delete
                            </DropdownMenuItem>
                          </DropdownMenuContent>
                        </DropdownMenu>
                      </div>
                    </li>
                  ))}
                </ul>
              )}
            </div>

            <div
              role="separator"
              aria-orientation="vertical"
              aria-label="Resize secrets sidebar"
              tabIndex={0}
              className="absolute inset-y-0 -end-1 z-10 w-2 cursor-col-resize touch-none"
              onPointerDown={(e) => {
                e.preventDefault()
                resizeRef.current = {
                  startX: e.clientX,
                  startW: sidebarWidth,
                }
                const target = e.currentTarget
                target.setPointerCapture(e.pointerId)

                const onMove = (ev: PointerEvent) => {
                  const start = resizeRef.current
                  if (!start) return
                  const next = Math.min(
                    SIDEBAR_MAX,
                    Math.max(
                      SIDEBAR_MIN,
                      start.startW + (ev.clientX - start.startX),
                    ),
                  )
                  setSidebarWidth(next)
                }
                const onUp = (ev: PointerEvent) => {
                  resizeRef.current = null
                  target.releasePointerCapture(ev.pointerId)
                  target.removeEventListener("pointermove", onMove)
                  target.removeEventListener("pointerup", onUp)
                  target.removeEventListener("pointercancel", onUp)
                }
                target.addEventListener("pointermove", onMove)
                target.addEventListener("pointerup", onUp)
                target.addEventListener("pointercancel", onUp)
              }}
              onKeyDown={(e) => {
                if (e.key === "ArrowLeft") {
                  e.preventDefault()
                  setSidebarWidth((w) => Math.max(SIDEBAR_MIN, w - 16))
                } else if (e.key === "ArrowRight") {
                  e.preventDefault()
                  setSidebarWidth((w) => Math.min(SIDEBAR_MAX, w + 16))
                }
              }}
            >
              <span className="absolute inset-y-3 end-0 w-px rounded-full bg-border transition-colors hover:bg-foreground/30 group-active:bg-foreground/40" />
            </div>
          </aside>

          <div className="min-w-0 flex-1 space-y-4 overflow-auto p-4">
            {!selectedId ? (
              <p className="py-12 text-center text-sm text-muted-foreground">
                Select a config or add a new one.
              </p>
            ) : fileQuery.isLoading ? (
              <p className="py-12 text-center text-sm text-muted-foreground">
                Loading…
              </p>
            ) : fileQuery.error ? (
              <p className="py-12 text-center text-sm text-destructive">
                Failed to load kubeconfig content.
              </p>
            ) : (
              <>
                <div className="grid gap-4 sm:grid-cols-2">
                  <div className="space-y-2">
                    <Label htmlFor="kube-name">Name</Label>
                    <Input
                      id="kube-name"
                      value={name}
                      onChange={(e) => {
                        setName(e.target.value)
                        setDirty(true)
                      }}
                      placeholder="production"
                    />
                  </div>
                  <div className="space-y-2">
                    <Label>Cluster context</Label>
                    <ReactSelect<{ label: string; value: string }, false>
                      size="default"
                      className="w-full"
                      isSearchable
                      isClearable={false}
                      options={[
                        {
                          label: "Current context from file",
                          value: "__default__",
                        },
                        ...contexts.map((cx) => ({
                          value: cx.name,
                          label: cx.cluster
                            ? `${cx.name} → cluster ${cx.cluster}`
                            : cx.name,
                        })),
                      ]}
                      value={context || "__default__"}
                      onValueChange={(v) =>
                        setContext(!v || v === "__default__" ? "" : v)
                      }
                      placeholder="Select context…"
                      noOptionsMessage={() => "No contexts in this kubeconfig"}
                    />
                    <p className="text-[11px] text-muted-foreground">
                      Context selects which cluster this secret talks to.
                    </p>
                  </div>
                </div>

                <div className="flex flex-wrap items-center gap-2 rounded-lg border bg-muted/20 px-3 py-2.5">
                  <div className="min-w-0 flex-1">
                    <p className="text-sm font-medium">
                      {selected?.active
                        ? "Default kubeconfig"
                        : "Not the default"}
                    </p>
                    <p className="text-xs text-muted-foreground">
                      {selected?.active
                        ? "UI and MCP tools use this secret and its selected context."
                        : "Set this secret as default so Kubernetes pages and MCP use it."}
                    </p>
                  </div>
                  {selected?.active ? (
                    <Badge variant="default">default</Badge>
                  ) : (
                    <Button
                      type="button"
                      size="sm"
                      disabled={
                        activateMutation.isPending ||
                        !selectedId ||
                        !selected?.exists ||
                        dirty
                      }
                      onClick={() =>
                        activateMutation.mutate({ id: selectedId || undefined })
                      }
                    >
                      <Zap className="size-3.5" />
                      Set as default
                    </Button>
                  )}
                </div>

                <div className="space-y-2">
                  <div className="flex items-center justify-between gap-2">
                    <Label htmlFor="kube-content">Kubeconfig YAML</Label>
                    {dirty ? (
                      <Badge variant="secondary">unsaved</Badge>
                    ) : null}
                  </div>
                  <div id="kube-content">
                    <KubeconfigYamlEditor
                      value={content}
                      onChange={(next) => {
                        setContent(next)
                        setDirty(true)
                      }}
                      height="min(60vh, 28rem)"
                    />
                  </div>
                  <p className="truncate font-mono text-[11px] text-muted-foreground">
                    {selected?.path}
                  </p>
                </div>

                {testResult ? (
                  <div
                    className={`flex items-start gap-2 rounded-lg border px-3 py-2 text-sm ${
                      testResult.ok
                        ? "border-emerald-500/30 bg-emerald-500/10 text-emerald-900 dark:text-emerald-200"
                        : "border-destructive/30 bg-destructive/10 text-destructive"
                    }`}
                  >
                    {testResult.ok ? (
                      <CheckCircle2 className="mt-0.5 size-4 shrink-0" />
                    ) : (
                      <XCircle className="mt-0.5 size-4 shrink-0" />
                    )}
                    <span>{testResult.message}</span>
                  </div>
                ) : null}

                <div className="flex flex-wrap gap-2">
                  <Button
                    disabled={
                      saveMutation.isPending || !name.trim() || !content.trim()
                    }
                    onClick={() => saveMutation.mutate()}
                  >
                    Save file
                  </Button>
                  <Button
                    variant="outline"
                    disabled={
                      activateMutation.isPending ||
                      !selectedId ||
                      selected?.active ||
                      dirty ||
                      !selected?.exists
                    }
                    onClick={() =>
                      activateMutation.mutate({ id: selectedId || undefined })
                    }
                  >
                    <Zap className="size-3.5" />
                    Set as default
                  </Button>
                  <Button
                    variant="outline"
                    disabled={
                      contextMutation.isPending ||
                      !selected?.active ||
                      dirty
                    }
                    onClick={() => contextMutation.mutate()}
                  >
                    Save context
                  </Button>
                  <Button
                    variant="outline"
                    disabled={testMutation.isPending || dirty}
                    onClick={() => testMutation.mutate(undefined)}
                  >
                    Test connection
                  </Button>
                  <Button
                    variant="destructive"
                    disabled={deleteMutation.isPending || !selected}
                    onClick={() => setRemoveTarget(selected)}
                  >
                    <Trash2 className="size-3.5" />
                    Remove
                  </Button>
                </div>
              </>
            )}
          </div>
        </div>
      </div>

      <Dialog open={createOpen} onOpenChange={setCreateOpen}>
        <DialogContent className="max-w-[calc(100%-2rem)] sm:max-w-4xl">
          <DialogHeader>
            <DialogTitle>Add kubeconfig secret</DialogTitle>
            <DialogDescription>
              Paste a full kubeconfig YAML (clusters, users, contexts). This secret
              can contain one or more clusters — you will choose which context/cluster
              is active after adding it.
            </DialogDescription>
          </DialogHeader>
          <div className="min-w-0 space-y-3">
            <div className="space-y-2">
              <Label>Name</Label>
              <Input
                value={newName}
                onChange={(e) => setNewName(e.target.value)}
                placeholder="staging"
              />
            </div>
            <div className="min-w-0 space-y-2">
              <Label>Config YAML</Label>
              <KubeconfigYamlEditor
                value={newContent}
                onChange={setNewContent}
                height="min(50vh, 20rem)"
              />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setCreateOpen(false)}>
              Cancel
            </Button>
            <Button
              disabled={
                createMutation.isPending ||
                !newName.trim() ||
                !newContent.trim()
              }
              onClick={() => createMutation.mutate()}
            >
              Add secret
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <AlertDialog
        open={!!removeTarget}
        onOpenChange={(o) => !o && setRemoveTarget(null)}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete kubeconfig secret?</AlertDialogTitle>
            <AlertDialogDescription>
              Delete “{removeTarget?.name}”?
              {removeTarget?.managed
                ? " The managed file on disk will be permanently deleted."
                : " The registry entry will be removed; the host file is left in place."}
              {removeTarget?.active
                ? " This secret is currently the default."
                : ""}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={deleteMutation.isPending}>
              Cancel
            </AlertDialogCancel>
            <AlertDialogAction
              className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
              disabled={deleteMutation.isPending || !removeTarget}
              onClick={(e) => {
                e.preventDefault()
                if (removeTarget) deleteMutation.mutate(removeTarget.id)
              }}
            >
              {deleteMutation.isPending ? "Deleting…" : "Delete"}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </ContentLoader>
  )
}
