import { useMemo, useState, type ReactNode } from "react"
import { Link } from "react-router"
import { useMutation, useQueryClient } from "@tanstack/react-query"
import {
  createColumnHelper,
  type ColumnDef,
  type RowSelectionState,
} from "@tanstack/react-table"
import {
  Box,
  FileText,
  Info,
  Search,
  SquareTerminal,
} from "lucide-react"
import { toast } from "sonner"

import { Button } from "@/components/ui/button"
import { ButtonGroup } from "@/components/ui/button-group"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { DataTable } from "@/components/ui/data-table"
import { Input } from "@/components/ui/input"
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip"
import { asArray } from "@/lib/as-array"
import { toastRequestError } from "@/lib/network"
import { cn } from "@/lib/utils"

import {
  killContainer,
  pauseContainer,
  removeContainer,
  restartContainer,
  resumeContainer,
  startContainer,
  stopContainer,
} from "../../containers/list/api"
import { formatPorts, stateBadgeClass } from "../../_shared/engine-format"
import { selectColumn } from "../../_shared/select-column"
import { DOCKER_STACKS_KEY, type StackContainer } from "../api"

const columnHelper = createColumnHelper<StackContainer>()

type BulkAction =
  | "start"
  | "stop"
  | "kill"
  | "restart"
  | "pause"
  | "resume"
  | "remove"

function formatCreated(ts?: number) {
  if (!ts) return "—"
  const d = new Date(ts * 1000)
  const pad = (n: number) => String(n).padStart(2, "0")
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
}

function QuickAction({
  label,
  to,
  children,
}: {
  label: string
  to: string
  children: ReactNode
}) {
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <Link
          to={to}
          className="inline-flex size-5 items-center justify-center rounded text-sky-600 transition-colors hover:bg-muted hover:text-sky-700 dark:text-sky-400 dark:hover:text-sky-300"
          aria-label={label}
        >
          {children}
        </Link>
      </TooltipTrigger>
      <TooltipContent>{label}</TooltipContent>
    </Tooltip>
  )
}

