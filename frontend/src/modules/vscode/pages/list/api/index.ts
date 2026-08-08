import ApiService, {
  REQUEST_HEADER_AUTH_KEY,
  TOKEN_TYPE,
} from "@/lib/network"
import { baseApiURL } from "@/lib/network/env"
import { getCurrentTokens } from "@/store/authorization"
import useAuthorizationStore from "@/store/authorization"

export const VSCODE_SESSIONS_FETCH_KEY = "vscodesessionspage"
export const VSCODE_SESSIONS_LIST_BASE = "/vscode/list"
export const VSCODE_SESSIONS_SINGLE_BASE = "/vscode/single"

export type CodeserverSessionStatus = "active" | "inactive" | string

export type CodeserverSessionListItem = {
  id: string
  user_id: string
  username: string
  email: string
  full_name: string
  name: string
  status: CodeserverSessionStatus
  path: string
  address: string
  port: number
  pid: number
  live: boolean
  connect_url: string
  created_at: string
  updated_at: string
}

export type AvailableUser = {
  id: string
  username: string
  email: string
  full_name: string
  label: string
}

export type CreateCodeserverSessionInput = {
  user_id?: string
  name?: string
  path?: string
  address?: string
  port?: number
  status?: string
  git_repo?: string
  git_branch?: string
  git_token?: string
}

export type UpdateCodeserverSessionInput = {
  name?: string
  path?: string
  address?: string
  port?: number
  status?: string
}

type ListResponse = {
  data: CodeserverSessionListItem[]
  is_admin?: boolean
}

type UsersResponse = {
  data: AvailableUser[]
}

type MutationResponse = {
  data?: CodeserverSessionListItem
  message?: string
  connect_url?: string
  error?: boolean
}

export type CodeserverRuntimeStatus = {
  installed: boolean
  option_installed?: boolean
  present?: boolean
  missing?: boolean
  detail?: string
  software_id?: string
  software_name?: string
  cli_command?: string
  software_queue?: import("@/modules/softwares/pages/list/api").SoftwareQueueSnapshot
  softwaresync_ready?: boolean
}

type StatusResponse = {
  data: CodeserverRuntimeStatus
}

export async function getCodeserverStatus() {
  return ApiService.fetchData<StatusResponse>({
    url: `${VSCODE_SESSIONS_LIST_BASE}/status`,
    method: "get",
  })
}

export type PathRoot = {
  path: string
  label: string
}

export type PathEntry = {
  name: string
  path: string
  has_children?: boolean
}

export type PathBrowseResult = {
  path: string
  parent?: string
  exists: boolean
  is_dir: boolean
  will_create: boolean
  prefix?: string
  entries: PathEntry[]
  roots: PathRoot[]
  admin: boolean
}

type PathBrowseResponse = {
  data: PathBrowseResult
}

/** Lists directories for the workspace-folder picker (role-scoped). */
export async function browseCodeserverPaths(path: string, userId?: string) {
  const params: Record<string, string> = {
    path: path.trim() || "/workspace",
  }
  if (userId?.trim()) {
    params.user_id = userId.trim()
  }
  return ApiService.fetchData<PathBrowseResponse>({
    url: `${VSCODE_SESSIONS_LIST_BASE}/paths`,
    method: "get",
    params,
  })
}

export async function getCodeserverSessionsList(options?: { mine?: boolean }) {
  const params = options?.mine ? { mine: "1" } : undefined
  return ApiService.fetchData<ListResponse>({
    url: VSCODE_SESSIONS_LIST_BASE,
    method: "get",
    params,
  })
}

export async function getAvailableUsers() {
  return ApiService.fetchData<UsersResponse>({
    url: `${VSCODE_SESSIONS_LIST_BASE}/available-users`,
    method: "get",
  })
}

export async function createCodeserverSession(
  body: CreateCodeserverSessionInput
) {
  return ApiService.fetchData<MutationResponse>({
    url: VSCODE_SESSIONS_SINGLE_BASE,
    method: "post",
    data: body,
  })
}

export async function updateCodeserverSession(
  id: string,
  body: UpdateCodeserverSessionInput
) {
  return ApiService.fetchData<MutationResponse>({
    url: `${VSCODE_SESSIONS_SINGLE_BASE}/${id}`,
    method: "put",
    data: body,
  })
}

export async function deleteCodeserverSession(id: string) {
  return ApiService.fetchData<MutationResponse>({
    url: `${VSCODE_SESSIONS_SINGLE_BASE}/${id}`,
    method: "delete",
  })
}

