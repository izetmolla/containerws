import { useMemo, useState } from "react"
import { useQuery } from "@tanstack/react-query"
import {
  Globe,
  Lock,
  Maximize2,
  Plus,
  Route,
  Tags,
  Trash2,
} from "lucide-react"

import { MonacoCodeEditor } from "@/components/monaco-editor"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { ReactSelect } from "@/components/ui/reactselect"
import { ReactSelectCreatable } from "@/components/ui/reactselectcreatable"
import { Separator } from "@/components/ui/separator"
import { cn } from "@/lib/utils"

import {
  getIngressFormOptions,
  K8S_INGRESSES_KEY,
  type IngressClassOption,
  type IngressFormOptions,
  type IngressPath,
  type IngressRule,
  type IngressServiceOption,
  type IngressTLS,
} from "../_shared/api"

type SelectOption = { label: string; value: string }

export function emptyIngressPath(): IngressPath {
  return {
    path: "/",
    path_type: "Prefix",
    service_name: "",
    service_port: 80,
  }
}

export function emptyIngressRule(): IngressRule {
  return { host: "", paths: [emptyIngressPath()] }
}

export function emptyIngressTLS(): IngressTLS {
  return { hosts: [], secret_name: "" }
}

export function useIngressFormOptions(namespace: string) {
  return useQuery({
    queryKey: [K8S_INGRESSES_KEY, "options", namespace || "default"],
    queryFn: () => getIngressFormOptions(namespace || "default"),
    enabled: Boolean(namespace?.trim()),
    staleTime: 15_000,
  })
}

function ensureOption(
  options: SelectOption[],
  value: string | null | undefined,
): SelectOption[] {
  const v = (value ?? "").trim()
  if (!v) return options
  if (options.some((o) => o.value === v)) return options
  return [{ label: v, value: v }, ...options]
}

function ensureOptions(
  options: SelectOption[],
  values: string[],
): SelectOption[] {
  let next = options
  for (const value of values) {
    next = ensureOption(next, value)
  }
  return next
}

const PATH_TYPE_OPTIONS: SelectOption[] = [
  { label: "Prefix", value: "Prefix" },
  { label: "Exact", value: "Exact" },
  { label: "ImplementationSpecific", value: "ImplementationSpecific" },
]

function classOptions(classes: IngressClassOption[]): SelectOption[] {
  return classes.map((c) => ({
    value: c.name,
    label: c.default ? `${c.name} (default)` : c.name,
  }))
}

function serviceOptions(services: IngressServiceOption[]): SelectOption[] {
  return services.map((s) => ({
    value: s.name,
    label: s.type ? `${s.name} · ${s.type}` : s.name,
  }))
}

function portOptionsForService(
  services: IngressServiceOption[],
  serviceName: string,
): SelectOption[] {
  const svc = services.find((s) => s.name === serviceName)
  if (!svc) return []
  return svc.ports.map((p) => ({
    value: String(p.port),
    label: p.name ? `${p.port} (${p.name})` : String(p.port),
  }))
}

function tlsSecretOptions(opts: IngressFormOptions | undefined): SelectOption[] {
  return (opts?.tls_secrets ?? []).map((s) => ({
    value: s.name,
    label: s.name,
  }))
}

type IngressClassFieldProps = {
  value: string
  onChange: (value: string) => void
  options?: IngressFormOptions
  isLoading?: boolean
}

export function IngressClassField({
  value,
  onChange,
  options,
  isLoading,
}: IngressClassFieldProps) {
  const opts = useMemo(
    () => ensureOption(classOptions(options?.classes ?? []), value),
    [options?.classes, value],
  )
  return (
    <div className="space-y-2">
      <Label>Ingress class</Label>
      <ReactSelectCreatable<SelectOption, false>
        size="sm"
        isClearable
        isSearchable
        isLoading={isLoading}
        options={opts}
        value={value || null}
        onValueChange={(v) => onChange(v ?? "")}
        placeholder="Select or type a class…"
        formatCreateLabel={(input) => `Use “${input}”`}
        noOptionsMessage={() => "No IngressClass found — type a name"}
      />
      <p className="text-xs text-muted-foreground">
        Pick a cluster IngressClass or enter a custom name.
      </p>
    </div>
  )
}

