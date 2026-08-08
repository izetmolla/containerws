import { useMemo, useState } from "react"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import {
  createColumnHelper,
  type ColumnDef,
} from "@tanstack/react-table"
import { Loader2, MoreHorizontal, Plus } from "lucide-react"
import { toast } from "sonner"

import ContentLoader from "@/components/content-loader"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { DataTable } from "@/components/ui/data-table"
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { ReactSelect } from "@/components/ui/reactselect"
import { Switch } from "@/components/ui/switch"
import { asArray } from "@/lib/as-array"
import { toastRequestError } from "@/lib/network"

import {
  createProxyHost,
  deleteProxyHost,
  getProxySettings,
  listProxyCertificates,
  listProxyHosts,
  PROXY_CERTS_KEY,
  PROXY_HOSTS_KEY,
  PROXY_SETTINGS_KEY,
  updateProxyHost,
  type ProxyHost,
  type UpsertHost,
} from "../_shared/api"
import { invalidateProxyQueries, runProxyApply } from "../_shared/apply"
import {
  DirtyBanner,
  PROXY_PAGE_DESCRIPTIONS,
  ProxyRefreshButton,
  ProxySubNav,
} from "../_shared/page-chrome"

const columnHelper = createColumnHelper<ProxyHost>()

const SCHEME_OPTIONS = [
  { value: "http", label: "HTTP only" },
  { value: "https", label: "HTTPS only" },
  { value: "both", label: "HTTP & HTTPS" },
]

const FORWARD_SCHEME_OPTIONS = [
  { value: "http", label: "http" },
  { value: "https", label: "https" },
]

const emptyForm = (): UpsertHost => ({
  name: "",
  domains: "",
  enabled: true,
  listen_scheme: "http",
  forward_scheme: "http",
  forward_host: "127.0.0.1",
  forward_port: 8080,
  upstream_type: "url",
  websocket: true,
  ssl_forced: false,
  block_exploits: true,
  caching_enabled: false,
  http2_support: true,
  certificate_id: null,
  locations: [],
})

