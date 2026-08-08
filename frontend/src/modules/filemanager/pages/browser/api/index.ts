import ApiService from "@/lib/network"
import { downloadAuthenticatedBlob } from "@/lib/network/download-blob"

export const FILEMANAGER_FETCH_KEY = "filemanager"
export const FILEMANAGER_LIST_BASE = "/filemanager/list"
export const FILEMANAGER_OPS_BASE = "/filemanager/ops"

export type FileEntry = {
  name: string
  path: string
  type: "directory" | "file" | "symlink" | "fifo" | "socket" | "device" | "other"
  size: number
  mode: string
  mode_octal: string
  mod_time: string
  owner?: string
  group?: string
  uid?: number
  gid?: number
  readable: boolean
  writable: boolean
  executable: boolean
  hidden: boolean
  mime_hint?: string
}

export type FileRoot = {
  path: string
  label: string
  icon?: string
  group?: "places" | "disks" | "trash" | string
}

export type FileUser = {
  user_id: string
  username: string
  shell_user: string
  home_dir: string
  uid: number
  gid: number
  is_admin: boolean
  is_root_linux: boolean
}

export type ListResult = {
  path: string
  parent?: string
  exists: boolean
  is_dir: boolean
  entries: FileEntry[]
  total: number
  truncated?: boolean
  roots: FileRoot[]
  user: FileUser
  show_hidden: boolean
}

export type MutationResponse<T = unknown> = {
  data: T
  message?: string
}

export async function listDirectory(path: string, showHidden = false) {
  return ApiService.fetchData<{ data: ListResult }>({
    url: FILEMANAGER_LIST_BASE,
    method: "get",
    params: {
      ...(path ? { path } : {}),
      show_hidden: showHidden ? "1" : "0",
    },
  })
}

export async function statPath(path: string) {
  return ApiService.fetchData<{
    data: { entry?: FileEntry; exists: boolean; user: FileUser }
  }>({
    url: `${FILEMANAGER_LIST_BASE}/stat`,
    method: "get",
    params: { path },
  })
}

export async function mkdir(path: string, mode?: string) {
  return ApiService.fetchData<MutationResponse<{ path: string; created: boolean }>>({
    url: `${FILEMANAGER_OPS_BASE}/mkdir`,
    method: "post",
    data: { path, mode },
  })
}

export async function createFile(path: string, content = "", mode?: string) {
  return ApiService.fetchData<MutationResponse<{ path: string; created: boolean }>>({
    url: `${FILEMANAGER_OPS_BASE}/create`,
    method: "post",
    data: { path, content, mode },
  })
}

export async function renamePath(source: string, destination: string) {
  return ApiService.fetchData<
    MutationResponse<{ source: string; destination: string }>
  >({
    url: `${FILEMANAGER_OPS_BASE}/rename`,
    method: "post",
    data: { source, destination },
  })
}

export async function movePath(source: string, destination: string) {
  return ApiService.fetchData<
    MutationResponse<{ source: string; destination: string }>
  >({
    url: `${FILEMANAGER_OPS_BASE}/move`,
    method: "post",
    data: { source, destination },
  })
}

export async function copyPath(source: string, destination: string) {
  return ApiService.fetchData<
    MutationResponse<{ source: string; destination: string; bytes: number }>
  >({
    url: `${FILEMANAGER_OPS_BASE}/copy`,
    method: "post",
    data: { source, destination },
  })
}

export async function duplicatePath(path: string) {
  return ApiService.fetchData<
    MutationResponse<{ source: string; destination: string; bytes: number }>
  >({
    url: `${FILEMANAGER_OPS_BASE}/duplicate`,
    method: "post",
    data: { path },
  })
}

export async function deletePath(path: string, recursive = true) {
  return ApiService.fetchData<MutationResponse<{ path: string; deleted: boolean }>>({
    url: `${FILEMANAGER_OPS_BASE}/delete`,
    method: "post",
    data: { path, recursive },
  })
}

export type TrashItem = {
  id: string
  original_path: string
  name: string
  is_dir: boolean
  deleted_at: string
  expires_at: string
  days_left: number
  size?: number
  trash_path: string
}

