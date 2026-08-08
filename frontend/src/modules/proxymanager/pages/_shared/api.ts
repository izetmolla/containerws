import ApiService from "@/lib/network"

export const PROXY_SETTINGS_KEY = "proxymanager-settings"
export const PROXY_RUNTIME_KEY = "proxymanager-runtime"
export const PROXY_STATUS_KEY = "proxymanager-status"
export const PROXY_HOSTS_KEY = "proxymanager-hosts"
export const PROXY_CERTS_KEY = "proxymanager-certificates"
export const PROXY_REDIRECTS_KEY = "proxymanager-redirects"
export const PROXY_RUNS_KEY = "proxymanager-runs"
export const PROXY_LOGS_KEY = "proxymanager-logs"

export const PROXY_SETTINGS_BASE = "/proxymanager/settings"
export const PROXY_HOSTS_LIST = "/proxymanager/hosts/list"
export const PROXY_HOSTS_SINGLE = "/proxymanager/hosts/single"
export const PROXY_CERTS_LIST = "/proxymanager/certificates/list"
export const PROXY_CERTS_SINGLE = "/proxymanager/certificates/single"
export const PROXY_REDIRECTS_LIST = "/proxymanager/redirects/list"
export const PROXY_REDIRECTS_SINGLE = "/proxymanager/redirects/single"
export const PROXY_APPLY_BASE = "/proxymanager/apply"
export const PROXY_LOGS_BASE = "/proxymanager/logs"

export type ProxyEngine = "fiber" | "nginx" | "traefik"
export type ProxyRuntime = "host" | "docker"
export type ProxyDockerNetworkMode = "published" | "host" | "macvlan"

export type ProxySettings = {
  id: string
  active_engine: ProxyEngine
  nginx_runtime: ProxyRuntime
  traefik_runtime: ProxyRuntime
  http_port: number
  https_port: number
  docker_environment_id?: string
  nginx_image?: string
  traefik_image?: string
  nginx_container_name?: string
  traefik_container_name?: string
  docker_network_mode?: ProxyDockerNetworkMode
  docker_publish_ip?: string
  docker_network_name?: string
  docker_ipv4_address?: string
  nginx_binary_path?: string
  nginx_config_path?: string
  nginx_systemd_unit?: string
  traefik_binary_path?: string
  traefik_config_path?: string
  traefik_systemd_unit?: string
  config_dir: string
  dirty: boolean
  last_applied_at?: string | null
  last_apply_error?: string
  last_apply_engine?: string
}

export type ProxyDockerNetwork = {
  id: string
  name: string
  driver: string
  scope?: string
}

export type ProxyRuntimeStatus = {
  docker_available: boolean
  docker_error?: string
  docker_networks?: ProxyDockerNetwork[]
  host_ips?: string[]
  nginx_binary?: string
  nginx_installed: boolean
  traefik_binary?: string
  traefik_installed: boolean
  systemd_available: boolean
  active_engine: ProxyEngine
  nginx_runtime: ProxyRuntime
  traefik_runtime: ProxyRuntime
  config_dir: string
  dirty: boolean
  last_applied_at?: string
  last_apply_error?: string
  last_apply_engine?: string
}

export type ProxyLocation = {
  id?: string
  path_prefix: string
  upstream_type: "url" | "app_path"
  upstream_target: string
  strip_prefix?: boolean
  websocket?: boolean
  order_nr?: number
  enabled?: boolean
}

export type ProxyHost = {
  id: string
  name: string
  domains: string
  enabled: boolean
  listen_scheme: "http" | "https" | "both"
  forward_scheme: "http" | "https"
  forward_host: string
  forward_port: number
  upstream_type: "url" | "app_path"
  upstream_target: string
  websocket: boolean
  ssl_forced: boolean
  block_exploits: boolean
  caching_enabled: boolean
  http2_support: boolean
  custom_headers?: Record<string, string>
  notes?: string
  certificate_id?: string | null
  order_nr?: number
  locations?: ProxyLocation[]
}

export type ProxyCertificate = {
  id: string
  name: string
  domains?: string
  source: "upload" | "path" | "letsencrypt"
  cert_path?: string
  key_path?: string
  cert_pem?: string
  key_pem?: string
  has_cert_pem?: boolean
  has_key_pem?: boolean
  letsencrypt_email?: string
  letsencrypt_status?: string
  notes?: string
}

export type ProxyRedirect = {
  id: string
  name: string
  enabled: boolean
  from_host: string
  from_path: string
  to_url: string
  status_code: number
  preserve_path: boolean
  order_nr?: number
  notes?: string
}

export type ProxyApplyRun = {
  id: string
  engine: string
  status: string
  started_at: string
  finished_at?: string | null
  log_text?: string
  error_text?: string
  files_json?: unknown[]
}

export type ProxyLogs = {
  engine: string
  runtime: string
  source: string
  lines: string[]
  text: string
  error?: string
}

export type ApplyResult = {
  run?: ProxyApplyRun
  files?: string[]
  engine?: string
  preview?: boolean
  log?: string
}

export type UpsertHost = {
  name: string
  domains: string
  enabled?: boolean
  listen_scheme?: string
  forward_scheme?: string
  forward_host?: string
  forward_port?: number
  upstream_type?: string
  upstream_target?: string
  websocket?: boolean
  ssl_forced?: boolean
  block_exploits?: boolean
  caching_enabled?: boolean
  http2_support?: boolean
  notes?: string
  certificate_id?: string | null
  order_nr?: number
  locations?: ProxyLocation[]
}

export type UpsertCert = {
  name: string
  domains?: string
  source?: string
  cert_pem?: string
  key_pem?: string
  cert_path?: string
  key_path?: string
  letsencrypt_email?: string
  notes?: string
}

