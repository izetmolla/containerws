import { Dot, icons } from "lucide-react"
import type { ComponentType, SVGProps } from "react"

type IconPropsBase = {
  className?: string
  stroke?: number
}

type IconProps =
  | (IconPropsBase & { svgd: string; name?: string })
  | (IconPropsBase & { name: string; svgd?: never })

type IconType = ComponentType<SVGProps<SVGSVGElement>>

const iconMap = icons as Record<string, IconType>

export function Icon({ name, className, svgd, stroke = 2 }: IconProps) {
  if (svgd) {
    return (
      <svg
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        strokeWidth={stroke}
        strokeLinecap="round"
        strokeLinejoin="round"
        className={className || "h-3.5 w-3.5"}
      >
        <path d={svgd} />
      </svg>
    )
  }

  const iconName = name ?? ""
  if (!iconName) return <Dot className={className} />

  const LucideIcon = iconMap[iconName]
  if (!LucideIcon) return <Dot className={className} />

  return <LucideIcon className={className} strokeWidth={stroke} />
}
