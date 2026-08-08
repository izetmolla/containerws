import { useState } from "react"
import { useOutletContext } from "react-router"
import { TerminalSquare } from "lucide-react"

import { Button } from "@/components/ui/button"
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { Switch } from "@/components/ui/switch"

import type { PodOutletContext } from "./layout"
import { PodExecTerminal } from "./exec-terminal"

const SHELL_PRESETS = [
  { value: "/bin/bash", label: "bash" },
  { value: "/bin/sh", label: "sh" },
  { value: "/bin/ash", label: "ash" },
  { value: "/bin/zsh", label: "zsh" },
] as const

function commandDisplayName(command: string) {
  const trimmed = command.trim()
  const base = trimmed.split(/\s+/)[0] || trimmed
  const slash = base.lastIndexOf("/")
  return slash >= 0 ? base.slice(slash + 1) : base
}

export default function PodExecPage() {
  const { namespace, name, pod } = useOutletContext<PodOutletContext>()
  const defaultContainer = pod.containers[0]?.name || ""

  const [container, setContainer] = useState(defaultContainer)
  const [preset, setPreset] = useState<string>("/bin/sh")
  const [customEnabled, setCustomEnabled] = useState(false)
  const [customCommand, setCustomCommand] = useState("")
  const [session, setSession] = useState<{
    container: string
    command: string
  } | null>(null)

  const running = pod.status === "Running"
  const activeCommand = session?.command ?? ""
  const commandLabel = commandDisplayName(activeCommand) || "—"

  const connect = () => {
    const command = customEnabled ? customCommand.trim() : preset.trim()
    const ctr = container.trim() || defaultContainer
    if (!command || !ctr) return
    setSession({ container: ctr, command })
  }

  if (!running) {
    return (
      <p className="text-sm text-muted-foreground">
        Pod must be Running before opening an exec session.
      </p>
    )
  }

  if (!pod.containers.length) {
    return (
      <p className="text-sm text-muted-foreground">
        This pod has no containers to exec into.
      </p>
    )
  }

  return (
    <div className="flex w-full min-w-0 flex-col gap-6">
      <Card>
        <CardHeader className="flex flex-row flex-wrap items-center justify-between gap-3 space-y-0 pb-3">
          <CardTitle className="flex items-center gap-2 text-base">
            <TerminalSquare className="size-4 text-muted-foreground" />
            Exec
          </CardTitle>
          {session ? (
            <Button variant="outline" onClick={() => setSession(null)}>
              Disconnect
            </Button>
          ) : null}
        </CardHeader>
        <CardContent className="space-y-5">
          {session ? (
            <p className="text-sm text-muted-foreground">
              Connected to{" "}
              <strong className="text-foreground">{session.container}</strong>{" "}
              using{" "}
              <strong className="text-foreground">{commandLabel}</strong>
            </p>
          ) : (
            <>
              <div className="grid gap-4 sm:grid-cols-2">
                <div className="space-y-2">
                  <Label>Container</Label>
                  <Select
                    value={container || defaultContainer}
                    onValueChange={(v) => {
                      if (v) setContainer(v)
                    }}
                  >
                    <SelectTrigger className="w-full">
                      <SelectValue placeholder="Container" />
                    </SelectTrigger>
                    <SelectContent>
                      {pod.containers.map((c) => (
                        <SelectItem key={c.name} value={c.name}>
                          {c.name}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>

                <div className="space-y-2">
                  <Label htmlFor="pod-exec-command">Command</Label>
                  {customEnabled ? (
                    <Input
                      id="pod-exec-command"
                      value={customCommand}
                      onChange={(e) => setCustomCommand(e.target.value)}
                      placeholder="/bin/sh"
                      autoComplete="off"
                    />
                  ) : (
                    <Select
                      value={preset}
                      onValueChange={(v) => {
                        if (v) setPreset(v)
                      }}
                    >
                      <SelectTrigger id="pod-exec-command" className="w-full">
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        {SHELL_PRESETS.map((o) => (
                          <SelectItem key={o.value} value={o.value}>
                            {o.label}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  )}
                </div>
              </div>

              <div className="flex items-center gap-2">
                <Switch
                  id="pod-custom-command"
                  checked={customEnabled}
                  onCheckedChange={(checked) => {
                    setCustomEnabled(checked)
                    if (checked && !customCommand) {
                      setCustomCommand(commandDisplayName(preset))
                    }
                  }}
                />
                <Label htmlFor="pod-custom-command" className="font-normal">
                  Use custom command
                </Label>
              </div>

              <Button
                onClick={connect}
                disabled={customEnabled && !customCommand.trim()}
              >
                Connect
              </Button>
            </>
          )}
        </CardContent>
      </Card>

      {session ? (
        <div className="min-h-[420px] flex-1 overflow-hidden rounded-xl border bg-zinc-950">
          <PodExecTerminal
            key={`${session.container}|${session.command}`}
            namespace={namespace}
            name={name}
            container={session.container}
            command={session.command}
            className="min-h-[420px] rounded-xl"
          />
        </div>
      ) : null}
    </div>
  )
}
