import ApiService from "@/lib/network"

import { z } from "zod"
import type { Tokens, User } from "@/types"
import i18n from "@/components/providers/i18n/lib"

export type RegisterResponse = {
  message?: string
  user: User
  tokens: Tokens
}

//do not add /api prefix here
const AUTH_BASE = "/authorization"
export async function register(
  data: RegisterPayload
): Promise<RegisterResponse> {
  return ApiService.fetchData<RegisterResponse>({
    url: `${AUTH_BASE}/register`,
    method: "post",
    data,
  })
}

export const registerSchema = z
  .object({
    first_name: z.string().min(1, {
      message: i18n.t("Please enter your first name", { ns: "authorization" }),
    }),
    last_name: z.string().min(1, {
      message: i18n.t("Please enter your last name", { ns: "authorization" }),
    }),
    email: z.string().min(1, {
      message: i18n.t("Please enter your email or username", {
        ns: "authorization",
      }),
    }),
    password: z.string().min(1, {
      message: i18n.t("Please enter your password", { ns: "authorization" }),
    }),
    confirm_password: z.string().min(1, {
      message: i18n.t("Please enter your confirm password", {
        ns: "authorization",
      }),
    }),
  })
  .refine((data) => data.password === data.confirm_password, {
    message: i18n.t("Passwords do not match", { ns: "authorization" }),
    path: ["confirm_password"],
  })

export type RegisterSchema = z.infer<typeof registerSchema>
export type RegisterPayload = Omit<RegisterSchema, "confirm_password">
