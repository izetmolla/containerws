import { Compass, Home, LifeBuoy, MapPinOff, SearchX } from "lucide-react"
import type { FC } from "react"
import { Link } from "react-router"
import { Button } from "@/components/ui/button"
import { cn } from "@/lib/utils"

const SUPPORT_EMAIL = "support@uet.edu.al"

const NotFound404: FC = () => {
  return (
    <section className="not-found-404 relative flex min-h-[min(100vh,900px)] w-full flex-col items-center justify-center overflow-hidden px-6 py-16">
      <style>{`
                @keyframes not-found-float {
                    0%, 100% { transform: translateY(0px); }
                    50% { transform: translateY(-10px); }
                }
                @keyframes not-found-ring {
                    from { transform: rotate(0deg); }
                    to { transform: rotate(360deg); }
                }
                @keyframes not-found-ring-reverse {
                    from { transform: rotate(360deg); }
                    to { transform: rotate(0deg); }
                }
                @keyframes not-found-pulse-ring {
                    0%, 100% { transform: scale(1); opacity: 0.35; }
                    50% { transform: scale(1.06); opacity: 0.65; }
                }
                @keyframes not-found-shimmer {
                    0% { background-position: 200% center; }
                    100% { background-position: -200% center; }
                }
                @keyframes not-found-fade-up {
                    from { opacity: 0; transform: translateY(20px); }
                    to { opacity: 1; transform: translateY(0); }
                }
                @keyframes not-found-orbit {
                    from { transform: rotate(0deg) translateX(72px) rotate(0deg); }
                    to { transform: rotate(360deg) translateX(72px) rotate(-360deg); }
                }
                .not-found-404 .icon-float {
                    animation: not-found-float 4s ease-in-out infinite;
                }
                .not-found-404 .ring-spin {
                    animation: not-found-ring 18s linear infinite;
                }
                .not-found-404 .ring-spin-reverse {
                    animation: not-found-ring-reverse 14s linear infinite;
                }
                .not-found-404 .pulse-ring {
                    animation: not-found-pulse-ring 3s ease-in-out infinite;
                }
                .not-found-404 .fade-up {
                    animation: not-found-fade-up 0.7s ease-out both;
                }
                .not-found-404 .fade-up-delay-1 {
                    animation: not-found-fade-up 0.7s ease-out 0.12s both;
                }
                .not-found-404 .fade-up-delay-2 {
                    animation: not-found-fade-up 0.7s ease-out 0.24s both;
                }
                .not-found-404 .code-shimmer {
                    background-size: 200% auto;
                    animation: not-found-shimmer 4s linear infinite;
                }
                .not-found-404 .orbit-dot {
                    animation: not-found-orbit 8s linear infinite;
                }
                .not-found-404 .orbit-dot-delay {
                    animation: not-found-orbit 8s linear infinite;
                    animation-delay: -2.6s;
                }
                .not-found-404 .orbit-dot-delay-2 {
                    animation: not-found-orbit 8s linear infinite;
                    animation-delay: -5.2s;
                }
            `}</style>

      {/* Ambient background */}
      <div className="pointer-events-none absolute inset-0" aria-hidden>
        <div className="absolute -top-24 left-1/4 size-72 rounded-full bg-primary/20 blur-3xl" />
        <div className="absolute right-1/4 -bottom-20 size-80 rounded-full bg-muted-foreground/15 blur-3xl" />
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
          <div className="pulse-ring absolute inset-2 rounded-full border-2 border-dashed border-primary/30" />
          <div className="ring-spin absolute inset-0 rounded-full border border-primary/25" />
          <div className="ring-spin-reverse absolute inset-4 rounded-full border border-dashed border-muted-foreground/20" />

          <span className="orbit-dot absolute top-1/2 left-1/2 size-2 -translate-x-1/2 -translate-y-1/2 rounded-full bg-primary shadow-[0_0_12px_var(--primary)]" />
          <span className="orbit-dot-delay absolute top-1/2 left-1/2 size-1.5 -translate-x-1/2 -translate-y-1/2 rounded-full bg-muted-foreground/80" />
          <span className="orbit-dot-delay-2 absolute top-1/2 left-1/2 size-1.5 -translate-x-1/2 -translate-y-1/2 rounded-full bg-primary/60" />

          <div className="icon-float relative flex size-28 items-center justify-center rounded-2xl border border-border/80 bg-card shadow-xl ring-1 ring-primary/10">
            <MapPinOff
              className="absolute size-16 stroke-[1.25] text-muted-foreground/90"
              aria-hidden
            />
            <div className="absolute -right-1 -bottom-1 flex size-11 items-center justify-center rounded-xl border border-primary/20 bg-primary/10 shadow-md">
              <SearchX className="size-5 text-primary" aria-hidden />
            </div>
            <Compass
              className="absolute -top-2 -left-2 size-8 text-primary/40"
              aria-hidden
            />
          </div>
        </div>

        <p
          className={cn(
            "code-shimmer fade-up-delay-1 bg-gradient-to-r from-foreground via-primary to-muted-foreground bg-clip-text text-7xl font-bold tracking-tighter text-transparent sm:text-8xl"
          )}
          aria-hidden
        >
          404
        </p>

        <h1 className="fade-up-delay-1 mt-4 text-2xl font-semibold tracking-tight sm:text-3xl">
          Page not found
        </h1>

        <p className="fade-up-delay-2 mt-3 max-w-md text-sm leading-relaxed text-muted-foreground sm:text-base">
          The page you&apos;re looking for doesn&apos;t exist, was moved, or the
          URL may be incorrect. Check the address or head back to the home page.
        </p>

        <div className="fade-up-delay-2 mt-10 flex w-full flex-col gap-3 sm:w-auto sm:flex-row sm:justify-center">
          <Button asChild size="lg" className="gap-2 shadow-md">
            <Link to="/">
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
            <a href={`mailto:${SUPPORT_EMAIL}?subject=Broken%20link%20(404)`}>
              <LifeBuoy className="size-4" aria-hidden />
              Report broken link
            </a>
          </Button>
        </div>

        <p className="fade-up-delay-2 mt-6 text-xs text-muted-foreground">
          Error code{" "}
          <span className="rounded-md bg-muted px-1.5 py-0.5 font-mono">
            404
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

export default NotFound404
