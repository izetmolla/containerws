import { useMemo, useState, type ReactNode } from "react"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import {
  createColumnHelper,
  type ColumnDef,
  type RowSelectionState,
} from "@tanstack/react-table"
import { MoreHorizontal, Plus, Search } from "lucide-react"
import { toast } from "sonner"

import ContentLoader from "@/components/content-loader"
import { Button } from "@/components/ui/button"
import { ButtonGroup } from "@/components/ui/button-group"
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
import { toastRequestError } from "@/lib/network"
import { cn } from "@/lib/utils"

import { EngineControls } from "../_shared/engine-controls"
import { EngineBanner } from "../_shared/engine-status"
import { DOCKER_ENGINE_KEY } from "../_shared/engine-api"
import {
  DOCKER_PAGE_DESCRIPTIONS,
  DockerRefreshButton,
  SummaryChip,
} from "../_shared/page-chrome"
import { selectColumn } from "../_shared/select-column"
import {
  activateDockerEnvironment,
  createDockerEnvironment,
  deleteDockerEnvironment,
  DOCKER_ENV_KEY,
  listDockerEnvironments,
  setStoredEnvironmentId,
  testDockerEnvironment,
  updateDockerEnvironment,
  type DockerConnType,
  type DockerEnvironment,
  type UpsertDockerEnvironment,
} from "./api"
import { asArray } from "@/lib/as-array"

const columnHelper = createColumnHelper<DockerEnvironment>()

const CONN_OPTIONS: { value: DockerConnType; label: string }[] = [
  { value: "unix", label: "Unix socket (local)" },
  { value: "ssh", label: "SSH tunnel" },
  { value: "tls", label: "TCP + TLS (remote)" },
]

type FormState = UpsertDockerEnvironment

const emptyForm = (): FormState => ({
  name: "",
  description: "",
  conn_type: "unix",
  socket_path: "/var/run/docker.sock",
  ssh_port: 22,
  ssh_remote_socket: "/var/run/docker.sock",
  tcp_port: 2376,
  tls_skip_verify: false,
  is_default: false,
})

