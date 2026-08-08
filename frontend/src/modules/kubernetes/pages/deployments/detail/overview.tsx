import { useMutation } from "@tanstack/react-query"
import { useState } from "react"
import { useNavigate, useOutletContext } from "react-router"
import { toast } from "sonner"

import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import {
  Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle,
} from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { toastRequestError } from "@/lib/network"

import {
  deleteDeployment, formatAge, pullRestartDeployment, restartDeployment, scaleDeployment,
} from "../../_shared/api"
import { KeyValueList, MetaGrid } from "../../_shared/resource-ui"
import type { DeploymentOutletContext } from "./layout"

export default function DeploymentOverviewPage() {
  const { namespace, name, deployment, invalidate } = useOutletContext<DeploymentOutletContext>()
  const navigate = useNavigate()
  const [scaleOpen, setScaleOpen] = useState(false)
  const [deleteOpen, setDeleteOpen] = useState(false)
  const [replicas, setReplicas] = useState(String(deployment.replicas))
  const scale = useMutation({
    mutationFn: () => scaleDeployment(namespace, name, Math.max(0, Number(replicas) || 0)),
    onSuccess: (res) => { toast.success(res.message || "Deployment scaled"); setScaleOpen(false); invalidate() },
    onError: (error) => toastRequestError(error, "Scale failed"),
  })
  const restart = useMutation({
    mutationFn: () => restartDeployment(namespace, name),
    onSuccess: (res) => { toast.success(res.message || "Deployment restarted"); invalidate() },
    onError: (error) => toastRequestError(error, "Restart failed"),
  })
  const pullRestart = useMutation({
    mutationFn: () => pullRestartDeployment(namespace, name),
    onSuccess: (res) => { toast.success(res.message || "Pulling image and restarting"); invalidate() },
    onError: (error) => toastRequestError(error, "Pull & restart failed"),
  })
  const remove = useMutation({
    mutationFn: () => deleteDeployment(namespace, name),
    onSuccess: (res) => { toast.success(res.message || "Deployment deleted"); navigate("/kubernetes/deployments") },
    onError: (error) => toastRequestError(error, "Delete failed"),
  })

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap justify-end gap-2">
        <Button size="sm" variant="outline" onClick={() => setScaleOpen(true)}>Scale</Button>
        <Button size="sm" variant="outline" disabled={restart.isPending} onClick={() => restart.mutate()}>Restart</Button>
        <Button size="sm" variant="outline" disabled={pullRestart.isPending} onClick={() => pullRestart.mutate()}>Pull image & restart</Button>
        <Button size="sm" variant="destructive" onClick={() => setDeleteOpen(true)}>Delete</Button>
      </div>
      <MetaGrid items={[
        { label: "Namespace", value: namespace }, { label: "Ready", value: deployment.ready },
        { label: "Replicas", value: deployment.replicas }, { label: "Available", value: deployment.available },
        { label: "Updated", value: deployment.updated_replicas }, { label: "Unavailable", value: deployment.unavailable },
        { label: "Strategy", value: deployment.strategy || "—" }, { label: "Age", value: formatAge(deployment.created_at) },
      ]} />
      <section className="space-y-2">
        <h2 className="text-sm font-medium">Containers</h2>
        <div className="divide-y rounded-xl border">
          {(deployment.containers || []).map((item) => (
            <div key={item.name} className="grid gap-1 px-3 py-2 sm:grid-cols-[180px_1fr]">
              <span className="font-medium">{item.name}</span><span className="font-mono text-xs">{item.image}</span>
            </div>
          ))}
        </div>
      </section>
      <section className="space-y-2"><h2 className="text-sm font-medium">Selector</h2><KeyValueList data={deployment.selector} /></section>
      <section className="space-y-2">
        <h2 className="text-sm font-medium">Conditions</h2>
        <div className="divide-y rounded-xl border">{(deployment.conditions || []).map((condition) => (
          <div key={condition.type} className="grid gap-2 px-3 py-2 sm:grid-cols-[140px_90px_1fr]">
            <span className="font-medium">{condition.type}</span>
            <Badge variant={condition.status === "True" ? "default" : "secondary"}>{condition.status}</Badge>
            <span className="text-sm text-muted-foreground">{condition.reason || condition.message || "—"}</span>
          </div>
        ))}</div>
      </section>
      <Dialog open={scaleOpen} onOpenChange={setScaleOpen}>
        <DialogContent><DialogHeader><DialogTitle>Scale deployment</DialogTitle><DialogDescription>{namespace}/{name}</DialogDescription></DialogHeader>
          <div className="space-y-2"><Label htmlFor="detail-replicas">Replicas</Label><Input id="detail-replicas" type="number" min={0} value={replicas} onChange={(e) => setReplicas(e.target.value)} /></div>
          <DialogFooter><Button variant="outline" onClick={() => setScaleOpen(false)}>Cancel</Button><Button disabled={scale.isPending} onClick={() => scale.mutate()}>Scale</Button></DialogFooter>
        </DialogContent>
      </Dialog>
      <Dialog open={deleteOpen} onOpenChange={setDeleteOpen}>
        <DialogContent><DialogHeader><DialogTitle>Delete deployment?</DialogTitle><DialogDescription>Delete {namespace}/{name}?</DialogDescription></DialogHeader>
          <DialogFooter><Button variant="outline" onClick={() => setDeleteOpen(false)}>Cancel</Button><Button variant="destructive" disabled={remove.isPending} onClick={() => remove.mutate()}>Delete</Button></DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
