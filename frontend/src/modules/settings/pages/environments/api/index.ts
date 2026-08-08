import ApiService from "@/lib/network"

export const SETTINGS_ENV_FETCH_KEY = "settingsenvironments"
export const SETTINGS_ENV_BASE = "/settings/environments"

export type OsEnvironment = {
  id: string
  name: string
  value: string
  group: string
  source: string
  is_core: boolean
  is_secret: boolean
  is_disabled: boolean
  is_textarea?: boolean
  secret_masked?: boolean
  created_at?: string
  updated_at?: string
}

export type EnvironmentsListResponse = {
  data: OsEnvironment[]
  groups?: string[]
  core_names?: string[]
  ungrouped?: string
  error?: boolean
  message?: string
}

export type EnvironmentMutationResponse = {
  data: OsEnvironment
  message?: string
  error?: boolean
}

export type CreateEnvironmentInput = {
  name: string
  value: string
  group?: string
  is_secret?: boolean
  is_disabled?: boolean
  is_textarea?: boolean
}

export type UpdateEnvironmentInput = {
  name?: string
  value?: string
  group?: string
  is_secret?: boolean
  is_disabled?: boolean
  is_textarea?: boolean
}

export async function listEnvironments(group?: string) {
  return ApiService.fetchData<EnvironmentsListResponse>({
    url: SETTINGS_ENV_BASE,
    method: "get",
    params: group ? { group } : undefined,
  })
}

export async function getEnvironment(id: string) {
  return ApiService.fetchData<EnvironmentMutationResponse>({
    url: `${SETTINGS_ENV_BASE}/${id}`,
    method: "get",
  })
}

export async function createEnvironment(input: CreateEnvironmentInput) {
  return ApiService.fetchData<EnvironmentMutationResponse>({
    url: SETTINGS_ENV_BASE,
    method: "post",
    data: {
      name: input.name,
      value: input.value,
      group: input.group || "",
      is_secret: Boolean(input.is_secret),
      is_disabled: Boolean(input.is_disabled),
      is_textarea: Boolean(input.is_textarea),
    },
  })
}

export async function updateEnvironment(
  id: string,
  input: UpdateEnvironmentInput
) {
  return ApiService.fetchData<EnvironmentMutationResponse>({
    url: `${SETTINGS_ENV_BASE}/${id}`,
    method: "put",
    data: input,
  })
}

export async function deleteEnvironment(id: string) {
  return ApiService.fetchData<{
    data: { id: string }
    message?: string
    error?: boolean
  }>({
    url: `${SETTINGS_ENV_BASE}/${id}`,
    method: "delete",
  })
}
