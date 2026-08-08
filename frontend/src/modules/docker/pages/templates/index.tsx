import { useMemo, useState } from "react"
import { useNavigate } from "react-router"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { Loader2, Rocket, Search } from "lucide-react"
import { toast } from "sonner"

import ContentLoader from "@/components/content-loader"
import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { ReactSelect } from "@/components/ui/reactselect"
import { asArray } from "@/lib/as-array"
import { toastRequestError } from "@/lib/network"

import { EngineBanner } from "../_shared/engine-status"
import { useEngineDown } from "../_shared/use-engine-status"
import { EnvironmentSelector } from "../_shared/environment-selector"
import {
  DOCKER_PAGE_DESCRIPTIONS,
  DockerRefreshButton,
  SummaryChip,
} from "../_shared/page-chrome"
import { DOCKER_STACKS_KEY } from "../stacks/api"
import {
  deployTemplate,
  DOCKER_TEMPLATES_KEY,
  listTemplates,
  type TemplateEnv,
  type TemplateRow,
} from "./api"

type Option = { value: string; label: string }

const TYPE_OPTIONS: Option[] = [
  { value: "all", label: "All types" },
  { value: "1", label: "Container" },
  { value: "3", label: "Compose stack" },
  { value: "2", label: "Swarm stack" },
]

function typeBadge(type: number) {
  switch (type) {
    case 2:
      return "Swarm"
    case 3:
      return "Compose"
    default:
      return "Container"
  }
}

function defaultEnvValues(envs?: TemplateEnv[]) {
  const out: Record<string, string> = {}
  for (const e of envs ?? []) {
    if (e.default) out[e.name] = e.default
    for (const opt of e.select ?? []) {
      if (opt.default) out[e.name] = opt.value
    }
    if (out[e.name] === undefined) out[e.name] = ""
  }
  return out
}

