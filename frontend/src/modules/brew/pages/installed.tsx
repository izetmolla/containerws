import { useState } from "react"
import { Link } from "react-router"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import {
  ArrowRightLeft,
  Download,
  RefreshCw,
  Trash2,
} from "lucide-react"
import { toast } from "sonner"

import ContentLoader from "@/components/content-loader"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { getRequestErrorMessage, withError } from "@/lib/network"
import { SOFTWARES_FETCH_KEY } from "@/modules/softwares/pages/list/api"

import {
  BREW_FORMULAE_KEY,
  BREW_INSTALLED_KEY,
  BREW_STATUS_KEY,
  getBrewInstalled,
  getBrewJob,
  getBrewStatus,
  runBrewAction,
  switchPackageManager,
} from "./api"
import { FormulaGlyph } from "./formula-glyph"
import { BrewInstallGate } from "./install-gate"

async function waitBrewJob(id: string) {
  for (let i = 0; i < 600; i++) {
    await new Promise((r) => setTimeout(r, 1000))
    const snap = await getBrewJob(id)
    const st = snap.data?.status
    if (st === "success" || st === "error") {
      if (st === "error") {
        throw new Error(snap.data?.error || "Brew action failed")
      }
      return snap
    }
  }
  throw new Error("Timed out waiting for brew job")
}

export default function BrewInstalledPage() {
  const queryClient = useQueryClient()
  const [busyName, setBusyName] = useState<string | null>(null)

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
      const job = await runBrewAction(action, [name], kind)
      const id = job.data?.id
      if (!id) return job
      return waitBrewJob(id)
    },
    onSuccess: (_res, vars) => {
      toast.success(`${vars.action} ${vars.name} completed`)
      void queryClient.invalidateQueries({ queryKey: [BREW_INSTALLED_KEY] })
      void queryClient.invalidateQueries({ queryKey: [BREW_FORMULAE_KEY] })
    },
    onError: (err) => {
      toast.error(getRequestErrorMessage(err, "Brew action failed"))
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
      void queryClient.invalidateQueries({ queryKey: [BREW_INSTALLED_KEY] })
      void queryClient.invalidateQueries({ queryKey: [SOFTWARES_FETCH_KEY] })
    },
    onError: (err) => {
      toast.error(getRequestErrorMessage(err, "Switch failed"))
    },
  })

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

  const items = installedQuery.data?.data?.items ?? []

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
        <Button variant="outline" size="sm" asChild>
          <Link to="/brew">Browse packages</Link>
        </Button>
      }
    >
      <div className="overflow-hidden rounded-xl border">
        <table className="w-full text-sm">
          <thead className="border-b bg-muted/40 text-left text-xs text-muted-foreground">
            <tr>
              <th className="px-3 py-2 font-medium">Package</th>
              <th className="px-3 py-2 font-medium">Type</th>
              <th className="px-3 py-2 font-medium">Version</th>
              <th className="px-3 py-2 font-medium">Manager</th>
              <th className="px-3 py-2 font-medium">Status</th>
              <th className="px-3 py-2 font-medium" />
            </tr>
          </thead>
          <tbody>
            {items.map((item) => {
              const ownedLocal = item.package_manager === "local"
              const busy = busyName === item.name || switchMutation.isPending
              const detailTo = `/brew/${encodeURIComponent(item.name)}${
                item.kind ? `?kind=${encodeURIComponent(item.kind)}` : ""
              }`
              return (
                <tr
                  key={`${item.kind || "formula"}:${item.name}`}
                  className="border-b last:border-0"
                >
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
            {items.length === 0 ? (
              <tr>
                <td
                  colSpan={5}
                  className="px-3 py-8 text-center text-muted-foreground"
                >
                  No formulae installed yet.
                </td>
              </tr>
            ) : null}
          </tbody>
        </table>
      </div>
    </ContentLoader>
  )
}
