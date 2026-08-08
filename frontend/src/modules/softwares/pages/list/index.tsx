import { useEffect, useMemo, useState } from "react"
import { Link, useNavigate } from "react-router"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { Loader2, Package, Sparkles } from "lucide-react"
import {
  parseAsInteger,
  parseAsString,
  parseAsStringLiteral,
  useQueryStates,
} from "nuqs"
import { toast } from "sonner"

import ContentLoader from "@/components/content-loader"
import { Button } from "@/components/ui/button"
import { asArray } from "@/lib/as-array"
import { getRequestErrorMessage, withError } from "@/lib/network"
import { cn } from "@/lib/utils"

import {
  SoftwareCatalog,
  type SoftwareCatalogFilters,
} from "../../components/software-catalog"
import {
  controlSoftwareService,
  enqueueSoftwareActions,
  getSoftwareQueue,
  getSoftwaresList,
  importRemoteSoftware,
  SOFTWARES_FETCH_KEY,
  type SoftwareListItem,
  type SoftwareQueueAction,
  type SoftwareServiceAction,
} from "./api"

const EMPTY_LIST_ITEMS: SoftwareListItem[] = []

const STATUS_VALUES = [
  "all",
  "installed",
  "update_available",
  "not_installed",
] as const
const SORT_VALUES = ["order", "name", "category", "recent"] as const
const SOURCE_VALUES = ["local", "all", "remote"] as const
const LIMIT_VALUES = [8, 12, 24, 48] as const

const softwareFiltersParsers = {
  q: parseAsString.withDefault(""),
  category: parseAsString.withDefault("all"),
  status: parseAsStringLiteral(STATUS_VALUES).withDefault("all"),
  sort: parseAsStringLiteral(SORT_VALUES).withDefault("order"),
  source: parseAsStringLiteral(SOURCE_VALUES).withDefault("local"),
  page: parseAsInteger.withDefault(1),
  limit: parseAsInteger.withDefault(12),
}

