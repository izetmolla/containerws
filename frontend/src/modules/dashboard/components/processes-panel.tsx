import { useMemo, useState } from "react"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import {
  ArrowDown,
  ArrowUp,
  ArrowUpDown,
  ListOrdered,
  Skull,
} from "lucide-react"
import { toast } from "sonner"

import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from "@/components/ui/alert-dialog"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import {
  autoRefreshInterval,
  useAutoRefreshMs,
} from "@/lib/auto-refresh"
import { cn } from "@/lib/utils"
import { getRequestErrorMessage } from "@/lib/network"

import {
  DASHBOARD_FETCH_KEY,
  getDashboardProcesses,
  killDashboardProcess,
  type DashboardProcess,
} from "../api"
import { formatPercent, metricCardClassName, usageTone } from "../lib/format"

type SortKey = "pid" | "user" | "name" | "cpu" | "memory" | "state"
type SortDir = "asc" | "desc"

const SORT_COLUMNS: {
  key: SortKey
  label: string
  align?: "left" | "right"
  className?: string
}[] = [
  { key: "pid", label: "PID" },
  { key: "user", label: "User" },
  { key: "name", label: "Process" },
  { key: "cpu", label: "CPU", align: "right" },
  { key: "memory", label: "Memory", align: "right" },
  { key: "state", label: "State" },
]

function defaultDir(key: SortKey): SortDir {
  return key === "cpu" || key === "memory" ? "desc" : "asc"
}

