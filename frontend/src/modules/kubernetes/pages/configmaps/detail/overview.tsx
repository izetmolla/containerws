import { useMutation } from "@tanstack/react-query"
import { Plus, Trash2 } from "lucide-react"
import { useEffect, useState } from "react"
import { useNavigate, useOutletContext } from "react-router"
import { toast } from "sonner"

import { Button } from "@/components/ui/button"
import {
  Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle,
} from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Textarea } from "@/components/ui/textarea"
import { toastRequestError } from "@/lib/network"

import { deleteConfigMap, updateConfigMap } from "../../_shared/api"
import { KeyValueList } from "../../_shared/resource-ui"
import type { ConfigMapOutletContext } from "./layout"

type Entry = { id: number; key: string; value: string }

export default function ConfigMapOverviewPage() {
  const { namespace, name, configMap, invalidate } = useOutletContext<ConfigMapOutletContext>()
  const navigate = useNavigate()
  const [entries, setEntries] = useState<Entry[]>([])
  const [deleteOpen, setDeleteOpen] = useState(false)
  useEffect(() => {
    setEntries(Object.entries(configMap.data || {}).map(([key, value], id) => ({ id, key, value })))
  }, [configMap.data])
  const save = useMutation({
    mutationFn: () => updateConfigMap(namespace, name, Object.fromEntries(entries.filter((item) => item.key).map((item) => [item.key, item.value]))),
    onSuccess: (res) => { toast.success(res.message || "ConfigMap saved"); invalidate() },
    onError: (error) => toastRequestError(error, "Save failed"),
  })
  const remove = useMutation({
    mutationFn: () => deleteConfigMap(namespace, name),
    onSuccess: (res) => { toast.success(res.message || "ConfigMap deleted"); navigate("/kubernetes/configmaps") },
    onError: (error) => toastRequestError(error, "Delete failed"),
  })
  const updateEntry = (id: number, patch: Partial<Entry>) =>
    setEntries((current) => current.map((item) => item.id === id ? { ...item, ...patch } : item))

  return (
    <div className="space-y-6">
      <div className="flex justify-end gap-2">
        <Button size="sm" variant="outline" onClick={() => setDeleteOpen(true)}>Delete</Button>
        <Button size="sm" disabled={save.isPending} onClick={() => save.mutate()}>Save</Button>
      </div>
      <section className="space-y-3">
        <div className="flex items-center justify-between"><h2 className="text-sm font-medium">Data</h2>
          <Button size="sm" variant="outline" onClick={() => setEntries((items) => [...items, { id: Date.now(), key: "", value: "" }])}><Plus className="size-3.5" />Add key</Button>
        </div>
        {entries.map((item) => (
          <div key={item.id} className="grid gap-2 rounded-xl border p-3 sm:grid-cols-[220px_1fr_auto]">
            <Input value={item.key} onChange={(e) => updateEntry(item.id, { key: e.target.value })} placeholder="Key" className="font-mono text-xs" />
            <Textarea value={item.value} onChange={(e) => updateEntry(item.id, { value: e.target.value })} placeholder="Value" className="min-h-20 font-mono text-xs" />
            <Button size="icon-sm" variant="ghost" onClick={() => setEntries((items) => items.filter((entry) => entry.id !== item.id))}><Trash2 className="size-4" /></Button>
          </div>
        ))}
      </section>
      <section className="space-y-2"><h2 className="text-sm font-medium">Labels</h2><KeyValueList data={configMap.labels} /></section>
      <section className="space-y-2"><h2 className="text-sm font-medium">Annotations</h2><KeyValueList data={configMap.annotations} /></section>
      <Dialog open={deleteOpen} onOpenChange={setDeleteOpen}>
        <DialogContent><DialogHeader><DialogTitle>Delete ConfigMap?</DialogTitle><DialogDescription>Delete {namespace}/{name}?</DialogDescription></DialogHeader>
          <DialogFooter><Button variant="outline" onClick={() => setDeleteOpen(false)}>Cancel</Button><Button variant="destructive" disabled={remove.isPending} onClick={() => remove.mutate()}>Delete</Button></DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
