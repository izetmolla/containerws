import { useMemo, useState } from "react"
import { useQuery } from "@tanstack/react-query"
import { useOutletContext } from "react-router"
import { Download, RefreshCw, WrapText } from "lucide-react"

import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { cn } from "@/lib/utils"

import { getContainerLogs } from "../list/api"
import type { ContainerOutletContext } from "./layout"

export default function ContainerLogsPage() {
  const { id, container } = useOutletContext<ContainerOutletContext>()
  const [wrap, setWrap] = useState(true)
  const [filter, setFilter] = useState("")

  const logsQuery = useQuery({
    queryKey: ["docker-container-logs", id],
    queryFn: () => getContainerLogs(id, 300),
    refetchInterval: 5_000,
  })

  const raw = logsQuery.data?.data?.logs || ""
  const filtered = useMemo(() => {
    const needle = filter.trim().toLowerCase()
    if (!needle || !raw) return raw
    return raw
      .split("\n")
      .filter((line) => line.toLowerCase().includes(needle))
      .join("\n")
  }, [raw, filter])

  const download = () => {
    const blob = new Blob([raw || ""], { type: "text/plain;charset=utf-8" })
    const url = URL.createObjectURL(blob)
    const a = document.createElement("a")
    a.href = url
    a.download = `${container.name || id}-logs.txt`
    a.click()
    URL.revokeObjectURL(url)
  }

  const running = container.state === "running"

  return (
    <div className="flex flex-col gap-3">
      <div className="flex flex-wrap items-center gap-2">
        <div className="relative min-w-[12rem] flex-1">
          <Input
            className="h-8"
            placeholder="Filter log lines…"
            value={filter}
            onChange={(e) => setFilter(e.target.value)}
          />
        </div>
        <Button
          size="sm"
          variant={wrap ? "secondary" : "outline"}
          onClick={() => setWrap((v) => !v)}
        >
          <WrapText data-icon="inline-start" />
          Wrap
        </Button>
        <Button
          size="sm"
          variant="outline"
          onClick={download}
          disabled={!raw}
        >
          <Download data-icon="inline-start" />
          Download
        </Button>
        <Button
          size="sm"
          variant="outline"
          onClick={() => void logsQuery.refetch()}
          disabled={logsQuery.isFetching}
        >
          <RefreshCw
            className={cn("size-3.5", logsQuery.isFetching && "animate-spin")}
          />
          Refresh
        </Button>
      </div>

      {!running ? (
        <p className="rounded-lg border border-dashed px-3 py-2 text-xs text-muted-foreground">
          Container is not running. Showing the last collected log buffer when
          available.
        </p>
      ) : null}

      <pre
        className={cn(
          "max-h-[60vh] overflow-auto rounded-xl border bg-muted/30 p-4 font-mono text-xs leading-relaxed",
          wrap ? "whitespace-pre-wrap break-all" : "whitespace-pre",
        )}
      >
        {logsQuery.isLoading
          ? "Loading logs…"
          : filtered || (filter.trim() ? "(no matching lines)" : "(no logs)")}
      </pre>
    </div>
  )
}
