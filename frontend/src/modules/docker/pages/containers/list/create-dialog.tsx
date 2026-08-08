import { useState } from "react"
import { toast } from "sonner"

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
import { cn } from "@/lib/utils"

import type { CreateContainerInput } from "./api"

type Option = { value: string; label: string }

const RESTART_OPTIONS: Option[] = [
  { value: "no", label: "No" },
  { value: "always", label: "Always" },
  { value: "unless-stopped", label: "Unless stopped" },
  { value: "on-failure", label: "On failure" },
]

const TABS = [
  { id: "simple", label: "Simple" },
  { id: "volumes", label: "Volumes" },
  { id: "network", label: "Network" },
  { id: "runtime", label: "Runtime" },
  { id: "resources", label: "Resources" },
  { id: "advanced", label: "Advanced" },
] as const

type TabId = (typeof TABS)[number]["id"]

type Props = {
  open: boolean
  pending?: boolean
  networkOptions?: Option[]
  onClose: () => void
  onSubmit: (body: CreateContainerInput) => void
}

function splitLines(text: string) {
  return text
    .split("\n")
    .map((s) => s.trim())
    .filter(Boolean)
}

function parseLabels(text: string): Record<string, string> {
  const out: Record<string, string> = {}
  for (const line of splitLines(text)) {
    const i = line.indexOf("=")
    if (i <= 0) continue
    out[line.slice(0, i).trim()] = line.slice(i + 1).trim()
  }
  return out
}

