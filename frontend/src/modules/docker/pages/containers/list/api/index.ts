import ApiService from "@/lib/network"
import { baseApiURL } from "@/lib/network/env"
import { getCurrentTokens } from "@/store/authorization"
import useAuthorizationStore from "@/store/authorization"

import { envQueryParams } from "../../../environments/api"

export const DOCKER_CONTAINERS_KEY = "docker-containers"
export const DOCKER_CONTAINERS_LIST = "/docker/containers/list"
export const DOCKER_CONTAINERS_SINGLE = "/docker/containers/single"

export type PortMapping = {
  ip?: string
  private_port: number
  public_port?: number
  type: string
}

export type ContainerRow = {
  id: string
  short_id: string
  names: string[]
  name: string
  image: string
  image_id?: string
  command?: string
  created: number
  state: string
  status: string
  ports: PortMapping[]
  labels?: Record<string, string>
  stack?: string
  ip_address?: string
  ip_addresses?: string[]
}

export type ContainerDetail = {
  id: string
  short_id: string
  name: string
  image: string
  state: string
  status: string
  created?: string
  ports?: PortMapping[]
  inspect?: DockerInspect
}

export type DockerInspect = {
  Id?: string
  Name?: string
  Created?: string
  Image?: string
  Config?: {
    Image?: string
    Env?: string[] | null
    Cmd?: string[] | null
    Entrypoint?: string[] | null
    WorkingDir?: string
    User?: string
    Hostname?: string
    Labels?: Record<string, string> | null
    Tty?: boolean
    OpenStdin?: boolean
    ExposedPorts?: Record<string, unknown> | null
  }
  State?: {
    Status?: string
    Running?: boolean
    Paused?: boolean
    Restarting?: boolean
    OOMKilled?: boolean
    Dead?: boolean
    Pid?: number
    ExitCode?: number
    Error?: string
    StartedAt?: string
    FinishedAt?: string
  }
  HostConfig?: {
    Binds?: string[] | null
    PortBindings?: Record<
      string,
      { HostIp?: string; HostPort?: string }[] | null
    > | null
    PublishAllPorts?: boolean
    Privileged?: boolean
    ReadonlyRootfs?: boolean
    AutoRemove?: boolean
    ExtraHosts?: string[] | null
    CapAdd?: string[] | null
    CapDrop?: string[] | null
    Dns?: string[] | null
    Devices?: {
      PathOnHost?: string
      PathInContainer?: string
      CgroupPermissions?: string
    }[] | null
    Memory?: number
    NanoCpus?: number
    RestartPolicy?: { Name?: string; MaximumRetryCount?: number }
    LogConfig?: { Type?: string; Config?: Record<string, string> | null }
    NetworkMode?: string
  }
  NetworkSettings?: {
    Networks?: Record<
      string,
      {
        NetworkID?: string
        EndpointID?: string
        Gateway?: string
        IPAddress?: string
        IPPrefixLen?: number
        IPv6Gateway?: string
        GlobalIPv6Address?: string
        MacAddress?: string
      } | null
    > | null
    IPAddress?: string
    Gateway?: string
    MacAddress?: string
  }
  Mounts?: {
    Type?: string
    Name?: string
    Source?: string
    Destination?: string
    Driver?: string
    Mode?: string
    RW?: boolean
    Propagation?: string
  }[]
}

export type CreateContainerInput = {
  image: string
  name?: string
  cmd?: string[]
  entrypoint?: string[]
  env?: string[]
  ports?: string[]
  publish_all?: boolean
  binds?: string[]
  networks?: string[]
  restart_policy?: string
  restart_retries?: number
  memory?: string
  cpus?: number
  working_dir?: string
  user?: string
  hostname?: string
  privileged?: boolean
  readonly_rootfs?: boolean
  auto_remove?: boolean
  labels?: Record<string, string>
  extra_hosts?: string[]
  devices?: string[]
  cap_add?: string[]
  cap_drop?: string[]
  dns?: string[]
  pull_if_missing?: boolean
  always_pull?: boolean
  start?: boolean
  tty?: boolean
  open_stdin?: boolean
  log_driver?: string
  log_opts?: Record<string, string>
}

export async function listContainers() {
  return ApiService.fetchData<{ data: ContainerRow[] }>({
    url: DOCKER_CONTAINERS_LIST,
    method: "get",
    params: envQueryParams(),
  })
}

export async function getContainer(id: string) {
  return ApiService.fetchData<{ data: ContainerDetail }>({
    url: `${DOCKER_CONTAINERS_SINGLE}/${id}`,
    method: "get",
    params: envQueryParams(),
  })
}

export async function createContainer(body: CreateContainerInput) {
  return ApiService.fetchData<{ data: ContainerDetail; message?: string }>({
    url: DOCKER_CONTAINERS_SINGLE,
    method: "post",
    data: body,
    params: envQueryParams(),
  })
}

export async function updateContainer(id: string, body: CreateContainerInput) {
  return ApiService.fetchData<{ data: ContainerDetail; message?: string }>({
    url: `${DOCKER_CONTAINERS_SINGLE}/${encodeURIComponent(id)}`,
    method: "put",
    data: body,
    params: envQueryParams(),
  })
}

export async function startContainer(id: string) {
  return ApiService.fetchData<{ data: ContainerDetail; message?: string }>({
    url: `${DOCKER_CONTAINERS_SINGLE}/${id}/start`,
    method: "post",
    params: envQueryParams(),
  })
}

export async function stopContainer(id: string) {
  return ApiService.fetchData<{ data: ContainerDetail; message?: string }>({
    url: `${DOCKER_CONTAINERS_SINGLE}/${id}/stop`,
    method: "post",
    params: envQueryParams(),
  })
}

