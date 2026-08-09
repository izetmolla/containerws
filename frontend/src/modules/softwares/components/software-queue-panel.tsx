import { Link } from "react-router"
import { Loader2, Package } from "lucide-react"

import { Button } from "@/components/ui/button"
import { cn } from "@/lib/utils"
import { SoftwareGlyph } from "@/modules/softwares/components/software-glyph"
import type {
  SoftwareQueueItem,
  SoftwareQueueSnapshot,
} from "@/modules/softwares/pages/list/api"

function actionLabel(action: string) {
  switch (action) {
    case "update":
      return "Update"
    case "uninstall":
      return "Uninstall"
    default:
      return "Install"
  }
}

export function SoftwareQueuePanel({
  queue,
  waiting,
  product = "VNC",
  className,
}: {
  queue?: SoftwareQueueSnapshot | null
  waiting?: boolean
  /** Feature waiting on the queue (e.g. "VNC", "VS Code"). */
  product?: string
  className?: string
}) {
  const items = queue?.items ?? []
  const pending = queue?.pending ?? 0
  const busy = pending > 0 || Boolean(queue?.running)
  const visible = items.filter(
    (it) =>
      it.status === "pending" ||
      it.status === "running" ||
      it.status === "error"
  )

  if (!busy && !waiting && visible.length === 0) {
    return null
  }

  return (
    <section
      className={cn(
        "grid gap-4 rounded-xl border bg-card p-5",
        waiting && "border-amber-500/40 bg-amber-500/5",
        className
      )}
    >
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="flex min-w-0 items-start gap-3">
          <div className="flex size-10 shrink-0 items-center justify-center rounded-xl bg-sky-600/15 text-sky-700 dark:text-sky-300">
            {busy || waiting ? (
              <Loader2 className="size-5 animate-spin" />
            ) : (
              <Package className="size-5" />
            )}
          </div>
          <div className="min-w-0 space-y-1">
            <h2 className="text-base font-semibold tracking-tight">
              Software install queue
            </h2>
            <p className="text-sm text-muted-foreground">
              {waiting
                ? pending > 0
                  ? `${product} is waiting for ${pending} software job${pending === 1 ? "" : "s"} to finish (or fail).`
                  : `${product} is waiting for the softwares catalog reconcile to finish…`
                : pending > 0
                  ? `${pending} in progress — ${product} continues when this queue clears.`
                  : visible.length > 0
                    ? `Some software jobs need attention. Failed items do not block ${product}.`
                    : "Queue is clear."}
            </p>
          </div>
        </div>
        <Button type="button" size="sm" variant="outline" asChild>
          <Link to="/softwares/installing">Open queue</Link>
        </Button>
      </div>

      {visible.length > 0 ? (
        <ul className="divide-y rounded-lg border bg-background/60">
          {visible.slice(0, 8).map((item) => (
            <QueueRow key={item.id} item={item} />
          ))}
        </ul>
      ) : null}

      {visible.length > 8 ? (
        <p className="text-xs text-muted-foreground">
          +{visible.length - 8} more — see Softwares → Installing
        </p>
      ) : null}
    </section>
  )
}

function queueItemHref(item: SoftwareQueueItem) {
  if (item.href) return item.href
  if (item.source === "brew" && item.brew_name) {
    const kind = item.brew_kind ? `?kind=${encodeURIComponent(item.brew_kind)}` : ""
    return `/brew/${encodeURIComponent(item.brew_name)}${kind}`
  }
  return `/softwares/${item.software_id}`
}

function QueueRow({ item }: { item: SoftwareQueueItem }) {
  const accent = item.color || "var(--primary)"
  const pending = item.status === "pending"
  const running = item.status === "running"
  const failed = item.status === "error"
  const statusText =
    item.message ||
    (pending
      ? "Waiting in queue…"
      : running
        ? "Installing…"
        : failed
          ? "Failed"
          : item.status)

  return (
    <li className="flex items-center gap-3 px-3 py-2.5">
      {item.image?.trim() ? (
        <div className="flex size-8 shrink-0 items-center justify-center overflow-hidden rounded-md border bg-background">
          <SoftwareGlyph
            name={item.icon}
            image={item.image}
            className="size-3.5"
            imgClassName="size-8 object-cover"
          />
        </div>
      ) : (
        <div
          className="flex size-8 shrink-0 items-center justify-center rounded-md text-white"
          style={{ backgroundColor: accent }}
        >
          <SoftwareGlyph name={item.icon} className="size-3.5" />
        </div>
      )}
      <div className="min-w-0 flex-1">
        <div className="flex flex-wrap items-center gap-2">
          <Link
            to={queueItemHref(item)}
            className="truncate text-sm font-medium hover:underline"
          >
            {item.software_name || item.software_id}
          </Link>
          <span className="text-xs text-muted-foreground">
            {item.source === "brew" ? `Brew · ${actionLabel(item.action)}` : actionLabel(item.action)}
          </span>
        </div>
        <p
          className={cn(
            "truncate text-xs",
            failed ? "text-destructive" : "text-muted-foreground"
          )}
          title={statusText}
        >
          {statusText}
        </p>
      </div>
      <span
        className={cn(
          "shrink-0 rounded-md px-2 py-0.5 text-[11px] font-medium",
          running && "bg-sky-500/15 text-sky-700 dark:text-sky-300",
          pending && "bg-muted text-muted-foreground",
          failed && "bg-destructive/15 text-destructive"
        )}
      >
        {running ? "Installing" : pending ? "Queued" : "Failed"}
      </span>
    </li>
  )
}
