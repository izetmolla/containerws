import { useMemo, useState } from "react"
import { Link, useNavigate } from "react-router"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import {
  createColumnHelper,
  type ColumnDef,
} from "@tanstack/react-table"
import { MoreHorizontal, Plus, Search } from "lucide-react"
import { toast } from "sonner"

import ContentLoader from "@/components/content-loader"
import { Button } from "@/components/ui/button"
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
import { asArray } from "@/lib/as-array"
import { toastRequestError } from "@/lib/network"
import { cn } from "@/lib/utils"

import { EngineBanner } from "../_shared/engine-status"
import { useEngineDown } from "../_shared/use-engine-status"
import { stateBadgeClass } from "../_shared/engine-format"
import { EnvironmentSelector } from "../_shared/environment-selector"
import {
  DOCKER_PAGE_DESCRIPTIONS,
  DockerRefreshButton,
  SummaryChip,
} from "../_shared/page-chrome"
import {
  DOCKER_STACKS_KEY,
  deployStack,
  listStacks,
  removeStack,
  stopStack,
  type StackRow,
} from "./api"

const columnHelper = createColumnHelper<StackRow>()

export default function DockerStacksPage() {
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const engineDown = useEngineDown()
  const [search, setSearch] = useState("")
  const [removeTarget, setRemoveTarget] = useState<StackRow | null>(null)

  const listQuery = useQuery({
    queryKey: [DOCKER_STACKS_KEY],
    queryFn: listStacks,
    refetchInterval: 12_000,
  })

  const invalidate = () =>
    queryClient.invalidateQueries({ queryKey: [DOCKER_STACKS_KEY] })

  const deployMutation = useMutation({
    mutationFn: (id: string) => deployStack(id),
    onSuccess: (res) => {
      toast.success(res.message || "Stack deployed")
      invalidate()
    },
    onError: (err) => toastRequestError(err, "Deploy failed"),
  })

  const stopMutation = useMutation({
    mutationFn: (id: string) => stopStack(id),
    onSuccess: (res) => {
      toast.success(res.message || "Stack stopped")
      invalidate()
    },
    onError: (err) => toastRequestError(err, "Stop failed"),
  })

  const removeMutation = useMutation({
    mutationFn: (id: string) => removeStack(id, true),
    onSuccess: (res) => {
      toast.success(res.message || "Stack removed")
      setRemoveTarget(null)
      invalidate()
    },
    onError: (err) => toastRequestError(err, "Remove failed"),
  })

  const rows = asArray(listQuery.data?.data)
  const filtered = useMemo(() => {
    const q = search.trim().toLowerCase()
    if (!q) return rows
    return rows.filter(
      (r) =>
        r.name.toLowerCase().includes(q) ||
        r.status.toLowerCase().includes(q) ||
        (r.template_title || "").toLowerCase().includes(q)
    )
  }, [rows, search])

  const columns = useMemo(
    () =>
      [
        columnHelper.accessor("name", {
          header: "Name",
          enableSorting: true,
          cell: ({ row }) => (
            <Link
              to={`/docker/stacks/edit?id=${encodeURIComponent(row.original.id)}`}
              className="font-medium text-sky-600 hover:underline dark:text-sky-400"
            >
              {row.original.name}
            </Link>
          ),
        }),
        columnHelper.accessor("status", {
          header: "Status",
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
        }),
        columnHelper.display({
          id: "containers",
          header: "Containers",
          cell: ({ row }) => (
            <span className="tabular-nums text-muted-foreground">
              {row.original.running_count}/{row.original.container_count}
            </span>
          ),
        }),
        columnHelper.accessor("template_title", {
          header: "Template",
          cell: ({ getValue }) => getValue() || "—",
        }),
        columnHelper.accessor("updated_at", {
          header: "Updated",
          cell: ({ getValue }) => {
            const v = getValue()
            try {
              return new Date(v).toLocaleString()
            } catch {
              return v
            }
          },
        }),
        columnHelper.display({
          id: "actions",
          enableSorting: false,
          cell: ({ row }) => (
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button variant="ghost" size="icon-sm">
                  <MoreHorizontal className="size-4" />
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end">
                <DropdownMenuItem
                  onClick={() =>
                    navigate(
                      `/docker/stacks/edit?id=${encodeURIComponent(row.original.id)}`
                    )
                  }
                >
                  Edit
                </DropdownMenuItem>
                <DropdownMenuItem
                  onClick={() => deployMutation.mutate(row.original.id)}
                >
                  Deploy / Update
                </DropdownMenuItem>
                <DropdownMenuItem
                  onClick={() => stopMutation.mutate(row.original.id)}
                >
                  Stop
                </DropdownMenuItem>
                <DropdownMenuSeparator />
                <DropdownMenuItem
                  className="text-destructive"
                  onClick={() => setRemoveTarget(row.original)}
                >
                  Remove
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          ),
          meta: { width: 48, className: "w-12" },
        }),
      ] as ColumnDef<StackRow, unknown>[],
    [deployMutation, navigate, stopMutation]
  )

  return (
    <ContentLoader
      title="Stacks"
      description={DOCKER_PAGE_DESCRIPTIONS.stacks}
      showHeaderSeparator
      breadcrumb={[
        { label: "Docker", to: "/docker" },
        { label: "Stacks" },
      ]}
      isLoading={listQuery.isLoading}
      error={engineDown ? undefined : listQuery.error}
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
      <div className="flex flex-col gap-4">
        <EngineBanner />
        <DataTable
          columns={columns}
          data={filtered}
          dense
          enableSorting
          enablePagination
          pageSize={10}
          paginationResetKey={search}
          emptyMessage="No stacks yet. Create a Compose stack to deploy services."
          toolbarStart={
            <div className="flex flex-wrap items-center gap-3">
              <div className="relative w-full max-w-xs min-w-[12rem]">
                <Search className="pointer-events-none absolute top-1/2 left-2.5 size-3.5 -translate-y-1/2 text-muted-foreground" />
                <Input
                  className="h-8 pl-8"
                  placeholder="Search stacks…"
                  value={search}
                  onChange={(e) => setSearch(e.target.value)}
                />
              </div>
              <SummaryChip>{rows.length} stacks</SummaryChip>
            </div>
          }
          toolbar={
            <Button size="sm" onClick={() => navigate("/docker/stacks/edit")}>
              <Plus data-icon="inline-start" />
              Add stack
            </Button>
          }
        />
      </div>

      <Dialog
        open={Boolean(removeTarget)}
        onOpenChange={(v) => !v && setRemoveTarget(null)}
      >
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>Remove stack</DialogTitle>
            <DialogDescription>
              Stop and remove stack {removeTarget?.name}? Containers created by
              this Compose project will be removed (volumes are kept).
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setRemoveTarget(null)}>
              Cancel
            </Button>
            <Button
              variant="destructive"
              disabled={removeMutation.isPending}
              onClick={() =>
                removeTarget && removeMutation.mutate(removeTarget.id)
              }
            >
              Remove
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </ContentLoader>
  )
}