type IngressEditorProps = {
  namespace: string
  ingressClass: string
  onIngressClassChange: (value: string) => void
  rules: IngressRule[]
  onRulesChange: (rules: IngressRule[]) => void
  tls: IngressTLS[]
  onTlsChange: (tls: IngressTLS[]) => void
  labels: Record<string, string>
  onLabelsChange: (labels: Record<string, string>) => void
  annotations: Record<string, string>
  onAnnotationsChange: (annotations: Record<string, string>) => void
  /** Sync labels/annotations editors when the resource identity changes. */
  metadataResetKey?: string
  compact?: boolean
}

type KVPair = { id: number; key: string; value: string }

export function recordToPairs(
  data?: Record<string, string> | null,
): KVPair[] {
  return Object.entries(data || {}).map(([key, value], id) => ({
    id,
    key,
    value,
  }))
}

export function pairsToRecord(pairs: KVPair[]): Record<string, string> {
  const out: Record<string, string> = {}
  for (const pair of pairs) {
    const key = pair.key.trim()
    if (!key) continue
    out[key] = pair.value
  }
  return out
}

type ValueEditorLanguage = "json" | "yaml" | "plain"

function tryPrettyJson(raw: string): string {
  const trimmed = raw.trim()
  if (!trimmed) return raw
  try {
    return JSON.stringify(JSON.parse(trimmed), null, 2)
  } catch {
    return raw
  }
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
    if (/^[\w./@-]+:(\s|$)/.test(t) || /^-(\s|$)/.test(t) || /:\s*[|>][-+]?(\s|$)/.test(t)) {
      hits++
    }
  }

  if (lines.length === 1) {
    // Single-line "key: value" is common plain annotation text; require structure.
    return /:\s*[|>]/.test(trimmed) || (hits === 1 && /\n\s+\S/.test(raw))
  }
  return hits >= 2 && hits / lines.length >= 0.4
}

/** Pick editor mode from the current value text. Empty → plain. */
function detectValueLanguage(raw: string): ValueEditorLanguage {
  const trimmed = raw.trim()
  if (!trimmed) return "plain"

  if (trimmed.startsWith("{") || trimmed.startsWith("[")) return "json"
  if (looksLikeYaml(trimmed)) return "yaml"
  return "plain"
}

function monacoLanguage(lang: ValueEditorLanguage): "json" | "yaml" | "plaintext" {
  if (lang === "json") return "json"
  if (lang === "yaml") return "yaml"
  return "plaintext"
}

