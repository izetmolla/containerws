import { Home, LifeBuoy, Lock, ShieldAlert, ShieldOff } from "lucide-react"
import type { FC } from "react"
import { Link } from "react-router"
import { Button } from "@/components/ui/button"
import { cn } from "@/lib/utils"
import useAuthorizationStore from "@/store/authorization"

const SUPPORT_EMAIL = "support@uet.edu.al"

const Unauthorized401: FC = () => {
  const clearAccessDenied = useAuthorizationStore((s) => s.clearAccessDenied)

  const handleGoHome = () => {
    clearAccessDenied()
  }
  return (
    <section className="unauthorized-401 relative flex min-h-[min(100vh,900px)] w-full flex-col items-center justify-center overflow-hidden px-6 py-16">
      <style>{`
                @keyframes unauthorized-float {
                    0%, 100% { transform: translateY(0px); }
                    50% { transform: translateY(-10px); }
                }
                @keyframes unauthorized-ring {
                    from { transform: rotate(0deg); }
                    to { transform: rotate(360deg); }
                }
                @keyframes unauthorized-ring-reverse {
                    from { transform: rotate(360deg); }
                    to { transform: rotate(0deg); }
                }
                @keyframes unauthorized-pulse-ring {
                    0%, 100% { transform: scale(1); opacity: 0.35; }
                    50% { transform: scale(1.06); opacity: 0.65; }
                }
                @keyframes unauthorized-shimmer {
                    0% { background-position: 200% center; }
                    100% { background-position: -200% center; }
                }
                @keyframes unauthorized-fade-up {
                    from { opacity: 0; transform: translateY(20px); }
                    to { opacity: 1; transform: translateY(0); }
                }
                @keyframes unauthorized-orbit {
                    from { transform: rotate(0deg) translateX(72px) rotate(0deg); }
                    to { transform: rotate(360deg) translateX(72px) rotate(-360deg); }
                }
                .unauthorized-401 .icon-float {
                    animation: unauthorized-float 4s ease-in-out infinite;
                }
                .unauthorized-401 .ring-spin {
                    animation: unauthorized-ring 18s linear infinite;
                }
                .unauthorized-401 .ring-spin-reverse {
                    animation: unauthorized-ring-reverse 14s linear infinite;
                }
                .unauthorized-401 .pulse-ring {
                    animation: unauthorized-pulse-ring 3s ease-in-out infinite;
                }
                .unauthorized-401 .fade-up {
                    animation: unauthorized-fade-up 0.7s ease-out both;
                }
                .unauthorized-401 .fade-up-delay-1 {
                    animation: unauthorized-fade-up 0.7s ease-out 0.12s both;
                }
                .unauthorized-401 .fade-up-delay-2 {
                    animation: unauthorized-fade-up 0.7s ease-out 0.24s both;
                }
                .unauthorized-401 .code-shimmer {
                    background-size: 200% auto;
                    animation: unauthorized-shimmer 4s linear infinite;
                }
                .unauthorized-401 .orbit-dot {
                    animation: unauthorized-orbit 8s linear infinite;
                }
                .unauthorized-401 .orbit-dot-delay {
                    animation: unauthorized-orbit 8s linear infinite;
                    animation-delay: -2.6s;
                }
                .unauthorized-401 .orbit-dot-delay-2 {
                    animation: unauthorized-orbit 8s linear infinite;
                    animation-delay: -5.2s;
                }
            `}</style>

      {/* Ambient background */}
      <div className="pointer-events-none absolute inset-0" aria-hidden>
        <div className="absolute -top-24 left-1/4 size-72 rounded-full bg-primary/20 blur-3xl" />
        <div className="absolute right-1/4 -bottom-20 size-80 rounded-full bg-destructive/15 blur-3xl" />
        <div className="absolute top-1/2 left-1/2 size-96 -translate-x-1/2 -translate-y-1/2 rounded-full bg-primary/10 blur-3xl" />
        <div
          className="absolute inset-0 opacity-[0.35] dark:opacity-[0.2]"
          style={{
            backgroundImage:
              "radial-gradient(circle at 1px 1px, var(--border) 1px, transparent 0)",
            backgroundSize: "28px 28px",
          }}
        />
      </div>

      <div className="relative z-10 flex max-w-lg flex-col items-center text-center">
        {/* Icon cluster */}
        <div className="fade-up relative mb-10 flex size-44 items-center justify-center">
          <div className="pulse-ring absolute inset-2 rounded-full border-2 border-dashed border-destructive/30" />
          <div className="ring-spin absolute inset-0 rounded-full border border-primary/25" />
          <div className="ring-spin-reverse absolute inset-4 rounded-full border border-dashed border-destructive/20" />

          <span className="orbit-dot absolute top-1/2 left-1/2 size-2 -translate-x-1/2 -translate-y-1/2 rounded-full bg-primary shadow-[0_0_12px_var(--primary)]" />
          <span className="orbit-dot-delay absolute top-1/2 left-1/2 size-1.5 -translate-x-1/2 -translate-y-1/2 rounded-full bg-destructive/80" />
          <span className="orbit-dot-delay-2 absolute top-1/2 left-1/2 size-1.5 -translate-x-1/2 -translate-y-1/2 rounded-full bg-muted-foreground/60" />

          <div className="icon-float relative flex size-28 items-center justify-center rounded-2xl border border-border/80 bg-card shadow-xl ring-1 ring-primary/10">
            <ShieldOff
              className="absolute size-16 stroke-[1.25] text-destructive/90"
              aria-hidden
            />
            <div className="absolute -right-1 -bottom-1 flex size-11 items-center justify-center rounded-xl border border-destructive/20 bg-destructive/10 shadow-md">
              <Lock className="size-5 text-destructive" aria-hidden />
            </div>
            <ShieldAlert
              className="absolute -top-2 -left-2 size-8 text-primary/40"
              aria-hidden
            />
          </div>
        </div>

        <p
          className={cn(
            "code-shimmer fade-up-delay-1 bg-gradient-to-r from-foreground via-primary to-destructive bg-clip-text text-7xl font-bold tracking-tighter text-transparent sm:text-8xl"
          )}
          aria-hidden
        >
          401
        </p>

        <h1 className="fade-up-delay-1 mt-4 text-2xl font-semibold tracking-tight sm:text-3xl">
          Unauthorized access
        </h1>

        <p className="fade-up-delay-2 mt-3 max-w-md text-sm leading-relaxed text-muted-foreground sm:text-base">
          You don&apos;t have permission to view this page. Sign in with the
          right account or ask an administrator to grant you access.
        </p>

        <div className="fade-up-delay-2 mt-10 flex w-full flex-col gap-3 sm:w-auto sm:flex-row sm:justify-center">
          <Button asChild size="lg" className="gap-2 shadow-md">
            <Link to="/" onClick={handleGoHome}>
              <Home className="size-4" aria-hidden />
              Go to home
            </Link>
          </Button>
          <Button
            asChild
            variant="outline"
            size="lg"
            className="gap-2 bg-background/80"
          >
            <a
              href={`mailto:${SUPPORT_EMAIL}?subject=Access%20request%20(401)`}
            >
              <LifeBuoy className="size-4" aria-hidden />
              Contact admin support
            </a>
          </Button>
        </div>

        <p className="fade-up-delay-2 mt-6 text-xs text-muted-foreground">
          Error code{" "}
          <span className="rounded-md bg-muted px-1.5 py-0.5 font-mono">
            401
          </span>
          {" · "}
          <a
            href={`mailto:${SUPPORT_EMAIL}`}
            className="text-primary hover:underline"
          >
            {SUPPORT_EMAIL}
          </a>
        </p>
      </div>
    </section>
  )
}

export default Unauthorized401
