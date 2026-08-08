"use client"

import { useQuery } from "@tanstack/react-query"
import { Outlet } from "react-router"

import {
  DashboardLayout,
  type SearchFn,
} from "@/components/layouts/dashboard1"

import Unauthorized401 from "@/components/errors/401"
import ContentLoader from "@/components/content-loader"
import { withError, withInitialData, withModule } from "@/lib/network"
import useAuthorizationStore, {
  useCurrentUser,
  signOutApi,
} from "@/store/authorization"

import { getGeneralData, type GeneralDataTypes } from "./sidebar/api"
import { getGlobalSearchData } from "./sidebar/search-api"
import ModuleSwitcher from "./sidebar/sidebarheader"
import ModuleHeader from "./sidebar/sidebarheader/module-header"
import { NavUser } from "./sidebar/nav-user"
import { CloudShell, CloudShellToggle } from "@/components/cloudshell"
import { DesktopToggle } from "@/components/desktop"
import { SoftwareQueueToggle } from "@/components/softwares/queue-toggle"
import { SidebarVersionLink } from "./sidebar-version-link"
import type { FC } from "react"

const searchFn: SearchFn = async ({ keyword }) => {
  return getGlobalSearchData({ keyword })
}


interface LayoutProps {
  cleanElement?: boolean
}

const Layout: FC<LayoutProps> = ({ cleanElement = false }) => {
  const { current_session, signOut } = useAuthorizationStore()
  const accessDenied = useAuthorizationStore((x) => x.accessDenied)
  const user = useCurrentUser()
  const { module } = withModule()

  const { isLoading, error, data } = useQuery({
    queryKey: ["general-data", module],
    queryFn: () => getGeneralData(),
    ...withInitialData<GeneralDataTypes>("general"),
    enabled: Boolean(current_session),
    staleTime: 0,
    refetchOnMount: "always",
  })

  if (!current_session || current_session === "") {
    window.location.replace(
      `/sign-in?redirectUrl=${encodeURIComponent(window.location.href)}`
    )
    return null
  }

  const handleSignOut = () => {
    signOut()
    void signOutApi()
    window.location.replace("/sign-in")
  }

  const sidebarHeader = (
    <ContentLoader
      isLoading={isLoading}
      error={withError(error, data)}
      minimalError
    >
      {data?.module?.id && data.module.name !== "portal" ? (
        <ModuleHeader module={data.module} />
      ) : (
        <ModuleSwitcher modules={data?.modules ?? []} />
      )}
    </ContentLoader>
  )

  const appVersion =
    data?.app_version?.trim() ||
    data?.module?.app_version?.trim() ||
    ""
  const versionLabel = !appVersion
    ? null
    : appVersion === "(untracked)"
      ? "dev"
      : appVersion

  if (cleanElement) {
    return <Outlet />
  }

  return (
    <>
      <DashboardLayout
        brand={{
          title: "Container Workspace Console",
          description: "Your university management system",
          homeTo: "/",
        }}
        navigations={data?.navigations ?? []}
        user={{
          firstName: user?.first_name,
          lastName: user?.last_name,
          email: user?.email,
          image: user?.image,
        }}
        searchFn={searchFn}
        sidebarHeader={sidebarHeader}
        sidebarFooter={
          <div className="flex flex-col gap-1">
            <NavUser />
            {versionLabel ? (
              <SidebarVersionLink versionLabel={versionLabel} />
            ) : null}
          </div>
        }
        headerActions={
          <>
            <SoftwareQueueToggle />
            <DesktopToggle />
            <CloudShellToggle />
          </>
        }
        onSignOut={handleSignOut}
        accessDenied={accessDenied}
        accessDeniedFallback={<Unauthorized401 />}
      >
        <Outlet />
      </DashboardLayout>
      <CloudShell />
    </>
  )
}

export default Layout