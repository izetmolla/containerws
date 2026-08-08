"use client"
import {
  ChevronsUpDown,
  // Plus
} from "lucide-react"

import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  // DropdownMenuSeparator,
  DropdownMenuShortcut,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import {
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  useSidebar,
} from "@/components/ui/sidebar"
import Icon from "@/components/icon"
import { useNavigate } from "react-router"
import { Logo } from "@/components/layouts/dashboard1"

import type { ModuleItem } from "../api"

export function ModuleSwitcher({ modules }: { modules: ModuleItem[] }) {
  const navigate = useNavigate()
  const { isMobile } = useSidebar()
  const stopLogoEvent = (
    e: React.MouseEvent<HTMLImageElement> | React.PointerEvent<HTMLImageElement>
  ) => {
    e.stopPropagation()
  }
  const onLogoClick = (e: React.MouseEvent<HTMLImageElement>) => {
    e.stopPropagation()
    navigate("/")
  }

  return (
    <SidebarMenu className="group-data-[collapsible=icon]:items-center">
      <SidebarMenuItem className="group-data-[collapsible=icon]:flex group-data-[collapsible=icon]:w-full group-data-[collapsible=icon]:justify-center">
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <SidebarMenuButton
              size="lg"
              className="group-data-[collapsible=icon]:size-8! group-data-[collapsible=icon]:justify-center group-data-[collapsible=icon]:gap-0 group-data-[collapsible=icon]:p-0! data-[state=open]:bg-sidebar-accent data-[state=open]:text-sidebar-accent-foreground"
            >
              <div className="flex aspect-square size-8 shrink-0 items-center justify-center rounded-lg bg-sidebar-primary text-sidebar-primary-foreground group-data-[collapsible=icon]:bg-transparent">
                <Logo onClick={onLogoClick} onPointerDown={stopLogoEvent} />
              </div>
              <div className="grid flex-1 text-left text-sm leading-tight group-data-[collapsible=icon]:hidden">
                <span className="truncate font-medium">
                  Container Workspace Console
                </span>
                <span className="truncate text-xs">
                  Your university management system
                </span>
              </div>
              <ChevronsUpDown className="ml-auto group-data-[collapsible=icon]:hidden" />
            </SidebarMenuButton>
          </DropdownMenuTrigger>
          <DropdownMenuContent
            className="w-(--radix-dropdown-menu-trigger-width) min-w-56 rounded-lg"
            align="start"
            side={isMobile ? "bottom" : "right"}
            sideOffset={4}
          >
            <DropdownMenuLabel className="text-xs text-muted-foreground">
              Modules
            </DropdownMenuLabel>
            {modules?.map((module, index) => (
              <DropdownMenuItem
                key={module.id}
                onClick={() => navigate(`/${module.name}`)}
                className="cursor-pointer gap-2 p-2"
              >
                <div className="flex size-6 items-center justify-center rounded-md border">
                  <Icon
                    name={module.icon ?? ""}
                    className="size-3.5 shrink-0"
                  />
                </div>
                {module.title}
                <DropdownMenuShortcut>⌘{index + 1}</DropdownMenuShortcut>
              </DropdownMenuItem>
            ))}
          </DropdownMenuContent>
        </DropdownMenu>
      </SidebarMenuItem>
    </SidebarMenu>
  )
}

export default ModuleSwitcher
