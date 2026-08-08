import { useMutation } from "@tanstack/react-query"
import { useState } from "react"
import { Link, useNavigate, useOutletContext } from "react-router"
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

import { deletePod, formatAge } from "../../_shared/api"
import { KeyValueList, MetaGrid } from "../../_shared/resource-ui"
import type { PodOutletContext } from "./layout"

export default function PodOverviewPage() {
  const { namespace, name, pod } = useOutletContext<PodOutletContext>()
  const navigate = useNavigate()
  const [confirmOpen, setConfirmOpen] = useState(false)
  const remove = useMutation({
    mutationFn: () => deletePod(namespace, name),
    onSuccess: (res) => {
      toast.success(res.message || "Pod deleted")
      navigate("/kubernetes/pods")
    },
    onError: (error) => toastRequestError(error, "Delete failed"),
  })

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between gap-3">
        <Badge variant={pod.status === "Running" ? "default" : "secondary"}>
          {pod.status}
        </Badge>
        <Button variant="destructive" size="sm" onClick={() => setConfirmOpen(true)}>
          Delete
        </Button>
      </div>
      <MetaGrid
        items={[
          { label: "Namespace", value: namespace },
          { label: "Ready", value: pod.ready },
          { label: "Restarts", value: pod.restarts },
          { label: "Pod IP", value: pod.ip || "—" },
          { label: "Host IP", value: pod.host_ip || "—" },
          {
            label: "Node",
            value: pod.node ? (
              <Link className="hover:underline" to={`/kubernetes/nodes/${encodeURIComponent(pod.node)}`}>
                {pod.node}
              </Link>
            ) : "—",
          },
          { label: "QoS class", value: pod.qos_class || "—" },
          { label: "Owner", value: pod.owner || "—" },
          { label: "Age", value: formatAge(pod.created_at) },
        ]}
      />
      <section className="space-y-2">
        <h2 className="text-sm font-medium">Containers</h2>
        <div className="overflow-x-auto rounded-xl border">
          <table className="w-full text-sm">
            <thead><tr className="border-b text-left text-xs text-muted-foreground">
              <th className="px-3 py-2">Name</th><th className="px-3 py-2">Image</th>
              <th className="px-3 py-2">State</th><th className="px-3 py-2">Ready</th>
              <th className="px-3 py-2">Restarts</th>
            </tr></thead>
            <tbody>{pod.containers.map((container) => (
              <tr key={container.name} className="border-b border-border/60">
                <td className="px-3 py-2 font-medium">{container.name}</td>
                <td className="px-3 py-2 font-mono text-xs">{container.image}</td>
                <td className="px-3 py-2">{container.state}</td>
                <td className="px-3 py-2">{container.ready ? "Yes" : "No"}</td>
                <td className="px-3 py-2">{container.restart_count}</td>
              </tr>
            ))}</tbody>
          </table>
        </div>
      </section>
      <section className="space-y-2"><h2 className="text-sm font-medium">Labels</h2><KeyValueList data={pod.labels} /></section>
      <section className="space-y-2"><h2 className="text-sm font-medium">Annotations</h2><KeyValueList data={pod.annotations} /></section>
      <Dialog open={confirmOpen} onOpenChange={setConfirmOpen}>
        <DialogContent>
          <DialogHeader><DialogTitle>Delete pod?</DialogTitle><DialogDescription>Delete {namespace}/{name}?</DialogDescription></DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setConfirmOpen(false)}>Cancel</Button>
            <Button variant="destructive" disabled={remove.isPending} onClick={() => remove.mutate()}>Delete</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
