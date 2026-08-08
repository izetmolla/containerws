import { useMemo, useState } from "react"
import { Link, useNavigate } from "react-router"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { ArrowLeft, Loader2, Plus } from "lucide-react"
import { toast } from "sonner"

import ContentLoader from "@/components/content-loader"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { ReactSelect } from "@/components/ui/reactselect"
import { ReactSelectCreatable } from "@/components/ui/reactselectcreatable"
import { Textarea } from "@/components/ui/textarea"
import { asArray } from "@/lib/as-array"
import { toastRequestError } from "@/lib/network"

import {
  createJob,
  createNamespace,
  getStoredNamespace,
  K8S_NAMESPACES_KEY,
  listNamespaces,
} from "../../_shared/api"
import { ClusterBanner } from "../../_shared/cluster-banner"

type Option = { label: string; value: string }

const RESTART_OPTIONS: Option[] = [
  { label: "Never", value: "Never" },
  { label: "OnFailure", value: "OnFailure" },
]

function splitTokens(raw: string): string[] {
  return raw
    .split(/\s+/)
    .map((s) => s.trim())
    .filter(Boolean)
}

export default function CreateJobPage() {
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const [name, setName] = useState("")
  const [namespace, setNamespace] = useState(
    () => getStoredNamespace() || "default",
  )
  const [image, setImage] = useState("busybox:1.36")
  const [commandText, setCommandText] = useState("/bin/sh -c")
  const [argsText, setArgsText] = useState("echo hello && sleep 5")
  const [completions, setCompletions] = useState("1")
  const [parallelism, setParallelism] = useState("1")
  const [backoffLimit, setBackoffLimit] = useState("6")
  const [restartPolicy, setRestartPolicy] = useState<"Never" | "OnFailure">(
    "Never",
  )

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
      const command = splitTokens(commandText)
      const args = argsText.trim() ? [argsText.trim()] : []
      // If command is "/bin/sh -c", keep as command parts; args is the script string.
      // If user put full command in command field only, that's fine too.
      const bodyCommand =
        command.length > 0
          ? command
          : undefined
      return createJob({
        name: name.trim(),
        namespace: namespace.trim() || "default",
        image: image.trim(),
        command: bodyCommand,
        args: args.length ? args : undefined,
        completions: Number(completions) || 1,
        parallelism: Number(parallelism) || 1,
        backoff_limit: Number(backoffLimit) || 6,
        restart_policy: restartPolicy,
      })
    },
    onSuccess: (res) => {
      toast.success(res.message || "Job created")
      const ns = namespace.trim() || "default"
      navigate(
        `/kubernetes/jobs/${encodeURIComponent(ns)}/${encodeURIComponent(name.trim())}`,
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
      title="Create Job"
      breadcrumb={[
        { label: "Kubernetes", to: "/kubernetes" },
        { label: "Jobs", to: "/kubernetes/jobs" },
        { label: "Create" },
      ]}
      rightComponent={
        <Button size="sm" variant="outline" asChild>
          <Link to="/kubernetes/jobs">
            <ArrowLeft className="size-3.5" />
            Back to list
          </Link>
        </Button>
      }
    >
      <div className="w-full space-y-6">
        <ClusterBanner />

        <p className="text-sm text-muted-foreground">
          Creates a batch Job on the cluster. Command is space-separated;
          arguments are passed as a single string (useful with{" "}
          <code className="rounded bg-muted px-1 py-0.5 font-mono text-xs">
            /bin/sh -c
          </code>
          ).
        </p>

        <div className="grid gap-4 sm:grid-cols-2">
          <div className="space-y-1.5 sm:col-span-2">
            <Label htmlFor="job-name">Name</Label>
            <Input
              id="job-name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="hello-job"
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
            <Label htmlFor="job-image">Image</Label>
            <Input
              id="job-image"
              value={image}
              onChange={(e) => setImage(e.target.value)}
              placeholder="busybox:1.36"
              className="font-mono text-sm"
            />
          </div>
          <div className="space-y-1.5 sm:col-span-2">
            <Label htmlFor="job-command">Command</Label>
            <Input
              id="job-command"
              value={commandText}
              onChange={(e) => setCommandText(e.target.value)}
              placeholder="/bin/sh -c"
              className="font-mono text-sm"
            />
          </div>
          <div className="space-y-1.5 sm:col-span-2">
            <Label htmlFor="job-args">Args</Label>
            <Textarea
              id="job-args"
              value={argsText}
              onChange={(e) => setArgsText(e.target.value)}
              placeholder="echo hello"
              className="min-h-20 font-mono text-xs"
            />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="job-completions">Completions</Label>
            <Input
              id="job-completions"
              type="number"
              min={1}
              value={completions}
              onChange={(e) => setCompletions(e.target.value)}
            />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="job-parallelism">Parallelism</Label>
            <Input
              id="job-parallelism"
              type="number"
              min={1}
              value={parallelism}
              onChange={(e) => setParallelism(e.target.value)}
            />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="job-backoff">Backoff limit</Label>
            <Input
              id="job-backoff"
              type="number"
              min={0}
              value={backoffLimit}
              onChange={(e) => setBackoffLimit(e.target.value)}
            />
          </div>
          <div className="space-y-1.5">
            <Label>Restart policy</Label>
            <ReactSelect<Option, false>
              size="sm"
              options={RESTART_OPTIONS}
              value={restartPolicy}
              isClearable={false}
              onValueChange={(v) => {
                if (v === "Never" || v === "OnFailure") setRestartPolicy(v)
              }}
            />
          </div>
        </div>

        <div className="flex flex-wrap justify-end gap-2">
          <Button variant="outline" asChild>
            <Link to="/kubernetes/jobs">Cancel</Link>
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
            Create Job
          </Button>
        </div>
      </div>
    </ContentLoader>
  )
}
