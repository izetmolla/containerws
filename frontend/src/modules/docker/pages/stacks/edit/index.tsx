import { useEffect, useRef, useState } from "react"
import { useNavigate, useSearchParams } from "react-router"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { parseAsStringLiteral, useQueryState } from "nuqs"
import { List, Pencil, RefreshCw } from "lucide-react"
import { toast } from "sonner"

import ContentLoader from "@/components/content-loader"
import { Button } from "@/components/ui/button"
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Switch } from "@/components/ui/switch"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { getRequestErrorInfo, toastRequestError } from "@/lib/network"
import { cn } from "@/lib/utils"

import { EngineBanner } from "../../_shared/engine-status"
import { EnvironmentSelector } from "../../_shared/environment-selector"
import {
  createStack,
  DOCKER_STACKS_KEY,
  getStack,
  removeStack,
  stopStack,
  updateStack,
  validateCompose,
  type StackDetail,
} from "../api"
import { ComposeYamlEditor } from "./compose-yaml-editor"
import {
  emptyEnvPair,
  parseEnvFile,
  serializeEnvFile,
  type EnvPair,
} from "./env-file"
import { StackEditorTab } from "./stack-editor-tab"
import { StackEnvVariablesSection } from "./stack-env-section"
import { StackOverviewTab } from "./stack-overview-tab"
import { validateComposeYamlClient } from "./validate-compose-client"

const EMPTY_COMPOSE = `services:
  web:
    image: nginx:alpine
    ports:
      - "8080:80"
    restart: unless-stopped
`

const VALIDATE_DEBOUNCE_MS = 700

const STACK_TABS = ["stack", "editor"] as const

