import ApiService, {
  type ResponseWithPagination,
} from "@/lib/network"

export const SOFTWARES_FETCH_KEY = "softwarespage"
export const SOFTWARES_LIST_BASE = "/softwares/list"
export const SOFTWARES_SINGLE_BASE = "/softwares/single"
export const SOFTWARES_INSTALL_BASE = "/softwares/install"
export const SOFTWARES_SERVICE_BASE = "/softwares/service"

export type SoftwareVersion = {
  id: string
  software_id: string
  version: string
  is_latest: boolean
  is_installed?: boolean
  has_update?: boolean
  can_uninstall?: boolean
  os_missing?: boolean
  install_script?: string
  uninstall_script?: string
  upgrade_script?: string
  custom_script?: string
  os?: string
  os_version?: string
  distro?: string
  distro_id?: string
  distro_version?: string
  arch?: string
  platform?: string
  package_family?: string
  kernel?: string
  virtualization?: string
  container_runtime?: string
  cloud_provider?: string
  created_at?: string
  updated_at?: string
}

export type Software = {
  id: string
  name: string
  details: string
  category: string
  sub_category: string
  tags: string[]
  service_units?: string[]
  can_control?: boolean
  control_backend?: string
  start_command?: string
  restart_command?: string
  stop_command?: string
  icon: string
  image?: string
  color: string
  order: number
  is_active: boolean
  registry_package_id?: string
  registry_slug?: string
  created_at?: string
  updated_at?: string
}

export type ServiceUnitStatus = {
  unit: string
  active: string
  sub?: string
  description?: string
  error?: string
}

export type ServiceStatus = {
  managed: boolean
  overall:
    | "running"
    | "stopped"
    | "partial"
    | "failed"
    | "unmanaged"
    | "unavailable"
    | string
  units: ServiceUnitStatus[]
}

export type SoftwareListItem = Software & {
  latest_version: SoftwareVersion | null
  installed_version?: SoftwareVersion | null
  is_installed?: boolean
  uninstalled?: boolean
  has_update?: boolean
  can_uninstall?: boolean
  can_control?: boolean
  os_missing?: boolean
  service_status?: ServiceStatus | null
  source?: "local" | "remote" | "both" | string
  is_remote?: boolean
  package_id?: string
  package_manager?: "local" | "brew" | string
  brew_available?: boolean
  can_switch_to_brew?: boolean
  can_switch_to_local?: boolean
}

export function isBrewManaged(item: {
  package_manager?: string | null
}): boolean {
  return item.package_manager === "brew"
}

export type SoftwaresListParams = {
  page?: number
  limit?: number
  q?: string
  category?: string
  status?:
    | "all"
    | "installed"
    | "uninstalled"
    | "update_available"
    | "not_installed"
  sort?: "order" | "name" | "category" | "recent"
  source?: "all" | "local" | "remote"
  /** Force re-fetch of remote registry indexes (search / remote source). */
  refresh?: boolean
}

export type SoftwaresListFacets = {
  categories: { name: string; count: number }[]
  update_count: number
  total_active: number
  remote_count?: number
}

export type SoftwaresListHost = {
  os?: string
  distro?: string
  distro_id?: string
  distro_version?: string
  arch?: string
  platform?: string
  package_family?: string
}

export type SoftwaresListResponse = ResponseWithPagination<SoftwareListItem> & {
  facets?: SoftwaresListFacets
  host?: SoftwaresListHost
}

export type SoftwareServiceAction = "start" | "stop" | "restart"

export type SoftwareServiceResponse = {
  software_id: string
  name: string
  can_control?: boolean
  status: ServiceStatus
  error?: boolean
  message?: string
}

export type SoftwareInstallStatus =
  | "not_installed"
  | "installed"
  | "uninstalled"
  | "missing"
  | "update_available"
  | "installing"
  | "updating"

export function softwareInstallStatus(
  item: Pick<
    SoftwareListItem,
    "is_installed" | "has_update" | "os_missing" | "uninstalled"
  >,
  busy?: "installing" | "updating" | null
): SoftwareInstallStatus {
  if (busy === "installing") return "installing"
  if (busy === "updating") return "updating"
  if (item.uninstalled) return "uninstalled"
  if (item.is_installed && item.os_missing) return "missing"
  if (item.has_update) return "update_available"
  if (item.is_installed) return "installed"
  return "not_installed"
}

