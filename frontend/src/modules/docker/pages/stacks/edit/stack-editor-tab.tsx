import { useEffect, useRef, useState } from "react"
import { Copy, Info } from "lucide-react"
import { toast } from "sonner"

import { Button } from "@/components/ui/button"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
} from "@/components/ui/card"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Switch } from "@/components/ui/switch"
import { getRequestErrorInfo } from "@/lib/network"

import {
  validateCompose,
  type StackDetail,
} from "../api"
import { ComposeYamlEditor } from "./compose-yaml-editor"
import {
  emptyEnvPair,
  serializeEnvFile,
  type EnvPair,
} from "./env-file"
import { StackContainersPanel } from "./stack-containers-panel"
import { StackEnvVariablesSection } from "./stack-env-section"
import { validateComposeYamlClient } from "./validate-compose-client"

const VALIDATE_DEBOUNCE_MS = 700

export function StackEditorTab({
  detail,
  yaml,
  onYamlChange,
  envPairs,
  onEnvPairsChange,
  envAdvanced,
  onEnvAdvancedChange,
  prune,
  onPruneChange,
  busy,
  onUpdate,
  onChanged,
}: {
  detail: StackDetail
  yaml: string
  onYamlChange: (value: string) => void
  envPairs: EnvPair[]
  onEnvPairsChange: (pairs: EnvPair[]) => void
  envAdvanced: boolean
  onEnvAdvancedChange: (value: boolean) => void
  prune: boolean
  onPruneChange: (value: boolean) => void
  busy: boolean
  onUpdate: (opts?: { pull?: boolean }) => void
  onChanged: () => void
}) {
  const [envOpen, setEnvOpen] = useState(true)
  const [updateOpen, setUpdateOpen] = useState(false)
  const [updatePull, setUpdatePull] = useState(false)
  const [serverError, setServerError] = useState<string | null>(null)
  const [serverValid, setServerValid] = useState(false)
  const [serverValidating, setServerValidating] = useState(false)
  const validateSeq = useRef(0)

  const envFile = serializeEnvFile(envPairs)
  const localIssue = validateComposeYamlClient(yaml)
  const yamlClientOk = !localIssue

  useEffect(() => {
    if (!yamlClientOk) return

    const seq = ++validateSeq.current
    const timer = window.setTimeout(() => {
      setServerValidating(true)
      setServerValid(false)
      void validateCompose({
        name: detail.name || "validate",
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
  }, [yaml, detail.name, yamlClientOk, envFile])

  const canUpdate =
    !busy && yamlClientOk && !serverValidating && !serverError

  const copyYaml = async () => {
    try {
      await navigator.clipboard.writeText(yaml)
      toast.success("Compose file copied")
    } catch {
      toast.error("Could not copy to clipboard")
    }
  }

  const openUpdateDialog = () => {
    if (!canUpdate) return
    setUpdatePull(false)
    setUpdateOpen(true)
  }

  const confirmUpdate = () => {
    setUpdateOpen(false)
    onUpdate({ pull: updatePull })
  }

  return (
    <div className="flex flex-col gap-6">
      <Card className="gap-0 py-0">
        <CardHeader className="border-b py-5">
          <CardDescription className="text-sm text-foreground/80">
            This stack will be deployed using{" "}
            <code className="rounded bg-muted px-1 py-0.5 text-[11px]">
              docker compose
            </code>
            . You can get more information about Compose file format in the{" "}
            <a
              href="https://docs.docker.com/compose/compose-file/"
              target="_blank"
              rel="noreferrer"
              className="text-sky-600 underline-offset-2 hover:underline dark:text-sky-400"
            >
              official documentation
            </a>
            .
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-5 py-5">
          <div className="flex flex-wrap items-start justify-between gap-3 rounded-lg border bg-muted/30 px-3 py-2.5 text-sm">
            <p className="flex items-start gap-2 text-muted-foreground">
              <Info className="mt-0.5 size-4 shrink-0" />
              Define or paste the content of your docker compose file here.
            </p>
            <Button
              type="button"
              variant="link"
              size="sm"
              className="h-auto px-0"
              onClick={() => void copyYaml()}
            >
              <Copy data-icon="inline-start" />
              Copy
            </Button>
          </div>

          <ComposeYamlEditor
            value={yaml}
            onChange={onYamlChange}
            disabled={busy}
            height={520}
            serverError={yamlClientOk ? serverError : null}
            serverValidating={yamlClientOk && serverValidating}
            serverValid={yamlClientOk && serverValid}
          />

          <StackEnvVariablesSection
            pairs={envPairs.length ? envPairs : [emptyEnvPair()]}
            onPairsChange={onEnvPairsChange}
            advanced={envAdvanced}
            onAdvancedChange={onEnvAdvancedChange}
            open={envOpen}
            onOpenChange={setEnvOpen}
            disabled={busy}
          />

          <div className="space-y-3">
            <h3 className="text-sm font-semibold">Options</h3>
            <div className="flex items-center justify-between gap-3 rounded-lg border px-4 py-3">
              <div>
                <p className="text-sm font-medium">Prune services</p>
                <p className="text-xs text-muted-foreground">
                  Remove services no longer defined in the Compose file
                  (orphans).
                </p>
              </div>
              <Switch checked={prune} onCheckedChange={onPruneChange} />
            </div>
          </div>

          <div className="space-y-3">
            <h3 className="text-sm font-semibold">Actions</h3>
            <Button size="lg" disabled={!canUpdate} onClick={openUpdateDialog}>
              {busy ? "Updating…" : "Update the stack"}
            </Button>
          </div>
        </CardContent>
      </Card>

      <Dialog
        open={updateOpen}
        onOpenChange={(open) => {
          if (!busy) {
            setUpdateOpen(open)
            if (!open) setUpdatePull(false)
          }
        }}
      >
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>Update the stack</DialogTitle>
            <DialogDescription>
              Deploy the current Compose file for{" "}
              <strong>{detail.name}</strong>
              {prune
                ? " and prune orphaned services."
                : "."}
            </DialogDescription>
          </DialogHeader>
          <div className="flex items-center justify-between gap-3 rounded-lg border px-4 py-3">
            <div className="space-y-0.5">
              <p className="text-sm font-medium">Re-pull images</p>
              <p className="text-xs text-muted-foreground">
                Pull the latest images for all services before deploying (
                <code className="text-[11px]">docker compose up --pull always</code>
                ).
              </p>
            </div>
            <Switch
              checked={updatePull}
              onCheckedChange={setUpdatePull}
              disabled={busy}
            />
          </div>
          <DialogFooter>
            <Button
              variant="outline"
              disabled={busy}
              onClick={() => setUpdateOpen(false)}
            >
              Cancel
            </Button>
            <Button disabled={busy} onClick={confirmUpdate}>
              {busy
                ? updatePull
                  ? "Pulling & updating…"
                  : "Updating…"
                : "Update the stack"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <StackContainersPanel
        stackName={detail.name}
        containers={detail.containers ?? []}
        onChanged={onChanged}
      />
    </div>
  )
}
