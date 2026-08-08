import { useMutation } from "@tanstack/react-query"
import { useEffect, useState } from "react"
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

import { deleteSecret, updateSecret } from "../../_shared/api"
import { KeyValueList, MetaGrid } from "../../_shared/resource-ui"
import {
  SmartSecretBox,
  type SecretEntry,
} from "../../_shared/smart-secret-box"
import type { SecretOutletContext } from "./layout"

export default function SecretOverviewPage() {
  const { namespace, name, secret, invalidate } =
    useOutletContext<SecretOutletContext>()
  const navigate = useNavigate()
  const [entries, setEntries] = useState<SecretEntry[]>([])
  const [deleteOpen, setDeleteOpen] = useState(false)

  useEffect(() => {
    setEntries(
      Object.entries(secret.data || {}).map(([key, value], id) => ({
        id,
        key,
        value,
      })),
    )
  }, [secret.data])

  const save = useMutation({
    mutationFn: () =>
      updateSecret(
        namespace,
        name,
        Object.fromEntries(
          entries
            .filter((item) => item.key)
            .map((item) => [item.key, item.value]),
        ),
      ),
    onSuccess: (res) => {
      toast.success(res.message || "Secret saved")
      invalidate()
    },
    onError: (error) => toastRequestError(error, "Save failed"),
  })

  const remove = useMutation({
    mutationFn: () => deleteSecret(namespace, name),
    onSuccess: (res) => {
      toast.success(res.message || "Secret deleted")
      navigate("/kubernetes/secrets")
    },
    onError: (error) => toastRequestError(error, "Delete failed"),
  })

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap justify-end gap-2">
        <Button size="sm" variant="outline" onClick={() => setDeleteOpen(true)}>
          Delete
        </Button>
        <Button
          size="sm"
          disabled={save.isPending}
          onClick={() => save.mutate()}
        >
          Save
        </Button>
      </div>

      <MetaGrid
        items={[
          { label: "Namespace", value: namespace },
          { label: "Type", value: secret.type },
          { label: "Keys", value: entries.length },
        ]}
      />

      <SmartSecretBox entries={entries} onChange={setEntries} />

      <section className="space-y-2">
        <h2 className="text-sm font-medium">Labels</h2>
        <KeyValueList data={secret.labels} />
      </section>

      <Dialog open={deleteOpen} onOpenChange={setDeleteOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Delete secret?</DialogTitle>
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