export function ProcessesPanel() {
  const queryClient = useQueryClient()
  const refreshMs = useAutoRefreshMs()
  const [sortBy, setSortBy] = useState<SortKey>("cpu")
  const [sortDir, setSortDir] = useState<SortDir>("desc")

  const processesQuery = useQuery({
    queryKey: [DASHBOARD_FETCH_KEY, "processes"],
    queryFn: () => getDashboardProcesses(80),
    refetchInterval: autoRefreshInterval(refreshMs),
    refetchIntervalInBackground: false,
  })

  const killMutation = useMutation({
    mutationFn: ({ pid, force }: { pid: number; force?: boolean }) =>
      killDashboardProcess(pid, force),
    onSuccess: (_data, vars) => {
      toast.success(
        vars.force
          ? `Force-killed PID ${vars.pid}`
          : `Sent SIGTERM to PID ${vars.pid}`
      )
      void queryClient.invalidateQueries({
        queryKey: [DASHBOARD_FETCH_KEY, "processes"],
      })
    },
    onError: (err, vars) => {
      toast.error(
        getRequestErrorMessage(err, `Failed to kill PID ${vars.pid}`)
      )
    },
  })

  const rows = useMemo(() => {
    const list = processesQuery.data?.data ?? []
    const dir = sortDir === "asc" ? 1 : -1
    return [...list].sort((a, b) => {
      let cmp: number
      switch (sortBy) {
        case "pid":
          cmp = a.pid - b.pid
          break
        case "user":
          cmp = (a.user || "").localeCompare(b.user || "")
          break
        case "name":
          cmp = a.name.localeCompare(b.name)
          break
        case "state":
          cmp = (a.state || "").localeCompare(b.state || "")
          break
        case "memory":
          cmp = a.memory_bytes - b.memory_bytes
          break
        case "cpu":
        default:
          cmp = a.cpu_percent - b.cpu_percent
          if (cmp === 0) cmp = a.memory_bytes - b.memory_bytes
          break
      }
      if (cmp === 0) cmp = a.pid - b.pid
      return cmp * dir
    })
  }, [processesQuery.data?.data, sortBy, sortDir])

  function toggleSort(key: SortKey) {
    if (sortBy === key) {
      setSortDir((d) => (d === "asc" ? "desc" : "asc"))
      return
    }
    setSortBy(key)
    setSortDir(defaultDir(key))
  }

  return (
    <Card className={metricCardClassName()}>
      <CardHeader className="flex flex-row flex-wrap items-start justify-between gap-3 space-y-0">
        <div className="space-y-1">
          <CardTitle className="flex items-center gap-2 text-base">
            <ListOrdered className="size-4 text-muted-foreground" />
            Processes
          </CardTitle>
          <p className="text-xs text-muted-foreground">
            Top processes by resource use. Click a column header to sort.
          </p>
        </div>
        <Badge variant="outline" className="shrink-0 tabular-nums">
          {rows.length} shown
        </Badge>
      </CardHeader>
      <CardContent>
        {processesQuery.isLoading && rows.length === 0 ? (
          <p className="text-sm text-muted-foreground">Loading processes…</p>
        ) : processesQuery.isError && rows.length === 0 ? (
          <p className="text-sm text-destructive">
            {getRequestErrorMessage(
              processesQuery.error,
              "Unable to load processes"
            )}
          </p>
        ) : rows.length === 0 ? (
          <p className="text-sm text-muted-foreground">No processes reported</p>
        ) : (
          <div className="relative h-[420px] overflow-hidden rounded-lg border border-border/60 bg-card shadow-md ring-1 ring-foreground/5">
            <div className="h-full overflow-x-auto overflow-y-auto">
              <table className="w-full min-w-[820px] border-collapse text-sm">
                <thead className="sticky top-0 z-10">
                  <tr className="border-b border-border/60 bg-muted/95 text-left text-xs text-muted-foreground shadow-[0_1px_0_0_hsl(var(--border)/0.8),0_8px_16px_-12px_rgba(0,0,0,0.35)] backdrop-blur-sm">
                    {SORT_COLUMNS.map((col) => (
                      <SortableTh
                        key={col.key}
                        label={col.label}
                        active={sortBy === col.key}
                        dir={sortBy === col.key ? sortDir : null}
                        align={col.align}
                        onClick={() => toggleSort(col.key)}
                      />
                    ))}
                    <th className="px-3 py-2.5 text-right font-medium">
                      Actions
                    </th>
                  </tr>
                </thead>
                <tbody>
                  {rows.map((proc) => (
                    <ProcessRow
                      key={proc.pid}
                      proc={proc}
                      killing={
                        killMutation.isPending &&
                        killMutation.variables?.pid === proc.pid
                      }
                      onKill={(force) =>
                        killMutation.mutate({ pid: proc.pid, force })
                      }
                    />
                  ))}
                </tbody>
              </table>
            </div>
          </div>
        )}
      </CardContent>
    </Card>
  )
}

function SortableTh({
  label,
  active,
  dir,
  align = "left",
  onClick,
}: {
  label: string
  active: boolean
  dir: SortDir | null
  align?: "left" | "right"
  onClick: () => void
}) {
  const Icon = !active ? ArrowUpDown : dir === "asc" ? ArrowUp : ArrowDown
  return (
    <th
      className={cn(
        "px-3 py-2.5 font-medium",
        align === "right" ? "text-right" : "text-left"
      )}
    >
      <button
        type="button"
        onClick={onClick}
        className={cn(
          "inline-flex items-center gap-1 rounded-md px-1 py-0.5 transition-colors hover:text-foreground",
          align === "right" && "flex-row-reverse",
          active ? "text-foreground" : "text-muted-foreground"
        )}
      >
        {label}
        <Icon
          className={cn("size-3.5", active ? "opacity-100" : "opacity-50")}
        />
      </button>
    </th>
  )
}

