import { useMemo, useSyncExternalStore } from "react"
import Editor, {
  type EditorProps,
  type OnChange,
  type OnMount,
} from "@monaco-editor/react"

import { cn } from "@/lib/utils"

const LINE_HEIGHT = 20
const VERTICAL_PADDING = 24

function subscribeTheme(onStoreChange: () => void) {
  const obs = new MutationObserver(onStoreChange)
  obs.observe(document.documentElement, {
    attributes: true,
    attributeFilter: ["class"],
  })
  return () => obs.disconnect()
}

function isDarkTheme(): boolean {
  if (typeof document === "undefined") return true
  return document.documentElement.classList.contains("dark")
}

function heightForContent(
  value: string,
  minLines: number,
  maxLines: number,
): number {
  const lines = Math.max(1, value.split(/\r?\n/).length)
  const clamped = Math.min(maxLines, Math.max(minLines, lines + 1))
  return clamped * LINE_HEIGHT + VERTICAL_PADDING
}

export type MonacoLanguage =
  | "yaml"
  | "json"
  | "plaintext"
  | "shell"
  | "markdown"
  | "typescript"
  | "javascript"
  | "html"
  | "css"
  | "xml"
  | "dockerfile"
  | "go"
  | "python"
  | "sql"
  | "ini"
  | string

export type MonacoCodeEditorProps = {
  value: string
  onChange?: (value: string) => void
  language?: MonacoLanguage
  /** Fixed height. Ignored when `autoHeight` is true. */
  height?: string | number
  /** Grow/shrink the editor to fit content between min/max lines. */
  autoHeight?: boolean
  minLines?: number
  maxLines?: number
  readOnly?: boolean
  className?: string
  options?: EditorProps["options"]
  onMount?: OnMount
}

/** Shared Monaco editor that follows the app light/dark theme. */
export function MonacoCodeEditor({
  value,
  onChange,
  language = "plaintext",
  height = "100%",
  autoHeight = false,
  minLines = 4,
  maxLines = 20,
  readOnly,
  className,
  options,
  onMount,
}: MonacoCodeEditorProps) {
  const dark = useSyncExternalStore(subscribeTheme, isDarkTheme, () => true)

  const autoPx = useMemo(
    () => (autoHeight ? heightForContent(value, minLines, maxLines) : null),
    [autoHeight, value, minLines, maxLines],
  )

  const resolvedHeight: string | number =
    autoPx != null
      ? autoPx
      : typeof height === "number"
        ? height
        : height === "100%"
          ? "100%"
          : height

  const handleChange: OnChange = (next) => {
    onChange?.(next ?? "")
  }

  const handleMount: OnMount = (editor, monaco) => {
    onMount?.(editor, monaco)
    if (autoHeight) {
      editor.layout()
    }
  }

  return (
    <div
      className={cn("relative w-full overflow-hidden", className)}
      style={
        resolvedHeight === "100%"
          ? { height: "100%", minHeight: `${minLines * LINE_HEIGHT + VERTICAL_PADDING}px` }
          : { height: resolvedHeight }
      }
    >
      <Editor
        height={typeof resolvedHeight === "number" ? resolvedHeight : "100%"}
        language={language}
        theme={dark ? "vs-dark" : "light"}
        value={value}
        onChange={handleChange}
        onMount={handleMount}
        loading={
          <pre className="h-full overflow-auto bg-background p-3 font-mono text-[13px] leading-5 whitespace-pre-wrap text-foreground/70">
            {value || " "}
          </pre>
        }
        options={{
          readOnly: Boolean(readOnly),
          minimap: { enabled: false },
          fontSize: 13,
          fontFamily:
            "ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace",
          lineHeight: LINE_HEIGHT,
          lineNumbers: "on",
          wordWrap: "on",
          scrollBeyondLastLine: false,
          automaticLayout: true,
          tabSize: 2,
          folding: false,
          stickyScroll: { enabled: false },
          overviewRulerLanes: 0,
          hideCursorInOverviewRuler: true,
          overviewRulerBorder: false,
          renderLineHighlight: readOnly ? "none" : "line",
          bracketPairColorization: { enabled: true },
          padding: { top: 12, bottom: 12 },
          scrollbar: autoHeight
            ? {
                vertical: "hidden",
                horizontal: "auto",
                handleMouseWheel: true,
                alwaysConsumeMouseWheel: false,
              }
            : undefined,
          ...options,
        }}
      />
    </div>
  )
}

/** Map a filename to a Monaco language id. */
export function languageFromFilename(filename: string): MonacoLanguage {
  const base = filename.split("/").pop()?.toLowerCase() || filename.toLowerCase()
  if (base === "dockerfile" || base.startsWith("dockerfile.")) return "dockerfile"
  if (base === "makefile" || base === "gnumakefile") return "plaintext"
  if (/\.(json|jsonc)$/.test(base)) return "json"
  if (/\.(ya?ml)$/.test(base)) return "yaml"
  if (/\.(md|markdown)$/.test(base)) return "markdown"
  if (/\.(ts|tsx)$/.test(base)) return "typescript"
  if (/\.(js|jsx|mjs|cjs)$/.test(base)) return "javascript"
  if (/\.(html?)$/.test(base)) return "html"
  if (/\.(css|scss|less|sass)$/.test(base)) return "css"
  if (/\.xml$/.test(base)) return "xml"
  if (/\.go$/.test(base)) return "go"
  if (/\.py$/.test(base)) return "python"
  if (/\.sql$/.test(base)) return "sql"
  if (/\.(sh|bash|zsh|fish)$/.test(base)) return "shell"
  if (/\.(ini|cfg|conf|env|properties|toml)$/.test(base)) return "ini"
  return "plaintext"
}
