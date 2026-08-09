import { useState } from "react"
import { Link, useNavigate } from "react-router"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { Box, CupSoda, Globe, Hexagon, Loader2, Save, Settings2, ShieldCheck } from "lucide-react"
import { toast } from "sonner"

import ContentLoader from "@/components/content-loader"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
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
import { Switch } from "@/components/ui/switch"
import {
  AUTO_REFRESH_OPTIONS,
  DEFAULT_AUTO_REFRESH_MS,
  useAutoRefreshMs,
  writeAutoRefreshMs,
} from "@/lib/auto-refresh"
import { getRequestErrorMessage, withError } from "@/lib/network"
import { cn } from "@/lib/utils"
import {
  controlSoftwareService,
  enqueueSoftwareActions,
} from "@/modules/softwares/pages/list/api"

import {
  getGeneralSettings,
  SETTINGS_GENERAL_FETCH_KEY,
  updateGeneralSettings,
  updateModuleSettings,
} from "./api"

export default function GeneralSettingsPage() {
  const queryClient = useQueryClient()
  const navigate = useNavigate()
  const [name, setName] = useState("")
  const [description, setDescription] = useState("")
  const refreshMs = useAutoRefreshMs()

  const query = useQuery({
    queryKey: [SETTINGS_GENERAL_FETCH_KEY],
    queryFn: getGeneralSettings,
  })

  const settingsData = query.data?.data
  const [prevSettingsData, setPrevSettingsData] = useState(settingsData)
  if (settingsData !== prevSettingsData) {
    setPrevSettingsData(settingsData)
    if (settingsData) {
      setName(settingsData.workspace_name || "")
      setDescription(settingsData.workspace_description || "")
    }
  }

  const saveMutation = useMutation({
    mutationFn: () =>
      updateGeneralSettings({
        workspace_name: name.trim(),
        workspace_description: description.trim(),
      }),
    onSuccess: (res) => {
      toast.success(res.message || "Settings saved")
      void queryClient.invalidateQueries({
        queryKey: [SETTINGS_GENERAL_FETCH_KEY],
      })
    },
    onError: (err) => {
      toast.error(getRequestErrorMessage(err, "Failed to save settings"))
    },
  })

  const moduleMutation = useMutation({
    mutationFn: updateModuleSettings,
    onSuccess: (res) => {
      toast.success(res.message || "Module settings saved")
      void queryClient.invalidateQueries({
        queryKey: [SETTINGS_GENERAL_FETCH_KEY],
      })
      void queryClient.invalidateQueries({ queryKey: ["general-data"] })
    },
    onError: (err) => {
      toast.error(getRequestErrorMessage(err, "Failed to update module"))
    },
  })

  const dockerActionMutation = useMutation({
    mutationFn: async () => {
      const docker = query.data?.data?.docker
      const id = docker?.software_id?.trim()
      if (!id) {
        throw new Error("Docker Engine is not in the Softwares catalog")
      }
      if (docker?.installed || docker?.binary_present) {
        if (docker?.running) {
          return { kind: "running" as const }
        }
        await controlSoftwareService(id, "start")
        return { kind: "started" as const }
      }
      const res = await enqueueSoftwareActions("install", [id])
      return { kind: "queued" as const, message: res.message }
    },
    onSuccess: (res) => {
      if (res.kind === "running") {
        toast.success("Docker Engine is already running")
        return
      }
      if (res.kind === "started") {
        toast.success("Docker Engine start requested")
        void queryClient.invalidateQueries({
          queryKey: [SETTINGS_GENERAL_FETCH_KEY],
        })
        return
      }
      toast.success(res.message || "Docker Engine install queued", {
        action: {
          label: "View queue",
          onClick: () => navigate("/softwares/installing"),
        },
      })
      navigate("/softwares/installing")
    },
    onError: (err) => {
      toast.error(getRequestErrorMessage(err, "Docker action failed"))
    },
  })

  const dirty =
    name.trim() !== (query.data?.data?.workspace_name || "").trim() ||
    description.trim() !==
      (query.data?.data?.workspace_description || "").trim()

  const dockerEnabled = Boolean(settingsData?.docker_enabled)
  const k8sEnabled = Boolean(settingsData?.kubernetes_enabled)
  const proxyEnabled = Boolean(settingsData?.proxymanager_enabled)
  const brewEnabled = Boolean(settingsData?.brew_enabled)
  const localhostAutoLogin = Boolean(settingsData?.localhost_auto_login)
  const docker = settingsData?.docker
  const kubernetes = settingsData?.kubernetes
  const proxymanager = settingsData?.proxymanager
  const brew = settingsData?.brew

  return (
    <ContentLoader
      title="General"
      description="Workspace name and basic panel settings."
      breadcrumb={[
        { label: "Settings", to: "/settings" },
        { label: "General" },
      ]}
      isLoading={query.isLoading}
      error={withError(query.error, query.data)}
      showHeaderSeparator
    >
      <div className="w-full space-y-8">
        <section className="space-y-4">
          <div>
            <h2 className="text-sm font-medium">Workspace</h2>
            <p className="mt-1 text-sm text-muted-foreground">
              Shown in the panel header and browser title where applicable.
            </p>
          </div>
          <Separator />
          <div className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="workspace-name">Name</Label>
              <Input
                id="workspace-name"
                value={name}
                maxLength={120}
                placeholder="Container Workspace"
                onChange={(e) => setName(e.target.value)}
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="workspace-description">Description</Label>
              <textarea
                id="workspace-description"
                value={description}
                maxLength={500}
                rows={4}
                placeholder="Optional short description of this workspace."
                onChange={(e) => setDescription(e.target.value)}
                className={cn(
                  "w-full min-w-0 rounded-lg border border-input bg-transparent px-2.5 py-2 text-base outline-none transition-colors",
                  "placeholder:text-muted-foreground focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50",
                  "disabled:cursor-not-allowed disabled:opacity-50 md:text-sm dark:bg-input/30"
                )}
              />
              <p className="text-xs text-muted-foreground">
                {description.length}/500
              </p>
            </div>
          </div>
          <div className="flex justify-end">
            <Button
              type="button"
              disabled={!dirty || saveMutation.isPending}
              onClick={() => saveMutation.mutate()}
            >
              {saveMutation.isPending ? (
                <Loader2 className="size-4 animate-spin" />
              ) : (
                <Save className="size-4" />
              )}
              Save changes
            </Button>
          </div>
        </section>

        <section className="space-y-4">
          <div>
            <h2 className="text-sm font-medium">Modules</h2>
            <p className="mt-1 text-sm text-muted-foreground">
              Enable Docker, Kubernetes, Proxy Manager, and Brew Package in the
              sidebar. Turning a module off hides its menu without uninstalling
              anything.
            </p>
          </div>
          <Separator />

          <div className="space-y-3">
            <div className="rounded-xl border bg-card p-4 shadow-xs">
              <div className="flex flex-wrap items-start justify-between gap-3">
                <div className="flex min-w-0 items-start gap-3">
                  <div className="mt-0.5 flex size-9 shrink-0 items-center justify-center rounded-lg bg-muted">
                    <Box className="size-4 text-muted-foreground" />
                  </div>
                  <div className="min-w-0 space-y-1">
                    <div className="flex flex-wrap items-center gap-2">
                      <h3 className="text-sm font-medium">Docker</h3>
                      {docker?.running ? (
                        <Badge variant="default">Running</Badge>
                      ) : docker?.installed || docker?.binary_present ? (
                        <Badge variant="secondary">Installed</Badge>
                      ) : (
                        <Badge variant="outline">Not installed</Badge>
                      )}
                    </div>
                    <p className="text-sm text-muted-foreground">
                      Manage containers, images, networks, volumes, and stacks.
                      When enabled, Docker appears in the left sidebar.
                    </p>
                  </div>
                </div>
                <Switch
                  checked={dockerEnabled}
                  disabled={moduleMutation.isPending}
                  onCheckedChange={(checked) =>
                    moduleMutation.mutate({ docker_enabled: checked })
                  }
                  aria-label="Enable Docker module"
                />
              </div>
              {dockerEnabled && !(docker?.installed || docker?.binary_present) ? (
                <div className="mt-4 flex flex-wrap items-center gap-2 border-t pt-4">
                  <Button
                    type="button"
                    size="sm"
                    disabled={
                      dockerActionMutation.isPending || !docker?.software_id
                    }
                    onClick={() => dockerActionMutation.mutate()}
                  >
                    {dockerActionMutation.isPending ? (
                      <Loader2 className="size-3.5 animate-spin" />
                    ) : (
                      <Box className="size-3.5" />
                    )}
                    Install &amp; start Docker
                  </Button>
                  <Button type="button" size="sm" variant="outline" asChild>
                    <Link to="/softwares">Open Softwares</Link>
                  </Button>
                </div>
              ) : null}
              {dockerEnabled &&
              (docker?.installed || docker?.binary_present) &&
              !docker?.running ? (
                <div className="mt-4 flex flex-wrap items-center gap-2 border-t pt-4">
                  <Button
                    type="button"
                    size="sm"
                    disabled={
                      dockerActionMutation.isPending || !docker?.software_id
                    }
                    onClick={() => dockerActionMutation.mutate()}
                  >
                    {dockerActionMutation.isPending ? (
                      <Loader2 className="size-3.5 animate-spin" />
                    ) : null}
                    Start Docker
                  </Button>
                </div>
              ) : null}
            </div>

            <div className="rounded-xl border bg-card p-4 shadow-xs">
              <div className="flex flex-wrap items-start justify-between gap-3">
                <div className="flex min-w-0 items-start gap-3">
                  <div className="mt-0.5 flex size-9 shrink-0 items-center justify-center rounded-lg bg-muted">
                    <Hexagon className="size-4 text-muted-foreground" />
                  </div>
                  <div className="min-w-0 space-y-1">
                    <div className="flex flex-wrap items-center gap-2">
                      <h3 className="text-sm font-medium">Kubernetes</h3>
                      {kubernetes?.configured ? (
                        <Badge variant="default">Configured</Badge>
                      ) : (
                        <Badge variant="outline">Needs kubeconfig</Badge>
                      )}
                    </div>
                    <p className="text-sm text-muted-foreground">
                      Browse cluster resources and manage kubeconfig secrets.
                      When enabled, Kubernetes appears in the left sidebar.
                    </p>
                  </div>
                </div>
                <Switch
                  checked={k8sEnabled}
                  disabled={moduleMutation.isPending}
                  onCheckedChange={(checked) =>
                    moduleMutation.mutate({ kubernetes_enabled: checked })
                  }
                  aria-label="Enable Kubernetes module"
                />
              </div>
              {k8sEnabled ? (
                <div className="mt-4 flex flex-wrap items-center gap-2 border-t pt-4">
                  <Button type="button" size="sm" variant="outline" asChild>
                    <Link to="/kubernetes/settings">
                      <Settings2 className="size-3.5" />
                      {kubernetes?.configured
                        ? "Manage kubeconfig"
                        : "Add kubeconfig"}
                    </Link>
                  </Button>
                </div>
              ) : null}
            </div>

            <div className="rounded-xl border bg-card p-4 shadow-xs">
              <div className="flex flex-wrap items-start justify-between gap-3">
                <div className="flex min-w-0 items-start gap-3">
                  <div className="mt-0.5 flex size-9 shrink-0 items-center justify-center rounded-lg bg-muted">
                    <Globe className="size-4 text-muted-foreground" />
                  </div>
                  <div className="min-w-0 space-y-1">
                    <div className="flex flex-wrap items-center gap-2">
                      <h3 className="text-sm font-medium">Proxy Manager</h3>
                      {proxymanager?.dirty ? (
                        <Badge variant="secondary">Needs apply</Badge>
                      ) : proxymanager?.configured ? (
                        <Badge variant="default">
                          {proxymanager.active_engine || "Configured"}
                        </Badge>
                      ) : (
                        <Badge variant="outline">Not configured</Badge>
                      )}
                    </div>
                    <p className="text-sm text-muted-foreground">
                      Reverse-proxy hosts with Fiber, Nginx, or Traefik. When
                      enabled, Proxy Manager appears in the sidebar.
                    </p>
                  </div>
                </div>
                <Switch
                  checked={proxyEnabled}
                  disabled={moduleMutation.isPending}
                  onCheckedChange={(checked) =>
                    moduleMutation.mutate({ proxymanager_enabled: checked })
                  }
                  aria-label="Enable Proxy Manager module"
                />
              </div>
              {proxyEnabled ? (
                <div className="mt-4 flex flex-wrap items-center gap-2 border-t pt-4">
                  <Button type="button" size="sm" variant="outline" asChild>
                    <Link to="/proxymanager/overview">
                      <Globe className="size-3.5" />
                      Open Proxy Manager
                    </Link>
                  </Button>
                </div>
              ) : null}
            </div>

            <div className="rounded-xl border bg-card p-4 shadow-xs">
              <div className="flex flex-wrap items-start justify-between gap-3">
                <div className="flex min-w-0 items-start gap-3">
                  <div className="mt-0.5 flex size-9 shrink-0 items-center justify-center rounded-lg bg-muted">
                    <CupSoda className="size-4 text-muted-foreground" />
                  </div>
                  <div className="min-w-0 space-y-1">
                    <div className="flex flex-wrap items-center gap-2">
                      <h3 className="text-sm font-medium">Brew Package</h3>
                      {brew?.installing || brew?.bootstrap?.running ? (
                        <Badge variant="secondary">Installing…</Badge>
                      ) : brew?.binary_present ? (
                        <Badge variant="default">Installed</Badge>
                      ) : (
                        <Badge variant="outline">Not installed</Badge>
                      )}
                    </div>
                    <p className="text-sm text-muted-foreground">
                      Browse and manage Homebrew formulae. When enabled, Brew
                      Manager appears in the sidebar and Homebrew is installed
                      automatically if missing.
                    </p>
                  </div>
                </div>
                <Switch
                  checked={brewEnabled}
                  disabled={moduleMutation.isPending}
                  onCheckedChange={(checked) =>
                    moduleMutation.mutate({ brew_enabled: checked })
                  }
                  aria-label="Enable Brew Package module"
                />
              </div>
              {brewEnabled ? (
                <div className="mt-4 flex flex-wrap items-center gap-2 border-t pt-4">
                  <Button type="button" size="sm" variant="outline" asChild>
                    <Link to="/brew">
                      <CupSoda className="size-3.5" />
                      Open Brew Manager
                    </Link>
                  </Button>
                </div>
              ) : null}
            </div>
          </div>
        </section>

        <section className="space-y-4">
          <div>
            <h2 className="text-sm font-medium">Security</h2>
            <p className="mt-1 text-sm text-muted-foreground">
              Access controls for opening the panel on this host.
            </p>
          </div>
          <Separator />
          <div className="rounded-xl border bg-card p-4 shadow-xs">
            <div className="flex flex-wrap items-start justify-between gap-3">
              <div className="flex min-w-0 items-start gap-3">
                <div className="mt-0.5 flex size-9 shrink-0 items-center justify-center rounded-lg bg-muted">
                  <ShieldCheck className="size-4 text-muted-foreground" />
                </div>
                <div className="min-w-0 space-y-1">
                  <h3 className="text-sm font-medium">Localhost auto-login</h3>
                  <p className="text-sm text-muted-foreground">
                    When enabled, opening the panel from{" "}
                    <code className="text-xs">127.0.0.1</code> or{" "}
                    <code className="text-xs">::1</code> signs you in as the
                    Linux user running this panel (no password prompt). Remote
                    clients are never auto-logged in.
                  </p>
                </div>
              </div>
              <Switch
                checked={localhostAutoLogin}
                disabled={moduleMutation.isPending}
                onCheckedChange={(checked) =>
                  moduleMutation.mutate({ localhost_auto_login: checked })
                }
                aria-label="Enable localhost auto-login"
              />
            </div>
          </div>
        </section>

        <section className="space-y-4">
          <div>
            <h2 className="text-sm font-medium">Dashboard</h2>
            <p className="mt-1 text-sm text-muted-foreground">
              Auto-refresh interval for live metrics and process lists. Stored
              in this browser.
            </p>
          </div>
          <Separator />
          <div className="space-y-2">
            <Label htmlFor="auto-refresh">Auto-refresh</Label>
            <Select
              value={String(refreshMs)}
              onValueChange={(v) => {
                const next = writeAutoRefreshMs(
                  Number(v ?? String(DEFAULT_AUTO_REFRESH_MS))
                )
                toast.success(
                  next > 0
                    ? `Auto-refresh set to ${next / 1000}s`
                    : "Auto-refresh turned off"
                )
              }}
            >
              <SelectTrigger id="auto-refresh" className="w-full max-w-xs">
                <SelectValue placeholder="Interval" />
              </SelectTrigger>
              <SelectContent>
                {AUTO_REFRESH_OPTIONS.map((opt) => (
                  <SelectItem key={opt.value} value={String(opt.value)}>
                    {opt.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
        </section>
      </div>
    </ContentLoader>
  )
}