export type UpsertRedirect = {
  name: string
  enabled?: boolean
  from_host: string
  from_path?: string
  to_url: string
  status_code?: number
  preserve_path?: boolean
  order_nr?: number
  notes?: string
}

export type UpdateSettings = Partial<ProxySettings>

export async function getProxySettings() {
  return ApiService.fetchData<{ data: ProxySettings }>({
    url: PROXY_SETTINGS_BASE,
    method: "get",
  })
}

export async function updateProxySettings(body: UpdateSettings) {
  return ApiService.fetchData<{ data: ProxySettings; message?: string }>({
    url: PROXY_SETTINGS_BASE,
    method: "put",
    data: body,
  })
}

export async function getProxyRuntime() {
  return ApiService.fetchData<{ data: ProxyRuntimeStatus }>({
    url: `${PROXY_SETTINGS_BASE}/runtime`,
    method: "get",
  })
}

export async function getProxyStatus() {
  return ApiService.fetchData<{
    data: {
      settings: ProxySettings
      runtime: ProxyRuntimeStatus
      last_run?: ProxyApplyRun
      fiber_table?: unknown
      module_enabled?: boolean
      components?: {
        engine?: string
        runtime?: string
        ready?: boolean
        missing?: string[]
        details?: string[]
        docker_ready?: boolean
      }
    }
  }>({
    url: `${PROXY_APPLY_BASE}/status`,
    method: "get",
  })
}

export async function applyProxy() {
  return ApiService.fetchData<{
    data?: ApplyResult
    message?: string
    error?: boolean
    run?: ProxyApplyRun
    files?: string[]
    log?: string
  }>({
    url: PROXY_APPLY_BASE,
    method: "post",
  })
}

export async function previewProxy() {
  return ApiService.fetchData<{
    data: { result?: unknown; contents?: Record<string, string>; error?: string }
  }>({
    url: `${PROXY_APPLY_BASE}/preview`,
    method: "get",
  })
}

export async function listProxyRuns() {
  return ApiService.fetchData<{ data: ProxyApplyRun[] }>({
    url: `${PROXY_APPLY_BASE}/runs`,
    method: "get",
  })
}

export async function getProxyLogs(tail = 200) {
  return ApiService.fetchData<{ data: ProxyLogs }>({
    url: PROXY_LOGS_BASE,
    method: "get",
    params: { tail: String(tail) },
  })
}

export async function listProxyHosts() {
  return ApiService.fetchData<{ data: ProxyHost[] }>({
    url: PROXY_HOSTS_LIST,
    method: "get",
  })
}

export async function createProxyHost(body: UpsertHost) {
  return ApiService.fetchData<{ data: ProxyHost; message?: string }>({
    url: PROXY_HOSTS_SINGLE,
    method: "post",
    data: body,
  })
}

export async function updateProxyHost(id: string, body: UpsertHost) {
  return ApiService.fetchData<{ data: ProxyHost; message?: string }>({
    url: `${PROXY_HOSTS_SINGLE}/${id}`,
    method: "put",
    data: body,
  })
}

export async function deleteProxyHost(id: string) {
  return ApiService.fetchData<{ message?: string }>({
    url: `${PROXY_HOSTS_SINGLE}/${id}`,
    method: "delete",
  })
}

export async function listProxyCertificates() {
  return ApiService.fetchData<{ data: ProxyCertificate[] }>({
    url: PROXY_CERTS_LIST,
    method: "get",
  })
}

export async function createProxyCertificate(body: UpsertCert) {
  return ApiService.fetchData<{ data: ProxyCertificate; message?: string }>({
    url: PROXY_CERTS_SINGLE,
    method: "post",
    data: body,
  })
}

export async function updateProxyCertificate(id: string, body: UpsertCert) {
  return ApiService.fetchData<{ data: ProxyCertificate; message?: string }>({
    url: `${PROXY_CERTS_SINGLE}/${id}`,
    method: "put",
    data: body,
  })
}

export async function deleteProxyCertificate(id: string) {
  return ApiService.fetchData<{ message?: string }>({
    url: `${PROXY_CERTS_SINGLE}/${id}`,
    method: "delete",
  })
}

export async function listProxyRedirects() {
  return ApiService.fetchData<{ data: ProxyRedirect[] }>({
    url: PROXY_REDIRECTS_LIST,
    method: "get",
  })
}

export async function createProxyRedirect(body: UpsertRedirect) {
  return ApiService.fetchData<{ data: ProxyRedirect; message?: string }>({
    url: PROXY_REDIRECTS_SINGLE,
    method: "post",
    data: body,
  })
}

export async function updateProxyRedirect(id: string, body: UpsertRedirect) {
  return ApiService.fetchData<{ data: ProxyRedirect; message?: string }>({
    url: `${PROXY_REDIRECTS_SINGLE}/${id}`,
    method: "put",
    data: body,
  })
}

export async function deleteProxyRedirect(id: string) {
  return ApiService.fetchData<{ message?: string }>({
    url: `${PROXY_REDIRECTS_SINGLE}/${id}`,
    method: "delete",
  })
}

/** Invalidate all proxy manager query caches after mutations / apply. */
export const PROXY_QUERY_KEYS = [
  PROXY_SETTINGS_KEY,
  PROXY_RUNTIME_KEY,
  PROXY_STATUS_KEY,
  PROXY_HOSTS_KEY,
  PROXY_CERTS_KEY,
  PROXY_REDIRECTS_KEY,
  PROXY_RUNS_KEY,
  PROXY_LOGS_KEY,
] as const
