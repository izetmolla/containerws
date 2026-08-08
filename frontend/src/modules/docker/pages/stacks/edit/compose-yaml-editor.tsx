import { AlertCircle, CheckCircle2, Loader2 } from "lucide-react"

import { MonacoCodeEditor } from "@/components/monaco-editor"
import { cn } from "@/lib/utils"

import { validateComposeYamlClient } from "./validate-compose-client"

export type ComposeYamlEditorProps = {
  value: string
  onChange: (value: string) => void
  /** Authoritative server-side validation message (e.g. docker compose config). */
  serverError?: string | null
  serverValidating?: boolean
  serverValid?: boolean
  className?: string
  height?: number | string
  disabled?: boolean
}

export function ComposeYamlEditor({
  value,
  onChange,
  serverError,
  serverValidating,
  serverValid,
  className,
  height = 460,
  disabled,
}: ComposeYamlEditorProps) {
  const clientIssue = validateComposeYamlClient(value)
  const hasClientIssue = Boolean(clientIssue)
  const hasServerError = Boolean(serverError)
  const showOk =
    !hasClientIssue &&
    !hasServerError &&
    !serverValidating &&
    value.trim().length > 0

  const pxHeight = typeof height === "number" ? `${height}px` : height

  return (
    <div className={cn("grid gap-2", className)}>
      <div
        className={cn(
          "overflow-hidden rounded-lg border border-input",
          "focus-within:border-ring focus-within:ring-[3px] focus-within:ring-ring/50",
          (hasClientIssue || hasServerError) && "border-destructive/60",
        )}
        style={{ height: pxHeight }}
      >
        <MonacoCodeEditor
          value={value}
          onChange={onChange}
          language="yaml"
          height="100%"
          readOnly={disabled}
        />
      </div>

      <div className="flex min-h-5 flex-wrap items-start gap-2 text-xs">
        {serverValidating ? (
          <p className="flex items-center gap-1.5 text-muted-foreground">
            <Loader2 className="size-3.5 animate-spin" />
            Checking with docker compose config…
          </p>
        ) : null}

        {!serverValidating && hasServerError ? (
          <pre className="w-full whitespace-pre-wrap break-words rounded-md border border-destructive/40 bg-destructive/10 px-3 py-2 font-mono text-[11px] leading-relaxed text-destructive">
            {serverError}
          </pre>
        ) : null}

        {!serverValidating && !hasServerError && hasClientIssue ? (
          <p className="flex items-start gap-1.5 text-destructive">
            <AlertCircle className="mt-0.5 size-3.5 shrink-0" />
            <span>
              {clientIssue?.line
                ? `Line ${clientIssue.line}: ${clientIssue.message}`
                : clientIssue?.message}
            </span>
          </p>
        ) : null}

        {showOk ? (
          <p className="flex items-center gap-1.5 text-emerald-700 dark:text-emerald-400">
            <CheckCircle2 className="size-3.5" />
            {serverValid ? "Valid Compose file" : "YAML looks valid"}
          </p>
        ) : null}
      </div>
    </div>
  )
}
