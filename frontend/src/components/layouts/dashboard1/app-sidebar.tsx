import * as React from "react"
import { useEffect } from "react"
import { useLocation } from "react-router"

import {
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarHeader,
  useSidebar,
} from "@/components/ui/sidebar"
import { ScrollArea } from "@/components/ui/scroll-area"
import { useIsTablet } from "./hooks/use-mobile"

import { BrandHeader } from "./brand-header"
import { NavigationItems } from "./navigations"
import { NavUser } from "./nav-user"
import type { BrandConfig, LayoutUser, NavigationItem } from "./types"

type AppSidebarProps = React.ComponentProps<typeof Sidebar> & {
  brand: BrandConfig
  navigations: NavigationItem[]
  user?: LayoutUser | null
  header?: React.ReactNode
  footer?: React.ReactNode
  onSignOut?: () => void
}

export function AppSidebar({
  brand,
  navigations,
  user,
  header,
  footer,
  onSignOut,
  ...props
}: AppSidebarProps) {
  const { pathname } = useLocation()
  const { setOpen, setOpenMobile, isMobile } = useSidebar()
  const isTablet = useIsTablet()
  const previousTablet = React.useRef<boolean | undefined>(undefined)

  useEffect(() => {
    if (isMobile) setOpenMobile(false)
    // Only respond to route changes while on mobile.
    // eslint-disable-next-line react-hooks/exhaustive-deps -- match admin console behavior
  }, [pathname])

  useEffect(() => {
    // Wait until viewport is measured.
    if (isTablet === undefined) return

    // First measurement: keep persisted cookie / defaultOpen — do not override.
    if (previousTablet.current === undefined) {
      previousTablet.current = isTablet
      return
    }

    // Only sync when the tablet breakpoint actually crosses.
    if (previousTablet.current === isTablet) return
    previousTablet.current = isTablet
    setOpen(!isTablet)
    // eslint-disable-next-line react-hooks/exhaustive-deps -- avoid setOpen identity loop
  }, [isTablet])

  return (
    <Sidebar collapsible="icon" {...props}>
      <SidebarHeader className="gap-0 p-0 group-data-[collapsible=icon]:flex group-data-[collapsible=icon]:items-center group-data-[collapsible=icon]:justify-center group-data-[collapsible=icon]:px-0 group-data-[collapsible=icon]:pt-2">
        {header ?? <BrandHeader brand={brand} />}
      </SidebarHeader>
      <SidebarContent>
        <ScrollArea className="h-full">
          <NavigationItems navigations={navigations} />
        </ScrollArea>
      </SidebarContent>
      <SidebarFooter>
        {footer ?? <NavUser user={user} onSignOut={onSignOut} />}
      </SidebarFooter>
    </Sidebar>
  )
}
