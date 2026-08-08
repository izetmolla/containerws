import { useEffect, useMemo, useRef, useState, type ComponentType } from "react"
import { useQuery } from "@tanstack/react-query"
import {
  Activity,
  ArrowDownToLine,
  ArrowUpFromLine,
  CircuitBoard,
  Cpu,
  HardDrive,
  MemoryStick,
  Network,
  RefreshCw,
  Server,
  Thermometer,
  Zap,
} from "lucide-react"

import ContentLoader from "@/components/content-loader"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { cn } from "@/lib/utils"
import { getRequestErrorMessage } from "@/lib/network"

import {
  DASHBOARD_FETCH_KEY,
  getDashboardMetrics,
  type DashboardMetrics,
  type DashboardTool,
  type GPUMetrics,
  type NetworkIface,
} from "./api"
import { ToolsPanel, ToolCard } from "./components/tools-panel"
import { isGpuTool } from "./components/tools-utils"
import { ProcessesPanel } from "./components/processes-panel"
import { UsageBar } from "./components/usage-bar"
import {
  autoRefreshInterval,
  useAutoRefreshMs,
} from "@/lib/auto-refresh"
import {
  appendGpuSample,
  gpuHistoryKey,
  type GpuHistorySample,
} from "./components/gpu-history"
import { UsageHistoryChart } from "./components/gpu-usage-chart"
import {
  clampPercent,
  formatPercent,
  formatUptime,
  metricCardClassName,
  usageTone,
} from "./lib/format"
import { asArray } from "@/lib/as-array"

function KpiCard({
  title,
  value,
  subtitle,
  icon: Icon,
  toneClass,
}: {
  title: string
  value: string
  subtitle: string
  icon: ComponentType<{ className?: string }>
  toneClass?: string
}) {
  return (
    <Card className={metricCardClassName()}>
      <CardHeader className="flex flex-row items-start justify-between gap-3 space-y-0 pb-2">
        <div className="space-y-1">
          <CardTitle className="text-xs font-medium tracking-wide text-muted-foreground uppercase">
            {title}
          </CardTitle>
          <div
            className={cn(
              "text-2xl font-semibold tracking-tight tabular-nums",
              toneClass
            )}
          >
            {value}
          </div>
        </div>
        <div className="rounded-lg bg-muted/70 p-2 text-muted-foreground">
          <Icon className="size-4" />
        </div>
      </CardHeader>
      <CardContent>
        <p className="text-xs text-muted-foreground">{subtitle}</p>
      </CardContent>
    </Card>
  )
}

function HostStrip({ data }: { data: DashboardMetrics }) {
  const kind = data.host.is_virtual_machine
    ? "Virtual machine"
    : data.host.is_containerized
      ? "Container"
      : "Bare metal"

  return (
    <div className="flex flex-wrap items-center gap-x-4 gap-y-2 rounded-xl border border-border/70 bg-card/40 px-4 py-3 text-sm">
      <div className="flex items-center gap-2 font-medium">
        <Server className="size-4 text-muted-foreground" />
        {data.host.hostname || "workspace"}
      </div>
      <Badge variant="outline">{kind}</Badge>
      {data.host.virtualization && data.host.virtualization !== "none" ? (
        <Badge variant="secondary">{data.host.virtualization}</Badge>
      ) : null}
      <span className="text-muted-foreground">
        {[data.host.distro, data.host.distro_version, data.host.arch]
          .filter(Boolean)
          .join(" · ") || data.host.os}
      </span>
      <span className="text-muted-foreground tabular-nums">
        {data.host.primary_ip || "—"}
      </span>
      <span className="ms-auto text-xs text-muted-foreground tabular-nums">
        Uptime {formatUptime(data.uptime_seconds)}
      </span>
    </div>
  )
}

