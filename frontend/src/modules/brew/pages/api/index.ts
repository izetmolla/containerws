import ApiService from "@/lib/network"

export const BREW_BASE = "/brew"
export const BREW_STATUS_KEY = "brew-status"
export const BREW_FORMULAE_KEY = "brew-formulae"
export const BREW_INSTALLED_KEY = "brew-installed"
export const BREW_ANALYTICS_KEY = "brew-analytics"
export const BREW_FORMULA_KEY = "brew-formula"

export type BrewBootstrap = {
  running?: boolean
  finished?: boolean
  success?: boolean
  error?: string
  log?: string
  started_at?: string
}

export type BrewStatus = {
  module_enabled: boolean
  binary_present: boolean
  brew_path?: string
  prefix?: string
  bootstrap?: BrewBootstrap
}

export type BrewPackageKind = "formula" | "cask"

export type BrewFormulaVersion = {
  formula: string
  version?: string
  kind: "stable" | "head" | "versioned" | "installed" | string
  current?: boolean
  installed?: boolean
  href?: string
}

export type BrewFormula = {
  kind?: BrewPackageKind | string
  name: string
  display_name?: string
  full_name?: string
  tap?: string
  desc?: string
  homepage?: string
  license?: string
  version?: string
  revision?: number
  head?: string
  stable_url?: string
  category?: string
  aliases?: string[]
  oldnames?: string[]
  versioned_formulae?: string[]
  dependencies?: string[]
  build_dependencies?: string[]
  executables?: string[]
  versions?: BrewFormulaVersion[]
  icon?: string
  logo?: string
  installed?: boolean
  installed_version?: string
  installed_versions?: string[]
  outdated?: boolean
  deprecated?: boolean
  disabled?: boolean
  analytics?: {
    install_30d?: number
    install_90d?: number
    install_365d?: number
  }
  ownership?: string
  software_id?: string
  software_name?: string
  package_manager?: string
  can_switch_to_local?: boolean
  can_switch_to_brew?: boolean
}

export type BrewCategoryFacet = { name: string; count: number }

export type BrewFormulaeResponse = {
  data: {
    items: BrewFormula[]
    total: number
    page: number
    limit: number
    total_pages: number
    categories: BrewCategoryFacet[]
  }
  message?: string
  error?: boolean
}

export type BrewJob = {
  id: string
  action: string
  names: string[]
  status: string
  log?: string
  error?: string
  started_at?: string
  ended_at?: string
}

export async function getBrewStatus() {
  return ApiService.fetchData<{ data: BrewStatus }>({
    url: `${BREW_BASE}/status`,
    method: "get",
  })
}

export async function startBrewBootstrap() {
  return ApiService.fetchData<{
    data: { started: boolean; bootstrap: BrewBootstrap }
    message?: string
  }>({
    url: `${BREW_BASE}/bootstrap`,
    method: "post",
  })
}

export async function getBrewFormulae(params: {
  q?: string
  category?: string
  kind?: "all" | BrewPackageKind | string
  page?: number
  limit?: number
}) {
  return ApiService.fetchData<BrewFormulaeResponse>({
    url: `${BREW_BASE}/formulae`,
    method: "get",
    params,
  })
}

export async function getBrewFormula(
  name: string,
  kind?: BrewPackageKind | string
) {
  return ApiService.fetchData<{ data: BrewFormula }>({
    url: `${BREW_BASE}/formulae/${encodeURIComponent(name)}`,
    method: "get",
    params: kind ? { kind } : undefined,
  })
}

export async function getBrewInstalled() {
  return ApiService.fetchData<{
    data: { items: BrewFormula[]; brew_missing?: boolean }
  }>({
    url: `${BREW_BASE}/installed`,
    method: "get",
  })
}

export async function getBrewAnalytics(days: 30 | 90 | 365 = 30) {
  return ApiService.fetchData<{
    data: { days: number; items: Array<BrewFormula & { rank: number }> }
  }>({
    url: `${BREW_BASE}/analytics/install/${days}`,
    method: "get",
  })
}

export async function runBrewAction(
  action: "install" | "upgrade" | "uninstall",
  names: string[],
  kind?: BrewPackageKind | string
) {
  return ApiService.fetchData<{ data: BrewJob; message?: string }>({
    url: `${BREW_BASE}/actions`,
    method: "post",
    data: { action, names, kind },
  })
}

export async function getBrewJob(id: string) {
  return ApiService.fetchData<{ data: BrewJob }>({
    url: `${BREW_BASE}/jobs/${encodeURIComponent(id)}`,
    method: "get",
  })
}

export async function switchPackageManager(
  softwareId: string,
  target: "local" | "brew"
) {
  return ApiService.fetchData<{
    data: {
      software_id: string
      token: string
      target: string
      package_manager: string
      message: string
    }
    message?: string
  }>({
    url: `${BREW_BASE}/switch`,
    method: "post",
    data: { software_id: softwareId, target },
  })
}
