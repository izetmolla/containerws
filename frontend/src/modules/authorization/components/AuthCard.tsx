import type { ReactNode } from "react"

import { BorderBeam } from "@/components/ui/border-beam"
import { cn } from "@/lib/utils" 

interface AuthCardProps {
  children: ReactNode
  className?: string
}

export function AuthCard({ children, className }: AuthCardProps) {
  return (
    <div
      className={cn(
        "relative overflow-hidden rounded-lg border border-border/60 bg-card p-8 shadow-lg sm:rounded-xl",
        className
      )}
    >
      {children}
      <BorderBeam
        size={200}
        duration={10}
        colorFrom="#6366f1"
        colorTo="#a78bfa"
        borderWidth={1}
      />
      <BorderBeam
        size={200}
        duration={10}
        delay={5}
        colorFrom="#818cf8"
        colorTo="#c4b5fd"
        borderWidth={1}
        reverse
      />
    </div>
  )
}
