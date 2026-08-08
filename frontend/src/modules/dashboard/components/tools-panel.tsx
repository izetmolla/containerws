import { Link } from "react-router"
import { CheckCircle2, Download, Wrench } from "lucide-react"

import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"

import type { DashboardTool } from "../api"
import { metricCardClassName } from "../lib/format"

import { isGpuTool } from "./tools-utils"

type ToolsPanelProps = {
  tools: DashboardTool[]
}

export function ToolsPanel({ tools }: ToolsPanelProps) {
  // GPU tools are offered on the Graphics card when a GPU is present.
  const missing = tools.filter((t) => !t.installed && !isGpuTool(t))
  if (missing.length === 0) return null

  return (
    <Card className={metricCardClassName()}>
      <CardHeader className="flex flex-row items-start justify-between gap-3 space-y-0">
        <div className="space-y-1">
          <CardTitle className="flex items-center gap-2 text-base">
            <Wrench className="size-4 text-muted-foreground" />
            Monitoring tools
          </CardTitle>
          <p className="text-xs text-muted-foreground">
            Optional CLI tools that deepen live diagnostics. Install missing ones
            from Softwares when you need them.
          </p>
        </div>
        <Badge variant="outline" className="shrink-0">
          {missing.length} to install
        </Badge>
      </CardHeader>
      <CardContent className="grid gap-3 md:grid-cols-3">
        {missing.map((tool) => (
          <ToolCard key={tool.key} tool={tool} />
        ))}
      </CardContent>
    </Card>
  )
}

export function ToolCard({ tool }: { tool: DashboardTool }) {
  return (
    <div className="flex h-full flex-col gap-3 rounded-xl border border-border/60 bg-muted/20 p-4">
      <div className="flex items-start justify-between gap-2">
        <div className="min-w-0">
          <div className="flex items-center gap-2">
            <span
              className="size-2.5 shrink-0 rounded-full"
              style={{ backgroundColor: tool.color || "var(--muted-foreground)" }}
              aria-hidden
            />
            <h3 className="truncate font-medium tracking-tight">{tool.name}</h3>
          </div>
          <p className="mt-1 text-[11px] text-muted-foreground">
            {tool.sub_category || tool.category}
            {tool.binary ? ` · ${tool.binary}` : ""}
          </p>
        </div>
        {tool.installed ? (
          <Badge variant="secondary" className="shrink-0 gap-1">
            <CheckCircle2 className="size-3" />
            Ready
          </Badge>
        ) : (
          <Badge variant="outline" className="shrink-0">
            Missing
          </Badge>
        )}
      </div>
      <p className="line-clamp-3 flex-1 text-xs text-muted-foreground">
        {tool.details}
      </p>
      {tool.installed ? (
        <p className="truncate text-[11px] text-muted-foreground tabular-nums">
          {tool.present_path || "Detected on this host"}
        </p>
      ) : tool.software_id ? (
        <Button asChild size="sm" variant="outline" className="w-full">
          <Link to={`/softwares/${tool.software_id}`}>
            <Download data-icon="inline-start" />
            Install {tool.name}
          </Link>
        </Button>
      ) : (
        <Button asChild size="sm" variant="outline" className="w-full">
          <Link to="/softwares">
            <Download data-icon="inline-start" />
            Open Softwares
          </Link>
        </Button>
      )}
    </div>
  )
}
