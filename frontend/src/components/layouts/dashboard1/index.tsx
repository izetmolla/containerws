"use client"

import type { CSSProperties, ReactNode } from "react"
import { Outlet, useLocation } from "react-router"

import { SidebarInset, SidebarProvider } from "@/components/ui/sidebar"
import { cn } from "@/lib/utils"

import { AppSidebar } from "./app-sidebar"
import { SiteHeader } from "./header"
import type {
  BrandConfig,
  DashboardLayoutProps,
  LayoutUser,
  NavigationItem,
} from "./types"

export type { BrandConfig, DashboardLayoutProps, LayoutUser, NavigationItem }

const DEFAULT_BRAND: BrandConfig = {
  title: "Dashboard",
  description: "Application console",
  homeTo: "/",
}

const layoutStyle = {
  "--sidebar-width": "calc(var(--spacing) * 64)",
  "--header-height": "calc(var(--spacing) * 14)",
  "--content-padding": "calc(var(--spacing) * 4)",
  "--content-margin": "calc(var(--spacing) * 1.5)",
  "--content-full-height":
    "calc(100vh - var(--header-height) - (var(--content-padding) * 2) - (var(--content-margin) * 2))",
} as CSSProperties

/** Routes that always use the full content column (ignore centered theme layout). */
function useFullWidthContent() {
  const { pathname } = useLocation()
  if (pathname === "/") return true
  const fullWidthPrefixes = [
    "/docker",
    "/kubernetes",
    "/proxymanager",
    "/filemanager",
    "/softwares",
    "/users",
    "/vscode",
    "/vnc-novnc",
    "/settings",
    "/cli",
    "/shell",
  ]
  return fullWidthPrefixes.some(
    (prefix) => pathname === prefix || pathname.startsWith(`${prefix}/`),
  )
}

type DashboardLayoutInnerProps = DashboardLayoutProps & {
  brand: BrandConfig
  navigations: NavigationItem[]
}

function DashboardLayoutInner({
  children,
  brand,
  navigations,
  user,
  sidebarHeader,
  sidebarFooter,
  headerActions,
  searchFn,
  accessDenied,
  accessDeniedFallback,
  onSignOut,
}: DashboardLayoutInnerProps) {
  const fullWidthContent = useFullWidthContent()
  const content: ReactNode = accessDenied
    ? (accessDeniedFallback ?? (
        <div className="flex flex-1 items-center justify-center p-8 text-center">
          <div className="space-y-1">
            <h2 className="text-xl font-semibold">Access denied</h2>
            <p className="text-sm text-muted-foreground">
              You do not have permission to view this page.
            </p>
          </div>
        </div>
      ))
    : (children ?? <Outlet />)

  return (
    <SidebarProvider defaultOpen style={layoutStyle}>
      <AppSidebar
        variant="inset"
        brand={brand}
        navigations={navigations}
        user={user}
        header={sidebarHeader}
        footer={sidebarFooter}
        onSignOut={onSignOut}
      />
      <SidebarInset>
        <SiteHeader
          actions={headerActions}
          searchFn={searchFn}
          user={user}
          onSignOut={onSignOut}
        />
        <div className="flex flex-1 flex-col bg-muted/40">
          <div
            className={cn(
              "@container/main w-full min-w-0 p-(--content-padding)",
              !fullWidthContent &&
                "xl:group-data-[theme-content-layout=centered]/layout:container xl:group-data-[theme-content-layout=centered]/layout:mx-auto"
            )}
          >
            {content}
          </div>
        </div>
      </SidebarInset>
    </SidebarProvider>
  )
}

/**
 * Dashboard chrome from the admin console layout (sidebar + header shell).
 *
 * App-specific data (nav, user, search, auth) is provided via props/slots so
 * this package stays reusable across products.
 *
 * @example
 * ```tsx
 * import { DashboardLayout } from "@/components/layouts/dashboard1"
 *
 * <DashboardLayout
 *   brand={{ title: "Container Workspace", description: "Admin" }}
 *   navigations={[{ title: "Main", children: [{ title: "Container Workspace", to: "/", icon: "ContainerWorkspace" }] }]}
 *   user={{ firstName: "Ada", email: "ada@example.com" }}
 * >
 *   <Outlet />
 * </DashboardLayout>
 * ```
 */
export function DashboardLayout({
  brand = DEFAULT_BRAND,
  navigations = [],
  requireAuth = false,
  signInPath = "/sign-in",
  user,
  ...props
}: DashboardLayoutProps) {
  if (requireAuth && !user) {
    if (typeof window !== "undefined") {
      const redirect = encodeURIComponent(window.location.href)
      window.location.replace(`${signInPath}?redirectUrl=${redirect}`)
    }
    return null
  }

  return (
    <DashboardLayoutInner
      brand={brand}
      navigations={navigations}
      user={user}
      {...props}
    />
  )
}

export { BrandHeader } from "./brand-header"
export { NavigationItems } from "./navigations"
export { NavUser } from "./nav-user"
export { SiteHeader } from "./header"
export { default as ThemeSwitch } from "./header/theme-switch"
export { default as UserMenu } from "./header/user-menu"
export { AppSidebar } from "./app-sidebar"
export { Logo } from "./logo"
export { Icon } from "./icon"
export { ThemeProvider } from "./providers/theme-provider"
export type { SearchFn } from "./header/search"
