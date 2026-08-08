import { Cloud, HardDrive } from "lucide-react"

import { cn } from "@/lib/utils"

import type { SoftwareListItem } from "../pages/list/api"

export function SourceBadge({
  software,
  className,
}: {
  software: Pick<SoftwareListItem, "is_remote" | "source">
  className?: string
}) {
  if (software.is_remote || software.source === "remote") {
    return (
      <span
        className={cn(
          "inline-flex items-center gap-1 rounded border border-sky-500/25 bg-sky-500/10 px-1.5 py-0.5 text-[10px] font-medium uppercase tracking-wide text-sky-700 dark:text-sky-300",
          className
        )}
        title="Available from package registry"
      >
        <Cloud className="size-3" />
        Remote
      </span>
    )
  }
  if (software.source === "both") {
    return (
      <span
        className={cn(
          "inline-flex items-center gap-1 rounded border border-border/70 bg-muted/40 px-1.5 py-0.5 text-[10px] font-medium uppercase tracking-wide text-muted-foreground",
          className
        )}
        title="Local package also listed in registry"
      >
        <HardDrive className="size-3" />
        Local
      </span>
    )
  }
  return null
}
