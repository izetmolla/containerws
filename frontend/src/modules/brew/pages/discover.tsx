import { useEffect, useMemo, useState } from "react"
import { Link, useNavigate } from "react-router"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import {
  ArrowRightLeft,
  ChevronLeft,
  ChevronRight,
  Download,
  ExternalLink,
  Loader2,
  RefreshCw,
  Search,
  Trash2,
} from "lucide-react"
import { toast } from "sonner"

import ContentLoader from "@/components/content-loader"
import { useDebounce } from "@/components/layouts/dashboard1/hooks/use-debounce"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { getRequestErrorMessage, withError } from "@/lib/network"
import { cn } from "@/lib/utils"
import { SOFTWARES_FETCH_KEY } from "@/modules/softwares/pages/list/api"

import {
  BREW_ANALYTICS_KEY,
  BREW_FORMULAE_KEY,
  BREW_INSTALLED_KEY,
  BREW_STATUS_KEY,
  getBrewAnalytics,
  getBrewFormulae,
  getBrewInstalled,
  getBrewStatus,
  runBrewAction,
  switchPackageManager,
  type BrewFormula,
} from "./api"
import { FormulaGlyph, HomebrewMark } from "./formula-glyph"
import { BrewInstallGate } from "./install-gate"

const PAGE_SIZE = 24

function FormulaCard({
  item,
  busy,
  onInstall,
  onUpgrade,
  onUninstall,
  onSwitch,
}: {
  item: BrewFormula
  busy?: boolean
  onInstall: (item: BrewFormula) => void
  onUpgrade: (item: BrewFormula) => void
  onUninstall: (item: BrewFormula) => void
  onSwitch?: (softwareId: string, target: "local" | "brew") => void
}) {
  const ownedBySoftwares =
    item.ownership === "local" || item.package_manager === "local"
  const ownedByBrew =
    item.ownership === "brew" || item.package_manager === "brew"
  const isCask = item.kind === "cask"
  const title = item.display_name || item.name
  const detailTo = `/brew/${encodeURIComponent(item.name)}${
    item.kind ? `?kind=${encodeURIComponent(item.kind)}` : ""
  }`

  return (
    <div className="group flex flex-col gap-3 rounded-xl border bg-card p-4 shadow-xs transition-colors hover:border-foreground/20">
      <div className="flex items-start gap-3">
        <FormulaGlyph
          name={title}
          icon={item.icon}
          category={item.category}
        />
        <div className="min-w-0 flex-1">
          <div className="flex items-start justify-between gap-2">
            <Link
              to={detailTo}
              className="truncate text-sm font-semibold tracking-tight hover:underline"
            >
              {title}
            </Link>
            <div className="flex shrink-0 flex-col items-end gap-1">
              {item.installed ? (
                <Badge variant={item.outdated ? "secondary" : "default"}>
                  {item.outdated ? "Update" : "Installed"}
                </Badge>
              ) : (
                <Badge variant="outline">Available</Badge>
              )}
            </div>
          </div>
          {item.display_name && item.display_name !== item.name ? (
            <p className="mt-0.5 font-mono text-[10px] text-muted-foreground">
              {item.name}
            </p>
          ) : null}
          <p className="mt-1 line-clamp-2 text-xs leading-relaxed text-muted-foreground">
            {item.desc || "No description"}
          </p>
          <div className="mt-2 flex flex-wrap items-center gap-1.5">
            {item.version ? (
              <span className="font-mono text-[10px] text-muted-foreground">
                v{item.version}
              </span>
            ) : null}
            <Badge variant="outline" className="h-5 px-1.5 text-[10px]">
              {isCask ? "App" : "Formula"}
            </Badge>
            {item.category ? (
              <Badge variant="outline" className="h-5 px-1.5 text-[10px]">
                {item.category}
              </Badge>
            ) : null}
            {ownedBySoftwares ? (
              <Badge variant="outline" className="h-5 px-1.5 text-[10px]">
                Softwares
              </Badge>
            ) : null}
            {ownedByBrew ? (
              <Badge variant="secondary" className="h-5 px-1.5 text-[10px]">
                Brew
              </Badge>
            ) : null}
          </div>
        </div>
      </div>
      <div className="mt-auto flex flex-wrap gap-2 border-t pt-3">
        {!item.installed ? (
          <Button
            size="sm"
            disabled={busy || ownedBySoftwares}
            onClick={() => onInstall(item)}
          >
            <Download className="size-3.5" />
            Install
          </Button>
        ) : null}
        {item.installed && item.outdated ? (
          <Button
            size="sm"
            variant="secondary"
            disabled={busy || ownedBySoftwares}
            onClick={() => onUpgrade(item)}
          >
            <RefreshCw className="size-3.5" />
            Upgrade
          </Button>
        ) : null}
        {item.installed ? (
          <Button
            size="sm"
            variant="outline"
            disabled={busy || ownedBySoftwares}
            onClick={() => onUninstall(item)}
          >
            <Trash2 className="size-3.5" />
            Uninstall
          </Button>
        ) : null}
        {item.can_switch_to_brew && item.software_id && onSwitch ? (
          <Button
            size="sm"
            variant="ghost"
            onClick={() => onSwitch(item.software_id!, "brew")}
          >
            <ArrowRightLeft className="size-3.5" />
            Use Brew
          </Button>
        ) : null}
        <Button size="sm" variant="ghost" asChild>
          <Link to={detailTo}>Details</Link>
        </Button>
      </div>
    </div>
  )
}

