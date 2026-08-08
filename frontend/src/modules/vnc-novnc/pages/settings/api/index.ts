import ApiService, {
  REQUEST_HEADER_AUTH_KEY,
  TOKEN_TYPE,
} from "@/lib/network"
import { baseApiURL } from "@/lib/network/env"
import { getCurrentTokens } from "@/store/authorization"
import useAuthorizationStore from "@/store/authorization"
import type { SoftwareQueueSnapshot } from "@/modules/softwares/pages/list/api"

export const VNC_SETUP_FETCH_KEY = "vncsetuppage"
export const VNC_SETUP_BASE = "/vnc-novnc/install/setup"

export type PackageFamily = "debian" | "rhel" | "arch" | "unknown" | string

export type HostPlan = {
  hostname: string
  os: string
  kernel: string
  arch: string
  platform: string
  distro: string
  distro_id: string
  distro_version: string
  device_type: string
  virtualization: string
  is_container: boolean
  is_vm: boolean
  package_family: PackageFamily
  package_manager: string
  supported: boolean
  notes?: string[]
  packages: string[]
  optional_packages?: string[]
}

export type StatusReport = {
  ready: boolean
  binaries: Record<string, boolean>
  novnc_roots: Record<string, boolean>
  missing?: string[]
  plan: HostPlan
  options?: VncOptionsSnapshot
  install?: ActiveInstallState
}

export type VncOptionsSnapshot = {
  installed: boolean
  present: boolean
  missing: boolean
}

export type ActiveInstallLine = {
  text: string
  stream: string
  at: number
}

export type ActiveInstallState = {
  active: boolean
  job_id?: string
  status: "idle" | "running" | "success" | "error" | "cancelled" | string
  phase?: "waiting_queue" | "installing" | string
  waiting_queue?: boolean
  message?: string
  auto?: boolean
  started_at?: string
  lines?: ActiveInstallLine[]
  software_queue?: SoftwareQueueSnapshot
}

export type DetectResponse = {
  data: {
    plan: HostPlan
    status: StatusReport
    options?: VncOptionsSnapshot
    install?: ActiveInstallState
    software_queue?: SoftwareQueueSnapshot
    softwaresync_ready?: boolean
  }
}

export type StatusResponse = {
  data: StatusReport & {
    software_queue?: SoftwareQueueSnapshot
    softwaresync_ready?: boolean
  }
}

export type SetupStreamEvent = {
  type: "start" | "log" | "done" | "cancelled" | "error"
  job_id?: string
  line?: string
  stream?: "stdout" | "stderr" | "system"
  message?: string
  success?: boolean
  plan?: HostPlan | StatusReport
  software_queue?: SoftwareQueueSnapshot
}

export type InstallTerminalLine = {
  id: string
  text: string
  stream: "stdout" | "stderr" | "system"
  at: number
}

export async function detectVncSetup() {
  return ApiService.fetchData<DetectResponse>({
    url: `${VNC_SETUP_BASE}/detect`,
    method: "get",
  })
}

export async function getVncSetupStatus() {
  return ApiService.fetchData<StatusResponse>({
    url: `${VNC_SETUP_BASE}/status`,
    method: "get",
  })
}

export async function cancelVncSetupJob(jobId: string) {
  return ApiService.fetchData<{
    data: { job_id: string; cancelled: boolean }
    message?: string
  }>({
    url: `${VNC_SETUP_BASE}/jobs/${jobId}/cancel`,
    method: "post",
  })
}

export const VNC_SERVICE_FETCH_KEY = "vncservicestatus"
export const VNC_SERVICE_BASE = "/vnc-novnc/service"

export type VncServiceSession = {
  id: string
  user_id: string
  username: string
  status: string
  live: boolean
  address: string
  vnc_port: number
  no_vnc_port: number
  log_path?: string
}

export type VncServiceStatus = {
  packages_ready: boolean
  desired: "running" | "stopped" | string
  running: boolean
  live_sessions: number
  active_sessions: number
  sessions: VncServiceSession[]
  service_log: string
  checked_at: string
}

export type VncServiceStatusResponse = {
  data: VncServiceStatus
  message?: string
}

export type VncServiceActionResponse = {
  data: {
    action: {
      started: number
      stopped: number
      skipped: number
      errors?: string[]
      sessions: VncServiceSession[]
    }
    status: VncServiceStatus
  }
  message?: string
}

export async function getVncServiceStatus() {
  return ApiService.fetchData<VncServiceStatusResponse>({
    url: `${VNC_SERVICE_BASE}/status`,
    method: "get",
  })
}

export async function startVncService() {
  return ApiService.fetchData<VncServiceActionResponse>({
    url: `${VNC_SERVICE_BASE}/start`,
    method: "post",
  })
}

export async function stopVncService() {
  return ApiService.fetchData<VncServiceActionResponse>({
    url: `${VNC_SERVICE_BASE}/stop`,
    method: "post",
  })
}

