import { useMemo, useState } from "react"
import { Link, useNavigate } from "react-router"
import { useMutation, useQuery } from "@tanstack/react-query"
import { ArrowLeft, Loader2, Plus } from "lucide-react"
import { toast } from "sonner"

import ContentLoader from "@/components/content-loader"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { ReactSelect } from "@/components/ui/reactselect"
import { asArray } from "@/lib/as-array"
import { toastRequestError } from "@/lib/network"

import {
  createSecret,
  getStoredNamespace,
  K8S_NAMESPACES_KEY,
  listNamespaces,
} from "../../_shared/api"
import { ClusterBanner } from "../../_shared/cluster-banner"
import {
  SmartSecretBox,
  type SecretEntry,
} from "../../_shared/smart-secret-box"

type Option = { label: string; value: string }

const SECRET_TYPES: Option[] = [
  { label: "Opaque", value: "Opaque" },
  { label: "TLS (kubernetes.io/tls)", value: "kubernetes.io/tls" },
  {
    label: "Docker config (kubernetes.io/dockerconfigjson)",
    value: "kubernetes.io/dockerconfigjson",
  },
  {
    label: "Basic auth (kubernetes.io/basic-auth)",
    value: "kubernetes.io/basic-auth",
  },
  {
    label: "SSH auth (kubernetes.io/ssh-auth)",
    value: "kubernetes.io/ssh-auth",
  },
]

export default function CreateSecretPage() {
  const navigate = useNavigate()
  const [name, setName] = useState("")
  const [namespace, setNamespace] = useState(
    () => getStoredNamespace() || "default",
  )
  const [type, setType] = useState("Opaque")
  const [entries, setEntries] = useState<SecretEntry[]>([
    { id: 1, key: "", value: "" },
  ])

  const nsQuery = useQuery({
    queryKey: [K8S_NAMESPACES_KEY],
    queryFn: listNamespaces,
    staleTime: 60_000,
  })
  const nsOptions = useMemo<Option[]>(
    () =>
      asArray(nsQuery.data?.data).map((n) => ({
        label: n.name,
        value: n.name,
      })),
    [nsQuery.data],
  )

  const createMutation = useMutation({
    mutationFn: () => {
      const data: Record<string, string> = {}
      for (const entry of entries) {
        const key = entry.key.trim()
        if (!key) continue
        data[key] = entry.value
      }
      return createSecret({
        name: name.trim(),
        namespace: namespace.trim() || "default",
        type: type || "Opaque",
        data,
      })
    },
    onSuccess: (res) => {
      toast.success(res.message || "Secret created")
      navigate(
        `/kubernetes/secrets/${encodeURIComponent(namespace.trim() || "default")}/${encodeURIComponent(name.trim())}`,
      )
    },
    onError: (err) => toastRequestError(err, "Create failed"),
  })

  const canSubmit =
    Boolean(name.trim()) &&
    Boolean(namespace.trim()) &&
    !createMutation.isPending

  return (
    <ContentLoader
      title="Create secret"
      breadcrumb={[
        { label: "Kubernetes", to: "/kubernetes" },
        { label: "Secrets", to: "/kubernetes/secrets" },
        { label: "Create" },
      ]}
      rightComponent={
        <Button size="sm" variant="outline" asChild>
          <Link to="/kubernetes/secrets">
            <ArrowLeft className="size-3.5" />
            Back to list
          </Link>
        </Button>
      }
    >
      <div className="w-full space-y-6">
        <ClusterBanner />

        <p className="text-sm text-muted-foreground">
          Creates a Secret on the cluster via the Kubernetes API. Values are not
          stored in the app database.
        </p>

        <div className="grid gap-4 sm:grid-cols-2">
          <div className="space-y-1.5 sm:col-span-2">
            <Label htmlFor="secret-name">Name</Label>
            <Input
              id="secret-name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="my-secret"
              autoComplete="off"
            />
          </div>
          <div className="space-y-1.5">
            <Label>Namespace</Label>
            <ReactSelect<Option, false>
              size="sm"
              options={nsOptions}
              value={namespace}
              isSearchable
              isClearable={false}
              isLoading={nsQuery.isLoading}
              placeholder="Select namespace"
              noOptionsMessage={() => "No namespaces"}
              onValueChange={(v) => {
                if (v) setNamespace(v)
              }}
            />
          </div>
          <div className="space-y-1.5">
            <Label>Type</Label>
            <ReactSelect<Option, false>
              size="sm"
              options={SECRET_TYPES}
              value={type}
              isSearchable
              isClearable={false}
              placeholder="Opaque"
              onValueChange={(v) => {
                if (v) setType(v)
              }}
            />
          </div>
        </div>

        <div className="space-y-2">
          <SmartSecretBox entries={entries} onChange={setEntries} />
        </div>

        <div className="flex flex-wrap justify-end gap-2">
          <Button variant="outline" asChild>
            <Link to="/kubernetes/secrets">Cancel</Link>
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
            Create secret
          </Button>
        </div>
      </div>
    </ContentLoader>
  )
}
