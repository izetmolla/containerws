import ApiService from "@/lib/network"
import { baseApiURL } from "@/lib/network/env"
import { getCurrentTokens } from "@/store/authorization"
import useAuthorizationStore from "@/store/authorization"

export const CLOUDSHELL_FETCH_KEY = "cloudshellpage"
export const CLOUDSHELL_SESSION_BASE = "/cloudshell/session"
export const CLOUDSHELL_SESSIONS_BASE = "/cloudshell/sessions"
export const CLOUDSHELL_WS_PATH = "/cloudshell/ws"

/** @deprecated Use CLOUDSHELL_* constants */
export const CLI_FETCH_KEY = CLOUDSHELL_FETCH_KEY
/** @deprecated */
export const CLI_SESSION_BASE = CLOUDSHELL_SESSION_BASE
/** @deprecated */
export const CLI_SESSIONS_BASE = CLOUDSHELL_SESSIONS_BASE
/** @deprecated */
export const CLI_WS_PATH = CLOUDSHELL_WS_PATH

export type CliUser = {
  user_id: string
  username: string
  display_name: string
  email?: string
  shell_user: string
  home_dir: string
  shell: string
  cwd: string
}

export type CliSessionResponse = {
  data: {
    user: CliUser
    hostname: string
    ws_path: string
  }
}

export type CliLiveSession = {
  id: string
  title: string
  created_at: string
  last_active: string
  attached: boolean
  alive: boolean
  status?: string
  shell_user?: string
  cwd?: string
  cols?: number
  rows?: number
}

export async function getCliSession() {
  return ApiService.fetchData<CliSessionResponse>({
    url: CLOUDSHELL_SESSION_BASE,
    method: "get",
  })
}

export async function listCliSessions() {
  return ApiService.fetchData<{ data: CliLiveSession[] }>({
    url: CLOUDSHELL_SESSIONS_BASE,
    method: "get",
  })
}

export async function killCliSession(sessionId: string) {
  return ApiService.fetchData<{ data: { id: string; killed: boolean } }>({
    url: `${CLOUDSHELL_SESSIONS_BASE}/${sessionId}`,
    method: "delete",
  })
}

export function buildCliWebSocketURL(opts?: {
  sessionId?: string | null
  title?: string | null
  asUser?: string | null
}): string {
  const tokens = getCurrentTokens(useAuthorizationStore.getState())
  const apiBase = baseApiURL() // http(s)://host/api
  const wsBase = apiBase.replace(/^http/i, "ws")
  const url = new URL(`${wsBase}${CLOUDSHELL_WS_PATH}`)
  if (tokens?.access_token) {
    url.searchParams.set("access_token", tokens.access_token)
  }
  if (opts?.sessionId) {
    url.searchParams.set("session_id", opts.sessionId)
  }
  if (opts?.title) {
    url.searchParams.set("title", opts.title)
  }
  const asUser =
    opts?.asUser ||
    (typeof window !== "undefined"
      ? new URLSearchParams(window.location.search).get("as_user")
      : null)
  if (asUser) {
    url.searchParams.set("as_user", asUser)
  }
  return url.toString()
}
