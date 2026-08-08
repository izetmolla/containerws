import { useMutation } from "@tanstack/react-query";
import { useState } from "react";
import { useNavigate, useOutletContext } from "react-router";
import { toast } from "sonner";

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

import {
  deleteDaemonSet,
  formatAge,
  restartDaemonSet,
} from "../../_shared/api";
import { KeyValueList, MetaGrid } from "../../_shared/resource-ui";
import type { DaemonSetOutletContext } from "./layout";

export default function DaemonSetOverviewPage() {
  const { namespace, name, daemonSet, invalidate } =
    useOutletContext<DaemonSetOutletContext>();
  const navigate = useNavigate();
  const [deleteOpen, setDeleteOpen] = useState(false);
  const restart = useMutation({
    mutationFn: () => restartDaemonSet(namespace, name),
    onSuccess: (res) => {
      toast.success(res.message || "DaemonSet restarted");
      invalidate();
    },
    onError: (error) => toastRequestError(error, "Restart failed"),
  });
  const remove = useMutation({
    mutationFn: () => deleteDaemonSet(namespace, name),
    onSuccess: (res) => {
      toast.success(res.message || "DaemonSet deleted");
      navigate("/kubernetes/daemonsets");
    },
    onError: (error) => toastRequestError(error, "Delete failed"),
  });
  return (
    <div className="space-y-6">
      <div className="flex flex-wrap justify-end gap-2">
        <Button
          size="sm"
          variant="outline"
          disabled={restart.isPending}
          onClick={() => restart.mutate()}
        >
          Restart
        </Button>
        <Button
          size="sm"
          variant="destructive"
          onClick={() => setDeleteOpen(true)}
        >
          Delete
        </Button>
      </div>
      <MetaGrid
        items={[
          { label: "Namespace", value: namespace },
          { label: "Desired", value: daemonSet.desired },
          { label: "Current", value: daemonSet.current },
          { label: "Ready", value: daemonSet.ready },
          { label: "Up-to-date", value: daemonSet.up_to_date },
          { label: "Available", value: daemonSet.available },
          { label: "Age", value: formatAge(daemonSet.created_at) },
        ]}
      />
      <section className="space-y-2">
        <h2 className="text-sm font-medium">Containers</h2>
        <div className="divide-y rounded-xl border">
          {(daemonSet.containers || []).map((container) => (
            <div
              key={container.name}
              className="grid gap-1 px-3 py-2 sm:grid-cols-[180px_1fr]"
            >
              <span className="font-medium">{container.name}</span>
              <span className="font-mono text-xs">{container.image}</span>
            </div>
          ))}
        </div>
      </section>
      <section className="space-y-2">
        <h2 className="text-sm font-medium">Selector</h2>
        <KeyValueList data={daemonSet.selector} />
      </section>
      <section className="space-y-2">
        <h2 className="text-sm font-medium">Labels</h2>
        <KeyValueList data={daemonSet.labels} />
      </section>
      <Dialog open={deleteOpen} onOpenChange={setDeleteOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Delete DaemonSet?</DialogTitle>
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
  );
}