function MemoryBreakdown({ data }: { data: DashboardMetrics }) {
  const total = data.memory.total_bytes || 1
  const cached = data.memory.cached_bytes
  const buffers = data.memory.buffers_bytes
  const free = data.memory.free_bytes
  const active = Math.max(0, total - free - buffers - cached)

  const activePct = clampPercent((active / total) * 100)
  const buffersPct = clampPercent((buffers / total) * 100)
  const cachedPct = clampPercent((cached / total) * 100)
  const freePct = Math.max(0, 100 - activePct - buffersPct - cachedPct)
  const usedPct = clampPercent(data.memory.used_percent)

  return (
    <Card className={metricCardClassName("h-full")}>
      <CardHeader>
        <CardTitle className="flex items-center gap-2 text-base">
          <MemoryStick className="size-4 text-muted-foreground" />
          Memory
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-5">
        <UsageBar
          label="RAM used"
          percent={usedPct}
          detail={`${data.memory.used_human} / ${data.memory.total_human}`}
        />
        <div className="flex h-3 overflow-hidden rounded-full bg-muted">
          <div
            className="bg-sky-500 transition-[width] duration-500"
            style={{ width: `${activePct}%` }}
            title="Active"
          />
          <div
            className="bg-cyan-500/80 transition-[width] duration-500"
            style={{ width: `${buffersPct}%` }}
            title="Buffers"
          />
          <div
            className="bg-teal-500/70 transition-[width] duration-500"
            style={{ width: `${cachedPct}%` }}
            title="Cached"
          />
          <div
            className="bg-muted-foreground/15 transition-[width] duration-500"
            style={{ width: `${freePct}%` }}
            title="Free"
          />
        </div>
        <div className="flex flex-wrap gap-3 text-[11px] text-muted-foreground">
          <span className="inline-flex items-center gap-1.5">
            <span className="size-2 rounded-full bg-sky-500" /> Active
          </span>
          <span className="inline-flex items-center gap-1.5">
            <span className="size-2 rounded-full bg-cyan-500/80" /> Buffers
          </span>
          <span className="inline-flex items-center gap-1.5">
            <span className="size-2 rounded-full bg-teal-500/70" /> Cached
          </span>
          <span className="inline-flex items-center gap-1.5">
            <span className="size-2 rounded-full bg-muted-foreground/30" /> Free
          </span>
        </div>
        <div className="grid grid-cols-2 gap-3 text-xs sm:grid-cols-4">
          <MemStat label="Used" value={data.memory.used_human} />
          <MemStat label="Available" value={data.memory.available_human} />
          <MemStat label="Cached" value={data.memory.cached_human} />
          <MemStat label="Buffers" value={data.memory.buffers_human} />
        </div>
        {data.memory.swap_total_bytes > 0 ? (
          <UsageBar
            label="Swap"
            percent={data.memory.swap_percent}
            detail={`${data.memory.swap_used_human} / ${data.memory.swap_total_human}`}
          />
        ) : (
          <p className="text-xs text-muted-foreground">Swap not configured</p>
        )}
      </CardContent>
    </Card>
  )
}

function MemStat({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-lg bg-muted/40 px-3 py-2">
      <div className="text-[11px] text-muted-foreground">{label}</div>
      <div className="mt-0.5 font-medium tabular-nums">{value || "—"}</div>
    </div>
  )
}

