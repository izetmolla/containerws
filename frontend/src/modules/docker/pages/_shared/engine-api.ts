import ApiService from "@/lib/network"

import { envQueryParams } from "../environments/api"

export const DOCKER_ENGINE_KEY = "docker-engine-status"
export const DOCKER_ENGINE_BASE = "/docker/engine"

export type EngineServiceStatus = {
  managed?: boolean
  overall?: string
  units?: Array<{
    unit?: string
    active?: string
    sub?: string
    description?: string
    error?: string
  }>
}

export type EngineSoftwareStatus = {
  binary_present?: boolean
  running?: boolean
  installed?: boolean
  can_control?: boolean
  software_id?: string
  software_name?: string
  service?: EngineServiceStatus
}

export type EngineStatus = {
  reachable: boolean
  sock?: string
  error?: string
  env_disabled?: boolean
  api_version?: string
  server_version?: string
  name?: string
  containers?: number
  containers_running?: number
  images?: number
  driver?: string
  architecture?: string
  ncpu?: number
  mem_total?: number
  environment?: {
    id: string
    name: string
    conn_type: string
    host_url: string
    is_default: boolean
    is_disabled?: boolean
  }
  engine?: EngineSoftwareStatus
}

export type EngineControlAction = "start" | "stop" | "restart"

export async function getEngineStatus() {
  return ApiService.fetchData<{ data: EngineStatus }>({
    url: `${DOCKER_ENGINE_BASE}/status`,
    method: "get",
    params: envQueryParams(),
  })
}

export async function controlDockerEngine(action: EngineControlAction) {
  return ApiService.fetchData<{
    data: {
      software_id?: string
      name?: string
      action?: string
      status?: EngineServiceStatus
      engine?: EngineSoftwareStatus
    }
    message?: string
  }>({
    url: `${DOCKER_ENGINE_BASE}/${action}`,
    method: "post",
  })
}
