import { useEffect, useState } from "react"
import type { ReactNode } from "react"
import { Link, useLocation } from "react-router"
import { useMutation } from "@tanstack/react-query"
import { Loader2, RotateCcw, Rocket } from "lucide-react"
import { toast } from "sonner"

import { MonacoCodeEditor } from "@/components/monaco-editor"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { toastRequestError } from "@/lib/network"
import { cn } from "@/lib/utils"

export type ResourceTab = {
  to: string
  label: string
  end?: boolean
}

export function ResourceSubNav({
  base,
  tabs,
}: {
  base: string
  tabs: ResourceTab[]
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
              "px-3 py-2 text-sm font-medium transition-colors",
              active
                ? "border-b-2 border-foreground text-foreground"
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
  return (
    <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
      {items.map((it) => (
        <div key={it.label} className="rounded-xl border bg-muted/20 px-4 py-3">
          <p className="text-xs text-muted-foreground">{it.label}</p>
          <div className="mt-1 truncate text-sm font-medium">{it.value}</div>
        </div>
      ))}
    </div>
  )
}

export function KeyValueList({
  data,
}: {
  data?: Record<string, string> | null
}) {
  const entries = Object.entries(data || {})
  if (!entries.length) {
    return <p className="text-sm text-muted-foreground">None</p>
  }
  return (
    <dl className="divide-y rounded-xl border">
      {entries.map(([k, v]) => (
        <div
          key={k}
          className="grid gap-1 px-3 py-2 sm:grid-cols-[180px_minmax(0,1fr)]"
        >
          <dt className="font-mono text-xs text-muted-foreground">{k}</dt>
          <dd className="break-all font-mono text-xs">{v}</dd>
        </div>
      ))}
    </dl>
  )
}

export function YamlViewer({
  yaml,
  loading,
}: {
  yaml?: string
  loading?: boolean
}) {
  return (
    <pre className="max-h-[70vh] overflow-auto rounded-xl border bg-muted/30 p-4 font-mono text-xs leading-relaxed whitespace-pre-wrap">
      {loading ? "Loading…" : yaml || "(empty)"}
    </pre>
  )
}

export type ResourceYamlEditorProps = {
  yaml?: string
  loading?: boolean
  onApply: (yaml: string) => Promise<{ message?: string; yaml?: string }>
  onApplied?: (yaml: string) => void
  onReload?: () => void
  className?: string
}

export function ResourceYamlEditor({
  yaml,
  loading,
  onApply,
  onApplied,
  onReload,
  className,
}: ResourceYamlEditorProps) {
  const [value, setValue] = useState("")
  const [baseline, setBaseline] = useState("")

  useEffect(() => {
    if (yaml == null) return
    setValue(yaml)
    setBaseline(yaml)
  }, [yaml])

  const dirty = value !== baseline

  const applyMutation = useMutation({
    mutationFn: () => onApply(value),
    onSuccess: (res) => {
      toast.success(res.message || "YAML applied")
      if (res.yaml) {
        setValue(res.yaml)
        setBaseline(res.yaml)
        onApplied?.(res.yaml)
      } else {
        setBaseline(value)
        onApplied?.(value)
      }
    },
    onError: (err) => toastRequestError(err, "Apply failed"),
  })

  if (loading && !baseline) {
    return (
      <p className="py-10 text-center text-sm text-muted-foreground">
        Loading YAML…
      </p>
    )
  }

  return (
    <div className={cn("space-y-3", className)}>
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div className="flex items-center gap-2 text-sm text-muted-foreground">
          <span>Edit and deploy this manifest</span>
          {dirty ? <Badge variant="secondary">unsaved</Badge> : null}
        </div>
        <div className="flex flex-wrap gap-2">
          <Button
            size="sm"
            variant="outline"
            disabled={!dirty || applyMutation.isPending}
            onClick={() => setValue(baseline)}
          >
            <RotateCcw className="size-3.5" />
            Reset
          </Button>
          {onReload ? (
            <Button
              size="sm"
              variant="outline"
              disabled={applyMutation.isPending || loading}
              onClick={onReload}
            >
              Reload
            </Button>
          ) : null}
          <Button
            size="sm"
            disabled={!value.trim() || applyMutation.isPending || !dirty}
            onClick={() => applyMutation.mutate()}
          >
            {applyMutation.isPending ? (
              <Loader2 className="size-3.5 animate-spin" />
            ) : (
              <Rocket className="size-3.5" />
            )}
            Deploy
          </Button>
        </div>
      </div>

      <div
        className={cn(
          "overflow-hidden rounded-xl border border-input",
          "focus-within:border-ring focus-within:ring-[3px] focus-within:ring-ring/50",
        )}
        style={{ height: "min(70vh, 640px)" }}
      >
        <MonacoCodeEditor
          value={value}
          onChange={setValue}
          language="yaml"
          height="100%"
        />
      </div>
      <p className="text-xs text-muted-foreground">
        Deploy runs a server-side apply for this resource only. Kind, name, and
        namespace in the YAML must match the current object. Pods often reject
        immutable field changes.
      </p>
    </div>
  )
}

export function LogsViewer({
  logs,
  loading,
}: {
  logs?: string
  loading?: boolean
}) {
  return (
    <pre className="max-h-[65vh] overflow-auto rounded-xl border bg-muted/30 p-4 font-mono text-xs leading-relaxed whitespace-pre-wrap">
      {loading ? "Loading logs…" : logs || "(no logs)"}
    </pre>
  )
}
