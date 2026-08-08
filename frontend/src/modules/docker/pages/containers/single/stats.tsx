import { useEffect, useMemo, useState, type ReactNode } from "react"
import { useQuery } from "@tanstack/react-query"
import { useOutletContext } from "react-router"
import {
  type ColumnDef,
} from "@tanstack/react-table"
import { Activity, Cpu, HardDrive, Info, MemoryStick } from "lucide-react"

import { useContentLoader } from "@/components/content-loader/context"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { DataTable } from "@/components/ui/data-table"
import { Input } from "@/components/ui/input"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"

import { formatBytes } from "../../_shared/engine-format"
import {
  getContainerStats,
  getContainerTop,
  type ContainerStats,
} from "../list/api"
import type { ContainerOutletContext } from "./layout"
import { LiveLineChart, type ChartSeries } from "./live-line-chart"

const REFRESH_OPTIONS = [
  { value: "1000", label: "1s" },
  { value: "3000", label: "3s" },
  { value: "5000", label: "5s" },
  { value: "10000", label: "10s" },
  { value: "30000", label: "30s" },
] as const

const COLORS = {
  memory: "#3b82f6",
  cpu: "#38bdf8",
  rx: "#3b82f6",
  tx: "#ef4444",
  read: "#3b82f6",
  write: "#ef4444",
} as const

type Sample = {
  t: number
  cpu: number
  memory: number
  memoryLimit: number
  networks: Record<string, { rx: number; tx: number }>
  blkRead: number
  blkWrite: number
}

type ProcessRow = {
  id: string
  cells: Record<string, string>
}

function formatPercent(n: number) {
  if (!Number.isFinite(n)) return "0%"
  if (n < 10) return `${n.toFixed(1)}%`
  return `${n.toFixed(0)}%`
}

function formatAxisBytes(n: number) {
  if (n <= 0) return "0 B"
  return formatBytes(n)
}

function networkSeriesFromSamples(samples: Sample[]): ChartSeries[] {
  const names = new Set<string>()
  for (const s of samples) {
    for (const name of Object.keys(s.networks)) names.add(name)
  }
  const ordered = [...names].sort()
  const series: ChartSeries[] = []
  ordered.forEach((name, i) => {
    const hue = (i * 47) % 360
    series.push({
      key: `rx-${name}`,
      label: `RX on ${name}`,
      color: i === 0 ? COLORS.rx : `hsl(${hue} 70% 55%)`,
      values: samples.map((s) => s.networks[name]?.rx ?? 0),
    })
    series.push({
      key: `tx-${name}`,
      label: `TX on ${name}`,
      color: i === 0 ? COLORS.tx : `hsl(${(hue + 30) % 360} 70% 50%)`,
      values: samples.map((s) => s.networks[name]?.tx ?? 0),
    })
  })
  if (series.length === 0) {
    return [
      {
        key: "rx",
        label: "RX",
        color: COLORS.rx,
        values: samples.map(() => 0),
      },
      {
        key: "tx",
        label: "TX",
        color: COLORS.tx,
        values: samples.map(() => 0),
      },
    ]
  }
  return series
}

function sampleFromStats(data: ContainerStats): Sample {
  const networks: Sample["networks"] = {}
  if (data.networks?.length) {
    for (const n of data.networks) {
      networks[n.name] = { rx: n.rx_bytes, tx: n.tx_bytes }
    }
  } else {
    networks.eth0 = {
      rx: data.network_rx ?? 0,
      tx: data.network_tx ?? 0,
    }
  }
  return {
    t: Date.now(),
    cpu: data.cpu_percent ?? 0,
    memory: data.memory_usage ?? 0,
    memoryLimit: data.memory_limit ?? 0,
    networks,
    blkRead: data.blkio_read ?? 0,
    blkWrite: data.blkio_write ?? 0,
  }
}

