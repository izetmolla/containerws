import ApiService from "@/lib/network"
import type { Software, SoftwareVersion } from "../../list/api"

export const PACKAGE_FETCH_KEY = "softwares-package"
export const PACKAGE_BASE = "/softwares/package"

export type PackageVersion = SoftwareVersion & {
  upgrade_script?: string
  custom_script?: string
  install_script?: string
  uninstall_script?: string
}

export type PackageResponse = {
  data: {
    software: Software
    versions: PackageVersion[]
  }
}

export type UpdateSoftwarePayload = {
  name?: string
  details?: string
  category?: string
  sub_category?: string
  tags?: string[]
  service_units?: string[]
  can_control?: boolean
  control_backend?: string
  start_command?: string
  restart_command?: string
  stop_command?: string
  icon?: string
  image?: string
  color?: string
  order?: number
  is_active?: boolean
}

export type VersionPayload = {
  version: string
  is_latest?: boolean
  install_script?: string
  uninstall_script?: string
  upgrade_script?: string
  custom_script?: string
  os?: string
  os_version?: string
  distro?: string
  distro_id?: string
  distro_version?: string
  arch?: string
  platform?: string
  package_family?: string
  kernel?: string
  virtualization?: string
  container_runtime?: string
  cloud_provider?: string
}

export async function getSoftwarePackage(id: string) {
  return ApiService.fetchData<PackageResponse>({
    url: `${PACKAGE_BASE}/${id}`,
    method: "get",
  })
}

export async function updateSoftwarePackage(
  id: string,
  data: UpdateSoftwarePayload
) {
  return ApiService.fetchData<{ data: Software; message?: string }>({
    url: `${PACKAGE_BASE}/${id}`,
    method: "put",
    data,
  })
}

export async function createSoftwareVersion(
  softwareId: string,
  data: VersionPayload
) {
  return ApiService.fetchData<{ data: PackageVersion; message?: string }>({
    url: `${PACKAGE_BASE}/${softwareId}/versions`,
    method: "post",
    data,
  })
}

export async function updateSoftwareVersion(
  softwareId: string,
  versionId: string,
  data: VersionPayload
) {
  return ApiService.fetchData<{ data: PackageVersion; message?: string }>({
    url: `${PACKAGE_BASE}/${softwareId}/versions/${versionId}`,
    method: "put",
    data,
  })
}

export async function deleteSoftwareVersion(
  softwareId: string,
  versionId: string
) {
  return ApiService.fetchData<{ message?: string }>({
    url: `${PACKAGE_BASE}/${softwareId}/versions/${versionId}`,
    method: "delete",
  })
}
