"use client"

import { useState, type FC } from "react"
import { AlertTriangle, ChevronRight } from "lucide-react"
import { Link, useLocation } from "react-router"

import {
  SidebarGroup,
  SidebarGroupContent,
  SidebarGroupLabel,
  SidebarMenu,
  SidebarMenuBadge,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarMenuSub,
  SidebarMenuSubButton,
  SidebarMenuSubItem,
  useSidebar,
} from "@/components/ui/sidebar"
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from "@/components/ui/collapsible"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { cn } from "@/lib/utils"
import { useDockerNeedsAttention } from "@/modules/docker/pages/_shared/use-engine-status"

import { Icon } from "./icon"
import { collectNavPaths, isNavItemActive, isNavTreeActive } from "./nav-utils"
import type { NavigationItem } from "./types"

export type { NavigationItem }

interface NavigationItemsProps {
  navigations: NavigationItem[]
}

const DEFAULT_PREFETCH = "render"

const menuButtonClassName =
  "hover:text-foreground active:text-foreground hover:bg-[var(--primary)]/10 active:bg-[var(--primary)]/10"

function isDockerNavItem(item: NavigationItem): boolean {
  if (item.to === "/docker" || item.to.startsWith("/docker/")) return true
  return Boolean(item.children?.some(isDockerNavItem))
}

function DockerEnvironmentsCta({ compact = false }: { compact?: boolean }) {
  const needsAttention = useDockerNeedsAttention()
  const { pathname } = useLocation()
  if (!needsAttention) return null
  if (pathname === "/docker/environments") return null

  if (compact) {
    return (
      <DropdownMenuItem asChild>
        <Link
          to="/docker/environments"
          className="gap-2 text-amber-800 focus:text-amber-900 dark:text-amber-300"
        >
          <AlertTriangle className="size-3.5" />
          Fix Docker…
        </Link>
      </DropdownMenuItem>
    )
  }

  return (
    <SidebarMenuSubItem>
      <SidebarMenuSubButton
        asChild
        className="mt-1 border border-amber-500/30 bg-amber-500/10 text-amber-900 hover:bg-amber-500/15 hover:text-amber-950 active:bg-amber-500/15 active:text-amber-950 dark:text-amber-200 dark:hover:text-amber-100"
      >
        <Link to="/docker/environments" prefetch={DEFAULT_PREFETCH}>
          <AlertTriangle className="size-3.5 shrink-0" />
          <span>Fix Docker</span>
        </Link>
      </SidebarMenuSubButton>
    </SidebarMenuSubItem>
  )
}

type NavCollapsibleProps = {
  item: NavigationItem
  pathname: string
  isMobile: boolean
  groupPaths: string[]
}

const NavCollapsible: FC<NavCollapsibleProps> = ({
  item,
  pathname,
  isMobile,
  groupPaths,
}) => {
  const branchActive = isNavTreeActive(pathname, item, groupPaths)
  const [open, setOpen] = useState(branchActive)
  const [prevBranchActive, setPrevBranchActive] = useState(branchActive)
  const dockerBranch = isDockerNavItem(item)

  if (branchActive !== prevBranchActive) {
    setPrevBranchActive(branchActive)
    if (branchActive) setOpen(true)
  }

  return (
    <>
      <div className="hidden group-data-[collapsible=icon]:block">
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <SidebarMenuButton
              tooltip={item.title}
              isActive={branchActive}
              className={menuButtonClassName}
            >
              {item.icon && <Icon name={item.icon} />}
              <span>{item.title}</span>
              <ChevronRight className="ms-auto transition-transform duration-200 group-data-[state=open]/collapsible:rotate-90" />
            </SidebarMenuButton>
          </DropdownMenuTrigger>
          <DropdownMenuContent
            side={isMobile ? "bottom" : "right"}
            align={isMobile ? "end" : "start"}
            className="min-w-48 rounded-lg"
          >
            <DropdownMenuLabel>{item.title}</DropdownMenuLabel>
            {item.children?.flatMap((subItem) =>
              Array.isArray(subItem.children) && subItem.children.length > 0
                ? [
                    <DropdownMenuLabel
                      key={`${subItem.title}-label`}
                      className="text-xs text-muted-foreground"
                    >
                      {subItem.title}
                    </DropdownMenuLabel>,
                    ...subItem.children.map((leaf) => (
                      <DropdownMenuItem
                        className={cn(
                          "hover:bg-[var(--primary)]/10! hover:text-foreground active:bg-[var(--primary)]/10! active:text-foreground",
                          isNavItemActive(pathname, leaf.to, groupPaths) &&
                            "bg-[var(--primary)]/10 text-foreground"
                        )}
                        asChild
                        key={leaf.title}
                      >
                        <Link
                          to={leaf.to}
                          target={leaf.newTab ? "_blank" : undefined}
                          prefetch={leaf.prefetch ?? DEFAULT_PREFETCH}
                        >
                          {leaf.title}
                        </Link>
                      </DropdownMenuItem>
                    )),
                  ]
                : [
                    <DropdownMenuItem
                      className={cn(
                        "hover:bg-[var(--primary)]/10! hover:text-foreground active:bg-[var(--primary)]/10! active:text-foreground",
                        isNavItemActive(pathname, subItem.to, groupPaths) &&
                          "bg-[var(--primary)]/10 text-foreground"
                      )}
                      asChild
                      key={subItem.title}
                    >
                      <Link
                        to={subItem.to}
                        target={subItem.newTab ? "_blank" : undefined}
                        prefetch={subItem.prefetch ?? DEFAULT_PREFETCH}
                      >
                        {subItem.title}
                      </Link>
                    </DropdownMenuItem>,
                  ]
            )}
            {dockerBranch ? <DockerEnvironmentsCta compact /> : null}
          </DropdownMenuContent>
        </DropdownMenu>
      </div>
      <Collapsible
        open={open}
        onOpenChange={setOpen}
        className="group/collapsible block group-data-[collapsible=icon]:hidden"
      >
        <CollapsibleTrigger asChild>
          <SidebarMenuButton
            className={menuButtonClassName}
            isActive={branchActive}
            tooltip={item.title}
          >
            {item.icon && <Icon name={item.icon} />}
            <span>{item.title}</span>
            <ChevronRight className="ms-auto transition-transform duration-200 group-data-[state=open]/collapsible:rotate-90" />
          </SidebarMenuButton>
        </CollapsibleTrigger>
        <CollapsibleContent>
          <SidebarMenuSub>
            {item.children?.map((subItem) =>
              Array.isArray(subItem.children) && subItem.children.length > 0 ? (
                <SidebarMenuSubItem key={subItem.title}>
                  <NavNestedGroup
                    item={subItem}
                    pathname={pathname}
                    groupPaths={groupPaths}
                  />
                </SidebarMenuSubItem>
              ) : (
                <SidebarMenuSubItem key={subItem.title}>
                  <SidebarMenuSubButton
                    className={menuButtonClassName}
                    isActive={isNavItemActive(pathname, subItem.to, groupPaths)}
                    asChild
                  >
                    <Link
                      to={subItem.to}
                      target={subItem.newTab ? "_blank" : undefined}
                      prefetch={subItem.prefetch ?? DEFAULT_PREFETCH}
                    >
                      <span>{subItem.title}</span>
                    </Link>
                  </SidebarMenuSubButton>
                </SidebarMenuSubItem>
              )
            )}
            {dockerBranch ? <DockerEnvironmentsCta /> : null}
          </SidebarMenuSub>
        </CollapsibleContent>
      </Collapsible>
    </>
  )
}

