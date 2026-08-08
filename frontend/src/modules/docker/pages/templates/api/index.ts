import ApiService from "@/lib/network"

import { envQueryParams } from "../../environments/api"

export const DOCKER_TEMPLATES_KEY = "docker-templates"
export const DOCKER_TEMPLATES_LIST = "/docker/templates/list"
export const DOCKER_TEMPLATES_SINGLE = "/docker/templates/single"

export type TemplateEnv = {
  name: string
  label?: string
  description?: string
  default?: string
  preset?: boolean
  select?: { text: string; value: string; default?: boolean }[]
}

export type TemplateRow = {
  id: number
  type: number
  type_label: string
  title: string
  description: string
  name?: string
  logo?: string
  note?: string
  platform?: string
  categories?: string[]
  image?: string
  ports?: string[]
  env?: TemplateEnv[]
  repository?: { url: string; stackfile: string }
  interactive?: boolean
  privileged?: boolean
  restart_policy?: string
}

export type TemplatesMeta = {
  version?: string
  source?: string
  total?: number
  returned?: number
  categories?: { name: string; count: number }[]
  cached_at?: string
}

export type DeployTemplateInput = {
  template_id: number
  name?: string
  env?: Record<string, string>
}

export async function listTemplates(params?: {
  q?: string
  category?: string
  type?: string
  refresh?: boolean
}) {
  return ApiService.fetchData<{ data: TemplateRow[]; meta?: TemplatesMeta }>({
    url: DOCKER_TEMPLATES_LIST,
    method: "get",
    params: {
      ...envQueryParams(),
      q: params?.q || undefined,
      category: params?.category || undefined,
      type: params?.type || undefined,
      refresh: params?.refresh ? "1" : undefined,
    },
  })
}

export async function getTemplate(id: number | string) {
  return ApiService.fetchData<{ data: TemplateRow }>({
    url: `${DOCKER_TEMPLATES_SINGLE}/${encodeURIComponent(String(id))}`,
    method: "get",
    params: envQueryParams(),
  })
}

export async function deployTemplate(body: DeployTemplateInput) {
  return ApiService.fetchData<{
    data: {
      kind: string
      stack_id?: string
      container_id?: string
      name?: string
      template_id?: number
    }
    message?: string
  }>({
    url: `${DOCKER_TEMPLATES_SINGLE}/deploy`,
    method: "post",
    data: body,
    params: envQueryParams(),
  })
}
