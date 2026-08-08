import type { ReactNode } from "react"
import { Link, useLocation } from "react-router"
import { Copy } from "lucide-react"
import { toast } from "sonner"
import { useMemo, useState } from "react"

import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { cn } from "@/lib/utils"

export type DockerTab = {
  to: string
  label: string
  end?: boolean
}

export function DockerSubNav({
  base,
  tabs,
}: {
  base: string
  tabs: DockerTab[]
}) {
  const location = useLocation()
  const onBase =
    location.pathname === base ||
    location.pathname === `${base}/` ||
    location.pathname.replace(/\/$/, "") === base.replace(/\/$/, "")

  return (
    <nav className="flex flex-wrap gap-1 border-b">
      {tabs.map((t) => {
        const href = t.end || t.to === "." ? base : `${base}/${t.to}`
        const active =
          t.end || t.to === "."
            ? onBase
            : location.pathname.endsWith(`/${t.to}`)
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

export function MetaGrid({
  items,
}: {
  items: { label: string; value: ReactNode }[]
}) {
  const visible = items.filter(
    (it) => it.value !== undefined && it.value !== null && it.value !== "",
  )
  if (!visible.length) return null
  return (
    <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
      {visible.map((it) => (
        <div key={it.label} className="rounded-xl border bg-muted/20 px-4 py-3">
          <p className="text-xs text-muted-foreground">{it.label}</p>
          <div className="mt-1 truncate text-sm font-medium break-all">
            {it.value}
          </div>
        </div>
      ))}
    </div>
  )
}

export function KeyValueList({
  data,
  searchable,
}: {
  data?: Record<string, string> | null
  searchable?: boolean
}) {
  const [q, setQ] = useState("")
  const entries = useMemo(() => {
    const all = Object.entries(data || {})
    const needle = q.trim().toLowerCase()
    if (!needle) return all
    return all.filter(
      ([k, v]) =>
        k.toLowerCase().includes(needle) || v.toLowerCase().includes(needle),
    )
  }, [data, q])

  if (!Object.keys(data || {}).length) {
    return <p className="text-sm text-muted-foreground">None</p>
  }

  return (
    <div className="space-y-2">
      {searchable ? (
        <Input
          className="h-8"
          placeholder="Filter…"
          value={q}
          onChange={(e) => setQ(e.target.value)}
        />
      ) : null}
      <dl className="divide-y rounded-xl border">
        {entries.length ? (
          entries.map(([k, v]) => (
            <div
              key={k}
              className="grid gap-1 px-3 py-2 sm:grid-cols-[minmax(8rem,180px)_minmax(0,1fr)_auto]"
            >
              <dt className="font-mono text-xs text-muted-foreground">{k}</dt>
              <dd className="break-all font-mono text-xs">{v}</dd>
              <Button
                type="button"
                size="icon-sm"
                variant="ghost"
                className="size-6 shrink-0"
                onClick={() => {
                  void navigator.clipboard.writeText(`${k}=${v}`).then(
                    () => toast.success("Copied"),
                    () => toast.error("Copy failed"),
                  )
                }}
                aria-label={`Copy ${k}`}
              >
                <Copy className="size-3" />
              </Button>
            </div>
          ))
        ) : (
          <p className="px-3 py-2 text-sm text-muted-foreground">No matches</p>
        )}
      </dl>
    </div>
  )
}

export function SectionCard({
  title,
  description,
  action,
  children,
  className,
}: {
  title: string
  description?: string
  action?: ReactNode
  children: ReactNode
  className?: string
}) {
  return (
    <section className={cn("rounded-xl border bg-background", className)}>
      <div className="flex flex-wrap items-start justify-between gap-2 border-b px-4 py-3">
        <div className="min-w-0">
          <h2 className="text-sm font-semibold tracking-tight">{title}</h2>
          {description ? (
            <p className="mt-0.5 text-xs text-muted-foreground">{description}</p>
          ) : null}
        </div>
        {action}
      </div>
      <div className="p-4">{children}</div>
    </section>
  )
}
