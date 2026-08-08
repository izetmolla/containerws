import { useMemo, useState } from "react"
import { Link, useNavigate } from "react-router"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import {
  ArrowLeft,
  ArrowUpCircle,
  CheckCircle2,
  Loader2,
  PackageCheck,
  RefreshCw,
} from "lucide-react"
import { toast } from "sonner"

import ContentLoader from "@/components/content-loader"
import { Button } from "@/components/ui/button"
import { getRequestErrorMessage, withError } from "@/lib/network"

import { SoftwareList } from "../../components/software-list"
import {
  controlSoftwareService,
  enqueueSoftwareActions,
  getSoftwareQueue,
  getSoftwaresList,
  INSTALLED_SOFTWARES_FETCH_KEY,
  SOFTWARES_FETCH_KEY,
  type SoftwareListItem,
  type SoftwareServiceAction,
} from "./api"

export default function SoftwaresInstalledPage() {
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set())

  const listQuery = useQuery({
    queryKey: [INSTALLED_SOFTWARES_FETCH_KEY, "list"],
    queryFn: () =>
      getSoftwaresList({
        page: 1,
        limit: 100,
        status: "all",
        sort: "name",
      }),
  })

  const queueQuery = useQuery({
    queryKey: [SOFTWARES_FETCH_KEY, "queue"],
    queryFn: getSoftwareQueue,
    refetchInterval: (q) => {
      const items = q.state.data?.data?.items ?? []
      const busy = items.some(
        (i) => i.status === "pending" || i.status === "running"
      )
      return busy || Boolean(q.state.data?.data?.running) ? 1500 : 5000
    },
  })

  const queueBusyById = useMemo(() => {
    const map = new Map<string, "installing" | "updating">()
    for (const it of queueQuery.data?.data?.items ?? []) {
      if (it.status !== "pending" && it.status !== "running") continue
      map.set(
        it.software_id,
        it.action === "update" ? "updating" : "installing"
      )
    }
    return map
  }, [queueQuery.data?.data?.items])

  const queueRelevantIds = useMemo(() => {
    const ids = new Set<string>()
    for (const it of queueQuery.data?.data?.items ?? []) {
      if (
        it.status === "pending" ||
        it.status === "running" ||
        it.status === "error"
      ) {
        ids.add(it.software_id)
      }
    }
    return ids
  }, [queueQuery.data?.data?.items])

  const items = useMemo(() => {
    const all = listQuery.data?.data ?? []
    return all.filter(
      (item) =>
        Boolean(item.is_installed) ||
        Boolean(item.uninstalled) ||
        Boolean(item.os_missing) ||
        queueRelevantIds.has(item.id)
    )
  }, [listQuery.data?.data, queueRelevantIds])

  const updateCount = useMemo(
    () =>
      items.filter((i) => i.has_update && !i.os_missing && !i.uninstalled)
        .length,
    [items]
  )
  const missingCount = useMemo(
    () => items.filter((i) => i.os_missing && !i.uninstalled).length,
    [items]
  )
  const uninstalledCount = useMemo(
    () => items.filter((i) => i.uninstalled).length,
    [items]
  )
  const installingCount = queueBusyById.size

  const allSelected =
    items.length > 0 && items.every((i) => selectedIds.has(i.id))
  const someSelected =
    !allSelected && items.some((i) => selectedIds.has(i.id))

  const toggleSelected = (id: string, next?: boolean) => {
    setSelectedIds((prev) => {
      const copy = new Set(prev)
      const shouldSelect = next ?? !copy.has(id)
      if (shouldSelect) copy.add(id)
      else copy.delete(id)
      return copy
    })
  }

  const toggleSelectAll = () => {
    if (allSelected) {
      setSelectedIds(new Set())
      return
    }
    setSelectedIds(new Set(items.map((i) => i.id)))
  }

  const checkUpdatesMutation = useMutation({
    mutationFn: async () => {
      const res = await queryClient.fetchQuery({
        queryKey: [INSTALLED_SOFTWARES_FETCH_KEY, "list"],
        queryFn: () =>
          getSoftwaresList({
            page: 1,
            limit: 100,
            status: "all",
            sort: "name",
          }),
      })
      const next = (res.data ?? []).filter(
        (item) =>
          Boolean(item.is_installed) ||
          Boolean(item.uninstalled) ||
          Boolean(item.os_missing) ||
          queueRelevantIds.has(item.id)
      )
      return next.filter(
        (i) => i.has_update && !i.os_missing && !i.uninstalled
      ).length
    },
    onSuccess: (count) => {
      void queryClient.invalidateQueries({
        queryKey: [SOFTWARES_FETCH_KEY, "list"],
      })
      if (count === 0) {
        toast.success("All installed software is up to date")
        return
      }
      toast.message(
        count === 1
          ? "1 update available"
          : `${count} updates available`
      )
    },
    onError: (err) => {
      toast.error(getRequestErrorMessage(err, "Could not check for updates"))
    },
  })

  const updateMutation = useMutation({
    mutationFn: (targets: SoftwareListItem[]) =>
      enqueueSoftwareActions(
        "update",
        targets.map((t) => t.id)
      ),
    onSuccess: (res, targets) => {
      const n = res.data?.queued ?? targets.length
      toast.success(res.message || `Queued ${n} update(s)`, {
        action: {
          label: "View queue",
          onClick: () => navigate("/softwares/installing"),
        },
      })
      void queryClient.invalidateQueries({
        queryKey: [SOFTWARES_FETCH_KEY, "queue"],
      })
      setSelectedIds(new Set())
      navigate("/softwares/installing")
    },
    onError: (err) => {
      toast.error(getRequestErrorMessage(err, "Could not queue updates"))
    },
  })

  const serviceMutation = useMutation({
    mutationFn: ({
      id,
      action,
    }: {
      id: string
      action: SoftwareServiceAction
    }) => controlSoftwareService(id, action),
    onSuccess: (_res, vars) => {
      toast.success(`Service ${vars.action} requested`)
      void queryClient.invalidateQueries({
        queryKey: [INSTALLED_SOFTWARES_FETCH_KEY, "list"],
      })
    },
    onError: (err) => {
      toast.error(getRequestErrorMessage(err, "Service action failed"))
    },
  })

  const busyAction = serviceMutation.isPending
    ? `${serviceMutation.variables?.id}:${serviceMutation.variables?.action}`
    : null

  const updatableSelected = items.filter(
    (i) =>
      selectedIds.has(i.id) && i.has_update && !i.os_missing && !i.uninstalled
  )
  const updatableAll = items.filter(
    (i) => i.has_update && !i.os_missing && !i.uninstalled
  )

  return (
    <ContentLoader
      title="Installed"
      breadcrumb={[
        { label: "Softwares", to: "/softwares" },
        { label: "Installed", to: "/softwares/installed" },
      ]}
      isLoading={listQuery.isLoading}
      error={withError(listQuery.error, listQuery.data)}
      showHeaderSeparator
      headerClassName="gap-4 pb-6"
      customTitle={
        <div className="relative overflow-hidden rounded-2xl border border-border/70 bg-gradient-to-br from-background via-background to-emerald-500/5 px-5 py-5 shadow-sm dark:to-emerald-500/10">
          <div
            aria-hidden
            className="pointer-events-none absolute -top-16 -right-10 h-40 w-40 rounded-full bg-emerald-500/10 blur-3xl"
          />
          <div className="relative flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
            <div className="flex min-w-0 items-start gap-3.5">
              <div className="flex h-12 w-12 shrink-0 items-center justify-center rounded-xl bg-emerald-600 text-white shadow-md shadow-emerald-600/25">
                <PackageCheck className="h-6 w-6" />
              </div>
              <div className="min-w-0 space-y-1">
                <div className="flex flex-wrap items-center gap-2">
                  <h1 className="text-2xl font-semibold tracking-tight text-foreground">
                    Installed
                  </h1>
                  <span className="inline-flex items-center gap-1 rounded-full border border-emerald-500/20 bg-emerald-500/10 px-2 py-0.5 text-[11px] font-medium text-emerald-700 dark:text-emerald-300">
                    <CheckCircle2 className="h-3 w-3" />
                    {items.length}
                  </span>
                  {missingCount > 0 ? (
                    <span className="inline-flex items-center rounded-full border border-amber-500/30 bg-amber-500/10 px-2 py-0.5 text-[11px] font-medium text-amber-800 dark:text-amber-300">
                      {missingCount} missing
                    </span>
                  ) : null}
                  {uninstalledCount > 0 ? (
                    <span className="inline-flex items-center rounded-full border border-zinc-500/30 bg-zinc-500/10 px-2 py-0.5 text-[11px] font-medium text-zinc-700 dark:text-zinc-300">
                      {uninstalledCount} uninstalled
                    </span>
                  ) : null}
                  {updateCount > 0 ? (
                    <span className="inline-flex items-center rounded-full border border-amber-500/30 bg-amber-500/10 px-2 py-0.5 text-[11px] font-medium text-amber-800 dark:text-amber-300">
                      {updateCount} update{updateCount === 1 ? "" : "s"}
                    </span>
                  ) : null}
                  {installingCount > 0 ? (
                    <span className="inline-flex items-center gap-1 rounded-full border border-sky-500/30 bg-sky-500/10 px-2 py-0.5 text-[11px] font-medium text-sky-700 dark:text-sky-300">
                      <Loader2 className="h-3 w-3 animate-spin" />
                      {installingCount} installing
                    </span>
                  ) : null}
                </div>
                <p className="max-w-xl text-sm leading-relaxed text-muted-foreground">
                  Software on this host — including intentionally uninstalled
                  apps (kept here, not auto-reinstalled), missing restores, and
                  install-queue items.
                </p>
              </div>
            </div>
            <div className="flex flex-wrap items-center gap-2">
              <Button variant="outline" size="sm" asChild>
                <Link to="/softwares">
                  <ArrowLeft className="mr-1.5 h-3.5 w-3.5" />
                  Catalog
                </Link>
              </Button>
              <Button
                size="sm"
                variant="outline"
                disabled={checkUpdatesMutation.isPending}
                onClick={() => checkUpdatesMutation.mutate()}
              >
                {checkUpdatesMutation.isPending ? (
                  <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" />
                ) : (
                  <RefreshCw className="mr-1.5 h-3.5 w-3.5" />
                )}
                Check for updates
              </Button>
              <Button
                size="sm"
                className="bg-amber-500 text-black hover:bg-amber-400"
                disabled={
                  updateMutation.isPending ||
                  (updatableSelected.length === 0 && updatableAll.length === 0)
                }
                onClick={() => {
                  const targets =
                    updatableSelected.length > 0
                      ? updatableSelected
                      : updatableAll
                  if (targets.length === 0) {
                    toast.message("No updates available")
                    return
                  }
                  updateMutation.mutate(targets)
                }}
              >
                {updateMutation.isPending ? (
                  <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" />
                ) : (
                  <ArrowUpCircle className="mr-1.5 h-3.5 w-3.5" />
                )}
                {updatableSelected.length > 0
                  ? `Update selected (${updatableSelected.length})`
                  : updateCount > 0
                    ? `Update all (${updateCount})`
                    : "Update all"}
              </Button>
            </div>
          </div>
        </div>
      }
    >
      {items.length === 0 ? (
        <div className="flex flex-col items-center justify-center rounded-xl border border-dashed border-border/80 px-6 py-16 text-center">
          <PackageCheck className="mb-3 h-10 w-10 text-muted-foreground/50" />
          <h2 className="text-lg font-medium">No installed software</h2>
          <p className="mt-1 max-w-sm text-sm text-muted-foreground">
            Install apps from the catalog. Missing and queued installs will
            appear here automatically.
          </p>
          <Button asChild className="mt-4" size="sm">
            <Link to="/softwares">Browse catalog</Link>
          </Button>
        </div>
      ) : (
        <SoftwareList
          items={items}
          selectedIds={selectedIds}
          allSelected={allSelected}
          someSelected={someSelected}
          onToggleSelect={toggleSelected}
          onToggleSelectAll={toggleSelectAll}
          onClick={(s) => navigate(`/softwares/${s.id}`)}
          onUpdate={(s) => navigate(`/softwares/${s.id}`)}
          onServiceAction={(id, action) =>
            serviceMutation.mutate({ id, action })
          }
          busyAction={busyAction}
          queueBusyById={queueBusyById}
        />
      )}
    </ContentLoader>
  )
}
