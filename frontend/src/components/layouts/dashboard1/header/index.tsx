import { PanelLeftClose, PanelLeftOpen } from "lucide-react"

import { Separator } from "@/components/ui/separator"
import { Button } from "@/components/ui/button"
import { useSidebar } from "@/components/ui/sidebar"

import Notifications from "./notifications"
import { SearchProvider, SearchDesktop, SearchMobileTrigger } from "./search"
import ThemeSwitch from "./theme-switch"
import { ThemeCustomizerPanel } from "../theme-customizer"
import UserMenu from "./user-menu"
import type { LayoutUser } from "../types"
import type { SearchFn } from "./search"

type SiteHeaderProps = {
  user?: LayoutUser | null
  onSignOut?: () => void
  searchFn?: SearchFn
  /** Extra actions rendered before notifications (optional). */
  actions?: React.ReactNode
}

export function SiteHeader({
  user,
  onSignOut,
  searchFn,
  actions,
}: SiteHeaderProps) {
  const { toggleSidebar, open } = useSidebar()

  return (
    <header className="sticky top-0 z-50 flex h-(--header-height) shrink-0 items-center gap-2 overflow-hidden border-b bg-background/40 backdrop-blur-md transition-[width,height] ease-linear group-has-data-[collapsible=icon]/sidebar-wrapper:h-(--header-height) md:rounded-ss-xl md:rounded-se-xl">
      <SearchProvider searchFn={searchFn}>
        <div className="grid w-full grid-cols-[auto_minmax(0,1fr)_auto] items-center gap-4 px-4 lg:gap-6 lg:px-6">
          <div className="flex shrink-0 items-center gap-1 justify-self-start lg:gap-2">
            <Button onClick={toggleSidebar} size="icon" variant="ghost">
              {open ? <PanelLeftClose /> : <PanelLeftOpen />}
            </Button>
            <SearchMobileTrigger />
            <Separator
              orientation="vertical"
              className="mx-2 hidden data-[orientation=vertical]:h-4 data-[orientation=vertical]:self-center lg:block"
            />
          </div>

          <div className="hidden w-full min-w-0 justify-center px-2 sm:px-4 lg:flex lg:px-6">
            <SearchDesktop />
          </div>

          <div className="flex shrink-0 items-center gap-2 justify-self-end">
            {actions}
            <Notifications />
            <ThemeSwitch />
            <ThemeCustomizerPanel />
            <Separator
              orientation="vertical"
              className="mx-2 data-[orientation=vertical]:h-4 data-[orientation=vertical]:self-center"
            />
            <UserMenu user={user} onSignOut={onSignOut} />
          </div>
        </div>
      </SearchProvider>
    </header>
  )
}