function PaginationBar({
  page,
  totalPages,
  total,
  limit,
  onPageChange,
  isFetching,
}: {
  page: number
  totalPages: number
  total: number
  limit: number
  onPageChange: (page: number) => void
  isFetching?: boolean
}) {
  const windowPages = useMemo(() => {
    const pages: number[] = []
    const start = Math.max(1, page - 2)
    const end = Math.min(totalPages, page + 2)
    for (let i = start; i <= end; i++) pages.push(i)
    return pages
  }, [page, totalPages])

  if (totalPages <= 1) return null
  const from = total === 0 ? 0 : (page - 1) * limit + 1
  const to = Math.min(page * limit, total)

  return (
    <div className="flex flex-col gap-3 border-t pt-4 sm:flex-row sm:items-center sm:justify-between">
      <p className="text-xs text-muted-foreground">
        Showing {from}–{to} of {total.toLocaleString()} packages
        {isFetching ? " · Updating…" : null}
      </p>
      <div className="flex flex-wrap items-center gap-1">
        <Button
          type="button"
          size="sm"
          variant="outline"
          disabled={page <= 1}
          onClick={() => onPageChange(page - 1)}
        >
          <ChevronLeft className="size-3.5" />
          Prev
        </Button>
        {windowPages[0] > 1 ? (
          <>
            <Button
              type="button"
              size="sm"
              variant={page === 1 ? "default" : "outline"}
              onClick={() => onPageChange(1)}
            >
              1
            </Button>
            {windowPages[0] > 2 ? (
              <span className="px-1 text-xs text-muted-foreground">…</span>
            ) : null}
          </>
        ) : null}
        {windowPages.map((p) => (
          <Button
            key={p}
            type="button"
            size="sm"
            variant={p === page ? "default" : "outline"}
            onClick={() => onPageChange(p)}
          >
            {p}
          </Button>
        ))}
        {windowPages[windowPages.length - 1] < totalPages ? (
          <>
            {windowPages[windowPages.length - 1] < totalPages - 1 ? (
              <span className="px-1 text-xs text-muted-foreground">…</span>
            ) : null}
            <Button
              type="button"
              size="sm"
              variant={page === totalPages ? "default" : "outline"}
              onClick={() => onPageChange(totalPages)}
            >
              {totalPages}
            </Button>
          </>
        ) : null}
        <Button
          type="button"
          size="sm"
          variant="outline"
          disabled={page >= totalPages}
          onClick={() => onPageChange(page + 1)}
        >
          Next
          <ChevronRight className="size-3.5" />
        </Button>
      </div>
    </div>
  )
}

