import { cn } from "@/lib/utils"
import {
  clampPercent,
  formatPercent,
  usageBarClass,
  usageTone,
} from "../lib/format"

type UsageBarProps = {
  label: string
  percent: number
  detail?: string
  className?: string
}

export function UsageBar({ label, percent, detail, className }: UsageBarProps) {
  const pct = clampPercent(percent)
  return (
    <div className={cn("space-y-2", className)}>
      <div className="flex items-baseline justify-between gap-3">
        <span className="text-xs font-medium text-muted-foreground">
          {label}
        </span>
        <span className={cn("text-sm font-semibold tabular-nums", usageTone(pct))}>
          {formatPercent(pct)}
          {detail ? (
            <span className="ms-2 text-xs font-normal text-muted-foreground">
              {detail}
            </span>
          ) : null}
        </span>
      </div>
      <div className="h-2 overflow-hidden rounded-full bg-muted">
        <div
          className={cn(
            "h-full rounded-full transition-[width] duration-500 ease-out",
            usageBarClass(pct)
          )}
          style={{ width: `${pct}%` }}
        />
      </div>
    </div>
  )
}