function CpuPanel({ data }: { data: DashboardMetrics }) {
  const cores = data.cpu.per_core_percent ?? []
  return (
    <Card className={metricCardClassName("h-full")}>
      <CardHeader>
        <CardTitle className="flex items-center gap-2 text-base">
          <Cpu className="size-4 text-muted-foreground" />
          CPU
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-5">
        <UsageBar
          label="Utilization"
          percent={data.cpu.usage_percent}
          detail={`${data.cpu.cores} cores`}
        />
        <div className="grid grid-cols-3 gap-3">
          <LoadStat label="Load 1m" value={data.cpu.load1} cores={data.cpu.cores} />
          <LoadStat label="Load 5m" value={data.cpu.load5} cores={data.cpu.cores} />
          <LoadStat label="Load 15m" value={data.cpu.load15} cores={data.cpu.cores} />
        </div>
        {cores.length > 0 ? (
          <div className="space-y-2">
            <div className="text-xs font-medium text-muted-foreground">
              Per-core usage
            </div>
            <div className="grid grid-cols-2 gap-2 sm:grid-cols-4">
              {cores.map((pct, i) => (
                <div key={i} className="space-y-1.5 rounded-lg bg-muted/40 px-2.5 py-2">
                  <div className="flex items-center justify-between text-[11px]">
                    <span className="text-muted-foreground">CPU{i}</span>
                    <span className={cn("tabular-nums font-medium", usageTone(pct))}>
                      {formatPercent(pct, 0)}
                    </span>
                  </div>
                  <div className="h-1.5 overflow-hidden rounded-full bg-muted">
                    <div
                      className={cn(
                        "h-full rounded-full transition-[width] duration-500",
                        pct >= 90
                          ? "bg-red-500"
                          : pct >= 75
                            ? "bg-amber-500"
                            : "bg-emerald-500"
                      )}
                      style={{ width: `${clampPercent(pct)}%` }}
                    />
                  </div>
                </div>
              ))}
            </div>
          </div>
        ) : null}
        {data.cpu.model ? (
          <p className="truncate text-xs text-muted-foreground" title={data.cpu.model}>
            {data.cpu.model}
          </p>
        ) : null}
      </CardContent>
    </Card>
  )
}

function LoadStat({
  label,
  value,
  cores,
}: {
  label: string
  value: number
  cores: number
}) {
  const pressure = cores > 0 ? (value / cores) * 100 : 0
  return (
    <div className="rounded-lg bg-muted/40 px-3 py-2">
      <div className="text-[11px] text-muted-foreground">{label}</div>
      <div className={cn("mt-0.5 text-lg font-semibold tabular-nums", usageTone(pressure))}>
        {Number.isFinite(value) ? value.toFixed(2) : "—"}
      </div>
    </div>
  )
}

function GpuPanel({
  data,
  tools,
}: {
  data: DashboardMetrics
  tools: DashboardTool[]
}) {
  const gpus = asArray(data.gpus)
  const [openedAt] = useState(() => Date.now())
  const [history, setHistory] = useState<Record<string, GpuHistorySample[]>>({})
  const lastStampRef = useRef("")

  const brands = useMemo(() => {
    const list = gpus
    const set = new Set(
      list.map((g) => (g.brand || "").toLowerCase()).filter(Boolean)
    )
    return set
  }, [gpus])

  const gpuTools = useMemo(() => {
    // Vendor-specific tools are offered only when that brand is present.
    // nvtop is universal and shown for any GPU (including "other").
    const brandForTool: Record<string, string[] | null> = {
      nvtop: null,
      "intel-gpu-tools": ["intel"],
      radeontop: ["amd"],
      gpustat: ["nvidia"],
    }
    return tools.filter((t) => {
      if (!isGpuTool(t) || t.installed) return false
      const required = brandForTool[t.key]
      if (required === undefined) {
        // Unknown GPU tool: show for any detected GPU.
        return true
      }
      if (required === null) return true
      return required.some((b) => brands.has(b))
    })
  }, [tools, brands])

  useEffect(() => {
    const list = gpus
    if (list.length === 0) return
    const stamp = data.collected_at || String(Date.now())
    if (stamp === lastStampRef.current) return
    lastStampRef.current = stamp

    const at = Date.now()
    setHistory((prev) => {
      let next = prev
      for (const gpu of list) {
        const key = gpuHistoryKey(gpu)
        next = appendGpuSample(next, key, {
          t: at,
          util: Number.isFinite(gpu.utilization_percent)
            ? gpu.utilization_percent
            : 0,
          mem: Number.isFinite(gpu.memory_percent) ? gpu.memory_percent : 0,
          memUtil: Number.isFinite(gpu.memory_util_percent)
            ? gpu.memory_util_percent
            : 0,
          clock: gpu.clock_graphics_mhz || 0,
          clockMax: gpu.clock_max_graphics_mhz || 0,
        })
      }
      return next === prev ? prev : { ...next }
    })
    // Sample once per metrics payload; gpus comes from that same snapshot.
    // eslint-disable-next-line react-hooks/exhaustive-deps -- collected_at is the poll signal
  }, [data.collected_at])

  const gpuList = gpus
  if (gpuList.length === 0) return null

  return (
    <Card className={metricCardClassName("w-full")}>
      <CardHeader className="pb-3">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <CardTitle className="flex items-center gap-2 text-base">
            <CircuitBoard className="size-4 text-muted-foreground" />
            Graphics
          </CardTitle>
          <div className="flex flex-wrap items-center gap-2">
            <Badge variant="outline" className="tabular-nums">
              {gpuList.length} {gpuList.length === 1 ? "GPU" : "GPUs"}
            </Badge>
            <span className="text-[11px] text-muted-foreground">
              Graphs since this page opened
            </span>
          </div>
        </div>
      </CardHeader>
      <CardContent className="space-y-6">
        {gpuList.map((gpu, idx) => {
          const key = gpuHistoryKey(gpu)
          return (
            <GpuCard
              key={key || idx}
              gpu={gpu}
              history={history[key] ?? []}
              openedAt={openedAt}
            />
          )
        })}

        {gpuTools.length > 0 ? (
          <div className="space-y-3 border-t border-border/50 pt-5">
            <div className="flex flex-wrap items-end justify-between gap-2">
              <div>
                <div className="text-sm font-medium tracking-tight">
                  GPU utilities
                </div>
                <p className="mt-0.5 text-xs text-muted-foreground">
                  Install CLI tools matched to this host&apos;s GPU vendor
                  (NVIDIA, AMD, Intel, and universal monitors).
                </p>
              </div>
              <Badge variant="outline">{gpuTools.length} to install</Badge>
            </div>
            <div className="grid gap-3 md:grid-cols-2">
              {gpuTools.map((tool) => (
                <ToolCard key={tool.key} tool={tool} />
              ))}
            </div>
          </div>
        ) : null}
      </CardContent>
    </Card>
  )
}

