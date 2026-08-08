"use client"

import { useQuery } from "@tanstack/react-query"
import { Link } from "react-router"

import { cn } from "@/lib/utils"
import {
  getUpdateStatus,
  SETTINGS_UPDATE_FETCH_KEY,
} from "@/modules/settings/pages/update/api"

export function SidebarVersionLink({ versionLabel }: { versionLabel: string }) {
  const updateQuery = useQuery({
    queryKey: [SETTINGS_UPDATE_FETCH_KEY, "status"],
    queryFn: getUpdateStatus,
    staleTime: 5 * 60_000,
    refetchInterval: 15 * 60_000,
    retry: 1,
  })

  const updateAvailable = Boolean(updateQuery.data?.data?.update_available)

  return (
    <Link
      to="/settings/update"
      className={cn(
        "mx-2 mb-1 flex items-center justify-center gap-1.5 rounded-md px-2 py-1 text-center text-[11px] italic leading-none text-muted-foreground transition-colors hover:bg-sidebar-accent hover:text-foreground",
        "group-data-[collapsible=icon]:hidden"
      )}
      title={
        updateAvailable
          ? "Update available — open Settings → Update"
          : "Open Settings → Update"
      }
    >
      <span>Version: {versionLabel}</span>
      {updateAvailable ? (
        <span className="rounded-full bg-sky-600 px-1.5 py-0.5 text-[9px] font-semibold not-italic leading-none tracking-wide text-white uppercase">
          New
        </span>
      ) : null}
    </Link>
  )
}
