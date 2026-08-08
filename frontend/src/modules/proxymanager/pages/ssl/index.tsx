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
import { Separator } from "@/components/ui/separator"
import { Switch } from "@/components/ui/switch"
import { Textarea } from "@/components/ui/textarea"
import { asArray } from "@/lib/as-array"
import { toastRequestError } from "@/lib/network"

import {
  createProxyCertificate,
  createProxyRedirect,
  deleteProxyCertificate,
  deleteProxyRedirect,
  getProxySettings,
  listProxyCertificates,
  listProxyRedirects,
  PROXY_CERTS_KEY,
  PROXY_REDIRECTS_KEY,
  PROXY_SETTINGS_KEY,
  updateProxyCertificate,
  updateProxyRedirect,
  type ProxyCertificate,
  type ProxyRedirect,
  type UpsertCert,
  type UpsertRedirect,
} from "../_shared/api"
import { invalidateProxyQueries, runProxyApply } from "../_shared/apply"
import {
  DirtyBanner,
  PROXY_PAGE_DESCRIPTIONS,
  ProxyRefreshButton,
  ProxySubNav,
} from "../_shared/page-chrome"

const certHelper = createColumnHelper<ProxyCertificate>()
const redirHelper = createColumnHelper<ProxyRedirect>()

const SOURCE_OPTIONS = [
  { value: "upload", label: "Upload / paste PEM" },
  { value: "path", label: "Filesystem path" },
  { value: "letsencrypt", label: "Let's Encrypt (stub)" },
]

const CODE_OPTIONS = [
  { value: "301", label: "301" },
  { value: "302", label: "302" },
  { value: "307", label: "307" },
  { value: "308", label: "308" },
]

const emptyCert = (): UpsertCert => ({
  name: "",
  domains: "",
  source: "upload",
  cert_pem: "",
  key_pem: "",
  cert_path: "",
  key_path: "",
})

const emptyRedirect = (): UpsertRedirect => ({
  name: "",
  enabled: true,
  from_host: "",
  from_path: "/",
  to_url: "https://",
  status_code: 301,
  preserve_path: false,
})

