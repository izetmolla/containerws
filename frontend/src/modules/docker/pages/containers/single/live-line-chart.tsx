import { useId, useMemo } from "react"

import { cn } from "@/lib/utils"

export type ChartSeries = {
  key: string
  label: string
  color: string
  values: number[]
}

type LiveLineChartProps = {
  series: ChartSeries[]
  /** Shared X timestamps (ms) aligned with series values. */
  times: number[]
  yMax?: number
  yMin?: number
  formatY: (n: number) => string
  className?: string
  height?: number
}

const PAD = { top: 16, right: 16, bottom: 28, left: 56 }

function niceMax(raw: number): number {
  if (!Number.isFinite(raw) || raw <= 0) return 1
  const exp = Math.floor(Math.log10(raw))
  const base = Math.pow(10, exp)
  const norm = raw / base
  const nice = norm <= 1 ? 1 : norm <= 2 ? 2 : norm <= 5 ? 5 : 10
  return nice * base
}

function buildPath(
  values: number[],
  width: number,
  height: number,
  yMin: number,
  yMax: number
): string {
  if (values.length === 0) return ""
  const span = Math.max(yMax - yMin, 1e-9)
  const n = Math.max(values.length - 1, 1)
  return values
    .map((v, i) => {
      const x = PAD.left + (i / n) * width
      const y = PAD.top + height - ((v - yMin) / span) * height
      return `${i === 0 ? "M" : "L"}${x.toFixed(2)} ${y.toFixed(2)}`
    })
    .join(" ")
}

export function LiveLineChart({
  series,
  times,
  yMax: yMaxProp,
  yMin = 0,
  formatY,
  className,
  height = 220,
}: LiveLineChartProps) {
  const gid = useId()
  const plotW = 640
  const plotH = height - PAD.top - PAD.bottom
  const innerW = plotW - PAD.left - PAD.right

  const computedMax = useMemo(() => {
    let max = 0
    for (const s of series) {
      for (const v of s.values) {
        if (v > max) max = v
      }
    }
    return niceMax(Math.max(max, yMaxProp ?? 0))
  }, [series, yMaxProp])

  const yMax = yMaxProp != null && yMaxProp > 0 ? yMaxProp : computedMax
  const ticks = 5
  const yTicks = Array.from({ length: ticks + 1 }, (_, i) => {
    return yMin + ((yMax - yMin) * i) / ticks
  })

  const hasData = times.length > 0

  return (
    <div className={cn("w-full", className)}>
      <svg
        viewBox={`0 0 ${plotW} ${height}`}
        className="h-auto w-full"
        role="img"
        aria-label="Live statistics chart"
      >
        <defs>
          {series.map((s) => (
            <linearGradient
              key={s.key}
              id={`${gid}-${s.key}`}
              x1="0"
              y1="0"
              x2="0"
              y2="1"
            >
              <stop offset="0%" stopColor={s.color} stopOpacity={0.18} />
              <stop offset="100%" stopColor={s.color} stopOpacity={0} />
            </linearGradient>
          ))}
        </defs>

        {yTicks.map((t) => {
          const span = Math.max(yMax - yMin, 1e-9)
          const y = PAD.top + plotH - ((t - yMin) / span) * plotH
          return (
            <g key={t}>
              <line
                x1={PAD.left}
                x2={PAD.left + innerW}
                y1={y}
                y2={y}
                className="stroke-border"
                strokeWidth={1}
              />
              <text
                x={PAD.left - 8}
                y={y + 3}
                textAnchor="end"
                className="fill-muted-foreground"
                fontSize={10}
              >
                {formatY(t)}
              </text>
            </g>
          )
        })}

        {hasData
          ? series.map((s) => {
              const path = buildPath(s.values, innerW, plotH, yMin, yMax)
              if (!path) return null
              const area =
                path +
                ` L${PAD.left + innerW} ${PAD.top + plotH}` +
                ` L${PAD.left} ${PAD.top + plotH} Z`
              const last = s.values[s.values.length - 1] ?? 0
              const n = Math.max(s.values.length - 1, 1)
              const lx = PAD.left + n * (innerW / n)
              const span = Math.max(yMax - yMin, 1e-9)
              const ly =
                PAD.top + plotH - ((last - yMin) / span) * plotH
              return (
                <g key={s.key}>
                  {s.values.length > 1 ? (
                    <path d={area} fill={`url(#${gid}-${s.key})`} />
                  ) : null}
                  <path
                    d={path}
                    fill="none"
                    stroke={s.color}
                    strokeWidth={2}
                    strokeLinejoin="round"
                    strokeLinecap="round"
                  />
                  <circle cx={lx} cy={ly} r={3.5} fill={s.color} />
                </g>
              )
            })
          : null}
      </svg>

      <div className="mt-1 flex flex-wrap items-center justify-center gap-4 text-xs text-muted-foreground">
        {series.map((s) => (
          <span key={s.key} className="inline-flex items-center gap-1.5">
            <span
              className="size-2.5 rounded-sm"
              style={{ backgroundColor: s.color }}
            />
            {s.label}
          </span>
        ))}
      </div>
    </div>
  )
}
