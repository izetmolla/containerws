import ApiService from "@/lib/network"

import { envQueryParams } from "../../environments/api"

export const DOCKER_VOLUMES_KEY = "docker-volumes"
export const DOCKER_VOLUMES_LIST = "/docker/volumes/list"
export const DOCKER_VOLUMES_SINGLE = "/docker/volumes/single"

export type VolumeRow = {
  name: string
  driver: string
  mountpoint: string
  created_at?: string
  scope: string
  labels?: Record<string, string>
}

export type CreateVolumeInput = {
  name?: string
  driver?: string
  labels?: Record<string, string>
}

export async function listVolumes() {
  return ApiService.fetchData<{ data: VolumeRow[] }>({
    url: DOCKER_VOLUMES_LIST,
    method: "get",
    params: envQueryParams(),
  })
}

export async function createVolume(body: CreateVolumeInput) {
  return ApiService.fetchData<{ data: unknown; message?: string }>({
    url: DOCKER_VOLUMES_SINGLE,
    method: "post",
    data: body,
    params: envQueryParams(),
  })
}

export async function inspectVolume(name: string) {
  return ApiService.fetchData<{ data: unknown }>({
    url: `${DOCKER_VOLUMES_SINGLE}/${encodeURIComponent(name)}`,
    method: "get",
    params: envQueryParams(),
  })
}

export async function removeVolume(name: string, force = false) {
  const q = force ? "?force=1" : ""
  return ApiService.fetchData<{ data: { name: string }; message?: string }>({
    url: `${DOCKER_VOLUMES_SINGLE}/${encodeURIComponent(name)}${q}`,
    method: "delete",
    params: envQueryParams(),
  })
}
