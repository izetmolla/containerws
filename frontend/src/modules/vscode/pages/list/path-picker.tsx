import { useState } from "react"
import { FolderSearch } from "lucide-react"

import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { cn } from "@/lib/utils"

import { FolderSelectDialog } from "./folder-select-dialog"

type PathPickerProps = {
  id?: string
  value: string
  onChange: (next: string) => void
  userId?: string
  disabled?: boolean
  placeholder?: string
  className?: string
  /** Dialog title when browsing folders. */
  browseTitle?: string
}

function normalizePath(raw: string): string {
  const trimmed = raw.trim()
  if (!trimmed) return "/workspace"
  const withSlash = trimmed.startsWith("/") ? trimmed : `/${trimmed}`
  const clean = withSlash.replace(/\/+/g, "/")
  if (clean.length > 1 && clean.endsWith("/")) return clean.slice(0, -1)
  return clean || "/workspace"
}

export function PathPicker({
  id,
  value,
  onChange,
  userId,
  disabled,
  placeholder = "/workspace",
  className,
  browseTitle = "Select workspace folder",
}: PathPickerProps) {
  const [browseOpen, setBrowseOpen] = useState(false)

  return (
    <div className={cn("grid gap-2", className)}>
      <div className="flex gap-2">
        <Input
          id={id}
          value={value}
          disabled={disabled}
          placeholder={placeholder}
          autoComplete="off"
          spellCheck={false}
          className="min-w-0 flex-1 font-mono text-sm"
          onChange={(e) => onChange(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter") {
              e.preventDefault()
              onChange(normalizePath(value))
            }
          }}
        />
        <Button
          type="button"
          variant="outline"
          disabled={disabled}
          className="shrink-0"
          onClick={() => setBrowseOpen(true)}
        >
          <FolderSearch className="size-4" />
          Browse
        </Button>
      </div>
      <p className="text-xs text-muted-foreground">
        Type a path or browse folders on this machine.
      </p>

      <FolderSelectDialog
        open={browseOpen}
        title={browseTitle}
        description="Choose where this VS Code workspace should live. Missing folders can be created when the workspace starts."
        initialPath={value || "/workspace"}
        userId={userId}
        onOpenChange={setBrowseOpen}
        onConfirm={(folder) => onChange(folder)}
      />
    </div>
  )
}
