import { useMemo, useState, type ReactNode } from "react"
import { Link, useNavigate } from "react-router"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import {
  createColumnHelper,
  type ColumnDef,
  type RowSelectionState,
} from "@tanstack/react-table"
import {
  Activity,
  ExternalLink,
  FileText,
  Info,
  MoreHorizontal,
  Plus,
  Search,
  SquareTerminal,
} from "lucide-react"
import { toast } from "sonner"

import ContentLoader from "@/components/content-loader"
import { Button } from "@/components/ui/button"
import { ButtonGroup } from "@/components/ui/button-group"
import { Checkbox } from "@/components/ui/checkbox"
import { DataTable } from "@/components/ui/data-table"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { Input } from "@/components/ui/input"
import { Switch } from "@/components/ui/switch"
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip"
import { getRequestErrorMessage, toastRequestError } from "@/lib/network"
import { cn } from "@/lib/utils"

import { EngineBanner } from "../../_shared/engine-status"
import { useEngineDown } from "../../_shared/use-engine-status"
import { stateBadgeClass } from "../../_shared/engine-format"
import { EnvironmentSelector } from "../../_shared/environment-selector"
import {
  DOCKER_PAGE_DESCRIPTIONS,
  DockerRefreshButton,
  SummaryChip,
} from "../../_shared/page-chrome"
import {
  DOCKER_CONTAINERS_KEY,
  killContainer,
  listContainers,
  pauseContainer,
  recreateContainer,
  removeContainer,
  restartContainer,
  resumeContainer,
  startContainer,
  stopContainer,
  type ContainerRow,
} from "./api"
import { asArray } from "@/lib/as-array"

const columnHelper = createColumnHelper<ContainerRow>()

type BulkAction =
  | "start"
  | "stop"
  | "kill"
  | "restart"
  | "pause"
  | "resume"
  | "remove"

type ConfirmState =
  | { kind: "remove" | "kill" | "recreate"; row: ContainerRow }
  | { kind: "bulk-remove" | "bulk-kill"; ids: string[] }
  | null

function formatCreated(ts?: number) {
  if (!ts) return "—"
  const d = new Date(ts * 1000)
  const pad = (n: number) => String(n).padStart(2, "0")
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
}

function QuickAction({
  label,
  to,
  disabled,
  children,
}: {
  label: string
  to?: string
  disabled?: boolean
  children: ReactNode
}) {
  const control =
    to && !disabled ? (
      <Link
        to={to}
        className="inline-flex size-5 items-center justify-center rounded text-sky-600 transition-colors hover:bg-muted hover:text-sky-700 dark:text-sky-400 dark:hover:text-sky-300"
        aria-label={label}
      >
        {children}
      </Link>
    ) : (
      <button
        type="button"
        disabled
        aria-label={label}
        className="inline-flex size-5 items-center justify-center rounded text-muted-foreground/50"
      >
        {children}
      </button>
    )
  return (
    <Tooltip>
      <TooltipTrigger asChild>{control}</TooltipTrigger>
      <TooltipContent side="top">{label}</TooltipContent>
    </Tooltip>
  )
}

