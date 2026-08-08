import { useMemo, useState, type ReactNode } from "react"
import { Link, useParams } from "react-router"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import {
  ArrowLeft,
  Loader2,
  Plus,
  Save,
  Star,
  Trash2,
} from "lucide-react"
import { toast } from "sonner"

import ContentLoader from "@/components/content-loader"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { getRequestErrorMessage, withError } from "@/lib/network"
import { cn } from "@/lib/utils"

import { SoftwareGlyph } from "../../components/software-glyph"
import { SOFTWARES_FETCH_KEY } from "../list/api"
import {
  createSoftwareVersion,
  deleteSoftwareVersion,
  getSoftwarePackage,
  PACKAGE_FETCH_KEY,
  updateSoftwarePackage,
  updateSoftwareVersion,
  type PackageVersion,
  type UpdateSoftwarePayload,
  type VersionPayload,
} from "./api"
import { BashScriptEditor } from "./components/bash-script-editor"

const EMPTY_PACKAGE_VERSIONS: PackageVersion[] = []

type SoftwareFormState = {
  name: string
  details: string
  category: string
  sub_category: string
  tags: string
  service_units: string
  can_control: boolean
  control_backend: string
  start_command: string
  restart_command: string
  stop_command: string
  icon: string
  image: string
  color: string
  order: string
  is_active: boolean
}

type VersionFormState = {
  version: string
  is_latest: boolean
  install_script: string
  uninstall_script: string
  upgrade_script: string
  custom_script: string
  os: string
  os_version: string
  distro: string
  distro_id: string
  distro_version: string
  arch: string
  platform: string
  package_family: string
  kernel: string
  virtualization: string
  container_runtime: string
  cloud_provider: string
}

const emptyVersionForm = (): VersionFormState => ({
  version: "",
  is_latest: false,
  install_script: "#!/usr/bin/env bash\nset -euo pipefail\n\n",
  uninstall_script: "#!/usr/bin/env bash\nset -euo pipefail\n\n",
  upgrade_script: "",
  custom_script: "",
  os: "linux",
  os_version: "",
  distro: "",
  distro_id: "",
  distro_version: "",
  arch: "",
  platform: "",
  package_family: "apt",
  kernel: "",
  virtualization: "",
  container_runtime: "",
  cloud_provider: "",
})

function versionToForm(v: PackageVersion): VersionFormState {
  return {
    version: v.version || "",
    is_latest: Boolean(v.is_latest),
    install_script: v.install_script || "",
    uninstall_script: v.uninstall_script || "",
    upgrade_script: v.upgrade_script || "",
    custom_script: v.custom_script || "",
    os: v.os || "",
    os_version: v.os_version || "",
    distro: v.distro || "",
    distro_id: v.distro_id || "",
    distro_version: v.distro_version || "",
    arch: v.arch || "",
    platform: v.platform || "",
    package_family: v.package_family || "",
    kernel: v.kernel || "",
    virtualization: v.virtualization || "",
    container_runtime: v.container_runtime || "",
    cloud_provider: v.cloud_provider || "",
  }
}

function formToVersionPayload(form: VersionFormState): VersionPayload {
  return {
    version: form.version.trim(),
    is_latest: form.is_latest,
    install_script: form.install_script,
    uninstall_script: form.uninstall_script,
    upgrade_script: form.upgrade_script,
    custom_script: form.custom_script,
    os: form.os.trim(),
    os_version: form.os_version.trim(),
    distro: form.distro.trim(),
    distro_id: form.distro_id.trim(),
    distro_version: form.distro_version.trim(),
    arch: form.arch.trim(),
    platform: form.platform.trim(),
    package_family: form.package_family.trim(),
    kernel: form.kernel.trim(),
    virtualization: form.virtualization.trim(),
    container_runtime: form.container_runtime.trim(),
    cloud_provider: form.cloud_provider.trim(),
  }
}

function splitCSV(raw: string): string[] {
  return raw
    .split(",")
    .map((s) => s.trim())
    .filter(Boolean)
}

