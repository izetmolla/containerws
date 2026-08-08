import { type ReactNode } from "react"
import { useTranslation } from "react-i18next"
import { Outlet } from "react-router"

import { DotPattern } from "@/components/ui/dot-pattern"
import { cn } from "@/lib/utils"
import { AuthHeroPanel } from "./AuthHeroPanel"
import { AuthCard } from "./AuthCard"

const AuthorizationLayout = () => {
  const { t } = useTranslation("authorization")

  return (
    <div className="auth-layout flex h-dvh min-h-dvh w-full overflow-hidden bg-background font-sans">
      <div className="relative hidden min-h-0 w-[42%] shrink-0 lg:block xl:w-1/2">
        <AuthHeroPanel />
      </div>

      <div className="relative flex min-h-0 min-w-0 flex-1 overflow-y-auto overscroll-y-contain lg:border-l lg:border-border">
        <DotPattern
          className="pointer-events-none absolute inset-0 [mask-image:radial-gradient(280px_circle_at_center,white,transparent)] text-foreground/5 sm:[mask-image:radial-gradient(360px_circle_at_center,white,transparent)] lg:hidden"
          width={20}
          height={20}
          cr={1}
        />

        <div
          className={cn(
            "relative flex min-h-full w-full items-start justify-center",
            "px-4 pt-[max(2rem,env(safe-area-inset-top,0px))]",
            "pb-[max(2rem,env(safe-area-inset-bottom,0px))]",
            "sm:px-6 sm:pt-[max(2.5rem,env(safe-area-inset-top,0px))] sm:pb-[max(2.5rem,env(safe-area-inset-bottom,0px))]",
            "md:items-center md:py-8",
            "lg:px-8 lg:py-6",
            "xl:px-10"
          )}
        >
          <div className="relative w-full max-w-[min(100%,28rem)] sm:max-w-md lg:max-w-[28rem]">
            <div className="mb-3 flex items-center gap-2.5 sm:mb-4">
              <div className="flex size-8 shrink-0 items-center justify-center rounded-lg bg-primary text-xs font-bold text-primary-foreground sm:size-8">
                H
              </div>
              <span className="text-sm font-medium text-muted-foreground">
                {t("Container Workspace Platform")}
              </span>
            </div>

            <AuthCard className="my-1 rounded-lg px-4 py-6 sm:my-0 sm:rounded-xl sm:px-5 sm:py-7 md:px-6 md:py-8">
              <Outlet />
            </AuthCard>
          </div>
        </div>
      </div>
    </div>
  )
}

export function AuthHeader({
  title,
  subtitle,
  compact = true,
  icon,
}: {
  title: string
  subtitle: string
  compact?: boolean
  icon?: ReactNode
}) {
  const centered = Boolean(icon)

  return (
    <div
      className={cn(
        compact ? "mb-3 sm:mb-4" : "mb-5 sm:mb-6",
        centered && "text-center"
      )}
    >
      {icon ? (
        <div className="mb-3 flex justify-center sm:mb-4">{icon}</div>
      ) : null}
      <h1
        className={cn(
          "font-semibold tracking-tight text-foreground",
          compact ? "text-base sm:text-lg" : "text-lg sm:text-xl",
          icon && "text-lg sm:text-xl"
        )}
      >
        {title}
      </h1>
      <p
        className={cn(
          "text-muted-foreground",
          compact ? "mt-0.5 text-xs sm:text-[13px]" : "mt-1 text-sm",
          centered && "mx-auto max-w-xs"
        )}
      >
        {subtitle}
      </p>
    </div>
  )
}

export default AuthorizationLayout
