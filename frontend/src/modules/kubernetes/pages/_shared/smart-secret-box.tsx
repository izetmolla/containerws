import { useMemo, useState, type ReactNode } from "react"
import { Check, Copy, Eye, EyeOff, Plus, Trash2 } from "lucide-react"
import { toast } from "sonner"

import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { ButtonGroup } from "@/components/ui/button-group"
import { Input } from "@/components/ui/input"
import { Textarea } from "@/components/ui/textarea"
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip"
import { cn } from "@/lib/utils"

export type SecretEntry = { id: number; key: string; value: string }

function byteLength(value: string): number {
  return new TextEncoder().encode(value).length
}

function formatBytes(n: number): string {
  if (n < 1024) return `${n} B`
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`
  return `${(n / (1024 * 1024)).toFixed(1)} MB`
}

type SmartSecretBoxProps = {
  entries: SecretEntry[]
  onChange: (entries: SecretEntry[]) => void
  className?: string
}

export function SmartSecretBox({
  entries,
  onChange,
  className,
}: SmartSecretBoxProps) {
  const [revealedIds, setRevealedIds] = useState<Set<number>>(() => new Set())
  const [copiedId, setCopiedId] = useState<number | null>(null)

  const allRevealed =
    entries.length > 0 && entries.every((entry) => revealedIds.has(entry.id))

  const updateEntry = (id: number, patch: Partial<SecretEntry>) => {
    onChange(
      entries.map((entry) => (entry.id === id ? { ...entry, ...patch } : entry)),
    )
  }

  const toggleReveal = (id: number) => {
    setRevealedIds((prev) => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  }

  const setAllRevealed = (reveal: boolean) => {
    setRevealedIds(reveal ? new Set(entries.map((e) => e.id)) : new Set())
  }

  const copyValue = async (entry: SecretEntry) => {
    try {
      await navigator.clipboard.writeText(entry.value)
      setCopiedId(entry.id)
      toast.success("Copied to clipboard")
      window.setTimeout(
        () => setCopiedId((id) => (id === entry.id ? null : id)),
        1500,
      )
    } catch {
      toast.error("Copy failed")
    }
  }

  const addEntry = () => {
    const id = Date.now()
    onChange([...entries, { id, key: "", value: "" }])
    setRevealedIds((prev) => new Set(prev).add(id))
  }

  const removeEntry = (id: number) => {
    onChange(entries.filter((entry) => entry.id !== id))
    setRevealedIds((prev) => {
      const next = new Set(prev)
      next.delete(id)
      return next
    })
  }

  return (
    <TooltipProvider delayDuration={200}>
      <section className={cn("space-y-3", className)}>
        <div className="flex flex-wrap items-center justify-between gap-2">
          <div className="space-y-0.5">
            <h2 className="text-sm font-medium">Data</h2>
            <p className="text-xs text-muted-foreground">
              Values are shown decoded. Kubernetes stores them Base64-encoded.
            </p>
          </div>
          <div className="flex flex-wrap items-center gap-2">
            <Button
              type="button"
              size="sm"
              variant="outline"
              disabled={entries.length === 0}
              onClick={() => setAllRevealed(!allRevealed)}
            >
              {allRevealed ? (
                <EyeOff className="size-3.5" />
              ) : (
                <Eye className="size-3.5" />
              )}
              {allRevealed ? "Hide all" : "Reveal all"}
            </Button>
            <Button type="button" size="sm" variant="outline" onClick={addEntry}>
              <Plus className="size-3.5" />
              Add key
            </Button>
          </div>
        </div>

        {entries.length === 0 ? (
          <div className="rounded-xl border border-dashed px-4 py-10 text-center">
            <p className="text-sm text-muted-foreground">No secret keys yet.</p>
            <Button
              type="button"
              size="sm"
              variant="outline"
              className="mt-3"
              onClick={addEntry}
            >
              <Plus className="size-3.5" />
              Add first key
            </Button>
          </div>
        ) : (
          <div className="space-y-3">
            {entries.map((entry) => (
              <SecretFieldCard
                key={entry.id}
                entry={entry}
                revealed={revealedIds.has(entry.id)}
                copied={copiedId === entry.id}
                onKeyChange={(key) => updateEntry(entry.id, { key })}
                onValueChange={(value) => updateEntry(entry.id, { value })}
                onToggleReveal={() => toggleReveal(entry.id)}
                onCopy={() => void copyValue(entry)}
                onRemove={() => removeEntry(entry.id)}
              />
            ))}
          </div>
        )}
      </section>
    </TooltipProvider>
  )
}

type SecretFieldCardProps = {
  entry: SecretEntry
  revealed: boolean
  copied: boolean
  onKeyChange: (key: string) => void
  onValueChange: (value: string) => void
  onToggleReveal: () => void
  onCopy: () => void
  onRemove: () => void
}

function SecretFieldCard({
  entry,
  revealed,
  copied,
  onKeyChange,
  onValueChange,
  onToggleReveal,
  onCopy,
  onRemove,
}: SecretFieldCardProps) {
  const bytes = useMemo(() => byteLength(entry.value), [entry.value])
  const masked = !revealed

  return (
    <div className="overflow-hidden rounded-xl border bg-card shadow-xs">
      <div className="flex flex-wrap items-center gap-2 border-b bg-muted/40 px-3 py-2">
        <Input
          value={entry.key}
          onChange={(e) => onKeyChange(e.target.value)}
          placeholder="KEY"
          aria-label="Secret key"
          className="h-8 min-w-[10rem] flex-1 border-transparent bg-transparent font-mono text-xs shadow-none focus-visible:border-input focus-visible:bg-background"
        />
        <div className="flex flex-wrap items-center gap-1.5">
          <Badge
            variant="secondary"
            className="font-mono text-[10px] font-normal"
          >
            {formatBytes(bytes)}
          </Badge>
          <ButtonGroup aria-label={`Actions for ${entry.key || "secret key"}`}>
            <IconAction
              label={revealed ? "Hide value" : "Reveal value"}
              onClick={onToggleReveal}
            >
              {revealed ? (
                <EyeOff className="size-3.5" />
              ) : (
                <Eye className="size-3.5" />
              )}
            </IconAction>
            <IconAction
              label="Copy value"
              onClick={onCopy}
              disabled={!entry.value}
            >
              {copied ? (
                <Check className="size-3.5 text-emerald-600" />
              ) : (
                <Copy className="size-3.5" />
              )}
            </IconAction>
            <IconAction label="Remove key" onClick={onRemove} destructive>
              <Trash2 className="size-3.5" />
            </IconAction>
          </ButtonGroup>
        </div>
      </div>

      <div className="relative p-3">
        <Textarea
          value={
            masked ? (entry.value ? "••••••••••••••••••••" : "") : entry.value
          }
          onChange={(e) => {
            if (!masked) onValueChange(e.target.value)
          }}
          readOnly={masked}
          placeholder={masked ? "Hidden — reveal to edit" : "Secret value"}
          spellCheck={false}
          className={cn(
            "min-h-24 resize-y font-mono text-xs",
            masked && "cursor-default text-muted-foreground select-none",
          )}
          aria-label={`Value for ${entry.key || "secret key"}`}
        />
        {masked && entry.value ? (
          <div className="pointer-events-none absolute inset-3 rounded-md bg-gradient-to-b from-transparent via-background/40 to-background/80" />
        ) : null}
      </div>
    </div>
  )
}

function IconAction({
  label,
  onClick,
  disabled,
  destructive,
  children,
}: {
  label: string
  onClick: () => void
  disabled?: boolean
  destructive?: boolean
  children: ReactNode
}) {
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <Button
          type="button"
          size="icon-sm"
          variant="outline"
          disabled={disabled}
          aria-label={label}
          className={cn(
            destructive &&
              "text-destructive hover:bg-destructive/10 hover:text-destructive",
          )}
          onClick={onClick}
        >
          {children}
        </Button>
      </TooltipTrigger>
      <TooltipContent side="top">{label}</TooltipContent>
    </Tooltip>
  )
}
