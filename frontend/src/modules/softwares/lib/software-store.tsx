import { useCallback, useMemo, useState, type ReactNode } from "react";
import { toast } from "sonner";
import { SEED_SOFTWARE, type Software } from "./software-data";
import { SoftwareCtx } from "./software-context";

const delay = (ms: number) => new Promise<void>((r) => setTimeout(r, ms));

export function SoftwareProvider({ children }: { children: ReactNode }) {
  const [items, setItems] = useState<Software[]>(SEED_SOFTWARE);

  const patch = useCallback((id: string, changes: Partial<Software>) => {
    setItems((prev) =>
      prev.map((s) => {
        if (s.id !== id) return s;
        const next = { ...s, ...changes };
        if (changes.installedVersion !== undefined) {
          next.versions = s.versions.map((vv) => ({
            ...vv,
            isCurrent: vv.version === changes.installedVersion,
          }));
        }
        return next;
      }),
    );
  }, []);

  const setStatus = useCallback(
    (id: string, status: Software["status"]) => patch(id, { status }),
    [patch],
  );

  const install = useCallback(
    async (id: string, version?: string) => {
      const target = SEED_SOFTWARE.find((s) => s.id === id);
      if (!target) return;
      setStatus(id, "installing");
      await delay(1500);
      const v = version ?? target.latestVersion;
      setItems((prev) =>
        prev.map((s) => {
          if (s.id !== id) return s;
          const isLatest = v === s.latestVersion;
          return {
            ...s,
            installedVersion: v,
            installedAt: new Date().toISOString().slice(0, 10),
            status: isLatest ? "installed" : "update_available",
            versions: s.versions.map((vv) => ({ ...vv, isCurrent: vv.version === v })),
          };
        }),
      );
      toast.success(`${target.name} ${v} installed`);
    },
    [setStatus],
  );

  const update = useCallback(
    async (id: string, version?: string) => {
      const target = SEED_SOFTWARE.find((s) => s.id === id);
      if (!target) return;
      setStatus(id, "updating");
      await delay(1500);
      setItems((prev) =>
        prev.map((s) => {
          if (s.id !== id) return s;
          const v = version ?? s.latestVersion;
          const isLatest = v === s.latestVersion;
          return {
            ...s,
            installedVersion: v,
            status: isLatest ? "installed" : "update_available",
            versions: s.versions.map((vv) => ({ ...vv, isCurrent: vv.version === v })),
          };
        }),
      );
      toast.success(`${target.name} updated to ${version ?? target.latestVersion}`);
    },
    [setStatus],
  );

  const uninstall = useCallback(async (id: string) => {
    const target = SEED_SOFTWARE.find((s) => s.id === id);
    if (!target) return;
    await delay(800);
    setItems((prev) =>
      prev.map((s) =>
        s.id === id
          ? {
              ...s,
              status: "not_installed",
              installedVersion: undefined,
              installedAt: undefined,
              versions: s.versions.map((vv) => ({ ...vv, isCurrent: false })),
            }
          : s,
      ),
    );
    toast.success(`${target.name} uninstalled`);
  }, []);

  const updateAll = useCallback(async () => {
    const targets = items.filter((s) => s.status === "update_available");
    await Promise.all(targets.map((t) => update(t.id)));
  }, [items, update]);

  const value = useMemo(
    () => ({
      items,
      getById: (id: string) => items.find((s) => s.id === id),
      install,
      update,
      uninstall,
      updateAll,
    }),
    [items, install, update, uninstall, updateAll],
  );

  return <SoftwareCtx.Provider value={value}>{children}</SoftwareCtx.Provider>;
}