function GpuCard({
  gpu,
  history,
  openedAt,
}: {
  gpu: GPUMetrics
  history: GpuHistorySample[]
  openedAt: number
}) {
  const showUtil =
    gpu.brand === "nvidia" ||
    gpu.brand === "amd" ||
    gpu.brand === "intel" ||
    (Number.isFinite(gpu.utilization_percent) && gpu.utilization_percent > 0)
  const hasMem = gpu.memory_total_bytes > 0
  const showMemUtil =
    gpu.brand === "nvidia" ||
    (Number.isFinite(gpu.memory_util_percent) && gpu.memory_util_percent > 0)
  const connectors = gpu.connectors ?? []
  const connected = connectors.filter((c) => c.status === "connected")
  const chartId = gpuHistoryKey(gpu)
  const showClockChart =
    !hasMem && gpu.clock_max_graphics_mhz > 0 && gpu.clock_graphics_mhz > 0

  const brandLabel =
    gpu.brand === "nvidia"
      ? "NVIDIA"
      : gpu.brand === "amd"
        ? "AMD"
        : gpu.brand === "intel"
          ? "Intel"
          : gpu.vendor || "GPU"

  return (
    <div className="space-y-5 rounded-xl border border-border/60 bg-muted/20 p-4 sm:p-5">
      <div className="flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between">
        <div className="min-w-0 space-y-2">
          <div className="flex flex-wrap items-center gap-2">
            <Badge variant="secondary">{brandLabel}</Badge>
            {gpu.boot_vga ? <Badge variant="outline">Primary</Badge> : null}
            {gpu.driver ? (
              <Badge variant="outline" className="font-mono text-[10px]">
                {gpu.driver}
                {gpu.driver_version ? ` ${gpu.driver_version}` : ""}
              </Badge>
            ) : null}
          </div>
          <h3 className="text-lg font-semibold tracking-tight sm:text-xl">
            {gpu.name || "Graphics device"}
          </h3>
          <p className="truncate text-xs text-muted-foreground">
            {[
              gpu.pci_slot && `PCI ${gpu.pci_slot}`,
              gpu.vendor_id &&
                gpu.device_id &&
                `${gpu.vendor_id.toUpperCase()}:${gpu.device_id.toUpperCase()}`,
              gpu.drm_card,
              gpu.uuid,
            ]
              .filter(Boolean)
              .join(" · ") || "—"}
          </p>
        </div>

        <div className="flex flex-wrap gap-2 lg:justify-end">
          {gpu.temperature_c > 0 ? (
            <GpuHighlight
              icon={Thermometer}
              label="Temp"
              value={`${gpu.temperature_c.toFixed(0)}°C`}
            />
          ) : null}
          {gpu.power_draw_w > 0 ? (
            <GpuHighlight
              icon={Zap}
              label="Power"
              value={
                gpu.power_limit_w > 0
                  ? `${gpu.power_draw_w.toFixed(0)} / ${gpu.power_limit_w.toFixed(0)} W`
                  : `${gpu.power_draw_w.toFixed(1)} W`
              }
            />
          ) : null}
          {gpu.clock_graphics_mhz > 0 ? (
            <GpuHighlight
              icon={Cpu}
              label="Clock"
              value={
                gpu.clock_max_graphics_mhz > 0
                  ? `${gpu.clock_graphics_mhz} / ${gpu.clock_max_graphics_mhz} MHz`
                  : `${gpu.clock_graphics_mhz} MHz`
              }
            />
          ) : null}
        </div>
      </div>

      <div
        className={cn(
          "grid gap-4",
          hasMem || showClockChart ? "xl:grid-cols-2" : "grid-cols-1"
        )}
      >
        <UsageHistoryChart
          id={`${chartId}-util`}
          title="GPU utilization"
          samples={history}
          metric="util"
          color="rgb(34 197 94)"
          openedAt={openedAt}
        />
        {hasMem ? (
          <UsageHistoryChart
            id={`${chartId}-mem`}
            title="Dedicated GPU memory"
            samples={history}
            metric="mem"
            color="rgb(56 189 248)"
            openedAt={openedAt}
          />
        ) : showClockChart ? (
          <UsageHistoryChart
            id={`${chartId}-clock`}
            title="Graphics clock"
            samples={history}
            metric="clock"
            maxY={gpu.clock_max_graphics_mhz}
            unit=" MHz"
            color="rgb(251 191 36)"
            formatValue={(v) => `${Math.round(v)} MHz`}
            openedAt={openedAt}
          />
        ) : null}
      </div>

      {(showUtil || hasMem || showMemUtil) && (
        <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
          {showUtil ? (
            <UsageBar
              label="GPU utilization"
              percent={gpu.utilization_percent}
            />
          ) : null}
          {hasMem ? (
            <UsageBar
              label="VRAM"
              percent={gpu.memory_percent}
              detail={`${gpu.memory_used_human || "0 B"} / ${gpu.memory_total_human}`}
            />
          ) : null}
          {showMemUtil ? (
            <UsageBar label="Memory controller" percent={gpu.memory_util_percent} />
          ) : null}
        </div>
      )}

      <div className="grid grid-cols-2 gap-2 sm:grid-cols-3 md:grid-cols-4 xl:grid-cols-6">
        <GpuStat label="Vendor" value={gpu.vendor || "—"} />
        <GpuStat
          label="PCI ID"
          value={
            gpu.vendor_id && gpu.device_id
              ? `${gpu.vendor_id.toUpperCase()}:${gpu.device_id.toUpperCase()}`
              : "—"
          }
        />
        <GpuStat label="Driver" value={gpu.driver || "—"} />
        {gpu.driver_version ? (
          <GpuStat label="Driver version" value={gpu.driver_version} />
        ) : null}
        {gpu.cuda_version ? <GpuStat label="CUDA" value={gpu.cuda_version} /> : null}
        {gpu.compute_capability ? (
          <GpuStat label="Compute cap." value={gpu.compute_capability} />
        ) : null}
        {gpu.clock_graphics_mhz > 0 ? (
          <GpuStat
            label="Graphics clock"
            value={`${gpu.clock_graphics_mhz} MHz`}
          />
        ) : null}
        {gpu.clock_max_graphics_mhz > 0 ? (
          <GpuStat
            label="Max clock"
            value={`${gpu.clock_max_graphics_mhz} MHz`}
          />
        ) : null}
        {gpu.clock_min_graphics_mhz > 0 ? (
          <GpuStat
            label="Min clock"
            value={`${gpu.clock_min_graphics_mhz} MHz`}
          />
        ) : null}
        {gpu.clock_memory_mhz > 0 ? (
          <GpuStat label="Memory clock" value={`${gpu.clock_memory_mhz} MHz`} />
        ) : null}
        {gpu.memory_total_human ? (
          <GpuStat label="VRAM total" value={gpu.memory_total_human} />
        ) : null}
        {gpu.memory_used_human && gpu.memory_total_bytes > 0 ? (
          <GpuStat label="VRAM used" value={gpu.memory_used_human} />
        ) : null}
        {gpu.memory_free_human && gpu.memory_total_bytes > 0 ? (
          <GpuStat label="VRAM free" value={gpu.memory_free_human} />
        ) : null}
        {gpu.temperature_c > 0 ? (
          <GpuStat label="Temperature" value={`${gpu.temperature_c.toFixed(0)}°C`} />
        ) : null}
        {gpu.power_draw_w > 0 ? (
          <GpuStat label="Power draw" value={`${gpu.power_draw_w.toFixed(1)} W`} />
        ) : null}
        {gpu.power_limit_w > 0 ? (
          <GpuStat label="Power limit" value={`${gpu.power_limit_w.toFixed(0)} W`} />
        ) : null}
        {gpu.fan_speed_percent > 0 ? (
          <GpuStat
            label="Fan"
            value={`${gpu.fan_speed_percent.toFixed(0)}%`}
          />
        ) : null}
        {gpu.pci_slot ? <GpuStat label="PCI slot" value={gpu.pci_slot} /> : null}
        {gpu.drm_card ? <GpuStat label="DRM" value={gpu.drm_card} /> : null}
      </div>

      {connectors.length > 0 ? (
        <div className="space-y-2">
          <div className="text-xs font-medium text-muted-foreground">
            Displays
            {connected.length > 0
              ? ` · ${connected.length} connected`
              : " · none connected"}
          </div>
          <div className="flex flex-wrap gap-2">
            {connectors.map((c) => (
              <Badge
                key={c.name}
                variant={c.status === "connected" ? "default" : "outline"}
                className="font-normal"
              >
                {c.name}
                <span className="ms-1.5 opacity-70">
                  {c.status || "unknown"}
                </span>
              </Badge>
            ))}
          </div>
        </div>
      ) : null}
    </div>
  )
}