export async function getSoftwaresList(params: SoftwaresListParams = {}) {
  const {
    page = 1,
    limit = 12,
    q = "",
    category = "all",
    status = "all",
    sort = "order",
    source = "local",
    refresh = false,
  } = params

  const wantsRemote =
    refresh || Boolean(q.trim()) || source === "remote" || source === "all"

  return ApiService.fetchData<SoftwaresListResponse>({
    url: SOFTWARES_LIST_BASE,
    method: "get",
    params: {
      page,
      limit,
      source,
      ...(q.trim() ? { q: q.trim() } : {}),
      ...(category && category !== "all" ? { category } : {}),
      ...(status && status !== "all" ? { status } : {}),
      ...(sort && sort !== "order" ? { sort } : {}),
      ...(wantsRemote ? { refresh: 1 } : {}),
    },
  })
}

export type ImportRemoteSoftwareResponse = {
  data: {
    software: Software
    version: SoftwareVersion
    created_sw?: boolean
    created_ver?: boolean
  }
  message?: string
}

/** Import a remote registry package into the local catalog. */
export async function importRemoteSoftware(opts: {
  name: string
  packageId?: string
  ref?: string
}) {
  return ApiService.fetchData<ImportRemoteSoftwareResponse>({
    url: "/softwares/package/import",
    method: "post",
    data: {
      name: opts.name,
      ...(opts.packageId ? { package_id: opts.packageId } : {}),
      ...(opts.ref ? { ref: opts.ref } : {}),
    },
  })
}

export async function controlSoftwareService(
  id: string,
  action: SoftwareServiceAction
) {
  return ApiService.fetchData<SoftwareServiceResponse>({
    url: `${SOFTWARES_SERVICE_BASE}/${id}/${action}`,
    method: "post",
  })
}

export async function getSoftwareServiceStatus(id: string) {
  return ApiService.fetchData<SoftwareServiceResponse>({
    url: `${SOFTWARES_SERVICE_BASE}/${id}/status`,
    method: "get",
  })
}

export type ServiceLogLine = {
  unit?: string
  text: string
  at?: string
}

export type ServiceLogsResponse = {
  software_id: string
  name: string
  units: string[]
  lines: ServiceLogLine[]
}

export async function getSoftwareServiceLogs(id: string, lines = 120) {
  return ApiService.fetchData<ServiceLogsResponse>({
    url: `${SOFTWARES_SERVICE_BASE}/${id}/logs`,
    method: "get",
    params: { lines },
  })
}

export type SoftwareQueueAction = "install" | "update" | "uninstall"

export type SoftwareQueueItem = {
  id: string
  software_id: string
  software_name: string
  action: SoftwareQueueAction | string
  status: "pending" | "running" | "success" | "error" | "skipped" | string
  job_id?: string
  message?: string
  icon?: string
  image?: string
  color?: string
  category?: string
  version?: string
  enqueued_at?: string
  finished_at?: string | null
}

export type SoftwareQueueSnapshot = {
  running: boolean
  pending: number
  items: SoftwareQueueItem[]
}

export type SoftwareEnqueueResponse = {
  data: {
    queued: number
    action: SoftwareQueueAction | string
    items: SoftwareQueueItem[]
    queue: SoftwareQueueSnapshot
  }
  message?: string
}

/** Queue bulk install/update/uninstall — executed one-by-one server-side. */
export async function enqueueSoftwareActions(
  action: SoftwareQueueAction,
  softwareIds: string[]
) {
  return ApiService.fetchData<SoftwareEnqueueResponse>({
    url: `${SOFTWARES_INSTALL_BASE}/queue`,
    method: "post",
    data: {
      action,
      software_ids: softwareIds,
    },
  })
}

export async function getSoftwareQueue() {
  return ApiService.fetchData<{ data: SoftwareQueueSnapshot }>({
    url: `${SOFTWARES_INSTALL_BASE}/queue`,
    method: "get",
  })
}

export async function retrySoftwareQueueItem(opts: {
  id?: string
  softwareId: string
  action?: SoftwareQueueAction
}) {
  return ApiService.fetchData<{
    data: { item: SoftwareQueueItem; queue: SoftwareQueueSnapshot }
    message?: string
  }>({
    url: `${SOFTWARES_INSTALL_BASE}/queue/retry`,
    method: "post",
    data: {
      ...(opts.id ? { id: opts.id } : {}),
      software_id: opts.softwareId,
      action: opts.action || "install",
    },
  })
}
