import type { SoftwareInstallStatus } from "../pages/list/api"
import {
  AlertTriangle,
  CheckCircle2,
  Circle,
  Download,
  Loader2,
  RefreshCw,
} from "lucide-react"
import { cn } from "@/lib/utils"

const styles: Record<SoftwareInstallStatus, string> = {
  not_installed: "bg-muted text-muted-foreground border-border",
  installed:
    "bg-emerald-500/10 text-emerald-700 border-emerald-500/20 dark:text-emerald-300",
  uninstalled:
    "bg-zinc-500/10 text-zinc-700 border-zinc-500/25 dark:text-zinc-300",
  missing:
    "bg-amber-500/10 text-amber-800 border-amber-500/30 dark:text-amber-300",
  update_available:
    "bg-amber-500/10 text-amber-700 border-amber-500/20 dark:text-amber-300",
  installing: "bg-blue-500/10 text-blue-700 border-blue-500/20 dark:text-blue-300",
  updating: "bg-blue-500/10 text-blue-700 border-blue-500/20 dark:text-blue-300",
}

const labels: Record<SoftwareInstallStatus, string> = {
  not_installed: "Not installed",
  installed: "Installed",
  uninstalled: "Uninstalled",
  missing: "Missing",
  update_available: "Update available",
  installing: "Installing",
  updating: "Updating",
}

export function StatusPill({
  status,
  compact,
}: {
  status: SoftwareInstallStatus
  compact?: boolean
}) {
  const Icon =
    status === "installed"
      ? CheckCircle2
      : status === "uninstalled"
        ? Circle
        : status === "missing"
          ? AlertTriangle
          : status === "update_available"
            ? RefreshCw
            : status === "installing" || status === "updating"
              ? Loader2
              : status === "not_installed"
                ? Circle
                : Download
  const spin = status === "installing" || status === "updating"
  return (
    <span
      className={cn(
        "inline-flex items-center border font-medium",
        compact
          ? "gap-1 rounded px-1.5 py-0.5 text-[11px] leading-none"
          : "gap-1.5 rounded-md px-2 py-0.5 text-xs",
        styles[status]
      )}
    >
      <Icon className={cn(compact ? "h-3 w-3" : "h-3 w-3", spin && "animate-spin")} />
      {labels[status]}
    </span>
  )
}