function GpuHighlight({
  icon: Icon,
  label,
  value,
}: {
  icon: ComponentType<{ className?: string }>
  label: string
  value: string
}) {
  return (
    <div className="rounded-lg border border-border/60 bg-card/60 px-3 py-2">
      <div className="flex items-center gap-1.5 text-[11px] text-muted-foreground">
        <Icon className="size-3.5" />
        {label}
      </div>
      <div className="mt-0.5 text-sm font-semibold tabular-nums">{value}</div>
    </div>
  )
}

function GpuStat({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-lg bg-muted/40 px-3 py-2">
      <div className="text-[11px] text-muted-foreground">{label}</div>
      <div className="mt-0.5 truncate font-medium tabular-nums" title={value}>
        {value}
      </div>
    </div>
  )
}

function DiskPanel({ data }: { data: DashboardMetrics }) {
  const disks = data.disks ?? []
  return (
    <Card className={metricCardClassName()}>
      <CardHeader>
        <CardTitle className="flex items-center gap-2 text-base">
          <HardDrive className="size-4 text-muted-foreground" />
          Storage
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-4">
        {disks.length === 0 ? (
          <p className="text-sm text-muted-foreground">No mounts reported</p>
        ) : (
          disks.map((disk) => (
            <div key={disk.mount} className="space-y-2">
              <div className="flex flex-wrap items-center justify-between gap-2">
                <div className="min-w-0">
                  <div className="font-medium tracking-tight">{disk.mount}</div>
                  <div className="truncate text-[11px] text-muted-foreground">
                    {[disk.device, disk.fstype].filter(Boolean).join(" · ") || "—"}
                  </div>
                </div>
                <div className="text-xs tabular-nums text-muted-foreground">
                  {disk.used_human} / {disk.total_human}
                </div>
              </div>
              <UsageBar label="Capacity" percent={disk.used_percent} />
            </div>
          ))
        )}
      </CardContent>
    </Card>
  )
}

