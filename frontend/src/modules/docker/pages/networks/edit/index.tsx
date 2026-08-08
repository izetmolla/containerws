import { useMemo, useState, type ReactNode } from "react"
import { Link, useNavigate, useSearchParams } from "react-router"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { Plus, RefreshCw, Trash2 } from "lucide-react"
import { toast } from "sonner"

import ContentLoader from "@/components/content-loader"
import { Button } from "@/components/ui/button"
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { ReactSelect } from "@/components/ui/reactselect"
import { Switch } from "@/components/ui/switch"
import { toastRequestError } from "@/lib/network"
import { cn } from "@/lib/utils"

import { EngineBanner } from "../../_shared/engine-status"
import { EnvironmentSelector } from "../../_shared/environment-selector"
import {
  createNetwork,
  disconnectNetworkContainer,
  DOCKER_NETWORKS_KEY,
  getNetwork,
  type CreateNetworkInput,
  type NetworkContainer,
  type NetworkDetail,
} from "../api"

type Option = { value: string; label: string }
type KvRow = { key: string; value: string }

const DRIVER_OPTIONS: Option[] = [
  { value: "bridge", label: "bridge" },
  { value: "overlay", label: "overlay" },
  { value: "macvlan", label: "macvlan" },
  { value: "ipvlan", label: "ipvlan" },
  { value: "host", label: "host" },
  { value: "none", label: "none" },
]

function emptyKv(): KvRow {
  return { key: "", value: "" }
}

function kvFromRecord(rec?: Record<string, string> | null): KvRow[] {
  if (!rec) return []
  return Object.entries(rec).map(([key, value]) => ({ key, value }))
}

function recordFromKv(rows: KvRow[]) {
  const out: Record<string, string> = {}
  for (const r of rows) {
    if (!r.key.trim()) continue
    out[r.key.trim()] = r.value
  }
  return out
}

