import { useEffect, useMemo, useState } from "react"
import { Link, useNavigate } from "react-router"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { ArrowLeft, Loader2, Plus } from "lucide-react"
import { toast } from "sonner"

import ContentLoader from "@/components/content-loader"
import { Button } from "@/components/ui/button"
import { Checkbox } from "@/components/ui/checkbox"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { ReactSelect } from "@/components/ui/reactselect"
import { ReactSelectCreatable } from "@/components/ui/reactselectcreatable"
import { asArray } from "@/lib/as-array"
import { toastRequestError } from "@/lib/network"

import {
  createNamespace,
  createPvc,
  getStoredNamespace,
  K8S_NAMESPACES_KEY,
  K8S_STORAGE_CLASSES_KEY,
  listNamespaces,
  listStorageClasses,
} from "../../_shared/api"
import { ClusterBanner } from "../../_shared/cluster-banner"

type Option = { label: string; value: string }

const ACCESS_MODES: Option[] = [
  { label: "ReadWriteOnce (RWO)", value: "ReadWriteOnce" },
  { label: "ReadOnlyMany (ROX)", value: "ReadOnlyMany" },
  { label: "ReadWriteMany (RWX)", value: "ReadWriteMany" },
  { label: "ReadWriteOncePod (RWOP)", value: "ReadWriteOncePod" },
]

const VOLUME_MODES: Option[] = [
  { label: "Filesystem", value: "Filesystem" },
  { label: "Block", value: "Block" },
]

export default function CreatePvcPage() {
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const [name, setName] = useState("")
  const [namespace, setNamespace] = useState(
    () => getStoredNamespace() || "default",
  )
  const [storage, setStorage] = useState("1Gi")
  const [accessModes, setAccessModes] = useState<string[]>(["ReadWriteOnce"])
  const [storageClass, setStorageClass] = useState<string | null>(null)
  const [volumeMode, setVolumeMode] = useState<"Filesystem" | "Block">(
    "Filesystem",
  )
  const [scTouched, setScTouched] = useState(false)

  const nsQuery = useQuery({
    queryKey: [K8S_NAMESPACES_KEY],
    queryFn: listNamespaces,
    staleTime: 60_000,
  })
  const scQuery = useQuery({
    queryKey: [K8S_STORAGE_CLASSES_KEY],
    queryFn: listStorageClasses,
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

  const scOptions = useMemo<Option[]>(() => {
    return asArray(scQuery.data?.data).map((sc) => ({
      label: sc.default ? `${sc.name} (default)` : sc.name,
      value: sc.name,
    }))
  }, [scQuery.data])

  useEffect(() => {
    if (scTouched || storageClass !== null) return
    const defaultSc = asArray(scQuery.data?.data).find((sc) => sc.default)?.name
    if (defaultSc) {
      setStorageClass(defaultSc)
      setScTouched(true)
    }
  }, [scQuery.data, scTouched, storageClass])

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
    mutationFn: () =>
      createPvc({
        name: name.trim(),
        namespace: namespace.trim() || "default",
        storage: storage.trim() || "1Gi",
        access_modes: accessModes,
        storage_class: storageClass || undefined,
        volume_mode: volumeMode,
      }),
    onSuccess: (res) => {
      toast.success(res.message || "PersistentVolumeClaim created")
      const ns = namespace.trim() || "default"
      navigate(
        `/kubernetes/persistentvolumeclaims/${encodeURIComponent(ns)}/${encodeURIComponent(name.trim())}`,
      )
    },
    onError: (err) => toastRequestError(err, "Create failed"),
  })

  const canSubmit =
    Boolean(name.trim()) &&
    Boolean(namespace.trim()) &&
    Boolean(storage.trim()) &&
    accessModes.length > 0 &&
    !createMutation.isPending

  return (
    <ContentLoader
      title="Create PersistentVolumeClaim"
      breadcrumb={[
        { label: "Kubernetes", to: "/kubernetes" },
        {
          label: "PersistentVolumeClaims",
          to: "/kubernetes/persistentvolumeclaims",
        },
        { label: "Create" },
      ]}
      rightComponent={
        <Button size="sm" variant="outline" asChild>
          <Link to="/kubernetes/persistentvolumeclaims">
            <ArrowLeft className="size-3.5" />
            Back to list
          </Link>
        </Button>
      }
    >
      <div className="w-full space-y-6">
        <ClusterBanner />

        <p className="text-sm text-muted-foreground">
          Creates a PersistentVolumeClaim on the cluster. Size uses Kubernetes
          quantities (e.g. <code className="font-mono text-xs">1Gi</code>,{" "}
          <code className="font-mono text-xs">500Mi</code>).
        </p>

        <div className="grid gap-4 sm:grid-cols-2">
          <div className="space-y-1.5 sm:col-span-2">
            <Label htmlFor="pvc-name">Name</Label>
            <Input
              id="pvc-name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="data-claim"
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
          <div className="space-y-1.5">
            <Label htmlFor="pvc-storage">Storage</Label>
            <Input
              id="pvc-storage"
              value={storage}
              onChange={(e) => setStorage(e.target.value)}
              placeholder="1Gi"
              className="font-mono text-sm"
            />
          </div>
          <div className="space-y-1.5">
            <Label>Volume mode</Label>
            <ReactSelect<Option, false>
              size="sm"
              options={VOLUME_MODES}
              value={volumeMode}
              isClearable={false}
              onValueChange={(v) => {
                if (v === "Filesystem" || v === "Block") setVolumeMode(v)
              }}
            />
          </div>
          <div className="space-y-1.5 sm:col-span-2">
            <Label>Storage class</Label>
            <ReactSelectCreatable<Option, false>
              size="sm"
              options={scOptions}
              value={storageClass}
              isSearchable
              isClearable
              isLoading={scQuery.isLoading}
              placeholder="Cluster default / type a name"
              formatCreateLabel={(input) => `Use “${input}”`}
              noOptionsMessage={() => "No StorageClasses — type a name"}
              onValueChange={(v) => {
                setScTouched(true)
                setStorageClass(v || null)
              }}
            />
          </div>
        </div>

        <div className="space-y-2">
          <Label>Access modes</Label>
          <div className="grid gap-2 sm:grid-cols-2">
            {ACCESS_MODES.map((mode) => {
              const checked = accessModes.includes(mode.value)
              return (
                <label
                  key={mode.value}
                  className="flex items-center gap-2 rounded-lg border px-3 py-2 text-sm"
                >
                  <Checkbox
                    checked={checked}
                    onCheckedChange={(v) => {
                      setAccessModes((prev) => {
                        if (v === true) {
                          return prev.includes(mode.value)
                            ? prev
                            : [...prev, mode.value]
                        }
                        return prev.filter((m) => m !== mode.value)
                      })
                    }}
                  />
                  {mode.label}
                </label>
              )
            })}
          </div>
        </div>

        <div className="flex flex-wrap justify-end gap-2">
          <Button variant="outline" asChild>
            <Link to="/kubernetes/persistentvolumeclaims">Cancel</Link>
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
            Create claim
          </Button>
        </div>
      </div>
    </ContentLoader>
  )
}
