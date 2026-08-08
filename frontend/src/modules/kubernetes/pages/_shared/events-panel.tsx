import { useQuery } from "@tanstack/react-query"
import { AlertTriangle, Info, RefreshCw } from "lucide-react"

import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { asArray } from "@/lib/as-array"
import { cn } from "@/lib/utils"

import {
  formatAge,
  K8S_EVENTS_KEY,
  listEvents,
  type EventRow,
} from "./api"

export function ResourceEventsPanel({
  namespace,
  kind,
  name,
  className,
}: {
  namespace?: string
  kind?: string
  name?: string
  className?: string
}) {
  const query = useQuery({
    queryKey: [K8S_EVENTS_KEY, namespace || "all", kind || "", name || ""],
    queryFn: () => listEvents(namespace, { kind, name }),
    refetchInterval: 10_000,
  })

  const rows = asArray(query.data?.data)

  return (
    <div className={cn("space-y-3", className)}>
      <div className="flex items-center justify-between gap-2">
        <div>
          <h3 className="text-sm font-medium">Events</h3>
          <p className="text-xs text-muted-foreground">
            Cluster events for this resource (newest first).
          </p>
        </div>
        <Button
          size="sm"
          variant="outline"
          onClick={() => void query.refetch()}
          disabled={query.isFetching}
        >
          <RefreshCw
            className={cn("size-3.5", query.isFetching && "animate-spin")}
          />
        </Button>
      </div>

      {query.isLoading ? (
        <p className="text-sm text-muted-foreground">Loading events…</p>
      ) : query.isError ? (
        <p className="text-sm text-destructive">Failed to load events.</p>
      ) : rows.length === 0 ? (
        <div className="rounded-lg border border-dashed px-4 py-8 text-center text-sm text-muted-foreground">
          No events for this resource.
        </div>
      ) : (
        <div className="overflow-hidden rounded-lg border">
          <table className="w-full text-sm">
            <thead className="bg-muted/40 text-left text-xs text-muted-foreground">
              <tr>
                <th className="px-3 py-2 font-medium">Type</th>
                <th className="px-3 py-2 font-medium">Reason</th>
                <th className="px-3 py-2 font-medium">Message</th>
                <th className="hidden px-3 py-2 font-medium md:table-cell">
                  Object
                </th>
                <th className="px-3 py-2 font-medium">Age</th>
                <th className="px-3 py-2 font-medium">Count</th>
              </tr>
            </thead>
            <tbody>
              {rows.map((ev: EventRow) => (
                <tr
                  key={`${ev.namespace}/${ev.name}/${ev.last_seen}`}
                  className="border-t border-border/60 align-top"
                >
                  <td className="px-3 py-2">
                    <Badge
                      variant={ev.type === "Warning" ? "destructive" : "secondary"}
                      className="gap-1"
                    >
                      {ev.type === "Warning" ? (
                        <AlertTriangle className="size-3" />
                      ) : (
                        <Info className="size-3" />
                      )}
                      {ev.type || "Normal"}
                    </Badge>
                  </td>
                  <td className="px-3 py-2 font-medium">{ev.reason || "—"}</td>
                  <td className="max-w-md px-3 py-2 text-muted-foreground">
                    <span className="line-clamp-3 break-words">{ev.message}</span>
                  </td>
                  <td className="hidden px-3 py-2 font-mono text-xs text-muted-foreground md:table-cell">
                    {ev.object || "—"}
                  </td>
                  <td className="whitespace-nowrap px-3 py-2 text-xs text-muted-foreground">
                    {formatAge(ev.last_seen)}
                  </td>
                  <td className="px-3 py-2 text-xs text-muted-foreground">
                    {ev.count || 1}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}
