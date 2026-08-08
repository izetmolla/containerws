import { z } from "zod"

import ApiService from "@/lib/network"

import { USERS_SINGLE_BASE, type VncProfile } from "../../../list/api"

export const USER_VNC_FETCH_KEY = "user-vnc"

/** TigerVNC VncAuth uses at most 8 characters; enforce a usable minimum. */
export const VNC_PASSWORD_MIN = 6
export const VNC_PASSWORD_MAX = 8

export const vncPasswordRules = [
  {
    id: "length",
    label: `Between ${VNC_PASSWORD_MIN} and ${VNC_PASSWORD_MAX} characters`,
    test: (value: string) =>
      value.length >= VNC_PASSWORD_MIN && value.length <= VNC_PASSWORD_MAX,
  },
  {
    id: "printable",
    label: "Printable ASCII only (no spaces)",
    test: (value: string) => value.length === 0 || /^[\x21-\x7E]+$/.test(value),
  },
  {
    id: "letter",
    label: "At least one letter (A–Z or a–z)",
    test: (value: string) => /[A-Za-z]/.test(value),
  },
  {
    id: "digit",
    label: "At least one number (0–9)",
    test: (value: string) => /\d/.test(value),
  },
] as const

export const vncPasswordSchema = z
  .string()
  .trim()
  .min(1, "Password is required")
  .superRefine((value, ctx) => {
    for (const rule of vncPasswordRules) {
      if (!rule.test(value)) {
        ctx.addIssue({ code: "custom", message: rule.label })
      }
    }
  })

export const vncPasswordFormSchema = z
  .object({
    password: vncPasswordSchema,
    confirm_password: z.string().min(1, "Confirm your password"),
  })
  .refine((data) => data.password === data.confirm_password, {
    message: "Passwords do not match",
    path: ["confirm_password"],
  })

export type VncPasswordFormValues = z.infer<typeof vncPasswordFormSchema>

export const vncCreateFormSchema = z
  .object({
    password: vncPasswordSchema,
    confirm_password: z.string().min(1, "Confirm your password"),
    start: z.boolean(),
  })
  .refine((data) => data.password === data.confirm_password, {
    message: "Passwords do not match",
    path: ["confirm_password"],
  })

export type VncCreateFormValues = z.infer<typeof vncCreateFormSchema>

export type VncBindAddress = {
  address: string
  interface: string
  label: string
  localhost: boolean
  family: string
}

export type VncSettingsInput = {
  status?: string
  address?: string
  geometry?: string
  depth?: number
  dpi?: number
  framerate?: number
  localhost_only?: boolean
  always_shared?: boolean
  accept_set_desktop_size?: boolean
  security_types?: string
  compare_fb?: number
  improved_hextile?: boolean
  desktop_session?: string
  quality?: number
  compression?: number
  autoconnect?: boolean
  reconnect?: boolean
  reconnect_delay?: number
  resize?: string
  view_only?: boolean
  show_dot?: boolean
  view_clip?: boolean
  shared?: boolean
  bell?: string
  logging?: string
  restart?: boolean
}

type MutationResponse = {
  data?: VncProfile
  message?: string
  warning?: string
  novnc_url?: string
}

type DetailResponse = {
  data: VncProfile
}

type AddressesResponse = {
  data: VncBindAddress[]
}

export async function getVncProfile(userId: string) {
  return ApiService.fetchData<DetailResponse>({
    url: `${USERS_SINGLE_BASE}/${userId}/vnc`,
    method: "get",
  })
}

export async function listVncBindAddresses(userId: string) {
  return ApiService.fetchData<AddressesResponse>({
    url: `${USERS_SINGLE_BASE}/${userId}/vnc/addresses`,
    method: "get",
  })
}

export async function createVncProfile(
  userId: string,
  body: { vnc_password: string; start?: boolean }
) {
  return ApiService.fetchData<MutationResponse>({
    url: `${USERS_SINGLE_BASE}/${userId}/vnc`,
    method: "post",
    data: body,
  })
}

export async function updateVncProfile(userId: string, body: VncSettingsInput) {
  return ApiService.fetchData<MutationResponse>({
    url: `${USERS_SINGLE_BASE}/${userId}/vnc`,
    method: "put",
    data: body,
  })
}

export async function setVncPassword(userId: string, vnc_password: string) {
  return ApiService.fetchData<MutationResponse>({
    url: `${USERS_SINGLE_BASE}/${userId}/vnc/password`,
    method: "post",
    data: { vnc_password },
  })
}

export async function startVncProfile(userId: string) {
  return ApiService.fetchData<MutationResponse>({
    url: `${USERS_SINGLE_BASE}/${userId}/vnc/start`,
    method: "post",
  })
}

export async function stopVncProfile(userId: string) {
  return ApiService.fetchData<MutationResponse>({
    url: `${USERS_SINGLE_BASE}/${userId}/vnc/stop`,
    method: "post",
  })
}

export async function uploadVncWallpaper(userId: string, file: File) {
  return ApiService.uploadFileData<MutationResponse>(
    `${USERS_SINGLE_BASE}/${userId}/vnc/wallpaper`,
    file
  )
}

export async function resetVncWallpaper(userId: string) {
  return ApiService.fetchData<MutationResponse>({
    url: `${USERS_SINGLE_BASE}/${userId}/vnc/wallpaper`,
    method: "delete",
  })
}

export function vncWallpaperSrc(userId: string, bust?: number | string) {
  const q = bust != null ? `?t=${bust}` : `?t=${Date.now()}`
  return `/api${USERS_SINGLE_BASE}/${userId}/vnc/wallpaper${q}`
}

