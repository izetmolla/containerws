"use client"

import { useEffect } from "react"
import { useQuery } from "@tanstack/react-query"
import { Monitor } from "lucide-react"

import { Button } from "@/components/ui/button"
import {
  getUser,
  openNovnc,
  USERS_FETCH_KEY,
} from "@/modules/users/pages/list/api"
import {
  getVncServiceStatus,
  VNC_SERVICE_FETCH_KEY,
} from "@/modules/vnc-novnc/pages/settings/api"
import { useCurrentUser } from "@/store/authorization"

/** Header control — opens the logged-in user's noVNC desktop in a new tab. */
export function DesktopToggle() {
  const user = useCurrentUser()
  const userId = user?.id?.trim() || ""

  const serviceQuery = useQuery({
    queryKey: [VNC_SERVICE_FETCH_KEY, "status"],
    queryFn: getVncServiceStatus,
    enabled: Boolean(userId),
    retry: false,
    refetchInterval: 12_000,
    staleTime: 8_000,
  })

  const userQuery = useQuery({
    queryKey: [USERS_FETCH_KEY, "detail", userId],
    queryFn: () => getUser(userId),
    enabled: Boolean(userId),
    retry: false,
    refetchInterval: 12_000,
    staleTime: 8_000,
  })

  const service = serviceQuery.data?.data
  const vnc = userQuery.data?.data?.vnc
  const packagesReady = Boolean(service?.packages_ready)
  const hasPassword = Boolean(vnc?.has_password)
  const sessionLive =
    Boolean(vnc?.live) ||
    Boolean(service?.sessions?.some((s) => s.user_id === userId && s.live))
  const serviceRunning = Boolean(service?.running)

  // Show only when tools are installed, this user set a VNC password,
  // and the desktop service/session is actually running.
  const showDesktop =
    packagesReady && hasPassword && (sessionLive || serviceRunning)

  useEffect(() => {
    if (!showDesktop) return
    const onKey = (e: KeyboardEvent) => {
      if ((e.ctrlKey || e.metaKey) && e.shiftKey && e.key.toLowerCase() === "d") {
        e.preventDefault()
        openNovnc("/novnc")
      }
    }
    window.addEventListener("keydown", onKey)
    return () => window.removeEventListener("keydown", onKey)
  }, [showDesktop])

  if (!showDesktop) {
    return null
  }

  return (
    <Button
      type="button"
      size="icon-sm"
      variant="ghost"
      onClick={() => openNovnc("/novnc")}
      title="Desktop (Ctrl+Shift+D)"
      aria-label="Desktop"
    >
      <Monitor />
      <span className="sr-only">Desktop</span>
    </Button>
  )
}
