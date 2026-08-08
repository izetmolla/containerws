import type { AxiosRequestConfig } from "axios"

import ApiService from "@/lib/network"
import { REQUEST_HEADER_AUTH_KEY, TOKEN_TYPE } from "@/lib/network/constants"
import type { RetryableConfig } from "@/lib/network/refresh"
import type { Tokens, User } from "@/types"
import { z } from "zod"
import i18n from "@/components/providers/i18n/lib"

export interface SignInResponse {
  user: User
  tokens: Tokens
  session_id?: string
}

const BASE_URL = "/authorization" //skip /api prefix

export type BrandingInfo = {
  workspace_name?: string
  os_name?: string
  os_label?: string
  os_version?: string
  hostname?: string
}

export type BrandingResponse = {
  data: BrandingInfo
}

export async function getBranding() {
  return ApiService.fetchData<BrandingResponse>({
    url: `${BASE_URL}/branding`,
    method: "get",
  })
}

export async function signIn(data: SignInSchema) {
  return ApiService.fetchData<SignInResponse>({
    url: `${BASE_URL}/signin`,
    method: "post",
    data,
  })
}

export const signInSchema = z.object({
  email: z.string().min(1, {
    message: i18n.t("Please enter your email or username", {
      ns: "authorization",
    }),
  }),
  password: z.string().min(1, {
    message: i18n.t("Please enter your password", { ns: "authorization" }),
  }),
})

export type SignInSchema = z.infer<typeof signInSchema>

export async function check(refreshToken: string) {
  const config = {
    url: `${BASE_URL}/check`,
    method: "post",
    data: { refresh_token: refreshToken },
    headers: {
      accept: "application/json",
      [REQUEST_HEADER_AUTH_KEY]: `${TOKEN_TYPE}${refreshToken}`,
    },
    _isRefresh: true,
  } satisfies AxiosRequestConfig & Pick<RetryableConfig, "_isRefresh">

  return ApiService.fetchData<SignInResponse>(config)
}
