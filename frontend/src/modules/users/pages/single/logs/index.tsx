import { useOutletContext } from "react-router"
import { FileText, Monitor, Shield, User as UserIcon } from "lucide-react"

import { cn } from "@/lib/utils"

import type { UserSingleOutletContext } from "../types"

function formatWhen(value?: string) {
  if (!value) return "—"
  const date = new Date(value.includes("T") ? value : value.replace(" ", "T"))
  if (Number.isNaN(date.getTime())) return value
  return new Intl.DateTimeFormat(undefined, {
    year: "numeric",
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
  }).format(date)
}

export default function UserLogsPage() {
  const { user } = useOutletContext<UserSingleOutletContext>()

  const entries = [
    {
      id: "panel-updated",
      icon: UserIcon,
      title: "Panel profile",
      detail: `Status ${user.status || "unknown"} · roles ${(user.roles || []).join(", ") || "none"}`,
      at: user.updated_at,
      tone: "default" as const,
    },
    {
      id: "linux",
      icon: Shield,
      title: "Linux account",
      detail: user.linux?.exists
        ? `${user.linux.locked ? "Locked" : "Unlocked"} · uid ${user.linux.uid || "—"} · ${user.linux.home_dir || "—"}`
        : "Not provisioned on this host",
      at: user.updated_at,
      tone: user.linux?.exists
        ? user.linux.locked
          ? ("warn" as const)
          : ("ok" as const)
        : ("muted" as const),
    },
    {
      id: "vnc",
      icon: Monitor,
      title: "VNC / noVNC session",
      detail: user.vnc
        ? `${user.vnc.live ? "Live" : user.vnc.status} · :${user.vnc.vnc_port} / noVNC :${user.vnc.no_vnc_port}`
        : "No desktop profile",
      at: user.updated_at,
      tone: user.vnc?.live
        ? ("ok" as const)
        : user.vnc
          ? ("warn" as const)
          : ("muted" as const),
    },
  ]

  return (
    <div className="w-full space-y-4">
      <div>
        <h2 className="text-sm font-semibold">Activity & status</h2>
        <p className="text-xs text-muted-foreground">
          Snapshot of this user&apos;s panel, Linux, and desktop state. Live
          stream logs can be wired here later.
        </p>
      </div>

      <ol className="relative space-y-0 border-s border-border/70 ms-3">
        {entries.map((entry) => {
          const Icon = entry.icon
          return (
            <li key={entry.id} className="relative pb-6 ps-6 last:pb-0">
              <span
                className={cn(
                  "absolute -start-[9px] top-1 flex size-4 items-center justify-center rounded-full ring-4 ring-background",
                  entry.tone === "ok" && "bg-emerald-500",
                  entry.tone === "warn" && "bg-amber-500",
                  entry.tone === "muted" && "bg-muted-foreground/40",
                  entry.tone === "default" && "bg-primary"
                )}
              />
              <div className="rounded-xl border bg-card p-4">
                <div className="flex items-start gap-3">
                  <div className="grid size-8 shrink-0 place-items-center rounded-lg bg-muted">
                    <Icon className="size-3.5" />
                  </div>
                  <div className="min-w-0 flex-1">
                    <div className="flex flex-wrap items-baseline justify-between gap-2">
                      <h3 className="text-sm font-medium">{entry.title}</h3>
                      <time className="text-[11px] text-muted-foreground tabular-nums">
                        {formatWhen(entry.at)}
                      </time>
                    </div>
                    <p className="mt-1 text-xs text-muted-foreground">
                      {entry.detail}
                    </p>
                  </div>
                </div>
              </div>
            </li>
          )
        })}
      </ol>

      <div className="flex items-start gap-2 rounded-xl border border-dashed bg-muted/20 px-4 py-3 text-xs text-muted-foreground">
        <FileText className="mt-0.5 size-3.5 shrink-0" aria-hidden />
        <p>
          Per-user journal and auth audit streams are not connected yet. Use
          host logs or VNC session logs for deeper diagnostics.
        </p>
      </div>
    </div>
  )
}