export default function ProxySslPage() {
  const queryClient = useQueryClient()
  const [certOpen, setCertOpen] = useState(false)
  const [editingCert, setEditingCert] = useState<ProxyCertificate | null>(null)
  const [certForm, setCertForm] = useState<UpsertCert>(emptyCert())

  const [redirOpen, setRedirOpen] = useState(false)
  const [editingRedir, setEditingRedir] = useState<ProxyRedirect | null>(null)
  const [redirForm, setRedirForm] = useState<UpsertRedirect>(emptyRedirect())
  const [applyAfter, setApplyAfter] = useState(true)

  const certsQuery = useQuery({
    queryKey: [PROXY_CERTS_KEY],
    queryFn: listProxyCertificates,
  })
  const redirsQuery = useQuery({
    queryKey: [PROXY_REDIRECTS_KEY],
    queryFn: listProxyRedirects,
  })
  const settingsQuery = useQuery({
    queryKey: [PROXY_SETTINGS_KEY],
    queryFn: getProxySettings,
  })

  const maybeApply = async () => {
    if (!applyAfter) return
    try {
      await runProxyApply(queryClient)
    } catch {
      /* toast already shown */
    }
  }

  const saveCert = useMutation({
    mutationFn: async () => {
      if (editingCert) return updateProxyCertificate(editingCert.id, certForm)
      return createProxyCertificate(certForm)
    },
    onSuccess: async (res) => {
      toast.success(res.message || "Saved")
      setCertOpen(false)
      setEditingCert(null)
      setCertForm(emptyCert())
      await invalidateProxyQueries(queryClient)
      await maybeApply()
    },
    onError: (err) => toastRequestError(err, "Save failed"),
  })

  const deleteCert = useMutation({
    mutationFn: async (id: string) => {
      await deleteProxyCertificate(id)
      await invalidateProxyQueries(queryClient)
      await maybeApply()
    },
    onSuccess: () => toast.success("Deleted"),
    onError: (err) => toastRequestError(err, "Delete failed"),
  })

  const saveRedir = useMutation({
    mutationFn: async () => {
      if (editingRedir) return updateProxyRedirect(editingRedir.id, redirForm)
      return createProxyRedirect(redirForm)
    },
    onSuccess: async (res) => {
      toast.success(res.message || "Saved")
      setRedirOpen(false)
      setEditingRedir(null)
      setRedirForm(emptyRedirect())
      await invalidateProxyQueries(queryClient)
      await maybeApply()
    },
    onError: (err) => toastRequestError(err, "Save failed"),
  })

  const deleteRedir = useMutation({
    mutationFn: async (id: string) => {
      await deleteProxyRedirect(id)
      await invalidateProxyQueries(queryClient)
      await maybeApply()
    },
    onSuccess: () => toast.success("Deleted"),
    onError: (err) => toastRequestError(err, "Delete failed"),
  })

  const applyMutation = useMutation({
    mutationFn: () => runProxyApply(queryClient),
  })

  const busy =
    saveCert.isPending ||
    saveRedir.isPending ||
    deleteCert.isPending ||
    deleteRedir.isPending ||
    applyMutation.isPending

  const certs = asArray(certsQuery.data?.data)
  const redirs = asArray(redirsQuery.data?.data)

  const certColumns = useMemo(
    () =>
      [
        certHelper.accessor("name", { header: "Name" }),
        certHelper.accessor("domains", { header: "Domains" }),
        certHelper.accessor("source", {
          header: "Source",
          cell: (info) => <Badge variant="secondary">{info.getValue()}</Badge>,
        }),
        certHelper.display({
          id: "pem",
          header: "Material",
          cell: ({ row }) =>
            row.original.has_cert_pem || row.original.cert_path
              ? "Present"
              : "Missing",
        }),
        certHelper.display({
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
                    const c = row.original
                    setEditingCert(c)
                    setCertForm({
                      name: c.name,
                      domains: c.domains,
                      source: c.source,
                      cert_path: c.cert_path,
                      key_path: c.key_path,
                      letsencrypt_email: c.letsencrypt_email,
                      notes: c.notes,
                    })
                    setCertOpen(true)
                  }}
                >
                  Edit
                </DropdownMenuItem>
                <DropdownMenuItem
                  className="text-destructive"
                  onClick={() => deleteCert.mutate(row.original.id)}
                >
                  Delete
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          ),
        }),
      ] as ColumnDef<ProxyCertificate, unknown>[],
    [deleteCert],
  )

  const redirColumns = useMemo(
    () =>
      [
        redirHelper.accessor("name", { header: "Name" }),
        redirHelper.accessor("from_host", { header: "From host" }),
        redirHelper.accessor("from_path", { header: "Path" }),
        redirHelper.accessor("to_url", { header: "To" }),
        redirHelper.accessor("status_code", { header: "Code" }),
        redirHelper.accessor("enabled", {
          header: "Enabled",
          cell: (info) =>
            info.getValue() ? <Badge>On</Badge> : <Badge variant="outline">Off</Badge>,
        }),
        redirHelper.display({
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
                    const r = row.original
                    setEditingRedir(r)
                    setRedirForm({
                      name: r.name,
                      enabled: r.enabled,
                      from_host: r.from_host,
                      from_path: r.from_path,
                      to_url: r.to_url,
                      status_code: r.status_code,
                      preserve_path: r.preserve_path,
                      notes: r.notes,
                    })
                    setRedirOpen(true)
                  }}
                >
                  Edit
                </DropdownMenuItem>
                <DropdownMenuItem
                  className="text-destructive"
                  onClick={() => deleteRedir.mutate(row.original.id)}
                >
                  Delete
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          ),
        }),
      ] as ColumnDef<ProxyRedirect, unknown>[],
    [deleteRedir],
  )

  return (
    <ContentLoader
      title="SSL & Redirects"
      description={PROXY_PAGE_DESCRIPTIONS.ssl}
      breadcrumb={[
        { label: "Proxy Manager", to: "/proxymanager" },
        { label: "SSL" },
      ]}
      isLoading={certsQuery.isLoading || redirsQuery.isLoading}
      error={certsQuery.error || redirsQuery.error}
      rightComponent={
        <div className="flex gap-2">
          <ProxyRefreshButton
            isFetching={certsQuery.isFetching || redirsQuery.isFetching}
            onClick={() => {
              void certsQuery.refetch()
              void redirsQuery.refetch()
            }}
          />
          <Button
            size="sm"
            variant="outline"
            onClick={() => applyMutation.mutate()}
            disabled={busy}
          >
            {applyMutation.isPending ? (
              <Loader2 className="size-3.5 animate-spin" />
            ) : null}
            Apply
          </Button>
        </div>
      }
    >
      <ProxySubNav />
      <DirtyBanner
        dirty={settingsQuery.data?.data?.dirty}
        lastError={settingsQuery.data?.data?.last_apply_error}
        onApply={() => applyMutation.mutate()}
        applying={busy}
      />

      <div className="mb-3 flex items-center justify-between">
        <h2 className="text-base font-semibold">Certificates</h2>
        <Button
          size="sm"
          onClick={() => {
            setEditingCert(null)
            setCertForm(emptyCert())
            setCertOpen(true)
          }}
        >
          <Plus className="size-3.5" />
          Add certificate
        </Button>
      </div>
      <DataTable columns={certColumns} data={certs} />

      <Separator className="my-8" />

      <div className="mb-3 flex items-center justify-between">
        <h2 className="text-base font-semibold">Redirects</h2>
        <Button
          size="sm"
          onClick={() => {
            setEditingRedir(null)
            setRedirForm(emptyRedirect())
            setRedirOpen(true)
          }}
        >
          <Plus className="size-3.5" />
          Add redirect
        </Button>
      </div>
      <DataTable columns={redirColumns} data={redirs} />

      <Dialog open={certOpen} onOpenChange={setCertOpen}>
        <DialogContent className="max-w-lg">
          <DialogHeader>
            <DialogTitle>
              {editingCert ? "Edit certificate" : "Add certificate"}
            </DialogTitle>
          </DialogHeader>
          <div className="grid gap-4 py-2">
            <div className="space-y-2">
              <Label>Name</Label>
              <Input
                value={certForm.name}
                onChange={(e) =>
                  setCertForm((f) => ({ ...f, name: e.target.value }))
                }
              />
            </div>
            <div className="space-y-2">
              <Label>Domains</Label>
              <Input
                value={certForm.domains || ""}
                onChange={(e) =>
                  setCertForm((f) => ({ ...f, domains: e.target.value }))
                }
              />
            </div>
            <div className="space-y-2">
              <Label>Source</Label>
              <ReactSelect
                options={SOURCE_OPTIONS}
                value={SOURCE_OPTIONS.find((o) => o.value === certForm.source)}
                onChange={(opt) =>
                  setCertForm((f) => ({
                    ...f,
                    source: opt?.value || "upload",
                  }))
                }
              />
            </div>
            {certForm.source === "upload" && (
              <>
                <div className="space-y-2">
                  <Label>Certificate PEM</Label>
                  <Textarea
                    rows={4}
                    value={certForm.cert_pem || ""}
                    onChange={(e) =>
                      setCertForm((f) => ({ ...f, cert_pem: e.target.value }))
                    }
                  />
                </div>
                <div className="space-y-2">
                  <Label>Private key PEM</Label>
                  <Textarea
                    rows={4}
                    value={certForm.key_pem || ""}
                    onChange={(e) =>
                      setCertForm((f) => ({ ...f, key_pem: e.target.value }))
                    }
                  />
                </div>
              </>
            )}
            {certForm.source === "path" && (
              <>
                <div className="space-y-2">
                  <Label>Cert path</Label>
                  <Input
                    value={certForm.cert_path || ""}
                    onChange={(e) =>
                      setCertForm((f) => ({ ...f, cert_path: e.target.value }))
                    }
                  />
                </div>
                <div className="space-y-2">
                  <Label>Key path</Label>
                  <Input
                    value={certForm.key_path || ""}
                    onChange={(e) =>
                      setCertForm((f) => ({ ...f, key_path: e.target.value }))
                    }
                  />
                </div>
              </>
            )}
            {certForm.source === "letsencrypt" && (
              <div className="space-y-2">
                <Label>Email</Label>
                <Input
                  value={certForm.letsencrypt_email || ""}
                  onChange={(e) =>
                    setCertForm((f) => ({
                      ...f,
                      letsencrypt_email: e.target.value,
                    }))
                  }
                />
                <p className="text-xs text-muted-foreground">
                  Automatic issuance is stubbed in this MVP — store metadata only.
                </p>
              </div>
            )}
            <div className="flex items-center justify-between gap-2 rounded-lg border border-dashed px-3 py-2">
              <Label htmlFor="cert-apply">Apply after save</Label>
              <Switch
                id="cert-apply"
                checked={applyAfter}
                onCheckedChange={setApplyAfter}
              />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setCertOpen(false)}>
              Cancel
            </Button>
            <Button onClick={() => saveCert.mutate()} disabled={busy}>
              {saveCert.isPending ? (
                <Loader2 className="size-3.5 animate-spin" />
              ) : null}
              Save
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={redirOpen} onOpenChange={setRedirOpen}>
        <DialogContent className="max-w-lg">
          <DialogHeader>
            <DialogTitle>
              {editingRedir ? "Edit redirect" : "Add redirect"}
            </DialogTitle>
          </DialogHeader>
          <div className="grid gap-4 py-2">
            <div className="space-y-2">
              <Label>Name</Label>
              <Input
                value={redirForm.name}
                onChange={(e) =>
                  setRedirForm((f) => ({ ...f, name: e.target.value }))
                }
              />
            </div>
            <div className="space-y-2">
              <Label>From host</Label>
              <Input
                value={redirForm.from_host}
                onChange={(e) =>
                  setRedirForm((f) => ({ ...f, from_host: e.target.value }))
                }
              />
            </div>
            <div className="space-y-2">
              <Label>From path</Label>
              <Input
                value={redirForm.from_path || "/"}
                onChange={(e) =>
                  setRedirForm((f) => ({ ...f, from_path: e.target.value }))
                }
              />
            </div>
            <div className="space-y-2">
              <Label>To URL</Label>
              <Input
                value={redirForm.to_url}
                onChange={(e) =>
                  setRedirForm((f) => ({ ...f, to_url: e.target.value }))
                }
              />
            </div>
            <div className="space-y-2">
              <Label>Status code</Label>
              <ReactSelect
                options={CODE_OPTIONS}
                value={CODE_OPTIONS.find(
                  (o) => o.value === String(redirForm.status_code || 301),
                )}
                onChange={(opt) =>
                  setRedirForm((f) => ({
                    ...f,
                    status_code: Number(opt?.value || 301),
                  }))
                }
              />
            </div>
            <div className="flex items-center justify-between">
              <Label>Preserve path</Label>
              <Switch
                checked={!!redirForm.preserve_path}
                onCheckedChange={(v) =>
                  setRedirForm((f) => ({ ...f, preserve_path: v }))
                }
              />
            </div>
            <div className="flex items-center justify-between">
              <Label>Enabled</Label>
              <Switch
                checked={redirForm.enabled !== false}
                onCheckedChange={(v) =>
                  setRedirForm((f) => ({ ...f, enabled: v }))
                }
              />
            </div>
            <div className="flex items-center justify-between gap-2 rounded-lg border border-dashed px-3 py-2">
              <Label htmlFor="redir-apply">Apply after save</Label>
              <Switch
                id="redir-apply"
                checked={applyAfter}
                onCheckedChange={setApplyAfter}
              />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setRedirOpen(false)}>
              Cancel
            </Button>
            <Button
              onClick={() => saveRedir.mutate()}
              disabled={busy}
            >
              {saveRedir.isPending ? (
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
