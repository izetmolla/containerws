import { cn } from "@/lib/utils"

export const authFieldClassName = cn(
  "w-full rounded-lg border border-input bg-background px-3 text-foreground",
  "transition-all duration-150 ease-out outline-none placeholder:text-muted-foreground",
  "focus:border-ring focus:ring-2 focus:ring-ring/20"
)

export const authFieldErrorClassName =
  "border-destructive focus:border-destructive focus:ring-destructive/20"

export const authLabelClassName = "block text-sm font-medium text-foreground"

/** Touch-friendly on mobile; compact from sm breakpoint up. */
export const authInputClass = "h-11 text-sm sm:h-[42px]"
export const authCompactInputClass = "h-11 text-sm sm:h-[38px]"

/** @deprecated Prefer authInputClass — kept for callers outside authorization. */
export const authInputStyle = { height: "42px", fontSize: "14px" }

/** @deprecated Prefer authCompactInputClass — kept for callers outside authorization. */
export const authCompactInputStyle = { height: "38px", fontSize: "14px" }

export const authFieldItemClass = "gap-1"
export const authFieldLabelClass = "text-[13px] sm:text-[13px]"
export const authCompactButtonClass =
  "mt-1 min-h-11 h-11 text-sm sm:h-[38px] sm:min-h-0"
export const authFooterLinkClass =
  "mt-4 px-1 text-center text-xs leading-relaxed text-muted-foreground sm:px-0"
export const authFormStackClass = "space-y-3"
export const authOAuthStackClass = "mb-3 space-y-3"
