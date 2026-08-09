"use client"

import { useNavigate } from "react-router"

import {
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
} from "@/components/ui/sidebar"

import { Logo } from "./logo"
import type { BrandConfig } from "./types"

type BrandHeaderProps = {
  brand: BrandConfig
}

export function BrandHeader({ brand }: BrandHeaderProps) {
  const navigate = useNavigate()
  const homeTo = brand.homeTo ?? "/"

  const goHome = () => {
    void navigate(homeTo)
  }

  return (
    <SidebarMenu className="group-data-[collapsible=icon]:items-center">
      <SidebarMenuItem className="group-data-[collapsible=icon]:flex group-data-[collapsible=icon]:w-full group-data-[collapsible=icon]:justify-center">
        <SidebarMenuButton
          type="button"
          onClick={goHome}
          className="h-(--header-height) cursor-pointer items-center gap-3 rounded-lg px-2 py-2 group-data-[collapsible=icon]:size-8! group-data-[collapsible=icon]:justify-center group-data-[collapsible=icon]:gap-0 group-data-[collapsible=icon]:p-0! hover:text-foreground"
        >
          <Logo src={brand.logoSrc} alt={brand.logoAlt ?? brand.title} />
          <div className="min-w-0 space-y-0.5 leading-tight group-data-[collapsible=icon]:hidden">
            <p className="truncate text-sm font-semibold text-foreground">
              {brand.title}
            </p>
            {brand.description ? (
              <p className="truncate text-xs text-muted-foreground">
                {brand.description}
              </p>
            ) : null}
          </div>
        </SidebarMenuButton>
      </SidebarMenuItem>
    </SidebarMenu>
  )
}
