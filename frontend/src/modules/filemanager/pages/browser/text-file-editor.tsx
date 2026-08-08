import { useMemo } from "react"

import {
  MonacoCodeEditor,
  languageFromFilename,
  type MonacoLanguage,
} from "@/components/monaco-editor"
import { cn } from "@/lib/utils"

import type { FileEntry } from "./api"

const TEXT_EXT =
  /\.(txt|md|markdown|rst|log|json|jsonc|ya?ml|toml|xml|csv|tsv|sql|go|tsx?|jsx?|mjs|cjs|py|rb|rs|c|h|cpp|cc|hpp|java|kt|swift|sh|bash|zsh|fish|ps1|bat|cmd|env|conf|cfg|ini|properties|service|timer|html?|css|scss|less|sass|svg|vue|svelte|astro|php|lua|r|pl|pm|tf|hcl|proto|graphql|gql|dockerfile|makefile|mk|cmake|gradle|mod|sum|lock|nix|ex|exs|erl|hrl|clj|edn|zig|nim|dart|scala|groovy|vim|el|tex|bib|org|adoc|textile|wiki|pem|crt|key|pub|known_hosts|htaccess)$/i

const TEXT_BASENAME =
  /^(dockerfile|makefile|gnumakefile|cmakelists\.txt|license|licence|readme|changelog|authors|copying|gemfile|rakefile|procfile|vagrantfile|\.gitignore|\.gitattributes|\.dockerignore|\.editorconfig|\.npmrc|\.nvmrc|\.env(\..+)?|\.bashrc|\.zshrc|\.profile|\.gitconfig)$/i

/** Files we treat as editable text in the file manager. */
export function isTextEditableFile(entry: FileEntry | null | undefined): boolean {
  if (!entry || entry.type === "directory") return false
  if (entry.mime_hint === "text") return true
  if (entry.mime_hint && entry.mime_hint !== "binary" && entry.mime_hint !== "text") {
    return false
  }
  const base = entry.name.split("/").pop() || entry.name
  return TEXT_BASENAME.test(base) || TEXT_EXT.test(base)
}

function looksLikeYaml(raw: string): boolean {
  const trimmed = raw.trim()
  if (!trimmed) return false
  if (trimmed.startsWith("---") || /^%YAML\b/m.test(trimmed)) return true

  const lines = trimmed
    .split(/\r?\n/)
    .map((l) => l.trimEnd())
    .filter((l) => l.trim() && !l.trim().startsWith("#"))
  if (lines.length < 1) return false

  let hits = 0
  for (const line of lines.slice(0, 30)) {
    const t = line.trim()
    if (
      /^[\w./@-]+:(\s|$)/.test(t) ||
      /^-(\s|$)/.test(t) ||
      /:\s*[|>][-+]?(\s|$)/.test(t)
    ) {
      hits++
    }
  }

  if (lines.length === 1) {
    return /:\s*[|>]/.test(trimmed) || (hits === 1 && /\n\s+\S/.test(raw))
  }
  return hits >= 2 && hits / lines.length >= 0.4
}

/**
 * Pick Monaco language from file contents. Empty / unknown → plaintext.
 * Filename is only a light hint when content is ambiguous.
 */
export function languageFromContent(
  content: string,
  filename = "",
): MonacoLanguage {
  const trimmed = content.trim()
  if (!trimmed) return "plaintext"

  if (trimmed.startsWith("{") || trimmed.startsWith("[")) return "json"
  if (looksLikeYaml(trimmed)) return "yaml"

  if (filename) {
    const fromName = languageFromFilename(filename)
    if (fromName !== "plaintext") return fromName
  }

  return "plaintext"
}

export function TextFileEditor({
  filename,
  value,
  onChange,
  readOnly,
  className,
}: {
  filename: string
  value: string
  onChange: (next: string) => void
  readOnly?: boolean
  className?: string
}) {
  const language = useMemo(
    () => languageFromContent(value, filename),
    [value, filename],
  )

  return (
    <div
      className={cn(
        "min-h-0 flex-1 overflow-hidden rounded-xl border border-input",
        "focus-within:border-ring focus-within:ring-[3px] focus-within:ring-ring/50",
        className,
      )}
      style={{ height: "min(60vh, 32rem)" }}
    >
      <MonacoCodeEditor
        value={value}
        onChange={onChange}
        language={language}
        height="100%"
        readOnly={readOnly}
      />
    </div>
  )
}