export default function ProxyHostsPage() {
  const queryClient = useQueryClient()
  const [editorOpen, setEditorOpen] = useState(false)
  const [editing, setEditing] = useState<ProxyHost | null>(null)
  const [form, setForm] = useState<UpsertHost>(emptyForm())
  const [applyAfter, setApplyAfter] = useState(true)

  const listQuery = useQuery({
    queryKey: [PROXY_HOSTS_KEY],
    queryFn: listProxyHosts,
  })
  const settingsQuery = useQuery({
    queryKey: [PROXY_SETTINGS_KEY],
    queryFn: getProxySettings,
  })
  const certsQuery = useQuery({
    queryKey: [PROXY_CERTS_KEY],
    queryFn: listProxyCertificates,
  })

  const certOptions = useMemo(
    () =>
      asArray(certsQuery.data?.data).map((c) => ({
        value: c.id,
        label: c.name,
      })),
    [certsQuery.data],
  )

  const saveMutation = useMutation({
    mutationFn: async () => {
      const body: UpsertHost = {
        ...form,
        upstream_target: undefined,
      }
      if (editing) return updateProxyHost(editing.id, body)
      return createProxyHost(body)
    },
    onSuccess: async (res) => {
      toast.success(res.message || "Host saved")
      setEditorOpen(false)
      setEditing(null)
      setForm(emptyForm())
      await invalidateProxyQueries(queryClient)
      if (applyAfter) {
        try {
          await runProxyApply(queryClient)
        } catch {
          /* toast already shown */
        }
      }
    },
    onError: (err) => toastRequestError(err, "Save failed"),
  })

  const deleteMutation = useMutation({
    mutationFn: async (id: string) => {
      await deleteProxyHost(id)
      await invalidateProxyQueries(queryClient)
      if (applyAfter) {
        try {
          await runProxyApply(queryClient)
        } catch {
          /* toast already shown */
        }
      }
    },
    onSuccess: () => toast.success("Host deleted"),
    onError: (err) => toastRequestError(err, "Delete failed"),
  })

  const applyMutation = useMutation({
    mutationFn: () => runProxyApply(queryClient),
  })

  const rows = asArray(listQuery.data?.data)

  const columns = useMemo(
    () =>
      [
        columnHelper.accessor("name", { header: "Name" }),
        columnHelper.accessor("domains", {
          header: "Domain names",
          cell: (info) => (
            <span className="font-mono text-xs">{info.getValue()}</span>
          ),
        }),
        columnHelper.display({
          id: "forward",
          header: "Forward to",
          cell: ({ row }) => {
            const h = row.original
            const target =
              h.forward_host
                ? `${h.forward_scheme}://${h.forward_host}:${h.forward_port}`
                : h.upstream_target
            return <span className="font-mono text-xs">{target}</span>
          },
        }),
        columnHelper.accessor("listen_scheme", {
          header: "Scheme",
          cell: (info) => <Badge variant="secondary">{info.getValue()}</Badge>,
        }),
        columnHelper.accessor("enabled", {
          header: "Status",
          cell: (info) =>
            info.getValue() ? (
              <Badge>Online</Badge>
            ) : (
              <Badge variant="outline">Disabled</Badge>
            ),
        }),
        columnHelper.display({
          id: "actions",
          cell: ({ row }) => (
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button variant="ghost" size="icon" className="size-8">
                  <MoreHorizontal className="size-4" />
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end">
                <DropdownMenuItem
                  onClick={() => {
                    const h = row.original
                    setEditing(h)
                    setForm({
                      name: h.name,
                      domains: h.domains,
                      enabled: h.enabled,
                      listen_scheme: h.listen_scheme,
                      forward_scheme: h.forward_scheme || "http",
                      forward_host: h.forward_host || "",
                      forward_port: h.forward_port || 80,
                      upstream_type: h.upstream_type,
                      websocket: h.websocket,
                      ssl_forced: h.ssl_forced,
                      block_exploits: h.block_exploits,
                      caching_enabled: h.caching_enabled,
                      http2_support: h.http2_support,
                      notes: h.notes,
                      certificate_id: h.certificate_id,
                      locations: h.locations || [],
                    })
                    setEditorOpen(true)
                  }}
                >
                  Edit
                </DropdownMenuItem>
                <DropdownMenuItem
                  className="text-destructive"
                  onClick={() => deleteMutation.mutate(row.original.id)}
                >
                  Delete
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          ),
        }),
      ] as ColumnDef<ProxyHost, unknown>[],
    [deleteMutation],
  )

  const needsCert =
    form.listen_scheme === "https" ||
    form.listen_scheme === "both" ||
    form.ssl_forced

  return (
    <ContentLoader
      title="Proxy Hosts"
      description={PROXY_PAGE_DESCRIPTIONS.hosts}
      breadcrumb={[
        { label: "Proxy Manager", to: "/proxymanager" },
        { label: "Hosts" },
      ]}
      isLoading={listQuery.isLoading}
      error={listQuery.error}
      rightComponent={
        <div className="flex gap-2">
          <ProxyRefreshButton
            isFetching={listQuery.isFetching}
            onClick={() => void listQuery.refetch()}
          />
          <Button
            size="sm"
            variant="outline"
            onClick={() => applyMutation.mutate()}
            disabled={applyMutation.isPending}
          >
            {applyMutation.isPending ? (
              <Loader2 className="size-3.5 animate-spin" />
            ) : null}
            Apply
          </Button>
          <Button
            size="sm"
            onClick={() => {
              setEditing(null)
              setForm(emptyForm())
              setEditorOpen(true)
            }}
          >
            <Plus className="size-3.5" />
            Add proxy host
          </Button>
        </div>
      }
    >
      <ProxySubNav />
      <DirtyBanner
        dirty={settingsQuery.data?.data?.dirty}
        lastError={settingsQuery.data?.data?.last_apply_error}
        onApply={() => applyMutation.mutate()}
        applying={applyMutation.isPending}
      />
      <DataTable columns={columns} data={rows} />

      <Dialog open={editorOpen} onOpenChange={setEditorOpen}>
        <DialogContent className="max-h-[90vh] max-w-lg overflow-y-auto">
          <DialogHeader>
            <DialogTitle>
              {editing ? "Edit proxy host" : "New proxy host"}
            </DialogTitle>
          </DialogHeader>
          <div className="grid gap-4 py-2">
            <div className="space-y-2">
              <Label>Domain names</Label>
              <Input
                value={form.domains}
                placeholder="app.example.com, www.example.com"
                onChange={(e) =>
                  setForm((f) => ({
                    ...f,
                    domains: e.target.value,
                    name: f.name || e.target.value.split(",")[0]?.trim() || "",
                  }))
                }
              />
              <p className="text-xs text-muted-foreground">
                Comma-separated hostnames (like Nginx Proxy Manager).
              </p>
            </div>
            <div className="space-y-2">
              <Label>Friendly name</Label>
              <Input
                value={form.name}
                onChange={(e) => setForm((f) => ({ ...f, name: e.target.value }))}
              />
            </div>

            <div className="grid gap-3 sm:grid-cols-3">
              <div className="space-y-2 sm:col-span-1">
                <Label>Forward scheme</Label>
                <ReactSelect
                  options={FORWARD_SCHEME_OPTIONS}
                  value={FORWARD_SCHEME_OPTIONS.find(
                    (o) => o.value === form.forward_scheme,
                  )}
                  onChange={(opt) =>
                    setForm((f) => ({
                      ...f,
                      forward_scheme: opt?.value || "http",
                    }))
                  }
                />
              </div>
              <div className="space-y-2 sm:col-span-1">
                <Label>Forward hostname / IP</Label>
                <Input
                  value={form.forward_host || ""}
                  placeholder="127.0.0.1"
                  onChange={(e) =>
                    setForm((f) => ({ ...f, forward_host: e.target.value }))
                  }
                />
              </div>
              <div className="space-y-2 sm:col-span-1">
                <Label>Forward port</Label>
                <Input
                  type="number"
                  value={form.forward_port ?? 80}
                  onChange={(e) =>
                    setForm((f) => ({
                      ...f,
                      forward_port: Number(e.target.value),
                    }))
                  }
                />
              </div>
            </div>

            <div className="space-y-2">
              <Label>Public listen scheme</Label>
              <ReactSelect
                options={SCHEME_OPTIONS}
                value={SCHEME_OPTIONS.find((o) => o.value === form.listen_scheme)}
                onChange={(opt) =>
                  setForm((f) => ({
                    ...f,
                    listen_scheme: opt?.value || "http",
                  }))
                }
              />
            </div>

            {needsCert ? (
              <div className="space-y-2">
                <Label>SSL certificate</Label>
                <ReactSelect
                  options={certOptions}
                  isClearable
                  value={
                    certOptions.find((o) => o.value === form.certificate_id) ||
                    null
                  }
                  onChange={(opt) =>
                    setForm((f) => ({
                      ...f,
                      certificate_id: opt?.value || null,
                    }))
                  }
                  placeholder="Select certificate…"
                />
              </div>
            ) : null}

            <div className="grid gap-3 sm:grid-cols-2">
              <div className="flex items-center justify-between gap-2 rounded-lg border px-3 py-2">
                <Label htmlFor="ws">Websockets</Label>
                <Switch
                  id="ws"
                  checked={!!form.websocket}
                  onCheckedChange={(v) =>
                    setForm((f) => ({ ...f, websocket: v }))
                  }
                />
              </div>
              <div className="flex items-center justify-between gap-2 rounded-lg border px-3 py-2">
                <Label htmlFor="sslforce">Force SSL</Label>
                <Switch
                  id="sslforce"
                  checked={!!form.ssl_forced}
                  onCheckedChange={(v) =>
                    setForm((f) => ({ ...f, ssl_forced: v }))
                  }
                />
              </div>
              <div className="flex items-center justify-between gap-2 rounded-lg border px-3 py-2">
                <Label htmlFor="block">Block exploits</Label>
                <Switch
                  id="block"
                  checked={form.block_exploits !== false}
                  onCheckedChange={(v) =>
                    setForm((f) => ({ ...f, block_exploits: v }))
                  }
                />
              </div>
              <div className="flex items-center justify-between gap-2 rounded-lg border px-3 py-2">
                <Label htmlFor="cache">Caching</Label>
                <Switch
                  id="cache"
                  checked={!!form.caching_enabled}
                  onCheckedChange={(v) =>
                    setForm((f) => ({ ...f, caching_enabled: v }))
                  }
                />
              </div>
              <div className="flex items-center justify-between gap-2 rounded-lg border px-3 py-2">
                <Label htmlFor="http2">HTTP/2</Label>
                <Switch
                  id="http2"
                  checked={form.http2_support !== false}
                  onCheckedChange={(v) =>
                    setForm((f) => ({ ...f, http2_support: v }))
                  }
                />
              </div>
              <div className="flex items-center justify-between gap-2 rounded-lg border px-3 py-2">
                <Label htmlFor="en">Enabled</Label>
                <Switch
                  id="en"
                  checked={form.enabled !== false}
                  onCheckedChange={(v) =>
                    setForm((f) => ({ ...f, enabled: v }))
                  }
                />
              </div>
            </div>

            <div className="flex items-center justify-between gap-2 rounded-lg border border-dashed px-3 py-2">
              <Label htmlFor="apply">Apply after save</Label>
              <Switch
                id="apply"
                checked={applyAfter}
                onCheckedChange={setApplyAfter}
              />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setEditorOpen(false)}>
              Cancel
            </Button>
            <Button
              onClick={() => saveMutation.mutate()}
              disabled={saveMutation.isPending}
            >
              {saveMutation.isPending ? (
                <Loader2 className="size-3.5 animate-spin" />
              ) : null}
              Save
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </ContentLoader>
  )
}
