import { useTranslation } from "react-i18next"

export function AuthOAuthDivider({ label }: { label?: string }) {
  const { t } = useTranslation("authorization")
  const dividerLabel = label ?? t("or continue with email")

  return (
    <div className="relative flex items-center py-1">
      <div className="grow border-t border-border" aria-hidden="true" />
      <span className="mx-2 max-w-[55%] shrink text-center text-[11px] leading-tight text-muted-foreground sm:mx-3 sm:max-w-none sm:text-xs">
        {dividerLabel}
      </span>
      <div className="grow border-t border-border" aria-hidden="true" />
    </div>
  )
}
