import { cn } from "@/lib/utils"

export function formatUptime(seconds: number): string {
  if (!Number.isFinite(seconds) || seconds <= 0) return "—"
  const total = Math.floor(seconds)
  const days = Math.floor(total / 86400)
  const hours = Math.floor((total % 86400) / 3600)
  const mins = Math.floor((total % 3600) / 60)
  if (days > 0) return `${days}d ${hours}h ${mins}m`
  if (hours > 0) return `${hours}h ${mins}m`
  return `${mins}m`
}

export function usageTone(percent: number): string {
  if (percent >= 90) return "text-red-600 dark:text-red-400"
  if (percent >= 75) return "text-amber-600 dark:text-amber-400"
  return "text-emerald-600 dark:text-emerald-400"
}

export function usageBarClass(percent: number): string {
  if (percent >= 90) return "bg-red-500"
  if (percent >= 75) return "bg-amber-500"
  return "bg-emerald-500"
}

export function clampPercent(value: number): number {
  if (!Number.isFinite(value)) return 0
  return Math.max(0, Math.min(100, value))
}

export function formatPercent(value: number, digits = 1): string {
  if (!Number.isFinite(value)) return "0%"
  return `${value.toFixed(digits)}%`
}

export function metricCardClassName(className?: string) {
  return cn(
    "rounded-xl border border-border/70 bg-card/60 shadow-sm backdrop-blur-sm",
    className
  )
}
