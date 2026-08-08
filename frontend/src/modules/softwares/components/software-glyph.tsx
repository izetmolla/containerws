import { useState } from "react"
import Icon from "@/components/icon"
import { cn } from "@/lib/utils"
import { icons, Package } from "lucide-react"

/** Renders a software logo `image` URL when present, else Lucide/emoji `icon`. */
export function SoftwareGlyph({
  name,
  image,
  className,
  imgClassName,
}: {
  name?: string | null
  image?: string | null
  className?: string
  imgClassName?: string
}) {
  const [broken, setBroken] = useState(false)
  const src = (image ?? "").trim()
  if (src && !broken) {
    return (
      <img
        src={src}
        alt=""
        className={cn("object-contain", imgClassName ?? className)}
        onError={() => setBroken(true)}
        loading="lazy"
        referrerPolicy="no-referrer"
      />
    )
  }

  const value = (name ?? "").trim()
  if (!value) {
    return <Package className={className} />
  }

  if (value in icons) {
    return <Icon name={value} className={className} />
  }

  // Seeded emoji / short glyph
  if ([...value].length <= 3) {
    return <span className={cn("leading-none", className)}>{value}</span>
  }

  return <Package className={className} />
}