function ValueEditorDialog({
  open,
  onOpenChange,
  annotationKey,
  value,
  onApply,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  annotationKey: string
  value: string
  onApply: (next: string) => void
}) {
  const [draft, setDraft] = useState(value)
  const [language, setLanguage] = useState<ValueEditorLanguage>("plain")
  const [prevOpen, setPrevOpen] = useState(open)

  if (open !== prevOpen) {
    setPrevOpen(open)
    if (open) {
      const detected = detectValueLanguage(value)
      setLanguage(detected)
      setDraft(detected === "json" ? tryPrettyJson(value) : value)
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="flex max-h-[min(90vh,48rem)] max-w-[calc(100%-2rem)] flex-col gap-4 overflow-hidden sm:max-w-3xl">
        <DialogHeader>
          <DialogTitle>Edit value</DialogTitle>
          <DialogDescription>
            {annotationKey.trim() ? (
              <>
                Editing{" "}
                <span className="font-mono text-foreground">
                  {annotationKey}
                </span>
              </>
            ) : (
              "Edit the annotation or label value."
            )}
          </DialogDescription>
        </DialogHeader>
        <div
          className={cn(
            "w-full overflow-hidden rounded-xl border border-input",
            "focus-within:border-ring focus-within:ring-[3px] focus-within:ring-ring/50",
          )}
        >
          {open ? (
            <MonacoCodeEditor
              key={language}
              value={draft}
              onChange={setDraft}
              language={monacoLanguage(language)}
              autoHeight
              minLines={4}
              maxLines={22}
            />
          ) : null}
        </div>
        <DialogFooter>
          <Button
            type="button"
            variant="outline"
            onClick={() => onOpenChange(false)}
          >
            Cancel
          </Button>
          <Button
            type="button"
            onClick={() => {
              onApply(draft)
              onOpenChange(false)
            }}
          >
            Apply
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

function KeyValueEditor({
  title,
  description,
  value,
  onChange,
  resetKey,
  keyPlaceholder = "key",
  valuePlaceholder = "value",
}: {
  title: string
  description: string
  value: Record<string, string>
  onChange: (next: Record<string, string>) => void
  /** Remount/sync when the underlying resource changes. */
  resetKey?: string
  keyPlaceholder?: string
  valuePlaceholder?: string
}) {
  const [pairs, setPairs] = useState<KVPair[]>(() => recordToPairs(value))
  const [seenKey, setSeenKey] = useState(resetKey)
  const [editingId, setEditingId] = useState<number | null>(null)
  if (resetKey !== seenKey) {
    setSeenKey(resetKey)
    setPairs(recordToPairs(value))
  }

  const commit = (next: KVPair[]) => {
    setPairs(next)
    onChange(pairsToRecord(next))
  }

  const editing = pairs.find((p) => p.id === editingId) ?? null

  return (
    <section className="space-y-3">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div>
          <h2 className="text-sm font-medium">{title}</h2>
          <p className="text-xs text-muted-foreground">{description}</p>
        </div>
        <Button
          type="button"
          size="sm"
          variant="outline"
          onClick={() =>
            commit([...pairs, { id: Date.now(), key: "", value: "" }])
          }
        >
          <Plus className="size-3.5" />
          Add
        </Button>
      </div>
      {!pairs.length ? (
        <div className="rounded-xl border border-dashed px-4 py-6 text-center text-sm text-muted-foreground">
          None yet. Add a key to get started.
        </div>
      ) : (
        <div className="space-y-2">
          {pairs.map((pair) => (
            <div
              key={pair.id}
              className="grid gap-2 rounded-lg border bg-muted/15 p-2 sm:grid-cols-[minmax(0,1fr)_minmax(0,1.4fr)_auto]"
            >
              <Input
                value={pair.key}
                onChange={(e) =>
                  commit(
                    pairs.map((p) =>
                      p.id === pair.id ? { ...p, key: e.target.value } : p,
                    ),
                  )
                }
                placeholder={keyPlaceholder}
                className="font-mono text-xs"
              />
              <div className="flex min-w-0 items-center gap-1">
                <Input
                  value={pair.value}
                  onChange={(e) =>
                    commit(
                      pairs.map((p) =>
                        p.id === pair.id ? { ...p, value: e.target.value } : p,
                      ),
                    )
                  }
                  placeholder={valuePlaceholder}
                  className="min-w-0 flex-1 font-mono text-xs"
                />
                <Button
                  type="button"
                  size="icon-sm"
                  variant="ghost"
                  title="Open in editor"
                  onClick={() => setEditingId(pair.id)}
                >
                  <Maximize2 className="size-3.5" />
                </Button>
              </div>
              <Button
                type="button"
                size="icon-sm"
                variant="ghost"
                className="text-destructive hover:bg-destructive/10 hover:text-destructive"
                onClick={() => commit(pairs.filter((p) => p.id !== pair.id))}
              >
                <Trash2 className="size-3.5" />
              </Button>
            </div>
          ))}
        </div>
      )}

      <ValueEditorDialog
        open={editingId != null}
        onOpenChange={(open) => {
          if (!open) setEditingId(null)
        }}
        annotationKey={editing?.key ?? ""}
        value={editing?.value ?? ""}
        onApply={(next) => {
          if (editingId == null) return
          commit(
            pairs.map((p) =>
              p.id === editingId ? { ...p, value: next } : p,
            ),
          )
        }}
      />
    </section>
  )
}

export function IngressAdvancedEditor({
  namespace,
  ingressClass,
  onIngressClassChange,
  rules,
  onRulesChange,
  tls,
  onTlsChange,
  labels,
  onLabelsChange,
  annotations,
  onAnnotationsChange,
  metadataResetKey,
  compact,
}: IngressEditorProps) {
  const optionsQuery = useIngressFormOptions(namespace)
  const options = optionsQuery.data?.data
  const services = options?.services ?? []

  const knownHosts = useMemo(() => {
    const set = new Set<string>()
    for (const rule of rules) {
      const h = rule.host.trim()
      if (h) set.add(h)
    }
    for (const entry of tls) {
      for (const h of entry.hosts) {
        if (h.trim()) set.add(h.trim())
      }
    }
    return Array.from(set)
      .sort((a, b) => a.localeCompare(b))
      .map((value) => ({ label: value, value }))
  }, [rules, tls])

  const updateRule = (index: number, next: IngressRule) => {
    onRulesChange(rules.map((r, i) => (i === index ? next : r)))
  }

  const updatePath = (
    ruleIndex: number,
    pathIndex: number,
    next: IngressPath,
  ) => {
    onRulesChange(
      rules.map((rule, i) => {
        if (i !== ruleIndex) return rule
        return {
          ...rule,
          paths: rule.paths.map((p, j) => (j === pathIndex ? next : p)),
        }
      }),
    )
  }

  return (
    <div className={compact ? "space-y-4" : "space-y-6"}>
      <section className="space-y-3 rounded-xl border bg-card p-4 shadow-xs">
        <div className="flex items-start gap-3">
          <div className="mt-0.5 flex size-9 shrink-0 items-center justify-center rounded-lg bg-muted">
            <Route className="size-4 text-muted-foreground" />
          </div>
          <div className="min-w-0 flex-1 space-y-3">
            <div>
              <h2 className="text-sm font-medium">Controller</h2>
              <p className="text-xs text-muted-foreground">
                Which ingress controller should handle this resource.
              </p>
            </div>
            <IngressClassField
              value={ingressClass}
              onChange={onIngressClassChange}
              options={options}
              isLoading={optionsQuery.isLoading}
            />
          </div>
        </div>
      </section>

      <section className="space-y-3">
        <div className="flex flex-wrap items-center justify-between gap-2">
          <div>
            <h2 className="text-sm font-medium">HTTP rules</h2>
            <p className="text-xs text-muted-foreground">
              Host + path routing to Services in{" "}
              <span className="font-mono">{namespace}</span>.
            </p>
          </div>
          <Button
            type="button"
            size="sm"
            variant="outline"
            onClick={() => onRulesChange([...rules, emptyIngressRule()])}
          >
            <Plus className="size-3.5" />
            Add rule
          </Button>
        </div>

        <div className="space-y-4">
          {rules.map((rule, ruleIndex) => (
            <div
              key={ruleIndex}
              className="overflow-hidden rounded-xl border bg-card shadow-xs"
            >
              <div className="flex flex-wrap items-center gap-3 border-b bg-muted/40 px-4 py-3">
                <Badge variant="secondary" className="font-mono text-[10px]">
                  Rule {ruleIndex + 1}
                </Badge>
                <div className="min-w-[14rem] flex-1">
                  <ReactSelectCreatable<SelectOption, false>
                    size="sm"
                    isClearable
                    isSearchable
                    options={ensureOption(knownHosts, rule.host)}
                    value={rule.host || null}
                    onValueChange={(v) =>
                      updateRule(ruleIndex, { ...rule, host: v ?? "" })
                    }
                    placeholder="Host (optional) — e.g. app.example.com"
                    formatCreateLabel={(input) => `Use host “${input}”`}
                  />
                </div>
                <Button
                  type="button"
                  size="icon-sm"
                  variant="ghost"
                  disabled={rules.length <= 1}
                  className="text-destructive hover:bg-destructive/10 hover:text-destructive"
                  onClick={() =>
                    onRulesChange(rules.filter((_, i) => i !== ruleIndex))
                  }
                >
                  <Trash2 className="size-3.5" />
                </Button>
              </div>

              <div className="space-y-3 p-4">
                <div className="flex items-center justify-between gap-2">
                  <div className="flex items-center gap-2 text-xs text-muted-foreground">
                    <Globe className="size-3.5" />
                    Paths
                  </div>
                  <Button
                    type="button"
                    size="sm"
                    variant="outline"
                    onClick={() =>
                      updateRule(ruleIndex, {
                        ...rule,
                        paths: [...rule.paths, emptyIngressPath()],
                      })
                    }
                  >
                    <Plus className="size-3.5" />
                    Path
                  </Button>
                </div>

                {rule.paths.map((path, pathIndex) => {
                  const ports = ensureOption(
                    portOptionsForService(services, path.service_name),
                    path.service_port > 0 ? String(path.service_port) : "",
                  )
                  const svcOpts = ensureOption(
                    serviceOptions(services),
                    path.service_name,
                  )
                  return (
                    <div
                      key={pathIndex}
                      className="grid gap-3 rounded-lg border bg-muted/15 p-3 lg:grid-cols-[minmax(0,1fr)_140px_minmax(0,1.2fr)_minmax(0,0.8fr)_auto]"
                    >
                      <div className="space-y-1.5">
                        <Label className="text-xs">Path</Label>
                        <Input
                          value={path.path}
                          onChange={(e) =>
                            updatePath(ruleIndex, pathIndex, {
                              ...path,
                              path: e.target.value,
                            })
                          }
                          placeholder="/"
                          className="font-mono text-xs"
                        />
                      </div>
                      <div className="space-y-1.5">
                        <Label className="text-xs">Path type</Label>
                        <ReactSelect<SelectOption, false>
                          size="sm"
                          options={PATH_TYPE_OPTIONS}
                          value={path.path_type || "Prefix"}
                          onValueChange={(v) =>
                            updatePath(ruleIndex, pathIndex, {
                              ...path,
                              path_type: v || "Prefix",
                            })
                          }
                          isSearchable={false}
                        />
                      </div>
                      <div className="space-y-1.5">
                        <Label className="text-xs">Service</Label>
                        <ReactSelectCreatable<SelectOption, false>
                          size="sm"
                          isClearable
                          isSearchable
                          isLoading={optionsQuery.isLoading}
                          options={svcOpts}
                          value={path.service_name || null}
                          onValueChange={(v) => {
                            const name = v ?? ""
                            const nextPorts = portOptionsForService(
                              services,
                              name,
                            )
                            const first = nextPorts[0]
                            updatePath(ruleIndex, pathIndex, {
                              ...path,
                              service_name: name,
                              service_port: first
                                ? Number(first.value)
                                : path.service_port || 80,
                              service_port_name: undefined,
                            })
                          }}
                          placeholder="Select or type service…"
                          formatCreateLabel={(input) => `Use service “${input}”`}
                          noOptionsMessage={() =>
                            "No services in namespace — type a name"
                          }
                        />
                      </div>
                      <div className="space-y-1.5">
                        <Label className="text-xs">Port</Label>
                        <ReactSelectCreatable<SelectOption, false>
                          size="sm"
                          isClearable
                          isSearchable
                          options={ports}
                          value={
                            path.service_port > 0
                              ? String(path.service_port)
                              : null
                          }
                          onValueChange={(v) =>
                            updatePath(ruleIndex, pathIndex, {
                              ...path,
                              service_port: Number(v) || 0,
                              service_port_name: undefined,
                            })
                          }
                          placeholder={
                            ports.length ? "Select port…" : "Port number…"
                          }
                          formatCreateLabel={(input) => `Use port ${input}`}
                          noOptionsMessage={() => "Type a port number"}
                        />
                      </div>
                      <div className="flex items-end">
                        <Button
                          type="button"
                          size="icon-sm"
                          variant="ghost"
                          disabled={rule.paths.length <= 1}
                          className="text-destructive hover:bg-destructive/10 hover:text-destructive"
                          onClick={() =>
                            updateRule(ruleIndex, {
                              ...rule,
                              paths: rule.paths.filter(
                                (_, i) => i !== pathIndex,
                              ),
                            })
                          }
                        >
                          <Trash2 className="size-3.5" />
                        </Button>
                      </div>
                    </div>
                  )
                })}
              </div>
            </div>
          ))}
        </div>
      </section>

      <section className="space-y-3">
        <div className="flex flex-wrap items-center justify-between gap-2">
          <div>
            <h2 className="text-sm font-medium">TLS certificates</h2>
            <p className="text-xs text-muted-foreground">
              Use <span className="font-medium">one</span> block when a single
              secret covers all hosts, or <span className="font-medium">many</span>{" "}
              blocks for different secrets/host groups. Secrets must be type{" "}
              <span className="font-mono">kubernetes.io/tls</span>.
            </p>
          </div>
          <Button
            type="button"
            size="sm"
            variant="outline"
            onClick={() => onTlsChange([...tls, emptyIngressTLS()])}
          >
            <Plus className="size-3.5" />
            Add certificate
          </Button>
        </div>

        {!tls.length ? (
          <div className="rounded-xl border border-dashed px-4 py-8 text-center text-sm text-muted-foreground">
            No TLS certificates. Traffic stays HTTP unless the controller adds
            defaults. Click “Add certificate” for HTTPS.
          </div>
        ) : (
          <div className="space-y-3">
            {tls.map((entry, index) => (
              <div
                key={index}
                className="overflow-hidden rounded-xl border bg-card shadow-xs"
              >
                <div className="flex items-center gap-2 border-b bg-muted/40 px-4 py-2.5">
                  <Lock className="size-3.5 text-muted-foreground" />
                  <span className="text-xs font-medium">
                    Certificate {index + 1}
                    {tls.length > 1 ? ` of ${tls.length}` : ""}
                  </span>
                  <Badge variant="outline" className="text-[10px] font-normal">
                    {entry.secret_name || "unset"}
                  </Badge>
                  <div className="flex-1" />
                  <Button
                    type="button"
                    size="icon-sm"
                    variant="ghost"
                    className="text-destructive hover:bg-destructive/10 hover:text-destructive"
                    onClick={() =>
                      onTlsChange(tls.filter((_, i) => i !== index))
                    }
                  >
                    <Trash2 className="size-3.5" />
                  </Button>
                </div>
                <div className="grid gap-3 p-4 sm:grid-cols-2">
                  <div className="space-y-1.5">
                    <Label className="text-xs">TLS secret</Label>
                    <ReactSelectCreatable<SelectOption, false>
                      size="sm"
                      isClearable
                      isSearchable
                      isLoading={optionsQuery.isLoading}
                      options={ensureOption(
                        tlsSecretOptions(options),
                        entry.secret_name,
                      )}
                      value={entry.secret_name || null}
                      onValueChange={(v) =>
                        onTlsChange(
                          tls.map((t, i) =>
                            i === index
                              ? { ...t, secret_name: v ?? "" }
                              : t,
                          ),
                        )
                      }
                      placeholder="Select TLS secret…"
                      formatCreateLabel={(input) => `Use secret “${input}”`}
                      noOptionsMessage={() =>
                        "No TLS secrets — type a secret name"
                      }
                    />
                  </div>
                  <div className="space-y-1.5">
                    <Label className="text-xs">Hosts covered by this cert</Label>
                    <ReactSelectCreatable<SelectOption, true>
                      size="sm"
                      isMulti
                      isClearable
                      isSearchable
                      options={ensureOptions(knownHosts, entry.hosts)}
                      value={entry.hosts}
                      onValueChange={(values) =>
                        onTlsChange(
                          tls.map((t, i) =>
                            i === index ? { ...t, hosts: [...values] } : t,
                          ),
                        )
                      }
                      placeholder="Select or type hosts…"
                      formatCreateLabel={(input) => `Add host “${input}”`}
                      closeMenuOnSelect={false}
                    />
                  </div>
                </div>
                <Separator />
                <p className="px-4 py-2 text-[11px] text-muted-foreground">
                  Tip: leave hosts empty to use the secret’s CN/SAN, or list the
                  hosts from your HTTP rules that this certificate should serve.
                </p>
              </div>
            ))}
          </div>
        )}
      </section>

      <section className="space-y-4 rounded-xl border bg-card p-4 shadow-xs">
        <div className="flex items-start gap-3">
          <div className="mt-0.5 flex size-9 shrink-0 items-center justify-center rounded-lg bg-muted">
            <Tags className="size-4 text-muted-foreground" />
          </div>
          <div className="min-w-0 flex-1 space-y-4">
            <div>
              <h2 className="text-sm font-medium">Metadata</h2>
              <p className="text-xs text-muted-foreground">
                Labels for selection/organization; annotations for controller
                settings (cert-manager, redirects, etc.).
              </p>
            </div>
            <KeyValueEditor
              title="Labels"
              description="Key/value pairs stored on the Ingress object."
              value={labels}
              onChange={onLabelsChange}
              resetKey={metadataResetKey ? `${metadataResetKey}:labels` : undefined}
              keyPlaceholder="app"
              valuePlaceholder="frontend"
            />
            <KeyValueEditor
              title="Annotations"
              description="Controller-specific options, e.g. cert-manager.io/cluster-issuer."
              value={annotations}
              onChange={onAnnotationsChange}
              resetKey={
                metadataResetKey ? `${metadataResetKey}:annotations` : undefined
              }
              keyPlaceholder="cert-manager.io/cluster-issuer"
              valuePlaceholder="letsencrypt"
            />
          </div>
        </div>
      </section>
    </div>
  )
}
