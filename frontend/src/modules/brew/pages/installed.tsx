import { useDeferredValue, useMemo, useState } from "react"
import { Link, useNavigate } from "react-router"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import {
  ArrowRightLeft,
  Download,
  RefreshCw,
  Search,
  Trash2,
} from "lucide-react"
import { toast } from "sonner"

import ContentLoader from "@/components/content-loader"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Checkbox } from "@/components/ui/checkbox"
import { Input } from "@/components/ui/input"
import { getRequestErrorMessage, withError } from "@/lib/network"
import { SOFTWARES_FETCH_KEY } from "@/modules/softwares/pages/list/api"

import {
  BREW_FORMULAE_KEY,
  BREW_INSTALLED_KEY,
  BREW_STATUS_KEY,
  checkBrewUpdates,
  getBrewInstalled,
  getBrewStatus,
  runBrewAction,
  switchPackageManager,
  type BrewFormula,
} from "./api"
import { FormulaGlyph } from "./formula-glyph"
import { BrewInstallGate } from "./install-gate"

function itemKey(item: BrewFormula) {
  return `${item.kind || "formula"}:${item.name}`
}

function isBrewManaged(item: BrewFormula) {
  return item.package_manager !== "local"
}

async function runBrewActionByKind(
  action: "upgrade" | "uninstall",
  items: BrewFormula[]
) {
  const formulae = items
    .filter((i) => (i.kind || "formula") !== "cask")
    .map((i) => i.name)
  const casks = items
    .filter((i) => i.kind === "cask")
    .map((i) => i.name)
  let queued = 0
  let lastMessage = ""
  if (formulae.length > 0) {
    const res = await runBrewAction(action, formulae, "formula")
    const data = res.data as { queued?: number } | undefined
    queued += data?.queued ?? formulae.length
    lastMessage = res.message || lastMessage
  }
  if (casks.length > 0) {
    const res = await runBrewAction(action, casks, "cask")
    const data = res.data as { queued?: number } | undefined
    queued += data?.queued ?? casks.length
    lastMessage = res.message || lastMessage
  }
  return { queued, message: lastMessage }
}

