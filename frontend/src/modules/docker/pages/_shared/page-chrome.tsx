import type { ReactNode } from "react"
import { RefreshCw } from "lucide-react"

import { Button } from "@/components/ui/button"
import { cn } from "@/lib/utils"

export const DOCKER_PAGE_DESCRIPTIONS = {
  containers: "Manage running and stopped containers on this Docker environment.",
  images: "Browse local images, prune unused layers, and pull new ones.",
  networks: "Inspect bridge, overlay, and custom networks attached to containers.",
  volumes: "Manage named volumes and see which containers use them.",
  stacks: "Deploy and control Compose stacks from YAML definitions.",
  templates: "Launch containers from curated application templates.",
  environments:
    "Configure local and remote Docker endpoints available to this workspace.",
} as const

export type DockerPageKey = keyof typeof DOCKER_PAGE_DESCRIPTIONS

export function DockerRefreshButton({
  onClick,
  isFetching,
  label = "Refresh",
}: {
  onClick: () => void
  isFetching?: boolean
  label?: string
}) {
  return (
    <Button
      variant="outline"
      size="sm"
      onClick={onClick}
      disabled={isFetching}
    >
      <RefreshCw
        className={cn("size-3.5", isFetching && "animate-spin")}
      />
      {label}
    </Button>
  )
}

export function SummaryChip({
  children,
  active,
  onClick,
}: {
  children: ReactNode
  active?: boolean
  onClick?: () => void
}) {
  const className = cn(
    "inline-flex items-center rounded-md border px-2 py-0.5 text-[11px] font-medium tabular-nums transition-colors",
    active
      ? "border-foreground/20 bg-foreground text-background"
      : "border-border bg-muted/40 text-muted-foreground",
    onClick && "cursor-pointer hover:border-foreground/30 hover:text-foreground",
  )
  if (onClick) {
    return (
      <button type="button" className={className} onClick={onClick}>
        {children}
      </button>
    )
  }
  return <span className={className}>{children}</span>
}
