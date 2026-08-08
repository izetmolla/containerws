import { useMutation, useQueryClient } from "@tanstack/react-query"
import { useState } from "react"
import { useNavigate, useOutletContext } from "react-router"
import { Pause, Play, Trash2, Zap } from "lucide-react"
import { toast } from "sonner"

import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { toastRequestError } from "@/lib/network"

import {
  deleteCronJob,
  formatAge,
  K8S_CRONJOBS_KEY,
  resumeCronJob,
  suspendCronJob,
  triggerCronJob,
} from "../../_shared/api"
import { KeyValueList, MetaGrid } from "../../_shared/resource-ui"
import type { CronJobOutletContext } from "./layout"

export default function CronJobOverviewPage() {
  const { namespace, name, cronJob, invalidate } =
    useOutletContext<CronJobOutletContext>()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const [open, setOpen] = useState(false)

  const refresh = () => {
    invalidate?.()
    void queryClient.invalidateQueries({ queryKey: [K8S_CRONJOBS_KEY] })
  }

  const remove = useMutation({
    mutationFn: () => deleteCronJob(namespace, name),
    onSuccess: (res) => {
      toast.success(res.message || "CronJob deleted")
      navigate("/kubernetes/jobs")
    },
    onError: (error) => toastRequestError(error, "Delete failed"),
  })

  const suspend = useMutation({
    mutationFn: () => suspendCronJob(namespace, name),
    onSuccess: (res) => {
      toast.success(res.message || "CronJob suspended")
      refresh()
    },
    onError: (error) => toastRequestError(error, "Suspend failed"),
  })

  const resume = useMutation({
    mutationFn: () => resumeCronJob(namespace, name),
    onSuccess: (res) => {
      toast.success(res.message || "CronJob resumed")
      refresh()
    },
    onError: (error) => toastRequestError(error, "Resume failed"),
  })

  const trigger = useMutation({
    mutationFn: () => triggerCronJob(namespace, name),
    onSuccess: (res) => {
      toast.success(res.message || `Triggered job ${res.data?.name || ""}`)
      refresh()
    },
    onError: (error) => toastRequestError(error, "Trigger failed"),
  })

  const busy =
    suspend.isPending || resume.isPending || trigger.isPending || remove.isPending

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap justify-end gap-2">
        {cronJob.suspend ? (
          <Button
            size="sm"
            variant="outline"
            disabled={busy}
            onClick={() => resume.mutate()}
          >
            <Play className="size-3.5" />
            Resume
          </Button>
        ) : (
          <Button
            size="sm"
            variant="outline"
            disabled={busy}
            onClick={() => suspend.mutate()}
          >
            <Pause className="size-3.5" />
            Suspend
          </Button>
        )}
        <Button
          size="sm"
          variant="outline"
          disabled={busy}
          onClick={() => trigger.mutate()}
        >
          <Zap className="size-3.5" />
          Trigger now
        </Button>
        <Button
          size="sm"
          variant="destructive"
          disabled={busy}
          onClick={() => setOpen(true)}
        >
          <Trash2 className="size-3.5" />
          Delete
        </Button>
      </div>
      <MetaGrid
        items={[
          { label: "Namespace", value: namespace },
          {
            label: "Schedule",
            value: <span className="font-mono">{cronJob.schedule}</span>,
          },
          {
            label: "Suspended",
            value: (
              <Badge variant={cronJob.suspend ? "secondary" : "outline"}>
                {cronJob.suspend ? "Yes" : "No"}
              </Badge>
            ),
          },
          {
            label: "Age",
            value: formatAge(cronJob.created_at),
          },
          {
            label: "Active jobs",
            value: String(cronJob.active ?? 0),
          },
          {
            label: "Last schedule",
            value: cronJob.last_schedule
              ? formatAge(cronJob.last_schedule)
              : "—",
          },
        ]}
      />
      <div className="space-y-2">
        <h3 className="text-sm font-medium">Labels</h3>
        <KeyValueList data={cronJob.labels} />
      </div>
      <div className="space-y-2">
        <h3 className="text-sm font-medium">Annotations</h3>
        <KeyValueList data={cronJob.annotations} />
      </div>

      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Delete CronJob?</DialogTitle>
            <DialogDescription>
              Delete {namespace}/{name}? Future scheduled runs will stop.
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setOpen(false)}>
              Cancel
            </Button>
            <Button
              variant="destructive"
              disabled={remove.isPending}
              onClick={() => remove.mutate()}
            >
              Delete
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
