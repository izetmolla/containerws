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

import { deleteNetworkPolicy, formatAge } from "../../_shared/api";
import { KeyValueList, MetaGrid } from "../../_shared/resource-ui";
import type { NetworkPolicyOutletContext } from "./layout";

function Rules({ title, rules }: { title: string; rules: unknown[] }) {
  return (
    <section className="space-y-2">
      <div className="flex items-center gap-2">
        <h2 className="text-sm font-medium">{title}</h2>
        <Badge variant="secondary">{rules.length}</Badge>
      </div>
      {rules.length ? (
        <div className="space-y-2">
          {rules.map((rule, index) => (
            <pre
              key={index}
              className="overflow-auto rounded-xl border bg-muted/30 p-3 font-mono text-xs leading-relaxed whitespace-pre-wrap"
            >
              {JSON.stringify(rule, null, 2)}
            </pre>
          ))}
        </div>
      ) : (
        <p className="text-sm text-muted-foreground">No rules configured.</p>
      )}
    </section>
  );
}

export default function NetworkPolicyOverviewPage() {
  const { namespace, name, policy } =
    useOutletContext<NetworkPolicyOutletContext>();
  const navigate = useNavigate();
  const [open, setOpen] = useState(false);
  const remove = useMutation({
    mutationFn: () => deleteNetworkPolicy(namespace, name),
    onSuccess: (res) => {
      toast.success(res.message || "NetworkPolicy deleted");
      navigate("/kubernetes/network-policies");
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
          { label: "Pod selector", value: policy.pod_selector || "All pods" },
          {
            label: "Policy types",
            value: policy.policy_types.join(", ") || "—",
          },
          { label: "Ingress rules", value: policy.ingress.length },
          { label: "Egress rules", value: policy.egress.length },
          { label: "Age", value: formatAge(policy.created_at) },
        ]}
      />
      <section className="space-y-2">
        <h2 className="text-sm font-medium">Labels</h2>
        <KeyValueList data={policy.labels} />
      </section>
      <Rules title="Ingress rules" rules={policy.ingress} />
      <Rules title="Egress rules" rules={policy.egress} />
      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Delete NetworkPolicy?</DialogTitle>
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
