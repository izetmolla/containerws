import { useEffect, useMemo, useState } from "react"
import { Link, useNavigate, useSearchParams } from "react-router"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import {
  AlertCircle,
  ArrowLeft,
  CheckCircle2,
  Eraser,
  ExternalLink,
  FileCode2,
  History,
  Loader2,
  Rocket,
  Save,
  Trash2,
  Unplug,
} from "lucide-react"
import { toast } from "sonner"

import ContentLoader from "@/components/content-loader"
import { MonacoCodeEditor } from "@/components/monaco-editor"
import { Badge } from "@/components/ui/badge"
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
import { ReactSelectCreatable } from "@/components/ui/reactselectcreatable"
import { asArray } from "@/lib/as-array"
import { getRequestErrorMessage, toastRequestError } from "@/lib/network"
import { cn } from "@/lib/utils"

import {
  applyApplicationManifests,
  applySavedApplication,
  createApplication,
  createNamespace,
  deleteApplication,
  formatAge,
  getApplication,
  getApplicationResources,
  getApplicationRevision,
  getStoredNamespace,
  K8S_APPLICATIONS_KEY,
  K8S_NAMESPACES_KEY,
  listApplicationRevisions,
  listNamespaces,
  removeApplicationFromCluster,
  restoreApplicationRevision,
  type LiveAppResource,
  type ManifestApplySummary,
  type ManifestResult,
  updateApplication,
  validateApplicationManifests,
} from "../../_shared/api"
import { ClusterBanner } from "../../_shared/cluster-banner"
import { k8sResourceHref } from "../../_shared/resource-href"

const SAMPLE = `apiVersion: v1
kind: ConfigMap
metadata:
  name: demo-app
  namespace: default
data:
  greeting: hello
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: demo-app
  namespace: default
spec:
  replicas: 1
  selector:
    matchLabels:
      app: demo-app
  template:
    metadata:
      labels:
        app: demo-app
    spec:
      containers:
        - name: demo
          image: nginx:alpine
          ports:
            - containerPort: 80
---
apiVersion: v1
kind: Service
metadata:
  name: demo-app
  namespace: default
spec:
  selector:
    app: demo-app
  ports:
    - port: 80
      targetPort: 80
`

type NsOption = { label: string; value: string }

function ResourceTitle({
  kind,
  name,
  namespace,
}: {
  kind?: string
  name?: string
  namespace?: string
}) {
  const href = k8sResourceHref({ kind, name, namespace })
  const nsHref = namespace
    ? `/kubernetes/namespaces/${encodeURIComponent(namespace)}`
    : null
  return (
    <div className="min-w-0 space-y-0.5">
      <p className="font-medium">
        {href ? (
          <Link className="hover:underline" to={href}>
            {kind || "Unknown"}
            {name ? (
              <span className="text-muted-foreground"> / {name}</span>
            ) : null}
            <ExternalLink className="ms-1 inline size-3.5 align-[-2px] text-muted-foreground" />
          </Link>
        ) : (
          <>
            {kind || "Unknown"}
            {name ? (
              <span className="text-muted-foreground"> / {name}</span>
            ) : null}
          </>
        )}
      </p>
      {namespace ? (
        <p className="font-mono text-[11px] text-muted-foreground">
          ns:{" "}
          {nsHref ? (
            <Link className="hover:underline" to={nsHref}>
              {namespace}
            </Link>
          ) : (
            namespace
          )}
        </p>
      ) : null}
    </div>
  )
}

function ResultRow({ row }: { row: ManifestResult }) {
  const failed = Boolean(row.error)
  return (
    <div
      className={cn(
        "flex flex-col gap-1 rounded-lg border px-3 py-2 text-sm sm:flex-row sm:items-start sm:justify-between",
        failed
          ? "border-destructive/40 bg-destructive/5"
          : "border-border bg-muted/20",
      )}
    >
      <div className="min-w-0 space-y-0.5">
        <div className="flex flex-wrap items-center gap-2">
          <span className="font-mono text-xs text-muted-foreground">
            #{row.index}
          </span>
          <ResourceTitle
            kind={row.kind}
            name={row.name}
            namespace={row.namespace}
          />
        </div>
        {row.error ? (
          <p className="text-xs text-destructive">{row.error}</p>
        ) : null}
      </div>
      <div className="shrink-0">
        {failed ? (
          <Badge variant="destructive">failed</Badge>
        ) : (
          <Badge variant="default">{row.action || "ok"}</Badge>
        )}
      </div>
    </div>
  )
}