export default function StackEditPage() {
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const [params] = useSearchParams()
  const editId = params.get("id")?.trim() || ""
  const isEdit = Boolean(editId)

  const [tab, setTab] = useQueryState(
    "tab",
    parseAsStringLiteral(STACK_TABS).withDefault("stack")
  )

  const [name, setName] = useState("")
  const [yaml, setYaml] = useState(EMPTY_COMPOSE)
  const [envPairs, setEnvPairs] = useState<EnvPair[]>(() => [emptyEnvPair()])
  const [envAdvanced, setEnvAdvanced] = useState(false)
  const [envOpen, setEnvOpen] = useState(false)
  const [deployOnSave, setDeployOnSave] = useState(true)
  const [prune, setPrune] = useState(true)
  const [nameError, setNameError] = useState(false)
  const [serverError, setServerError] = useState<string | null>(null)
  const [serverValid, setServerValid] = useState(false)
  const [serverValidating, setServerValidating] = useState(false)
  const validateSeq = useRef(0)

  const detailQuery = useQuery({
    queryKey: [DOCKER_STACKS_KEY, "edit", editId],
    queryFn: () => getStack(editId),
    enabled: isEdit,
    refetchInterval: isEdit ? 12_000 : false,
  })

  const detail = detailQuery.data?.data
  const [prevDetail, setPrevDetail] = useState<StackDetail | undefined>()
  if (detail !== prevDetail) {
    setPrevDetail(detail)
    if (detail) {
      setName(detail.name || "")
      setYaml(detail.compose_yaml || "")
      setEnvPairs(parseEnvFile(detail.env_file || ""))
      setEnvAdvanced(false)
      setServerError(null)
      setServerValid(false)
    }
  }

  const [prevIsEdit, setPrevIsEdit] = useState(isEdit)
  if (isEdit !== prevIsEdit) {
    setPrevIsEdit(isEdit)
    if (!isEdit) {
      setName("")
      setYaml(EMPTY_COMPOSE)
      setEnvPairs([emptyEnvPair()])
      setEnvAdvanced(false)
      setDeployOnSave(true)
      setServerError(null)
      setServerValid(false)
    }
  }

  const envFile = serializeEnvFile(envPairs)
  const localIssue = validateComposeYamlClient(yaml)
  const yamlClientOk = !localIssue

  useEffect(() => {
    if (isEdit || !yamlClientOk) return

    const seq = ++validateSeq.current
    const timer = window.setTimeout(() => {
      setServerValidating(true)
      setServerValid(false)
      void validateCompose({
        name: name.trim() || "validate",
        compose_yaml: yaml,
        env_file: envFile,
      })
        .then(() => {
          if (seq !== validateSeq.current) return
          setServerError(null)
          setServerValid(true)
        })
        .catch((err: unknown) => {
          if (seq !== validateSeq.current) return
          setServerValid(false)
          const info = getRequestErrorInfo(err, "Compose validation failed")
          setServerError(info.description || info.title)
        })
        .finally(() => {
          if (seq === validateSeq.current) setServerValidating(false)
        })
    }, VALIDATE_DEBOUNCE_MS)

    return () => {
      window.clearTimeout(timer)
      validateSeq.current += 1
    }
  }, [yaml, name, yamlClientOk, isEdit, envFile])

  const invalidate = () => {
    void queryClient.invalidateQueries({ queryKey: [DOCKER_STACKS_KEY] })
  }

  const saveMutation = useMutation({
    mutationFn: async (opts?: { pull?: boolean }) => {
      const body = {
        name: name.trim(),
        compose_yaml: yaml,
        env_file: envFile,
        deploy: isEdit ? true : deployOnSave,
        pull: Boolean(opts?.pull),
        prune,
      }
      if (isEdit) return updateStack(editId, body)
      return createStack(body)
    },
    onSuccess: (res) => {
      toast.success(res.message || (isEdit ? "Stack updated" : "Stack created"))
      invalidate()
      const id = res.data?.id
      if (id && !isEdit) {
        navigate(`/docker/stacks/edit?id=${encodeURIComponent(id)}`)
      } else if (isEdit) {
        void detailQuery.refetch()
      }
    },
    onError: (err) =>
      toastRequestError(err, isEdit ? "Update failed" : "Create failed"),
  })

  const stopMutation = useMutation({
    mutationFn: () => stopStack(editId),
    onSuccess: (res) => {
      toast.success(res.message || "Stack stopped")
      invalidate()
      void detailQuery.refetch()
    },
    onError: (err) => toastRequestError(err, "Stop failed"),
  })

  const removeMutation = useMutation({
    mutationFn: () => removeStack(editId, true),
    onSuccess: (res) => {
      toast.success(res.message || "Stack removed")
      invalidate()
      navigate("/docker/stacks")
    },
    onError: (err) => toastRequestError(err, "Remove failed"),
  })

  const busy =
    saveMutation.isPending ||
    stopMutation.isPending ||
    removeMutation.isPending

  const onCreateSave = () => {
    if (!name.trim()) {
      setNameError(true)
      return
    }
    const issue = validateComposeYamlClient(yaml)
    if (issue) {
      toast.error(issue.message)
      return
    }
    if (yamlClientOk && serverError) {
      toast.error("Fix Compose validation errors before saving")
      return
    }
    saveMutation.mutate(undefined)
  }

  const onUpdateStack = (opts?: { pull?: boolean }) => {
    const issue = validateComposeYamlClient(yaml)
    if (issue) {
      toast.error(issue.message)
      return
    }
    saveMutation.mutate(opts)
  }

  const stackName = detail?.name || name || "Stack"

  if (!isEdit) {
    const canSave =
      !busy && yamlClientOk && !serverValidating && !serverError

    return (
      <ContentLoader
        title="Add stack"
        breadcrumb={[
          { label: "Docker", to: "/docker" },
          { label: "Stacks", to: "/docker/stacks" },
          { label: "Add stack" },
        ]}
        rightComponent={
          <div className="flex flex-wrap items-center gap-2">
            <EnvironmentSelector />
            <Button
              size="sm"
              variant="outline"
              onClick={() => window.location.reload()}
            >
              <RefreshCw className="size-3.5" />
            </Button>
          </div>
        }
      >
        <div className="flex w-full flex-col gap-6">
          <EngineBanner />
          <Card className="gap-0 py-0">
            <CardHeader className="border-b py-5">
              <CardTitle>Stack</CardTitle>
              <CardDescription>
                Deploy a Docker Compose project. Name becomes the Compose
                project label on containers.
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-5 py-6">
              <div className="grid max-w-md gap-1.5">
                <Label>Name</Label>
                <Input
                  value={name}
                  onChange={(e) => {
                    setName(e.target.value)
                    if (e.target.value.trim()) setNameError(false)
                  }}
                  placeholder="e.g. wordpress"
                  className={cn(nameError && "border-destructive")}
                />
                {nameError ? (
                  <p className="text-xs text-destructive">Name is required</p>
                ) : null}
              </div>

              <div className="grid gap-1.5">
                <div className="flex items-center justify-between gap-3">
                  <Label>Compose editor</Label>
                  <span className="text-[11px] text-muted-foreground">
                    docker-compose.yml
                  </span>
                </div>
                <ComposeYamlEditor
                  value={yaml}
                  onChange={setYaml}
                  disabled={busy}
                  height={520}
                  serverError={yamlClientOk ? serverError : null}
                  serverValidating={yamlClientOk && serverValidating}
                  serverValid={yamlClientOk && serverValid}
                />
              </div>

              <StackEnvVariablesSection
                pairs={envPairs}
                onPairsChange={setEnvPairs}
                advanced={envAdvanced}
                onAdvancedChange={setEnvAdvanced}
                open={envOpen}
                onOpenChange={setEnvOpen}
                disabled={busy}
              />

              <div className="flex items-center justify-between gap-3 rounded-lg border px-4 py-3">
                <div>
                  <p className="text-sm font-medium">Deploy after save</p>
                  <p className="text-xs text-muted-foreground">
                    Run{" "}
                    <code className="text-[11px]">docker compose up -d</code>{" "}
                    immediately
                  </p>
                </div>
                <Switch
                  checked={deployOnSave}
                  onCheckedChange={setDeployOnSave}
                />
              </div>
            </CardContent>
            <CardFooter className="flex flex-wrap gap-2 border-t py-4">
              <Button size="lg" disabled={!canSave} onClick={onCreateSave}>
                {saveMutation.isPending ? "Creating…" : "Create the stack"}
              </Button>
              <Button
                size="lg"
                variant="outline"
                onClick={() => navigate("/docker/stacks")}
              >
                Cancel
              </Button>
            </CardFooter>
          </Card>
        </div>
      </ContentLoader>
    )
  }

  return (
    <ContentLoader
      title="Stack details"
      breadcrumb={[
        { label: "Docker", to: "/docker" },
        { label: "Stacks", to: "/docker/stacks" },
        { label: stackName },
      ]}
      isLoading={detailQuery.isLoading}
      error={detailQuery.error}
      rightComponent={
        <div className="flex flex-wrap items-center gap-2">
          <EnvironmentSelector />
          <Button
            size="sm"
            variant="outline"
            onClick={() => void detailQuery.refetch()}
          >
            <RefreshCw className="size-3.5" />
          </Button>
        </div>
      }
    >
      <div className="flex w-full flex-col gap-6">
        <EngineBanner />

        {detail?.status === "error" && detail.message ? (
          <pre className="overflow-x-auto whitespace-pre-wrap rounded-lg border border-destructive/40 bg-destructive/10 px-4 py-3 text-xs text-destructive">
            {detail.message}
          </pre>
        ) : null}

        <Tabs
          value={tab}
          onValueChange={(v) => {
            if (v === "stack" || v === "editor") void setTab(v)
          }}
          className="gap-4"
        >
          <TabsList className="h-9">
            <TabsTrigger value="stack" className="gap-1.5 px-3">
              <List className="size-3.5" />
              Stack
            </TabsTrigger>
            <TabsTrigger value="editor" className="gap-1.5 px-3">
              <Pencil className="size-3.5" />
              Editor
            </TabsTrigger>
          </TabsList>

          <TabsContent value="stack" className="mt-0 outline-none">
            {detail ? (
              <StackOverviewTab
                detail={detail}
                busy={busy}
                onStop={() => stopMutation.mutate()}
                onRemove={() => {
                  if (
                    window.confirm(
                      `Remove stack ${detail.name}? Containers will be deleted.`
                    )
                  ) {
                    removeMutation.mutate()
                  }
                }}
                onCreateTemplate={() => {
                  toast.message("Create template from stack", {
                    description:
                      "Copy the Compose YAML from the Editor tab into Templates for now.",
                  })
                  void setTab("editor")
                }}
                onChanged={() => {
                  invalidate()
                  void detailQuery.refetch()
                }}
                onOpenEditor={() => void setTab("editor")}
              />
            ) : null}
          </TabsContent>

          <TabsContent value="editor" className="mt-0 outline-none">
            {detail ? (
              <StackEditorTab
                detail={detail}
                yaml={yaml}
                onYamlChange={setYaml}
                envPairs={envPairs}
                onEnvPairsChange={setEnvPairs}
                envAdvanced={envAdvanced}
                onEnvAdvancedChange={setEnvAdvanced}
                prune={prune}
                onPruneChange={setPrune}
                busy={busy}
                onUpdate={onUpdateStack}
                onChanged={() => {
                  invalidate()
                  void detailQuery.refetch()
                }}
              />
            ) : null}
          </TabsContent>
        </Tabs>
      </div>
    </ContentLoader>
  )
}
