"use client"

import { Link } from "react-router"
import { useQuery } from "@tanstack/react-query"

import { Button } from "@/components/ui/button"
import { cn } from "@/lib/utils"
import {
  getSoftwareQueue,
  SOFTWARES_FETCH_KEY,
} from "@/modules/softwares/pages/list/api"

/** Header badge — active software install queue count. Hidden when idle. */
export function SoftwareQueueToggle() {
  const queueQuery = useQuery({
    queryKey: [SOFTWARES_FETCH_KEY, "queue"],
    queryFn: getSoftwareQueue,
    refetchInterval: (q) => {
      const pending = q.state.data?.data?.pending ?? 0
      const running = Boolean(q.state.data?.data?.running)
      return pending > 0 || running ? 2_000 : 12_000
    },
    staleTime: 2_000,
  })

  const snap = queueQuery.data?.data
  const activeCount = (snap?.items ?? []).filter(
    (item) => item.status === "pending" || item.status === "running"
  ).length
  const count =
    activeCount > 0
      ? activeCount
      : snap?.pending && snap.pending > 0
        ? snap.pending
        : snap?.running
          ? 1
          : 0

  if (count <= 0) return null

  const label =
    count === 1
      ? "1 software in the install queue"
      : `${count} softwares in the install queue`

  return (
    <Button
      asChild
      type="button"
      size="icon-sm"
      className={cn(
        "size-7 min-w-7 rounded-full border-0 bg-amber-400 px-0 font-semibold text-amber-950 shadow-sm tabular-nums",
        "hover:bg-amber-300 hover:text-amber-950",
        "animate-pulse"
      )}
      title={label}
      aria-label={label}
    >
      <Link to="/softwares/installing">{count > 99 ? "99+" : count}</Link>
    </Button>
  )
}