export function CreateContainerDialog({
  open,
  pending,
  networkOptions = [],
  onClose,
  onSubmit,
}: Props) {
  const [tab, setTab] = useState<TabId>("simple")
  const [image, setImage] = useState("nginx:alpine")
  const [name, setName] = useState("")
  const [ports, setPorts] = useState("8080:80")
  const [env, setEnv] = useState("")
  const [restart, setRestart] = useState("unless-stopped")
  const [startAfter, setStartAfter] = useState(true)
  const [pullIfMissing, setPullIfMissing] = useState(true)
  const [binds, setBinds] = useState("")
  const [networks, setNetworks] = useState<string[]>([])
  const [cmd, setCmd] = useState("")
  const [entrypoint, setEntrypoint] = useState("")
  const [workdir, setWorkdir] = useState("")
  const [user, setUser] = useState("")
  const [hostname, setHostname] = useState("")
  const [privileged, setPrivileged] = useState(false)
  const [readonlyRoot, setReadonlyRoot] = useState(false)
  const [autoRemove, setAutoRemove] = useState(false)
  const [memory, setMemory] = useState("")
  const [cpus, setCpus] = useState("")
  const [labels, setLabels] = useState("")
  const [extraHosts, setExtraHosts] = useState("")
  const [devices, setDevices] = useState("")
  const [capAdd, setCapAdd] = useState("")
  const [capDrop, setCapDrop] = useState("")
  const [dns, setDns] = useState("")

  const submit = () => {
    const img = image.trim()
    if (!img) {
      toast.message("Image is required")
      return
    }
    const body: CreateContainerInput = {
      image: img,
      name: name.trim() || undefined,
      ports: splitLines(ports.replace(/,/g, "\n")),
      env: splitLines(env),
      restart_policy: restart,
      start: startAfter,
      pull_if_missing: pullIfMissing,
      binds: splitLines(binds),
      networks: networks.length ? networks : undefined,
      cmd: cmd.trim() ? cmd.trim().split(/\s+/) : undefined,
      entrypoint: entrypoint.trim()
        ? entrypoint.trim().split(/\s+/)
        : undefined,
      working_dir: workdir.trim() || undefined,
      user: user.trim() || undefined,
      hostname: hostname.trim() || undefined,
      privileged,
      readonly_rootfs: readonlyRoot,
      auto_remove: autoRemove,
      memory: memory.trim() || undefined,
      cpus: cpus.trim() ? Number(cpus) : undefined,
      labels: parseLabels(labels),
      extra_hosts: splitLines(extraHosts),
      devices: splitLines(devices),
      cap_add: splitLines(capAdd.replace(/,/g, "\n")),
      cap_drop: splitLines(capDrop.replace(/,/g, "\n")),
      dns: splitLines(dns.replace(/,/g, "\n")),
    }
    onSubmit(body)
  }

  return (
    <Dialog open={open} onOpenChange={(v) => !v && onClose()}>
      <DialogContent className="flex max-h-[90vh] flex-col gap-0 overflow-hidden p-0 sm:max-w-2xl">
        <DialogHeader className="border-b px-6 py-4">
          <DialogTitle>Add container</DialogTitle>
          <DialogDescription>
            Create a container on the local Docker Engine. Fill Simple first;
            open other tabs for volumes, networks, and advanced options.
          </DialogDescription>
        </DialogHeader>

        <div className="flex gap-1 overflow-x-auto border-b px-4 pt-2">
          {TABS.map((t) => (
            <button
              key={t.id}
              type="button"
              className={cn(
                "shrink-0 rounded-t-md px-3 py-2 text-sm font-medium transition-colors",
                tab === t.id
                  ? "bg-muted text-foreground"
                  : "text-muted-foreground hover:text-foreground"
              )}
              onClick={() => setTab(t.id)}
            >
              {t.label}
            </button>
          ))}
        </div>

        <div className="min-h-0 flex-1 overflow-y-auto px-6 py-4">
          {tab === "simple" ? (
            <div className="grid gap-4">
              <Field label="Image *" hint="e.g. nginx:alpine, redis:7">
                <Input
                  value={image}
                  onChange={(e) => setImage(e.target.value)}
                  placeholder="nginx:alpine"
                />
              </Field>
              <Field label="Name" hint="Optional container name">
                <Input
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  placeholder="my-nginx"
                />
              </Field>
              <Field
                label="Published ports"
                hint="One per line: host:container[/tcp|udp]"
              >
                <textarea
                  className="min-h-20 w-full rounded-md border bg-background px-3 py-2 font-mono text-sm"
                  value={ports}
                  onChange={(e) => setPorts(e.target.value)}
                  placeholder={"8080:80\n8443:443"}
                />
              </Field>
              <Field label="Environment" hint="KEY=value per line">
                <textarea
                  className="min-h-20 w-full rounded-md border bg-background px-3 py-2 font-mono text-sm"
                  value={env}
                  onChange={(e) => setEnv(e.target.value)}
                  placeholder="FOO=bar"
                />
              </Field>
              <Field label="Restart policy">
                <ReactSelect<Option, false>
                  size="sm"
                  options={RESTART_OPTIONS}
                  value={restart}
                  onValueChange={(v) => v && setRestart(v)}
                />
              </Field>
              <label className="flex items-center gap-2 text-sm">
                <input
                  type="checkbox"
                  className="size-4 accent-foreground"
                  checked={startAfter}
                  onChange={(e) => setStartAfter(e.target.checked)}
                />
                Start after create
              </label>
              <label className="flex items-center gap-2 text-sm">
                <input
                  type="checkbox"
                  className="size-4 accent-foreground"
                  checked={pullIfMissing}
                  onChange={(e) => setPullIfMissing(e.target.checked)}
                />
                Pull image if missing
              </label>
            </div>
          ) : null}

          {tab === "volumes" ? (
            <Field
              label="Bind mounts"
              hint="host:container[:ro|rw] — one per line"
            >
              <textarea
                className="min-h-40 w-full rounded-md border bg-background px-3 py-2 font-mono text-sm"
                value={binds}
                onChange={(e) => setBinds(e.target.value)}
                placeholder="/data:/app/data:rw"
              />
            </Field>
          ) : null}

          {tab === "network" ? (
            <Field label="Networks" hint="Attach to one or more networks">
              <ReactSelect<Option, true>
                size="sm"
                isMulti
                options={networkOptions}
                value={networks}
                onValueChange={(vals) => setNetworks([...(vals ?? [])])}
                placeholder="Select networks"
              />
            </Field>
          ) : null}

          {tab === "runtime" ? (
            <div className="grid gap-4">
              <Field label="Command" hint="Space-separated">
                <Input value={cmd} onChange={(e) => setCmd(e.target.value)} />
              </Field>
              <Field label="Entrypoint" hint="Space-separated">
                <Input
                  value={entrypoint}
                  onChange={(e) => setEntrypoint(e.target.value)}
                />
              </Field>
              <div className="grid gap-4 sm:grid-cols-2">
                <Field label="Working dir">
                  <Input
                    value={workdir}
                    onChange={(e) => setWorkdir(e.target.value)}
                  />
                </Field>
                <Field label="User">
                  <Input
                    value={user}
                    onChange={(e) => setUser(e.target.value)}
                  />
                </Field>
              </div>
              <Field label="Hostname">
                <Input
                  value={hostname}
                  onChange={(e) => setHostname(e.target.value)}
                />
              </Field>
              <label className="flex items-center gap-2 text-sm">
                <input
                  type="checkbox"
                  className="size-4 accent-foreground"
                  checked={privileged}
                  onChange={(e) => setPrivileged(e.target.checked)}
                />
                Privileged
              </label>
              <label className="flex items-center gap-2 text-sm">
                <input
                  type="checkbox"
                  className="size-4 accent-foreground"
                  checked={readonlyRoot}
                  onChange={(e) => setReadonlyRoot(e.target.checked)}
                />
                Read-only root filesystem
              </label>
              <label className="flex items-center gap-2 text-sm">
                <input
                  type="checkbox"
                  className="size-4 accent-foreground"
                  checked={autoRemove}
                  onChange={(e) => setAutoRemove(e.target.checked)}
                />
                Auto-remove on exit
              </label>
            </div>
          ) : null}

          {tab === "resources" ? (
            <div className="grid gap-4 sm:grid-cols-2">
              <Field label="Memory" hint="e.g. 512m, 1g">
                <Input
                  value={memory}
                  onChange={(e) => setMemory(e.target.value)}
                  placeholder="512m"
                />
              </Field>
              <Field label="CPUs" hint="e.g. 0.5, 2">
                <Input
                  value={cpus}
                  onChange={(e) => setCpus(e.target.value)}
                  placeholder="1"
                  type="number"
                  step="0.1"
                  min="0"
                />
              </Field>
            </div>
          ) : null}

          {tab === "advanced" ? (
            <div className="grid gap-4">
              <Field label="Labels" hint="key=value per line">
                <textarea
                  className="min-h-20 w-full rounded-md border bg-background px-3 py-2 font-mono text-sm"
                  value={labels}
                  onChange={(e) => setLabels(e.target.value)}
                />
              </Field>
              <Field label="Extra hosts" hint="hostname:ip per line">
                <textarea
                  className="min-h-16 w-full rounded-md border bg-background px-3 py-2 font-mono text-sm"
                  value={extraHosts}
                  onChange={(e) => setExtraHosts(e.target.value)}
                />
              </Field>
              <Field label="Devices" hint="host:container[:perms]">
                <textarea
                  className="min-h-16 w-full rounded-md border bg-background px-3 py-2 font-mono text-sm"
                  value={devices}
                  onChange={(e) => setDevices(e.target.value)}
                />
              </Field>
              <div className="grid gap-4 sm:grid-cols-2">
                <Field label="Cap add">
                  <Input
                    value={capAdd}
                    onChange={(e) => setCapAdd(e.target.value)}
                    placeholder="NET_ADMIN"
                  />
                </Field>
                <Field label="Cap drop">
                  <Input
                    value={capDrop}
                    onChange={(e) => setCapDrop(e.target.value)}
                  />
                </Field>
              </div>
              <Field label="DNS">
                <Input
                  value={dns}
                  onChange={(e) => setDns(e.target.value)}
                  placeholder="8.8.8.8"
                />
              </Field>
            </div>
          ) : null}
        </div>

        <DialogFooter className="border-t px-6 py-4">
          <Button variant="outline" onClick={onClose} disabled={pending}>
            Cancel
          </Button>
          <Button onClick={submit} disabled={pending}>
            {pending ? "Creating…" : "Create container"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

function Field({
  label,
  hint,
  children,
}: {
  label: string
  hint?: string
  children: React.ReactNode
}) {
  return (
    <div className="grid gap-1.5">
      <Label className="text-sm">{label}</Label>
      {children}
      {hint ? <p className="text-xs text-muted-foreground">{hint}</p> : null}
    </div>
  )
}
