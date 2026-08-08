import ApiService, {
  type ResponseWithPagination,
} from "@/lib/network"

export const REMOTEPKG_FETCH_KEY = "softwares-remotepkg"
export const REMOTEPKG_LIST_BASE = "/softwares/remotepkg/list"
export const REMOTEPKG_PACKAGES_BASE = "/softwares/remotepkg/packages"
export const REMOTEPKG_BASE = "/softwares/remotepkg"

export type SoftwarePackageRegistry = {
  id: string
  package_url: string
  username?: string
  has_token?: boolean
  has_password?: boolean
  remote_count?: number
  catalog_error?: string
  is_default?: boolean
  created_at?: string
  updated_at?: string
}

export type RemotePackageItem = {
  name: string
  details?: string
  category?: string
  sub_category?: string
  tags?: string[]
  icon?: string
  image?: string
  color?: string
  order?: number
  service_units?: string[]
  package_id: string
  package_url: string
}

export type RegistryPayload = {
  package_url: string
  token?: string
  username?: string
  password?: string
  clear_token?: boolean
  clear_password?: boolean
}

export async function getRegistriesList(q = "") {
  return ApiService.fetchData<ResponseWithPagination<SoftwarePackageRegistry>>({
    url: REMOTEPKG_LIST_BASE,
    method: "get",
    params: q.trim() ? { q: q.trim() } : undefined,
  })
}

export async function getRemotePackages(opts?: {
  q?: string
  packageId?: string
}) {
  return ApiService.fetchData<ResponseWithPagination<RemotePackageItem>>({
    url: REMOTEPKG_PACKAGES_BASE,
    method: "get",
    params: {
      ...(opts?.q?.trim() ? { q: opts.q.trim() } : {}),
      ...(opts?.packageId ? { package_id: opts.packageId } : {}),
    },
  })
}

export async function createRegistry(data: RegistryPayload) {
  return ApiService.fetchData<{
    data: SoftwarePackageRegistry
    message?: string
  }>({
    url: REMOTEPKG_BASE,
    method: "post",
    data,
  })
}

export async function updateRegistry(id: string, data: RegistryPayload) {
  return ApiService.fetchData<{
    data: SoftwarePackageRegistry
    message?: string
  }>({
    url: `${REMOTEPKG_BASE}/${id}`,
    method: "put",
    data,
  })
}

export async function deleteRegistry(id: string) {
  return ApiService.fetchData<{ message?: string }>({
    url: `${REMOTEPKG_BASE}/${id}`,
    method: "delete",
  })
}