export default function NetworkEditPage() {
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const [params] = useSearchParams()
  const editId = params.get("id")?.trim() || ""
  const isEdit = Boolean(editId)

  const [name, setName] = useState("")
  const [driver, setDriver] = useState("bridge")
  const [driverOpts, setDriverOpts] = useState<KvRow[]>([])
  const [v4Subnet, setV4Subnet] = useState("")
  const [v4Gateway, setV4Gateway] = useState("")
  const [v4Range, setV4Range] = useState("")
  const [v4Excluded, setV4Excluded] = useState<string[]>([])
  const [enableIPv6, setEnableIPv6] = useState(false)
  const [v6Subnet, setV6Subnet] = useState("")
  const [v6Gateway, setV6Gateway] = useState("")
  const [v6Range, setV6Range] = useState("")
  const [v6Excluded, setV6Excluded] = useState<string[]>([])
  const [labels, setLabels] = useState<KvRow[]>([])
  const [internal, setInternal] = useState(false)
  const [attachable, setAttachable] = useState(false)
  const [nameError, setNameError] = useState(false)

  const detailQuery = useQuery({
    queryKey: [DOCKER_NETWORKS_KEY, "edit", editId],
    queryFn: () => getNetwork(editId),
    enabled: isEdit,
  })

  const applyDetail = (d: NetworkDetail) => {
    setName(d.name || "")
    setDriver(d.driver || "bridge")
    setDriverOpts(kvFromRecord(d.options))
    setInternal(Boolean(d.internal))
    setAttachable(Boolean(d.attachable))
    setEnableIPv6(Boolean(d.enable_ipv6))
    setLabels(kvFromRecord(d.labels))
    setV4Subnet(d.ipv4?.subnet || "")
    setV4Gateway(d.ipv4?.gateway || "")
    setV4Range(d.ipv4?.ip_range || "")
    setV4Excluded(d.ipv4?.excluded_ips || [])
    setV6Subnet(d.ipv6?.subnet || "")
    setV6Gateway(d.ipv6?.gateway || "")
    setV6Range(d.ipv6?.ip_range || "")
    setV6Excluded(d.ipv6?.excluded_ips || [])
  }

  const detailData = isEdit ? detailQuery.data?.data : undefined
  const [prevDetailData, setPrevDetailData] = useState(detailData)
  if (detailData !== prevDetailData) {
    setPrevDetailData(detailData)
    if (detailData) applyDetail(detailData)
  }

  const containers = detailQuery.data?.data?.containers ?? []

  const saveMutation = useMutation({
    mutationFn: (body: CreateNetworkInput) => createNetwork(body),
    onSuccess: (res) => {
      toast.success(res.message || "Network created")
      void queryClient.invalidateQueries({ queryKey: [DOCKER_NETWORKS_KEY] })
      const id = res.data?.id
      if (id) navigate(`/docker/networks/edit?id=${encodeURIComponent(id)}`)
      else navigate("/docker/networks")
    },
    onError: (err) => toastRequestError(err, "Create failed"),
  })

  const disconnectMutation = useMutation({
    mutationFn: (containerId: string) =>
      disconnectNetworkContainer(editId, containerId, true),
    onSuccess: (res) => {
      toast.success(res.message || "Disconnected")
      void queryClient.invalidateQueries({
        queryKey: [DOCKER_NETWORKS_KEY, "edit", editId],
      })
    },
    onError: (err) => toastRequestError(err, "Disconnect failed"),
  })

  const buildBody = (): CreateNetworkInput | null => {
    const n = name.trim()
    if (!n) {
      setNameError(true)
      toast.message("Name is required")
      return null
    }
    setNameError(false)
    const body: CreateNetworkInput = {
      name: n,
      driver,
      options: recordFromKv(driverOpts),
      internal,
      attachable,
      enable_ipv6: enableIPv6,
      labels: recordFromKv(labels),
    }
    if (v4Subnet.trim()) {
      body.ipv4 = {
        subnet: v4Subnet.trim(),
        gateway: v4Gateway.trim() || undefined,
        ip_range: v4Range.trim() || undefined,
        excluded_ips: v4Excluded.map((s) => s.trim()).filter(Boolean),
      }
    }
    if (enableIPv6 && v6Subnet.trim()) {
      body.ipv6 = {
        subnet: v6Subnet.trim(),
        gateway: v6Gateway.trim() || undefined,
        ip_range: v6Range.trim() || undefined,
        excluded_ips: v6Excluded.map((s) => s.trim()).filter(Boolean),
      }
    }
    return body
  }

  const onCreate = () => {
    const body = buildBody()
    if (!body) return
    saveMutation.mutate(body)
  }

  const readOnly = isEdit
  const protectedNet = useMemo(
    () => ["bridge", "host", "none"].includes(name),
    [name]
  )

  return (
    <ContentLoader
      title={isEdit ? "Network" : "Create network"}
      breadcrumb={[
        { label: "Docker", to: "/docker" },
        { label: "Networks", to: "/docker/networks" },
        { label: isEdit ? name || "Edit" : "Add network" },
      ]}
      isLoading={isEdit && detailQuery.isLoading}
      error={isEdit ? detailQuery.error : undefined}
      rightComponent={
        <div className="flex flex-wrap items-center gap-2">
          <EnvironmentSelector />
          <Button
            size="sm"
            variant="outline"
            onClick={() =>
              isEdit ? detailQuery.refetch() : window.location.reload()
            }
          >
            <RefreshCw className="size-3.5" />
          </Button>
        </div>
      }
    >
      <div className="flex w-full min-w-0 flex-col gap-6">
        <EngineBanner />

        {isEdit && protectedNet ? (
          <p className="rounded-lg border border-amber-500/30 bg-amber-500/10 px-4 py-3 text-sm text-amber-900 dark:text-amber-200">
            Built-in networks cannot be modified. You can still review connected
            containers below.
          </p>
        ) : null}

        {isEdit ? (
          <p className="text-sm text-muted-foreground">
            Docker network settings are immutable after creation. This page shows
            the current configuration and containers attached to this network.
          </p>
        ) : null}

        <Card className="gap-0 py-0">
          <CardHeader className="border-b py-5">
            <CardTitle>Name</CardTitle>
          </CardHeader>
          <CardContent className="space-y-6 py-6">
            <Field label="Name">
              <Input
                value={name}
                disabled={readOnly}
                onChange={(e) => {
                  setName(e.target.value)
                  if (e.target.value.trim()) setNameError(false)
                }}
                placeholder="e.g. myNetwork"
                className={cn(nameError && "border-destructive")}
              />
              {nameError ? (
                <p className="text-xs text-destructive">Name is required</p>
              ) : null}
            </Field>
          </CardContent>
        </Card>

        <Card className="gap-0 py-0">
          <CardHeader className="border-b py-5">
            <CardTitle>Driver configuration</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4 py-6">
            <Field label="Driver">
              <ReactSelect<Option, false>
                size="sm"
                options={DRIVER_OPTIONS}
                value={driver}
                isDisabled={readOnly}
                onValueChange={(v) => v && setDriver(v)}
              />
            </Field>
            <KvEditor
              label="Driver options"
              rows={driverOpts}
              onChange={setDriverOpts}
              addLabel="Add driver option"
              disabled={readOnly}
              keyPlaceholder="com.docker.network.bridge.name"
              valuePlaceholder="value"
            />
          </CardContent>
        </Card>

        <Card className="gap-0 py-0">
          <CardHeader className="border-b py-5">
            <CardTitle>IPV4 Network configuration</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4 py-6">
            <div className="grid gap-4 sm:grid-cols-3">
              <Field label="Subnet">
                <Input
                  value={v4Subnet}
                  disabled={readOnly}
                  onChange={(e) => setV4Subnet(e.target.value)}
                  placeholder="e.g. 172.20.0.0/16"
                  className="font-mono text-sm"
                />
              </Field>
              <Field label="Gateway">
                <Input
                  value={v4Gateway}
                  disabled={readOnly}
                  onChange={(e) => setV4Gateway(e.target.value)}
                  placeholder="e.g. 172.20.10.11"
                  className="font-mono text-sm"
                />
              </Field>
              <Field label="IP range">
                <Input
                  value={v4Range}
                  disabled={readOnly}
                  onChange={(e) => setV4Range(e.target.value)}
                  placeholder="e.g. 172.20.10.128/25"
                  className="font-mono text-sm"
                />
              </Field>
            </div>
            <StringListEditor
              label="Excluded IPs"
              rows={v4Excluded}
              onChange={setV4Excluded}
              addLabel="Add excluded IP"
              disabled={readOnly}
              placeholder="e.g. 172.20.10.5"
            />
          </CardContent>
        </Card>

        <Card className="gap-0 py-0">
          <CardHeader className="border-b py-5">
            <div className="flex flex-wrap items-center justify-between gap-3">
              <CardTitle>IPV6 Network configuration</CardTitle>
              {!readOnly ? (
                <div className="flex items-center gap-2">
                  <Label htmlFor="enable-ipv6" className="text-sm font-normal">
                    Enable IPv6
                  </Label>
                  <Switch
                    id="enable-ipv6"
                    checked={enableIPv6}
                    onCheckedChange={setEnableIPv6}
                  />
                </div>
              ) : enableIPv6 ? (
                <span className="text-xs text-muted-foreground">IPv6 enabled</span>
              ) : null}
            </div>
          </CardHeader>
          {(enableIPv6 || (readOnly && (v6Subnet || v6Gateway))) && (
            <CardContent className="space-y-4 py-6">
              <div className="grid gap-4 sm:grid-cols-3">
                <Field label="Subnet">
                  <Input
                    value={v6Subnet}
                    disabled={readOnly}
                    onChange={(e) => setV6Subnet(e.target.value)}
                    placeholder="e.g. 2001:db8::/48"
                    className="font-mono text-sm"
                  />
                </Field>
                <Field label="Gateway">
                  <Input
                    value={v6Gateway}
                    disabled={readOnly}
                    onChange={(e) => setV6Gateway(e.target.value)}
                    placeholder="e.g. 2001:db8::1"
                    className="font-mono text-sm"
                  />
                </Field>
                <Field label="IP range">
                  <Input
                    value={v6Range}
                    disabled={readOnly}
                    onChange={(e) => setV6Range(e.target.value)}
                    placeholder="e.g. 2001:db8::/64"
                    className="font-mono text-sm"
                  />
                </Field>
              </div>
              <StringListEditor
                label="Excluded IPs"
                rows={v6Excluded}
                onChange={setV6Excluded}
                addLabel="Add excluded IP"
                disabled={readOnly}
                placeholder="e.g. 2001:db8::10"
              />
            </CardContent>
          )}
        </Card>

        <Card className="gap-0 py-0">
          <CardHeader className="border-b py-5">
            <CardTitle>Advanced configuration</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4 py-6">
            <KvEditor
              label="Labels"
              rows={labels}
              onChange={setLabels}
              addLabel="Add label"
              disabled={readOnly}
              keyPlaceholder="com.example.key"
              valuePlaceholder="value"
            />
            <div className="flex items-center justify-between gap-4 rounded-md border px-3 py-2.5">
              <div>
                <Label htmlFor="isolated" className="cursor-pointer">
                  Isolated network
                </Label>
                <p className="text-xs text-muted-foreground">
                  Containers on this network cannot communicate with external
                  networks.
                </p>
              </div>
              <Switch
                id="isolated"
                checked={internal}
                disabled={readOnly}
                onCheckedChange={setInternal}
              />
            </div>
            <div className="flex items-center justify-between gap-4 rounded-md border px-3 py-2.5">
              <div>
                <Label htmlFor="attachable" className="cursor-pointer">
                  Enable manual container attachment
                </Label>
                <p className="text-xs text-muted-foreground">
                  Allow containers to be manually attached to this network.
                </p>
              </div>
              <Switch
                id="attachable"
                checked={attachable}
                disabled={readOnly}
                onCheckedChange={setAttachable}
              />
            </div>
          </CardContent>
          {!isEdit ? (
            <CardFooter className="justify-start gap-3 border-t py-4">
              <Button
                size="lg"
                disabled={saveMutation.isPending || !name.trim()}
                onClick={onCreate}
              >
                {saveMutation.isPending
                  ? "Creating…"
                  : "Create the network"}
              </Button>
              <Button
                size="lg"
                variant="outline"
                onClick={() => navigate("/docker/networks")}
              >
                Cancel
              </Button>
            </CardFooter>
          ) : null}
        </Card>

        {isEdit ? (
          <Card className="gap-0 py-0">
            <CardHeader className="border-b py-5">
              <CardTitle>Containers in this network</CardTitle>
              <CardDescription>
                {containers.length
                  ? `${containers.length} container${containers.length === 1 ? "" : "s"} connected`
                  : "No containers are currently attached to this network."}
              </CardDescription>
            </CardHeader>
            <CardContent className="py-0">
              {containers.length ? (
                <div className="overflow-x-auto">
                  <table className="w-full min-w-[640px] border-collapse text-sm">
                    <thead>
                      <tr className="border-b bg-muted/30 text-left text-[11px] font-medium tracking-wide text-muted-foreground uppercase">
                        <th className="px-4 py-2.5">Name</th>
                        <th className="px-4 py-2.5">IPv4</th>
                        <th className="px-4 py-2.5">IPv6</th>
                        <th className="px-4 py-2.5">MAC</th>
                        <th className="px-4 py-2.5 w-28" />
                      </tr>
                    </thead>
                    <tbody>
                      {containers.map((c: NetworkContainer) => (
                        <tr
                          key={c.id}
                          className="border-b border-border/50 last:border-0"
                        >
                          <td className="px-4 py-2.5">
                            <Link
                              to={`/docker/containers/${c.id}`}
                              className="font-medium text-sky-600 hover:underline dark:text-sky-400"
                            >
                              {c.name}
                            </Link>
                          </td>
                          <td className="px-4 py-2.5 font-mono text-xs text-muted-foreground">
                            {c.ipv4_address || "—"}
                          </td>
                          <td className="px-4 py-2.5 font-mono text-xs text-muted-foreground">
                            {c.ipv6_address || "—"}
                          </td>
                          <td className="px-4 py-2.5 font-mono text-xs text-muted-foreground">
                            {c.mac_address || "—"}
                          </td>
                          <td className="px-4 py-2.5 text-end">
                            <Button
                              size="sm"
                              variant="outline"
                              disabled={
                                disconnectMutation.isPending || protectedNet
                              }
                              onClick={() =>
                                disconnectMutation.mutate(c.id)
                              }
                            >
                              Disconnect
                            </Button>
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              ) : (
                <p className="px-4 py-8 text-center text-sm text-muted-foreground">
                  No containers on this network.
                </p>
              )}
            </CardContent>
          </Card>
        ) : null}
      </div>
    </ContentLoader>
  )
}

function Field({
  label,
  children,
}: {
  label: string
  children: ReactNode
}) {
  return (
    <div className="grid gap-1.5">
      <Label className="text-sm">{label}</Label>
      {children}
    </div>
  )
}

function KvEditor({
  label,
  rows,
  onChange,
  addLabel,
  disabled,
  keyPlaceholder = "name",
  valuePlaceholder = "value",
}: {
  label: string
  rows: KvRow[]
  onChange: (rows: KvRow[]) => void
  addLabel: string
  disabled?: boolean
  keyPlaceholder?: string
  valuePlaceholder?: string
}) {
  return (
    <div className="space-y-2">
      <p className="text-sm font-medium">{label}</p>
      {rows.map((row, idx) => (
        <div key={idx} className="grid grid-cols-[1fr_1fr_auto] gap-2">
          <Input
            placeholder={keyPlaceholder}
            value={row.key}
            disabled={disabled}
            onChange={(e) =>
              onChange(
                rows.map((r, i) =>
                  i === idx ? { ...r, key: e.target.value } : r
                )
              )
            }
          />
          <Input
            placeholder={valuePlaceholder}
            value={row.value}
            disabled={disabled}
            onChange={(e) =>
              onChange(
                rows.map((r, i) =>
                  i === idx ? { ...r, value: e.target.value } : r
                )
              )
            }
          />
          {!disabled ? (
            <Button
              size="icon-sm"
              variant="ghost"
              onClick={() => onChange(rows.filter((_, i) => i !== idx))}
            >
              <Trash2 className="size-3.5" />
            </Button>
          ) : (
            <span />
          )}
        </div>
      ))}
      {!disabled ? (
        <Button
          size="sm"
          variant="link"
          className="h-auto px-0"
          onClick={() => onChange([...rows, emptyKv()])}
        >
          <Plus data-icon="inline-start" />
          {addLabel}
        </Button>
      ) : null}
    </div>
  )
}

function StringListEditor({
  label,
  rows,
  onChange,
  addLabel,
  disabled,
  placeholder,
}: {
  label: string
  rows: string[]
  onChange: (rows: string[]) => void
  addLabel: string
  disabled?: boolean
  placeholder?: string
}) {
  return (
    <div className="space-y-2">
      <p className="text-sm font-medium">{label}</p>
      {rows.map((row, idx) => (
        <div key={idx} className="flex gap-2">
          <Input
            value={row}
            disabled={disabled}
            placeholder={placeholder}
            className="font-mono text-sm"
            onChange={(e) =>
              onChange(rows.map((r, i) => (i === idx ? e.target.value : r)))
            }
          />
          {!disabled ? (
            <Button
              size="icon-sm"
              variant="ghost"
              onClick={() => onChange(rows.filter((_, i) => i !== idx))}
            >
              <Trash2 className="size-3.5" />
            </Button>
          ) : null}
        </div>
      ))}
      {!disabled ? (
        <Button
          size="sm"
          variant="link"
          className="h-auto px-0"
          onClick={() => onChange([...rows, ""])}
        >
          <Plus data-icon="inline-start" />
          {addLabel}
        </Button>
      ) : null}
    </div>
  )
}
