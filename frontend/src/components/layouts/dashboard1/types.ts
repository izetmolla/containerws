import type { ReactNode } from "react"
import type { SearchFn } from "./header/search/api"

export type NavigationItem = {
  title: string
  to: string
  icon?: string
  isExternal?: boolean
  isComing?: boolean
  isDataBadge?: string
  isNew?: boolean
  newTab?: boolean
  prefetch?: "none" | "render" | "intent"
  roles?: string[]
  children?: NavigationItem[]
}

export type BrandConfig = {
  title: string
  description?: string
  logoSrc?: string
  logoAlt?: string
  homeTo?: string
}

export type LayoutUser = {
  firstName?: string
  lastName?: string
  email?: string
  image?: string
}

export type DashboardLayoutProps = {
  children?: ReactNode
  brand?: BrandConfig
  navigations?: NavigationItem[]
  user?: LayoutUser | null
  /** Rendered in the sidebar header (defaults to brand logo + title). */
  sidebarHeader?: ReactNode
  /** Rendered in the sidebar footer (defaults to user menu). */
  sidebarFooter?: ReactNode
  /** Extra actions in the header before notifications. */
  headerActions?: ReactNode
  /** Optional global search backend. When omitted, search UI still shows with empty results. */
  searchFn?: SearchFn
  /** Shown instead of children when access is denied. */
  accessDenied?: boolean
  accessDeniedFallback?: ReactNode
  /** Sign-in redirect path used when `requireAuth` is true and no user is provided. */
  requireAuth?: boolean
  signInPath?: string
  onSignOut?: () => void
}
