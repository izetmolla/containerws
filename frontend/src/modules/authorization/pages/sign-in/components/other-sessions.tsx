import { useState } from "react"
import { useTranslation } from "react-i18next"
import { MoreVertical, Plus } from "lucide-react"
import { toast } from "sonner"

import {
  Avatar,
  AvatarFallback,
  AvatarImage,
} from "@/components/ui/avatar"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { getRequestErrorMessage } from "@/lib/network/errors"
import { removeStoredSession, verifyStoredSession } from "@/lib/switch-account"
import { generateAvatarFallback } from "@/lib/utils"
import { cn } from "@/lib/utils"
import useAuthorizationStore from "@/store/authorization"
import type { AuthSession } from "@/types"
import { AuthHeader } from "../../../components/layout"

function getSessionDisplayName(session: AuthSession) {
  return [session.user?.first_name, session.user?.last_name]
    .filter(Boolean)
    .join(" ")
    .trim()
}

type OtherSessionsProps = {
  onSelectSession?: (sessionId: string) => void
  onUseAnotherAccount?: () => void
  onSessionInvalid?: (sessionId: string) => void
  className?: string
}

export function OtherSessions({
  onSelectSession,
  onUseAnotherAccount,
  onSessionInvalid,
  className,
}: OtherSessionsProps) {
  const { t } = useTranslation("authorization")
  const [checkingSessionId, setCheckingSessionId] = useState<string | null>(
    null
  )
  const sessions = useAuthorizationStore((state) => state.sessions)
  const currentSession = useAuthorizationStore((state) => state.current_session)

  const handleSelectSession = async (session: AuthSession) => {
    const previousSessionId = currentSession
    setCheckingSessionId(session.session_id)

    try {
      const result = await verifyStoredSession(session, previousSessionId)

      if (result.ok) {
        onSelectSession?.(session.session_id)
        return
      }

      onSessionInvalid?.(session.session_id)
      toast.error(t("This account session has expired. Please sign in again."))
    } catch (error) {
      onSessionInvalid?.(session.session_id)
      toast.error(
        getRequestErrorMessage(
          error,
          t("This account session has expired. Please sign in again.")
        )
      )
    } finally {
      setCheckingSessionId(null)
    }
  }

  if (sessions.length === 0) {
    return null
  }

  const enableSessionScroll = sessions.length > 3

  return (
    <div className={cn("text-foreground", className)}>
      <AuthHeader
        title={t("Pick an account")}
        subtitle={t(
          "Choose a saved account to continue on Container Workspace Platform"
        )}
        compact
      />

      <div className="overflow-hidden rounded-lg border border-border bg-card/40">
        <div
          className={cn(
            "divide-y divide-border",
            enableSessionScroll &&
              "max-h-[9.75rem] overflow-y-auto overscroll-y-contain [scrollbar-gutter:stable]"
          )}
        >
          {sessions.map((session) => {
            const name = getSessionDisplayName(session)
            const isCurrent = session.session_id === currentSession
            const isChecking = checkingSessionId === session.session_id

            return (
              <div
                key={session.session_id}
                className={cn(
                  "group relative min-h-[3.25rem]",
                  isCurrent ? "bg-muted/80" : "hover:bg-muted/50"
                )}
              >
                <button
                  type="button"
                  disabled={Boolean(checkingSessionId)}
                  onClick={() => void handleSelectSession(session)}
                  className="flex w-full items-center gap-2.5 px-2.5 py-2 pr-9 text-left transition-colors disabled:cursor-wait disabled:opacity-70"
                >
                  <Avatar className="size-8 shrink-0 rounded-md">
                    <AvatarImage
                      src={session.user?.image}
                      alt={name || session.user.email}
                    />
                    <AvatarFallback className="rounded-md bg-primary/10 text-[10px] font-medium text-primary">
                      {generateAvatarFallback(name || session.user.email)}
                    </AvatarFallback>
                  </Avatar>

                  <span className="min-w-0 flex-1">
                    <span className="block truncate text-xs leading-tight font-semibold text-foreground">
                      {name || session.user.email}
                    </span>
                    <span className="mt-px block truncate text-[11px] leading-tight text-muted-foreground">
                      {session.user.email}
                    </span>
                    {isChecking ? (
                      <span className="mt-px block text-[11px] leading-tight text-muted-foreground">
                        {t("Checking session…")}
                      </span>
                    ) : isCurrent ? (
                      <span className="mt-px block text-[11px] leading-tight text-muted-foreground">
                        {t("Signed in")}
                      </span>
                    ) : null}
                  </span>
                </button>

                <DropdownMenu>
                  <DropdownMenuTrigger asChild>
                    <button
                      type="button"
                      disabled={Boolean(checkingSessionId)}
                      className={cn(
                        "absolute top-1/2 right-1.5 flex size-7 -translate-y-1/2 items-center justify-center rounded-md",
                        "text-muted-foreground transition-colors hover:bg-muted hover:text-foreground",
                        "focus-visible:ring-1 focus-visible:ring-ring focus-visible:outline-none",
                        "disabled:cursor-not-allowed disabled:opacity-50"
                      )}
                      aria-label={t("Options for {{name}}", {
                        name: name || session.user.email,
                      })}
                      onClick={(event) => event.stopPropagation()}
                    >
                      <MoreVertical className="size-3.5" />
                    </button>
                  </DropdownMenuTrigger>
                  <DropdownMenuContent align="end" className="min-w-36">
                    <DropdownMenuItem
                      className="cursor-pointer text-xs"
                      onClick={() => removeStoredSession(session.session_id)}
                    >
                      {t("Sign out")}
                    </DropdownMenuItem>
                  </DropdownMenuContent>
                </DropdownMenu>
              </div>
            )
          })}
        </div>

        <button
          type="button"
          disabled={Boolean(checkingSessionId)}
          onClick={onUseAnotherAccount}
          className="flex w-full items-center gap-2.5 border-t border-border px-2.5 py-2 text-left transition-colors hover:bg-muted/50 disabled:cursor-not-allowed disabled:opacity-70"
        >
          <span className="flex size-8 shrink-0 items-center justify-center rounded-full bg-muted">
            <Plus className="size-3.5 text-foreground" aria-hidden="true" />
          </span>
          <span className="text-xs text-foreground">
            {t("Use another account")}
          </span>
        </button>
      </div>
    </div>
  )
}

export default OtherSessions
