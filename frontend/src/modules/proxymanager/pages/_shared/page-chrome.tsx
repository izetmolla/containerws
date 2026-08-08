import type { ReactNode } from "react"
import { Link, useLocation } from "react-router"
import { RefreshCw } from "lucide-react"

import { Button } from "@/components/ui/button"
import { cn } from "@/lib/utils"

export const PROXY_PAGE_DESCRIPTIONS = {
  overview:
    "Select Fiber, Nginx, or Traefik and choose host or Docker runtime.",
  hosts: "Define virtual hosts, domains, upstreams, and path locations.",
  ssl: "Manage TLS certificates and HTTP redirects.",
  status: "Apply configuration to the active engine and review apply history.",
  logs: "Live output from the active proxy engine (Docker, journal, or apply runs).",
} as const

export type ProxyPageKey = keyof typeof PROXY_PAGE_DESCRIPTIONS

const TABS: { to: string; label: string }[] = [
  { to: "overview", label: "Settings" },
  { to: "hosts", label: "Hosts" },
  { to: "ssl", label: "SSL & Redirects" },
  { to: "status", label: "Status" },
  { to: "logs", label: "Logs" },
]

export function ProxySubNav() {
  const location = useLocation()
  return (
    <nav className="mb-4 flex flex-wrap gap-1 border-b">
      {TABS.map((t) => {
        const href = `/proxymanager/${t.to}`
        const active = location.pathname.startsWith(href)
        return (
          <Link
            key={t.to}
            to={href}
            className={cn(
              "relative px-3 py-2 text-sm font-medium transition-colors",
              active
                ? "text-foreground after:absolute after:inset-x-0 after:bottom-0 after:h-0.5 after:bg-primary"
                : "text-muted-foreground hover:text-foreground",
            )}
          >
            {t.label}
          </Link>
        )
      })}
    </nav>
  )
}

export function ProxyRefreshButton({
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
      <RefreshCw className={cn("size-3.5", isFetching && "animate-spin")} />
      {label}
    </Button>
  )
}

export function DirtyBanner({
  dirty,
  lastError,
  onApply,
  applying,
}: {
  dirty?: boolean
  lastError?: string
  onApply?: () => void
  applying?: boolean
}) {
  if (!dirty && !lastError) return null
  return (
    <div
      className={cn(
        "mb-4 flex flex-wrap items-center justify-between gap-3 rounded-xl border px-4 py-3 text-sm",
        lastError
          ? "border-destructive/40 bg-destructive/10 text-destructive"
          : "border-amber-500/40 bg-amber-500/10 text-amber-900 dark:text-amber-100",
      )}
    >
      <div>
        {lastError ? (
          <p>Last apply failed: {lastError}</p>
        ) : (
          <p>Configuration changed — apply to reload the active engine.</p>
        )}
      </div>
      {onApply ? (
        <Button size="sm" onClick={onApply} disabled={applying}>
          {applying ? "Applying…" : "Apply now"}
        </Button>
      ) : (
        <Button size="sm" asChild>
          <Link to="/proxymanager/status">Go to Status</Link>
        </Button>
      )}
    </div>
  )
}

export function SummaryChip({
  label,
  value,
}: {
  label: string
  value: ReactNode
}) {
  return (
    <div className="rounded-xl border bg-muted/20 px-4 py-3">
      <p className="text-xs text-muted-foreground">{label}</p>
      <div className="mt-1 text-sm font-medium">{value}</div>
    </div>
  )
}
