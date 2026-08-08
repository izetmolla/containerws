import { useState } from "react"
import { Link, useOutletContext } from "react-router"
import { ExternalLink, Terminal } from "lucide-react"

import { LiveTerminal } from "@/components/cloudshell/terminal/live-terminal"
import { Button } from "@/components/ui/button"
import { cn } from "@/lib/utils"

import type { UserSingleOutletContext } from "../types"

export default function UserShellPage() {
  const { user, id } = useOutletContext<UserSingleOutletContext>()
  const [status, setStatus] = useState<
    "connecting" | "connected" | "disconnected" | "error"
  >("connecting")
  const [reconnectToken, setReconnectToken] = useState(0)

  const asUser = user.username?.trim() || null
  const fullscreenHref = asUser
    ? `/shell?as_user=${encodeURIComponent(asUser)}`
    : "/shell"

  if (!asUser) {
    return (
      <section className="rounded-xl border bg-card p-8 text-center">
        <Terminal className="mx-auto size-8 text-muted-foreground" />
        <h2 className="mt-3 text-base font-semibold">No Linux username</h2>
        <p className="mt-1 text-sm text-muted-foreground">
          Set a username on the General tab before opening a shell for this
          user.
        </p>
        <Button className="mt-4" asChild>
          <Link to={`/users/${id}`}>Back to General</Link>
        </Button>
      </section>
    )
  }

  return (
    <div className="flex flex-col gap-3">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div>
          <h2 className="text-sm font-semibold">Terminal</h2>
          <p className="text-xs text-muted-foreground">
            Interactive shell as <code className="text-[11px]">{asUser}</code>
          </p>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <span
            className={cn(
              "inline-flex items-center gap-1.5 rounded-md px-2 py-1 text-[11px] font-medium capitalize",
              status === "connected"
                ? "bg-emerald-500/15 text-emerald-700 dark:text-emerald-300"
                : status === "error"
                  ? "bg-red-500/15 text-red-700 dark:text-red-300"
                  : "bg-muted text-muted-foreground"
            )}
          >
            <span
              className={cn(
                "size-1.5 rounded-full",
                status === "connected"
                  ? "bg-emerald-500"
                  : status === "error"
                    ? "bg-red-500"
                    : "bg-muted-foreground"
              )}
              aria-hidden
            />
            {status}
          </span>
          <Button
            variant="outline"
            size="sm"
            onClick={() => setReconnectToken((n) => n + 1)}
          >
            Reconnect
          </Button>
          <Button variant="outline" size="sm" asChild>
            <a href={fullscreenHref} target="_blank" rel="noopener noreferrer">
              <ExternalLink data-icon="inline-start" />
              Fullscreen
            </a>
          </Button>
        </div>
      </div>

      <div className="h-[min(70vh,560px)] overflow-hidden rounded-xl border border-border/70 bg-zinc-950 shadow-sm">
        <LiveTerminal
          className="h-full rounded-none"
          asUser={asUser}
          title={`${asUser} · user shell`}
          reconnectToken={reconnectToken}
          onStatusChange={setStatus}
        />
      </div>
    </div>
  )
}
