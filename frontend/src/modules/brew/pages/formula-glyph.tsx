import { useState } from "react"

import { cn } from "@/lib/utils"

const CATEGORY_TONE: Record<string, string> = {
  Databases: "from-sky-500/20 to-sky-600/10 text-sky-700 dark:text-sky-300",
  Languages:
    "from-violet-500/20 to-violet-600/10 text-violet-700 dark:text-violet-300",
  "Developer Tools":
    "from-emerald-500/20 to-emerald-600/10 text-emerald-700 dark:text-emerald-300",
  Networking:
    "from-blue-500/20 to-blue-600/10 text-blue-700 dark:text-blue-300",
  Media: "from-rose-500/20 to-rose-600/10 text-rose-700 dark:text-rose-300",
  Security:
    "from-amber-500/20 to-amber-600/10 text-amber-800 dark:text-amber-300",
  Monitoring:
    "from-teal-500/20 to-teal-600/10 text-teal-700 dark:text-teal-300",
  "Desktop Apps":
    "from-indigo-500/20 to-indigo-600/10 text-indigo-700 dark:text-indigo-300",
  Utilities:
    "from-orange-500/20 to-orange-600/10 text-orange-800 dark:text-orange-300",
}

/** Official Homebrew mark (brew.sh brand asset). */
export function HomebrewMark({
  className,
  title = "Homebrew",
}: {
  className?: string
  title?: string
}) {
  return (
    <img
      src="https://brew.sh/assets/img/homebrew.svg"
      alt={title}
      className={cn("object-contain", className)}
      loading="lazy"
      referrerPolicy="no-referrer"
    />
  )
}

/** Formula logo: homepage favicon/logo with monogram fallback. */
export function FormulaGlyph({
  name,
  icon,
  logo,
  category,
  className,
  size = "md",
}: {
  name: string
  icon?: string | null
  logo?: string | null
  category?: string | null
  className?: string
  size?: "sm" | "md" | "lg" | "xl"
}) {
  const candidates = [logo, icon]
    .map((v) => (v ?? "").trim())
    .filter(Boolean)
  const [index, setIndex] = useState(0)
  const src = candidates[index] ?? ""
  const letter = (name.trim().charAt(0) || "?").toUpperCase()
  const tone =
    CATEGORY_TONE[category ?? ""] ??
    "from-muted to-muted/60 text-muted-foreground"
  const box =
    size === "xl"
      ? "size-20 rounded-2xl"
      : size === "lg"
        ? "size-14 rounded-2xl"
        : size === "sm"
          ? "size-8 rounded-lg"
          : "size-10 rounded-xl"
  const img =
    size === "xl"
      ? "size-12"
      : size === "lg"
        ? "size-8"
        : size === "sm"
          ? "size-4"
          : "size-5"

  if (src) {
    return (
      <div
        className={cn(
          "flex shrink-0 items-center justify-center overflow-hidden border bg-background shadow-xs",
          box,
          className
        )}
      >
        <img
          key={src}
          src={src}
          alt=""
          className={cn("object-contain", img)}
          loading="lazy"
          referrerPolicy="no-referrer"
          onError={() => setIndex((i) => i + 1)}
        />
      </div>
    )
  }

  return (
    <div
      className={cn(
        "flex shrink-0 items-center justify-center border bg-gradient-to-br font-semibold shadow-xs",
        box,
        tone,
        size === "xl"
          ? "text-3xl"
          : size === "lg"
            ? "text-xl"
            : size === "sm"
              ? "text-xs"
              : "text-sm",
        className
      )}
      aria-hidden
    >
      {letter}
    </div>
  )
}
