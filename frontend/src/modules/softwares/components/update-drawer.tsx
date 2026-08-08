import { Button } from "@/components/ui/button";
import { Progress } from "@/components/ui/progress";
import { Sheet, SheetContent, SheetHeader, SheetTitle } from "@/components/ui/sheet";
import type { Software } from "../lib/software-data";
import { useSoftware } from "../lib/use-software";
import { ArrowRight, RefreshCw } from "lucide-react";

export function UpdateDrawer({
  software,
  open,
  onOpenChange,
  onViewHistory,
}: {
  software: Software | null;
  open: boolean;
  onOpenChange: (v: boolean) => void;
  onViewHistory?: (s: Software) => void;
}) {
  const { update, getById } = useSoftware();
  const live = software ? getById(software.id) ?? software : null;
  if (!live) return null;

  const installedIdx = live.versions.findIndex((v) => v.version === live.installedVersion);
  const newerVersions = installedIdx > 0 ? live.versions.slice(0, installedIdx) : live.versions.slice(0, 1);
  const isUpdating = live.status === "updating";

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent className="w-full overflow-y-auto sm:max-w-md">
        <SheetHeader>
          <div className="flex items-center gap-3">
            <div className="flex h-12 w-12 items-center justify-center rounded-lg border bg-muted text-2xl">
              {live.icon}
            </div>
            <div>
              <SheetTitle>{live.name}</SheetTitle>
              <p className="text-xs text-muted-foreground">{live.publisher}</p>
            </div>
          </div>
        </SheetHeader>

        <div className="mt-6 space-y-6 px-4">
          <div className="rounded-lg border bg-muted/30 p-4">
            <p className="mb-3 text-xs font-medium uppercase tracking-wide text-muted-foreground">
              Version change
            </p>
            <div className="flex items-center gap-3">
              <span className="rounded-md border bg-background px-2.5 py-1 font-mono text-sm">
                v{live.installedVersion}
              </span>
              <ArrowRight className="h-4 w-4 text-muted-foreground" />
              <span className="rounded-md border border-amber-500/30 bg-amber-500/10 px-2.5 py-1 font-mono text-sm text-amber-500">
                v{live.latestVersion}
              </span>
            </div>
          </div>

          {isUpdating && (
            <div className="rounded-lg border border-blue-500/30 bg-blue-500/5 p-4">
              <p className="mb-2 text-sm font-medium text-blue-500">Updating…</p>
              <Progress value={65} className="h-1.5" />
              <p className="mt-2 text-xs text-muted-foreground">
                Do not close this window.
              </p>
            </div>
          )}

          <div>
            <p className="mb-3 text-xs font-medium uppercase tracking-wide text-muted-foreground">
              What's new
            </p>
            <div className="space-y-3">
              {newerVersions.map((v, i) => (
                <div key={v.version} className="rounded-lg border p-3">
                  <div className="mb-1 flex items-center justify-between">
                    <span className="font-mono text-sm font-medium">v{v.version}</span>
                    <span className="text-xs text-muted-foreground">{v.releaseDate}</span>
                  </div>
                  <p className="text-sm text-muted-foreground">{v.changelog}</p>
                  {i === 0 && newerVersions.length > 1 && (
                    <p className="mt-2 text-xs text-muted-foreground">
                      + {newerVersions.length - 1} version
                      {newerVersions.length - 1 === 1 ? "" : "s"} since your install
                    </p>
                  )}
                </div>
              ))}
            </div>
          </div>

          <div className="flex flex-col gap-2 pb-6">
            <Button
              className="w-full bg-amber-500 text-black hover:bg-amber-400"
              disabled={isUpdating}
              onClick={() => void update(live.id)}
            >
              <RefreshCw className="mr-2 h-4 w-4" />
              Update now
            </Button>
            <Button
              variant="ghost"
              className="w-full"
              onClick={() => {
                onOpenChange(false);
                onViewHistory?.(live);
              }}
            >
              View full version history
            </Button>
          </div>
        </div>
      </SheetContent>
    </Sheet>
  );
}