export default function DockerTemplatesPage() {
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const engineDown = useEngineDown()
  const [search, setSearch] = useState("")
  const [category, setCategory] = useState("all")
  const [typeFilter, setTypeFilter] = useState("all")
  const [deployTarget, setDeployTarget] = useState<TemplateRow | null>(null)
  const [deployName, setDeployName] = useState("")
  const [envValues, setEnvValues] = useState<Record<string, string>>({})

  const listQuery = useQuery({
    queryKey: [DOCKER_TEMPLATES_KEY, search, category, typeFilter],
    queryFn: () =>
      listTemplates({
        q: search.trim() || undefined,
        category: category !== "all" ? category : undefined,
        type: typeFilter !== "all" ? typeFilter : undefined,
      }),
    staleTime: 60_000,
  })

  const refreshMutation = useMutation({
    mutationFn: () =>
      listTemplates({
        q: search.trim() || undefined,
        category: category !== "all" ? category : undefined,
        type: typeFilter !== "all" ? typeFilter : undefined,
        refresh: true,
      }),
    onSuccess: (res) => {
      queryClient.setQueryData(
        [DOCKER_TEMPLATES_KEY, search, category, typeFilter],
        res
      )
      toast.success("Templates refreshed from Portainer registry")
    },
    onError: (err) => toastRequestError(err, "Refresh failed"),
  })

  const deployMutation = useMutation({
    mutationFn: () => {
      if (!deployTarget) throw new Error("No template selected")
      return deployTemplate({
        template_id: deployTarget.id,
        name: deployName.trim() || undefined,
        env: envValues,
      })
    },
    onSuccess: (res) => {
      toast.success(res.message || "Template deployed")
      setDeployTarget(null)
      void queryClient.invalidateQueries({ queryKey: [DOCKER_STACKS_KEY] })
      if (res.data?.kind === "stack" && res.data.stack_id) {
        navigate(
          `/docker/stacks/edit?id=${encodeURIComponent(res.data.stack_id)}`
        )
      } else if (res.data?.kind === "container" && res.data.container_id) {
        navigate(`/docker/containers/${res.data.container_id}`)
      }
    },
    onError: (err) => toastRequestError(err, "Deploy failed"),
  })

  const rows = asArray(listQuery.data?.data)
  const meta = listQuery.data?.meta
  const categoryOptions: Option[] = useMemo(() => {
    const cats = asArray(meta?.categories)
    return [
      { value: "all", label: "All categories" },
      ...cats
        .slice()
        .sort((a, b) => a.name.localeCompare(b.name))
        .map((c) => ({
          value: c.name,
          label: `${c.name} (${c.count})`,
        })),
    ]
  }, [meta?.categories])

  const openDeploy = (tpl: TemplateRow) => {
    setDeployTarget(tpl)
    setDeployName(tpl.name || tpl.title.toLowerCase().replace(/\s+/g, "-"))
    setEnvValues(defaultEnvValues(tpl.env))
  }

  return (
    <ContentLoader
      title="Templates"
      description={`${DOCKER_PAGE_DESCRIPTIONS.templates} Source: Portainer v3 registry.`}
      showHeaderSeparator
      breadcrumb={[
        { label: "Docker", to: "/docker" },
        { label: "Templates" },
      ]}
      isLoading={listQuery.isLoading}
      error={engineDown ? undefined : listQuery.error}
      rightComponent={
        <div className="flex flex-wrap items-center gap-2">
          <EnvironmentSelector />
          <DockerRefreshButton
            onClick={() => refreshMutation.mutate()}
            isFetching={refreshMutation.isPending}
            label="Refresh catalog"
          />
        </div>
      }
    >
      <div className="flex flex-col gap-4">
        <EngineBanner />

        <div className="flex flex-wrap items-center gap-2">
          <div className="relative w-full max-w-xs min-w-[12rem]">
            <Search className="pointer-events-none absolute top-1/2 left-2.5 size-3.5 -translate-y-1/2 text-muted-foreground" />
            <Input
              className="h-8 pl-8"
              placeholder="Search templates…"
              value={search}
              onChange={(e) => setSearch(e.target.value)}
            />
          </div>
          <div className="min-w-[11rem]">
            <ReactSelect<Option, false>
              size="sm"
              options={categoryOptions}
              value={category}
              onValueChange={(v) => v && setCategory(v)}
            />
          </div>
          <div className="min-w-[10rem]">
            <ReactSelect<Option, false>
              size="sm"
              options={TYPE_OPTIONS}
              value={typeFilter}
              onValueChange={(v) => v && setTypeFilter(v)}
            />
          </div>
          {meta?.total != null ? (
            <SummaryChip>{meta.total} templates</SummaryChip>
          ) : null}
          {meta?.cached_at ? (
            <span className="text-xs text-muted-foreground">
              Cached {new Date(meta.cached_at).toLocaleString()}
            </span>
          ) : null}
        </div>

        {rows.length === 0 ? (
          <div className="rounded-xl border border-dashed px-6 py-16 text-center text-sm text-muted-foreground">
            No templates match your filters. Refresh the catalog or clear filters.
          </div>
        ) : (
          <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
            {rows.map((tpl) => (
              <article
                key={tpl.id}
                className="flex flex-col gap-3 rounded-xl border border-border/70 bg-card/40 p-4 shadow-sm"
              >
                <div className="flex items-start gap-3">
                  {tpl.logo ? (
                    <img
                      src={tpl.logo}
                      alt=""
                      className="size-10 rounded-lg object-contain bg-muted/40 p-1"
                    />
                  ) : (
                    <div className="grid size-10 place-items-center rounded-lg bg-muted text-xs font-medium text-muted-foreground">
                      App
                    </div>
                  )}
                  <div className="min-w-0 flex-1">
                    <div className="flex flex-wrap items-center gap-2">
                      <h3 className="truncate font-medium tracking-tight">
                        {tpl.title}
                      </h3>
                      <span className="rounded-full bg-muted px-2 py-0.5 text-[10px] font-medium uppercase tracking-wide text-muted-foreground">
                        {typeBadge(tpl.type)}
                      </span>
                    </div>
                    <p className="mt-1 line-clamp-2 text-xs text-muted-foreground">
                      {tpl.description}
                    </p>
                  </div>
                </div>
                <div className="mt-auto flex flex-wrap items-center justify-between gap-2 pt-1">
                  <div className="flex flex-wrap gap-1">
                    {(tpl.categories ?? []).slice(0, 3).map((c) => (
                      <span
                        key={c}
                        className="rounded-md bg-muted/60 px-1.5 py-0.5 text-[10px] text-muted-foreground"
                      >
                        {c}
                      </span>
                    ))}
                  </div>
                  <Button size="sm" onClick={() => openDeploy(tpl)}>
                    <Rocket data-icon="inline-start" />
                    Deploy
                  </Button>
                </div>
              </article>
            ))}
          </div>
        )}
      </div>

      <Dialog
        open={Boolean(deployTarget)}
        onOpenChange={(v) => !v && setDeployTarget(null)}
      >
        <DialogContent className="sm:max-w-lg">
          <DialogHeader>
            <DialogTitle>Deploy {deployTarget?.title}</DialogTitle>
            <DialogDescription>
              {deployTarget?.type === 1
                ? "Creates and starts a container from this template."
                : "Downloads the Compose file and deploys it as a stack."}
            </DialogDescription>
          </DialogHeader>
          <div className="grid max-h-[55vh] gap-3 overflow-y-auto py-1">
            <div className="grid gap-1.5">
              <Label>Name</Label>
              <Input
                value={deployName}
                onChange={(e) => setDeployName(e.target.value)}
                placeholder="resource name"
              />
            </div>
            {(deployTarget?.env ?? [])
              .filter((e) => !e.preset)
              .map((e) => (
                <div key={e.name} className="grid gap-1.5">
                  <Label>{e.label || e.name}</Label>
                  {e.description ? (
                    <p className="text-[11px] text-muted-foreground">
                      {e.description}
                    </p>
                  ) : null}
                  {e.select?.length ? (
                    <ReactSelect<Option, false>
                      size="sm"
                      options={e.select.map((o) => ({
                        value: o.value,
                        label: o.text,
                      }))}
                      value={envValues[e.name] || ""}
                      onValueChange={(v) =>
                        v &&
                        setEnvValues((prev) => ({ ...prev, [e.name]: v }))
                      }
                    />
                  ) : (
                    <Input
                      value={envValues[e.name] ?? ""}
                      onChange={(ev) =>
                        setEnvValues((prev) => ({
                          ...prev,
                          [e.name]: ev.target.value,
                        }))
                      }
                      placeholder={e.default || e.name}
                    />
                  )}
                </div>
              ))}
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setDeployTarget(null)}>
              Cancel
            </Button>
            <Button
              disabled={deployMutation.isPending}
              onClick={() => deployMutation.mutate()}
            >
              {deployMutation.isPending ? (
                <Loader2 className="size-4 animate-spin" />
              ) : (
                <Rocket data-icon="inline-start" />
              )}
              Deploy
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </ContentLoader>
  )
}