export default function ContainerStatsPage() {
  const { id, container } = useOutletContext<ContainerOutletContext>()
  const { setTitle } = useContentLoader()
  const [refreshMs, setRefreshMs] = useState(5_000)
  const [history, setHistory] = useState<{
    id: string
    lastAt: number
    samples: Sample[]
  }>({ id, lastAt: 0, samples: [] })
  const [processSearch, setProcessSearch] = useState("")

  const enabled = container.state === "running"
  const displayName = container.name || container.short_id || id

  useEffect(() => {
    setTitle("Container statistics")
  }, [setTitle])

  const statsQuery = useQuery({
    queryKey: ["docker-container-stats", id],
    queryFn: () => getContainerStats(id),
    enabled,
    refetchInterval: enabled ? refreshMs : false,
    refetchIntervalInBackground: false,
  })

  const topQuery = useQuery({
    queryKey: ["docker-container-top", id],
    queryFn: () => getContainerTop(id),
    enabled,
    refetchInterval: enabled ? refreshMs : false,
    refetchIntervalInBackground: false,
  })

  // Reset / append during render when the container or poll result changes.
  if (history.id !== id) {
    setHistory({ id, lastAt: 0, samples: [] })
  } else if (
    statsQuery.isSuccess &&
    statsQuery.dataUpdatedAt > history.lastAt
  ) {
    setHistory({
      id,
      lastAt: statsQuery.dataUpdatedAt,
      samples: [...history.samples, sampleFromStats(statsQuery.data.data)],
    })
  }

  const samples = useMemo(
    () => (history.id === id ? history.samples : []),
    [history.id, history.samples, id]
  )
  const latest = samples.at(-1)
  const networkTotal = latest
    ? Object.values(latest.networks).reduce(
        (sum, n) => sum + n.rx + n.tx,
        0
      )
    : 0
  const ioTotal = latest ? latest.blkRead + latest.blkWrite : 0

  const times = useMemo(() => samples.map((s) => s.t), [samples])
  const memoryLimit = latest?.memoryLimit ?? 0
  const cpuMax = useMemo(() => {
    let max = 1
    for (const s of samples) {
      if (s.cpu > max) max = s.cpu
    }
    return Math.max(1, Math.ceil(max * 1.25 * 10) / 10)
  }, [samples])

  const cpuSpark = useMemo(() => samples.map((s) => s.cpu), [samples])
  const memorySpark = useMemo(() => samples.map((s) => s.memory), [samples])
  const networkSpark = useMemo(
    () =>
      samples.map((s) =>
        Object.values(s.networks).reduce((sum, n) => sum + n.rx + n.tx, 0)
      ),
    [samples]
  )
  const ioSpark = useMemo(
    () => samples.map((s) => s.blkRead + s.blkWrite),
    [samples]
  )

  const memorySeries: ChartSeries[] = [
    {
      key: "memory",
      label: "Memory",
      color: COLORS.memory,
      values: samples.map((s) => s.memory),
    },
  ]
  const cpuSeries: ChartSeries[] = [
    {
      key: "cpu",
      label: "CPU",
      color: COLORS.cpu,
      values: samples.map((s) => s.cpu),
    },
  ]
  const networkSeries = networkSeriesFromSamples(samples)
  const ioSeries: ChartSeries[] = [
    {
      key: "read",
      label: "Read (Aggregate)",
      color: COLORS.read,
      values: samples.map((s) => s.blkRead),
    },
    {
      key: "write",
      label: "Write (Aggregate)",
      color: COLORS.write,
      values: samples.map((s) => s.blkWrite),
    },
  ]

  const titles = useMemo(
    () => topQuery.data?.data.titles ?? [],
    [topQuery.data?.data.titles]
  )
  const processRows: ProcessRow[] = useMemo(() => {
    const processes = topQuery.data?.data.processes ?? []
    return processes.map((row, idx) => {
      const cells: Record<string, string> = {}
      titles.forEach((title, i) => {
        cells[title] = row[i] ?? ""
      })
      return { id: `${row[1] ?? idx}-${idx}`, cells }
    })
  }, [topQuery.data?.data.processes, titles])

  const filteredRows = useMemo(() => {
    const q = processSearch.trim().toLowerCase()
    if (!q) return processRows
    return processRows.filter((row) =>
      Object.values(row.cells).some((v) => v.toLowerCase().includes(q))
    )
  }, [processRows, processSearch])

  const columns = useMemo<ColumnDef<ProcessRow>[]>(() => {
    const keys = titles.length ? titles : ["CMD"]
    return keys.map((title) => ({
      id: title,
      accessorFn: (row: ProcessRow) => row.cells[title] ?? "",
      header: title,
      cell: ({ getValue }) => {
        const v = String(getValue() ?? "")
        if (title === "CMD") {
          return (
            <span className="max-w-[28rem] truncate font-mono text-xs" title={v}>
              {v || "—"}
            </span>
          )
        }
        return <span className="tabular-nums">{v || "—"}</span>
      },
    }))
  }, [titles])

  if (!enabled) {
    return (
      <div className="rounded-xl border border-dashed px-6 py-12 text-center">
        <p className="text-sm font-medium">Stats unavailable</p>
        <p className="mt-1 text-sm text-muted-foreground">
          Start the container to stream CPU, memory, network, and process
          metrics. Network chart values are cumulative bytes.
        </p>
      </div>
    )
  }

  return (
    <div className="flex w-full min-w-0 flex-col gap-6">
      <Card>
        <CardHeader className="flex flex-row flex-wrap items-start justify-between gap-4 space-y-0 pb-3">
          <div className="space-y-1.5">
            <CardTitle className="flex items-center gap-2 text-base">
              <Info className="size-4 text-muted-foreground" />
              About statistics
            </CardTitle>
            <p className="max-w-3xl text-sm text-muted-foreground">
              This view displays real-time statistics about the container{" "}
              <strong className="text-foreground">{displayName}</strong> as
              well as a list of the running processes inside this container.
              Charts start from the moment this page is opened.
            </p>
          </div>
          <div className="flex items-center gap-2">
            <span className="text-sm text-muted-foreground">Refresh rate</span>
            <Select
              value={String(refreshMs)}
              onValueChange={(v) => {
                if (v) setRefreshMs(Number(v))
              }}
            >
              <SelectTrigger className="w-[5.5rem]" size="sm">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {REFRESH_OPTIONS.map((o) => (
                  <SelectItem key={o.value} value={o.value}>
                    {o.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
        </CardHeader>
      </Card>

      <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
        <MetricCard
          label="CPU"
          value={
            latest ? `${(latest.cpu ?? 0).toFixed(1)}%` : "—"
          }
          values={cpuSpark}
          color={COLORS.cpu}
        />
        <MetricCard
          label="Memory"
          value={
            latest
              ? `${formatBytes(latest.memory)} / ${formatBytes(latest.memoryLimit)}`
              : "—"
          }
          values={memorySpark}
          color={COLORS.memory}
        />
        <MetricCard
          label="Network"
          value={latest ? formatBytes(networkTotal) : "—"}
          values={networkSpark}
          color={COLORS.rx}
        />
        <MetricCard
          label="I/O"
          value={latest ? formatBytes(ioTotal) : "—"}
          values={ioSpark}
          color={COLORS.read}
        />
      </div>

      <div className="grid gap-4 lg:grid-cols-2">
        <ChartCard
          icon={<MemoryStick className="size-4" />}
          title="Memory usage"
        >
          <LiveLineChart
            series={memorySeries}
            times={times}
            yMax={memoryLimit > 0 ? memoryLimit : undefined}
            formatY={formatAxisBytes}
          />
        </ChartCard>

        <ChartCard icon={<Cpu className="size-4" />} title="CPU usage">
          <LiveLineChart
            series={cpuSeries}
            times={times}
            yMax={cpuMax}
            formatY={formatPercent}
          />
        </ChartCard>

        <ChartCard
          icon={<Activity className="size-4" />}
          title="Network usage (aggregate)"
        >
          <LiveLineChart
            series={networkSeries}
            times={times}
            formatY={formatAxisBytes}
          />
        </ChartCard>

        <ChartCard
          icon={<HardDrive className="size-4" />}
          title="I/O usage (aggregate)"
        >
          <LiveLineChart
            series={ioSeries}
            times={times}
            formatY={formatAxisBytes}
          />
        </ChartCard>
      </div>

      <Card className="gap-0 py-0">
        <CardHeader className="flex flex-row flex-wrap items-center justify-between gap-3 border-b py-4">
          <CardTitle className="flex items-center gap-2 text-base">
            <Activity className="size-4 text-muted-foreground" />
            Processes
          </CardTitle>
          <Input
            value={processSearch}
            onChange={(e) => setProcessSearch(e.target.value)}
            placeholder="Search…"
            className="h-9 max-w-xs"
          />
        </CardHeader>
        <CardContent className="px-0 py-0">
          <DataTable
            columns={columns}
            data={filteredRows}
            enablePagination
            pageSize={10}
            emptyMessage={
              topQuery.isLoading
                ? "Loading processes…"
                : topQuery.isError
                  ? "Unable to load processes"
                  : "No processes"
            }
          />
        </CardContent>
      </Card>
    </div>
  )
}

function MetricCard({
  label,
  value,
  values,
  color,
}: {
  label: string
  value: string
  values: number[]
  color: string
}) {
  return (
    <Card className="gap-0 py-0">
      <CardContent className="flex items-center justify-between gap-3 py-4">
        <div className="min-w-0 space-y-1">
          <p className="text-xs text-muted-foreground">{label}</p>
          <p className="truncate text-2xl font-semibold tracking-tight tabular-nums">
            {value}
          </p>
        </div>
        <MiniSparkline values={values} color={color} />
      </CardContent>
    </Card>
  )
}

function MiniSparkline({
  values,
  color,
}: {
  values: number[]
  color: string
}) {
  const w = 96
  const h = 36
  if (values.length === 0) {
    return <div className="h-9 w-24 shrink-0" aria-hidden />
  }
  let min = Infinity
  let max = -Infinity
  for (const v of values) {
    if (v < min) min = v
    if (v > max) max = v
  }
  if (!Number.isFinite(min) || !Number.isFinite(max)) {
    min = 0
    max = 1
  }
  if (max <= min) max = min + 1
  const n = Math.max(values.length - 1, 1)
  const path = values
    .map((v, i) => {
      const x = (i / n) * w
      const y = h - ((v - min) / (max - min)) * (h - 4) - 2
      return `${i === 0 ? "M" : "L"}${x.toFixed(1)} ${y.toFixed(1)}`
    })
    .join(" ")

  return (
    <svg
      width={w}
      height={h}
      viewBox={`0 0 ${w} ${h}`}
      className="shrink-0 opacity-90"
      aria-hidden
    >
      <path
        d={path}
        fill="none"
        stroke={color}
        strokeWidth={1.75}
        strokeLinejoin="round"
        strokeLinecap="round"
      />
    </svg>
  )
}

function ChartCard({
  icon,
  title,
  children,
}: {
  icon: ReactNode
  title: string
  children: ReactNode
}) {
  return (
    <Card className="gap-0 overflow-hidden py-0">
      <CardHeader className="border-b py-3">
        <CardTitle className="flex items-center gap-2 text-sm font-medium">
          <span className="text-muted-foreground">{icon}</span>
          {title}
        </CardTitle>
      </CardHeader>
      <CardContent className="py-3">{children}</CardContent>
    </Card>
  )
}
