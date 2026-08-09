import { useEffect } from "react"
import { Link } from "react-router"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { CupSoda, Loader2, Terminal } from "lucide-react"
import { toast } from "sonner"

import { Button } from "@/components/ui/button"
import { getRequestErrorMessage } from "@/lib/network"

import {
  BREW_STATUS_KEY,
  getBrewStatus,
  startBrewBootstrap,
  type BrewStatus,
} from "./api"

type BrewInstallGateProps = {
  status: BrewStatus
  onReady: () => void
}

export function BrewInstallGate({ status, onReady }: BrewInstallGateProps) {
  const queryClient = useQueryClient()
  const boot = status.bootstrap

  const poll = useQuery({
    queryKey: [BREW_STATUS_KEY, "gate"],
    queryFn: getBrewStatus,
    refetchInterval: (q) => {
      const d = q.state.data?.data
      if (d?.binary_present) return false
      if (d?.bootstrap?.running) return 2000
      return 5000
    },
  })

  useEffect(() => {
    if (poll.data?.data?.binary_present) {
      onReady()
    }
  }, [poll.data?.data?.binary_present, onReady])

  const bootstrapMutation = useMutation({
    mutationFn: startBrewBootstrap,
    onSuccess: () => {
      toast.success("Homebrew install started")
      void queryClient.invalidateQueries({ queryKey: [BREW_STATUS_KEY] })
    },
    onError: (err) => {
      toast.error(getRequestErrorMessage(err, "Failed to start Homebrew install"))
    },
  })

  const live = poll.data?.data ?? status
  const running = Boolean(live.bootstrap?.running || boot?.running)
  const log = live.bootstrap?.log || boot?.log || ""

  return (
    <div className="mx-auto flex max-w-2xl flex-col gap-6 py-10">
      <div className="flex items-start gap-4">
        <div className="flex size-12 items-center justify-center rounded-xl bg-muted">
          <CupSoda className="size-6 text-muted-foreground" />
        </div>
        <div className="space-y-1">
          <h1 className="text-xl font-semibold tracking-tight">
            Install Homebrew
          </h1>
          <p className="text-sm text-muted-foreground">
            Brew Manager needs Homebrew on this host. Enabling Brew Package
            starts the official installer from{" "}
            <a
              href="https://brew.sh/"
              target="_blank"
              rel="noreferrer"
              className="underline underline-offset-2"
            >
              brew.sh
            </a>
            .
          </p>
        </div>
      </div>

      <div className="flex flex-wrap gap-2">
        <Button
          type="button"
          disabled={running || bootstrapMutation.isPending || live.binary_present}
          onClick={() => bootstrapMutation.mutate()}
        >
          {running || bootstrapMutation.isPending ? (
            <Loader2 className="size-4 animate-spin" />
          ) : (
            <CupSoda className="size-4" />
          )}
          {running ? "Installing…" : "Install Homebrew"}
        </Button>
        <Button type="button" variant="outline" asChild>
          <Link to="/settings">Back to Settings</Link>
        </Button>
      </div>

      {live.bootstrap?.error ? (
        <p className="text-sm text-destructive">{live.bootstrap.error}</p>
      ) : null}

      {log ? (
        <div className="overflow-hidden rounded-xl border bg-card">
          <div className="flex items-center gap-2 border-b px-3 py-2 text-xs text-muted-foreground">
            <Terminal className="size-3.5" />
            Install log
          </div>
          <pre className="max-h-80 overflow-auto p-3 font-mono text-xs leading-relaxed whitespace-pre-wrap">
            {log}
          </pre>
        </div>
      ) : null}
    </div>
  )
}
