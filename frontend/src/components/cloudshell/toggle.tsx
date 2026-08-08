"use client"

import { SquareTerminal } from "lucide-react"

import { Button } from "@/components/ui/button"
import { cn } from "@/lib/utils"

import { useCloudShellStore } from "./store"

/** Header control to open/close Cloud Shell (replaces the floating launcher). */
export function CloudShellToggle() {
  const open = useCloudShellStore((s) => s.open)
  const toggleShell = useCloudShellStore((s) => s.toggleShell)

  return (
    <Button
      type="button"
      size="icon-sm"
      variant="ghost"
      className={cn("relative", open && "bg-muted text-foreground")}
      onClick={() => toggleShell()}
      title="Cloud Shell (Ctrl+Shift+B)"
      aria-label="Cloud Shell"
      aria-pressed={open}
    >
      <SquareTerminal />
      <span className="sr-only">Cloud Shell</span>
    </Button>
  )
}
