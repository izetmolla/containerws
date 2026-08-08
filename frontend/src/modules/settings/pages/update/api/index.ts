import ApiService from "@/lib/network"

export const SETTINGS_UPDATE_FETCH_KEY = "settingsupdate"
export const SETTINGS_UPDATE_BASE = "/settings/update"

export type UpdateStatus = {
  current_version: string
  commit_sha: string
  binary_path: string
  goos: string
  goarch: string
  repo: string
  expected_asset: string
  latest_tag: string
  update_available: boolean
  last_check: string
  last_error: string
  releases_count: number
}

export type UpdateRelease = {
  tag: string
  name: string
  published_at: string
  prerelease: boolean
  draft?: boolean
  html_url: string
  body?: string
  newer: boolean
  asset_name?: string
  asset_url?: string
  asset_size?: number
  has_asset: boolean
}

export type UpdateStatusResponse = {
  data: UpdateStatus
  message?: string
  error?: boolean
}

export type UpdateReleasesResponse = {
  data: {
    releases: UpdateRelease[]
    latest_tag: string
    last_check: string
    last_error: string
    repo: string
    goos: string
    goarch: string
  }
  error?: boolean
  message?: string
}

export type ApplyUpdateResponse = {
  data: {
    version: string
    binary_path: string
    asset_name: string
    restarting: boolean
    previous: string
    pid: number
  }
  message?: string
  error?: boolean
}

export async function getUpdateStatus() {
  return ApiService.fetchData<UpdateStatusResponse>({
    url: `${SETTINGS_UPDATE_BASE}/status`,
    method: "get",
  })
}

export async function checkForUpdates() {
  return ApiService.fetchData<UpdateStatusResponse>({
    url: `${SETTINGS_UPDATE_BASE}/check`,
    method: "post",
  })
}

export async function listUpdateReleases() {
  return ApiService.fetchData<UpdateReleasesResponse>({
    url: `${SETTINGS_UPDATE_BASE}/releases`,
    method: "get",
  })
}

export async function applyUpdate(version: string, force = false) {
  return ApiService.fetchData<ApplyUpdateResponse>({
    url: `${SETTINGS_UPDATE_BASE}/apply`,
    method: "post",
    data: { version, force },
    // Large downloads; keep client waiting.
    timeout: 900_000,
  })
}
