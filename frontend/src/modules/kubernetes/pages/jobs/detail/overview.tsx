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

import { deleteJob, formatAge } from "../../_shared/api";
import { KeyValueList, MetaGrid } from "../../_shared/resource-ui";
import type { JobOutletContext } from "./layout";

export default function JobOverviewPage() {
  const { namespace, name, job } = useOutletContext<JobOutletContext>();
  const navigate = useNavigate();
  const [open, setOpen] = useState(false);
  const remove = useMutation({
    mutationFn: () => deleteJob(namespace, name),
    onSuccess: (res) => {
      toast.success(res.message || "Job deleted");
      navigate("/kubernetes/jobs");
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
          { label: "Completions", value: job.completions },
          { label: "Succeeded", value: job.succeeded },
          { label: "Failed", value: job.failed },
          { label: "Active", value: job.active },
          { label: "Duration", value: job.duration || "—" },
          { label: "Parallelism", value: job.parallelism ?? "—" },
          { label: "Backoff limit", value: job.backoff_limit ?? "—" },
          { label: "Age", value: formatAge(job.created_at) },
        ]}
      />
      <section className="space-y-2">
        <h2 className="text-sm font-medium">Containers</h2>
        <div className="divide-y rounded-xl border">
          {(job.containers || []).map((container) => (
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
        <h2 className="text-sm font-medium">Conditions</h2>
        <div className="divide-y rounded-xl border">
          {(job.conditions || []).map((condition) => (
            <div
              key={condition.type}
              className="grid gap-2 px-3 py-2 sm:grid-cols-[140px_90px_1fr]"
            >
              <span className="font-medium">{condition.type}</span>
              <Badge
                variant={condition.status === "True" ? "default" : "secondary"}
              >
                {condition.status}
              </Badge>
              <span className="text-sm text-muted-foreground">
                {condition.reason || condition.message || "—"}
              </span>
            </div>
          ))}
        </div>
      </section>
      <section className="space-y-2">
        <h2 className="text-sm font-medium">Labels</h2>
        <KeyValueList data={job.labels} />
      </section>
      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Delete Job?</DialogTitle>
            <DialogDescription>
              Delete {namespace}/{name}?
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
