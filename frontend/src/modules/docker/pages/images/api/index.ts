import ApiService from "@/lib/network"

import { envQueryParams } from "../../environments/api"

export const DOCKER_IMAGES_KEY = "docker-images"
export const DOCKER_IMAGES_LIST = "/docker/images/list"
export const DOCKER_IMAGES_SINGLE = "/docker/images/single"

export type ImageRow = {
  id: string
  short_id: string
  repo_tags: string[]
  repo_digests?: string[]
  created: number
  size: number
  containers: number
  in_use: boolean
  labels?: Record<string, string>
}

export async function listImages() {
  return ApiService.fetchData<{ data: ImageRow[] }>({
    url: DOCKER_IMAGES_LIST,
    method: "get",
    params: envQueryParams(),
  })
}

export async function pullImage(
  image: string,
  tag?: string,
  opts?: { force?: boolean }
) {
  return ApiService.fetchData<{
    data: unknown
    message?: string
    skipped?: boolean
  }>({
    url: DOCKER_IMAGES_SINGLE,
    method: "post",
    data: { image, tag, force: Boolean(opts?.force), re_pull: Boolean(opts?.force) },
    params: envQueryParams(),
  })
}

export async function inspectImage(id: string) {
  return ApiService.fetchData<{ data: unknown }>({
    url: `${DOCKER_IMAGES_SINGLE}/${encodeURIComponent(id)}`,
    method: "get",
    params: envQueryParams(),
  })
}

export async function removeImage(id: string, force = false) {
  const q = force ? "?force=1" : ""
  return ApiService.fetchData<{ data: unknown; message?: string }>({
    url: `${DOCKER_IMAGES_SINGLE}/${encodeURIComponent(id)}${q}`,
    method: "delete",
    params: envQueryParams(),
  })
}

export async function pruneImages() {
  return ApiService.fetchData<{ data: unknown; message?: string }>({
    url: `${DOCKER_IMAGES_SINGLE}/prune`,
    method: "post",
    params: envQueryParams(),
  })
}