export default function SoftwaresPackagePage() {
  const { id = "" } = useParams()
  const queryClient = useQueryClient()
  const [softwareForm, setSoftwareForm] = useState<SoftwareFormState | null>(
    null
  )
  const [selectedVersionId, setSelectedVersionId] = useState<string | "new">(
    "new"
  )
  const [versionForm, setVersionForm] = useState<VersionFormState>(
    emptyVersionForm
  )
  const [scriptTab, setScriptTab] = useState("install")

  const packageQuery = useQuery({
    queryKey: [PACKAGE_FETCH_KEY, id],
    queryFn: () => getSoftwarePackage(id),
    enabled: Boolean(id),
  })

  const software = packageQuery.data?.data?.software
  // Stable empty fallback — a fresh `[]` each render would infinite-loop the
  // render-time prevVersions sync below while the query is still loading.
  const versions = packageQuery.data?.data?.versions ?? EMPTY_PACKAGE_VERSIONS

  const [prevSoftware, setPrevSoftware] = useState(software)
  if (software !== prevSoftware) {
    setPrevSoftware(software)
    if (software) {
      setSoftwareForm({
        name: software.name || "",
        details: software.details || "",
        category: software.category || "",
        sub_category: software.sub_category || "",
        tags: (software.tags || []).join(", "),
        service_units: (software.service_units || []).join(", "),
        can_control: Boolean(software.can_control),
        control_backend: software.control_backend || "",
        start_command: software.start_command || "",
        restart_command: software.restart_command || "",
        stop_command: software.stop_command || "",
        icon: software.icon || "",
        image: software.image || "",
        color: software.color || "#0ea5e9",
        order: String(software.order ?? 0),
        is_active: Boolean(software.is_active),
      })
    }
  }

  const [prevVersions, setPrevVersions] = useState(versions)
  if (versions !== prevVersions) {
    setPrevVersions(versions)
    if (!versions.length) {
      setSelectedVersionId("new")
      setVersionForm(emptyVersionForm())
    } else {
      setSelectedVersionId((prev) => {
        if (prev === "new") return versions[0].id
        if (versions.some((v) => v.id === prev)) return prev
        return versions[0].id
      })
    }
  }

  const versionSelectionKey = `${selectedVersionId}:${versions.map((v) => v.id).join(",")}`
  const [prevVersionSelection, setPrevVersionSelection] =
    useState(versionSelectionKey)
  if (versionSelectionKey !== prevVersionSelection) {
    setPrevVersionSelection(versionSelectionKey)
    if (selectedVersionId === "new") {
      setVersionForm(emptyVersionForm())
    } else {
      const found = versions.find((v) => v.id === selectedVersionId)
      if (found) setVersionForm(versionToForm(found))
    }
  }

  const saveSoftwareMutation = useMutation({
    mutationFn: (payload: UpdateSoftwarePayload) =>
      updateSoftwarePackage(id, payload),
    onSuccess: (res) => {
      toast.success(res.message || "Software saved")
      void queryClient.invalidateQueries({ queryKey: [PACKAGE_FETCH_KEY, id] })
      void queryClient.invalidateQueries({ queryKey: [SOFTWARES_FETCH_KEY] })
    },
    onError: (err) => {
      toast.error(getRequestErrorMessage(err, "Could not save software"))
    },
  })

  const saveVersionMutation = useMutation({
    mutationFn: async () => {
      const payload = formToVersionPayload(versionForm)
      if (!payload.version) {
        throw new Error("Version label is required")
      }
      if (selectedVersionId === "new") {
        return createSoftwareVersion(id, payload)
      }
      return updateSoftwareVersion(id, selectedVersionId, payload)
    },
    onSuccess: (res) => {
      toast.success(res.message || "Version saved")
      const nextId = res.data?.id
      void queryClient.invalidateQueries({ queryKey: [PACKAGE_FETCH_KEY, id] })
      void queryClient.invalidateQueries({ queryKey: [SOFTWARES_FETCH_KEY] })
      if (nextId) setSelectedVersionId(nextId)
    },
    onError: (err) => {
      toast.error(getRequestErrorMessage(err, "Could not save version"))
    },
  })

  const deleteVersionMutation = useMutation({
    mutationFn: () => {
      if (selectedVersionId === "new") {
        throw new Error("Nothing to delete")
      }
      return deleteSoftwareVersion(id, selectedVersionId)
    },
    onSuccess: (res) => {
      toast.success(res.message || "Version deleted")
      setSelectedVersionId("new")
      void queryClient.invalidateQueries({ queryKey: [PACKAGE_FETCH_KEY, id] })
      void queryClient.invalidateQueries({ queryKey: [SOFTWARES_FETCH_KEY] })
    },
    onError: (err) => {
      toast.error(getRequestErrorMessage(err, "Could not delete version"))
    },
  })

  const accent = softwareForm?.color || software?.color || "var(--primary)"

  const patchSoftware = <K extends keyof SoftwareFormState>(
    key: K,
    value: SoftwareFormState[K]
  ) => {
    setSoftwareForm((prev) => (prev ? { ...prev, [key]: value } : prev))
  }

  const patchVersion = <K extends keyof VersionFormState>(
    key: K,
    value: VersionFormState[K]
  ) => {
    setVersionForm((prev) => ({ ...prev, [key]: value }))
  }

  const targetingFields = useMemo(
    () =>
      [
        ["os", "OS", "linux"],
        ["os_version", "OS version", ""],
        ["distro", "Distro", "Ubuntu"],
        ["distro_id", "Distro ID", "ubuntu"],
        ["distro_version", "Distro version", "24.04"],
        ["arch", "Arch", "amd64"],
        ["platform", "Platform", "linux/amd64"],
        ["package_family", "Package family", "apt"],
        ["kernel", "Kernel", ""],
        ["virtualization", "Virtualization", ""],
        ["container_runtime", "Container runtime", ""],
        ["cloud_provider", "Cloud provider", ""],
      ] as const,
    []
  )

  return (
    <ContentLoader
      title="Package editor"
      breadcrumb={[
        { label: "Softwares", to: "/softwares" },
        ...(software
          ? [
              { label: software.name, to: `/softwares/${id}` },
              { label: "Package", to: `/softwares/${id}/package` },
            ]
          : [{ label: "Package", to: `/softwares/${id}/package` }]),
      ]}
      isLoading={packageQuery.isLoading}
      error={withError(packageQuery.error, packageQuery.data)}
      showHeaderSeparator
      headerClassName="gap-4 pb-6"
    >
      {!software || !softwareForm ? null : (
        <div className="space-y-8">
          <div className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
            <div className="flex min-w-0 items-start gap-3">
              {softwareForm.image.trim() ? (
                <div className="flex h-12 w-12 shrink-0 items-center justify-center overflow-hidden rounded-xl border border-border/60 bg-background shadow-sm">
                  <SoftwareGlyph
                    name={softwareForm.icon || software.icon}
                    image={softwareForm.image}
                    className="h-6 w-6"
                    imgClassName="h-12 w-12 object-cover"
                  />
                </div>
              ) : (
                <div
                  className="flex h-12 w-12 shrink-0 items-center justify-center rounded-xl text-white shadow-sm"
                  style={{ backgroundColor: accent }}
                >
                  <SoftwareGlyph
                    name={softwareForm.icon || software.icon}
                    className="h-6 w-6"
                  />
                </div>
              )}
              <div className="min-w-0">
                <h1 className="truncate text-2xl font-semibold tracking-tight">
                  {softwareForm.name || "Untitled software"}
                </h1>
                <p className="text-sm text-muted-foreground">
                  Edit metadata, versions, and install scripts.
                </p>
              </div>
            </div>
            <div className="flex flex-wrap gap-2">
              <Button variant="outline" size="sm" asChild>
                <Link to={`/softwares/${id}`}>
                  <ArrowLeft className="mr-1.5 h-3.5 w-3.5" />
                  Back
                </Link>
              </Button>
              <Button
                size="sm"
                disabled={saveSoftwareMutation.isPending}
                onClick={() => {
                  saveSoftwareMutation.mutate({
                    name: softwareForm.name.trim(),
                    details: softwareForm.details,
                    category: softwareForm.category.trim(),
                    sub_category: softwareForm.sub_category.trim(),
                    tags: splitCSV(softwareForm.tags),
                    service_units: splitCSV(softwareForm.service_units),
                    can_control: softwareForm.can_control,
                    control_backend: softwareForm.control_backend.trim(),
                    start_command: softwareForm.start_command.trim(),
                    restart_command: softwareForm.restart_command.trim(),
                    stop_command: softwareForm.stop_command.trim(),
                    icon: softwareForm.icon.trim(),
                    image: softwareForm.image.trim(),
                    color: softwareForm.color.trim(),
                    order: Number(softwareForm.order) || 0,
                    is_active: softwareForm.is_active,
                  })
                }}
              >
                {saveSoftwareMutation.isPending ? (
                  <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" />
                ) : (
                  <Save className="mr-1.5 h-3.5 w-3.5" />
                )}
                Save software
              </Button>
            </div>
          </div>

          <section className="space-y-4 rounded-xl border border-border/70 bg-card/40 p-4 sm:p-5">
            <h2 className="text-sm font-semibold tracking-wide text-muted-foreground uppercase">
              Software
            </h2>
            <div className="grid gap-4 sm:grid-cols-2">
              <Field label="Name">
                <Input
                  value={softwareForm.name}
                  onChange={(e) => patchSoftware("name", e.target.value)}
                />
              </Field>
              <Field label="Category">
                <Input
                  value={softwareForm.category}
                  onChange={(e) => patchSoftware("category", e.target.value)}
                />
              </Field>
              <Field label="Sub-category">
                <Input
                  value={softwareForm.sub_category}
                  onChange={(e) =>
                    patchSoftware("sub_category", e.target.value)
                  }
                />
              </Field>
              <Field label="Order">
                <Input
                  type="number"
                  value={softwareForm.order}
                  onChange={(e) => patchSoftware("order", e.target.value)}
                />
              </Field>
              <Field label="Icon">
                <Input
                  value={softwareForm.icon}
                  onChange={(e) => patchSoftware("icon", e.target.value)}
                  placeholder="Package"
                />
              </Field>
              <Field label="Image URL">
                <Input
                  value={softwareForm.image}
                  onChange={(e) => patchSoftware("image", e.target.value)}
                  placeholder="https://… or data:image/…"
                />
              </Field>
              <Field label="Color">
                <div className="flex gap-2">
                  <Input
                    type="color"
                    className="h-8 w-12 p-1"
                    value={
                      softwareForm.color.startsWith("#")
                        ? softwareForm.color
                        : "#0ea5e9"
                    }
                    onChange={(e) => patchSoftware("color", e.target.value)}
                  />
                  <Input
                    value={softwareForm.color}
                    onChange={(e) => patchSoftware("color", e.target.value)}
                  />
                </div>
              </Field>
              <Field label="Tags (comma-separated)" className="sm:col-span-2">
                <Input
                  value={softwareForm.tags}
                  onChange={(e) => patchSoftware("tags", e.target.value)}
                />
              </Field>
              <Field
                label="Service units (comma-separated)"
                className="sm:col-span-2"
                hint="Systemd unit names used for Start / Stop / Restart and journal logs (e.g. docker.service)."
              >
                <Input
                  value={softwareForm.service_units}
                  onChange={(e) =>
                    patchSoftware("service_units", e.target.value)
                  }
                  placeholder="docker.service"
                />
              </Field>
              <label className="flex items-center gap-2 text-sm sm:col-span-2">
                <input
                  type="checkbox"
                  className="size-3.5 accent-foreground"
                  checked={softwareForm.can_control}
                  onChange={(e) => {
                    const checked = e.target.checked
                    patchSoftware("can_control", checked)
                    if (
                      checked &&
                      !softwareForm.control_backend &&
                      softwareForm.service_units
                        .toLowerCase()
                        .includes("docker")
                    ) {
                      patchSoftware("control_backend", "docker")
                    }
                    if (checked && softwareForm.service_units.trim()) {
                      const units = softwareForm.service_units
                        .split(",")
                        .map((s) => s.trim())
                        .filter(Boolean)
                        .join(" ")
                      if (units) {
                        if (!softwareForm.start_command.trim()) {
                          patchSoftware(
                            "start_command",
                            `systemctl start ${units}`
                          )
                        }
                        if (!softwareForm.restart_command.trim()) {
                          patchSoftware(
                            "restart_command",
                            `systemctl restart ${units}`
                          )
                        }
                        if (!softwareForm.stop_command.trim()) {
                          patchSoftware(
                            "stop_command",
                            `systemctl stop ${units}`
                          )
                        }
                      }
                    }
                  }}
                />
                Controllable service (expose Start / Stop / Restart)
              </label>
              <Field
                label="Control backend"
                hint="systemd (default) or docker for Docker Engine–style direct control."
              >
                <Input
                  value={softwareForm.control_backend}
                  onChange={(e) =>
                    patchSoftware("control_backend", e.target.value)
                  }
                  placeholder="systemd"
                  disabled={!softwareForm.can_control}
                />
              </Field>
              <Field
                label="Start command"
                className="sm:col-span-2"
                hint="Shell command run for Start (e.g. systemctl start nginx.service). Auto-filled from service units when empty."
              >
                <Input
                  value={softwareForm.start_command}
                  onChange={(e) =>
                    patchSoftware("start_command", e.target.value)
                  }
                  placeholder="systemctl start nginx.service"
                  disabled={!softwareForm.can_control}
                  className="font-mono text-xs"
                />
              </Field>
              <Field
                label="Restart command"
                className="sm:col-span-2"
                hint="Shell command run for Restart."
              >
                <Input
                  value={softwareForm.restart_command}
                  onChange={(e) =>
                    patchSoftware("restart_command", e.target.value)
                  }
                  placeholder="systemctl restart nginx.service"
                  disabled={!softwareForm.can_control}
                  className="font-mono text-xs"
                />
              </Field>
              <Field
                label="Stop command"
                className="sm:col-span-2"
                hint="Shell command run for Stop."
              >
                <Input
                  value={softwareForm.stop_command}
                  onChange={(e) =>
                    patchSoftware("stop_command", e.target.value)
                  }
                  placeholder="systemctl stop nginx.service"
                  disabled={!softwareForm.can_control}
                  className="font-mono text-xs"
                />
              </Field>
              <Field label="Details" className="sm:col-span-2">
                <textarea
                  value={softwareForm.details}
                  onChange={(e) => patchSoftware("details", e.target.value)}
                  rows={3}
                  className="w-full rounded-lg border border-input bg-transparent px-2.5 py-2 text-sm outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50"
                />
              </Field>
              <label className="flex items-center gap-2 text-sm sm:col-span-2">
                <input
                  type="checkbox"
                  className="size-3.5 accent-foreground"
                  checked={softwareForm.is_active}
                  onChange={(e) =>
                    patchSoftware("is_active", e.target.checked)
                  }
                />
                Active in catalog
              </label>
            </div>
          </section>

          <section className="space-y-4 rounded-xl border border-border/70 bg-card/40 p-4 sm:p-5">
            <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
              <h2 className="text-sm font-semibold tracking-wide text-muted-foreground uppercase">
                Versions
              </h2>
              <div className="flex flex-wrap gap-2">
                <Button
                  size="sm"
                  variant="outline"
                  onClick={() => {
                    setSelectedVersionId("new")
                    setVersionForm(emptyVersionForm())
                    setScriptTab("install")
                  }}
                >
                  <Plus className="mr-1.5 h-3.5 w-3.5" />
                  New version
                </Button>
                <Button
                  size="sm"
                  disabled={saveVersionMutation.isPending}
                  onClick={() => saveVersionMutation.mutate()}
                >
                  {saveVersionMutation.isPending ? (
                    <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" />
                  ) : (
                    <Save className="mr-1.5 h-3.5 w-3.5" />
                  )}
                  {selectedVersionId === "new"
                    ? "Create version"
                    : "Save version"}
                </Button>
                {selectedVersionId !== "new" ? (
                  <Button
                    size="sm"
                    variant="outline"
                    className="border-destructive/40 text-destructive hover:bg-destructive/10"
                    disabled={deleteVersionMutation.isPending}
                    onClick={() => {
                      if (
                        !window.confirm(
                          "Delete this version? Install history that points to it may break."
                        )
                      ) {
                        return
                      }
                      deleteVersionMutation.mutate()
                    }}
                  >
                    {deleteVersionMutation.isPending ? (
                      <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" />
                    ) : (
                      <Trash2 className="mr-1.5 h-3.5 w-3.5" />
                    )}
                    Delete
                  </Button>
                ) : null}
              </div>
            </div>

            <div className="flex flex-wrap gap-2">
              {versions.map((v) => (
                <button
                  key={v.id}
                  type="button"
                  onClick={() => setSelectedVersionId(v.id)}
                  className={cn(
                    "inline-flex items-center gap-1.5 rounded-lg border px-3 py-1.5 text-sm transition-colors",
                    selectedVersionId === v.id
                      ? "border-sky-500/40 bg-sky-500/10 text-foreground"
                      : "border-border/70 bg-background/40 text-muted-foreground hover:bg-muted/40"
                  )}
                >
                  <span className="font-mono">v{v.version}</span>
                  {v.is_latest ? (
                    <Badge variant="outline" className="gap-1 text-[10px]">
                      <Star className="h-2.5 w-2.5" />
                      latest
                    </Badge>
                  ) : null}
                </button>
              ))}
              <button
                type="button"
                onClick={() => {
                  setSelectedVersionId("new")
                  setVersionForm(emptyVersionForm())
                }}
                className={cn(
                  "inline-flex items-center gap-1.5 rounded-lg border border-dashed px-3 py-1.5 text-sm",
                  selectedVersionId === "new"
                    ? "border-sky-500/40 bg-sky-500/10 text-foreground"
                    : "border-border/70 text-muted-foreground hover:bg-muted/40"
                )}
              >
                <Plus className="h-3.5 w-3.5" />
                Draft
              </button>
            </div>

            <div className="grid gap-4 sm:grid-cols-2">
              <Field label="Version label">
                <Input
                  value={versionForm.version}
                  onChange={(e) => patchVersion("version", e.target.value)}
                  placeholder="1.0.0"
                />
              </Field>
              <label className="flex items-end gap-2 pb-1 text-sm">
                <input
                  type="checkbox"
                  className="size-3.5 accent-foreground"
                  checked={versionForm.is_latest}
                  onChange={(e) =>
                    patchVersion("is_latest", e.target.checked)
                  }
                />
                Mark as latest
              </label>
            </div>

            <div>
              <h3 className="mb-2 text-xs font-semibold tracking-wide text-muted-foreground uppercase">
                Host targeting
              </h3>
              <p className="mb-3 text-xs text-muted-foreground">
                Leave blank to match any host. Used when picking the right
                script set for this machine.
              </p>
              <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
                {targetingFields.map(([key, label, placeholder]) => (
                  <Field key={key} label={label}>
                    <Input
                      value={versionForm[key]}
                      placeholder={placeholder}
                      onChange={(e) => patchVersion(key, e.target.value)}
                    />
                  </Field>
                ))}
              </div>
            </div>

            <Tabs value={scriptTab} onValueChange={setScriptTab}>
              <TabsList>
                <TabsTrigger value="install">Install</TabsTrigger>
                <TabsTrigger value="uninstall">Uninstall</TabsTrigger>
                <TabsTrigger value="upgrade">Upgrade</TabsTrigger>
                <TabsTrigger value="custom">Custom Script</TabsTrigger>
              </TabsList>
              <TabsContent value="install" className="mt-3">
                <BashScriptEditor
                  label="Install script"
                  value={versionForm.install_script}
                  onChange={(v) => patchVersion("install_script", v)}
                />
              </TabsContent>
              <TabsContent value="uninstall" className="mt-3">
                <BashScriptEditor
                  label="Uninstall script"
                  value={versionForm.uninstall_script}
                  onChange={(v) => patchVersion("uninstall_script", v)}
                />
              </TabsContent>
              <TabsContent value="upgrade" className="mt-3">
                <BashScriptEditor
                  label="Upgrade script"
                  value={versionForm.upgrade_script}
                  onChange={(v) => patchVersion("upgrade_script", v)}
                  placeholder={
                    "#!/usr/bin/env bash\nset -euo pipefail\n\n# Optional — leave empty to reuse install\n"
                  }
                />
              </TabsContent>
              <TabsContent value="custom" className="mt-3">
                <BashScriptEditor
                  label="Custom script"
                  value={versionForm.custom_script}
                  onChange={(v) => patchVersion("custom_script", v)}
                  placeholder={
                    "#!/usr/bin/env bash\nset -euo pipefail\n\n# Runs after a successful install when non-empty (config / post-setup)\n"
                  }
                />
                <p className="mt-2 text-xs text-muted-foreground">
                  Executed automatically after install succeeds when this field
                  is not empty. Use it for configuration and post-setup steps.
                </p>
              </TabsContent>
            </Tabs>
          </section>
        </div>
      )}
    </ContentLoader>
  )
}

function Field({
  label,
  children,
  className,
  hint,
}: {
  label: string
  children: ReactNode
  className?: string
  hint?: string
}) {
  return (
    <div className={cn("space-y-1.5", className)}>
      <Label className="text-xs text-muted-foreground">{label}</Label>
      {children}
      {hint ? (
        <p className="text-[11px] text-muted-foreground">{hint}</p>
      ) : null}
    </div>
  )
}