function NetworkPanel({ data }: { data: DashboardMetrics }) {
  const ifaces = useMemo(() => {
    const list = data.network?.interfaces ?? []
    return [...list].sort((a, b) => {
      if (a.name === "lo") return 1
      if (b.name === "lo") return -1
      return b.rx_rate_bps + b.tx_rate_bps - (a.rx_rate_bps + a.tx_rate_bps)
    })
  }, [data.network?.interfaces])

  return (
    <Card className={metricCardClassName()}>
      <CardHeader>
        <CardTitle className="flex items-center gap-2 text-base">
          <Network className="size-4 text-muted-foreground" />
          Network
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="grid gap-3 sm:grid-cols-2">
          <div className="rounded-lg border border-border/60 bg-muted/30 px-3 py-3">
            <div className="flex items-center gap-2 text-xs text-muted-foreground">
              <ArrowDownToLine className="size-3.5" />
              Receive
            </div>
            <div className="mt-1 text-xl font-semibold tabular-nums">
              {data.network.rx_rate_human || "0 B/s"}
            </div>
          </div>
          <div className="rounded-lg border border-border/60 bg-muted/30 px-3 py-3">
            <div className="flex items-center gap-2 text-xs text-muted-foreground">
              <ArrowUpFromLine className="size-3.5" />
              Transmit
            </div>
            <div className="mt-1 text-xl font-semibold tabular-nums">
              {data.network.tx_rate_human || "0 B/s"}
            </div>
          </div>
        </div>

        <div className="overflow-x-auto rounded-lg border border-border/60">
          <table className="w-full min-w-[640px] border-collapse text-sm">
            <thead>
              <tr className="border-b border-border/60 bg-muted/30 text-left text-xs text-muted-foreground">
                <th className="px-3 py-2 font-medium">Interface</th>
                <th className="px-3 py-2 font-medium">RX rate</th>
                <th className="px-3 py-2 font-medium">TX rate</th>
                <th className="px-3 py-2 font-medium">RX total</th>
                <th className="px-3 py-2 font-medium">TX total</th>
                <th className="px-3 py-2 font-medium">Errors</th>
              </tr>
            </thead>
            <tbody>
              {ifaces.map((iface) => (
                <NetworkRow key={iface.name} iface={iface} />
              ))}
            </tbody>
          </table>
        </div>
      </CardContent>
    </Card>
  )
}

