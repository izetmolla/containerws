import ApiService from "@/lib/network"

export const SETTINGS_MCP_FETCH_KEY = "settingsmcp"
export const SETTINGS_MCP_STANDALONE_BASE = "/settings/mcp/standalone"
export const SETTINGS_MCP_ADDRESSES_BASE = "/settings/mcp/addresses"
export const SETTINGS_MCP_KEYS_BASE = "/settings/mcp/keys"

export type McpBindAddress = {
  address: string
  interface?: string
  label: string
  family?: string
}

export type McpStandaloneStatus = {
  enabled: boolean
  running: boolean
  address: string
  port: number
  listen_addr: string
  public_url: string
  last_error?: string
  source: string
  main_api_mcp: string
  bind_addresses?: McpBindAddress[]
}

export type McpKey = {
  id: string
  name: string
  description: string
  key_prefix: string
  key_suffix?: string
  key?: string
  status: string
  expires_at: string | null
  last_used_at: string | null
  last_used_ip: string
  created_by: string
  created_at: string
  updated_at: string
}

export type CreateMcpKeyInput = {
  name: string
  description?: string
  expires_in_days?: number
}

export type UpdateMcpKeyInput = {
  name?: string
  description?: string
  status?: "active" | "inactive"
}

type StandaloneResponse = {
  data: McpStandaloneStatus
  message?: string
  error?: boolean
}

type KeysListResponse = {
  data: McpKey[]
  error?: boolean
}

type KeyMutationResponse = {
  data: McpKey
  message?: string
  error?: boolean
}

export async function getMcpStandalone() {
  return ApiService.fetchData<StandaloneResponse>({
    url: SETTINGS_MCP_STANDALONE_BASE,
    method: "get",
  })
}

export async function updateMcpStandalone(input: {
  enabled: boolean
  address: string
  port: number
}) {
  return ApiService.fetchData<StandaloneResponse>({
    url: SETTINGS_MCP_STANDALONE_BASE,
    method: "put",
    data: input,
  })
}

export async function listMcpBindAddresses() {
  return ApiService.fetchData<{ data: McpBindAddress[]; error?: boolean }>({
    url: SETTINGS_MCP_ADDRESSES_BASE,
    method: "get",
  })
}

export async function listMcpKeys() {
  return ApiService.fetchData<KeysListResponse>({
    url: SETTINGS_MCP_KEYS_BASE,
    method: "get",
  })
}

/** Fetch one key including the full secret (for clipboard copy). */
export async function getMcpKey(id: string) {
  return ApiService.fetchData<KeyMutationResponse>({
    url: `${SETTINGS_MCP_KEYS_BASE}/${id}`,
    method: "get",
  })
}

export async function createMcpKey(input: CreateMcpKeyInput) {
  return ApiService.fetchData<KeyMutationResponse>({
    url: SETTINGS_MCP_KEYS_BASE,
    method: "post",
    data: {
      name: input.name,
      description: input.description || "",
      expires_in_days: input.expires_in_days ?? 0,
    },
  })
}

export async function updateMcpKey(id: string, input: UpdateMcpKeyInput) {
  return ApiService.fetchData<KeyMutationResponse>({
    url: `${SETTINGS_MCP_KEYS_BASE}/${id}`,
    method: "put",
    data: input,
  })
}

export async function revokeMcpKey(id: string) {
  return ApiService.fetchData<KeyMutationResponse>({
    url: `${SETTINGS_MCP_KEYS_BASE}/${id}/revoke`,
    method: "post",
  })
}

export async function deleteMcpKey(id: string) {
  return ApiService.fetchData<{
    data: { id: string }
    message?: string
    error?: boolean
  }>({
    url: `${SETTINGS_MCP_KEYS_BASE}/${id}`,
    method: "delete",
  })
}