export type ServiceLogEvent = {
  type: "start" | "log" | "done" | "error"
  path?: string
  line?: string
  stream?: "stdout" | "stderr" | "system"
  message?: string
}

/**
 * Streams aggregated VNC service + session logs over SSE.
 */
export async function streamVncServiceLogs(handlers: {
  onEvent: (event: ServiceLogEvent) => void
  signal?: AbortSignal
}): Promise<void> {
  const res = await fetch(`${baseApiURL()}${VNC_SERVICE_BASE}/logs/stream`, {
    method: "GET",
    headers: authHeaders(),
    signal: handlers.signal,
  })

  if (!res.ok) {
    let message = `Log stream failed (${res.status})`
    try {
      const body = (await res.json()) as { message?: string }
      if (body.message) message = body.message
    } catch {
      // ignore
    }
    throw new Error(message)
  }

  if (!res.body) {
    throw new Error("Log stream unavailable")
  }

  const reader = res.body.getReader()
  const decoder = new TextDecoder()
  let buffer = ""

  const emitBlock = (block: string) => {
    const lines = block.split("\n")
    for (const raw of lines) {
      const line = raw.trimEnd()
      if (!line.startsWith("data:")) continue
      const payload = line.slice(5).trimStart()
      if (!payload) continue
      try {
        handlers.onEvent(JSON.parse(payload) as ServiceLogEvent)
      } catch {
        // skip malformed frames
      }
    }
  }

  while (true) {
    const { done, value } = await reader.read()
    if (done) break
    buffer += decoder.decode(value, { stream: true })
    const parts = buffer.split("\n\n")
    buffer = parts.pop() ?? ""
    for (const part of parts) {
      emitBlock(part)
    }
  }

  if (buffer.trim()) {
    emitBlock(buffer)
  }
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

/**
 * Streams VNC/noVNC package setup over SSE.
 * Abort via `signal` or call `cancelVncSetupJob(jobId)` after `start`.
 */
export async function streamVncSetup(handlers: {
  onEvent: (event: SetupStreamEvent) => void
  signal?: AbortSignal
}): Promise<void> {
  const res = await fetch(`${baseApiURL()}${VNC_SETUP_BASE}/stream`, {
    method: "POST",
    headers: authHeaders(),
    signal: handlers.signal,
  })

  if (!res.ok) {
    let message = `Setup failed (${res.status})`
    try {
      const body = (await res.json()) as { message?: string }
      if (body.message) message = body.message
    } catch {
      // ignore parse errors
    }
    throw new Error(message)
  }

  if (!res.body) {
    throw new Error("Setup stream unavailable")
  }

  const reader = res.body.getReader()
  const decoder = new TextDecoder()
  let buffer = ""

  const emitBlock = (block: string) => {
    const lines = block.split("\n")
    for (const raw of lines) {
      const line = raw.trimEnd()
      if (!line.startsWith("data:")) continue
      const payload = line.slice(5).trimStart()
      if (!payload) continue
      try {
        handlers.onEvent(JSON.parse(payload) as SetupStreamEvent)
      } catch {
        // skip malformed frames
      }
    }
  }

  while (true) {
    const { done, value } = await reader.read()
    if (done) break
    buffer += decoder.decode(value, { stream: true })
    const parts = buffer.split("\n\n")
    buffer = parts.pop() ?? ""
    for (const part of parts) {
      emitBlock(part)
    }
  }

  if (buffer.trim()) {
    emitBlock(buffer)
  }
}

// --- Optional RDP (xrdp) setup — separate from VNC/noVNC packages ---

export const RDP_SETUP_FETCH_KEY = "rdpsetup"
export const RDP_SETUP_BASE = "/vnc-novnc/rdp/install/setup"

export type RdpStatusReport = {
  ready: boolean
  running: boolean
  binaries: Record<string, boolean>
  missing?: string[]
  port: number
  plan: HostPlan
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
    message?: string
  }>({
    url: `${RDP_SETUP_BASE}/jobs/${jobId}/cancel`,
    method: "post",
  })
}

export async function streamRdpSetup(handlers: {
  onEvent: (event: SetupStreamEvent) => void
  signal?: AbortSignal
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

  if (!res.body) {
    throw new Error("RDP setup stream unavailable")
  }

  const reader = res.body.getReader()
  const decoder = new TextDecoder()
  let buffer = ""

  const emitBlock = (block: string) => {
    const lines = block.split("\n")
    for (const raw of lines) {
      const line = raw.trimEnd()
      if (!line.startsWith("data:")) continue
      const payload = line.slice(5).trimStart()
      if (!payload) continue
      try {
        handlers.onEvent(JSON.parse(payload) as SetupStreamEvent)
      } catch {
        // skip malformed frames
      }
    }
  }

  while (true) {
    const { done, value } = await reader.read()
    if (done) break
    buffer += decoder.decode(value, { stream: true })
    const parts = buffer.split("\n\n")
    buffer = parts.pop() ?? ""
    for (const part of parts) {
      emitBlock(part)
    }
  }

  if (buffer.trim()) {
    emitBlock(buffer)
  }
}
