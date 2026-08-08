import { useMutation } from "@tanstack/react-query"
import { useState } from "react"
import { useNavigate, useOutletContext } from "react-router"
import { toast } from "sonner"

import { Button } from "@/components/ui/button"
import {
  Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle,
} from "@/components/ui/dialog"
import { toastRequestError } from "@/lib/network"

import { deleteService, formatAge } from "../../_shared/api"
import { KeyValueList, MetaGrid } from "../../_shared/resource-ui"
import type { ServiceOutletContext } from "./layout"

export default function ServiceOverviewPage() {
  const { namespace, name, service } = useOutletContext<ServiceOutletContext>()
  const navigate = useNavigate()
  const [open, setOpen] = useState(false)
  const remove = useMutation({
    mutationFn: () => deleteService(namespace, name),
    onSuccess: (res) => { toast.success(res.message || "Service deleted"); navigate("/kubernetes/services") },
    onError: (error) => toastRequestError(error, "Delete failed"),
  })
  return (
    <div className="space-y-6">
      <div className="flex justify-end"><Button size="sm" variant="destructive" onClick={() => setOpen(true)}>Delete</Button></div>
      <MetaGrid items={[
        { label: "Namespace", value: namespace }, { label: "Type", value: service.type },
        { label: "Cluster IP", value: service.cluster_ip || "—" }, { label: "External IP", value: service.external_ip || "—" },
        { label: "Age", value: formatAge(service.created_at) },
      ]} />
      <section className="space-y-2">
        <h2 className="text-sm font-medium">Ports</h2>
        <div className="overflow-x-auto rounded-xl border"><table className="w-full text-sm">
          <thead><tr className="border-b text-left text-xs text-muted-foreground"><th className="px-3 py-2">Name</th><th className="px-3 py-2">Port</th><th className="px-3 py-2">Target</th><th className="px-3 py-2">Node port</th><th className="px-3 py-2">Protocol</th></tr></thead>
          <tbody>{service.ports.map((port, index) => <tr key={`${port.name || index}-${port.port}`} className="border-b border-border/60">
            <td className="px-3 py-2">{port.name || "—"}</td><td className="px-3 py-2">{port.port}</td><td className="px-3 py-2">{port.target_port}</td><td className="px-3 py-2">{port.node_port || "—"}</td><td className="px-3 py-2">{port.protocol}</td>
          </tr>)}</tbody>
        </table></div>
      </section>
      <section className="space-y-2"><h2 className="text-sm font-medium">Selector</h2><KeyValueList data={service.selector} /></section>
      <section className="space-y-2"><h2 className="text-sm font-medium">Labels</h2><KeyValueList data={service.labels} /></section>
      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent><DialogHeader><DialogTitle>Delete service?</DialogTitle><DialogDescription>Delete {namespace}/{name}?</DialogDescription></DialogHeader>
          <DialogFooter><Button variant="outline" onClick={() => setOpen(false)}>Cancel</Button><Button variant="destructive" disabled={remove.isPending} onClick={() => remove.mutate()}>Delete</Button></DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