export async function moveToTrash(path: string) {
  return ApiService.fetchData<
    MutationResponse<{
      id: string
      original_path: string
      name: string
      deleted_at: string
    }>
  >({
    url: `${FILEMANAGER_OPS_BASE}/trash`,
    method: "post",
    data: { path },
  })
}

export async function listTrash() {
  return ApiService.fetchData<{
    data: {
      path: string
      items: TrashItem[]
      retention: string
      total: number
    }
  }>({
    url: `${FILEMANAGER_OPS_BASE}/trash`,
    method: "get",
  })
}

export async function restoreTrashItem(id: string) {
  return ApiService.fetchData<
    MutationResponse<{
      id: string
      original_path: string
      restored_to: string
      name: string
    }>
  >({
    url: `${FILEMANAGER_OPS_BASE}/trash/restore`,
    method: "post",
    data: { id },
  })
}

export async function permanentlyDeleteTrashItem(id: string) {
  return ApiService.fetchData<MutationResponse<{ id: string; deleted: boolean }>>({
    url: `${FILEMANAGER_OPS_BASE}/trash/delete`,
    method: "post",
    data: { id },
  })
}

export async function emptyTrash() {
  return ApiService.fetchData<MutationResponse<{ deleted: number }>>({
    url: `${FILEMANAGER_OPS_BASE}/trash/empty`,
    method: "post",
    data: {},
  })
}

export function trashIdFromEntry(entry: FileEntry): string | null {
  const hint = entry.mime_hint || ""
  if (hint.startsWith("trash:")) return hint.slice("trash:".length)
  return null
}

export function isTrashPath(path: string, homeDir?: string) {
  if (!path) return false
  if (path.endsWith("/.containerws/trash") || path === "/.containerws/trash") {
    return true
  }
  if (homeDir) {
    const trash = `${homeDir.replace(/\/+$/, "")}/.containerws/trash`
    return path === trash
  }
  return /\.containerws\/trash$/.test(path)
}

export async function chmodPath(path: string, mode: string) {
  return ApiService.fetchData<
    MutationResponse<{ path: string; mode: string; mode_octal: string }>
  >({
    url: `${FILEMANAGER_OPS_BASE}/chmod`,
    method: "post",
    data: { path, mode },
  })
}

export async function uploadFile(dir: string, file: File) {
  return ApiService.uploadFileData<
    MutationResponse<{ path: string; bytes: number; name: string }>
  >(`${FILEMANAGER_OPS_BASE}/upload`, file, {
    body: { path: dir },
  })
}

export async function readFile(path: string) {
  return ApiService.fetchData<{
    data: {
      path: string
      content: string
      size: number
      truncated?: boolean
      mode: string
    }
  }>({
    url: `${FILEMANAGER_OPS_BASE}/read`,
    method: "get",
    params: { path },
  })
}

export async function writeFile(path: string, content: string) {
  return ApiService.fetchData<MutationResponse<{ path: string; bytes: number }>>({
    url: `${FILEMANAGER_OPS_BASE}/write`,
    method: "post",
    data: { path, content },
  })
}

export async function downloadFile(path: string) {
  const name = path.split("/").filter(Boolean).pop() || "download"
  await downloadAuthenticatedBlob(
    `${FILEMANAGER_OPS_BASE}/download`,
    { path },
    name
  )
}

export function formatBytes(n: number) {
  if (!Number.isFinite(n) || n < 0) return "—"
  if (n < 1024) return `${n} B`
  const units = ["KB", "MB", "GB", "TB"]
  let v = n
  let i = -1
  do {
    v /= 1024
    i += 1
  } while (v >= 1024 && i < units.length - 1)
  return `${v.toFixed(v >= 10 || i === 0 ? 0 : 1)} ${units[i]}`
}

export function formatModTime(iso: string) {
  if (!iso) return "—"
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return iso
  const now = new Date()
  const sameYear = d.getFullYear() === now.getFullYear()
  return d.toLocaleString(undefined, {
    year: sameYear ? undefined : "numeric",
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  })
}
