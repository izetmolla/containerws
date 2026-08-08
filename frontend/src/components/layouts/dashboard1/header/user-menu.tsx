import {
  Bell,
  LogOut,
  User,
  Monitor,
  Moon,
  Palette,
  Settings,
  Sun,
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
import { useTheme, type Theme } from "../providers/use-theme"
import { generateAvatarFallback } from "../nav-utils"
import type { LayoutUser } from "../types"

type UserMenuProps = {
  user?: LayoutUser | null
  onSignOut?: () => void
}

export default function UserMenu({ user, onSignOut }: UserMenuProps) {
  const { theme, setTheme } = useTheme()
  const fullName = [user?.firstName, user?.lastName].filter(Boolean).join(" ")
  const displayName = fullName || user?.email || "User"

  const handleLogout = () => {
    onSignOut?.()
  }

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Avatar>
          <AvatarImage src={user?.image} alt={displayName} />
          <AvatarFallback className="rounded-lg">
            {generateAvatarFallback(displayName)}
          </AvatarFallback>
        </Avatar>
      </DropdownMenuTrigger>
      <DropdownMenuContent
        className="w-(--radix-dropdown-menu-trigger-width) min-w-60"
        align="end"
      >
        <DropdownMenuLabel className="p-0">
          <div className="flex items-center gap-2 px-1 py-1.5 text-start text-sm">
            <Avatar>
              <AvatarImage src={user?.image} alt={displayName} />
              <AvatarFallback className="rounded-lg">
                {generateAvatarFallback(displayName)}
              </AvatarFallback>
            </Avatar>
            <div className="grid flex-1 text-start text-sm leading-tight">
              <span className="truncate font-semibold">{displayName}</span>
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
              Appearance
            </DropdownMenuSubTrigger>
            <DropdownMenuSubContent
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
        </DropdownMenuGroup>
        {onSignOut ? (
          <DropdownMenuItem onClick={handleLogout} className="cursor-pointer">
            <LogOut className="size-3.5 opacity-70" />
            Log out
          </DropdownMenuItem>
        ) : null}
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
