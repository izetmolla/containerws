import ApiService from "@/lib/network"
import { USERS_SINGLE_BASE } from "../../../list/api"

export const USER_KEYS_FETCH_KEY = "user-ssh-keys"
export const USER_SSH_SESSIONS_FETCH_KEY = "user-ssh-sessions"

export type AuthorizedKey = {
  index: number
  type: string
  fingerprint: string
  comment: string
  options?: string[]
  line: string
}

export type IdentityKey = {
  exists: boolean
  type?: string
  private_path?: string
  public_path?: string
  public_key?: string
  fingerprint?: string
  comment?: string
  private_key?: string
  has_private: boolean
  has_public: boolean
}

export type SSHKeysStatus = {
  username: string
  home_dir: string
  ssh_dir: string
  ssh_dir_exists: boolean
  authorized_keys_path: string
  authorized_keys: AuthorizedKey[]
  authorized_keys_count: number
  identity: IdentityKey
}

export type SSHConnection = {
  id: string
  username: string
  tty: string
  remote_host: string
  remote_port?: number
  local_addr?: string
  local_port?: number
  started_at?: string
  idle?: string
  pid: number
  shell_pid?: number
  shell_command?: string
  command?: string
  via_ssh: boolean
  kind?: "interactive" | "tunnel" | "local" | string
}

type KeysResponse = {
  data: SSHKeysStatus
  message?: string
}

type SessionsResponse = {
  data: SSHConnection[]
  message?: string
}

function keysBase(userId: string) {
  return `${USERS_SINGLE_BASE}/${userId}/keys`
}

export async function getUserKeys(userId: string, includePrivate = false) {
  const q = includePrivate ? "?include_private=1" : ""
  return ApiService.fetchData<KeysResponse>({
    url: `${keysBase(userId)}${q}`,
    method: "get",
  })
}

export async function addAuthorizedKey(
  userId: string,
  body: { key: string; comment?: string }
) {
  return ApiService.fetchData<KeysResponse>({
    url: `${keysBase(userId)}/authorized`,
    method: "post",
    data: body,
  })
}

export async function removeAuthorizedKey(userId: string, index: number) {
  return ApiService.fetchData<KeysResponse>({
    url: `${keysBase(userId)}/authorized/${index}`,
    method: "delete",
  })
}

export async function generateIdentityKey(
  userId: string,
  body: {
    type?: "ed25519" | "rsa"
    comment?: string
    passphrase?: string
    overwrite?: boolean
    bits?: number
  }
) {
  return ApiService.fetchData<KeysResponse>({
    url: `${keysBase(userId)}/identity`,
    method: "post",
    data: body,
  })
}

export async function deleteIdentityKey(userId: string) {
  return ApiService.fetchData<KeysResponse>({
    url: `${keysBase(userId)}/identity`,
    method: "delete",
  })
}

export async function authorizeIdentityKey(userId: string) {
  return ApiService.fetchData<KeysResponse>({
    url: `${keysBase(userId)}/identity/authorize`,
    method: "post",
  })
}

export async function listSSHSessions(userId: string) {
  return ApiService.fetchData<SessionsResponse>({
    url: `${keysBase(userId)}/sessions`,
    method: "get",
  })
}

export async function killSSHSession(userId: string, sessionId: string) {
  return ApiService.fetchData<SessionsResponse>({
    url: `${keysBase(userId)}/sessions/${encodeURIComponent(sessionId)}`,
    method: "delete",
  })
}