function NavNestedGroup({
  item,
  pathname,
  groupPaths,
}: {
  item: NavigationItem
  pathname: string
  groupPaths: string[]
}) {
  const branchActive = isNavTreeActive(pathname, item, groupPaths)
  const [open, setOpen] = useState(branchActive)
  const [prevBranchActive, setPrevBranchActive] = useState(branchActive)

  if (branchActive !== prevBranchActive) {
    setPrevBranchActive(branchActive)
    if (branchActive) setOpen(true)
  }

  return (
    <Collapsible open={open} onOpenChange={setOpen} className="w-full">
      <CollapsibleTrigger asChild>
        <SidebarMenuSubButton
          className={cn(menuButtonClassName, "w-full justify-between")}
          isActive={branchActive}
        >
          <span>{item.title}</span>
          <ChevronRight className="size-3.5 transition-transform duration-200 group-data-[state=open]/collapsible:rotate-90" />
        </SidebarMenuSubButton>
      </CollapsibleTrigger>
      <CollapsibleContent>
        <ul className="ms-2 border-s border-sidebar-border py-0.5 ps-2">
          {item.children?.map((leaf) => (
            <li key={leaf.title}>
              <SidebarMenuSubButton
                className={menuButtonClassName}
                isActive={isNavItemActive(pathname, leaf.to, groupPaths)}
                asChild
              >
                <Link
                  to={leaf.to}
                  target={leaf.newTab ? "_blank" : undefined}
                  prefetch={leaf.prefetch ?? DEFAULT_PREFETCH}
                >
                  <span>{leaf.title}</span>
                </Link>
              </SidebarMenuSubButton>
            </li>
          ))}
        </ul>
      </CollapsibleContent>
    </Collapsible>
  )
}

export function NavigationItems({ navigations }: NavigationItemsProps) {
  const { pathname } = useLocation()
  const { isMobile } = useSidebar()

  return (
    <>
      {navigations.map((nav) => {
        const groupPaths = collectNavPaths(nav.children)

        return (
          <SidebarGroup key={nav.title}>
            <SidebarGroupLabel>{nav.title}</SidebarGroupLabel>
            <SidebarGroupContent className="flex flex-col gap-2">
              <SidebarMenu>
                {nav.children?.map((item) => (
                  <SidebarMenuItem key={item.title}>
                    {Array.isArray(item.children) &&
                    item.children.length > 0 ? (
                      <NavCollapsible
                        item={item}
                        pathname={pathname}
                        isMobile={isMobile}
                        groupPaths={groupPaths}
                      />
                    ) : (
                      <SidebarMenuButton
                        className={menuButtonClassName}
                        isActive={isNavItemActive(
                          pathname,
                          item.to,
                          groupPaths
                        )}
                        tooltip={item.title}
                        asChild
                      >
                        <Link
                          to={item.to}
                          target={item.newTab ? "_blank" : undefined}
                          prefetch={item.prefetch ?? DEFAULT_PREFETCH}
                        >
                          {item.icon && <Icon name={item.icon} />}
                          <span>{item.title}</span>
                        </Link>
                      </SidebarMenuButton>
                    )}
                    {!!item.isComing && (
                      <SidebarMenuBadge className="opacity-50 peer-hover/menu-button:text-foreground">
                        Coming
                      </SidebarMenuBadge>
                    )}
                    {!!item.isNew && (
                      <SidebarMenuBadge className="border border-green-400 text-green-600 peer-hover/menu-button:text-green-600">
                        New
                      </SidebarMenuBadge>
                    )}
                    {!!item.isDataBadge && (
                      <SidebarMenuBadge className="peer-hover/menu-button:text-foreground">
                        {item.isDataBadge}
                      </SidebarMenuBadge>
                    )}
                  </SidebarMenuItem>
                ))}
              </SidebarMenu>
            </SidebarGroupContent>
          </SidebarGroup>
        )
      })}
    </>
  )
}
