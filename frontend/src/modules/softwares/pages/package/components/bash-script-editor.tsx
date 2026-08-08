import { useMemo, useRef, type KeyboardEvent } from "react"
import { cn } from "@/lib/utils"

type BashScriptEditorProps = {
  label: string
  value: string
  onChange: (next: string) => void
  placeholder?: string
  minRows?: number
  className?: string
}

/** Matches `leading-5` (1.25rem) used by gutter + textarea. */
const LINE_PX = 20

export function BashScriptEditor({
  label,
  value,
  onChange,
  placeholder = "#!/usr/bin/env bash\nset -euo pipefail\n",
  minRows = 14,
  className,
}: BashScriptEditorProps) {
  const textareaRef = useRef<HTMLTextAreaElement>(null)
  const lines = useMemo(() => {
    const parts = value.split("\n")
    return parts.length === 0 ? [""] : parts
  }, [value])

  const rowCount = Math.max(minRows, lines.length + 1)
  const bodyHeight = rowCount * LINE_PX

  const onKeyDown = (e: KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key !== "Tab") return
    e.preventDefault()
    const el = e.currentTarget
    const start = el.selectionStart
    const end = el.selectionEnd
    const next = value.slice(0, start) + "  " + value.slice(end)
    onChange(next)
    requestAnimationFrame(() => {
      el.selectionStart = el.selectionEnd = start + 2
    })
  }

  return (
    <div className={cn("space-y-1.5", className)}>
      <div className="flex items-center justify-between gap-2">
        <label className="text-sm font-medium text-foreground">{label}</label>
        <span className="font-mono text-[11px] text-muted-foreground">
          bash · {lines.length} line{lines.length === 1 ? "" : "s"}
        </span>
      </div>
      <div className="overflow-hidden rounded-lg border border-border/80 bg-[#0d1117] shadow-sm">
        <div className="flex items-center gap-1.5 border-b border-white/5 px-3 py-1.5">
          <span className="size-2.5 rounded-full bg-red-500/70" />
          <span className="size-2.5 rounded-full bg-amber-500/70" />
          <span className="size-2.5 rounded-full bg-emerald-500/70" />
          <span className="ml-2 font-mono text-[10px] text-white/40">
            script.sh
          </span>
        </div>
        {/* One scrollbar only — textarea grows with content; outer clips at max height. */}
        <div className="max-h-[28rem] min-h-[12rem] overflow-auto">
          <div className="flex min-h-[12rem]">
            <div
              aria-hidden
              className="shrink-0 select-none border-r border-white/5 bg-[#0d1117] px-2 py-3 text-right font-mono text-[12px] leading-5 text-white/25"
            >
              {Array.from({ length: rowCount }, (_, i) => (
                <div key={i} style={{ height: LINE_PX }}>
                  {i + 1}
                </div>
              ))}
            </div>
            <textarea
              ref={textareaRef}
              spellCheck={false}
              value={value}
              onChange={(e) => onChange(e.target.value)}
              onKeyDown={onKeyDown}
              placeholder={placeholder}
              rows={rowCount}
              style={{ height: bodyHeight + 24 /* py-3 */ }}
              className={cn(
                "min-w-0 flex-1 resize-none overflow-hidden bg-transparent px-3 py-3 font-mono text-[12.5px] leading-5",
                "text-[#e6edf3] outline-none placeholder:text-white/25"
              )}
            />
          </div>
        </div>
      </div>
    </div>
  )
}
