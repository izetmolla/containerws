import { NavLink, Outlet, useParams } from "react-router"
import { useQuery, useQueryClient } from "@tanstack/react-query"
import {
  AppWindow,
  FileText,
  HardDrive,
  KeyRound,
  Monitor,
  Terminal,
  User as UserIcon,
} from "lucide-react"

import ContentLoader from "@/components/content-loader"
import { Avatar, AvatarFallback } from "@/components/ui/avatar"
import { withError } from "@/lib/network"
import { cn, generateAvatarFallback } from "@/lib/utils"

import { getUser, USERS_FETCH_KEY } from "../list/api"
import type { UserSingleOutletContext } from "./types"

const tabs = [
  { to: ".", end: true, label: "General", icon: UserIcon },
  { to: "vnc", end: false, label: "VNC & NoVNC", icon: Monitor },
  { to: "shell", end: false, label: "Terminal", icon: Terminal },
  { to: "novnc", end: false, label: "Desktop", icon: AppWindow },
  { to: "keys", end: false, label: "SSH Keys", icon: KeyRound },
  { to: "logs", end: false, label: "Logs", icon: FileText },
  { to: "storage", end: false, label: "Storage", icon: HardDrive },
] as const

function statusTone(status: string) {
  if (status === "active") {
    return {
      pill: "bg-emerald-500/12 text-emerald-700 ring-emerald-500/20 dark:text-emerald-300",
      dot: "bg-emerald-500",
    }
  }
  if (status === "disabled" || status === "suspended") {
    return {
      pill: "bg-amber-500/12 text-amber-800 ring-amber-500/20 dark:text-amber-300",
      dot: "bg-amber-500",
    }
  }
  return {
    pill: "bg-muted text-muted-foreground ring-border",
    dot: "bg-muted-foreground",
  }
}

export default function UserSingleLayout() {
  const { id = "" } = useParams()
  const queryClient = useQueryClient()

  const detailQuery = useQuery({
    queryKey: [USERS_FETCH_KEY, "detail", id],
    queryFn: () => getUser(id),
    enabled: !!id,
  })

  const user = detailQuery.data?.data
  const title =
    user?.full_name?.trim() || user?.username || user?.email || "User"

  const invalidate = () => {
    void queryClient.invalidateQueries({ queryKey: [USERS_FETCH_KEY] })
  }

  const outletContext: UserSingleOutletContext | null = user
    ? { user, id, invalidate }
    : null

  const subtitle = [user?.username, user?.email]
    .filter(Boolean)
    .filter((v, i, arr) => arr.indexOf(v) === i && v !== title)
    .join(" · ")

  const desktopLive = Boolean(user?.vnc?.live)
  const tone = statusTone(user?.status || "unknown")

  return (
    <ContentLoader
      title={title}
      breadcrumb={[
        { label: "Users", to: "/users" },
        { label: title },
      ]}
      isLoading={detailQuery.isLoading}
      error={withError(detailQuery.error, detailQuery.data)}
      showHeaderSeparator={false}
      headerClassName="mb-5"
      customTitle={
        user ? (
          <div className="flex min-w-0 items-center gap-3.5">
            <Avatar className="size-11 rounded-xl shadow-sm ring-1 ring-border after:rounded-xl">
              <AvatarFallback className="rounded-xl bg-gradient-to-br from-primary/15 to-primary/5 text-sm font-semibold tracking-tight text-primary">
                {generateAvatarFallback(title)}
              </AvatarFallback>
            </Avatar>
            <div className="min-w-0 space-y-1">
              <div className="flex min-w-0 flex-wrap items-center gap-2">
                <h2 className="truncate text-xl font-semibold tracking-tight">
                  {title}
                </h2>
                <span
                  className={cn(
                    "inline-flex items-center gap-1.5 rounded-full px-2 py-0.5 text-[11px] font-medium capitalize ring-1 ring-inset",
                    tone.pill
                  )}
                >
                  <span
                    className={cn("size-1.5 rounded-full", tone.dot)}
                    aria-hidden
                  />
                  {user.status || "unknown"}
                </span>
                {user.has_vnc || user.vnc ? (
                  <span
                    className={cn(
                      "inline-flex items-center gap-1.5 rounded-full px-2 py-0.5 text-[11px] font-medium ring-1 ring-inset",
                      desktopLive
                        ? "bg-sky-500/12 text-sky-800 ring-sky-500/20 dark:text-sky-300"
                        : "bg-muted/80 text-muted-foreground ring-border"
                    )}
                  >
                    <span
                      className={cn(
                        "size-1.5 rounded-full",
                        desktopLive ? "bg-sky-500" : "bg-muted-foreground/50"
                      )}
                      aria-hidden
                    />
                    {desktopLive ? "Desktop live" : "Desktop stopped"}
                  </span>
                ) : null}
              </div>
              {subtitle ? (
                <p className="truncate text-sm text-muted-foreground">
                  {subtitle}
                </p>
              ) : null}
            </div>
          </div>
        ) : (
          <h2 className="text-xl font-semibold tracking-tight">{title}</h2>
        )
      }
    >
      {user && outletContext ? (
        <div className="flex flex-col gap-5">
          <nav
            className="flex gap-0.5 overflow-x-auto overflow-y-hidden rounded-lg border bg-card/60 p-1 shadow-sm"
            aria-label="User sections"
          >
            {tabs.map((tab) => {
              const Icon = tab.icon
              return (
                <NavLink
                  key={tab.to}
                  to={tab.to}
                  end={tab.end}
                  className={({ isActive }) =>
                    cn(
                      "inline-flex shrink-0 items-center gap-1.5 rounded-md px-3 py-2 text-sm font-medium transition-colors",
                      isActive
                        ? "bg-background text-foreground shadow-sm ring-1 ring-border"
                        : "text-muted-foreground hover:bg-muted/60 hover:text-foreground"
                    )
                  }
                >
                  <Icon className="size-3.5 opacity-80" aria-hidden />
                  {tab.label}
                </NavLink>
              )
            })}
          </nav>

          <Outlet context={outletContext} />
        </div>
      ) : null}
    </ContentLoader>
  )
}
