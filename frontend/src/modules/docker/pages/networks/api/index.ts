import ApiService from "@/lib/network"

import { envQueryParams } from "../../environments/api"

export const DOCKER_NETWORKS_KEY = "docker-networks"
export const DOCKER_NETWORKS_LIST = "/docker/networks/list"
export const DOCKER_NETWORKS_SINGLE = "/docker/networks/single"

export type NetworkRow = {
  id: string
  short_id: string
  name: string
  driver: string
  scope: string
  created?: string
  internal: boolean
  attachable: boolean
  ingress: boolean
  labels?: Record<string, string>
}

export type NetworkIpamConfig = {
  subnet?: string
  gateway?: string
  ip_range?: string
  excluded_ips?: string[]
}

export type NetworkContainer = {
  id: string
  name: string
  endpoint_id?: string
  mac_address?: string
  ipv4_address?: string
  ipv6_address?: string
}

export type NetworkDetail = {
  id: string
  short_id: string
  name: string
  driver: string
  scope?: string
  internal: boolean
  attachable: boolean
  ingress?: boolean
  enable_ipv6?: boolean
  options?: Record<string, string>
  labels?: Record<string, string>
  created?: string
  ipv4?: NetworkIpamConfig
  ipv6?: NetworkIpamConfig
  containers?: NetworkContainer[]
  raw?: unknown
}

export type CreateNetworkInput = {
  name: string
  driver?: string
  options?: Record<string, string>
  internal?: boolean
  attachable?: boolean
  enable_ipv6?: boolean
  labels?: Record<string, string>
  ipv4?: NetworkIpamConfig
  ipv6?: NetworkIpamConfig
  // legacy
  subnet?: string
  gateway?: string
  ip_range?: string
}

export async function listNetworks() {
  return ApiService.fetchData<{ data: NetworkRow[] }>({
    url: DOCKER_NETWORKS_LIST,
    method: "get",
    params: envQueryParams(),
  })
}

export async function createNetwork(body: CreateNetworkInput) {
  return ApiService.fetchData<{ data: NetworkDetail; message?: string }>({
    url: DOCKER_NETWORKS_SINGLE,
    method: "post",
    data: body,
    params: envQueryParams(),
  })
}

export async function getNetwork(id: string) {
  return ApiService.fetchData<{ data: NetworkDetail }>({
    url: `${DOCKER_NETWORKS_SINGLE}/${encodeURIComponent(id)}`,
    method: "get",
    params: envQueryParams(),
  })
}

/** @deprecated use getNetwork */
export async function inspectNetwork(id: string) {
  return getNetwork(id)
}

export async function disconnectNetworkContainer(
  networkId: string,
  containerId: string,
  force = true
) {
  return ApiService.fetchData<{ data: NetworkDetail; message?: string }>({
    url: `${DOCKER_NETWORKS_SINGLE}/${encodeURIComponent(networkId)}/disconnect`,
    method: "post",
    data: { container_id: containerId, force },
    params: envQueryParams(),
  })
}

export async function removeNetwork(id: string) {
  return ApiService.fetchData<{ data: { id: string }; message?: string }>({
    url: `${DOCKER_NETWORKS_SINGLE}/${encodeURIComponent(id)}`,
    method: "delete",
    params: envQueryParams(),
  })
}
