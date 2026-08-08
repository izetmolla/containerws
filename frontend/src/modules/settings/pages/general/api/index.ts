import ApiService from "@/lib/network"

export const SETTINGS_GENERAL_FETCH_KEY = "settingsgeneral"
export const SETTINGS_GENERAL_BASE = "/settings/general"

export type DockerModuleStatus = {
  binary_present: boolean
  running: boolean
  installed: boolean
  software_id: string
  software_name: string
}

export type KubernetesModuleStatus = {
  configured: boolean
  active_id: string
  files: number
}

export type ProxymanagerModuleStatus = {
  active_engine: string
  dirty: boolean
  configured: boolean
  last_applied_at?: string
}

export type GeneralSettings = {
  workspace_name: string
  workspace_description: string
  docker_enabled: boolean
  kubernetes_enabled: boolean
  proxymanager_enabled: boolean
  docker: DockerModuleStatus
  kubernetes: KubernetesModuleStatus
  proxymanager: ProxymanagerModuleStatus
}

export type GeneralSettingsResponse = {
  data: GeneralSettings
  message?: string
  error?: boolean
}

export async function getGeneralSettings() {
  return ApiService.fetchData<GeneralSettingsResponse>({
    url: SETTINGS_GENERAL_BASE,
    method: "get",
  })
}

export async function updateGeneralSettings(input: {
  workspace_name: string
  workspace_description: string
}) {
  return ApiService.fetchData<GeneralSettingsResponse>({
    url: SETTINGS_GENERAL_BASE,
    method: "put",
    data: {
      workspace_name: input.workspace_name,
      workspace_description: input.workspace_description,
    },
  })
}

export async function updateModuleSettings(input: {
  docker_enabled?: boolean
  kubernetes_enabled?: boolean
  proxymanager_enabled?: boolean
}) {
  return ApiService.fetchData<GeneralSettingsResponse>({
    url: `${SETTINGS_GENERAL_BASE}/modules`,
    method: "put",
    data: input,
  })
}
