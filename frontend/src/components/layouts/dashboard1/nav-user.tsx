import {
  Bell,
  LogOut,
  Monitor,
  Moon,
  Palette,
  Settings,
  Sun,
  User,
} from "lucide-react"
import { Link } from "react-router"

import {
  Avatar,
  AvatarFallback,
  AvatarImage,
} from "@/components/ui/avatar"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuSeparator,
  DropdownMenuSub,
  DropdownMenuSubContent,
  DropdownMenuSubTrigger,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import {
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  useSidebar,
} from "@/components/ui/sidebar"

import { generateAvatarFallback } from "./nav-utils"
import type { LayoutUser } from "./types"

type ThemeValue = "light" | "dark" | "system"

type NavUserProps = {
  user?: LayoutUser | null
  theme?: ThemeValue
  onThemeChange?: (theme: ThemeValue) => void
  onSignOut?: () => void
  profileTo?: string
  notificationsTo?: string
  settingsTo?: string
}

export function NavUser({
  user,
  theme,
  onThemeChange,
  onSignOut,
  profileTo = "/account",
  notificationsTo = "/notifications",
  settingsTo = "/settings",
}: NavUserProps) {
  const { isMobile } = useSidebar()
  const fullName = [user?.firstName, user?.lastName].filter(Boolean).join(" ")
  const displayName = fullName || user?.email || "User"

  return (
    <SidebarMenu>
      <SidebarMenuItem>
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <SidebarMenuButton
              size="lg"
              className="cursor-pointer data-[state=open]:bg-sidebar-accent data-[state=open]:text-sidebar-accent-foreground"
            >
              <Avatar className="rounded-full">
                <AvatarImage src={user?.image} alt={displayName} />
                <AvatarFallback className="rounded-lg">
                  {generateAvatarFallback(displayName)}
                </AvatarFallback>
              </Avatar>
              <div className="grid flex-1 text-start text-sm leading-tight">
                <span className="truncate font-medium">{displayName}</span>
                {user?.email ? (
                  <span className="truncate text-xs text-muted-foreground">
                    {user.email}
                  </span>
                ) : null}
              </div>
            </SidebarMenuButton>
          </DropdownMenuTrigger>
          <DropdownMenuContent
            className="w-(--radix-dropdown-menu-trigger-width) min-w-56 rounded-lg"
            side={isMobile ? "bottom" : "right"}
            align="end"
            sideOffset={4}
          >
            <DropdownMenuLabel className="p-0 font-normal">
              <div className="flex items-center gap-2 px-1 py-1.5 text-start text-sm">
                <Avatar className="h-8 w-8 rounded-lg">
                  <AvatarImage src={user?.image} alt={displayName} />
                  <AvatarFallback className="rounded-lg">
                    {generateAvatarFallback(displayName)}
                  </AvatarFallback>
                </Avatar>
                <div className="grid flex-1 text-start text-sm leading-tight">
                  <span className="truncate font-medium">{displayName}</span>
                  {user?.email ? (
                    <span className="truncate text-xs text-muted-foreground">
                      {user.email}
                    </span>
                  ) : null}
                </div>
              </div>
            </DropdownMenuLabel>
            <DropdownMenuSeparator />
            <DropdownMenuGroup>
              <DropdownMenuItem asChild className="cursor-pointer">
                <Link to={profileTo}>
                  <User />
                  My Profile
                </Link>
              </DropdownMenuItem>
              <DropdownMenuItem asChild className="cursor-pointer">
                <Link to={notificationsTo}>
                  <Bell />
                  Notifications
                </Link>
              </DropdownMenuItem>
              <DropdownMenuItem asChild className="cursor-pointer">
                <Link to={settingsTo}>
                  <Settings />
                  Settings
                </Link>
              </DropdownMenuItem>
              {theme != null && onThemeChange ? (
                <DropdownMenuSub>
                  <DropdownMenuSubTrigger>
                    <Palette className="size-3.5 opacity-70" />
                    Appearance
                  </DropdownMenuSubTrigger>
                  <DropdownMenuSubContent
                    align="start"
                    className="min-w-36 cursor-pointer"
                  >
                    <DropdownMenuRadioGroup
                      value={theme}
                      onValueChange={(value) =>
                        onThemeChange(value as ThemeValue)
                      }
                    >
                      <DropdownMenuRadioItem value="light">
                        <Sun className="size-3.5 opacity-70" />
                        Light
                      </DropdownMenuRadioItem>
                      <DropdownMenuRadioItem value="dark">
                        <Moon className="size-3.5 opacity-70" />
                        Dark
                      </DropdownMenuRadioItem>
                      <DropdownMenuRadioItem value="system">
                        <Monitor className="size-3.5 opacity-70" />
                        System
                      </DropdownMenuRadioItem>
                    </DropdownMenuRadioGroup>
                  </DropdownMenuSubContent>
                </DropdownMenuSub>
              ) : null}
            </DropdownMenuGroup>
            {onSignOut ? (
              <DropdownMenuItem onClick={onSignOut} className="cursor-pointer">
                <LogOut className="size-3.5 opacity-70" />
                Log out
              </DropdownMenuItem>
            ) : null}
          </DropdownMenuContent>
        </DropdownMenu>
      </SidebarMenuItem>
    </SidebarMenu>
  )
}
