import ApiService, { withModule } from "@/lib/network"
import type { NavigationItem } from "@/components/layouts/dashboard1"

export type ModuleItem = {
  id: string
  name: string
  title: string
  icon?: string
  description?: string
  roles?: string[]
  app_version?: string
  commit_sha?: string
}

export interface GeneralDataTypes {
  modules: ModuleItem[]
  module: ModuleItem
  current_user_id: string
  navigations: NavigationItem[]
  /** Running binary version (ldflags); falls back to "(untracked)" in dev. */
  app_version?: string
  commit_sha?: string
}

export function getGeneralData() {
  return ApiService.fetchDataBody<GeneralDataTypes>({
    url: "/general",
    method: "get",
    params: withModule(),
  })
}
