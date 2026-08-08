import { useEffect, useState } from "react"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { Box, Container, Loader2, Network, Save, Server } from "lucide-react"
import { toast } from "sonner"

import ContentLoader from "@/components/content-loader"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Separator } from "@/components/ui/separator"
import { toastRequestError } from "@/lib/network"
import { cn } from "@/lib/utils"

import {
  getProxyRuntime,
  getProxySettings,
  PROXY_RUNTIME_KEY,
  PROXY_SETTINGS_KEY,
  updateProxySettings,
  type ProxyDockerNetworkMode,
  type ProxyEngine,
  type ProxyRuntime,
  type ProxySettings,
} from "../_shared/api"
import { invalidateProxyQueries, runProxyApply } from "../_shared/apply"
import {
  DirtyBanner,
  PROXY_PAGE_DESCRIPTIONS,
  ProxyRefreshButton,
  ProxySubNav,
  SummaryChip,
} from "../_shared/page-chrome"
import { ReactSelect } from "@/components/ui/reactselect"

const ENGINES: {
  value: ProxyEngine
  label: string
  description: string
  icon: typeof Network
}[] = [
  {
    value: "fiber",
    label: "Fiber",
    description: "In-process reverse proxy inside this app (Host-based).",
    icon: Network,
  },
  {
    value: "nginx",
    label: "Nginx",
    description: "Generate nginx.conf and run on the host or in Docker.",
    icon: Server,
  },
  {
    value: "traefik",
    label: "Traefik",
    description: "Generate Traefik file-provider config; host or Docker.",
    icon: Box,
  },
]

const RUNTIMES: {
  value: ProxyRuntime
  label: string
  description: string
  icon: typeof Server
}[] = [
  {
    value: "docker",
    label: "Docker",
    description: "Start/reload a managed container with published ports.",
    icon: Container,
  },
  {
    value: "host",
    label: "Host",
    description: "Use a binary already installed on this VM.",
    icon: Server,
  },
]

const DOCKER_NET_MODES: {
  value: ProxyDockerNetworkMode
  label: string
  description: string
}[] = [
  {
    value: "published",
    label: "Published ports",
    description:
      "Map container ports to a host IP (default bridge). Use when the proxy shares the host's ports.",
  },
  {
    value: "host",
    label: "Host network",
    description:
      "network_mode=host — listen directly on the host stack (no port publish).",
  },
  {
    value: "macvlan",
    label: "Macvlan / dedicated IP",
    description:
      "Attach to a Docker network (macvlan/ipvlan/custom) with an optional static IPv4.",
  },
]