export default function DockerEnvironmentsPage() {
  const queryClient = useQueryClient()
  const [search, setSearch] = useState("")
  const [rowSelection, setRowSelection] = useState<RowSelectionState>({})
  const [editorOpen, setEditorOpen] = useState(false)
  const [editing, setEditing] = useState<DockerEnvironment | null>(null)
  const [form, setForm] = useState<FormState>(emptyForm())
  const [removeTarget, setRemoveTarget] = useState<DockerEnvironment | null>(
    null
  )
  const [bulkDeleteIds, setBulkDeleteIds] = useState<string[] | null>(null)

  const listQuery = useQuery({
    queryKey: [DOCKER_ENV_KEY],
    queryFn: listDockerEnvironments,
  })

  const invalidate = () =>
    queryClient.invalidateQueries({ queryKey: [DOCKER_ENV_KEY] })

  const saveMutation = useMutation({
    mutationFn: async () => {
      if (editing) {
        return updateDockerEnvironment(editing.id, form)
      }
      return createDockerEnvironment(form)
    },
    onSuccess: (res) => {
      toast.success(res.message || "Saved")
      setEditorOpen(false)
      setEditing(null)
      setForm(emptyForm())
      invalidate()
    },
    onError: (err) => toastRequestError(err, "Save failed"),
  })

  const activateMutation = useMutation({
    mutationFn: (id: string) => activateDockerEnvironment(id),
    onSuccess: (res) => {
      toast.success(res.message || "Activated")
      if (res.data?.id) setStoredEnvironmentId(res.data.id)
      invalidate()
      void queryClient.invalidateQueries({ queryKey: [DOCKER_ENGINE_KEY] })
      void queryClient.invalidateQueries({ queryKey: ["docker-containers"] })
    },
    onError: (err) => toastRequestError(err, "Activate failed"),
  })

  const toggleDisabledMutation = useMutation({
    mutationFn: ({ id, disabled }: { id: string; disabled: boolean }) =>
      updateDockerEnvironment(id, { is_disabled: disabled }),
    onSuccess: (res, vars) => {
      toast.success(
        res.message || (vars.disabled ? "Environment disabled" : "Environment enabled")
      )
      invalidate()
      void queryClient.invalidateQueries({ queryKey: [DOCKER_ENGINE_KEY] })
    },
    onError: (err) => toastRequestError(err, "Update failed"),
  })

  const testMutation = useMutation({
    mutationFn: (id: string) => testDockerEnvironment(id),
    onSuccess: (res) => {
      if (res.data?.ok) toast.success(res.message || "Connection OK")
      else toast.error(res.data?.error || res.message || "Connection failed")
    },
    onError: (err) => toastRequestError(err, "Test failed"),
  })

  const deleteMutation = useMutation({
    mutationFn: (id: string) => deleteDockerEnvironment(id),
    onSuccess: (res) => {
      toast.success(res.message || "Deleted")
      setRemoveTarget(null)
      invalidate()
    },
    onError: (err) => toastRequestError(err, "Delete failed"),
  })

  const bulkDeleteMutation = useMutation({
    mutationFn: async (ids: string[]) => {
      const results = await Promise.allSettled(
        ids.map((id) => deleteDockerEnvironment(id))
      )
      const failed = results.filter((r) => r.status === "rejected").length
      return { ok: results.length - failed, failed, total: results.length }
    },
    onSuccess: (res) => {
      if (res.failed === 0) toast.success(`Deleted ${res.ok} environment(s)`)
      else toast.warning(`Deleted ${res.ok}, ${res.failed} failed of ${res.total}`)
      setBulkDeleteIds(null)
      setRowSelection({})
      invalidate()
    },
    onError: (err) => toastRequestError(err, "Bulk delete failed"),
  })

  const rows = asArray(listQuery.data?.data)
  const filtered = useMemo(() => {
    const list = rows
    const q = search.trim().toLowerCase()
    if (!q) return list
    return list.filter(
      (r) =>
        r.name.toLowerCase().includes(q) ||
        r.conn_type.includes(q) ||
        (r.host_url || "").toLowerCase().includes(q)
    )
  }, [rows, search])

  const selectedIds = useMemo(
    () => Object.keys(rowSelection).filter((id) => rowSelection[id]),
    [rowSelection]
  )
  const hasSelection = selectedIds.length > 0
  const busy = deleteMutation.isPending || bulkDeleteMutation.isPending

  const openCreate = () => {
    setEditing(null)
    setForm(emptyForm())
    setEditorOpen(true)
  }

  const openEdit = (row: DockerEnvironment) => {
    setEditing(row)
    setForm({
      name: row.name,
      description: row.description || "",
      conn_type: row.conn_type,
      socket_path: row.socket_path || "/var/run/docker.sock",
      ssh_host: row.ssh_host || "",
      ssh_port: row.ssh_port || 22,
      ssh_user: row.ssh_user || "",
      ssh_remote_socket: row.ssh_remote_socket || "/var/run/docker.sock",
      ssh_private_key: "",
      ssh_passphrase: "",
      tcp_host: row.tcp_host || "",
      tcp_port: row.tcp_port || 2376,
      tls_ca_cert: "",
      tls_cert: "",
      tls_key: "",
      tls_skip_verify: row.tls_skip_verify,
      is_default: row.is_default,
      is_disabled: row.is_disabled,
    })
    setEditorOpen(true)
  }

  const columns = useMemo(
    () =>
      [
        selectColumn(columnHelper, "environment"),
        columnHelper.accessor("name", {
          header: "Name",
          enableSorting: true,
          cell: ({ row }) => (
            <div className="flex max-w-[16rem] flex-col gap-0.5">
              <span title={row.original.name} className="truncate font-medium">
                {row.original.name}
              </span>
              {row.original.description ? (
                <span
                  title={row.original.description}
                  className="truncate text-xs text-muted-foreground"
                >
                  {row.original.description}
                </span>
              ) : null}
            </div>
          ),
          meta: { className: "max-w-[16rem]" },
        }),
        columnHelper.accessor("conn_type", {
          header: "Type",
          enableSorting: true,
          cell: ({ getValue }) => (
            <span className="rounded-full bg-muted px-2 py-0.5 font-mono text-xs uppercase">
              {getValue()}
            </span>
          ),
        }),
        columnHelper.accessor("host_url", {
          header: "Host",
          enableSorting: true,
          cell: ({ getValue }) => {
            const text = getValue() || "—"
            return (
              <span
                title={text}
                className="block max-w-[16rem] truncate font-mono text-xs text-muted-foreground"
              >
                {text}
              </span>
            )
          },
          meta: { className: "max-w-[16rem]" },
        }),
        columnHelper.display({
          id: "flags",
          enableSorting: false,
          header: "Status",
          cell: ({ row }) => (
            <div className="flex flex-wrap gap-1">
              {row.original.is_default ? (
                <span className="rounded-full bg-emerald-500/15 px-2 py-0.5 text-[11px] font-medium text-emerald-700 dark:text-emerald-300">
                  default
                </span>
              ) : null}
              {row.original.is_disabled ? (
                <span className="rounded-full bg-amber-500/15 px-2 py-0.5 text-[11px] font-medium text-amber-800 dark:text-amber-300">
                  disabled
                </span>
              ) : null}
            </div>
          ),
        }),
        columnHelper.display({
          id: "actions",
          enableSorting: false,
          cell: ({ row }) => (
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button variant="ghost" size="icon-sm">
                  <MoreHorizontal className="size-4" />
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end">
                <DropdownMenuItem onClick={() => openEdit(row.original)}>
                  Edit
                </DropdownMenuItem>
                <DropdownMenuItem
                  disabled={row.original.is_disabled}
                  onClick={() => activateMutation.mutate(row.original.id)}
                >
                  Set as default
                </DropdownMenuItem>
                <DropdownMenuItem
                  onClick={() => testMutation.mutate(row.original.id)}
                >
                  Test connection
                </DropdownMenuItem>
                <DropdownMenuItem
                  disabled={toggleDisabledMutation.isPending}
                  onClick={() =>
                    toggleDisabledMutation.mutate({
                      id: row.original.id,
                      disabled: !row.original.is_disabled,
                    })
                  }
                >
                  {row.original.is_disabled ? "Enable" : "Disable"}
                </DropdownMenuItem>
                <DropdownMenuSeparator />
                <DropdownMenuItem
                  className="text-destructive"
                  onClick={() => setRemoveTarget(row.original)}
                >
                  Delete
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          ),
          meta: { width: 48, className: "w-12" },
        }),
      ] as ColumnDef<DockerEnvironment, unknown>[],
    [activateMutation, testMutation, toggleDisabledMutation]
  )

  const patch = <K extends keyof FormState>(key: K, value: FormState[K]) =>
    setForm((prev) => ({ ...prev, [key]: value }))

  return (
    <ContentLoader
      title="Environments"
      description={DOCKER_PAGE_DESCRIPTIONS.environments}
      showHeaderSeparator
      breadcrumb={[
        { label: "Docker", to: "/docker" },
        { label: "Environments" },
      ]}
      isLoading={listQuery.isLoading}
      error={listQuery.error}
      rightComponent={
        <div className="flex gap-2">
          <DockerRefreshButton
            onClick={() => invalidate()}
            isFetching={listQuery.isFetching}
          />
        </div>
      }
    >
      <div className="flex flex-col gap-4">
        <EngineBanner showControls={false} />
        <EngineControls />
        <DataTable
          columns={columns}
          data={filtered}
          dense
          enableRowSelection
          enableSorting
          enablePagination
          pageSize={10}
          paginationResetKey={search}
          rowSelection={rowSelection}
          onRowSelectionChange={setRowSelection}
          getRowId={(row) => row.id}
          emptyMessage="No environments found. Add a local socket or remote Engine endpoint."
          toolbarStart={
            <div className="flex flex-wrap items-center gap-3">
              <div className="relative w-full max-w-xs min-w-[12rem]">
                <Search className="pointer-events-none absolute top-1/2 left-2.5 size-3.5 -translate-y-1/2 text-muted-foreground" />
                <Input
                  className="h-8 pl-8"
                  placeholder="Search environments…"
                  value={search}
                  onChange={(e) => setSearch(e.target.value)}
                />
              </div>
              <SummaryChip>{filtered.length} environments</SummaryChip>
              {hasSelection ? (
                <span className="text-xs text-muted-foreground">
                  {selectedIds.length} selected
                </span>
              ) : null}
            </div>
          }
          toolbar={
            <div className="flex flex-wrap items-center gap-2">
              <ButtonGroup aria-label="Environment actions">
                <Button
                  size="sm"
                  variant="destructive"
                  disabled={!hasSelection || busy}
                  onClick={() => setBulkDeleteIds(selectedIds)}
                >
                  Delete
                </Button>
              </ButtonGroup>
              <Button size="sm" onClick={openCreate}>
                <Plus data-icon="inline-start" />
                Add environment
              </Button>
            </div>
          }
        />
      </div>

      <Dialog
        open={editorOpen}
        onOpenChange={(v) => {
          if (!v) {
            setEditorOpen(false)
            setEditing(null)
          }
        }}
      >
        <DialogContent className="flex max-h-[90vh] flex-col gap-0 overflow-hidden p-0 sm:max-w-lg">
          <DialogHeader className="border-b px-6 py-4">
            <DialogTitle>
              {editing ? "Edit environment" : "Add environment"}
            </DialogTitle>
            <DialogDescription>
              Choose how this workspace reaches a Docker Engine API.
            </DialogDescription>
          </DialogHeader>
          <div className="min-h-0 flex-1 space-y-4 overflow-y-auto px-6 py-4">
            <Field label="Name *">
              <Input
                value={form.name}
                onChange={(e) => patch("name", e.target.value)}
                placeholder="Production"
              />
            </Field>
            <Field label="Description">
              <Input
                value={form.description || ""}
                onChange={(e) => patch("description", e.target.value)}
              />
            </Field>
            <Field label="Connection type">
              <ReactSelect<{ value: DockerConnType; label: string }, false>
                size="sm"
                options={CONN_OPTIONS}
                value={form.conn_type}
                onValueChange={(v) => v && patch("conn_type", v)}
              />
            </Field>

            {form.conn_type === "unix" ? (
              <Field
                label="Socket path"
                hint="Default /var/run/docker.sock (or Docker Desktop socket)"
              >
                <Input
                  value={form.socket_path || ""}
                  onChange={(e) => patch("socket_path", e.target.value)}
                  className="font-mono text-sm"
                />
              </Field>
            ) : null}

            {form.conn_type === "ssh" ? (
              <>
                <div className="grid gap-3 sm:grid-cols-2">
                  <Field label="SSH host *">
                    <Input
                      value={form.ssh_host || ""}
                      onChange={(e) => patch("ssh_host", e.target.value)}
                    />
                  </Field>
                  <Field label="Port">
                    <Input
                      type="number"
                      value={form.ssh_port || 22}
                      onChange={(e) =>
                        patch("ssh_port", Number(e.target.value) || 22)
                      }
                    />
                  </Field>
                </div>
                <Field label="User">
                  <Input
                    value={form.ssh_user || ""}
                    onChange={(e) => patch("ssh_user", e.target.value)}
                    placeholder="root"
                  />
                </Field>
                <Field
                  label="Private key (PEM)"
                  hint={
                    editing?.has_ssh_key
                      ? "Leave blank to keep the existing key"
                      : "Required for SSH auth"
                  }
                >
                  <textarea
                    className="min-h-28 w-full rounded-md border bg-background px-3 py-2 font-mono text-xs"
                    value={form.ssh_private_key || ""}
                    onChange={(e) => patch("ssh_private_key", e.target.value)}
                    placeholder="-----BEGIN OPENSSH PRIVATE KEY-----"
                  />
                </Field>
                <Field label="Key passphrase">
                  <Input
                    type="password"
                    value={form.ssh_passphrase || ""}
                    onChange={(e) => patch("ssh_passphrase", e.target.value)}
                  />
                </Field>
                <Field label="Remote Docker socket">
                  <Input
                    value={form.ssh_remote_socket || ""}
                    onChange={(e) => patch("ssh_remote_socket", e.target.value)}
                    className="font-mono text-sm"
                  />
                </Field>
              </>
            ) : null}

            {form.conn_type === "tls" ? (
              <>
                <div className="grid gap-3 sm:grid-cols-2">
                  <Field label="TCP host *">
                    <Input
                      value={form.tcp_host || ""}
                      onChange={(e) => patch("tcp_host", e.target.value)}
                      placeholder="docker.example.com"
                    />
                  </Field>
                  <Field label="Port" hint="2376 with TLS">
                    <Input
                      type="number"
                      value={form.tcp_port || 2376}
                      onChange={(e) =>
                        patch("tcp_port", Number(e.target.value) || 2376)
                      }
                    />
                  </Field>
                </div>
                <Field label="CA certificate (ca.pem)">
                  <textarea
                    className="min-h-20 w-full rounded-md border bg-background px-3 py-2 font-mono text-xs"
                    value={form.tls_ca_cert || ""}
                    onChange={(e) => patch("tls_ca_cert", e.target.value)}
                    placeholder={
                      editing?.has_tls_ca ? "Leave blank to keep" : ""
                    }
                  />
                </Field>
                <Field label="Client certificate (cert.pem)">
                  <textarea
                    className="min-h-20 w-full rounded-md border bg-background px-3 py-2 font-mono text-xs"
                    value={form.tls_cert || ""}
                    onChange={(e) => patch("tls_cert", e.target.value)}
                  />
                </Field>
                <Field label="Client key (key.pem)">
                  <textarea
                    className="min-h-20 w-full rounded-md border bg-background px-3 py-2 font-mono text-xs"
                    value={form.tls_key || ""}
                    onChange={(e) => patch("tls_key", e.target.value)}
                  />
                </Field>
                <label className="flex items-center gap-2 text-sm">
                  <input
                    type="checkbox"
                    className="size-4 accent-foreground"
                    checked={Boolean(form.tls_skip_verify)}
                    onChange={(e) => patch("tls_skip_verify", e.target.checked)}
                  />
                  Skip TLS verify (insecure)
                </label>
              </>
            ) : null}

            <label className="flex items-center gap-2 text-sm">
              <input
                type="checkbox"
                className="size-4 accent-foreground"
                checked={Boolean(form.is_default)}
                onChange={(e) => patch("is_default", e.target.checked)}
              />
              Set as default environment
            </label>
            <label className="flex items-center gap-2 text-sm">
              <input
                type="checkbox"
                className="size-4 accent-foreground"
                checked={Boolean(form.is_disabled)}
                onChange={(e) => patch("is_disabled", e.target.checked)}
              />
              Disabled
            </label>
          </div>
          <DialogFooter className="border-t px-6 py-4">
            <Button variant="outline" onClick={() => setEditorOpen(false)}>
              Cancel
            </Button>
            <Button
              disabled={!form.name.trim() || saveMutation.isPending}
              onClick={() => saveMutation.mutate()}
            >
              {saveMutation.isPending ? "Saving…" : "Save"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog
        open={Boolean(removeTarget)}
        onOpenChange={(v) => !v && setRemoveTarget(null)}
      >
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>Delete environment</DialogTitle>
            <DialogDescription>
              Remove {removeTarget?.name}? Resource pages will fall back to
              another default if needed.
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setRemoveTarget(null)}>
              Cancel
            </Button>
            <Button
              variant="destructive"
              disabled={deleteMutation.isPending}
              onClick={() =>
                removeTarget && deleteMutation.mutate(removeTarget.id)
              }
            >
              Delete
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog
        open={Boolean(bulkDeleteIds?.length)}
        onOpenChange={(v) => !v && setBulkDeleteIds(null)}
      >
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>Delete environments</DialogTitle>
            <DialogDescription>
              Delete {bulkDeleteIds?.length} selected environment(s)? Resource
              pages will fall back to another default if needed.
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setBulkDeleteIds(null)}>
              Cancel
            </Button>
            <Button
              variant="destructive"
              disabled={bulkDeleteMutation.isPending}
              onClick={() =>
                bulkDeleteIds && bulkDeleteMutation.mutate(bulkDeleteIds)
              }
            >
              {bulkDeleteMutation.isPending ? "Deleting…" : "Delete"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </ContentLoader>
  )
}

function Field({
  label,
  hint,
  children,
}: {
  label: string
  hint?: string
  children: ReactNode
}) {
  return (
    <div className={cn("grid gap-1.5")}>
      <Label className="text-sm">{label}</Label>
      {children}
      {hint ? <p className="text-xs text-muted-foreground">{hint}</p> : null}
    </div>
  )
}
