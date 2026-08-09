import { Badge } from "@/components/ui/badge"

export function ManagerBadge({
  packageManager,
}: {
  packageManager?: string | null
}) {
  if (packageManager === "brew") {
    return <Badge variant="secondary">Brew</Badge>
  }
  if (packageManager === "local") {
    return <Badge variant="outline">Local</Badge>
  }
  return <span className="text-muted-foreground">—</span>
}
