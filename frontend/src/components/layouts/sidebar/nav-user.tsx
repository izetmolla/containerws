"use client"

import {
  Bell,
  LogOut,
  Monitor,
  Moon,
  Palette,
  Settings,
  ShieldCog,
  Sun,
  User,
} from "lucide-react"
import { MoreVerticalIcon } from "lucide-react"
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
import { useTheme, type Theme } from "@/components/layouts/dashboard1/providers/use-theme"
import useAuthorizationStore, {
  useCurrentUser,
  signOutApi,
} from "@/store/authorization"
import { generateAvatarFallback } from "@/lib/utils"
// import { SwitchAccountMenuItems } from "@/components/switch-account"
// import LanguageSwitch from "@/components/language-switch"

export function NavUser() {
  const user = useCurrentUser()
  const { signOut } = useAuthorizationStore()
  const { theme, setTheme } = useTheme()
  const fullName = `${user?.first_name} ${user?.last_name}`
  const { isMobile } = useSidebar()

  const handleLogout = () => {
    signOut()
    void signOutApi()
    window.location.replace("/sign-in")
  }

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
                <AvatarImage src={user?.image} alt={fullName} />
                <AvatarFallback className="rounded-lg">
                  {generateAvatarFallback(fullName)}
                </AvatarFallback>
              </Avatar>
              <div className="grid flex-1 text-left text-sm leading-tight">
                <span className="truncate font-medium">{fullName}</span>
                <span className="truncate text-xs text-muted-foreground">
                  {user?.email}
                </span>
              </div>
              <MoreVerticalIcon className="ml-auto size-4" />
            </SidebarMenuButton>
          </DropdownMenuTrigger>
          <DropdownMenuContent
            className="w-(--radix-dropdown-menu-trigger-width) min-w-56 rounded-lg"
            side={isMobile ? "bottom" : "right"}
            align="end"
            sideOffset={4}
          >
            <DropdownMenuLabel className="p-0 font-normal">
              <div className="flex items-center gap-2 px-1 py-1.5 text-left text-sm">
                <Avatar className="h-8 w-8 rounded-lg">
                  <AvatarImage src={user?.image} alt={fullName} />
                  <AvatarFallback className="rounded-lg">
                    {generateAvatarFallback(fullName)}
                  </AvatarFallback>
                </Avatar>
                <div className="grid flex-1 text-left text-sm leading-tight">
                  <span className="truncate font-medium">{fullName}</span>
                  <span className="truncate text-xs text-muted-foreground">
                    {user?.email}
                  </span>
                </div>
              </div>
            </DropdownMenuLabel>
            <DropdownMenuSeparator />
            <DropdownMenuGroup>
              <DropdownMenuItem asChild className="cursor-pointer">
                <Link to="/account">
                  <User />
                  My Profile
                </Link>
              </DropdownMenuItem>
              <DropdownMenuItem asChild className="cursor-pointer">
                <Link to="/notifications">
                  <Bell />
                  Notifications
                </Link>
              </DropdownMenuItem>
              <DropdownMenuItem asChild className="cursor-pointer">
                <Link to="/settings">
                  <Settings />
                  Settings
                </Link>
              </DropdownMenuItem>
              <DropdownMenuSub>
                <DropdownMenuSubTrigger>
                  <Palette className="size-3.5 opacity-70" />
                  Apparence
                </DropdownMenuSubTrigger>
                <DropdownMenuSubContent
                  // side="left"
                  align="start"
                  className="min-w-36 cursor-pointer"
                >
                  <DropdownMenuRadioGroup
                    value={theme}
                    onValueChange={(value) => setTheme(value as Theme)}
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
              {/* <LanguageSwitch variant="menu" /> */}
              {user?.roles?.includes("admin:rw") && (
                <DropdownMenuItem className="cursor-pointer" asChild>
                  <Link to="/cadmin">
                    <ShieldCog />
                    CAdmin
                  </Link>
                </DropdownMenuItem>
              )}
            </DropdownMenuGroup>
            {/* <SwitchAccountMenuItems /> */}
            <DropdownMenuItem onClick={handleLogout} className="cursor-pointer">
              <LogOut className="size-3.5 opacity-70" />
              Log out
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </SidebarMenuItem>
    </SidebarMenu>
  )
}
