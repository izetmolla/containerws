import { useState } from "react"
import { useMutation } from "@tanstack/react-query"
import { useNavigate, useOutletContext } from "react-router"
import { toast } from "sonner"

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
  deleteIngress,
  updateIngress,
  type IngressRule,
  type IngressTLS,
} from "../../_shared/api"
import {
  emptyIngressRule,
  IngressAdvancedEditor,
} from "../ingress-form"
import type { IngressOutletContext } from "./layout"

export default function IngressOverviewPage() {
  const { namespace, name, ingress, invalidate } =
    useOutletContext<IngressOutletContext>()
  const navigate = useNavigate()
  const [deleteOpen, setDeleteOpen] = useState(false)
  const [ingressClass, setIngressClass] = useState(ingress.ingress_class || "")
  const [rules, setRules] = useState<IngressRule[]>(
    ingress.rules?.length ? ingress.rules : [emptyIngressRule()],
  )
  const [tls, setTls] = useState<IngressTLS[]>(ingress.tls || [])
  const [labels, setLabels] = useState<Record<string, string>>(
    ingress.labels || {},
  )
  const [annotations, setAnnotations] = useState<Record<string, string>>(
    ingress.annotations || {},
  )
  const [syncedIngress, setSyncedIngress] = useState(ingress)
  if (ingress !== syncedIngress) {
    setSyncedIngress(ingress)
    setIngressClass(ingress.ingress_class || "")
    setRules(ingress.rules?.length ? ingress.rules : [emptyIngressRule()])
    setTls(ingress.tls || [])
    setLabels(ingress.labels || {})
    setAnnotations(ingress.annotations || {})
  }

  const metadataResetKey = `${namespace}/${name}:${JSON.stringify(ingress.labels || {})}:${JSON.stringify(ingress.annotations || {})}`

  const save = useMutation({
    mutationFn: () =>
      updateIngress(namespace, name, {
        ingress_class: ingressClass.trim() || undefined,
        rules: rules.map((rule) => ({
          host: rule.host.trim(),
          paths: rule.paths.map((p) => ({
            path: p.path.trim() || "/",
            path_type: p.path_type || "Prefix",
            service_name: p.service_name.trim(),
            service_port: Number(p.service_port) || 0,
            service_port_name: p.service_port_name?.trim() || undefined,
          })),
        })),
        tls: tls
          .filter((t) => t.secret_name.trim() || t.hosts.some((h) => h.trim()))
          .map((t) => ({
            secret_name: t.secret_name.trim(),
            hosts: t.hosts.map((h) => h.trim()).filter(Boolean),
          })),
        labels,
        annotations,
      }),
    onSuccess: (res) => {
      toast.success(res.message || "Ingress updated")
      invalidate()
    },
    onError: (error) => toastRequestError(error, "Update failed"),
  })

  const remove = useMutation({
    mutationFn: () => deleteIngress(namespace, name),
    onSuccess: (res) => {
      toast.success(res.message || "Ingress deleted")
      navigate("/kubernetes/ingresses")
    },
    onError: (error) => toastRequestError(error, "Delete failed"),
  })

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap justify-end gap-2">
        <Button
          size="sm"
          disabled={save.isPending}
          onClick={() => save.mutate()}
        >
          Save changes
        </Button>
        <Button
          size="sm"
          variant="destructive"
          onClick={() => setDeleteOpen(true)}
        >
          Delete
        </Button>
      </div>

      <IngressAdvancedEditor
        namespace={namespace}
        ingressClass={ingressClass}
        onIngressClassChange={setIngressClass}
        rules={rules}
        onRulesChange={setRules}
        tls={tls}
        onTlsChange={setTls}
        labels={labels}
        onLabelsChange={setLabels}
        annotations={annotations}
        onAnnotationsChange={setAnnotations}
        metadataResetKey={metadataResetKey}
      />

      <Dialog open={deleteOpen} onOpenChange={setDeleteOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Delete ingress?</DialogTitle>
            <DialogDescription>
              Delete {namespace}/{name}?
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setDeleteOpen(false)}>
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
