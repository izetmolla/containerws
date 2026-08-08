import { useEffect, useMemo, useState } from "react"

import { cn } from "@/lib/utils"

import { clampPercent, formatPercent } from "../lib/format"

import type { GpuHistorySample } from "./gpu-history"

type MetricKey = "util" | "mem" | "memUtil" | "clock"

type UsageHistoryChartProps = {
  id: string
  title: string
  samples: GpuHistorySample[]
  metric: MetricKey
  maxY?: number
  unit?: string
  color?: string
  formatValue?: (v: number) => string
  className?: string
  openedAt: number
}

function readMetric(s: GpuHistorySample, metric: MetricKey): number {
  switch (metric) {
    case "util":
      return s.util
    case "mem":
      return s.mem
    case "memUtil":
      return s.memUtil
    case "clock":
      return s.clock
  }
}

export function UsageHistoryChart({
  id,
  title,
  samples,
  metric,
  maxY = 100,
  unit = "%",
  color = "rgb(34 197 94)",
  formatValue,
  className,
  openedAt,
}: UsageHistoryChartProps) {
  const width = 640
  const height = 160
  const padL = 36
  const padR = 12
  const padT = 16
  const padB = 22
  const plotW = width - padL - padR
  const plotH = height - padT - padB

  const [now, setNow] = useState(() => Date.now())

  useEffect(() => {
    const id = window.setInterval(() => setNow(Date.now()), 1000)
    return () => window.clearInterval(id)
  }, [])

  const values = useMemo(
    () =>
      samples.map((s) => {
        const raw = readMetric(s, metric)
        if (maxY <= 0) return 0
        return clampPercent((raw / maxY) * 100)
      }),
    [samples, metric, maxY]
  )

  const current = samples.length > 0 ? readMetric(samples[samples.length - 1]!, metric) : 0
  const peak = samples.reduce((m, s) => Math.max(m, readMetric(s, metric)), 0)

  const { linePath, areaPath } = useMemo(() => {
    if (values.length === 0) {
      return { linePath: "", areaPath: "" }
    }
    const n = Math.max(values.length - 1, 1)
    const pts = values.map((v, i) => {
      const x = padL + (i / n) * plotW
      const y = padT + plotH * (1 - v / 100)
      return { x, y }
    })
    const line = pts
      .map((p, i) => `${i === 0 ? "M" : "L"}${p.x.toFixed(2)},${p.y.toFixed(2)}`)
      .join(" ")
    const first = pts[0]!
    const last = pts[pts.length - 1]!
    const area = [
      line,
      `L${last.x.toFixed(2)},${(padT + plotH).toFixed(2)}`,
      `L${first.x.toFixed(2)},${(padT + plotH).toFixed(2)}`,
      "Z",
    ].join(" ")
    return { linePath: line, areaPath: area }
  }, [values, padL, padT, plotW, plotH])

  const elapsedLabel = formatElapsed(now - openedAt)
  const fmt =
    formatValue ??
    ((v: number) => (unit === "%" ? formatPercent(v, 0) : `${Math.round(v)}${unit}`))

  const yTicks = [0, 25, 50, 75, 100]
  const gradId = `gpu-fill-${id}`

  return (
    <div className={cn("space-y-2", className)}>
      <div className="flex items-baseline justify-between gap-3">
        <div className="text-xs font-medium text-muted-foreground">{title}</div>
        <div className="flex items-baseline gap-3 text-xs tabular-nums">
          <span className="text-muted-foreground">
            Peak <span className="font-medium text-foreground">{fmt(peak)}</span>
          </span>
          <span className="text-lg font-semibold tracking-tight text-foreground">
            {fmt(current)}
          </span>
        </div>
      </div>

      <div className="overflow-hidden rounded-lg border border-border/60 bg-[#0b1220] dark:bg-[#070b14]">
        <svg
          viewBox={`0 0 ${width} ${height}`}
          className="h-[140px] w-full sm:h-[160px]"
          role="img"
          aria-label={`${title} history`}
        >
          <defs>
            <linearGradient id={gradId} x1="0" y1="0" x2="0" y2="1">
              <stop offset="0%" stopColor={color} stopOpacity="0.45" />
              <stop offset="100%" stopColor={color} stopOpacity="0.02" />
            </linearGradient>
          </defs>

          {yTicks.map((tick) => {
            const y = padT + plotH * (1 - tick / 100)
            return (
              <g key={tick}>
                <line
                  x1={padL}
                  x2={width - padR}
                  y1={y}
                  y2={y}
                  stroke="rgba(148,163,184,0.18)"
                  strokeWidth={1}
                />
                <text
                  x={padL - 6}
                  y={y + 3}
                  textAnchor="end"
                  fill="rgba(148,163,184,0.7)"
                  fontSize={10}
                >
                  {Math.round((tick / 100) * maxY)}
                </text>
              </g>
            )
          })}

          {areaPath ? <path d={areaPath} fill={`url(#${gradId})`} /> : null}
          {linePath ? (
            <path
              d={linePath}
              fill="none"
              stroke={color}
              strokeWidth={2}
              strokeLinejoin="round"
              strokeLinecap="round"
            />
          ) : null}

          {values.length === 0 ? (
            <text
              x={width / 2}
              y={height / 2}
              textAnchor="middle"
              fill="rgba(148,163,184,0.7)"
              fontSize={12}
            >
              Collecting samples…
            </text>
          ) : null}

          <text x={padL} y={height - 6} fill="rgba(148,163,184,0.65)" fontSize={10}>
            {elapsedLabel} ago
          </text>
          <text
            x={width - padR}
            y={height - 6}
            textAnchor="end"
            fill="rgba(148,163,184,0.65)"
            fontSize={10}
          >
            now
          </text>
        </svg>
      </div>
    </div>
  )
}

function formatElapsed(ms: number): string {
  const sec = Math.max(0, Math.floor(ms / 1000))
  if (sec < 60) return `${sec}s`
  const min = Math.floor(sec / 60)
  const rem = sec % 60
  if (min < 60) return rem > 0 ? `${min}m ${rem}s` : `${min}m`
  const hr = Math.floor(min / 60)
  const remMin = min % 60
  return remMin > 0 ? `${hr}h ${remMin}m` : `${hr}h`
}