export function StackContainersPanel({
  stackName,
  containers,
  onChanged,
}: {
  stackName: string
  containers: StackContainer[]
  onChanged: () => void
}) {
  const queryClient = useQueryClient()
  const [search, setSearch] = useState("")
  const [rowSelection, setRowSelection] = useState<RowSelectionState>({})

  const rows = useMemo(() => {
    const q = search.trim().toLowerCase()
    const list = asArray(containers)
    if (!q) return list
    return list.filter(
      (c) =>
        c.name.toLowerCase().includes(q) ||
        c.image.toLowerCase().includes(q) ||
        (c.service || "").toLowerCase().includes(q)
    )
  }, [containers, search])

  const selectedIds = useMemo(
    () => Object.keys(rowSelection).filter((id) => rowSelection[id]),
    [rowSelection]
  )
  const hasSelection = selectedIds.length > 0

  const bulkMutation = useMutation({
    mutationFn: async ({
      action,
      ids,
    }: {
      action: BulkAction
      ids: string[]
    }) => {
      const run = async (id: string) => {
        switch (action) {
          case "start":
            return startContainer(id)
          case "stop":
            return stopContainer(id)
          case "kill":
            return killContainer(id)
          case "restart":
            return restartContainer(id)
          case "pause":
            return pauseContainer(id)
          case "resume":
            return resumeContainer(id)
          case "remove":
            return removeContainer(id, { force: true })
        }
      }
      const results = await Promise.allSettled(ids.map(run))
      const failed = results.filter((r) => r.status === "rejected").length
      if (failed) {
        throw new Error(`${failed} of ${ids.length} actions failed`)
      }
    },
    onSuccess: (_d, vars) => {
      toast.success(
        `${vars.action} applied to ${vars.ids.length} container${vars.ids.length === 1 ? "" : "s"}`
      )
      setRowSelection({})
      void queryClient.invalidateQueries({ queryKey: [DOCKER_STACKS_KEY] })
      onChanged()
    },
    onError: (err) => toastRequestError(err, "Bulk action failed"),
  })

  const columns = useMemo(() => {
    const cols: ColumnDef<StackContainer, unknown>[] = [
      selectColumn(columnHelper, "container") as ColumnDef<
        StackContainer,
        unknown
      >,
      columnHelper.accessor("name", {
        header: "Name",
        cell: ({ row }) => (
          <Link
            to={`/docker/containers/${row.original.id}`}
            className="font-medium text-sky-600 hover:underline dark:text-sky-400"
          >
            {row.original.name}
          </Link>
        ),
      }) as ColumnDef<StackContainer, unknown>,
      columnHelper.accessor("state", {
        header: "State",
        cell: ({ getValue }) => (
          <span
            className={cn(
              "inline-flex rounded-full px-2 py-0.5 text-[11px] font-medium capitalize",
              stateBadgeClass(getValue())
            )}
          >
            {getValue()}
          </span>
        ),
      }) as ColumnDef<StackContainer, unknown>,
      columnHelper.display({
        id: "quick",
        header: "Quick actions",
        cell: ({ row }) => (
          <div className="flex items-center gap-1">
            <QuickAction
              label="Logs"
              to={`/docker/containers/${row.original.id}/logs`}
            >
              <FileText className="size-3.5" />
            </QuickAction>
            <QuickAction
              label="Inspect"
              to={`/docker/containers/${row.original.id}/inspect`}
            >
              <Info className="size-3.5" />
            </QuickAction>
            <QuickAction
              label="Console"
              to={`/docker/containers/${row.original.id}`}
            >
              <SquareTerminal className="size-3.5" />
            </QuickAction>
          </div>
        ),
        meta: { className: "w-[110px]" },
      }),
      columnHelper.display({
        id: "stack",
        header: "Stack",
        cell: () => (
          <span className="text-muted-foreground">{stackName}</span>
        ),
      }),
      columnHelper.accessor("image", {
        header: "Image",
        cell: ({ getValue }) => (
          <span className="font-mono text-xs text-muted-foreground">
            {getValue()}
          </span>
        ),
      }) as ColumnDef<StackContainer, unknown>,
      columnHelper.accessor("created", {
        header: "Created",
        cell: ({ getValue }) => (
          <span className="whitespace-nowrap text-muted-foreground">
            {formatCreated(getValue())}
          </span>
        ),
      }) as ColumnDef<StackContainer, unknown>,
      columnHelper.accessor("ip_address", {
        header: "IP Address",
        cell: ({ getValue }) => (
          <span className="font-mono text-xs text-muted-foreground">
            {getValue() || "—"}
          </span>
        ),
      }) as ColumnDef<StackContainer, unknown>,
      columnHelper.display({
        id: "ports",
        header: "Published Ports",
        cell: ({ row }) => (
          <span className="font-mono text-xs text-muted-foreground">
            {formatPorts(row.original.ports)}
          </span>
        ),
      }),
    ]
    return cols
  }, [stackName])

  const runBulk = (action: BulkAction) => {
    if (!hasSelection) return
    if (
      (action === "remove" || action === "kill") &&
      !window.confirm(
        `${action === "remove" ? "Remove" : "Kill"} ${selectedIds.length} container(s)?`
      )
    ) {
      return
    }
    bulkMutation.mutate({ action, ids: selectedIds })
  }

  return (
    <Card className="gap-0 py-0">
      <CardHeader className="flex flex-row flex-wrap items-center justify-between gap-3 border-b py-4">
        <CardTitle className="flex items-center gap-2 text-base">
          <Box className="size-4 text-muted-foreground" />
          Containers
        </CardTitle>
        <div className="flex flex-wrap items-center gap-2">
          <div className="relative">
            <Search className="pointer-events-none absolute top-1/2 left-2.5 size-3.5 -translate-y-1/2 text-muted-foreground" />
            <Input
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              placeholder="Search…"
              className="h-8 w-[160px] ps-8"
            />
          </div>
          <TooltipProvider delayDuration={200}>
            <ButtonGroup>
              {(
                [
                  ["start", "Start"],
                  ["stop", "Stop"],
                  ["kill", "Kill"],
                  ["restart", "Restart"],
                  ["pause", "Pause"],
                  ["resume", "Resume"],
                ] as const
              ).map(([action, label]) => (
                <Button
                  key={action}
                  size="sm"
                  variant="outline"
                  disabled={!hasSelection || bulkMutation.isPending}
                  onClick={() => runBulk(action)}
                >
                  {label}
                </Button>
              ))}
              <Button
                size="sm"
                variant="destructive"
                disabled={!hasSelection || bulkMutation.isPending}
                onClick={() => runBulk("remove")}
              >
                Remove
              </Button>
            </ButtonGroup>
          </TooltipProvider>
        </div>
      </CardHeader>
      <CardContent className="p-0">
        <TooltipProvider delayDuration={200}>
          <DataTable
            columns={columns}
            data={rows}
            getRowId={(row) => row.id}
            enableRowSelection
            rowSelection={rowSelection}
            onRowSelectionChange={setRowSelection}
            emptyMessage="No containers found for this stack yet."
            pageSize={10}
          />
        </TooltipProvider>
      </CardContent>
    </Card>
  )
}