export default function BrewInstalledPage() {
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const [busyName, setBusyName] = useState<string | null>(null)
  const [search, setSearch] = useState("")
  const deferredSearch = useDeferredValue(search)
  const [selected, setSelected] = useState<Set<string>>(new Set())

  const statusQuery = useQuery({
    queryKey: [BREW_STATUS_KEY],
    queryFn: getBrewStatus,
  })
  const status = statusQuery.data?.data

  const installedQuery = useQuery({
    queryKey: [BREW_INSTALLED_KEY],
    queryFn: getBrewInstalled,
    enabled: Boolean(status?.binary_present),
  })

  const items = installedQuery.data?.data?.items ?? []

  const filtered = useMemo(() => {
    const q = deferredSearch.trim().toLowerCase()
    if (!q) return items
    return items.filter((item) => {
      const hay = [
        item.name,
        item.display_name,
        item.desc,
        item.category,
        item.kind,
      ]
        .filter(Boolean)
        .join(" ")
        .toLowerCase()
      return hay.includes(q)
    })
  }, [items, deferredSearch])

  const selectable = useMemo(
    () => filtered.filter((i) => isBrewManaged(i)),
    [filtered]
  )

  const allFilteredSelected =
    selectable.length > 0 && selectable.every((i) => selected.has(itemKey(i)))
  const someFilteredSelected = selectable.some((i) => selected.has(itemKey(i)))

  const selectedItems = useMemo(
    () => items.filter((i) => selected.has(itemKey(i)) && isBrewManaged(i)),
    [items, selected]
  )
  const selectedOutdated = selectedItems.filter((i) => i.outdated)
  const outdatedCount = items.filter(
    (i) => i.outdated && isBrewManaged(i)
  ).length

  const invalidateBrew = () => {
    void queryClient.invalidateQueries({
      queryKey: [SOFTWARES_FETCH_KEY, "queue"],
    })
    void queryClient.invalidateQueries({ queryKey: [BREW_INSTALLED_KEY] })
    void queryClient.invalidateQueries({ queryKey: [BREW_FORMULAE_KEY] })
  }

  const actionMutation = useMutation({
    mutationFn: async ({
      action,
      name,
      kind,
    }: {
      action: "upgrade" | "uninstall"
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
      invalidateBrew()
      navigate("/softwares/installing")
    },
    onError: (err) => {
      toast.error(getRequestErrorMessage(err, "Could not queue Brew action"))
    },
    onSettled: () => setBusyName(null),
  })

  const bulkMutation = useMutation({
    mutationFn: async ({
      action,
      targets,
    }: {
      action: "upgrade" | "uninstall"
      targets: BrewFormula[]
    }) => runBrewActionByKind(action, targets),
    onSuccess: (res, vars) => {
      toast.success(
        res.message ||
          `Queued ${res.queued || vars.targets.length} ${vars.action}(s)`,
        {
          action: {
            label: "View queue",
            onClick: () => navigate("/softwares/installing"),
          },
        }
      )
      setSelected(new Set())
      invalidateBrew()
      navigate("/softwares/installing")
    },
    onError: (err) => {
      toast.error(getRequestErrorMessage(err, "Bulk action failed"))
    },
  })

  const checkUpdatesMutation = useMutation({
    mutationFn: () => checkBrewUpdates({ upgrade: true }),
    onSuccess: (res) => {
      const queued = res.data?.queued ?? 0
      const outdated = res.data?.outdated?.length ?? 0
      toast.success(res.message || `Found ${outdated} outdated`, {
        action:
          queued > 0
            ? {
                label: "View queue",
                onClick: () => navigate("/softwares/installing"),
              }
            : undefined,
      })
      setSelected(new Set())
      invalidateBrew()
      if (queued > 0) {
        navigate("/softwares/installing")
      }
    },
    onError: (err) => {
      toast.error(getRequestErrorMessage(err, "Check for updates failed"))
    },
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
      void queryClient.invalidateQueries({ queryKey: [BREW_INSTALLED_KEY] })
      void queryClient.invalidateQueries({ queryKey: [SOFTWARES_FETCH_KEY] })
    },
    onError: (err) => {
      toast.error(getRequestErrorMessage(err, "Switch failed"))
    },
  })

  const toggleOne = (item: BrewFormula, on: boolean) => {
    if (!isBrewManaged(item)) return
    const key = itemKey(item)
    setSelected((prev) => {
      const next = new Set(prev)
      if (on) next.add(key)
      else next.delete(key)
      return next
    })
  }

  const toggleAllFiltered = (on: boolean) => {
    setSelected((prev) => {
      const next = new Set(prev)
      for (const item of selectable) {
        const key = itemKey(item)
        if (on) next.add(key)
        else next.delete(key)
      }
      return next
    })
  }

  if (status && !status.binary_present) {
    return (
      <ContentLoader
        title="Installed"
        description="Formulae installed via Homebrew"
        breadcrumb={[
          { label: "Brew Manager", to: "/brew" },
          { label: "Installed" },
        ]}
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

  const busyBulk =
    bulkMutation.isPending ||
    checkUpdatesMutation.isPending ||
    actionMutation.isPending

  return (
    <ContentLoader
      title="Installed"
      description="Packages installed via Homebrew on this host."
      breadcrumb={[
        { label: "Brew Manager", to: "/brew" },
        { label: "Installed" },
      ]}
      isLoading={installedQuery.isLoading}
      error={withError(installedQuery.error, installedQuery.data)}
      showHeaderSeparator
      rightComponent={
        <div className="flex flex-wrap items-center gap-2">
          <Button
            variant="secondary"
            size="sm"
            disabled={busyBulk || !status?.binary_present}
            onClick={() => checkUpdatesMutation.mutate()}
          >
            <RefreshCw
              className={`size-3.5 ${checkUpdatesMutation.isPending ? "animate-spin" : ""}`}
            />
            {checkUpdatesMutation.isPending
              ? "Checking…"
              : outdatedCount > 0
                ? `Check & update (${outdatedCount})`
                : "Check for updates"}
          </Button>
          <Button variant="outline" size="sm" asChild>
            <Link to="/brew">Browse packages</Link>
          </Button>
        </div>
      }
    >
      <div className="flex flex-col gap-3">
        <div className="relative max-w-md">
          <Search className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-muted-foreground" />
          <Input
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="Search installed packages…"
            className="ps-9"
            aria-label="Search installed packages"
          />
        </div>

        {selectedItems.length > 0 ? (
          <div className="flex flex-wrap items-center gap-2 rounded-xl border bg-muted/30 px-3 py-2">
            <span className="text-sm text-muted-foreground">
              {selectedItems.length} selected
              {selectedOutdated.length > 0
                ? ` · ${selectedOutdated.length} outdated`
                : ""}
            </span>
            <div className="ms-auto flex flex-wrap gap-1.5">
              <Button
                size="sm"
                variant="secondary"
                disabled={busyBulk || selectedOutdated.length === 0}
                onClick={() =>
                  bulkMutation.mutate({
                    action: "upgrade",
                    targets: selectedOutdated,
                  })
                }
              >
                <RefreshCw className="size-3.5" />
                Upgrade selected
              </Button>
              <Button
                size="sm"
                variant="outline"
                disabled={busyBulk}
                onClick={() =>
                  bulkMutation.mutate({
                    action: "uninstall",
                    targets: selectedItems,
                  })
                }
              >
                <Trash2 className="size-3.5" />
                Uninstall selected
              </Button>
              <Button
                size="sm"
                variant="ghost"
                disabled={busyBulk}
                onClick={() => setSelected(new Set())}
              >
                Clear
              </Button>
            </div>
          </div>
        ) : null}

        <div className="overflow-hidden rounded-xl border">
          <table className="w-full text-sm">
            <thead className="border-b bg-muted/40 text-left text-xs text-muted-foreground">
              <tr>
                <th className="w-10 px-3 py-2">
                  <Checkbox
                    checked={
                      allFilteredSelected
                        ? true
                        : someFilteredSelected
                          ? "indeterminate"
                          : false
                    }
                    onCheckedChange={(v) => toggleAllFiltered(v === true)}
                    disabled={selectable.length === 0}
                    aria-label="Select all visible"
                  />
                </th>
                <th className="px-3 py-2 font-medium">Package</th>
                <th className="px-3 py-2 font-medium">Type</th>
                <th className="px-3 py-2 font-medium">Version</th>
                <th className="px-3 py-2 font-medium">Manager</th>
                <th className="px-3 py-2 font-medium">Status</th>
                <th className="px-3 py-2 font-medium" />
              </tr>
            </thead>
            <tbody>
              {filtered.map((item) => {
                const ownedLocal = item.package_manager === "local"
                const key = itemKey(item)
                const isSelected = selected.has(key)
                const busy =
                  busyName === item.name ||
                  switchMutation.isPending ||
                  busyBulk
                const detailTo = `/brew/${encodeURIComponent(item.name)}${
                  item.kind ? `?kind=${encodeURIComponent(item.kind)}` : ""
                }`
                return (
                  <tr
                    key={key}
                    className="border-b last:border-0 data-[selected=true]:bg-muted/25"
                    data-selected={isSelected || undefined}
                  >
                    <td className="px-3 py-2">
                      <Checkbox
                        checked={isSelected}
                        disabled={ownedLocal}
                        onCheckedChange={(v) => toggleOne(item, v === true)}
                        aria-label={`Select ${item.display_name || item.name}`}
                      />
                    </td>
                    <td className="px-3 py-2">
                      <div className="flex items-center gap-2.5">
                        <FormulaGlyph
                          name={item.display_name || item.name}
                          icon={item.icon}
                          category={item.category}
                          size="sm"
                        />
                        <div className="min-w-0">
                          <Link
                            to={detailTo}
                            className="font-medium hover:underline"
                          >
                            {item.display_name || item.name}
                          </Link>
                          {item.desc ? (
                            <p className="line-clamp-1 text-xs text-muted-foreground">
                              {item.desc}
                            </p>
                          ) : null}
                        </div>
                      </div>
                    </td>
                    <td className="px-3 py-2">
                      <Badge variant="outline">
                        {item.kind === "cask" ? "App" : "Formula"}
                      </Badge>
                    </td>
                    <td className="px-3 py-2 text-muted-foreground">
                      {item.version || item.installed_version || "—"}
                    </td>
                    <td className="px-3 py-2">
                      {ownedLocal ? (
                        <Badge variant="outline">Softwares</Badge>
                      ) : (
                        <Badge variant="secondary">Brew</Badge>
                      )}
                    </td>
                    <td className="px-3 py-2">
                      {item.outdated ? (
                        <Badge variant="secondary">Outdated</Badge>
                      ) : (
                        <Badge variant="default">Up to date</Badge>
                      )}
                    </td>
                    <td className="px-3 py-2">
                      <div className="flex flex-wrap justify-end gap-1">
                        {item.outdated && !ownedLocal ? (
                          <Button
                            size="sm"
                            variant="secondary"
                            disabled={busy}
                            onClick={() =>
                              actionMutation.mutate({
                                action: "upgrade",
                                name: item.name,
                                kind: item.kind,
                              })
                            }
                          >
                            <RefreshCw className="size-3.5" />
                            Upgrade
                          </Button>
                        ) : null}
                        {!ownedLocal ? (
                          <Button
                            size="sm"
                            variant="outline"
                            disabled={busy}
                            onClick={() =>
                              actionMutation.mutate({
                                action: "uninstall",
                                name: item.name,
                                kind: item.kind,
                              })
                            }
                          >
                            <Trash2 className="size-3.5" />
                            Uninstall
                          </Button>
                        ) : null}
                        {ownedLocal && item.software_id ? (
                          <Button
                            size="sm"
                            variant="outline"
                            disabled={busy}
                            onClick={() =>
                              switchMutation.mutate({
                                softwareId: item.software_id!,
                                target: "brew",
                              })
                            }
                          >
                            <ArrowRightLeft className="size-3.5" />
                            Switch to Brew
                          </Button>
                        ) : null}
                        {item.package_manager === "brew" && item.software_id ? (
                          <Button
                            size="sm"
                            variant="outline"
                            disabled={busy}
                            onClick={() =>
                              switchMutation.mutate({
                                softwareId: item.software_id!,
                                target: "local",
                              })
                            }
                          >
                            <Download className="size-3.5" />
                            Switch to Local
                          </Button>
                        ) : null}
                      </div>
                    </td>
                  </tr>
                )
              })}
              {filtered.length === 0 ? (
                <tr>
                  <td
                    colSpan={7}
                    className="px-3 py-8 text-center text-muted-foreground"
                  >
                    {items.length === 0
                      ? "No formulae installed yet."
                      : "No packages match your search."}
                  </td>
                </tr>
              ) : null}
            </tbody>
          </table>
        </div>
      </div>
    </ContentLoader>
  )
}