export default function ProxySettingsPage() {
  const queryClient = useQueryClient()
  const settingsQuery = useQuery({
    queryKey: [PROXY_SETTINGS_KEY],
    queryFn: getProxySettings,
  })
  const runtimeQuery = useQuery({
    queryKey: [PROXY_RUNTIME_KEY],
    queryFn: getProxyRuntime,
  })

  const [form, setForm] = useState<Partial<ProxySettings> | null>(null)

  useEffect(() => {
    if (settingsQuery.data?.data) {
      setForm({ ...settingsQuery.data.data })
    }
  }, [settingsQuery.data])

  const saveMutation = useMutation({
    mutationFn: async () => {
      const saved = await updateProxySettings(form || {})
      let applied = true
      try {
        await runProxyApply(queryClient, { quiet: true })
      } catch {
        applied = false
      }
      return { saved, applied }
    },
    onSuccess: ({ saved, applied }) => {
      if (applied) {
        toast.success(saved.message || "Saved and applied")
      } else {
        toast.warning(
          "Settings saved, but apply failed — check Status / Logs for details",
        )
      }
      void invalidateProxyQueries(queryClient)
    },
    onError: (err) => toastRequestError(err, "Save failed"),
  })

  const applyMutation = useMutation({
    mutationFn: () => runProxyApply(queryClient),
  })

  const settings = settingsQuery.data?.data
  const runtime = runtimeQuery.data?.data
  const loading = settingsQuery.isLoading || !form
  const engine = (form?.active_engine || "fiber") as ProxyEngine
  const runtimeMode = (
    engine === "nginx" ? form?.nginx_runtime : form?.traefik_runtime
  ) as ProxyRuntime | undefined
  const busy = saveMutation.isPending || applyMutation.isPending

  return (
    <ContentLoader
      title="Proxy settings"
      description={PROXY_PAGE_DESCRIPTIONS.overview}
      breadcrumb={[
        { label: "Proxy Manager", to: "/proxymanager" },
        { label: "Settings" },
      ]}
      isLoading={loading}
      error={settingsQuery.error}
      rightComponent={
        <div className="flex gap-2">
          <ProxyRefreshButton
            isFetching={settingsQuery.isFetching}
            onClick={() => {
              void settingsQuery.refetch()
              void runtimeQuery.refetch()
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
          <Button
            size="sm"
            onClick={() => saveMutation.mutate()}
            disabled={busy}
          >
            {saveMutation.isPending ? (
              <Loader2 className="size-3.5 animate-spin" />
            ) : (
              <Save className="size-3.5" />
            )}
            Save &amp; Apply
          </Button>
        </div>
      }
    >
      <ProxySubNav />
      <DirtyBanner
        dirty={settings?.dirty}
        lastError={settings?.last_apply_error}
        onApply={() => applyMutation.mutate()}
        applying={busy}
      />

      <div className="mb-6 grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
        <SummaryChip label="Active engine" value={settings?.active_engine || "—"} />
        <SummaryChip
          label="Docker"
          value={
            runtime?.docker_available
              ? "Available"
              : runtime?.docker_error || "Unavailable"
          }
        />
        <SummaryChip
          label="Nginx on host"
          value={runtime?.nginx_installed ? runtime.nginx_binary || "Yes" : "Not found"}
        />
        <SummaryChip
          label="Traefik on host"
          value={
            runtime?.traefik_installed ? runtime.traefik_binary || "Yes" : "Not found"
          }
        />
      </div>

      <div className="space-y-8">
        <section className="space-y-3">
          <div>
            <h2 className="text-sm font-semibold">Proxy engine</h2>
            <p className="mt-1 text-sm text-muted-foreground">
              Pick how traffic is proxied. Only one engine is active at a time —
              shared hosts/SSL rules are converted to that engine&apos;s config on
              Apply.
            </p>
          </div>
          <div className="grid gap-3 sm:grid-cols-3">
            {ENGINES.map((item) => {
              const Icon = item.icon
              const selected = engine === item.value
              return (
                <button
                  key={item.value}
                  type="button"
                  onClick={() =>
                    setForm((f) => ({ ...f, active_engine: item.value }))
                  }
                  className={cn(
                    "rounded-xl border p-4 text-left transition-colors",
                    selected
                      ? "border-primary bg-primary/5 ring-1 ring-primary"
                      : "hover:bg-muted/40",
                  )}
                >
                  <Icon className="mb-2 size-5 text-muted-foreground" />
                  <div className="text-sm font-medium">{item.label}</div>
                  <p className="mt-1 text-xs text-muted-foreground">
                    {item.description}
                  </p>
                </button>
              )
            })}
          </div>
        </section>

        {(engine === "nginx" || engine === "traefik") && (
          <section className="space-y-3">
            <div>
              <h2 className="text-sm font-semibold">
                {engine === "nginx" ? "Nginx" : "Traefik"} runtime
              </h2>
              <p className="mt-1 text-sm text-muted-foreground">
                Run on the VM host binary, or under Docker when the engine is
                available.
              </p>
            </div>
            <div className="grid gap-3 sm:grid-cols-2">
              {RUNTIMES.map((item) => {
                const Icon = item.icon
                const selected = (runtimeMode || "docker") === item.value
                return (
                  <button
                    key={item.value}
                    type="button"
                    onClick={() =>
                      setForm((f) =>
                        engine === "nginx"
                          ? { ...f, nginx_runtime: item.value }
                          : { ...f, traefik_runtime: item.value },
                      )
                    }
                    className={cn(
                      "rounded-xl border p-4 text-left transition-colors",
                      selected
                        ? "border-primary bg-primary/5 ring-1 ring-primary"
                        : "hover:bg-muted/40",
                    )}
                  >
                    <Icon className="mb-2 size-5 text-muted-foreground" />
                    <div className="text-sm font-medium">{item.label}</div>
                    <p className="mt-1 text-xs text-muted-foreground">
                      {item.description}
                    </p>
                  </button>
                )
              })}
            </div>

            {(runtimeMode || "docker") === "docker" && (
              <div className="space-y-4">
                <div className="grid gap-4 sm:grid-cols-2">
                  <div className="space-y-2">
                    <Label>Image</Label>
                    <Input
                      value={
                        engine === "nginx"
                          ? form?.nginx_image || ""
                          : form?.traefik_image || ""
                      }
                      onChange={(e) =>
                        setForm((f) =>
                          engine === "nginx"
                            ? { ...f, nginx_image: e.target.value }
                            : { ...f, traefik_image: e.target.value },
                        )
                      }
                    />
                  </div>
                  <div className="space-y-2">
                    <Label>Container name</Label>
                    <Input
                      value={
                        engine === "nginx"
                          ? form?.nginx_container_name || ""
                          : form?.traefik_container_name || ""
                      }
                      onChange={(e) =>
                        setForm((f) =>
                          engine === "nginx"
                            ? { ...f, nginx_container_name: e.target.value }
                            : { ...f, traefik_container_name: e.target.value },
                        )
                      }
                    />
                  </div>
                </div>

                <div className="space-y-3">
                  <div>
                    <h3 className="text-sm font-medium">Docker networking</h3>
                    <p className="mt-1 text-xs text-muted-foreground">
                      Use host network or a macvlan IP when you cannot (or do
                      not want to) publish ports on the Docker host.
                    </p>
                  </div>
                  <div className="grid gap-3 sm:grid-cols-3">
                    {DOCKER_NET_MODES.map((item) => {
                      const selected =
                        (form?.docker_network_mode || "published") ===
                        item.value
                      return (
                        <button
                          key={item.value}
                          type="button"
                          onClick={() =>
                            setForm((f) => ({
                              ...f,
                              docker_network_mode: item.value,
                            }))
                          }
                          className={cn(
                            "rounded-xl border p-3 text-left transition-colors",
                            selected
                              ? "border-primary bg-primary/5 ring-1 ring-primary"
                              : "hover:bg-muted/40",
                          )}
                        >
                          <div className="text-sm font-medium">{item.label}</div>
                          <p className="mt-1 text-xs text-muted-foreground">
                            {item.description}
                          </p>
                        </button>
                      )
                    })}
                  </div>

                  {(form?.docker_network_mode || "published") ===
                    "published" && (
                    <div className="space-y-2">
                      <Label>Publish bind IP</Label>
                      <ReactSelect
                        isClearable
                        options={[
                          { value: "0.0.0.0", label: "0.0.0.0 (all interfaces)" },
                          ...(runtime?.host_ips || []).map((ip) => ({
                            value: ip,
                            label: ip,
                          })),
                        ]}
                        value={
                          form?.docker_publish_ip
                            ? {
                                value: form.docker_publish_ip,
                                label: form.docker_publish_ip,
                              }
                            : {
                                value: "0.0.0.0",
                                label: "0.0.0.0 (all interfaces)",
                              }
                        }
                        onChange={(opt) =>
                          setForm((f) => ({
                            ...f,
                            docker_publish_ip:
                              !opt?.value || opt.value === "0.0.0.0"
                                ? ""
                                : opt.value,
                          }))
                        }
                        placeholder="Select host IP…"
                      />
                      <p className="text-xs text-muted-foreground">
                        Host IP used for published HTTP/HTTPS ports. Leave all
                        interfaces if unsure.
                      </p>
                    </div>
                  )}

                  {(form?.docker_network_mode || "published") === "host" && (
                    <p className="rounded-lg border border-dashed px-3 py-2 text-xs text-muted-foreground">
                      Container shares the host network. Listen ports below are
                      bound on the host itself (no Docker port mapping).
                    </p>
                  )}

                  {(form?.docker_network_mode || "published") === "macvlan" && (
                    <div className="grid gap-4 sm:grid-cols-2">
                      <div className="space-y-2">
                        <Label>Docker network</Label>
                        <ReactSelect
                          options={(runtime?.docker_networks || []).map((n) => ({
                            value: n.name,
                            label: `${n.name} (${n.driver})`,
                          }))}
                          value={
                            form?.docker_network_name
                              ? {
                                  value: form.docker_network_name,
                                  label: form.docker_network_name,
                                }
                              : null
                          }
                          onChange={(opt) =>
                            setForm((f) => ({
                              ...f,
                              docker_network_name: opt?.value || "",
                            }))
                          }
                          placeholder="Select network…"
                          noOptionsMessage={() =>
                            runtime?.docker_available
                              ? "No networks found"
                              : "Docker unavailable"
                          }
                        />
                        <Input
                          className="mt-2"
                          placeholder="Or type network name…"
                          value={form?.docker_network_name || ""}
                          onChange={(e) =>
                            setForm((f) => ({
                              ...f,
                              docker_network_name: e.target.value,
                            }))
                          }
                        />
                      </div>
                      <div className="space-y-2">
                        <Label>Static IPv4 (optional)</Label>
                        <Input
                          placeholder="e.g. 192.168.1.50"
                          value={form?.docker_ipv4_address || ""}
                          onChange={(e) =>
                            setForm((f) => ({
                              ...f,
                              docker_ipv4_address: e.target.value,
                            }))
                          }
                        />
                        <p className="text-xs text-muted-foreground">
                          Assign a macvlan/ipvlan address so the proxy is
                          reachable without host port publish.
                        </p>
                      </div>
                    </div>
                  )}
                </div>
              </div>
            )}
          </section>
        )}

        <Separator />

        <section className="space-y-4">
          <div>
            <h2 className="text-sm font-semibold">Listen ports &amp; paths</h2>
            <p className="mt-1 text-sm text-muted-foreground">
              Used when applying Nginx/Traefik. With published ports these are
              host mappings; with host/macvlan they are the process listen ports.
              Fiber uses the app&apos;s own HTTP listener.
            </p>
          </div>
          <div className="grid gap-4 sm:grid-cols-2">
            <div className="space-y-2">
              <Label htmlFor="http_port">HTTP port</Label>
              <Input
                id="http_port"
                type="number"
                value={form?.http_port ?? 80}
                onChange={(e) =>
                  setForm((f) => ({ ...f, http_port: Number(e.target.value) }))
                }
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="https_port">HTTPS port</Label>
              <Input
                id="https_port"
                type="number"
                value={form?.https_port ?? 443}
                onChange={(e) =>
                  setForm((f) => ({ ...f, https_port: Number(e.target.value) }))
                }
              />
            </div>
          </div>
          <div className="space-y-2">
            <Label htmlFor="config_dir">Config directory</Label>
            <Input
              id="config_dir"
              value={form?.config_dir || ""}
              placeholder="/config/containerws/proxymanager"
              onChange={(e) =>
                setForm((f) => ({ ...f, config_dir: e.target.value }))
              }
            />
            <p className="text-xs text-muted-foreground">
              Generated nginx/traefik/fiber files only. Hosts and certificates
              stay in SQLite. Default is{" "}
              <code className="text-[11px]">/config/containerws/proxymanager</code>
              .
            </p>
          </div>
        </section>
      </div>
    </ContentLoader>
  )
}
