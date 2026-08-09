import ApiService, {
  REQUEST_HEADER_AUTH_KEY,
  TOKEN_TYPE,
} from "@/lib/network"
import { baseApiURL } from "@/lib/network/env"
import { getCurrentTokens } from "@/store/authorization"
import useAuthorizationStore from "@/store/authorization"

import {
  SOFTWARES_FETCH_KEY,
  SOFTWARES_INSTALL_BASE,
  SOFTWARES_SINGLE_BASE,
  type Software,
  type SoftwareVersion,
} from "../../list/api"

export { SOFTWARES_FETCH_KEY }

export type SoftwareSingleResponse = {
  data: {
    software: Software
    versions: SoftwareVersion[]
    latest_version: SoftwareVersion | null
    installed_version?: SoftwareVersion | null
    is_installed?: boolean
    uninstalled?: boolean
    has_update?: boolean
    can_uninstall?: boolean
    can_control?: boolean
    os_missing?: boolean
    service_status?: import("../../list/api").ServiceStatus | null
    from_remote?: boolean
    synced_from_remote?: boolean
    sync_error?: string
    package_manager?: string
    brew_available?: boolean
    can_switch_to_brew?: boolean
    can_switch_to_local?: boolean
    brew_token?: string
  }
}

export type SoftwareInstallResponse = {
  data: {
    software: Software
    latest_version: SoftwareVersion
    success: boolean
    stdout?: string
    stderr?: string
    error?: string
  }
  message?: string
}

export type InstallStreamEvent = {
  type: "start" | "system" | "log" | "done" | "cancelled" | "error"
  job_id?: string
  line?: string
  stream?: "stdout" | "stderr" | "system"
  message?: string
  success?: boolean
  version?: string
  name?: string
}

export type InstallTerminalLine = {
  id: string
  text: string
  stream: "stdout" | "stderr" | "system"
  at: number
}

export type InstallJobSnapshot = {
  id: string
  software_id: string
  software_name?: string
  version_id?: string
  version?: string
  status: "running" | "success" | "error" | "cancelled" | string
  message?: string
  failure_reason?: string
  exit_code?: number | null
  lines: {
    stream: "stdout" | "stderr" | "system" | string
    text: string
    at: number
  }[]
  started_at?: string
  finished_at?: string | null
}

export type InstallJobResponse = {
  data: InstallJobSnapshot | null
}

/** Load software detail; by default syncs metadata from the remote registry. */
export async function getSoftwareSingle(id: string, opts?: { sync?: boolean }) {
  const sync = opts?.sync !== false
  return ApiService.fetchData<SoftwareSingleResponse>({
    url: `${SOFTWARES_SINGLE_BASE}/${id}`,
    method: "get",
    params: sync ? { sync: 1 } : { sync: 0 },
  })
}

export async function installSoftware(id: string) {
  return ApiService.fetchData<SoftwareInstallResponse>({
    url: `${SOFTWARES_INSTALL_BASE}/${id}`,
    method: "post",
  })
}

export async function getLatestInstallJob(softwareId: string) {
  return ApiService.fetchData<InstallJobResponse>({
    url: `${SOFTWARES_INSTALL_BASE}/${softwareId}/job`,
    method: "get",
  })
}

export async function cancelInstallJob(jobId: string) {
  return ApiService.fetchData<{
    data: { job_id: string; cancelled: boolean }
    message?: string
  }>({
    url: `${SOFTWARES_INSTALL_BASE}/jobs/${jobId}/cancel`,
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

async function readSSEStream(
  res: Response,
  onEvent: (event: InstallStreamEvent) => void
): Promise<void> {
  if (!res.body) {
    throw new Error("Install stream unavailable")
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
        onEvent(JSON.parse(payload) as InstallStreamEvent)
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

/**
 * Streams install progress over SSE. Prefer this for the terminal UI.
 * Abort via `signal` or call `cancelInstallJob(jobId)` after `start`.
 * Disconnecting does not cancel the server-side job.
 */
export async function streamInstallSoftware(
  id: string,
  handlers: {
    onEvent: (event: InstallStreamEvent) => void
    signal?: AbortSignal
  }
): Promise<void> {
  const res = await fetch(
    `${baseApiURL()}${SOFTWARES_INSTALL_BASE}/${id}/stream`,
    {
      method: "POST",
      headers: authHeaders(),
      signal: handlers.signal,
    }
  )

  if (!res.ok) {
    let message = `Install failed (${res.status})`
    try {
      const body = (await res.json()) as { message?: string }
      if (body.message) message = body.message
    } catch {
      // ignore parse errors
    }
    throw new Error(message)
  }

  await readSSEStream(res, handlers.onEvent)
}

/** Reattach to an existing install job (replay + live tail). */
export async function streamInstallJob(
  jobId: string,
  handlers: {
    onEvent: (event: InstallStreamEvent) => void
    signal?: AbortSignal
  }
): Promise<void> {
  const res = await fetch(
    `${baseApiURL()}${SOFTWARES_INSTALL_BASE}/jobs/${jobId}/stream`,
    {
      method: "GET",
      headers: authHeaders(),
      signal: handlers.signal,
    }
  )

  if (!res.ok) {
    let message = `Could not resume install (${res.status})`
    try {
      const body = (await res.json()) as { message?: string }
      if (body.message) message = body.message
    } catch {
      // ignore
    }
    throw new Error(message)
  }

  await readSSEStream(res, handlers.onEvent)
}

export type ServiceLogStreamEvent = {
  type: "start" | "log" | "done" | "error"
  software_id?: string
  name?: string
  units?: string[]
  unit?: string
  line?: string
  at?: string
  stream?: string
  message?: string
}

/** Live journalctl follow for softwares with service_units. */
export async function streamSoftwareServiceLogs(
  id: string,
  handlers: {
    onEvent: (event: ServiceLogStreamEvent) => void
    signal?: AbortSignal
    lines?: number
  }
): Promise<void> {
  const q = new URLSearchParams()
  if (handlers.lines) q.set("lines", String(handlers.lines))
  const qs = q.toString()
  const res = await fetch(
    `${baseApiURL()}/softwares/service/${id}/logs/stream${qs ? `?${qs}` : ""}`,
    {
      method: "GET",
      headers: authHeaders(),
      signal: handlers.signal,
    }
  )

  if (!res.ok) {
    let message = `Could not stream service logs (${res.status})`
    try {
      const body = (await res.json()) as { message?: string }
      if (body.message) message = body.message
    } catch {
      // ignore
    }
    throw new Error(message)
  }

  await readSSEStream(res, (ev) => {
    handlers.onEvent(ev as unknown as ServiceLogStreamEvent)
  })
}
