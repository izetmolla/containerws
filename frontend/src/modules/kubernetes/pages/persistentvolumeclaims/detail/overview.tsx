import { useMutation } from "@tanstack/react-query";
import { useState } from "react";
import { useNavigate, useOutletContext } from "react-router";
import { toast } from "sonner";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { toastRequestError } from "@/lib/network";

import { deletePvc, formatAge } from "../../_shared/api";
import { KeyValueList, MetaGrid } from "../../_shared/resource-ui";
import type { PvcOutletContext } from "./layout";

export default function PvcOverviewPage() {
  const { namespace, name, pvc } = useOutletContext<PvcOutletContext>();
  const navigate = useNavigate();
  const [open, setOpen] = useState(false);
  const remove = useMutation({
    mutationFn: () => deletePvc(namespace, name),
    onSuccess: (res) => {
      toast.success(res.message || "PersistentVolumeClaim deleted");
      navigate("/kubernetes/persistentvolumeclaims");
    },
    onError: (error) => toastRequestError(error, "Delete failed"),
  });
  return (
    <div className="space-y-6">
      <div className="flex justify-end">
        <Button size="sm" variant="destructive" onClick={() => setOpen(true)}>
          Delete
        </Button>
      </div>
      <MetaGrid
        items={[
          { label: "Namespace", value: namespace },
          {
            label: "Status",
            value: (
              <Badge variant={pvc.status === "Bound" ? "default" : "secondary"}>
                {pvc.status}
              </Badge>
            ),
          },
          { label: "Capacity", value: pvc.capacity || "—" },
          { label: "Requested", value: pvc.request || "—" },
          { label: "Access modes", value: pvc.access_modes?.join(", ") || "—" },
          { label: "Storage class", value: pvc.storage_class || "—" },
          { label: "Volume", value: pvc.volume || "—" },
          { label: "Volume mode", value: pvc.volume_mode || "—" },
          { label: "Age", value: formatAge(pvc.created_at) },
        ]}
      />
      <section className="space-y-2">
        <h2 className="text-sm font-medium">Labels</h2>
        <KeyValueList data={pvc.labels} />
      </section>
      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Delete PersistentVolumeClaim?</DialogTitle>
            <DialogDescription>
              Delete {namespace}/{name}? Workloads using this claim may lose
              access to their storage.
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
  );
}
