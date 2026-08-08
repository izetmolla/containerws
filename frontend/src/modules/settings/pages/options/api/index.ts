import ApiService from "@/lib/network"

export const SETTINGS_OPTIONS_FETCH_KEY = "settingsoptions"
export const SETTINGS_OPTIONS_BASE = "/settings/options"

export type OsOption = {
  id: string
  name: string
  value: string
  group?: string
  created_at?: string
  updated_at?: string
}

export type OptionsListResponse = {
  data: OsOption[]
  groups?: string[]
  error?: boolean
  message?: string
}

export type OptionMutationResponse = {
  data: OsOption
  message?: string
  error?: boolean
}

export type CreateOptionInput = {
  name: string
  value: string
}

export type UpdateOptionInput = {
  name?: string
  value?: string
}

export async function listOptions() {
  return ApiService.fetchData<OptionsListResponse>({
    url: SETTINGS_OPTIONS_BASE,
    method: "get",
  })
}

export async function getOption(id: string) {
  return ApiService.fetchData<OptionMutationResponse>({
    url: `${SETTINGS_OPTIONS_BASE}/${id}`,
    method: "get",
  })
}

export async function createOption(input: CreateOptionInput) {
  return ApiService.fetchData<OptionMutationResponse>({
    url: SETTINGS_OPTIONS_BASE,
    method: "post",
    data: {
      name: input.name,
      value: input.value,
    },
  })
}

export async function updateOption(id: string, input: UpdateOptionInput) {
  return ApiService.fetchData<OptionMutationResponse>({
    url: `${SETTINGS_OPTIONS_BASE}/${id}`,
    method: "put",
    data: input,
  })
}

export async function deleteOption(id: string) {
  return ApiService.fetchData<{
    data: { id: string }
    message?: string
    error?: boolean
  }>({
    url: `${SETTINGS_OPTIONS_BASE}/${id}`,
    method: "delete",
  })
}
