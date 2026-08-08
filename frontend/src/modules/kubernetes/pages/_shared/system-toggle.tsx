import { Label } from "@/components/ui/label"
import { Switch } from "@/components/ui/switch"
import { cn } from "@/lib/utils"

import { getStoredNamespace, setStoredNamespace } from "./api"
import {
  isSystemNamespace,
  useShowSystemResources,
} from "./system-resources"

const NS_EVENT = "cws-k8s-namespace"

export function SystemResourcesToggle({
  className,
  id = "k8s-show-system",
}: {
  className?: string
  id?: string
}) {
  const [showSystem, setShowSystem] = useShowSystemResources()

  return (
    <div
      className={cn(
        "flex items-center gap-2 rounded-lg border bg-muted/20 px-2.5 py-1.5",
        className,
      )}
    >
      <Switch
        id={id}
        size="sm"
        checked={showSystem}
        onCheckedChange={(checked) => {
          setShowSystem(checked)
          if (!checked) {
            const ns = getStoredNamespace()
            if (ns && isSystemNamespace(ns)) {
              setStoredNamespace("")
              window.dispatchEvent(new Event(NS_EVENT))
            }
          }
        }}
        aria-label="Show system resources"
      />
      <Label htmlFor={id} className="cursor-pointer text-xs font-medium">
        System
      </Label>
    </div>
  )
}
