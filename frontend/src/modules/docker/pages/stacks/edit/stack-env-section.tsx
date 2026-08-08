import { useRef, useState } from "react"
import {
  ChevronDown,
  Info,
  Plus,
  SquareCheckBig,
  Trash2,
  Upload,
} from "lucide-react"
import { toast } from "sonner"

import { Button } from "@/components/ui/button"
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from "@/components/ui/collapsible"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Textarea } from "@/components/ui/textarea"
import { cn } from "@/lib/utils"

import {
  emptyEnvPair,
  parseEnvFile,
  serializeEnvFile,
  type EnvPair,
} from "./env-file"

export function StackEnvVariablesSection({
  pairs,
  onPairsChange,
  advanced,
  onAdvancedChange,
  open,
  onOpenChange,
  disabled,
}: {
  pairs: EnvPair[]
  onPairsChange: (pairs: EnvPair[]) => void
  advanced: boolean
  onAdvancedChange: (value: boolean) => void
  open: boolean
  onOpenChange: (open: boolean) => void
  disabled?: boolean
}) {
  const fileRef = useRef<HTMLInputElement>(null)
  const [advancedText, setAdvancedText] = useState(() =>
    serializeEnvFile(pairs)
  )

  const syncAdvancedFromPairs = (next: EnvPair[]) => {
    setAdvancedText(serializeEnvFile(next))
  }

  const setSimple = (next: EnvPair[]) => {
    onPairsChange(next)
    syncAdvancedFromPairs(next)
  }

  const enterAdvanced = () => {
    setAdvancedText(serializeEnvFile(pairs))
    onAdvancedChange(true)
  }

  const leaveAdvanced = () => {
    onPairsChange(parseEnvFile(advancedText))
    onAdvancedChange(false)
  }

  const onAdvancedTextChange = (text: string) => {
    setAdvancedText(text)
    onPairsChange(parseEnvFile(text))
  }

  const loadFromFile = (file: File) => {
    const reader = new FileReader()
    reader.onload = () => {
      const text = typeof reader.result === "string" ? reader.result : ""
      const next = parseEnvFile(text)
      if (advanced) {
        setAdvancedText(serializeEnvFile(next))
      }
      onPairsChange(next)
      toast.success(`Loaded ${next.filter((p) => p.name.trim()).length} variable(s)`)
    }
    reader.onerror = () => toast.error("Could not read .env file")
    reader.readAsText(file)
  }

  return (
    <Collapsible open={open} onOpenChange={onOpenChange}>
      <CollapsibleTrigger asChild>
        <button
          type="button"
          className={cn(
            "flex w-full items-center justify-between rounded-lg border px-4 py-3 text-left text-sm font-medium",
            "hover:bg-muted/40"
          )}
        >
          Environment variables
          <ChevronDown
            className={cn(
              "size-4 text-muted-foreground transition-transform",
              open && "rotate-180"
            )}
          />
        </button>
      </CollapsibleTrigger>

      <CollapsibleContent className="space-y-4 pt-4">
        <p className="text-sm text-muted-foreground">
          Enter values below for referenced{" "}
          <code className="rounded bg-muted px-1 py-0.5 text-[11px]">
            {"${VAR}"}
          </code>{" "}
          variables in your compose file. You can also use a{" "}
          <code className="rounded bg-muted px-1 py-0.5 text-[11px]">
            stack.env
          </code>{" "}
          file — for example{" "}
          <code className="rounded bg-muted px-1 py-0.5 text-[11px]">
            TAG=v1.5
          </code>
          .
        </p>

        <div className="rounded-lg border border-sky-500/30 bg-sky-500/10 px-4 py-3 text-sm text-sky-950 dark:text-sky-100">
          <p className="mb-2 flex items-center gap-2 font-medium">
            <Info className="size-4 shrink-0" />
            stack.env file operation
          </p>
          <ul className="list-disc space-y-1 ps-5 text-xs leading-relaxed text-sky-950/90 dark:text-sky-100/90">
            <li>
              When deploying via <strong>Repository</strong>, the stack.env file
              must already reside in the Git repo.
            </li>
            <li>
              When deploying via <strong>Web editor</strong>,{" "}
              <strong>Upload</strong> or <strong>Custom template</strong>{" "}
              deployment, the stack.env file is auto created from what you set
              below.
            </li>
          </ul>
        </div>

        <div className="space-y-1">
          <button
            type="button"
            disabled={disabled}
            onClick={() => (advanced ? leaveAdvanced() : enterAdvanced())}
            className="inline-flex items-center gap-2 text-sm font-medium text-sky-600 hover:underline dark:text-sky-400"
          >
            <SquareCheckBig
              className={cn(
                "size-4",
                advanced ? "opacity-100" : "opacity-40"
              )}
            />
            Advanced mode
          </button>
          <p className="text-xs text-muted-foreground">
            Switch to advanced mode to copy &amp; paste multiple variables.
          </p>
        </div>

        {advanced ? (
          <Textarea
            value={advancedText}
            onChange={(e) => onAdvancedTextChange(e.target.value)}
            disabled={disabled}
            spellCheck={false}
            placeholder={"FOO=bar\nTAG=v1.5"}
            className="min-h-[160px] font-mono text-xs"
          />
        ) : (
          <div className="space-y-3">
            {pairs.map((pair, idx) => {
              const nameMissing =
                !pair.name.trim() && pair.value.trim() !== ""
              return (
                <div
                  key={pair.id}
                  className="grid gap-3 sm:grid-cols-[1fr_1fr_auto]"
                >
                  <div className="grid gap-1.5">
                    <Label className="text-xs text-muted-foreground">
                      name
                    </Label>
                    <Input
                      value={pair.name}
                      disabled={disabled}
                      placeholder="e.g. FOO"
                      className={cn(nameMissing && "border-amber-500")}
                      onChange={(e) => {
                        const next = pairs.map((p, i) =>
                          i === idx ? { ...p, name: e.target.value } : p
                        )
                        setSimple(next)
                      }}
                    />
                    {nameMissing ? (
                      <p className="text-xs text-amber-600 dark:text-amber-400">
                        ⚠ Environment variable name is required.
                      </p>
                    ) : null}
                  </div>
                  <div className="grid gap-1.5">
                    <Label className="text-xs text-muted-foreground">
                      value
                    </Label>
                    <Input
                      value={pair.value}
                      disabled={disabled}
                      placeholder="e.g. bar"
                      onChange={(e) => {
                        const next = pairs.map((p, i) =>
                          i === idx ? { ...p, value: e.target.value } : p
                        )
                        setSimple(next)
                      }}
                    />
                  </div>
                  <div className="flex items-end pb-0.5">
                    <Button
                      type="button"
                      size="icon-sm"
                      variant="ghost"
                      disabled={disabled || pairs.length <= 1}
                      className="text-destructive hover:bg-destructive/10 hover:text-destructive"
                      aria-label="Remove variable"
                      onClick={() => {
                        const next = pairs.filter((_, i) => i !== idx)
                        setSimple(next.length ? next : [emptyEnvPair()])
                      }}
                    >
                      <Trash2 className="size-4" />
                    </Button>
                  </div>
                </div>
              )
            })}
          </div>
        )}

        <div className="flex flex-wrap gap-2">
          {!advanced ? (
            <Button
              type="button"
              variant="outline"
              size="sm"
              disabled={disabled}
              onClick={() => setSimple([...pairs, emptyEnvPair()])}
            >
              <Plus data-icon="inline-start" />
              Add an environment variable
            </Button>
          ) : null}
          <Button
            type="button"
            variant="outline"
            size="sm"
            disabled={disabled}
            onClick={() => fileRef.current?.click()}
          >
            <Upload data-icon="inline-start" />
            Load variables from .env file
          </Button>
          <input
            ref={fileRef}
            type="file"
            accept=".env,text/plain"
            className="hidden"
            onChange={(e) => {
              const file = e.target.files?.[0]
              e.target.value = ""
              if (file) loadFromFile(file)
            }}
          />
        </div>

        <p className="text-xs text-muted-foreground">
          Environment changes will not take effect until redeployment occurs
          (Update the stack).
        </p>
      </CollapsibleContent>
    </Collapsible>
  )
}