function NetworkRow({ iface }: { iface: NetworkIface }) {
  const errors = iface.rx_errors + iface.tx_errors
  const drops = iface.rx_dropped + iface.tx_dropped
  return (
    <tr className="border-b border-border/40 last:border-0 hover:bg-muted/20">
      <td className="px-3 py-2.5 font-medium">
        <div className="flex items-center gap-2">
          {iface.name}
          {iface.name === "lo" ? (
            <Badge variant="outline" className="h-5 text-[10px]">
              loopback
            </Badge>
          ) : null}
        </div>
      </td>
      <td className="px-3 py-2.5 tabular-nums text-muted-foreground">
        {iface.rx_rate_human || "0 B/s"}
      </td>
      <td className="px-3 py-2.5 tabular-nums text-muted-foreground">
        {iface.tx_rate_human || "0 B/s"}
      </td>
      <td className="px-3 py-2.5 tabular-nums">{iface.rx_human}</td>
      <td className="px-3 py-2.5 tabular-nums">{iface.tx_human}</td>
      <td className="px-3 py-2.5 tabular-nums text-muted-foreground">
        {errors + drops > 0 ? (
          <span className="text-amber-600 dark:text-amber-400">
            {errors} err · {drops} drop
          </span>
        ) : (
          "0"
        )}
      </td>
    </tr>
  )
}

