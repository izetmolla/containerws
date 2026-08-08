import { useMemo, useState } from "react"
import { Link, useNavigate } from "react-router"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { ArrowLeft, Loader2, Plus, Trash2 } from "lucide-react"
import { toast } from "sonner"

import ContentLoader from "@/components/content-loader"
import { Button } from "@/components/ui/button"
import { Checkbox } from "@/components/ui/checkbox"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { ReactSelectCreatable } from "@/components/ui/reactselectcreatable"
import { asArray } from "@/lib/as-array"
import { toastRequestError } from "@/lib/network"

import {
  createNamespace,
  createNetworkPolicy,
  getStoredNamespace,
  K8S_NAMESPACES_KEY,
  listNamespaces,
} from "../../_shared/api"
import { ClusterBanner } from "../../_shared/cluster-banner"

type Option = { label: string; value: string }
type SelectorRow = { id: number; key: string; value: string }

export default function CreateNetworkPolicyPage() {
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const [name, setName] = useState("")
  const [namespace, setNamespace] = useState(
    () => getStoredNamespace() || "default",
  )
  const [ingress, setIngress] = useState(true)
  const [egress, setEgress] = useState(false)
  const [allowSameNs, setAllowSameNs] = useState(false)
  const [selectors, setSelectors] = useState<SelectorRow[]>([
    { id: 1, key: "", value: "" },
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
      const pod_selector: Record<string, string> = {}
      for (const row of selectors) {
        const key = row.key.trim()
        if (!key) continue
        pod_selector[key] = row.value.trim()
      }
      const policy_types: string[] = []
      if (ingress) policy_types.push("Ingress")
      if (egress) policy_types.push("Egress")
      return createNetworkPolicy({
        name: name.trim(),
        namespace: namespace.trim() || "default",
        pod_selector,
        policy_types,
        allow_from_same_namespace: allowSameNs,
      })
    },
    onSuccess: (res) => {
      toast.success(res.message || "NetworkPolicy created")
      const ns = namespace.trim() || "default"
      navigate(
        `/kubernetes/network-policies/${encodeURIComponent(ns)}/${encodeURIComponent(name.trim())}`,
      )
    },
    onError: (err) => toastRequestError(err, "Create failed"),
  })

  const canSubmit =
    Boolean(name.trim()) &&
    Boolean(namespace.trim()) &&
    (ingress || egress) &&
    !createMutation.isPending

  return (
    <ContentLoader
      title="Create NetworkPolicy"
      breadcrumb={[
        { label: "Kubernetes", to: "/kubernetes" },
        { label: "NetworkPolicies", to: "/kubernetes/network-policies" },
        { label: "Create" },
      ]}
      rightComponent={
        <Button size="sm" variant="outline" asChild>
          <Link to="/kubernetes/network-policies">
            <ArrowLeft className="size-3.5" />
            Back to list
          </Link>
        </Button>
      }
    >
      <div className="w-full space-y-6">
        <ClusterBanner />

        <p className="text-sm text-muted-foreground">
          Creates a NetworkPolicy on the cluster. Empty pod selector matches all
          pods in the namespace. With Ingress selected and no allow rule, ingress
          is denied by default.
        </p>

        <div className="grid gap-4 sm:grid-cols-2">
          <div className="space-y-1.5 sm:col-span-2">
            <Label htmlFor="np-name">Name</Label>
            <Input
              id="np-name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="deny-all-ingress"
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
        </div>

        <div className="space-y-3">
          <Label>Policy types</Label>
          <div className="flex flex-wrap gap-4">
            <label className="flex items-center gap-2 text-sm">
              <Checkbox
                checked={ingress}
                onCheckedChange={(v) => setIngress(v === true)}
              />
              Ingress
            </label>
            <label className="flex items-center gap-2 text-sm">
              <Checkbox
                checked={egress}
                onCheckedChange={(v) => setEgress(v === true)}
              />
              Egress
            </label>
          </div>
          {ingress ? (
            <label className="flex items-start gap-2 text-sm">
              <Checkbox
                className="mt-0.5"
                checked={allowSameNs}
                onCheckedChange={(v) => setAllowSameNs(v === true)}
              />
              <span>
                Allow ingress from pods in the same namespace
                <span className="mt-0.5 block text-xs text-muted-foreground">
                  Off = deny all ingress for selected pods
                </span>
              </span>
            </label>
          ) : null}
        </div>

        <div className="space-y-2">
          <div className="flex items-center justify-between gap-2">
            <div>
              <Label>Pod selector</Label>
              <p className="text-xs text-muted-foreground">
                MatchLabels — leave empty to select all pods
              </p>
            </div>
            <Button
              type="button"
              size="sm"
              variant="outline"
              onClick={() =>
                setSelectors((prev) => [
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
            {selectors.map((row) => (
              <div key={row.id} className="flex gap-2">
                <Input
                  className="font-mono text-xs"
                  placeholder="key"
                  value={row.key}
                  onChange={(e) =>
                    setSelectors((prev) =>
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
                    setSelectors((prev) =>
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
                  disabled={selectors.length <= 1}
                  onClick={() =>
                    setSelectors((prev) => prev.filter((r) => r.id !== row.id))
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
            <Link to="/kubernetes/network-policies">Cancel</Link>
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
            Create NetworkPolicy
          </Button>
        </div>
      </div>
    </ContentLoader>
  )
}
