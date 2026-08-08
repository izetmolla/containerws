import ApiService from "@/lib/network"

export const DOCKER_ENV_KEY = "docker-environments"
export const DOCKER_ENV_BASE = "/docker/environments"
export const DOCKER_ENV_STORAGE_KEY = "cws.docker.environment_id"

export type DockerConnType = "unix" | "ssh" | "tls"

export type DockerEnvironment = {
  id: string
  name: string
  description?: string
  conn_type: DockerConnType
  host_url?: string
  socket_path?: string
  ssh_host?: string
  ssh_port?: number
  ssh_user?: string
  ssh_remote_socket?: string
  tcp_host?: string
  tcp_port?: number
  tls_skip_verify?: boolean
  is_default: boolean
  is_disabled: boolean
  has_ssh_key?: boolean
  has_tls_ca?: boolean
  has_tls_cert?: boolean
  has_tls_key?: boolean
  created_at?: string
  updated_at?: string
  // secrets only on get
  ssh_private_key?: string
  ssh_passphrase?: string
  tls_ca_cert?: string
  tls_cert?: string
  tls_key?: string
}

export type UpsertDockerEnvironment = {
  name: string
  description?: string
  conn_type: DockerConnType
  socket_path?: string
  ssh_host?: string
  ssh_port?: number
  ssh_user?: string
  ssh_private_key?: string
  ssh_passphrase?: string
  ssh_remote_socket?: string
  tcp_host?: string
  tcp_port?: number
  tls_ca_cert?: string
  tls_cert?: string
  tls_key?: string
  tls_skip_verify?: boolean
  is_default?: boolean
  is_disabled?: boolean
}

export function getStoredEnvironmentId(): string | null {
  try {
    return localStorage.getItem(DOCKER_ENV_STORAGE_KEY)
  } catch {
    return null
  }
}

export function setStoredEnvironmentId(id: string | null) {
  try {
    if (!id) localStorage.removeItem(DOCKER_ENV_STORAGE_KEY)
    else localStorage.setItem(DOCKER_ENV_STORAGE_KEY, id)
  } catch {
    // ignore
  }
}

export function envQueryParams(environmentId?: string | null) {
  const id = environmentId ?? getStoredEnvironmentId()
  return id ? { environment_id: id } : undefined
}

export async function listDockerEnvironments() {
  return ApiService.fetchData<{ data: DockerEnvironment[] }>({
    url: DOCKER_ENV_BASE,
    method: "get",
  })
}

export async function getDockerEnvironment(id: string) {
  return ApiService.fetchData<{ data: DockerEnvironment }>({
    url: `${DOCKER_ENV_BASE}/${id}`,
    method: "get",
  })
}

export async function createDockerEnvironment(body: UpsertDockerEnvironment) {
  return ApiService.fetchData<{ data: DockerEnvironment; message?: string }>({
    url: DOCKER_ENV_BASE,
    method: "post",
    data: body,
  })
}

export async function updateDockerEnvironment(
  id: string,
  body: Partial<UpsertDockerEnvironment>
) {
  return ApiService.fetchData<{ data: DockerEnvironment; message?: string }>({
    url: `${DOCKER_ENV_BASE}/${id}`,
    method: "put",
    data: body,
  })
}

export async function deleteDockerEnvironment(id: string) {
  return ApiService.fetchData<{ data: { id: string }; message?: string }>({
    url: `${DOCKER_ENV_BASE}/${id}`,
    method: "delete",
  })
}

export async function activateDockerEnvironment(id: string) {
  return ApiService.fetchData<{ data: DockerEnvironment; message?: string }>({
    url: `${DOCKER_ENV_BASE}/${id}/activate`,
    method: "post",
  })
}

export async function testDockerEnvironment(id: string) {
  return ApiService.fetchData<{
    data: { ok: boolean; error?: string; host_url?: string }
    message?: string
  }>({
    url: `${DOCKER_ENV_BASE}/${id}/test`,
    method: "post",
  })
}
