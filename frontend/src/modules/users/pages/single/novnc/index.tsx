import { useMemo, useState } from "react"
import { Link, useOutletContext } from "react-router"
import { useMutation } from "@tanstack/react-query"
import {
  ExternalLink,
  Monitor,
  RefreshCw,
} from "lucide-react"
import { toast } from "sonner"

import { Button } from "@/components/ui/button"
import { getRequestErrorMessage } from "@/lib/network"
import { cn } from "@/lib/utils"

import {
  novncClientURL,
  openNovnc,
  withNovncAuth,
} from "../../list/api"
import { startVncProfile } from "../vnc/api"
import type { UserSingleOutletContext } from "../types"

export default function UserNovncPage() {
  const { user, id, invalidate } = useOutletContext<UserSingleOutletContext>()
  const [iframeKey, setIframeKey] = useState(0)

  const rawUrl = useMemo(() => {
    if (user.novnc_url?.trim()) return user.novnc_url.trim()
    if (user.vnc?.id) return novncClientURL(user.vnc.id)
    return ""
  }, [user.novnc_url, user.vnc?.id])

  const iframeSrc = useMemo(
    () => (rawUrl ? withNovncAuth(rawUrl) : ""),
    [rawUrl]
  )

  const live = Boolean(user.vnc?.live)
  const hasProfile = Boolean(user.has_vnc || user.vnc)

  const startMutation = useMutation({
    mutationFn: () => startVncProfile(id),
    onSuccess: (res) => {
      toast.success(res.message || "Desktop started")
      invalidate()
      setIframeKey((n) => n + 1)
    },
    onError: (err) =>
      toast.error(getRequestErrorMessage(err, "Failed to start desktop")),
  })

  if (!hasProfile) {
    return (
      <section className="rounded-xl border bg-card p-8 text-center">
        <Monitor className="mx-auto size-8 text-muted-foreground" />
        <h2 className="mt-3 text-base font-semibold">No VNC profile</h2>
        <p className="mt-1 text-sm text-muted-foreground">
          Create a desktop profile on the VNC &amp; NoVNC tab before opening
          noVNC for this user.
        </p>
        <Button className="mt-4" asChild>
          <Link to={`/users/${id}/vnc`}>Open VNC settings</Link>
        </Button>
      </section>
    )
  }

  return (
    <div className="flex flex-col gap-3">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div>
          <h2 className="text-sm font-semibold">noVNC</h2>
          <p className="text-xs text-muted-foreground">
            Remote desktop for{" "}
            <code className="text-[11px]">
              {user.username || user.full_name || "user"}
            </code>
          </p>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <span
            className={cn(
              "inline-flex items-center gap-1.5 rounded-md px-2 py-1 text-[11px] font-medium",
              live
                ? "bg-emerald-500/15 text-emerald-700 dark:text-emerald-300"
                : "bg-muted text-muted-foreground"
            )}
          >
            <span
              className={cn(
                "size-1.5 rounded-full",
                live ? "bg-emerald-500" : "bg-muted-foreground"
              )}
              aria-hidden
            />
            {live ? "Live" : "Stopped"}
          </span>
          {!live ? (
            <Button
              size="sm"
              disabled={startMutation.isPending}
              onClick={() => startMutation.mutate()}
            >
              Start desktop
            </Button>
          ) : null}
          <Button
            variant="outline"
            size="sm"
            disabled={!iframeSrc}
            onClick={() => setIframeKey((n) => n + 1)}
          >
            <RefreshCw data-icon="inline-start" />
            Reload
          </Button>
          <Button
            variant="outline"
            size="sm"
            disabled={!rawUrl}
            onClick={() => openNovnc(rawUrl)}
          >
            <ExternalLink data-icon="inline-start" />
            Open in tab
          </Button>
        </div>
      </div>

      <div className="h-[min(75vh,720px)] overflow-hidden rounded-xl border border-border/70 bg-zinc-950 shadow-sm">
        {iframeSrc && live ? (
          <iframe
            key={iframeKey}
            title="noVNC desktop"
            src={iframeSrc}
            className="h-full w-full border-0 bg-black"
            allow="clipboard-read; clipboard-write; fullscreen"
          />
        ) : (
          <div className="flex h-full flex-col items-center justify-center gap-3 px-6 text-center">
            <Monitor className="size-8 text-zinc-500" />
            <p className="text-sm text-zinc-300">
              {live
                ? "Desktop URL is not available."
                : "Desktop is stopped. Start it to load noVNC here."}
            </p>
            {!live ? (
              <Button
                size="sm"
                disabled={startMutation.isPending}
                onClick={() => startMutation.mutate()}
              >
                Start desktop
              </Button>
            ) : null}
          </div>
        )}
      </div>
    </div>
  )
}
