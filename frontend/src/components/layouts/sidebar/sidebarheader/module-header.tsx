import { Logo } from "@/components/layouts/dashboard1"
import {
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
} from "@/components/ui/sidebar"
import { useNavigate } from "react-router"
import { type FC } from "react"
import type { ModuleItem } from "../api"

interface ModuleHeaderProps {
  module: ModuleItem
}
const ModuleHeader: FC<ModuleHeaderProps> = ({ module }) => {
  const navigate = useNavigate()
  const stopLogoEvent = (
    e: React.MouseEvent<HTMLImageElement> | React.PointerEvent<HTMLImageElement>
  ) => {
    e.stopPropagation()
  }

  const onLogoClick = (e: React.MouseEvent<HTMLImageElement>) => {
    stopLogoEvent(e)
    navigate("/")
  }

  const onTitleClick = (e: React.MouseEvent<HTMLDivElement>) => {
    e.stopPropagation()
    navigate(`/${module.name}`)
  }

  return (
    <SidebarMenu className="group-data-[collapsible=icon]:items-center">
      <SidebarMenuItem className="group-data-[collapsible=icon]:flex group-data-[collapsible=icon]:w-full group-data-[collapsible=icon]:justify-center">
        <SidebarMenuButton className="h-(--header-height) cursor-pointer items-center gap-3 rounded-lg px-2 py-2 group-data-[collapsible=icon]:size-8! group-data-[collapsible=icon]:justify-center group-data-[collapsible=icon]:gap-0 group-data-[collapsible=icon]:p-0! hover:text-foreground">
          <Logo onClick={onLogoClick} onPointerDown={stopLogoEvent} />
          <div
            className="min-w-0 cursor-pointer space-y-0.5 leading-tight group-data-[collapsible=icon]:hidden"
            onClick={onTitleClick}
          >
            <p className="truncate text-sm font-semibold text-foreground">
              {module.title || "Container Workspace Console"}
            </p>
            <p className="truncate text-xs text-muted-foreground">
              {module.description || "Your university management system"}
            </p>
          </div>
        </SidebarMenuButton>
      </SidebarMenuItem>
    </SidebarMenu>
  )
}

export default ModuleHeader
