import ApiService from "@/lib/network"

export const DASHBOARD_FETCH_KEY = "dashboard"
export const DASHBOARD_METRICS_BASE = "/dashboard/metrics"
export const DASHBOARD_TOOLS_BASE = "/dashboard/tools"
export const DASHBOARD_PROCESSES_BASE = "/dashboard/processes"

export type HostMetrics = {
  hostname: string
  primary_ip: string
  default_iface: string
  os: string
  kernel: string
  distro: string
  distro_version: string
  arch: string
  virtualization: string
  is_containerized: boolean
  is_virtual_machine: boolean
}

export type CPUMetrics = {
  cores: number
  model: string
  usage_percent: number
  load1: number
  load5: number
  load15: number
  per_core_percent: number[]
}

export type MemoryMetrics = {
  total_bytes: number
  used_bytes: number
  available_bytes: number
  free_bytes: number
  buffers_bytes: number
  cached_bytes: number
  swap_total_bytes: number
  swap_used_bytes: number
  used_percent: number
  swap_percent: number
  total_human: string
  used_human: string
  available_human: string
  cached_human: string
  buffers_human: string
  swap_total_human: string
  swap_used_human: string
}

export type DiskMetrics = {
  mount: string
  device: string
  fstype: string
  total_bytes: number
  used_bytes: number
  free_bytes: number
  used_percent: number
  total_human: string
  used_human: string
  free_human: string
}

export type NetworkIface = {
  name: string
  rx_bytes: number
  tx_bytes: number
  rx_packets: number
  tx_packets: number
  rx_errors: number
  tx_errors: number
  rx_dropped: number
  tx_dropped: number
  rx_rate_bps: number
  tx_rate_bps: number
  rx_rate_human: string
  tx_rate_human: string
  rx_human: string
  tx_human: string
}

export type NetworkMetrics = {
  rx_bytes_total: number
  tx_bytes_total: number
  rx_rate_bps: number
  tx_rate_bps: number
  rx_rate_human: string
  tx_rate_human: string
  interfaces: NetworkIface[]
}

export type GPUConnector = {
  name: string
  type: string
  status: string
  enabled: string
}

export type GPUMetrics = {
  index: number
  name: string
  vendor: string
  vendor_id: string
  device_id: string
  pci_slot: string
  driver: string
  drm_card: string
  boot_vga: boolean
  brand: string
  memory_total_bytes: number
  memory_used_bytes: number
  memory_free_bytes: number
  memory_total_human: string
  memory_used_human: string
  memory_free_human: string
  memory_percent: number
  utilization_percent: number
  memory_util_percent: number
  temperature_c: number
  power_draw_w: number
  power_limit_w: number
  fan_speed_percent: number
  clock_graphics_mhz: number
  clock_max_graphics_mhz: number
  clock_min_graphics_mhz: number
  clock_memory_mhz: number
  uuid: string
  driver_version: string
  cuda_version?: string
  compute_capability: string
  connectors: GPUConnector[]
}

export type DashboardMetrics = {
  collected_at: string
  host: HostMetrics
  cpu: CPUMetrics
  memory: MemoryMetrics
  disks: DiskMetrics[]
  network: NetworkMetrics
  gpus: GPUMetrics[]
  uptime_seconds: number
}

export type DashboardTool = {
  key: string
  name: string
  details: string
  category: string
  sub_category: string
  icon: string
  color: string
  binary: string
  installed: boolean
  present_path?: string
  software_id?: string
  version?: string
}

export type DashboardMetricsResponse = {
  data: DashboardMetrics
  tools?: DashboardTool[]
}

export type DashboardToolsResponse = {
  data: DashboardTool[]
}

export type DashboardProcess = {
  pid: number
  ppid: number
  name: string
  user: string
  cmdline: string
  state: string
  threads: number
  cpu_percent: number
  memory_bytes: number
  memory_human: string
  memory_percent: number
}

export type DashboardProcessesResponse = {
  data: DashboardProcess[]
}

export async function getDashboardMetrics() {
  return ApiService.fetchData<DashboardMetricsResponse>({
    url: DASHBOARD_METRICS_BASE,
    method: "get",
  })
}

export async function getDashboardTools() {
  return ApiService.fetchData<DashboardToolsResponse>({
    url: DASHBOARD_TOOLS_BASE,
    method: "get",
  })
}

export async function getDashboardProcesses(limit = 50) {
  return ApiService.fetchData<DashboardProcessesResponse>({
    url: DASHBOARD_PROCESSES_BASE,
    method: "get",
    params: { limit },
  })
}

export async function killDashboardProcess(pid: number, force = false) {
  return ApiService.fetchData<{ data: { pid: number; killed: boolean; force: boolean } }>({
    url: `${DASHBOARD_PROCESSES_BASE}/${pid}/kill`,
    method: "post",
    params: force ? { force: "1" } : undefined,
  })
}

