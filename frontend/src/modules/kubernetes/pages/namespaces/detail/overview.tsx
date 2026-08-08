import { useMutation } from "@tanstack/react-query"
import { useState } from "react"
import { Link, useNavigate, useOutletContext } from "react-router"
import { toast } from "sonner"

import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import {
  Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle,
} from "@/components/ui/dialog"
import { toastRequestError } from "@/lib/network"

import { deleteNamespace, formatAge } from "../../_shared/api"
import { KeyValueList, MetaGrid } from "../../_shared/resource-ui"
import type { NamespaceOutletContext } from "./layout"

export default function NamespaceOverviewPage() {
  const { name, namespace } = useOutletContext<NamespaceOutletContext>()
  const navigate = useNavigate()
  const [open, setOpen] = useState(false)
  const remove = useMutation({
    mutationFn: () => deleteNamespace(name),
    onSuccess: (res) => { toast.success(res.message || "Namespace deleted"); navigate("/kubernetes/namespaces") },
    onError: (error) => toastRequestError(error, "Delete failed"),
  })
  const resourceLink = (path: string, count: number) => (
    <Link className="hover:underline" to={`/kubernetes/${path}`}>{count}</Link>
  )

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <Badge variant={namespace.status === "Active" ? "default" : "secondary"}>{namespace.status}</Badge>
        <Button size="sm" variant="destructive" onClick={() => setOpen(true)}>Delete</Button>
      </div>
      <MetaGrid items={[
        { label: "Age", value: formatAge(namespace.created_at) },
        { label: "Pods", value: resourceLink("pods", namespace.pods) },
        { label: "Deployments", value: resourceLink("deployments", namespace.deployments) },
        { label: "Services", value: resourceLink("services", namespace.services) },
        { label: "ConfigMaps", value: resourceLink("configmaps", namespace.configmaps) },
        { label: "Secrets", value: resourceLink("secrets", namespace.secrets) },
      ]} />
      <p className="text-xs text-muted-foreground">Select “{name}” in the namespace filter on a resource list to narrow these counts.</p>
      <section className="space-y-2"><h2 className="text-sm font-medium">Labels</h2><KeyValueList data={namespace.labels} /></section>
      <section className="space-y-2"><h2 className="text-sm font-medium">Annotations</h2><KeyValueList data={namespace.annotations} /></section>
      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent><DialogHeader><DialogTitle>Delete namespace?</DialogTitle><DialogDescription>Delete “{name}” and its namespaced resources?</DialogDescription></DialogHeader>
          <DialogFooter><Button variant="outline" onClick={() => setOpen(false)}>Cancel</Button><Button variant="destructive" disabled={remove.isPending} onClick={() => remove.mutate()}>Delete</Button></DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
