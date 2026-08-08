import { useMemo, useState } from "react"
import { useOutletContext } from "react-router"
import { Copy } from "lucide-react"
import { toast } from "sonner"

import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"

import type { ContainerOutletContext } from "./layout"

export default function ContainerInspectPage() {
  const { container } = useOutletContext<ContainerOutletContext>()
  const [filter, setFilter] = useState("")
  const json = useMemo(
    () => JSON.stringify(container.inspect ?? container, null, 2),
    [container],
  )

  const filtered = useMemo(() => {
    const needle = filter.trim().toLowerCase()
    if (!needle) return json
    return json
      .split("\n")
      .filter((line) => line.toLowerCase().includes(needle))
      .join("\n")
  }, [json, filter])

  return (
    <div className="flex flex-col gap-3">
      <div className="sticky top-0 z-[1] flex flex-wrap items-center gap-2 bg-background/95 py-1 backdrop-blur">
        <Input
          className="h-8 min-w-[12rem] flex-1"
          placeholder="Filter JSON lines…"
          value={filter}
          onChange={(e) => setFilter(e.target.value)}
        />
        <Button
          size="sm"
          variant="outline"
          onClick={() => {
            void navigator.clipboard.writeText(json).then(
              () => toast.success("Inspect JSON copied"),
              () => toast.error("Copy failed"),
            )
          }}
        >
          <Copy data-icon="inline-start" />
          Copy JSON
        </Button>
      </div>
      <pre className="max-h-[70vh] overflow-auto rounded-xl border bg-muted/30 p-4 font-mono text-xs leading-relaxed">
        {filtered || (filter.trim() ? "(no matching lines)" : "(empty)")}
      </pre>
    </div>
  )
}
