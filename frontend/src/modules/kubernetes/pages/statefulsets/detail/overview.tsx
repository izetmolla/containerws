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
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { toastRequestError } from "@/lib/network";

import {
  deleteStatefulSet,
  formatAge,
  restartStatefulSet,
  scaleStatefulSet,
} from "../../_shared/api";
import { KeyValueList, MetaGrid } from "../../_shared/resource-ui";
import type { StatefulSetOutletContext } from "./layout";

export default function StatefulSetOverviewPage() {
  const { namespace, name, statefulSet, invalidate } =
    useOutletContext<StatefulSetOutletContext>();
  const navigate = useNavigate();
  const [scaleOpen, setScaleOpen] = useState(false);
  const [deleteOpen, setDeleteOpen] = useState(false);
  const [replicas, setReplicas] = useState(String(statefulSet.replicas));
  const scale = useMutation({
    mutationFn: () =>
      scaleStatefulSet(namespace, name, Math.max(0, Number(replicas) || 0)),
    onSuccess: (res) => {
      toast.success(res.message || "StatefulSet scaled");
      setScaleOpen(false);
      invalidate();
    },
    onError: (error) => toastRequestError(error, "Scale failed"),
  });
  const restart = useMutation({
    mutationFn: () => restartStatefulSet(namespace, name),
    onSuccess: (res) => {
      toast.success(res.message || "StatefulSet restarted");
      invalidate();
    },
    onError: (error) => toastRequestError(error, "Restart failed"),
  });
  const remove = useMutation({
    mutationFn: () => deleteStatefulSet(namespace, name),
    onSuccess: (res) => {
      toast.success(res.message || "StatefulSet deleted");
      navigate("/kubernetes/statefulsets");
    },
    onError: (error) => toastRequestError(error, "Delete failed"),
  });
  return (
    <div className="space-y-6">
      <div className="flex flex-wrap justify-end gap-2">
        <Button size="sm" variant="outline" onClick={() => setScaleOpen(true)}>
          Scale
        </Button>
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
          { label: "Ready", value: statefulSet.ready },
          { label: "Replicas", value: statefulSet.replicas },
          { label: "Current", value: statefulSet.current_replicas ?? "—" },
          { label: "Updated", value: statefulSet.updated_replicas ?? "—" },
          { label: "Service", value: statefulSet.service_name || "—" },
          {
            label: "Update strategy",
            value: statefulSet.update_strategy || "—",
          },
          { label: "Age", value: formatAge(statefulSet.created_at) },
        ]}
      />
      <section className="space-y-2">
        <h2 className="text-sm font-medium">Containers</h2>
        <div className="divide-y rounded-xl border">
          {(statefulSet.containers || []).map((container) => (
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
        <KeyValueList data={statefulSet.selector} />
      </section>
      <section className="space-y-2">
        <h2 className="text-sm font-medium">Labels</h2>
        <KeyValueList data={statefulSet.labels} />
      </section>
      <Dialog open={scaleOpen} onOpenChange={setScaleOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Scale StatefulSet</DialogTitle>
            <DialogDescription>
              {namespace}/{name}
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-2">
            <Label htmlFor="detail-replicas">Replicas</Label>
            <Input
              id="detail-replicas"
              type="number"
              min={0}
              value={replicas}
              onChange={(event) => setReplicas(event.target.value)}
            />
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setScaleOpen(false)}>
              Cancel
            </Button>
            <Button disabled={scale.isPending} onClick={() => scale.mutate()}>
              Scale
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
      <Dialog open={deleteOpen} onOpenChange={setDeleteOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Delete StatefulSet?</DialogTitle>
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
