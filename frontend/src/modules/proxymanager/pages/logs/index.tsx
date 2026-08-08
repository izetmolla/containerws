import { useQuery } from "@tanstack/react-query"
import { Loader2 } from "lucide-react"

import ContentLoader from "@/components/content-loader"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"

import { getProxyLogs, PROXY_LOGS_KEY } from "../_shared/api"
import {
  PROXY_PAGE_DESCRIPTIONS,
  ProxyRefreshButton,
  ProxySubNav,
  SummaryChip,
} from "../_shared/page-chrome"

export default function ProxyLogsPage() {
  const logsQuery = useQuery({
    queryKey: [PROXY_LOGS_KEY],
    queryFn: () => getProxyLogs(300),
    refetchInterval: 8_000,
  })

  const data = logsQuery.data?.data

  return (
    <ContentLoader
      title="Proxy logs"
      description={PROXY_PAGE_DESCRIPTIONS.logs}
      breadcrumb={[
        { label: "Proxy Manager", to: "/proxymanager" },
        { label: "Logs" },
      ]}
      isLoading={logsQuery.isLoading}
      error={logsQuery.error}
      rightComponent={
        <div className="flex gap-2">
          <ProxyRefreshButton
            isFetching={logsQuery.isFetching}
            onClick={() => void logsQuery.refetch()}
          />
          <Button
            size="sm"
            variant="outline"
            onClick={() => void logsQuery.refetch()}
            disabled={logsQuery.isFetching}
          >
            {logsQuery.isFetching ? (
              <Loader2 className="size-3.5 animate-spin" />
            ) : null}
            Refresh
          </Button>
        </div>
      }
    >
      <ProxySubNav />

      <div className="mb-4 grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
        <SummaryChip label="Engine" value={data?.engine || "—"} />
        <SummaryChip label="Runtime" value={data?.runtime || "—"} />
        <SummaryChip label="Source" value={data?.source || "—"} />
        <SummaryChip
          label="Status"
          value={
            data?.error ? (
              <Badge variant="destructive">Error</Badge>
            ) : (
              <Badge>Live</Badge>
            )
          }
        />
      </div>

      {data?.error ? (
        <div className="mb-4 rounded-xl border border-destructive/40 bg-destructive/10 px-4 py-3 text-sm text-destructive">
          {data.error}
        </div>
      ) : null}

      <pre className="max-h-[min(70vh,40rem)] overflow-auto rounded-xl border bg-zinc-950 p-4 font-mono text-xs leading-relaxed text-zinc-100">
        {data?.text?.trim() ||
          (logsQuery.isFetching ? "Loading…" : "No log output yet.")}
      </pre>
      <p className="mt-2 text-xs text-muted-foreground">
        Auto-refreshes every 8 seconds while this tab is open.
      </p>
    </ContentLoader>
  )
}
