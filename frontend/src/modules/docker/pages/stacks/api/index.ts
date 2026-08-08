import ApiService from "@/lib/network"

import { envQueryParams } from "../../environments/api"

export const DOCKER_STACKS_KEY = "docker-stacks"
export const DOCKER_STACKS_LIST = "/docker/stacks/list"
export const DOCKER_STACKS_SINGLE = "/docker/stacks/single"

export type StackRow = {
  id: string
  name: string
  environment_id: string
  status: string
  message?: string
  template_id?: number | null
  template_title?: string
  container_count: number
  running_count: number
  created_at: string
  updated_at: string
  compose_preview?: string
}

export type StackContainerPort = {
  ip?: string
  private_port: number
  public_port?: number
  type: string
}

export type StackContainer = {
  id: string
  short_id?: string
  name: string
  state: string
  status: string
  image: string
  service?: string
  created?: number
  ip_address?: string
  ports?: StackContainerPort[]
}

export type StackDetail = StackRow & {
  compose_yaml: string
  env_file?: string
  containers: StackContainer[]
}

export type UpsertStackInput = {
  name: string
  compose_yaml: string
  env_file?: string
  deploy?: boolean
  pull?: boolean
  prune?: boolean
  template_id?: number
  template_title?: string
}

export async function listStacks() {
  return ApiService.fetchData<{ data: StackRow[] }>({
    url: DOCKER_STACKS_LIST,
    method: "get",
    params: envQueryParams(),
  })
}

export async function getStack(id: string) {
  return ApiService.fetchData<{ data: StackDetail }>({
    url: `${DOCKER_STACKS_SINGLE}/${encodeURIComponent(id)}`,
    method: "get",
    params: envQueryParams(),
  })
}

export async function createStack(
  body: UpsertStackInput,
  environmentId?: string | null
) {
  return ApiService.fetchData<{ data: StackDetail; message?: string }>({
    url: DOCKER_STACKS_SINGLE,
    method: "post",
    data: body,
    params: envQueryParams(environmentId),
  })
}

export async function updateStack(id: string, body: UpsertStackInput) {
  return ApiService.fetchData<{ data: StackDetail; message?: string }>({
    url: `${DOCKER_STACKS_SINGLE}/${encodeURIComponent(id)}`,
    method: "put",
    data: body,
    params: envQueryParams(),
  })
}

export async function deployStack(id: string) {
  return ApiService.fetchData<{ data: StackDetail; message?: string }>({
    url: `${DOCKER_STACKS_SINGLE}/${encodeURIComponent(id)}/deploy`,
    method: "post",
    params: envQueryParams(),
  })
}

export async function stopStack(id: string) {
  return ApiService.fetchData<{ data: StackDetail; message?: string }>({
    url: `${DOCKER_STACKS_SINGLE}/${encodeURIComponent(id)}/stop`,
    method: "post",
    params: envQueryParams(),
  })
}

export async function removeStack(id: string, force = false) {
  return ApiService.fetchData<{ data: { id: string }; message?: string }>({
    url: `${DOCKER_STACKS_SINGLE}/${encodeURIComponent(id)}`,
    method: "delete",
    params: { ...envQueryParams(), ...(force ? { force: "1" } : {}) },
  })
}

export type ValidateComposeInput = {
  name?: string
  compose_yaml: string
  env_file?: string
}

export async function validateCompose(body: ValidateComposeInput) {
  return ApiService.fetchData<{
    data: { valid: boolean }
    message?: string
  }>({
    url: `${DOCKER_STACKS_SINGLE}/validate`,
    method: "post",
    data: body,
    params: envQueryParams(),
  })
}
