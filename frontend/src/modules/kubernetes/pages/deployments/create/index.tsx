import { useMemo, useState } from "react"
import { Link, useNavigate } from "react-router"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { ArrowLeft, Loader2, Plus, Trash2 } from "lucide-react"
import { toast } from "sonner"

import ContentLoader from "@/components/content-loader"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { ReactSelectCreatable } from "@/components/ui/reactselectcreatable"
import { Textarea } from "@/components/ui/textarea"
import { asArray } from "@/lib/as-array"
import { toastRequestError } from "@/lib/network"

import {
  createDeployment,
  createNamespace,
  getStoredNamespace,
  K8S_NAMESPACES_KEY,
  listNamespaces,
} from "../../_shared/api"
import { ClusterBanner } from "../../_shared/cluster-banner"

type Option = { label: string; value: string }
type LabelRow = { id: number; key: string; value: string }

function splitTokens(raw: string): string[] {
  return raw
    .split(/\s+/)
    .map((s) => s.trim())
    .filter(Boolean)
}

export default function CreateDeploymentPage() {
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const [name, setName] = useState("")
  const [namespace, setNamespace] = useState(
    () => getStoredNamespace() || "default",
  )
  const [image, setImage] = useState("nginx:alpine")
  const [replicas, setReplicas] = useState("1")
  const [port, setPort] = useState("80")
  const [commandText, setCommandText] = useState("")
  const [argsText, setArgsText] = useState("")
  const [labels, setLabels] = useState<LabelRow[]>([
    { id: 1, key: "app", value: "" },
  ])

  const nsQuery = useQuery({
    queryKey: [K8S_NAMESPACES_KEY],
    queryFn: listNamespaces,
    staleTime: 60_000,
  })
  const nsOptions = useMemo<Option[]>(() => {
    const fromCluster = asArray(nsQuery.data?.data).map((n) => ({
      label: n.name,
      value: n.name,
    }))
    if (namespace && !fromCluster.some((o) => o.value === namespace)) {
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

  const createMutation = useMutation({
    mutationFn: () => {
      const labelMap: Record<string, string> = {}
      for (const row of labels) {
        const key = row.key.trim()
        if (!key) continue
        labelMap[key] = row.value.trim() || name.trim()
      }
      const command = splitTokens(commandText)
      const args = argsText.trim() ? [argsText.trim()] : undefined
      const replicaCount = Number.parseInt(replicas, 10)
      const portNum = Number.parseInt(port, 10)
      return createDeployment({
        name: name.trim(),
        namespace: namespace.trim() || "default",
        image: image.trim(),
        replicas: Number.isFinite(replicaCount) ? replicaCount : 1,
        port: Number.isFinite(portNum) && portNum > 0 ? portNum : undefined,
        labels: labelMap,
        command: command.length ? command : undefined,
        args,
      })
    },
    onSuccess: (res) => {
      toast.success(res.message || "Deployment created")
      const ns = namespace.trim() || "default"
      navigate(
        `/kubernetes/deployments/${encodeURIComponent(ns)}/${encodeURIComponent(name.trim())}`,
      )
    },
    onError: (err) => toastRequestError(err, "Create failed"),
  })

  const canSubmit =
    Boolean(name.trim()) &&
    Boolean(namespace.trim()) &&
    Boolean(image.trim()) &&
    !createMutation.isPending

  return (
    <ContentLoader
      title="Create Deployment"
      breadcrumb={[
        { label: "Kubernetes", to: "/kubernetes" },
        { label: "Deployments", to: "/kubernetes/deployments" },
        { label: "Create" },
      ]}
      rightComponent={
        <Button size="sm" variant="outline" asChild>
          <Link to="/kubernetes/deployments">
            <ArrowLeft className="size-3.5" />
            Back to list
          </Link>
        </Button>
      }
    >
      <div className="w-full space-y-6">
        <ClusterBanner />

        <p className="text-sm text-muted-foreground">
          Creates a Deployment with a single container. Replicas, image, optional
          container port, and labels are applied to the selector and pod template.
        </p>

        <div className="grid gap-4 sm:grid-cols-2">
          <div className="space-y-1.5 sm:col-span-2">
            <Label htmlFor="ds-name">Name</Label>
            <Input
              id="ds-name"
              value={name}
              onChange={(e) => {
                const next = e.target.value
                setName(next)
                setLabels((prev) =>
                  prev.map((row) =>
                    row.key === "app" && (!row.value || row.value === name)
                      ? { ...row, value: next }
                      : row,
                  ),
                )
              }}
              placeholder="web"
              autoComplete="off"
            />
          </div>
          <div className="space-y-1.5 sm:col-span-2">
            <Label>Namespace</Label>
            <ReactSelectCreatable<Option, false>
              size="sm"
              options={nsOptions}
              value={namespace}
              isSearchable
              isClearable={false}
              isLoading={nsQuery.isLoading || createNsMutation.isPending}
              placeholder="Select or create namespace"
              formatCreateLabel={(input) => `Create namespace “${input}”`}
              isValidNewOption={(input) => {
                const v = input.trim()
                if (!v) return false
                if (!/^[a-z0-9]([-a-z0-9]*[a-z0-9])?$/.test(v)) return false
                return !nsOptions.some((o) => o.value === v)
              }}
              onCreateOption={(input) => {
                const ns = input.trim()
                if (ns) createNsMutation.mutate(ns)
              }}
              onValueChange={(v) => {
                if (v) setNamespace(v)
              }}
            />
          </div>
          <div className="space-y-1.5 sm:col-span-2">
            <Label htmlFor="ds-image">Image</Label>
            <Input
              id="ds-image"
              value={image}
              onChange={(e) => setImage(e.target.value)}
              placeholder="nginx:alpine"
              className="font-mono text-sm"
            />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="ds-replicas">Replicas</Label>
            <Input
              id="ds-replicas"
              type="number"
              min={0}
              value={replicas}
              onChange={(e) => setReplicas(e.target.value)}
            />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="ds-port">Container port (optional)</Label>
            <Input
              id="ds-port"
              type="number"
              min={1}
              max={65535}
              value={port}
              onChange={(e) => setPort(e.target.value)}
              placeholder="80"
            />
          </div>
          <div className="space-y-1.5 sm:col-span-2">
            <Label htmlFor="ds-command">Command (optional)</Label>
            <Input
              id="ds-command"
              value={commandText}
              onChange={(e) => setCommandText(e.target.value)}
              placeholder="/bin/sh -c"
              className="font-mono text-sm"
            />
          </div>
          <div className="space-y-1.5 sm:col-span-2">
            <Label htmlFor="ds-args">Args (optional)</Label>
            <Textarea
              id="ds-args"
              value={argsText}
              onChange={(e) => setArgsText(e.target.value)}
              placeholder="sleep infinity"
              className="min-h-16 font-mono text-xs"
            />
          </div>
        </div>

        <div className="space-y-2">
          <div className="flex items-center justify-between gap-2">
            <div>
              <Label>Selector labels</Label>
              <p className="text-xs text-muted-foreground">
                Defaults to <code className="font-mono">app=&lt;name&gt;</code>{" "}
                when empty
              </p>
            </div>
            <Button
              type="button"
              size="sm"
              variant="outline"
              onClick={() =>
                setLabels((prev) => [
                  ...prev,
                  { id: (prev.at(-1)?.id ?? 0) + 1, key: "", value: "" },
                ])
              }
            >
              <Plus className="size-3.5" />
              Add label
            </Button>
          </div>
          <div className="space-y-2">
            {labels.map((row) => (
              <div key={row.id} className="flex gap-2">
                <Input
                  className="font-mono text-xs"
                  placeholder="key"
                  value={row.key}
                  onChange={(e) =>
                    setLabels((prev) =>
                      prev.map((r) =>
                        r.id === row.id ? { ...r, key: e.target.value } : r,
                      ),
                    )
                  }
                />
                <Input
                  className="font-mono text-xs"
                  placeholder="value"
                  value={row.value}
                  onChange={(e) =>
                    setLabels((prev) =>
                      prev.map((r) =>
                        r.id === row.id ? { ...r, value: e.target.value } : r,
                      ),
                    )
                  }
                />
                <Button
                  type="button"
                  size="icon-sm"
                  variant="ghost"
                  disabled={labels.length <= 1}
                  onClick={() =>
                    setLabels((prev) => prev.filter((r) => r.id !== row.id))
                  }
                >
                  <Trash2 className="size-3.5" />
                </Button>
              </div>
            ))}
          </div>
        </div>

        <div className="flex flex-wrap justify-end gap-2">
          <Button variant="outline" asChild>
            <Link to="/kubernetes/deployments">Cancel</Link>
          </Button>
          <Button
            disabled={!canSubmit}
            onClick={() => createMutation.mutate()}
          >
            {createMutation.isPending ? (
              <Loader2 className="size-3.5 animate-spin" />
            ) : (
              <Plus className="size-3.5" />
            )}
            Create Deployment
          </Button>
        </div>
      </div>
    </ContentLoader>
  )
}