function LiveResourceRow({ row }: { row: LiveAppResource }) {
  return (
    <div className="flex flex-wrap items-center justify-between gap-2 rounded-lg border px-3 py-2 text-sm">
      <div className="min-w-0">
        <ResourceTitle
          kind={row.kind}
          name={row.name}
          namespace={row.namespace}
        />
        <p className="font-mono text-[11px] text-muted-foreground">
          {row.apiVersion}
        </p>
      </div>
      <div className="flex items-center gap-2">
        {row.ready ? (
          <Badge variant="outline" className="font-mono text-[10px]">
            {row.ready}
          </Badge>
        ) : null}
        <Badge variant={row.exists ? "default" : "secondary"}>
          {row.status || (row.exists ? "Present" : "Missing")}
        </Badge>
      </div>
    </div>
  )
}

export default function ApplicationEditPage() {
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const [params] = useSearchParams()
  const editId = params.get("id")?.trim() || ""
  const isEdit = Boolean(editId)

  const [name, setName] = useState("")
  const [yaml, setYaml] = useState(SAMPLE)
  const [namespace, setNamespace] = useState<string | null>(
    () => getStoredNamespace() || "default",
  )
  const [yamlNsHint, setYamlNsHint] = useState<string | null>(null)
  const [nsError, setNsError] = useState<string | null>(null)
  const [summary, setSummary] = useState<ManifestApplySummary | null>(null)
  const [removeOpen, setRemoveOpen] = useState(false)
  const [uninstallOpen, setUninstallOpen] = useState(false)
  const [hydratedId, setHydratedId] = useState<string | null>(null)

  const detailQuery = useQuery({
    queryKey: [K8S_APPLICATIONS_KEY, editId],
    queryFn: () => getApplication(editId),
    enabled: isEdit,
  })

  const resourcesQuery = useQuery({
    queryKey: [K8S_APPLICATIONS_KEY, editId, "resources"],
    queryFn: () => getApplicationResources(editId),
    enabled: isEdit,
    refetchInterval: 10_000,
  })

  const revisionsQuery = useQuery({
    queryKey: [K8S_APPLICATIONS_KEY, editId, "revisions"],
    queryFn: () => listApplicationRevisions(editId),
    enabled: isEdit,
  })

  const nsQuery = useQuery({
    queryKey: [K8S_NAMESPACES_KEY],
    queryFn: listNamespaces,
    staleTime: 60_000,
  })
  const nsOptions = useMemo<NsOption[]>(() => {
    const fromCluster = asArray(nsQuery.data?.data).map((n) => ({
      label: n.name,
      value: n.name,
    }))
    if (
      namespace &&
      !fromCluster.some((o) => o.value === namespace)
    ) {
      return [{ label: namespace, value: namespace }, ...fromCluster]
    }
    return fromCluster
  }, [nsQuery.data, namespace])

  const createNsMutation = useMutation({
    mutationFn: (ns: string) => createNamespace(ns),
    onSuccess: (res, ns) => {
      toast.success(res.message || `Namespace “${ns}” created`)
      setNamespace(ns)
      void queryClient.invalidateQueries({ queryKey: [K8S_NAMESPACES_KEY] })
    },
    onError: (err) => toastRequestError(err, "Create namespace failed"),
  })

  useEffect(() => {
    if (!isEdit) {
      if (hydratedId !== null) {
        setHydratedId(null)
        setName("")
        setYaml(SAMPLE)
        setNamespace(getStoredNamespace() || "default")
        setSummary(null)
        setNsError(null)
      }
      return
    }
    const app = detailQuery.data?.data
    if (!app || app.id !== editId || hydratedId === editId) return
    setHydratedId(editId)
    setName(app.name)
    setYaml(app.yaml)
    setNamespace(app.namespace || null)
    setYamlNsHint(app.namespace || null)
    setNsError(null)
    setSummary(null)
  }, [detailQuery.data, editId, hydratedId, isEdit])

  useEffect(() => {
    if (!yaml.trim()) {
      setYamlNsHint(null)
      setNsError(null)
      return
    }
    const t = window.setTimeout(() => {
      void validateApplicationManifests({
        yaml,
        namespace: namespace || undefined,
      })
        .then((res) => {
          setYamlNsHint(res.data.namespace || null)
          setNsError(null)
        })
        .catch((err: unknown) => {
          setNsError(getRequestErrorMessage(err, "Invalid manifests"))
          setYamlNsHint(null)
        })
    }, 450)
    return () => window.clearTimeout(t)
  }, [yaml, namespace])

  const invalidate = () => {
    void queryClient.invalidateQueries({
      queryKey: [K8S_APPLICATIONS_KEY, editId, "revisions"],
    })
    void queryClient.invalidateQueries({ queryKey: [K8S_APPLICATIONS_KEY] })
  }

  const saveMutation = useMutation({
    mutationFn: async () => {
      const body = {
        name: name.trim(),
        namespace: namespace || undefined,
        yaml,
      }
      if (isEdit) return updateApplication(editId, body)
      return createApplication(body)
    },
    onSuccess: (res) => {
      toast.success(res.message || "Saved")
      setYaml(res.data.yaml)
      setNamespace(res.data.namespace || null)
      setSummary(null)
      invalidate()
      if (!isEdit) {
        navigate(
          `/kubernetes/applications/edit?id=${encodeURIComponent(res.data.id)}`,
          { replace: true },
        )
      }
    },
    onError: (err) => toastRequestError(err, "Save failed"),
  })

  const applyMutation = useMutation({
    mutationFn: async (dryRun: boolean) => {
      if (isEdit) {
        return applySavedApplication(editId, {
          yaml,
          namespace: namespace || undefined,
          dry_run: dryRun,
        })
      }
      return applyApplicationManifests({
        yaml,
        namespace: namespace || undefined,
        dry_run: dryRun,
        name: name.trim() || undefined,
      })
    },
    onSuccess: (res, dryRun) => {
      const s = res.data.summary
      setSummary(s)
      if (res.data.yaml) setYaml(res.data.yaml)
      if (res.data.namespace) setNamespace(res.data.namespace)
      if (s.failed > 0 && s.applied === 0) {
        toast.error(res.message || "Apply failed")
      } else if (s.failed > 0) {
        toast.warning(res.message || "Completed with errors")
      } else {
        toast.success(res.message || "Applied")
      }
      invalidate()
      const appId = res.data.application?.id || editId
      if (!dryRun && res.data.application?.id && !isEdit) {
        navigate(
          `/kubernetes/applications/edit?id=${encodeURIComponent(res.data.application.id)}`,
          { replace: true },
        )
      }
      if (appId) {
        void queryClient.invalidateQueries({
          queryKey: [K8S_APPLICATIONS_KEY, appId, "resources"],
        })
      }
    },
    onError: (err) => toastRequestError(err, "Apply failed"),
  })

  const deleteMutation = useMutation({
    mutationFn: () => deleteApplication(editId),
    onSuccess: (res) => {
      toast.success(res.message || "Deleted")
      invalidate()
      navigate("/kubernetes/applications")
    },
    onError: (err) => toastRequestError(err, "Delete failed"),
  })

  const uninstallMutation = useMutation({
    mutationFn: () => removeApplicationFromCluster(editId),
    onSuccess: (res) => {
      const s = res.data.summary
      setSummary(s)
      setUninstallOpen(false)
      if (s.failed > 0 && s.applied === 0) {
        toast.error(res.message || "Remove failed")
      } else if (s.failed > 0) {
        toast.warning(res.message || "Completed with errors")
      } else {
        toast.success(res.message || "Removed from cluster")
      }
      invalidate()
      void queryClient.invalidateQueries({
        queryKey: [K8S_APPLICATIONS_KEY, editId, "resources"],
      })
    },
    onError: (err) => toastRequestError(err, "Remove failed"),
  })

  const loadRevisionMutation = useMutation({
    mutationFn: (version: number) => getApplicationRevision(editId, version),
    onSuccess: (res) => {
      setYaml(res.data.yaml)
      setName(res.data.name)
      setNamespace(res.data.namespace || null)
      setSummary(null)
      toast.success(`Loaded v${res.data.version} into editor (not saved)`)
    },
    onError: (err) => toastRequestError(err, "Load revision failed"),
  })

  const restoreRevisionMutation = useMutation({
    mutationFn: (version: number) =>
      restoreApplicationRevision(editId, version),
    onSuccess: (res) => {
      setYaml(res.data.yaml)
      setName(res.data.name)
      setNamespace(res.data.namespace || null)
      setSummary(null)
      toast.success(res.message || "Revision restored")
      invalidate()
      void queryClient.invalidateQueries({
        queryKey: [K8S_APPLICATIONS_KEY, editId, "resources"],
      })
    },
    onError: (err) => toastRequestError(err, "Restore failed"),
  })

  const busy =
    saveMutation.isPending ||
    applyMutation.isPending ||
    uninstallMutation.isPending ||
    loadRevisionMutation.isPending ||
    restoreRevisionMutation.isPending
  const liveResources = asArray(resourcesQuery.data?.data.resources)
  const revisions = asArray(revisionsQuery.data?.data.revisions)
  const currentVersion = detailQuery.data?.data.version || 1
  const title = isEdit ? name || "Edit application" : "Create application"

  return (
    <ContentLoader
      title={title}
      breadcrumb={[
        { label: "Kubernetes", to: "/kubernetes" },
        { label: "Applications", to: "/kubernetes/applications" },
        { label: isEdit ? "Edit" : "Create" },
      ]}
      isLoading={isEdit && detailQuery.isLoading}
      error={isEdit ? detailQuery.error : undefined}
      rightComponent={
        <Button size="sm" variant="outline" asChild>
          <Link to="/kubernetes/applications">
            <ArrowLeft className="size-3.5" />
            Back to list
          </Link>
        </Button>
      }
    >
      <div className="space-y-4">
        <ClusterBanner />

        <div className="rounded-xl border bg-muted/10 px-4 py-3 text-sm text-muted-foreground">
          Only name, YAML, and namespace are stored in SQLite
          {isEdit ? (
            <>
              {" "}
              (current YAML{" "}
              <Badge variant="secondary" className="align-middle font-mono">
                v{currentVersion}
              </Badge>
              ).
            </>
          ) : (
            ". "
          )}{" "}
          Each save/apply that changes the manifest creates a version snapshot.
          Click live resources below to open them in the UI.
        </div>

        <div className="grid gap-3 sm:grid-cols-[1fr_240px]">
          <div className="space-y-1.5">
            <Label>Application name</Label>
            <Input
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="my-app"
            />
          </div>
          <div className="space-y-1.5">
            <Label>
              Namespace{" "}
              <span className="font-normal text-muted-foreground">
                (optional override)
              </span>
            </Label>
            <ReactSelectCreatable<NsOption, false>
              size="sm"
              options={nsOptions}
              value={namespace}
              isSearchable
              isClearable
              isLoading={nsQuery.isLoading || createNsMutation.isPending}
              placeholder="Select or create namespace"
              noOptionsMessage={() => "Type a namespace name to create"}
              formatCreateLabel={(input) => `Create namespace “${input}”`}
              isValidNewOption={(input) => {
                const v = input.trim()
                if (!v) return false
                if (!/^[a-z0-9]([-a-z0-9]*[a-z0-9])?$/.test(v)) return false
                return !nsOptions.some((o) => o.value === v)
              }}
              onCreateOption={(input) => {
                const ns = input.trim()
                if (!ns) return
                createNsMutation.mutate(ns)
              }}
              onValueChange={(v) => setNamespace(v || null)}
            />
            <p className="text-[11px] text-muted-foreground">
              {createNsMutation.isPending
                ? "Creating namespace…"
                : namespace
                  ? `Will rewrite namespaced objects to “${namespace}” on save/apply.`
                  : yamlNsHint
                    ? `Detected in YAML: ${yamlNsHint}`
                    : "Clear to keep namespaces from the YAML (must be uniform). Type a new name to create a namespace."}
            </p>
          </div>
        </div>

        {nsError ? (
          <div className="flex items-start gap-2 rounded-lg border border-destructive/40 bg-destructive/5 px-3 py-2 text-sm text-destructive">
            <AlertCircle className="mt-0.5 size-4 shrink-0" />
            <span>{nsError}</span>
          </div>
        ) : null}

        <div className="flex flex-wrap gap-2">
          <Button
            type="button"
            size="sm"
            variant="outline"
            disabled={busy}
            onClick={() => setYaml(SAMPLE)}
          >
            <FileCode2 className="size-3.5" />
            Sample
          </Button>
          <Button
            type="button"
            size="sm"
            variant="outline"
            disabled={busy || !yaml.trim()}
            onClick={() => {
              setYaml("")
              setSummary(null)
              setNsError(null)
            }}
          >
            <Eraser className="size-3.5" />
            Clear
          </Button>
          <div className="ms-auto flex flex-wrap gap-2">
            {isEdit ? (
              <>
                <Button
                  type="button"
                  size="sm"
                  variant="outline"
                  disabled={busy}
                  onClick={() => setUninstallOpen(true)}
                >
                  <Unplug className="size-3.5" />
                  Remove from cluster
                </Button>
                <Button
                  type="button"
                  size="sm"
                  variant="destructive"
                  disabled={busy}
                  onClick={() => setRemoveOpen(true)}
                >
                  <Trash2 className="size-3.5" />
                  Delete from store
                </Button>
              </>
            ) : null}
            <Button
              type="button"
              size="sm"
              variant="outline"
              disabled={busy || !yaml.trim() || !name.trim() || !!nsError}
              onClick={() => saveMutation.mutate()}
            >
              {saveMutation.isPending ? (
                <Loader2 className="size-3.5 animate-spin" />
              ) : (
                <Save className="size-3.5" />
              )}
              Save
            </Button>
            <Button
              type="button"
              size="sm"
              variant="outline"
              disabled={busy || !yaml.trim() || !!nsError}
              onClick={() => applyMutation.mutate(true)}
            >
              {busy && applyMutation.variables === true ? (
                <Loader2 className="size-3.5 animate-spin" />
              ) : (
                <CheckCircle2 className="size-3.5" />
              )}
              Dry run
            </Button>
            <Button
              type="button"
              size="sm"
              disabled={busy || !yaml.trim() || !!nsError}
              onClick={() => applyMutation.mutate(false)}
            >
              {busy && applyMutation.variables === false ? (
                <Loader2 className="size-3.5 animate-spin" />
              ) : (
                <Rocket className="size-3.5" />
              )}
              Apply
            </Button>
          </div>
        </div>

        <div
          className={cn(
            "overflow-hidden rounded-xl border border-input",
            "focus-within:border-ring focus-within:ring-[3px] focus-within:ring-ring/50",
            nsError && "border-destructive/50",
          )}
          style={{ height: "min(50vh, 26rem)" }}
        >
          <MonacoCodeEditor
            value={yaml}
            onChange={setYaml}
            language="yaml"
            height="100%"
          />
        </div>

        {isEdit ? (
          <div className="space-y-2">
            <div className="flex items-center justify-between gap-2">
              <h2 className="text-sm font-medium">Live resources</h2>
              <Button
                size="sm"
                variant="ghost"
                onClick={() => void resourcesQuery.refetch()}
              >
                Refresh
              </Button>
            </div>
            {liveResources.length === 0 ? (
              <p className="text-xs text-muted-foreground">
                Apply the application to create resources in the cluster.
              </p>
            ) : (
              <div className="space-y-2">
                {liveResources.map((row) => (
                  <LiveResourceRow
                    key={`${row.kind}/${row.namespace || ""}/${row.name}`}
                    row={row}
                  />
                ))}
              </div>
            )}
          </div>
        ) : null}

        {isEdit ? (
          <div className="space-y-2">
            <div className="flex items-center justify-between gap-2">
              <h2 className="flex items-center gap-2 text-sm font-medium">
                <History className="size-3.5" />
                YAML versions
              </h2>
              <Button
                size="sm"
                variant="ghost"
                onClick={() => void revisionsQuery.refetch()}
              >
                Refresh
              </Button>
            </div>
            {revisions.length === 0 ? (
              <p className="text-xs text-muted-foreground">
                Save or apply to create the first version snapshot.
              </p>
            ) : (
              <div className="space-y-2">
                {revisions.map((rev) => (
                  <div
                    key={rev.id}
                    className="flex flex-wrap items-center justify-between gap-2 rounded-lg border px-3 py-2 text-sm"
                  >
                    <div className="min-w-0 space-y-0.5">
                      <div className="flex flex-wrap items-center gap-2">
                        <Badge
                          variant={rev.current ? "default" : "outline"}
                          className="font-mono"
                        >
                          v{rev.version}
                        </Badge>
                        <Badge variant="secondary" className="capitalize">
                          {rev.source}
                        </Badge>
                        {rev.current ? (
                          <span className="text-xs text-muted-foreground">
                            current
                          </span>
                        ) : null}
                      </div>
                      <p className="text-xs text-muted-foreground">
                        {formatAge(rev.created_at)}
                        {rev.namespace ? ` · ${rev.namespace}` : ""}
                        {rev.note ? ` · ${rev.note}` : ""}
                      </p>
                    </div>
                    <div className="flex flex-wrap gap-2">
                      <Button
                        size="sm"
                        variant="outline"
                        disabled={busy}
                        onClick={() => loadRevisionMutation.mutate(rev.version)}
                      >
                        Load
                      </Button>
                      <Button
                        size="sm"
                        variant="outline"
                        disabled={busy || rev.current}
                        onClick={() =>
                          restoreRevisionMutation.mutate(rev.version)
                        }
                      >
                        Restore
                      </Button>
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>
        ) : null}

        {summary ? (
          <div className="space-y-3">
            <div className="flex flex-wrap items-center gap-2">
              <h2 className="text-sm font-medium">Apply results</h2>
              {summary.dry_run ? (
                <Badge variant="secondary">dry-run</Badge>
              ) : null}
              <Badge variant="outline">
                {summary.applied}/{summary.total} ok
              </Badge>
              {summary.failed > 0 ? (
                <Badge variant="destructive">{summary.failed} failed</Badge>
              ) : null}
            </div>
            <div className="space-y-2">
              {summary.results.map((row) => (
                <ResultRow
                  key={`${row.index}-${row.kind}-${row.name}`}
                  row={row}
                />
              ))}
            </div>
          </div>
        ) : null}
      </div>

      <Dialog open={removeOpen} onOpenChange={setRemoveOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Delete from store?</DialogTitle>
            <DialogDescription>
              Removes “{name}” from SQLite. Cluster resources are not deleted
              automatically.
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setRemoveOpen(false)}>
              Cancel
            </Button>
            <Button
              variant="destructive"
              disabled={deleteMutation.isPending}
              onClick={() => deleteMutation.mutate()}
            >
              Delete from store
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={uninstallOpen} onOpenChange={setUninstallOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Remove from cluster?</DialogTitle>
            <DialogDescription>
              Deletes Kubernetes resources for “{name}”. The saved application
              stays in SQLite.
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setUninstallOpen(false)}>
              Cancel
            </Button>
            <Button
              variant="destructive"
              disabled={uninstallMutation.isPending}
              onClick={() => uninstallMutation.mutate()}
            >
              Remove from cluster
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </ContentLoader>
  )
}