export default function BrewDiscoverPage() {
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const [q, setQ] = useState("")
  const debouncedQ = useDebounce(q, 500).trim()
  const [category, setCategory] = useState("")
  const [page, setPage] = useState(1)
  const [busyName, setBusyName] = useState<string | null>(null)

  useEffect(() => {
    setPage(1)
  }, [debouncedQ, category])

  const statusQuery = useQuery({
    queryKey: [BREW_STATUS_KEY],
    queryFn: getBrewStatus,
    refetchInterval: (q) =>
      q.state.data?.data?.bootstrap?.running ? 2000 : false,
  })

  const status = statusQuery.data?.data

  const formulaeQuery = useQuery({
    queryKey: [BREW_FORMULAE_KEY, debouncedQ, category, page],
    queryFn: () =>
      getBrewFormulae({
        q: debouncedQ || undefined,
        category: category || undefined,
        page,
        limit: PAGE_SIZE,
      }),
    enabled: Boolean(status?.binary_present),
    placeholderData: (prev) => prev,
  })

  const analyticsQuery = useQuery({
    queryKey: [BREW_ANALYTICS_KEY, 30],
    queryFn: () => getBrewAnalytics(30),
    enabled: Boolean(status?.binary_present) && !debouncedQ && !category,
  })

  const installedQuery = useQuery({
    queryKey: [BREW_INSTALLED_KEY],
    queryFn: getBrewInstalled,
    enabled: Boolean(status?.binary_present),
  })

  const actionMutation = useMutation({
    mutationFn: async ({
      action,
      name,
      kind,
    }: {
      action: "install" | "upgrade" | "uninstall"
      name: string
      kind?: string
    }) => {
      setBusyName(name)
      return runBrewAction(action, [name], kind)
    },
    onSuccess: (res, vars) => {
      toast.success(res.message || `${vars.action} ${vars.name} queued`, {
        action: {
          label: "View queue",
          onClick: () => navigate("/softwares/installing"),
        },
      })
      void queryClient.invalidateQueries({
        queryKey: [SOFTWARES_FETCH_KEY, "queue"],
      })
      void queryClient.invalidateQueries({ queryKey: [BREW_FORMULAE_KEY] })
      void queryClient.invalidateQueries({ queryKey: [BREW_INSTALLED_KEY] })
      navigate("/softwares/installing")
    },
    onError: (err) => {
      toast.error(getRequestErrorMessage(err, "Could not queue Brew action"))
    },
    onSettled: () => setBusyName(null),
  })

  const switchMutation = useMutation({
    mutationFn: ({
      softwareId,
      target,
    }: {
      softwareId: string
      target: "local" | "brew"
    }) => switchPackageManager(softwareId, target),
    onSuccess: (res) => {
      toast.success(res.message || "Switched package manager")
      void queryClient.invalidateQueries({ queryKey: [BREW_FORMULAE_KEY] })
      void queryClient.invalidateQueries({ queryKey: [BREW_INSTALLED_KEY] })
      void queryClient.invalidateQueries({ queryKey: [SOFTWARES_FETCH_KEY] })
    },
    onError: (err) => {
      toast.error(getRequestErrorMessage(err, "Switch failed"))
    },
  })

  const categories = formulaeQuery.data?.data?.categories ?? []
  const items = formulaeQuery.data?.data?.items ?? []
  const total = formulaeQuery.data?.data?.total ?? 0
  const totalPages = Math.max(
    1,
    formulaeQuery.data?.data?.total_pages ??
      (total > 0 ? Math.ceil(total / PAGE_SIZE) : 1)
  )
  const popular = analyticsQuery.data?.data?.items?.slice(0, 8) ?? []
  const installedCount = installedQuery.data?.data?.items?.length ?? 0

  if (statusQuery.isLoading) {
    return (
      <ContentLoader
        title="Brew Manager"
        description="Homebrew formulae and desktop apps"
        isLoading
        breadcrumb={[{ label: "Brew Manager" }]}
      >
        <div />
      </ContentLoader>
    )
  }

  if (status && !status.binary_present) {
    return (
      <ContentLoader
        title="Brew Manager"
        description="Homebrew formulae and desktop apps"
        breadcrumb={[{ label: "Brew Manager" }]}
        error={withError(statusQuery.error, statusQuery.data)}
      >
        <BrewInstallGate
          status={status}
          onReady={() => {
            void queryClient.invalidateQueries({ queryKey: [BREW_STATUS_KEY] })
          }}
        />
      </ContentLoader>
    )
  }

  return (
    <ContentLoader
      title="Brew Manager"
      description="Browse Homebrew formulae and desktop apps (casks) with search, categories, and one-click installs."
      breadcrumb={[{ label: "Brew Manager" }]}
      isLoading={formulaeQuery.isLoading && !formulaeQuery.data}
      error={withError(formulaeQuery.error, formulaeQuery.data)}
      showHeaderSeparator
      rightComponent={
        <div className="flex gap-2">
          <Button variant="outline" size="sm" asChild>
            <Link to="/brew/installed">Installed ({installedCount})</Link>
          </Button>
          <Button variant="outline" size="sm" asChild>
            <a href="https://brew.sh/" target="_blank" rel="noreferrer">
              brew.sh
              <ExternalLink className="size-3.5" />
            </a>
          </Button>
        </div>
      }
    >
      <div className="space-y-8">
        <section className="relative overflow-hidden rounded-2xl border bg-[#2e2a24] text-[#f5f2eb] shadow-sm">
          <div
            aria-hidden
            className="pointer-events-none absolute inset-0 bg-[radial-gradient(ellipse_at_top_right,rgba(245,166,35,0.28),transparent_55%)]"
          />
          <div className="relative flex flex-col gap-6 p-6 md:flex-row md:items-center md:justify-between md:p-8">
            <div className="flex min-w-0 items-start gap-4">
              <div className="flex size-16 shrink-0 items-center justify-center rounded-2xl bg-white/95 p-2 shadow-sm">
                <HomebrewMark className="size-12" />
              </div>
              <div className="min-w-0 space-y-2">
                <p className="text-xs font-medium tracking-[0.16em] text-[#f5a623] uppercase">
                  Homebrew
                </p>
                <h2 className="text-2xl font-semibold tracking-tight md:text-3xl">
                  The package manager for everywhere
                </h2>
                <p className="max-w-xl text-sm text-white/70">
                  Search formulae and desktop apps from Homebrew, install with
                  your local <code className="text-white/90">brew</code> CLI,
                  and keep Softwares ownership in sync.
                </p>
              </div>
            </div>
            <div className="flex shrink-0 flex-col gap-2 sm:flex-row md:flex-col">
              <Button
                className="bg-[#f5a623] text-[#2e2a24] hover:bg-[#ffb84d]"
                onClick={() => {
                  const el = document.getElementById("brew-search")
                  el?.focus()
                }}
              >
                <Search className="size-4" />
                Browse packages
              </Button>
              <Button
                variant="outline"
                className="border-white/20 bg-transparent text-white hover:bg-white/10 hover:text-white"
                asChild
              >
                <Link to="/brew/installed">View installed</Link>
              </Button>
            </div>
          </div>
        </section>

        <section className="space-y-3">
          <div className="relative">
            <Search className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-muted-foreground" />
            <Input
              id="brew-search"
              value={q}
              onChange={(e) => setQ(e.target.value)}
              placeholder="Search packages, apps, or descriptions…"
              className="h-11 pl-10"
            />
          </div>
          <div className="flex flex-wrap gap-2">
            <button
              type="button"
              onClick={() => setCategory("")}
              className={cn(
                "rounded-full border px-3 py-1 text-xs transition-colors",
                !category
                  ? "border-foreground bg-foreground text-background"
                  : "hover:bg-muted"
              )}
            >
              All
            </button>
            {categories
              .slice()
              .sort((a, b) => a.name.localeCompare(b.name))
              .map((c) => (
                <button
                  key={c.name}
                  type="button"
                  onClick={() =>
                    setCategory((prev) => (prev === c.name ? "" : c.name))
                  }
                  className={cn(
                    "rounded-full border px-3 py-1 text-xs transition-colors",
                    category === c.name
                      ? "border-foreground bg-foreground text-background"
                      : "hover:bg-muted"
                  )}
                >
                  {c.name} ({c.count})
                </button>
              ))}
          </div>
        </section>

        {!debouncedQ && !category && popular.length > 0 ? (
          <section className="space-y-3">
            <div className="flex items-center justify-between gap-2">
              <h3 className="text-sm font-medium">Popular this month</h3>
              <span className="text-xs text-muted-foreground">
                Homebrew analytics · 30 days
              </span>
            </div>
            <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
              {popular.map((item) => (
                <FormulaCard
                  key={`${item.kind || "formula"}:${item.name}`}
                  item={item}
                  busy={busyName === item.name || switchMutation.isPending}
                  onInstall={(pkg) =>
                    actionMutation.mutate({
                      action: "install",
                      name: pkg.name,
                      kind: pkg.kind,
                    })
                  }
                  onUpgrade={(pkg) =>
                    actionMutation.mutate({
                      action: "upgrade",
                      name: pkg.name,
                      kind: pkg.kind,
                    })
                  }
                  onUninstall={(pkg) =>
                    actionMutation.mutate({
                      action: "uninstall",
                      name: pkg.name,
                      kind: pkg.kind,
                    })
                  }
                />
              ))}
            </div>
          </section>
        ) : null}

        <section className="space-y-3">
          <div className="flex items-center justify-between gap-2">
            <h3 className="text-sm font-medium">
              {debouncedQ || category ? "Results" : "All packages"}
            </h3>
            <div className="flex items-center gap-2 text-xs text-muted-foreground">
              {formulaeQuery.isFetching ? (
                <Loader2 className="size-3.5 animate-spin" />
              ) : null}
              <span>
                Page {page} of {totalPages} · {total.toLocaleString()} total
              </span>
            </div>
          </div>
          <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
            {items.map((item) => (
              <FormulaCard
                key={`${item.kind || "formula"}:${item.name}`}
                item={item}
                busy={busyName === item.name || switchMutation.isPending}
                onInstall={(pkg) =>
                  actionMutation.mutate({
                    action: "install",
                    name: pkg.name,
                    kind: pkg.kind,
                  })
                }
                onUpgrade={(pkg) =>
                  actionMutation.mutate({
                    action: "upgrade",
                    name: pkg.name,
                    kind: pkg.kind,
                  })
                }
                onUninstall={(pkg) =>
                  actionMutation.mutate({
                    action: "uninstall",
                    name: pkg.name,
                    kind: pkg.kind,
                  })
                }
                onSwitch={(softwareId, target) =>
                  switchMutation.mutate({ softwareId, target })
                }
              />
            ))}
          </div>
          {items.length === 0 && !formulaeQuery.isLoading ? (
            <p className="rounded-xl border border-dashed px-4 py-10 text-center text-sm text-muted-foreground">
              No packages matched your search.
            </p>
          ) : null}
          <PaginationBar
            page={page}
            totalPages={totalPages}
            total={total}
            limit={PAGE_SIZE}
            isFetching={formulaeQuery.isFetching}
            onPageChange={(next) => {
              setPage(next)
              window.scrollTo({ top: 0, behavior: "smooth" })
            }}
          />
        </section>
      </div>
    </ContentLoader>
  )
}
