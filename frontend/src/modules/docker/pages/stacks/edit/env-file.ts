export type EnvPair = {
  id: string
  name: string
  value: string
}

let envSeq = 0
export function nextEnvId() {
  envSeq += 1
  return `env-${envSeq}-${Date.now().toString(36)}`
}

export function emptyEnvPair(): EnvPair {
  return { id: nextEnvId(), name: "", value: "" }
}

/** Parse `.env` / stack.env text into editable name/value rows. */
export function parseEnvFile(text: string): EnvPair[] {
  const raw = text.replace(/\r\n/g, "\n")
  const pairs: EnvPair[] = []
  for (const line of raw.split("\n")) {
    const trimmed = line.trim()
    if (!trimmed || trimmed.startsWith("#")) continue
    const eq = trimmed.indexOf("=")
    if (eq < 0) {
      pairs.push({ id: nextEnvId(), name: trimmed, value: "" })
      continue
    }
    let value = trimmed.slice(eq + 1)
    if (
      (value.startsWith('"') && value.endsWith('"')) ||
      (value.startsWith("'") && value.endsWith("'"))
    ) {
      value = value.slice(1, -1)
    }
    pairs.push({
      id: nextEnvId(),
      name: trimmed.slice(0, eq).trim(),
      value,
    })
  }
  return pairs.length ? pairs : [emptyEnvPair()]
}

/** Serialize rows to `.env` file content (skips blank names). */
export function serializeEnvFile(pairs: EnvPair[]): string {
  const lines: string[] = []
  for (const p of pairs) {
    const name = p.name.trim()
    if (!name) continue
    const value = p.value
    const needsQuote =
      value.includes(" ") ||
      value.includes("#") ||
      value.includes("=") ||
      value.includes("\n")
    if (needsQuote) {
      lines.push(`${name}="${value.replace(/\\/g, "\\\\").replace(/"/g, '\\"')}"`)
    } else {
      lines.push(`${name}=${value}`)
    }
  }
  return lines.join("\n")
}

export function envPairsHaveErrors(pairs: EnvPair[]): boolean {
  return pairs.some((p) => !p.name.trim() && p.value.trim() !== "")
}
