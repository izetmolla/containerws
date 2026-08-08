import { useMemo, useState } from "react"
import { useMutation, useQuery } from "@tanstack/react-query"
import {
  ArrowRightLeft,
  Copy,
  FilePlus2,
  Pencil,
  Square,
  Trash2,
} from "lucide-react"
import { toast } from "sonner"

import { Button } from "@/components/ui/button"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { asArray } from "@/lib/as-array"
import { toastRequestError } from "@/lib/network"

import {
  getStoredEnvironmentId,
  listDockerEnvironments,
} from "../../environments/api"
import {
  createStack,
  type StackDetail,
} from "../api"
import { StackContainersPanel } from "./stack-containers-panel"

export function StackOverviewTab({
  detail,
  busy,
  onStop,
  onRemove,
  onCreateTemplate,
  onChanged,
  onOpenEditor,
}: {
  detail: StackDetail
  busy: boolean
  onStop: () => void
  onRemove: () => void
  onCreateTemplate: () => void
  onChanged: () => void
  onOpenEditor: () => void
}) {
  const currentEnvId = getStoredEnvironmentId() || detail.environment_id
  const [dupName, setDupName] = useState("")
  const [targetEnv, setTargetEnv] = useState("")

  const envsQuery = useQuery({
    queryKey: ["docker-environments", "stack-migrate"],
    queryFn: listDockerEnvironments,
  })
  const environments = asArray(envsQuery.data?.data).filter(
    (e) => !e.is_disabled
  )

  const otherEnvs = useMemo(
    () => environments.filter((e) => e.id !== currentEnvId),
    [environments, currentEnvId]
  )

  const duplicateMutation = useMutation({
    mutationFn: () => {
      const name = (dupName.trim() || `${detail.name}-copy`).toLowerCase()
      return createStack(
        {
          name,
          compose_yaml: detail.compose_yaml,
          env_file: detail.env_file || "",
          deploy: false,
        },
        currentEnvId
      )
    },
    onSuccess: (res) => {
      toast.success(res.message || "Stack duplicated")
      onChanged()
      if (res.data?.id) {
        window.location.assign(
          `/docker/stacks/edit?id=${encodeURIComponent(res.data.id)}`
        )
      }
    },
    onError: (err) => toastRequestError(err, "Duplicate failed"),
  })

  const migrateMutation = useMutation({
    mutationFn: () => {
      if (!targetEnv) throw new Error("Select an environment")
      const name = (dupName.trim() || detail.name).toLowerCase()
      return createStack(
        {
          name,
          compose_yaml: detail.compose_yaml,
          env_file: detail.env_file || "",
          deploy: false,
        },
        targetEnv
      )
    },
    onSuccess: (res) => {
      toast.success(res.message || "Stack migrated")
      onChanged()
      if (res.data?.id) {
        window.location.assign(
          `/docker/stacks/edit?id=${encodeURIComponent(res.data.id)}`
        )
      }
    },
    onError: (err) => toastRequestError(err, "Migrate failed"),
  })

  const canDuplicate = !duplicateMutation.isPending && !busy
  const canMigrate =
    Boolean(targetEnv) && !migrateMutation.isPending && !busy && otherEnvs.length > 0

  return (
    <div className="flex flex-col gap-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h2 className="text-lg font-semibold tracking-tight">{detail.name}</h2>
          <p className="text-sm text-muted-foreground capitalize">
            Status: {detail.status}
            {detail.running_count != null
              ? ` · ${detail.running_count}/${detail.container_count} running`
              : null}
          </p>
        </div>
        <div className="flex flex-wrap gap-2">
          <Button
            variant="destructive"
            size="sm"
            disabled={busy}
            onClick={onStop}
          >
            <Square data-icon="inline-start" />
            Stop this stack
          </Button>
          <Button
            variant="destructive"
            size="sm"
            disabled={busy}
            onClick={onRemove}
          >
            <Trash2 data-icon="inline-start" />
            Delete this stack
          </Button>
          <Button
            variant="outline"
            size="sm"
            disabled={busy}
            onClick={onCreateTemplate}
          >
            <FilePlus2 data-icon="inline-start" />
            Create template from stack
          </Button>
          <Button variant="secondary" size="sm" onClick={onOpenEditor}>
            <Pencil data-icon="inline-start" />
            Edit compose
          </Button>
        </div>
      </div>

      <Card className="gap-0 py-0">
        <CardHeader className="border-b py-4">
          <CardTitle className="flex items-center gap-2 text-base">
            <Copy className="size-4 text-muted-foreground" />
            Stack duplication / migration
          </CardTitle>
          <CardDescription>
            Duplicate in this environment, or migrate the Compose file to
            another Docker environment.
          </CardDescription>
        </CardHeader>
        <CardContent className="grid gap-4 py-5 sm:grid-cols-2">
          <div className="grid gap-1.5">
            <Label htmlFor="dup-name">Stack name (optional for migration)</Label>
            <Input
              id="dup-name"
              value={dupName}
              onChange={(e) => setDupName(e.target.value)}
              placeholder={detail.name}
            />
          </div>
          <div className="grid gap-1.5">
            <Label>Select an environment</Label>
            <Select
              value={targetEnv || undefined}
              onValueChange={setTargetEnv}
            >
              <SelectTrigger className="w-full">
                <SelectValue placeholder="Target environment for migrate" />
              </SelectTrigger>
              <SelectContent>
                {otherEnvs.map((env) => (
                  <SelectItem key={env.id} value={env.id}>
                    {env.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <div className="flex flex-wrap gap-2 sm:col-span-2">
            <Button
              variant="outline"
              disabled={!canMigrate}
              onClick={() => migrateMutation.mutate()}
            >
              <ArrowRightLeft data-icon="inline-start" />
              Migrate
            </Button>
            <Button
              variant="outline"
              disabled={!canDuplicate}
              onClick={() => duplicateMutation.mutate()}
            >
              <Copy data-icon="inline-start" />
              Duplicate
            </Button>
          </div>
        </CardContent>
      </Card>

      <StackContainersPanel
        stackName={detail.name}
        containers={detail.containers ?? []}
        onChanged={onChanged}
      />
    </div>
  )
}
