import { useMemo, useState } from "react"
import { Link } from "react-router"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import {
  createColumnHelper,
  type ColumnDef,
} from "@tanstack/react-table"
import {
  Cloud,
  MoreHorizontal,
  Pencil,
  Plus,
  Search,
  Trash2,
} from "lucide-react"
import { toast } from "sonner"

import ContentLoader from "@/components/content-loader"
import { Button } from "@/components/ui/button"
import { DataTable } from "@/components/ui/data-table"
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
import { getRequestErrorMessage, withError } from "@/lib/network"
import { cn } from "@/lib/utils"

import {
  createRegistry,
  deleteRegistry,
  getRegistriesList,
  getRemotePackages,
  REMOTEPKG_FETCH_KEY,
  updateRegistry,
  type RegistryPayload,
  type RemotePackageItem,
  type SoftwarePackageRegistry,
} from "./api"
import { SoftwareGlyph } from "../../components/software-glyph"

const registryHelper = createColumnHelper<SoftwarePackageRegistry>()
const packageHelper = createColumnHelper<RemotePackageItem>()

type DialogMode = "create" | "edit" | null

function formatDate(value?: string) {
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

const emptyForm = {
  package_url: "",
  username: "",
  token: "",
  password: "",
  clear_token: false,
  clear_password: false,
}

export default function SoftwaresRemotePkgPage() {
  const queryClient = useQueryClient()
  const [registrySearch, setRegistrySearch] = useState("")
  const [packageSearch, setPackageSearch] = useState("")
  const [dialogMode, setDialogMode] = useState<DialogMode>(null)
  const [editing, setEditing] = useState<SoftwarePackageRegistry | null>(null)
  const [form, setForm] = useState(emptyForm)
  const [deleteTarget, setDeleteTarget] =
    useState<SoftwarePackageRegistry | null>(null)

  const registriesQuery = useQuery({
    queryKey: [REMOTEPKG_FETCH_KEY, "list", registrySearch],
    queryFn: () => getRegistriesList(registrySearch),
  })

  const packagesQuery = useQuery({
    queryKey: [REMOTEPKG_FETCH_KEY, "packages", packageSearch],
    queryFn: () => getRemotePackages({ q: packageSearch }),
  })

  const invalidate = () => {
    void queryClient.invalidateQueries({ queryKey: [REMOTEPKG_FETCH_KEY] })
  }

  const saveMutation = useMutation({
    mutationFn: async () => {
      const payload: RegistryPayload = {
        package_url: form.package_url.trim(),
        username: form.username.trim() || undefined,
      }
      if (form.token.trim()) payload.token = form.token.trim()
      if (form.password) payload.password = form.password
      if (dialogMode === "edit") {
        if (form.clear_token) payload.clear_token = true
        if (form.clear_password) payload.clear_password = true
        if (!editing?.id) throw new Error("Missing registry id")
        return updateRegistry(editing.id, payload)
      }
      return createRegistry(payload)
    },
    onSuccess: (res) => {
      toast.success(
        res.message ||
          (dialogMode === "edit" ? "Registry updated" : "Registry created")
      )
      closeDialog()
      invalidate()
    },
    onError: (err) =>
      toast.error(getRequestErrorMessage(err, "Failed to save registry")),
  })

  const deleteMutation = useMutation({
    mutationFn: (row: SoftwarePackageRegistry) => deleteRegistry(row.id),
    onSuccess: (res) => {
      toast.success(res.message || "Registry deleted")
      setDeleteTarget(null)
      invalidate()
    },
    onError: (err) =>
      toast.error(getRequestErrorMessage(err, "Failed to delete registry")),
  })

  const openCreate = () => {
    setEditing(null)
    setForm(emptyForm)
    setDialogMode("create")
  }

  const openEdit = (row: SoftwarePackageRegistry) => {
    setEditing(row)
    setForm({
      package_url: row.package_url || "",
      username: row.username || "",
      token: "",
      password: "",
      clear_token: false,
      clear_password: false,
    })
    setDialogMode("edit")
  }

  const closeDialog = () => {
    setDialogMode(null)
    setEditing(null)
    setForm(emptyForm)
  }

  const registries = registriesQuery.data?.data ?? []
  const remotePackages = packagesQuery.data?.data ?? []

  const registryColumns = useMemo(
    () =>
      [
        registryHelper.accessor("package_url", {
          header: "Registry",
          cell: ({ row }) => {
            const item = row.original
            return (
              <div className="min-w-0 max-w-md space-y-1">
                <div className="flex flex-wrap items-center gap-1.5">
                  <a
                    href={item.package_url}
                    target="_blank"
                    rel="noreferrer"
                    className="truncate text-sm font-medium hover:underline"
                  >
                    {item.package_url}
                  </a>
                  {item.is_default ? (
                    <span className="rounded border border-sky-500/25 bg-sky-500/10 px-1.5 py-0.5 text-[10px] font-medium uppercase tracking-wide text-sky-700 dark:text-sky-300">
                      Default
                    </span>
                  ) : null}
                </div>
                <p className="truncate font-mono text-[11px] text-muted-foreground">
                  {item.id}
                </p>
              </div>
            )
          },
        }),
        registryHelper.display({
          id: "auth",
          header: "Auth",
          cell: ({ row }) => {
            const item = row.original
            return (
              <div className="space-y-0.5 text-xs text-muted-foreground">
                <div>{item.username?.trim() || "—"}</div>
                <div className="flex flex-wrap gap-1.5">
                  <span
                    className={cn(
                      "rounded px-1.5 py-0.5 text-[10px] font-medium uppercase",
                      item.has_token
                        ? "bg-emerald-500/15 text-emerald-700 dark:text-emerald-300"
                        : "bg-muted text-muted-foreground"
                    )}
                  >
                    {item.has_token ? "Token" : "No token"}
                  </span>
                  <span
                    className={cn(
                      "rounded px-1.5 py-0.5 text-[10px] font-medium uppercase",
                      item.has_password
                        ? "bg-emerald-500/15 text-emerald-700 dark:text-emerald-300"
                        : "bg-muted text-muted-foreground"
                    )}
                  >
                    {item.has_password ? "Password" : "No password"}
                  </span>
                </div>
              </div>
            )
          },
        }),
        registryHelper.accessor("remote_count", {
          header: "Packages",
          cell: ({ row }) => {
            const item = row.original
            if (item.catalog_error) {
              return (
                <span
                  className="text-xs text-amber-700 dark:text-amber-300"
                  title={item.catalog_error}
                >
                  Error
                </span>
              )
            }
            return (
              <span className="font-mono text-xs tabular-nums">
                {item.remote_count ?? 0}
              </span>
            )
          },
        }),
        registryHelper.accessor("updated_at", {
          header: "Updated",
          cell: ({ getValue }) => (
            <span className="text-xs text-muted-foreground">
              {formatDate(getValue())}
            </span>
          ),
        }),
        registryHelper.display({
          id: "actions",
          header: "",
          meta: { className: "w-12 !px-1.5", width: 48 },
          cell: ({ row }) => {
            const item = row.original
            return (
              <DropdownMenu>
                <DropdownMenuTrigger asChild>
                  <Button
                    variant="ghost"
                    size="icon-sm"
                    className="size-8"
                    aria-label="Registry actions"
                  >
                    <MoreHorizontal className="size-4" />
                  </Button>
                </DropdownMenuTrigger>
                <DropdownMenuContent align="end" className="min-w-40">
                  <DropdownMenuItem onClick={() => openEdit(item)}>
                    <Pencil className="size-3.5" />
                    Edit
                  </DropdownMenuItem>
                  <DropdownMenuSeparator />
                  <DropdownMenuItem
                    variant="destructive"
                    onClick={() => setDeleteTarget(item)}
                  >
                    <Trash2 className="size-3.5" />
                    Delete
                  </DropdownMenuItem>
                </DropdownMenuContent>
              </DropdownMenu>
            )
          },
        }),
      ] as ColumnDef<SoftwarePackageRegistry, unknown>[],
    []
  )

  const packageColumns = useMemo(
    () =>
      [
        packageHelper.accessor("name", {
          header: "Package",
          cell: ({ row }) => {
            const item = row.original
            const accent = item.color || "var(--primary)"
            return (
              <div className="flex min-w-0 max-w-sm items-center gap-2.5">
                {item.image?.trim() ? (
                  <div className="flex size-8 shrink-0 items-center justify-center overflow-hidden rounded border border-border/60 bg-background">
                    <SoftwareGlyph
                      name={item.icon}
                      image={item.image}
                      className="h-4 w-4"
                      imgClassName="size-8 object-cover"
                    />
                  </div>
                ) : (
                  <div
                    className="flex size-8 shrink-0 items-center justify-center rounded text-white"
                    style={{ backgroundColor: accent }}
                  >
                    <SoftwareGlyph name={item.icon} className="h-4 w-4" />
                  </div>
                )}
                <div className="min-w-0">
                  <div className="truncate text-sm font-medium">{item.name}</div>
                  {item.details ? (
                    <p className="truncate text-[11px] text-muted-foreground">
                      {item.details}
                    </p>
                  ) : null}
                </div>
              </div>
            )
          },
        }),
        packageHelper.accessor("category", {
          header: "Category",
          cell: ({ row }) => {
            const item = row.original
            if (!item.category) {
              return <span className="text-xs text-muted-foreground">—</span>
            }
            return (
              <div className="min-w-0 max-w-36">
                <div className="truncate text-xs">{item.category}</div>
                {item.sub_category ? (
                  <div className="truncate text-[11px] text-muted-foreground">
                    {item.sub_category}
                  </div>
                ) : null}
              </div>
            )
          },
        }),
        packageHelper.accessor("package_url", {
          header: "Registry",
          cell: ({ getValue }) => (
            <span className="max-w-xs truncate text-xs text-muted-foreground">
              {getValue()}
            </span>
          ),
        }),
      ] as ColumnDef<RemotePackageItem, unknown>[],
    []
  )

  return (
    <ContentLoader
      title="Remote packages"
      breadcrumb={[
        { label: "Softwares", to: "/softwares" },
        { label: "Remote packages", to: "/softwares/remotepkg" },
      ]}
      isLoading={registriesQuery.isLoading}
      error={withError(registriesQuery.error, registriesQuery.data)}
      showHeaderSeparator
      headerClassName="gap-4 pb-6"
      customTitle={
        <div className="relative overflow-hidden rounded-2xl border border-border/70 bg-gradient-to-br from-background via-background to-sky-500/5 px-5 py-5 shadow-sm dark:to-sky-500/10">
          <div className="relative flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
            <div className="flex min-w-0 items-start gap-3.5">
              <div className="flex h-12 w-12 shrink-0 items-center justify-center rounded-xl bg-sky-600 text-white shadow-md shadow-sky-600/25">
                <Cloud className="h-6 w-6" />
              </div>
              <div className="min-w-0 space-y-1">
                <h1 className="text-2xl font-semibold tracking-tight">
                  Remote packages
                </h1>
                <p className="max-w-xl text-sm text-muted-foreground">
                  Manage GitHub package registries and browse remote software
                  from their catalogs.
                </p>
              </div>
            </div>
            <div className="flex flex-wrap items-center gap-2">
              <Button asChild size="sm" variant="outline">
                <Link to="/softwares">Catalog</Link>
              </Button>
              <Button size="sm" onClick={openCreate}>
                <Plus className="mr-1.5 h-3.5 w-3.5" />
                Add registry
              </Button>
            </div>
          </div>
        </div>
      }
    >
      <div className="space-y-8">
        <section className="space-y-3">
          <div className="flex items-end justify-between gap-3">
            <div>
              <h2 className="text-base font-semibold tracking-tight">
                Registries
              </h2>
              <p className="text-sm text-muted-foreground">
                SoftwarePackage rows used for import, publish, and catalog merge.
              </p>
            </div>
          </div>
          <DataTable
            dense
            columns={registryColumns}
            data={registries}
            emptyMessage="No registries configured."
            toolbarStart={
              <div className="relative max-w-sm flex-1">
                <Search className="pointer-events-none absolute left-2.5 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                <Input
                  value={registrySearch}
                  onChange={(e) => setRegistrySearch(e.target.value)}
                  placeholder="Search registries…"
                  className="pl-8"
                />
              </div>
            }
            toolbar={
              <Button size="sm" variant="outline" onClick={openCreate}>
                <Plus className="mr-1.5 h-3.5 w-3.5" />
                Add
              </Button>
            }
          />
        </section>

        <section className="space-y-3">
          <div>
            <h2 className="text-base font-semibold tracking-tight">
              Remote packages
            </h2>
            <p className="text-sm text-muted-foreground">
              Packages listed in softwares/index.json across configured
              registries.
            </p>
          </div>
          <DataTable
            dense
            columns={packageColumns}
            data={remotePackages}
            emptyMessage={
              packagesQuery.isLoading
                ? "Loading remote packages…"
                : "No remote packages found (empty index or registry unreachable)."
            }
            toolbarStart={
              <div className="relative max-w-sm flex-1">
                <Search className="pointer-events-none absolute left-2.5 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                <Input
                  value={packageSearch}
                  onChange={(e) => setPackageSearch(e.target.value)}
                  placeholder="Search remote packages…"
                  className="pl-8"
                />
              </div>
            }
          />
        </section>
      </div>

      <Dialog
        open={dialogMode !== null}
        onOpenChange={(open) => {
          if (!open) closeDialog()
        }}
      >
        <DialogContent className="sm:max-w-lg">
          <DialogHeader>
            <DialogTitle>
              {dialogMode === "edit" ? "Edit registry" : "Add registry"}
            </DialogTitle>
            <DialogDescription>
              Point at a GitHub package repo (e.g.
              https://github.com/izetmolla/containerwspkg). Token is optional for
              public repos; required for publish.
            </DialogDescription>
          </DialogHeader>
          <div className="grid gap-4 py-2">
            <div className="grid gap-2">
              <Label htmlFor="package_url">Package URL</Label>
              <Input
                id="package_url"
                value={form.package_url}
                onChange={(e) =>
                  setForm((f) => ({ ...f, package_url: e.target.value }))
                }
                placeholder="https://github.com/owner/repo"
                autoFocus
              />
            </div>
            <div className="grid gap-2">
              <Label htmlFor="username">Username</Label>
              <Input
                id="username"
                value={form.username}
                onChange={(e) =>
                  setForm((f) => ({ ...f, username: e.target.value }))
                }
                placeholder="Optional"
                autoComplete="off"
              />
            </div>
            <div className="grid gap-2">
              <Label htmlFor="token">
                Token
                {dialogMode === "edit" && editing?.has_token
                  ? " (leave blank to keep)"
                  : ""}
              </Label>
              <Input
                id="token"
                type="password"
                value={form.token}
                onChange={(e) =>
                  setForm((f) => ({
                    ...f,
                    token: e.target.value,
                    clear_token: false,
                  }))
                }
                placeholder="GitHub PAT / access token"
                autoComplete="new-password"
              />
              {dialogMode === "edit" && editing?.has_token ? (
                <label className="flex items-center gap-2 text-xs text-muted-foreground">
                  <input
                    type="checkbox"
                    checked={form.clear_token}
                    onChange={(e) =>
                      setForm((f) => ({
                        ...f,
                        clear_token: e.target.checked,
                        token: e.target.checked ? "" : f.token,
                      }))
                    }
                  />
                  Clear stored token
                </label>
              ) : null}
            </div>
            <div className="grid gap-2">
              <Label htmlFor="password">
                Password
                {dialogMode === "edit" && editing?.has_password
                  ? " (leave blank to keep)"
                  : ""}
              </Label>
              <Input
                id="password"
                type="password"
                value={form.password}
                onChange={(e) =>
                  setForm((f) => ({
                    ...f,
                    password: e.target.value,
                    clear_password: false,
                  }))
                }
                placeholder="Optional"
                autoComplete="new-password"
              />
              {dialogMode === "edit" && editing?.has_password ? (
                <label className="flex items-center gap-2 text-xs text-muted-foreground">
                  <input
                    type="checkbox"
                    checked={form.clear_password}
                    onChange={(e) =>
                      setForm((f) => ({
                        ...f,
                        clear_password: e.target.checked,
                        password: e.target.checked ? "" : f.password,
                      }))
                    }
                  />
                  Clear stored password
                </label>
              ) : null}
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={closeDialog}>
              Cancel
            </Button>
            <Button
              disabled={
                !form.package_url.trim() || saveMutation.isPending
              }
              onClick={() => saveMutation.mutate()}
            >
              {saveMutation.isPending
                ? "Saving…"
                : dialogMode === "edit"
                  ? "Save changes"
                  : "Create"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <AlertDialog
        open={Boolean(deleteTarget)}
        onOpenChange={(open) => {
          if (!open) setDeleteTarget(null)
        }}
      >
        <AlertDialogContent size="default">
          <AlertDialogHeader>
            <AlertDialogTitle>Delete registry?</AlertDialogTitle>
            <AlertDialogDescription>
              This removes{" "}
              <span className="font-medium text-foreground">
                {deleteTarget?.package_url}
              </span>{" "}
              from the local database. The GitHub repo is not deleted. The
              default registry may be re-seeded automatically.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction
              variant="destructive"
              disabled={deleteMutation.isPending}
              onClick={() => {
                if (deleteTarget) deleteMutation.mutate(deleteTarget)
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
