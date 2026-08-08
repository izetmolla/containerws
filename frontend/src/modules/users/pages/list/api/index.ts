import ApiService from "@/lib/network"
import useAuthorizationStore, {
  getCurrentTokens,
} from "@/store/authorization"

export const USERS_FETCH_KEY = "userspage"
export const USERS_LIST_BASE = "/users/list"
export const USERS_SINGLE_BASE = "/users/single"

function currentAccessToken(): string {
  return getCurrentTokens(useAuthorizationStore.getState())?.access_token ?? ""
}

/** Build /novnc/vnc.html URL; session_id + access_token are embedded for WS upgrades. */
export function novncClientURL(sessionId: string, accessToken?: string): string {
  const id = sessionId.trim()
  const token = (accessToken ?? currentAccessToken()).trim()
  const wsQuery = new URLSearchParams({ session_id: id })
  if (token) wsQuery.set("access_token", token)

  const q = new URLSearchParams({
    session_id: id,
    path: `websockify?${wsQuery.toString()}`,
    autoconnect: "true",
    reconnect: "true",
    reconnect_delay: "2000",
    resize: "remote",
    quality: "9",
    compression: "0",
    show_dot: "false",
    view_only: "false",
    view_clip: "false",
    shared: "true",
    bell: "on",
    logging: "warn",
  })
  if (token) q.set("access_token", token)
  return `/novnc/vnc.html?${q.toString()}`
}

