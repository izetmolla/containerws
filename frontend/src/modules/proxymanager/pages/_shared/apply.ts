import { toast } from "sonner"
import type { QueryClient } from "@tanstack/react-query"

import { getRequestErrorMessage } from "@/lib/network"

import {
  applyProxy,
  PROXY_QUERY_KEYS,
  type ApplyResult,
} from "./api"

export async function invalidateProxyQueries(queryClient: QueryClient) {
  await Promise.all(
    PROXY_QUERY_KEYS.map((key) =>
      queryClient.invalidateQueries({ queryKey: [key] }),
    ),
  )
}

/** Apply active engine config; toast success or surface API error details. */
export async function runProxyApply(
  queryClient: QueryClient,
  opts?: { quiet?: boolean },
): Promise<ApplyResult | undefined> {
  try {
    const res = await applyProxy()
    if (!opts?.quiet) {
      toast.success(res.message || "Configuration applied")
    }
    await invalidateProxyQueries(queryClient)
    return res.data
  } catch (err) {
    const ax = err as {
      response?: { data?: Record<string, unknown> }
      message?: string
    }
    const data = ax.response?.data
    const log = typeof data?.log === "string" ? data.log : ""
    const apiErr =
      typeof data?.error === "string"
        ? data.error
        : typeof data?.message === "string"
          ? data.message
          : ""
    const msg =
      apiErr || getRequestErrorMessage(err, "Apply failed")
    const detail = log ? `${msg}\n\n${log}` : msg
    toast.error(detail.slice(0, 800))
    await invalidateProxyQueries(queryClient)
    throw err
  }
}