function ProcessRow({
  proc,
  killing,
  onKill,
}: {
  proc: DashboardProcess
  killing: boolean
  onKill: (force?: boolean) => void
}) {
  const protectedPid = proc.pid <= 1

  return (
    <tr className="border-b border-border/40 last:border-0 hover:bg-muted/20">
      <td className="px-3 py-2.5 tabular-nums font-medium">{proc.pid}</td>
      <td className="px-3 py-2.5 text-muted-foreground">
        {proc.user || "—"}
      </td>
      <td className="px-3 py-2.5">
        <div className="min-w-0">
          <div className="font-medium tracking-tight">{proc.name}</div>
          <div
            className="max-w-[360px] truncate text-[11px] text-muted-foreground"
            title={proc.cmdline}
          >
            {proc.cmdline}
          </div>
        </div>
      </td>
      <td
        className={cn(
          "px-3 py-2.5 text-right font-medium tabular-nums",
          usageTone(proc.cpu_percent)
        )}
      >
        {formatPercent(proc.cpu_percent)}
      </td>
      <td className="px-3 py-2.5 text-right">
        <div className="font-medium tabular-nums">
          {proc.memory_human || "0 B"}
        </div>
        <div className="text-[11px] text-muted-foreground tabular-nums">
          {formatPercent(proc.memory_percent)}
        </div>
      </td>
      <td className="px-3 py-2.5">
        <Badge variant="outline" className="h-5 font-mono text-[10px]">
          {proc.state || "—"}
        </Badge>
      </td>
      <td className="px-3 py-2.5 text-right">
        {protectedPid ? (
          <span className="text-[11px] text-muted-foreground">Protected</span>
        ) : (
          <KillProcessDialog
            proc={proc}
            killing={killing}
            onKill={onKill}
          />
        )}
      </td>
    </tr>
  )
}

function KillProcessDialog({
  proc,
  killing,
  onKill,
}: {
  proc: DashboardProcess
  killing: boolean
  onKill: (force?: boolean) => void
}) {
  return (
    <AlertDialog>
      <AlertDialogTrigger asChild>
        <Button
          size="sm"
          variant="outline"
          className="h-7 gap-1.5 text-destructive hover:bg-destructive/10 hover:text-destructive"
          disabled={killing}
        >
          <Skull className="size-3.5" />
          Kill
        </Button>
      </AlertDialogTrigger>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>Kill process {proc.pid}?</AlertDialogTitle>
          <AlertDialogDescription asChild>
            <div className="space-y-3 text-sm text-muted-foreground">
              <p>
                This will stop{" "}
                <span className="font-medium text-foreground">{proc.name}</span>
                {proc.user ? (
                  <>
                    {" "}
                    running as{" "}
                    <span className="font-medium text-foreground">
                      {proc.user}
                    </span>
                  </>
                ) : null}
                . Unsaved work in that process may be lost.
              </p>
              <div className="rounded-lg border border-border/70 bg-muted/40 p-3 text-xs">
                <div className="grid gap-1.5">
                  <div className="flex justify-between gap-3">
                    <span className="text-muted-foreground">Command</span>
                    <span
                      className="max-w-[260px] truncate text-right font-medium text-foreground"
                      title={proc.cmdline}
                    >
                      {proc.cmdline || proc.name}
                    </span>
                  </div>
                  <div className="flex justify-between gap-3">
                    <span className="text-muted-foreground">CPU</span>
                    <span className="tabular-nums text-foreground">
                      {formatPercent(proc.cpu_percent)}
                    </span>
                  </div>
                  <div className="flex justify-between gap-3">
                    <span className="text-muted-foreground">Memory</span>
                    <span className="tabular-nums text-foreground">
                      {proc.memory_human || "0 B"}
                    </span>
                  </div>
                </div>
              </div>
              <p>
                <span className="font-medium text-foreground">Kill</span> sends
                SIGTERM (graceful).{" "}
                <span className="font-medium text-foreground">Force kill</span>{" "}
                sends SIGKILL immediately.
              </p>
            </div>
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>Cancel</AlertDialogCancel>
          <AlertDialogAction
            onClick={() => onKill(true)}
            className="bg-muted text-foreground hover:bg-muted/80"
          >
            Force kill
          </AlertDialogAction>
          <AlertDialogAction
            onClick={() => onKill(false)}
            className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
          >
            Kill process
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  )
}
