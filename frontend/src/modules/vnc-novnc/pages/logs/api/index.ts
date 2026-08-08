import ApiService, {
  REQUEST_HEADER_AUTH_KEY,
  TOKEN_TYPE,
} from "@/lib/network"
import { baseApiURL } from "@/lib/network/env"
import { getCurrentTokens } from "@/store/authorization"
import useAuthorizationStore from "@/store/authorization"

export const VNC_LOGS_FETCH_KEY = "vnclogspage"
export const VNC_SERVICE_BASE = "/vnc-novnc/service"

export type InstallTerminalLine = {
  id: string
  text: string
  stream: "stdout" | "stderr" | "system"
  at: number
}

export type ServiceLogEvent = {
  type: "start" | "log" | "done" | "error"
  path?: string
  line?: string
  stream?: "stdout" | "stderr" | "system"
  message?: string
}

export type SnapshotResponse = {
  data: {
    paths: string[]
    lines: string[]
  }
}

export async function getVncServiceLogSnapshot() {
  return ApiService.fetchData<SnapshotResponse>({
    url: `${VNC_SERVICE_BASE}/logs`,
    method: "get",
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
    for (const raw of block.split("\n")) {
      const line = raw.trimEnd()
      if (!line.startsWith("data:")) continue
      const payload = line.slice(5).trimStart()
      if (!payload) continue
      try {
        handlers.onEvent(JSON.parse(payload) as ServiceLogEvent)
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
    for (const part of parts) {
      emitBlock(part)
    }
  }

  if (buffer.trim()) {
    emitBlock(buffer)
  }
}