export default function Dashboard() {
  const refreshMs = useAutoRefreshMs()

  const metricsQuery = useQuery({
    queryKey: [DASHBOARD_FETCH_KEY, "metrics"],
    queryFn: getDashboardMetrics,
    refetchInterval: autoRefreshInterval(refreshMs),
    refetchIntervalInBackground: false,
  })

  const data = metricsQuery.data?.data
  const tools: DashboardTool[] = metricsQuery.data?.tools ?? []
  const primaryDisk = data?.disks?.[0]

  if (metricsQuery.isLoading && !data) {
    return <ContentLoader />
  }

  if (metricsQuery.isError && !data) {
    return (
      <div className="rounded-xl border border-destructive/30 bg-destructive/5 p-6 text-sm">
        <p className="font-medium text-destructive">Failed to load metrics</p>
        <p className="mt-1 text-muted-foreground">
          {getRequestErrorMessage(metricsQuery.error, "Unable to fetch VM resources")}
        </p>
        <Button
          className="mt-4"
          variant="outline"
          size="sm"
          onClick={() => void metricsQuery.refetch()}
        >
          <RefreshCw data-icon="inline-start" />
          Retry
        </Button>
      </div>
    )
  }

  if (!data) return null

  const collectedAt = data.collected_at
    ? new Date(data.collected_at).toLocaleTimeString()
    : "—"

  return (
    <div className="flex w-full flex-col gap-4 pb-6">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">Dashboard</h1>
          <p className="mt-1 text-sm text-muted-foreground">
            Live VM resource analytics — CPU, memory, GPU, storage, and network.
          </p>
        </div>
        <div className="flex flex-wrap items-center gap-3">
          <div className="flex items-center gap-2 text-xs text-muted-foreground">
            <span
              className={cn(
                "size-1.5 rounded-full",
                refreshMs > 0 ? "animate-pulse bg-emerald-500" : "bg-muted-foreground"
              )}
              aria-hidden
            />
            Updated {collectedAt}
          </div>
          <Button
            variant="outline"
            size="sm"
            onClick={() => void metricsQuery.refetch()}
            disabled={metricsQuery.isFetching}
          >
            <RefreshCw
              className={cn(metricsQuery.isFetching && "animate-spin")}
              data-icon="inline-start"
            />
            Refresh
          </Button>
        </div>
      </div>

      <HostStrip data={data} />

      <ToolsPanel tools={tools} />

      <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
        <KpiCard
          title="CPU"
          value={formatPercent(data.cpu.usage_percent)}
          subtitle={`${data.cpu.cores} cores · load ${data.cpu.load1.toFixed(2)}`}
          icon={Cpu}
          toneClass={usageTone(data.cpu.usage_percent)}
        />
        <KpiCard
          title="Memory"
          value={formatPercent(data.memory.used_percent)}
          subtitle={`${data.memory.used_human} of ${data.memory.total_human}`}
          icon={MemoryStick}
          toneClass={usageTone(data.memory.used_percent)}
        />
        <KpiCard
          title="Disk"
          value={primaryDisk ? formatPercent(primaryDisk.used_percent) : "—"}
          subtitle={
            primaryDisk
              ? `${primaryDisk.mount} · ${primaryDisk.used_human} / ${primaryDisk.total_human}`
              : "No mount data"
          }
          icon={HardDrive}
          toneClass={primaryDisk ? usageTone(primaryDisk.used_percent) : undefined}
        />
        <KpiCard
          title="Network"
          value={data.network.rx_rate_human || "0 B/s"}
          subtitle={`↓ ${data.network.rx_rate_human || "0 B/s"} · ↑ ${data.network.tx_rate_human || "0 B/s"}`}
          icon={Activity}
        />
      </div>

      <div className="grid gap-4 xl:grid-cols-2">
        <CpuPanel data={data} />
        <MemoryBreakdown data={data} />
      </div>

      {(data.gpus?.length ?? 0) > 0 ? (
        <GpuPanel data={data} tools={tools} />
      ) : null}

      <div className="grid gap-4 xl:grid-cols-2">
        <DiskPanel data={data} />
        <NetworkPanel data={data} />
      </div>

      <ProcessesPanel />
    </div>
  )
}