export async function disableCodeserverSession(id: string) {
  return ApiService.fetchData<MutationResponse>({
    url: `${VSCODE_SESSIONS_SINGLE_BASE}/${id}/disable`,
    method: "post",
  })
}

export async function enableCodeserverSession(id: string) {
  return ApiService.fetchData<MutationResponse>({
    url: `${VSCODE_SESSIONS_SINGLE_BASE}/${id}/enable`,
    method: "post",
  })
}

export async function openCodeserverSession(id: string) {
  return ApiService.fetchData<MutationResponse>({
    url: `${VSCODE_SESSIONS_SINGLE_BASE}/${id}/open`,
    method: "post",
  })
}

/** One-click: create/reuse session for the current user and open a folder in VS Code. */
export async function openCodeserverEditor(options?: {
  path?: string
  shellSessionId?: string
}) {
  const data: { path?: string; shell_session_id?: string } = {}
  if (options?.path?.trim()) data.path = options.path.trim()
  if (options?.shellSessionId?.trim()) {
    data.shell_session_id = options.shellSessionId.trim()
  }
  return ApiService.fetchData<MutationResponse & { reused?: boolean }>({
    url: `${VSCODE_SESSIONS_SINGLE_BASE}/open-editor`,
    method: "post",
    data,
  })
}

export type CreateSessionStreamEvent = {
  type: "start" | "system" | "log" | "done" | "error"
  line?: string
  stream?: "stdout" | "stderr" | "system"
  message?: string
  success?: boolean
  connect_url?: string
  data?: CodeserverSessionListItem
}

function authHeaders(): HeadersInit {
  const tokens = getCurrentTokens(useAuthorizationStore.getState())
  const headers: Record<string, string> = {
    Accept: "text/event-stream",
    "Content-Type": "application/json",
  }
  if (tokens?.access_token) {
    headers[REQUEST_HEADER_AUTH_KEY] = `${TOKEN_TYPE}${tokens.access_token}`
  }
  return headers
}

/** Streams create+start progress over SSE for the New workspace wizard. */
export async function streamCreateCodeserverSession(
  body: CreateCodeserverSessionInput,
  handlers: {
    onEvent: (event: CreateSessionStreamEvent) => void
    signal?: AbortSignal
  }
): Promise<void> {
  const res = await fetch(
    `${baseApiURL()}${VSCODE_SESSIONS_SINGLE_BASE}/stream`,
    {
      method: "POST",
      headers: authHeaders(),
      body: JSON.stringify(body),
      signal: handlers.signal,
    }
  )

  if (!res.ok) {
    let message = `Create workspace failed (${res.status})`
    try {
      const payload = (await res.json()) as { message?: string }
      if (payload.message) message = payload.message
    } catch {
      // ignore
    }
    throw new Error(message)
  }

  if (!res.body) {
    throw new Error("Create workspace stream unavailable")
  }

  await readSessionStream(res.body, handlers.onEvent)
}

/** Streams reactivate/restart progress for an existing workspace. */
export async function streamReactivateCodeserverSession(
  id: string,
  handlers: {
    onEvent: (event: CreateSessionStreamEvent) => void
    signal?: AbortSignal
  }
): Promise<void> {
  const res = await fetch(
    `${baseApiURL()}${VSCODE_SESSIONS_SINGLE_BASE}/${id}/reactivate/stream`,
    {
      method: "POST",
      headers: authHeaders(),
      signal: handlers.signal,
    }
  )

  if (!res.ok) {
    let message = `Start workspace failed (${res.status})`
    try {
      const payload = (await res.json()) as { message?: string }
      if (payload.message) message = payload.message
    } catch {
      // ignore
    }
    throw new Error(message)
  }

  if (!res.body) {
    throw new Error("Start workspace stream unavailable")
  }

  await readSessionStream(res.body, handlers.onEvent)
}

async function readSessionStream(
  body: ReadableStream<Uint8Array>,
  onEvent: (event: CreateSessionStreamEvent) => void
) {
  const reader = body.getReader()
  const decoder = new TextDecoder()
  let buffer = ""

  const emitBlock = (block: string) => {
    for (const raw of block.split("\n")) {
      const line = raw.trimEnd()
      if (!line.startsWith("data:")) continue
      const payload = line.slice(5).trimStart()
      if (!payload) continue
      try {
        onEvent(JSON.parse(payload) as CreateSessionStreamEvent)
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