export default function SoftwaresListPage() {
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const [filters, setFilters] = useQueryStates(softwareFiltersParsers, {
    history: "replace",
    clearOnDefault: true,
  })

  // Clamp unsupported limit values from the URL to a known page size.
  const limit = (LIMIT_VALUES as readonly number[]).includes(filters.limit)
    ? filters.limit
    : 12
  const catalogFilters: SoftwareCatalogFilters = {
    ...filters,
    page: Math.max(1, filters.page),
    limit,
  }

  // Debounce search so typing doesn't hammer the API.
  const [qDraft, setQDraft] = useState(catalogFilters.q)
  const [prevQ, setPrevQ] = useState(catalogFilters.q)
  if (catalogFilters.q !== prevQ) {
    setPrevQ(catalogFilters.q)
    setQDraft(catalogFilters.q)
  }
  useEffect(() => {
    if (qDraft === catalogFilters.q) return
    const t = window.setTimeout(() => {
      void setFilters({ q: qDraft.trim(), page: 1 })
    }, 300)
    return () => window.clearTimeout(t)
  }, [qDraft, catalogFilters.q, setFilters])

  const { data, isLoading, isFetching, error } = useQuery({
    queryKey: [SOFTWARES_FETCH_KEY, "list", catalogFilters],
    queryFn: () =>
      getSoftwaresList({
        page: catalogFilters.page,
        limit: catalogFilters.limit,
        q: catalogFilters.q,
        category: catalogFilters.category,
        status: catalogFilters.status,
        sort: catalogFilters.sort,
        source: catalogFilters.source,
        // Re-hit GitHub registries when searching or browsing Remote/All.
        refresh:
          Boolean(catalogFilters.q.trim()) ||
          catalogFilters.source === "remote" ||
          catalogFilters.source === "all",
      }),
    placeholderData: (prev) => prev,
  })

  const serviceMutation = useMutation({
    mutationFn: ({
      id,
      action,
    }: {
      id: string
      action: SoftwareServiceAction
    }) => controlSoftwareService(id, action),
    onSuccess: (res, vars) => {
      toast.success(`${vars.action} · ${res.status.overall}`)
      void queryClient.invalidateQueries({
        queryKey: [SOFTWARES_FETCH_KEY, "list"],
      })
    },
    onError: (err) => {
      toast.error(getRequestErrorMessage(err, "Service action failed"))
    },
  })

  const bulkMutation = useMutation({
    mutationFn: async ({
      action,
      items,
    }: {
      action: SoftwareQueueAction
      items: SoftwareListItem[]
    }) => {
      const ids: string[] = []
      for (const item of items) {
        if (item.is_remote) {
          if (action !== "install") continue
          const imported = await importRemoteSoftware({
            name: item.name,
            packageId: item.package_id,
          })
          ids.push(imported.data.software.id)
          continue
        }
        ids.push(item.id)
      }
      if (ids.length === 0) {
        throw new Error("No software available to queue for this action")
      }
      return enqueueSoftwareActions(action, ids)
    },
    onSuccess: (res, vars) => {
      const n = res.data?.queued ?? vars.items.length
      toast.success(
        res.message ||
          `Queued ${n} ${vars.action}${n === 1 ? "" : "s"} — running one by one`,
        {
          action: {
            label: "View queue",
            onClick: () => navigate("/softwares/installing"),
          },
        }
      )
      void queryClient.invalidateQueries({
        queryKey: [SOFTWARES_FETCH_KEY, "list"],
      })
      void queryClient.invalidateQueries({
        queryKey: [SOFTWARES_FETCH_KEY, "queue"],
      })
      if (vars.action === "uninstall") {
        navigate("/softwares/installing")
      }
    },
    onError: (err) => {
      toast.error(getRequestErrorMessage(err, "Could not queue actions"))
    },
  })

  const openMutation = useMutation({
    mutationFn: async (item: SoftwareListItem) => {
      if (!item.is_remote) return item.id
      const imported = await importRemoteSoftware({
        name: item.name,
        packageId: item.package_id,
      })
      return imported.data.software.id
    },
    onSuccess: (id) => {
      void queryClient.invalidateQueries({
        queryKey: [SOFTWARES_FETCH_KEY, "list"],
      })
      navigate(`/softwares/${id}`)
    },
    onError: (err) => {
      toast.error(getRequestErrorMessage(err, "Could not import package"))
    },
  })

  const queueQuery = useQuery({
    queryKey: [SOFTWARES_FETCH_KEY, "queue"],
    queryFn: getSoftwareQueue,
    refetchInterval: (q) => {
      const pending = q.state.data?.data?.pending ?? 0
      const running = Boolean(q.state.data?.data?.running)
      return pending > 0 || running ? 2000 : false
    },
  })

  // Refresh the catalog while the bulk queue is draining.
  useEffect(() => {
    const snap = queueQuery.data?.data
    if (!snap?.running && !(snap?.pending && snap.pending > 0)) return
    const t = window.setInterval(() => {
      void queryClient.invalidateQueries({
        queryKey: [SOFTWARES_FETCH_KEY, "list"],
      })
    }, 4000)
    return () => window.clearInterval(t)
  }, [queueQuery.data?.data, queryClient])

  const items = data?.data ?? EMPTY_LIST_ITEMS
  const pagination = data?.pagination ?? {
    page: catalogFilters.page,
    limit: catalogFilters.limit,
    total: 0,
    total_pages: 1,
    pageCount: 1,
  }
  const busyAction = serviceMutation.isPending
    ? `${serviceMutation.variables?.id}:${serviceMutation.variables?.action}`
    : null

  const onFiltersChange = (patch: Partial<SoftwareCatalogFilters>) => {
    if (patch.q !== undefined) {
      setQDraft(patch.q)
      if (Object.keys(patch).length === 1) return
    }
    void setFilters({
      ...patch,
      q: patch.q ?? qDraft,
      page: patch.page ?? (patch.q !== undefined ? 1 : catalogFilters.page),
    })
  }

  const queuePending = queueQuery.data?.data?.pending ?? 0
  const queueItems = asArray(queueQuery.data?.data?.items)
  const queueBusy = Boolean(queueQuery.data?.data?.running) || queuePending > 0
  const showInstallingButton = queueBusy || queueItems.length > 0

  const queueBusyById = useMemo(() => {
    const map = new Map<string, "installing" | "updating">()
    for (const it of queueItems) {
      if (it.status !== "pending" && it.status !== "running") continue
      map.set(
        it.software_id,
        it.action === "update" ? "updating" : "installing"
      )
    }
    return map
  }, [queueItems])

  const openSoftware = (item: SoftwareListItem) => {
    openMutation.mutate(item)
  }

  return (
    <ContentLoader
      title="Software"
      breadcrumb={[{ label: "Softwares", to: "/softwares" }]}
      isLoading={isLoading}
      error={withError(error, data)}
      showHeaderSeparator
      headerClassName="gap-4 pb-6"
      customTitle={
        <div className="relative overflow-hidden rounded-2xl border border-border/70 bg-gradient-to-br from-background via-background to-sky-500/5 px-5 py-5 shadow-sm dark:to-sky-500/10">
          <div
            aria-hidden
            className="pointer-events-none absolute -top-16 -right-10 h-40 w-40 rounded-full bg-sky-500/10 blur-3xl"
          />
          <div className="relative flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
            <div className="flex min-w-0 items-start gap-3.5">
              <div className="flex h-12 w-12 shrink-0 items-center justify-center rounded-xl bg-sky-600 text-white shadow-md shadow-sky-600/25">
                <Package className="h-6 w-6" />
              </div>
              <div className="min-w-0 space-y-1">
                <div className="flex flex-wrap items-center gap-2">
                  <h1 className="text-2xl font-semibold tracking-tight text-foreground">
                    Software
                  </h1>
                  <span className="inline-flex items-center gap-1 rounded-full border border-sky-500/20 bg-sky-500/10 px-2 py-0.5 text-[11px] font-medium text-sky-700 dark:text-sky-300">
                    <Sparkles className="h-3 w-3" />
                    Catalog
                  </span>
                </div>
                <p className="max-w-xl text-sm leading-relaxed text-muted-foreground">
                  Packages that match this machine
                  {data?.host?.distro_id
                    ? ` (${data.host.distro_id}${
                        data.host.distro_version
                          ? ` ${data.host.distro_version}`
                          : ""
                      }${data.host.arch ? ` · ${data.host.arch}` : ""})`
                    : ""}
                  — search, filter, and install.
                  {data?.facets?.remote_count
                    ? ` ${data.facets.remote_count} available from remote.`
                    : ""}
                </p>
              </div>
            </div>
            <div className="flex flex-wrap items-center gap-2">
              {showInstallingButton ? (
                <Button
                  asChild
                  size="sm"
                  className="shrink-0 bg-sky-600 text-white shadow-sm hover:bg-sky-600/90"
                >
                  <Link to="/softwares/installing">
                    <Loader2
                      className={cn(
                        "mr-1.5 h-3.5 w-3.5",
                        queueBusy && "animate-spin"
                      )}
                    />
                    Installing
                    {queuePending > 0 ? (
                      <span className="ml-1.5 inline-flex h-5 min-w-5 items-center justify-center rounded-full bg-white/20 px-1.5 text-[11px] font-semibold tabular-nums">
                        {queuePending}
                      </span>
                    ) : null}
                  </Link>
                </Button>
              ) : null}
              <Button asChild size="sm" variant="outline" className="shrink-0">
                <Link to="/softwares/installed">Installed</Link>
              </Button>
              <Button asChild size="sm" variant="outline" className="shrink-0">
                <Link to="/softwares/remotepkg">Remote pkgs</Link>
              </Button>
            </div>
          </div>
        </div>
      }
    >
      <SoftwareCatalog
        items={items}
        facets={data?.facets}
        filters={{ ...catalogFilters, q: qDraft }}
        pagination={{
          page: pagination.page,
          limit: pagination.limit,
          total: pagination.total,
          total_pages: pagination.total_pages || pagination.pageCount || 1,
        }}
        isFetching={isFetching && !isLoading}
        onFiltersChange={onFiltersChange}
        onOpen={openSoftware}
        onUpdate={openSoftware}
        onBulkAction={(action, selected) =>
          bulkMutation.mutate({ action, items: selected })
        }
        bulkBusy={bulkMutation.isPending}
        onServiceAction={(id, action) =>
          serviceMutation.mutate({ id, action })
        }
        busyAction={busyAction}
        queueBusyById={queueBusyById}
      />
    </ContentLoader>
  )
}