/** Inject the current JWT so /novnc works even without a WEB session cookie. */
export function withNovncAuth(url: string): string {
  const raw = stringsTrim(url)
  if (!raw) return raw
  const token = currentAccessToken()
  if (!token) return raw

  try {
    const base =
      typeof window !== "undefined" ? window.location.origin : "http://localhost"
    const parsed = new URL(raw, base)
    if (!parsed.pathname.startsWith("/novnc")) {
      return raw
    }
    parsed.searchParams.set("access_token", token)

    const pathParam = parsed.searchParams.get("path")
    if (pathParam) {
      const ws = new URL(pathParam, "http://novnc.local")
      ws.searchParams.set("access_token", token)
      const q = ws.searchParams.toString()
      parsed.searchParams.set(
        "path",
        q ? `${ws.pathname.replace(/^\//, "")}?${q}` : ws.pathname.replace(/^\//, "")
      )
    }

    return `${parsed.pathname}?${parsed.searchParams.toString()}`
  } catch {
    return raw
  }
}

function stringsTrim(s: string) {
  return s.trim()
}

/** Open noVNC in a new tab with panel JWT attached. */
export function openNovnc(url: string) {
  const target = withNovncAuth(url) || "/novnc"
  window.open(target, "_blank", "noopener,noreferrer")
}

export type UserListItem = {
  id: string
  username: string
  email: string
  first_name: string
  last_name: string
  full_name: string
  status: string
  roles: string[]
  is_confirmed: boolean
  linux_exists: boolean
  linux_groups?: string[]
  linux_shell?: string
  linux_home?: string
  linux_locked?: boolean
  vnc_session_id?: string
  vnc_status?: string
  vnc_port?: number
  no_vnc_port?: number
  has_vnc: boolean
  created_at: string
  updated_at: string
}

export type LinuxAccount = {
  username: string
  uid?: string
  gid?: string
  name?: string
  home_dir?: string
  shell?: string
  groups?: string[]
  primary_group?: string
  exists: boolean
  locked?: boolean
}

export type VncProfile = {
  id: string
  user_id: string
  status: string
  address: string
  vnc_port: number
  no_vnc_port: number
  has_password: boolean
  live?: boolean
  novnc_url: string
  geometry?: string
  depth?: number
  dpi?: number
  framerate?: number
  localhost_only?: boolean
  always_shared?: boolean
  accept_set_desktop_size?: boolean
  security_types?: string
  compare_fb?: number
  improved_hextile?: boolean
  desktop_session?: string
  quality?: number
  compression?: number
  autoconnect?: boolean
  reconnect?: boolean
  reconnect_delay?: number
  resize?: string
  view_only?: boolean
  show_dot?: boolean
  view_clip?: boolean
  shared?: boolean
  bell?: string
  logging?: string
  wallpaper_path?: string
  has_wallpaper?: boolean
  wallpaper_url?: string
  rdp_enabled?: boolean
  rdp_address?: string
  rdp_port?: number
  created_at?: string
  updated_at?: string
}

export type UserDetail = UserListItem & {
  organization_email?: string
  image?: string
  linux?: LinuxAccount
  vnc?: VncProfile
  novnc_url?: string
  terminal_url?: string
}

export type FormOptions = {
  groups: string[]
  system_groups?: string[]
  common_groups: string[]
  shells: string[]
  statuses: string[]
  panel_roles: string[]
}

export type CreateUserInput = {
  username: string
  email: string
  first_name: string
  last_name: string
  password: string
  status: string
  roles: string[]
  is_confirmed: boolean
  create_linux: boolean
  linux_password?: string
  linux_shell?: string
  linux_home?: string
  linux_groups?: string[]
  linux_primary_group?: string
  linux_create_home?: boolean
  create_vnc: boolean
  vnc_password?: string
  start_vnc?: boolean
}

export type UpdateUserInput = {
  username?: string
  email?: string
  first_name?: string
  last_name?: string
  organization_email?: string
  status?: string
  roles?: string[]
  is_confirmed?: boolean
}

type ListResponse = { data: UserListItem[] }
type OptionsResponse = { data: FormOptions }
type DetailResponse = { data: UserDetail; message?: string; novnc_url?: string }
type MutationResponse = {
  data?: UserDetail | LinuxAccount | VncProfile
  message?: string
  warnings?: string[]
  novnc_url?: string
  error?: boolean
}

export async function getUsersList() {
  return ApiService.fetchData<ListResponse>({
    url: USERS_LIST_BASE,
    method: "get",
  })
}

export async function getUserFormOptions() {
  return ApiService.fetchData<OptionsResponse>({
    url: `${USERS_LIST_BASE}/options`,
    method: "get",
  })
}

export async function getUser(id: string) {
  return ApiService.fetchData<DetailResponse>({
    url: `${USERS_SINGLE_BASE}/${id}`,
    method: "get",
  })
}

export async function createUser(body: CreateUserInput) {
  return ApiService.fetchData<MutationResponse>({
    url: USERS_SINGLE_BASE,
    method: "post",
    data: body,
  })
}

export async function updateUser(id: string, body: UpdateUserInput) {
  return ApiService.fetchData<MutationResponse>({
    url: `${USERS_SINGLE_BASE}/${id}`,
    method: "put",
    data: body,
  })
}

export async function deleteUser(
  id: string,
  opts?: { deleteLinux?: boolean; removeHome?: boolean }
) {
  const q = new URLSearchParams()
  if (opts?.deleteLinux) q.set("delete_linux", "true")
  if (opts?.removeHome) q.set("remove_home", "true")
  const qs = q.toString()
  return ApiService.fetchData<MutationResponse>({
    url: `${USERS_SINGLE_BASE}/${id}${qs ? `?${qs}` : ""}`,
    method: "delete",
  })
}

export async function setPanelPassword(id: string, password: string) {
  return ApiService.fetchData<MutationResponse>({
    url: `${USERS_SINGLE_BASE}/${id}/password`,
    method: "post",
    data: { password },
  })
}

export async function provisionLinux(
  id: string,
  body: {
    password?: string
    shell?: string
    home_dir?: string
    groups?: string[]
    primary_group?: string
    create_home?: boolean
  }
) {
  return ApiService.fetchData<MutationResponse>({
    url: `${USERS_SINGLE_BASE}/${id}/linux`,
    method: "post",
    data: body,
  })
}

export async function updateLinux(
  id: string,
  body: {
    shell?: string
    home_dir?: string
    groups?: string[]
    append_groups?: string[]
    password?: string
    lock?: boolean
    full_name?: string
  }
) {
  return ApiService.fetchData<MutationResponse>({
    url: `${USERS_SINGLE_BASE}/${id}/linux`,
    method: "put",
    data: body,
  })
}

export async function setLinuxGroups(id: string, groups: string[]) {
  return ApiService.fetchData<MutationResponse>({
    url: `${USERS_SINGLE_BASE}/${id}/linux/groups`,
    method: "post",
    data: { groups },
  })
}

export async function setLinuxPassword(id: string, password: string) {
  return ApiService.fetchData<MutationResponse>({
    url: `${USERS_SINGLE_BASE}/${id}/linux/password`,
    method: "post",
    data: { password },
  })
}

export async function lockLinux(id: string) {
  return ApiService.fetchData<MutationResponse>({
    url: `${USERS_SINGLE_BASE}/${id}/linux/lock`,
    method: "post",
  })
}

export async function unlockLinux(id: string) {
  return ApiService.fetchData<MutationResponse>({
    url: `${USERS_SINGLE_BASE}/${id}/linux/unlock`,
    method: "post",
  })
}

export async function createVncProfile(
  id: string,
  body: { vnc_password: string; start?: boolean }
) {
  return ApiService.fetchData<MutationResponse>({
    url: `${USERS_SINGLE_BASE}/${id}/vnc`,
    method: "post",
    data: body,
  })
}

export async function startVncProfile(id: string) {
  return ApiService.fetchData<MutationResponse>({
    url: `${USERS_SINGLE_BASE}/${id}/vnc/start`,
    method: "post",
  })
}

export async function stopVncProfile(id: string) {
  return ApiService.fetchData<MutationResponse>({
    url: `${USERS_SINGLE_BASE}/${id}/vnc/stop`,
    method: "post",
  })
}
