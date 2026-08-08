import { forwardRef, useRef, type ReactNode } from "react"
import { useTranslation } from "react-i18next"
import { Building2, CalendarDays, Shield, Users } from "lucide-react"

import { AnimatedBeam } from "@/components/ui/animated-beam" 
import { DotPattern } from "@/components/ui/dot-pattern"
import { cn } from "@/lib/utils" 

const BeamNode = forwardRef<
  HTMLDivElement,
  { className?: string; children?: ReactNode }
>(({ className, children }, ref) => (
  <div
    ref={ref}
    className={cn(
      "z-10 flex size-10 items-center justify-center rounded-full border border-white/10 sm:size-11 lg:size-12",
      "bg-white/5 p-2.5 shadow-[0_0_24px_-12px_rgba(99,102,241,0.6)] backdrop-blur-sm sm:p-3",
      className
    )}
  >
    {children}
  </div>
))
BeamNode.displayName = "BeamNode"

export function AuthHeroPanel() {
  const { t } = useTranslation("authorization")
  const containerRef = useRef<HTMLDivElement>(null)
  const centerRef = useRef<HTMLDivElement>(null)
  const usersRef = useRef<HTMLDivElement>(null)
  const calendarRef = useRef<HTMLDivElement>(null)
  const shieldRef = useRef<HTMLDivElement>(null)
  const buildingRef = useRef<HTMLDivElement>(null)

  return (
    <div className="relative flex h-full min-h-0 w-full flex-col overflow-hidden bg-zinc-950">
      <DotPattern
        className="[mask-image:radial-gradient(480px_circle_at_center,white,transparent)] text-white/20 lg:[mask-image:radial-gradient(560px_circle_at_center,white,transparent)]"
        width={20}
        height={20}
        cr={1}
      />

      <div className="absolute inset-0 bg-linear-to-br from-indigo-950/40 via-zinc-950 to-zinc-950" />

      <div className="relative z-10 flex min-h-0 flex-1 flex-col justify-center px-6 py-8 lg:px-8 xl:px-12">
        <div className="mb-6 lg:mb-8 xl:mb-10">
          <p className="text-[11px] font-medium tracking-widest text-indigo-400 uppercase sm:text-xs">
            {t("Human Resources")}
          </p>
          <h2 className="mt-2 text-2xl font-semibold tracking-tight text-white lg:text-3xl">
            {t("Manage your workforce")}
            <br />
            <span className="text-zinc-400">{t("with confidence.")}</span>
          </h2>
          <p className="mt-3 max-w-sm text-sm leading-relaxed text-zinc-500 lg:mt-4">
            {t(
              "Streamline onboarding, time tracking, and document management — all connected in one secure platform."
            )}
          </p>
        </div>

        <div
          ref={containerRef}
          className="relative mx-auto flex h-[200px] w-full max-w-md items-center justify-center sm:h-[240px] lg:h-[260px] xl:h-[280px]"
        >
          <div className="flex size-full flex-col items-stretch justify-between">
            <div className="flex flex-row items-center justify-between px-2 sm:px-4">
              <BeamNode ref={usersRef}>
                <Users
                  className="size-4 text-indigo-300 sm:size-5"
                  strokeWidth={1.5}
                />
              </BeamNode>
              <BeamNode ref={calendarRef}>
                <CalendarDays
                  className="size-4 text-violet-300 sm:size-5"
                  strokeWidth={1.5}
                />
              </BeamNode>
            </div>

            <div className="flex flex-row items-center justify-center">
              <BeamNode
                ref={centerRef}
                className="size-14 border-indigo-500/30 bg-indigo-500/10 sm:size-16"
              >
                <span className="text-base font-bold text-white sm:text-lg">
                  H
                </span>
              </BeamNode>
            </div>

            <div className="flex flex-row items-center justify-between px-2 sm:px-4">
              <BeamNode ref={shieldRef}>
                <Shield
                  className="size-4 text-emerald-300 sm:size-5"
                  strokeWidth={1.5}
                />
              </BeamNode>
              <BeamNode ref={buildingRef}>
                <Building2
                  className="size-4 text-sky-300 sm:size-5"
                  strokeWidth={1.5}
                />
              </BeamNode>
            </div>
          </div>

          <AnimatedBeam
            containerRef={containerRef}
            fromRef={usersRef}
            toRef={centerRef}
            curvature={-60}
            gradientStartColor="#818cf8"
            gradientStopColor="#6366f1"
            pathColor="#6366f1"
            pathOpacity={0.15}
            duration={4}
          />
          <AnimatedBeam
            containerRef={containerRef}
            fromRef={calendarRef}
            toRef={centerRef}
            curvature={-60}
            gradientStartColor="#a78bfa"
            gradientStopColor="#8b5cf6"
            pathColor="#8b5cf6"
            pathOpacity={0.15}
            duration={4}
            delay={1}
            reverse
          />
          <AnimatedBeam
            containerRef={containerRef}
            fromRef={shieldRef}
            toRef={centerRef}
            curvature={60}
            gradientStartColor="#34d399"
            gradientStopColor="#10b981"
            pathColor="#10b981"
            pathOpacity={0.15}
            duration={4}
            delay={0.5}
          />
          <AnimatedBeam
            containerRef={containerRef}
            fromRef={buildingRef}
            toRef={centerRef}
            curvature={60}
            gradientStartColor="#38bdf8"
            gradientStopColor="#0ea5e9"
            pathColor="#0ea5e9"
            pathOpacity={0.15}
            duration={4}
            delay={1.5}
            reverse
          />
        </div>
      </div>

      <div className="relative z-10 shrink-0 border-t border-white/5 px-6 py-5 lg:px-8 lg:py-6 xl:px-12">
        <p className="text-sm font-medium text-zinc-400">
          {t("Admin console")}
        </p>
        <p className="mt-0.5 text-xs text-zinc-600">
          {t("Trusted by teams worldwide for workforce management.")}
        </p>
      </div>
    </div>
  )
}
