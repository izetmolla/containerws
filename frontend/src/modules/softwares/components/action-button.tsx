import { Button } from "@/components/ui/button"
import {
  softwareInstallStatus,
  type SoftwareListItem,
} from "../pages/list/api"
import { Download, ExternalLink, Loader2, RefreshCw } from "lucide-react"

export function ActionButton({
  software,
  onOpen,
  onUpdate,
  size = "sm",
  fullWidth,
  busy,
}: {
  software: SoftwareListItem
  onOpen?: (s: SoftwareListItem) => void
  onUpdate?: (s: SoftwareListItem) => void
  size?: "sm" | "default"
  fullWidth?: boolean
  busy?: "installing" | "updating" | null
}) {
  const status = softwareInstallStatus(software, busy)
  const cls = fullWidth ? "w-full" : ""

  if (status === "installing" || status === "updating") {
    return (
      <Button size={size} variant="secondary" disabled className={cls}>
        <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" />
        {status === "installing" ? "Installing…" : "Updating…"}
      </Button>
    )
  }

  if (status === "update_available") {
    return (
      <Button
        size={size}
        className={`${cls} bg-amber-500 text-black hover:bg-amber-400`}
        onClick={(e) => {
          e.stopPropagation()
          onUpdate?.(software)
        }}
      >
        <RefreshCw className="mr-1.5 h-3.5 w-3.5" />
        Update
      </Button>
    )
  }

  if (status === "installed") {
    return (
      <Button
        size={size}
        variant="secondary"
        className={cls}
        onClick={(e) => {
          e.stopPropagation()
          onOpen?.(software)
        }}
      >
        <ExternalLink className="mr-1.5 h-3.5 w-3.5" />
        Open
      </Button>
    )
  }

  if (status === "missing") {
    return (
      <Button
        size={size}
        className={`${cls} bg-amber-500 text-black hover:bg-amber-400`}
        onClick={(e) => {
          e.stopPropagation()
          onOpen?.(software)
        }}
      >
        <Download className="mr-1.5 h-3.5 w-3.5" />
        Reinstall
      </Button>
    )
  }

  if (status === "uninstalled") {
    return (
      <Button
        size={size}
        className={cls}
        onClick={(e) => {
          e.stopPropagation()
          onOpen?.(software)
        }}
      >
        <Download className="mr-1.5 h-3.5 w-3.5" />
        Reinstall
      </Button>
    )
  }

  return (
    <Button
      size={size}
      className={cls}
      onClick={(e) => {
        e.stopPropagation()
        onOpen?.(software)
      }}
    >
      <Download className="mr-1.5 h-3.5 w-3.5" />
      Install
    </Button>
  )
}