export default function DockerContainersPage() {
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const [search, setSearch] = useState("")
  const [rowSelection, setRowSelection] = useState<RowSelectionState>({})
  const [confirm, setConfirm] = useState<ConfirmState>(null)
  const [recreatePull, setRecreatePull] = useState(false)

  const listQuery = useQuery({
    queryKey: [DOCKER_CONTAINERS_KEY],
    queryFn: listContainers,
    refetchInterval: 8_000,
  })

  const invalidate = () =>
    queryClient.invalidateQueries({ queryKey: [DOCKER_CONTAINERS_KEY] })

  const runAction = async (
    id: string,
    action: BulkAction | "recreate",
    opts?: { pull?: boolean }
  ) => {
    switch (action) {
      case "start":
        return startContainer(id)
      case "stop":
        return stopContainer(id)
      case "restart":
        return restartContainer(id)
      case "kill":
        return killContainer(id)
      case "pause":
        return pauseContainer(id)
      case "resume":
        return resumeContainer(id)
      case "remove":
        return removeContainer(id, { force: true })
      case "recreate":
        return recreateContainer(id, { pull: opts?.pull })
    }
  }

  const actionMutation = useMutation({
    mutationFn: async ({
      id,
      action,
      pull,
    }: {
      id: string
      action: BulkAction | "recreate"
      pull?: boolean
    }) => runAction(id, action, { pull }),
    onSuccess: (res, vars) => {
      toast.success(res.message || `Container ${vars.action}`)
      setConfirm(null)
      setRecreatePull(false)
      invalidate()
      if (
        vars.action === "recreate" &&
        "data" in res &&
        res.data &&
        "id" in res.data
      ) {
        const id = (res.data as { id?: string }).id
        if (id) navigate(`/docker/containers/${id}`)
      }
    },
    onError: (err) => toastRequestError(err, "Container action failed"),
  })

  const bulkMutation = useMutation({
    mutationFn: async ({
      ids,
      action,
    }: {
      ids: string[]
      action: BulkAction
    }) => {
      const results = await Promise.allSettled(
        ids.map((id) => runAction(id, action))
      )
      const failed = results.filter((r) => r.status === "rejected").length
      const ok = results.length - failed
      return { ok, failed, total: results.length }
    },
    onSuccess: (res, vars) => {
      if (res.failed === 0) {
        toast.success(`${vars.action}: ${res.ok} container(s)`)
      } else {
        toast.warning(
          `${vars.action}: ${res.ok} ok, ${res.failed} failed of ${res.total}`
        )
      }
      setConfirm(null)
      setRowSelection({})
      invalidate()
    },
    onError: (err) => toastRequestError(err, "Bulk action failed"),
  })

  const rows = asArray(listQuery.data?.data)
  const filtered = useMemo(() => {
    const list = rows
    const q = search.trim().toLowerCase()
    if (!q) return list
    return list.filter(
      (r) =>
        r.name?.toLowerCase().includes(q) ||
        r.image?.toLowerCase().includes(q) ||
        r.short_id?.toLowerCase().includes(q) ||
        r.state?.toLowerCase().includes(q) ||
        r.status?.toLowerCase().includes(q) ||
        r.stack?.toLowerCase().includes(q) ||
        r.ip_address?.toLowerCase().includes(q)
    )
  }, [rows, search])

  const selectedIds = useMemo(
    () => Object.keys(rowSelection).filter((id) => rowSelection[id]),
    [rowSelection]
  )
  const selectedRows = useMemo(
    () => filtered.filter((r) => selectedIds.includes(r.id)),
    [filtered, selectedIds]
  )
  const hasSelection = selectedIds.length > 0
  const anyRunning = selectedRows.some((r) => r.state === "running")
  const anyStopped = selectedRows.some(
    (r) => r.state !== "running" && r.state !== "paused"
  )
  const anyPaused = selectedRows.some((r) => r.state === "paused")

  const columns = useMemo(
    () =>
      [
        columnHelper.display({
          id: "select",
          enableSorting: false,
          header: ({ table }) => (
            <Checkbox
              checked={
                table.getIsAllPageRowsSelected()
                  ? true
                  : table.getIsSomePageRowsSelected()
                    ? "indeterminate"
                    : false
              }
              onCheckedChange={(v) => table.toggleAllPageRowsSelected(!!v)}
              aria-label="Select all"
            />
          ),
          cell: ({ row }) => (
            <Checkbox
              checked={row.getIsSelected()}
              onCheckedChange={(v) => row.toggleSelected(!!v)}
              aria-label={`Select ${row.original.name || row.original.short_id}`}
              onClick={(e) => e.stopPropagation()}
            />
          ),
          meta: { width: 40, className: "w-10 px-2" },
        }),
        columnHelper.accessor("name", {
          header: "Name",
          cell: ({ row }) => {
            const label = row.original.name || row.original.short_id
            return (
              <Link
                to={`/docker/containers/${row.original.id}`}
                title={label}
                className="block max-w-[14rem] truncate font-medium text-sky-600 hover:underline dark:text-sky-400"
              >
                {label}
              </Link>
            )
          },
          meta: { className: "max-w-[14rem]" },
        }),
        columnHelper.accessor("state", {
          header: "State",
          cell: ({ row }) => (
            <span
              className={cn(
                "inline-flex rounded-full px-2 py-0.5 text-xs font-medium capitalize",
                stateBadgeClass(row.original.state)
              )}
              title={row.original.status}
            >
              {row.original.state}
            </span>
          ),
        }),
        columnHelper.display({
          id: "quick",
          enableSorting: false,
          header: "Quick actions",
          cell: ({ row }) => {
            const id = row.original.id
            const running = row.original.state === "running"
            return (
              <div className="flex items-center gap-0.5">
                <QuickAction
                  label="Logs"
                  to={`/docker/containers/${id}/logs`}
                >
                  <FileText className="size-3" />
                </QuickAction>
                <QuickAction
                  label="Inspect"
                  to={`/docker/containers/${id}/inspect`}
                >
                  <Info className="size-3" />
                </QuickAction>
                <QuickAction
                  label="Stats"
                  to={`/docker/containers/${id}/stats`}
                >
                  <Activity className="size-3" />
                </QuickAction>
                <QuickAction
                  label={running ? "Console" : "Start container to open console"}
                  to={running ? `/docker/containers/${id}/console` : undefined}
                  disabled={!running}
                >
                  <SquareTerminal className="size-3" />
                </QuickAction>
              </div>
            )
          },
        }),
        columnHelper.accessor("stack", {
          header: "Stack",
          cell: ({ getValue }) => (
            <span className="text-muted-foreground text-xs">
              {getValue() || "—"}
            </span>
          ),
        }),
        columnHelper.accessor("image", {
          header: "Image",
          cell: ({ getValue }) => {
            const text = getValue()
            return (
              <Link
                to="/docker/images"
                title={text}
                className="block max-w-[16rem] truncate font-mono text-xs text-sky-600 hover:underline dark:text-sky-400"
              >
                {text}
              </Link>
            )
          },
          meta: { className: "max-w-[16rem]" },
        }),
        columnHelper.accessor("created", {
          header: "Created",
          cell: ({ getValue }) => (
            <span className="whitespace-nowrap font-mono text-xs text-muted-foreground">
              {formatCreated(getValue())}
            </span>
          ),
        }),
        columnHelper.accessor((row) => row.ip_address || row.ip_addresses?.[0] || "", {
          id: "ip",
          header: "IP address",
          cell: ({ row }) => {
            const ips =
              row.original.ip_addresses?.length
                ? row.original.ip_addresses
                : row.original.ip_address
                  ? [row.original.ip_address]
                  : []
            return (
              <span className="font-mono text-xs text-muted-foreground">
                {ips.length ? ips.join(", ") : "—"}
              </span>
            )
          },
        }),
        columnHelper.display({
          id: "ports",
          enableSorting: false,
          header: "Published ports",
          cell: ({ row }) => {
            const published = (row.original.ports || []).filter(
              (p) => p.public_port
            )
            if (!published.length) {
              return <span className="text-muted-foreground text-xs">—</span>
            }
            return (
              <div className="flex flex-col gap-0.5">
                {published.map((p, i) => {
                  const host = p.ip && p.ip !== "0.0.0.0" ? p.ip : "localhost"
                  const href = `http://${host}:${p.public_port}`
                  return (
                    <a
                      key={`${p.public_port}-${p.private_port}-${i}`}
                      href={href}
                      target="_blank"
                      rel="noreferrer"
                      className="inline-flex items-center gap-1 font-mono text-xs text-sky-600 hover:underline dark:text-sky-400"
                    >
                      {p.public_port}:{p.private_port}
                      <ExternalLink className="size-3 opacity-70" />
                    </a>
                  )
                })}
              </div>
            )
          },
        }),
        columnHelper.display({
          id: "actions",
          enableSorting: false,
          header: "",
          cell: ({ row }) => {
            const c = row.original
            const running = c.state === "running"
            const paused = c.state === "paused"
            return (
              <DropdownMenu>
                <DropdownMenuTrigger asChild>
                  <Button variant="ghost" size="icon-sm">
                    <MoreHorizontal className="size-4" />
                  </Button>
                </DropdownMenuTrigger>
                <DropdownMenuContent align="end">
                  <DropdownMenuItem
                    onClick={() => navigate(`/docker/containers/${c.id}`)}
                  >
                    Open
                  </DropdownMenuItem>
                  <DropdownMenuItem
                    onClick={() =>
                      navigate(`/docker/containers/edit?id=${encodeURIComponent(c.id)}`)
                    }
                  >
                    Edit
                  </DropdownMenuItem>
                  {running ? (
                    <DropdownMenuItem
                      onClick={() =>
                        actionMutation.mutate({ id: c.id, action: "stop" })
                      }
                    >
                      Stop
                    </DropdownMenuItem>
                  ) : paused ? (
                    <DropdownMenuItem
                      onClick={() =>
                        actionMutation.mutate({ id: c.id, action: "resume" })
                      }
                    >
                      Resume
                    </DropdownMenuItem>
                  ) : (
                    <DropdownMenuItem
                      onClick={() =>
                        actionMutation.mutate({ id: c.id, action: "start" })
                      }
                    >
                      Start
                    </DropdownMenuItem>
                  )}
                  {running ? (
                    <DropdownMenuItem
                      onClick={() =>
                        actionMutation.mutate({ id: c.id, action: "pause" })
                      }
                    >
                      Pause
                    </DropdownMenuItem>
                  ) : null}
                  <DropdownMenuItem
                    onClick={() =>
                      actionMutation.mutate({ id: c.id, action: "restart" })
                    }
                  >
                    Restart
                  </DropdownMenuItem>
                  <DropdownMenuItem
                    onClick={() => setConfirm({ kind: "kill", row: c })}
                  >
                    Kill
                  </DropdownMenuItem>
                  <DropdownMenuItem
                    onClick={() => {
                      setRecreatePull(false)
                      setConfirm({ kind: "recreate", row: c })
                    }}
                  >
                    Recreate
                  </DropdownMenuItem>
                  <DropdownMenuSeparator />
                  <DropdownMenuItem
                    className="text-destructive"
                    onClick={() => setConfirm({ kind: "remove", row: c })}
                  >
                    Remove
                  </DropdownMenuItem>
                </DropdownMenuContent>
              </DropdownMenu>
            )
          },
          meta: { width: 48, className: "w-12" },
        }),
      ] as ColumnDef<ContainerRow, unknown>[],
    [actionMutation, navigate]
  )

  const engineDown = useEngineDown()
  const listErrorIsEngine =
    listQuery.isError &&
    /not reachable|docker engine|connection refused/i.test(
      getRequestErrorMessage(listQuery.error, ""),
    )

  const runningCount = useMemo(
    () => rows.filter((r) => r.state === "running").length,
    [rows],
  )
  const stoppedCount = useMemo(
    () => rows.filter((r) => r.state !== "running").length,
    [rows],
  )

  const busy = actionMutation.isPending || bulkMutation.isPending

  const runBulk = (action: BulkAction) => {
    if (!selectedIds.length) return
    if (action === "remove") {
      setConfirm({ kind: "bulk-remove", ids: selectedIds })
      return
    }
    if (action === "kill") {
      setConfirm({ kind: "bulk-kill", ids: selectedIds })
      return
    }
    bulkMutation.mutate({ ids: selectedIds, action })
  }

  return (
    <ContentLoader
      title="Containers"
      description={DOCKER_PAGE_DESCRIPTIONS.containers}
      showHeaderSeparator
      breadcrumb={[
        { label: "Docker", to: "/docker" },
        { label: "Containers" },
      ]}
      isLoading={listQuery.isLoading}
      error={engineDown || listErrorIsEngine ? undefined : listQuery.error}
      rightComponent={
        <div className="flex flex-wrap items-center gap-2">
          <EnvironmentSelector />
          <DockerRefreshButton
            onClick={() => invalidate()}
            isFetching={listQuery.isFetching}
          />
        </div>
      }
    >
      <TooltipProvider delayDuration={200}>
        <div className="flex flex-col gap-4">
          <EngineBanner />
          <DataTable
            columns={columns}
            data={filtered}
            dense
            enableRowSelection
            enableSorting
            enablePagination
            pageSize={10}
            paginationResetKey={search}
            rowSelection={rowSelection}
            onRowSelectionChange={setRowSelection}
            getRowId={(row) => row.id}
            emptyMessage="No containers found. Create one to get started."
            toolbarStart={
              <div className="flex flex-wrap items-center gap-3">
                <div className="relative w-full max-w-xs min-w-[12rem]">
                  <Search className="pointer-events-none absolute top-1/2 left-2.5 size-3.5 -translate-y-1/2 text-muted-foreground" />
                  <Input
                    className="h-8 pl-8"
                    placeholder="Search…"
                    value={search}
                    onChange={(e) => setSearch(e.target.value)}
                  />
                </div>
                <div className="flex flex-wrap items-center gap-1.5">
                  <SummaryChip>
                    {rows.length} total
                  </SummaryChip>
                  <SummaryChip>
                    {runningCount} running
                  </SummaryChip>
                  <SummaryChip>
                    {stoppedCount} other
                  </SummaryChip>
                </div>
                {hasSelection ? (
                  <span className="text-xs text-muted-foreground">
                    {selectedIds.length} selected
                  </span>
                ) : null}
              </div>
            }
            toolbar={
              <div className="flex flex-wrap items-center gap-2">
                <ButtonGroup aria-label="Container actions">
                  <BulkBtn
                    disabled={!hasSelection || busy || !anyStopped}
                    onClick={() => runBulk("start")}
                  >
                    Start
                  </BulkBtn>
                  <BulkBtn
                    disabled={!hasSelection || busy || !anyRunning}
                    onClick={() => runBulk("stop")}
                  >
                    Stop
                  </BulkBtn>
                  <BulkBtn
                    disabled={!hasSelection || busy || !anyRunning}
                    onClick={() => runBulk("kill")}
                  >
                    Kill
                  </BulkBtn>
                  <BulkBtn
                    disabled={!hasSelection || busy}
                    onClick={() => runBulk("restart")}
                  >
                    Restart
                  </BulkBtn>
                  <BulkBtn
                    disabled={!hasSelection || busy || !anyRunning}
                    onClick={() => runBulk("pause")}
                  >
                    Pause
                  </BulkBtn>
                  <BulkBtn
                    disabled={!hasSelection || busy || !anyPaused}
                    onClick={() => runBulk("resume")}
                  >
                    Resume
                  </BulkBtn>
                  <BulkBtn
                    variant="destructive"
                    disabled={!hasSelection || busy}
                    onClick={() => runBulk("remove")}
                  >
                    Remove
                  </BulkBtn>
                </ButtonGroup>
                <Button
                  size="sm"
                  onClick={() => navigate("/docker/containers/edit")}
                >
                  <Plus data-icon="inline-start" />
                  Add container
                </Button>
              </div>
            }
          />
        </div>
      </TooltipProvider>

      <Dialog
        open={Boolean(confirm)}
        onOpenChange={(v) => {
          if (!v) {
            setConfirm(null)
            setRecreatePull(false)
          }
        }}
      >
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>
              {confirm?.kind === "remove" || confirm?.kind === "bulk-remove"
                ? "Remove container"
                : confirm?.kind === "kill" || confirm?.kind === "bulk-kill"
                  ? "Kill container"
                  : "Recreate container"}
            </DialogTitle>
            <DialogDescription>
              {confirm?.kind === "remove"
                ? `Force-remove ${confirm.row.name || confirm.row.short_id}? This cannot be undone.`
                : confirm?.kind === "bulk-remove"
                  ? `Force-remove ${confirm.ids.length} container(s)? This cannot be undone.`
                  : confirm?.kind === "kill"
                    ? `Send SIGKILL to ${confirm.row.name || confirm.row.short_id}?`
                    : confirm?.kind === "bulk-kill"
                      ? `Send SIGKILL to ${confirm.ids.length} container(s)?`
                      : confirm?.kind === "recreate"
                        ? `Recreate ${confirm.row.name || confirm.row.short_id} with the same configuration? The current container will be stopped and replaced.`
                        : null}
            </DialogDescription>
          </DialogHeader>
          {confirm?.kind === "recreate" ? (
            <div className="flex items-center justify-between gap-3 rounded-lg border px-4 py-3">
              <div className="space-y-0.5">
                <p className="text-sm font-medium">Re-pull image</p>
                <p className="text-xs text-muted-foreground">
                  Pull the latest{" "}
                  <code className="text-[11px]">{confirm.row.image}</code>{" "}
                  before recreating.
                </p>
              </div>
              <Switch
                checked={recreatePull}
                onCheckedChange={setRecreatePull}
                disabled={busy}
              />
            </div>
          ) : null}
          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => {
                setConfirm(null)
                setRecreatePull(false)
              }}
            >
              Cancel
            </Button>
            <Button
              variant={
                confirm?.kind === "remove" || confirm?.kind === "bulk-remove"
                  ? "destructive"
                  : "default"
              }
              disabled={busy}
              onClick={() => {
                if (!confirm) return
                if (confirm.kind === "bulk-remove") {
                  bulkMutation.mutate({ ids: confirm.ids, action: "remove" })
                  return
                }
                if (confirm.kind === "bulk-kill") {
                  bulkMutation.mutate({ ids: confirm.ids, action: "kill" })
                  return
                }
                if (
                  confirm.kind === "remove" ||
                  confirm.kind === "kill" ||
                  confirm.kind === "recreate"
                ) {
                  actionMutation.mutate({
                    id: confirm.row.id,
                    action: confirm.kind,
                    pull:
                      confirm.kind === "recreate" ? recreatePull : undefined,
                  })
                }
              }}
            >
              {busy
                ? confirm?.kind === "recreate" && recreatePull
                  ? "Pulling & recreating…"
                  : "Working…"
                : confirm?.kind === "recreate"
                  ? "Recreate"
                  : "Confirm"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </ContentLoader>
  )
}

function BulkBtn({
  children,
  disabled,
  onClick,
  variant = "outline",
}: {
  children: ReactNode
  disabled?: boolean
  onClick: () => void
  variant?: "outline" | "destructive"
}) {
  return (
    <Button size="sm" variant={variant} disabled={disabled} onClick={onClick}>
      {children}
    </Button>
  )
}