export async function restartContainer(id: string) {
  return ApiService.fetchData<{ data: ContainerDetail; message?: string }>({
    url: `${DOCKER_CONTAINERS_SINGLE}/${id}/restart`,
    method: "post",
    params: envQueryParams(),
  })
}

export async function killContainer(id: string) {
  return ApiService.fetchData<{ data: ContainerDetail; message?: string }>({
    url: `${DOCKER_CONTAINERS_SINGLE}/${id}/kill`,
    method: "post",
    params: envQueryParams(),
  })
}

export async function pauseContainer(id: string) {
  return ApiService.fetchData<{ data: ContainerDetail; message?: string }>({
    url: `${DOCKER_CONTAINERS_SINGLE}/${id}/pause`,
    method: "post",
    params: envQueryParams(),
  })
}

export async function resumeContainer(id: string) {
  return ApiService.fetchData<{ data: ContainerDetail; message?: string }>({
    url: `${DOCKER_CONTAINERS_SINGLE}/${id}/resume`,
    method: "post",
    params: envQueryParams(),
  })
}

export async function removeContainer(
  id: string,
  opts?: { force?: boolean; volumes?: boolean }
) {
  const q = new URLSearchParams()
  if (opts?.force !== false) q.set("force", "1")
  if (opts?.volumes) q.set("volumes", "1")
  const qs = q.toString()
  return ApiService.fetchData<{ data: { id: string }; message?: string }>({
    url: `${DOCKER_CONTAINERS_SINGLE}/${id}${qs ? `?${qs}` : ""}`,
    method: "delete",
    params: envQueryParams(),
  })
}

export async function recreateContainer(
  id: string,
  opts?: { pull?: boolean }
) {
  return ApiService.fetchData<{ data: ContainerDetail; message?: string }>({
    url: `${DOCKER_CONTAINERS_SINGLE}/${id}/recreate`,
    method: "post",
    data: { pull: Boolean(opts?.pull) },
    params: envQueryParams(),
  })
}

export async function getContainerLogs(id: string, tail = 200) {
  return ApiService.fetchData<{ data: { logs: string } }>({
    url: `${DOCKER_CONTAINERS_SINGLE}/${id}/logs`,
    method: "get",
    params: { ...envQueryParams(), tail },
  })
}

export async function getContainerStats(id: string) {
  return ApiService.fetchData<{ data: ContainerStats }>({
    url: `${DOCKER_CONTAINERS_SINGLE}/${id}/stats`,
    method: "get",
    params: envQueryParams(),
  })
}

export type ContainerStatsNetwork = {
  name: string
  rx_bytes: number
  tx_bytes: number
}

export type ContainerStats = {
  read_at?: string
  cpu_percent?: number
  memory_usage?: number
  memory_limit?: number
  memory_percent?: number
  network_rx?: number
  network_tx?: number
  networks?: ContainerStatsNetwork[]
  blkio_read?: number
  blkio_write?: number
}

export type ContainerTop = {
  titles: string[]
  processes: string[][]
}

export async function getContainerTop(id: string) {
  return ApiService.fetchData<{ data: ContainerTop }>({
    url: `${DOCKER_CONTAINERS_SINGLE}/${id}/top`,
    method: "get",
    params: envQueryParams(),
  })
}

export async function updateRestartPolicy(
  id: string,
  body: { name: string; maximum_retry_count?: number }
) {
  return ApiService.fetchData<{ data: ContainerDetail; message?: string }>({
    url: `${DOCKER_CONTAINERS_SINGLE}/${id}/restart-policy`,
    method: "post",
    data: body,
    params: envQueryParams(),
  })
}

export async function connectContainerNetwork(id: string, networkId: string) {
  return ApiService.fetchData<{ data: ContainerDetail; message?: string }>({
    url: `${DOCKER_CONTAINERS_SINGLE}/${id}/networks/connect`,
    method: "post",
    data: { network_id: networkId },
    params: envQueryParams(),
  })
}

export async function disconnectContainerNetwork(
  id: string,
  networkId: string,
  force = true
) {
  return ApiService.fetchData<{ data: ContainerDetail; message?: string }>({
    url: `${DOCKER_CONTAINERS_SINGLE}/${id}/networks/disconnect`,
    method: "post",
    data: { network_id: networkId, force },
    params: envQueryParams(),
  })
}

export async function commitContainer(
  id: string,
  body: {
    repository: string
    tag?: string
    pause?: boolean
    message?: string
    author?: string
  }
) {
  return ApiService.fetchData<{
    data: { id: string; reference: string }
    message?: string
  }>({
    url: `${DOCKER_CONTAINERS_SINGLE}/${id}/commit`,
    method: "post",
    data: body,
    params: envQueryParams(),
  })
}

/** WebSocket URL for interactive `docker exec` into a container. */
export function buildContainerExecWebSocketURL(opts: {
  id: string
  command: string
  user?: string
}): string {
  const tokens = getCurrentTokens(useAuthorizationStore.getState())
  const apiBase = baseApiURL()
  const wsBase = apiBase.replace(/^http/i, "ws")
  const url = new URL(
    `${wsBase}${DOCKER_CONTAINERS_SINGLE}/${encodeURIComponent(opts.id)}/exec`
  )
  if (tokens?.access_token) {
    url.searchParams.set("access_token", tokens.access_token)
  }
  url.searchParams.set("command", opts.command)
  if (opts.user?.trim()) {
    url.searchParams.set("user", opts.user.trim())
  }
  const env = envQueryParams()
  if (env?.environment_id) {
    url.searchParams.set("environment_id", env.environment_id)
  }
  return url.toString()
}
