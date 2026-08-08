import ApiService, {
  REQUEST_HEADER_AUTH_KEY,
  TOKEN_TYPE,
} from "@/lib/network"
import { baseApiURL } from "@/lib/network/env"
import { getCurrentTokens } from "@/store/authorization"
import useAuthorizationStore from "@/store/authorization"

import { USERS_SINGLE_BASE } from "../../list/api"

export const RDP_SETUP_FETCH_KEY = "rdpsetup"
export const RDP_SETUP_BASE = "/vnc-novnc/rdp/install/setup"
export const USER_RDP_FETCH_KEY = "user-rdp"

export type RdpHostPlan = {
  hostname: string
  os: string
  distro: string
  distro_id: string
  package_family: string
  package_manager: string
  supported: boolean
  notes?: string[]
  packages: string[]
}

export type RdpStatusReport = {
  ready: boolean
  running: boolean
  binaries: Record<string, boolean>
  missing?: string[]
  port: number
  plan: RdpHostPlan
}

export type RdpBindAddress = {
  address: string
  interface: string
  label: string
  localhost: boolean
  family: string
}

export type RdpUserState = {
  enabled: boolean
  packages_ready: boolean
  service_running: boolean
  rdp_address: string
  rdp_port: number
  port: number
  addresses?: RdpBindAddress[]
  missing?: string[]
  plan?: RdpHostPlan
  username?: string
  has_profile?: boolean
  connect_hint?: string
}

export type SetupStreamEvent = {
  type: "start" | "log" | "done" | "cancelled" | "error"
  job_id?: string
  line?: string
  stream?: "stdout" | "stderr" | "system"
  message?: string
  success?: boolean
  plan?: RdpHostPlan | RdpStatusReport
}

export type InstallTerminalLine = {
  id: string
  text: string
  stream: "stdout" | "stderr" | "system"
  at: number
}

export async function getRdpSetupStatus() {
  return ApiService.fetchData<{ data: RdpStatusReport }>({
    url: `${RDP_SETUP_BASE}/status`,
    method: "get",
  })
}

export async function cancelRdpSetupJob(jobId: string) {
  return ApiService.fetchData<{
    data: { job_id: string; cancelled: boolean }
  }>({
    url: `${RDP_SETUP_BASE}/jobs/${jobId}/cancel`,
    method: "post",
  })
}

export async function getUserRdp(userId: string) {
  return ApiService.fetchData<{ data: RdpUserState }>({
    url: `${USERS_SINGLE_BASE}/${userId}/rdp`,
    method: "get",
  })
}

export async function enableUserRdp(
  userId: string,
  body?: { rdp_address?: string }
) {
  return ApiService.fetchData<{ data: RdpUserState; message?: string }>({
    url: `${USERS_SINGLE_BASE}/${userId}/rdp/enable`,
    method: "post",
    data: body || {},
  })
}

export async function updateUserRdp(
  userId: string,
  body: { rdp_address: string }
) {
  return ApiService.fetchData<{ data: RdpUserState; message?: string }>({
    url: `${USERS_SINGLE_BASE}/${userId}/rdp`,
    method: "put",
    data: body,
  })
}

export async function disableUserRdp(userId: string) {
  return ApiService.fetchData<{ data: RdpUserState; message?: string }>({
    url: `${USERS_SINGLE_BASE}/${userId}/rdp/disable`,
    method: "post",
  })
}

export async function startUserRdp(userId: string) {
  return ApiService.fetchData<{ data: RdpUserState; message?: string }>({
    url: `${USERS_SINGLE_BASE}/${userId}/rdp/start`,
    method: "post",
  })
}

export async function stopUserRdp(userId: string) {
  return ApiService.fetchData<{ data: RdpUserState; message?: string }>({
    url: `${USERS_SINGLE_BASE}/${userId}/rdp/stop`,
    method: "post",
  })
}

function authHeaders(): HeadersInit {
  const tokens = getCurrentTokens(useAuthorizationStore.getState())
  const headers: Record<string, string> = {
    Accept: "text/event-stream",
  }
  if (tokens?.access_token) {
    headers[REQUEST_HEADER_AUTH_KEY] = `${TOKEN_TYPE}${tokens.access_token}`
  }
  return headers
}

export async function streamRdpSetup(handlers: {
  onEvent: (event: SetupStreamEvent) => void
  signal?: AbortSignal
  /** Force package reinstall (apt --reinstall / equivalent). */
  reinstall?: boolean
}): Promise<void> {
  const qs = handlers.reinstall ? "?reinstall=1" : ""
  const res = await fetch(`${baseApiURL()}${RDP_SETUP_BASE}/stream${qs}`, {
    method: "POST",
    headers: authHeaders(),
    signal: handlers.signal,
  })

  if (!res.ok) {
    let message = `RDP setup failed (${res.status})`
    try {
      const body = (await res.json()) as { message?: string }
      if (body.message) message = body.message
    } catch {
      // ignore
    }
    throw new Error(message)
  }
  if (!res.body) throw new Error("Setup stream unavailable")

  const reader = res.body.getReader()
  const decoder = new TextDecoder()
  let buffer = ""

  const emitBlock = (block: string) => {
    for (const raw of block.split("\n")) {
      const line = raw.trimEnd()
      if (!line.startsWith("data:")) continue
      const payload = line.slice(5).trimStart()
      if (!payload) continue
      try {
        handlers.onEvent(JSON.parse(payload) as SetupStreamEvent)
      } catch {
        // skip
      }
    }
  }

  while (true) {
    const { done, value } = await reader.read()
    if (done) break
    buffer += decoder.decode(value, { stream: true })
    const parts = buffer.split("\n\n")
    buffer = parts.pop() ?? ""
    for (const part of parts) emitBlock(part)
  }
  if (buffer.trim()) emitBlock(buffer)
}
