import { useEffect, useState } from "react"
import { useOutletContext } from "react-router"
import { CircleHelp, TerminalSquare } from "lucide-react"

import { useContentLoader } from "@/components/content-loader/context"
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
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip"

import type { ContainerOutletContext } from "./layout"
import { ExecTerminal } from "./exec-terminal"

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

export default function ContainerConsolePage() {
  const { id, container } = useOutletContext<ContainerOutletContext>()
  const { setTitle } = useContentLoader()

  const [preset, setPreset] = useState<string>("/bin/bash")
  const [customEnabled, setCustomEnabled] = useState(false)
  const [customCommand, setCustomCommand] = useState("")
  const [user, setUser] = useState(container.inspect?.Config?.User || "")
  const [session, setSession] = useState<{
    command: string
    user: string
  } | null>(null)

  const running = container.state === "running"
  const activeCommand = session?.command ?? ""
  const activeUser = session?.user?.trim() ?? ""
  const userLabel = activeUser || "default user"
  const commandLabel = commandDisplayName(activeCommand) || "—"

  useEffect(() => {
    setTitle("Container console")
  }, [setTitle])

  const connect = () => {
    const command = customEnabled
      ? customCommand.trim()
      : preset.trim()
    if (!command) return
    setSession({
      command,
      user: user.trim(),
    })
  }

  const disconnect = () => {
    setSession(null)
  }

  if (!running) {
    return (
      <div className="rounded-xl border border-dashed px-6 py-12 text-center">
        <p className="text-sm font-medium">Console unavailable</p>
        <p className="mt-1 text-sm text-muted-foreground">
          Start container <span className="font-medium">{container.name}</span>{" "}
          before opening an exec session.
        </p>
      </div>
    )
  }

  return (
    <div className="flex w-full min-w-0 flex-col gap-6">
      <Card>
        <CardHeader className="flex flex-row flex-wrap items-center justify-between gap-3 space-y-0 pb-3">
          <CardTitle className="flex items-center gap-2 text-base">
            <TerminalSquare className="size-4 text-muted-foreground" />
            Execute
          </CardTitle>
          {session ? (
            <Button variant="outline" onClick={disconnect}>
              Disconnect
            </Button>
          ) : null}
        </CardHeader>
        <CardContent className="space-y-5">
          {session ? (
            <p className="text-sm text-muted-foreground">
              Exec into container as{" "}
              <strong className="text-foreground">{userLabel}</strong> using
              command{" "}
              <strong className="text-foreground">{commandLabel}</strong>
            </p>
          ) : (
            <>
              <div className="grid gap-4 sm:grid-cols-2">
                <div className="space-y-2">
                  <Label htmlFor="console-command">Command</Label>
                  {customEnabled ? (
                    <Input
                      id="console-command"
                      value={customCommand}
                      onChange={(e) => setCustomCommand(e.target.value)}
                      placeholder="bash"
                      autoComplete="off"
                    />
                  ) : (
                    <Select
                      value={preset}
                      onValueChange={(v) => {
                        if (v) setPreset(v)
                      }}
                    >
                      <SelectTrigger id="console-command" className="w-full">
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

                <div className="flex items-end gap-3 pb-1">
                  <div className="flex items-center gap-2">
                    <Switch
                      id="custom-command"
                      checked={customEnabled}
                      onCheckedChange={(checked) => {
                        setCustomEnabled(checked)
                        if (checked && !customCommand) {
                          setCustomCommand(commandDisplayName(preset))
                        }
                      }}
                    />
                    <Label htmlFor="custom-command" className="font-normal">
                      Use custom command
                    </Label>
                  </div>
                </div>
              </div>

              <div className="max-w-sm space-y-2">
                <div className="flex items-center gap-1.5">
                  <Label htmlFor="console-user">User</Label>
                  <TooltipProvider>
                    <Tooltip>
                      <TooltipTrigger asChild>
                        <button
                          type="button"
                          className="text-muted-foreground hover:text-foreground"
                          aria-label="User help"
                        >
                          <CircleHelp className="size-3.5" />
                        </button>
                      </TooltipTrigger>
                      <TooltipContent className="max-w-xs">
                        Optional. Leave empty to exec as the container&apos;s
                        default user.
                      </TooltipContent>
                    </Tooltip>
                  </TooltipProvider>
                </div>
                <Input
                  id="console-user"
                  value={user}
                  onChange={(e) => setUser(e.target.value)}
                  placeholder="default user"
                  autoComplete="off"
                />
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
          <ExecTerminal
            key={`${session.command}|${session.user || "default"}`}
            containerId={id}
            command={session.command}
            user={session.user}
            className="min-h-[420px] rounded-xl"
          />
        </div>
      ) : null}
    </div>
  )
}
