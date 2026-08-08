/**
 * Helpers for inspecting and formatting backend error responses.
 *
 * The backend has two distinct error shapes the UI has to deal with:
 *
 *   1. HTTP 4xx/5xx with a JSON body like
 *      `{ error: true, message?: string, code?: string }`.
 *   2. HTTP 200 with an "in-band" error object in the same shape -
 *      this happens for legacy endpoints that always reply 200.
 *
 * These helpers normalize both into a single string suitable for a
 * toast.
 */

import axios from "axios"
import { toast } from "sonner"

/**
 * Wraps an upstream error and a response body into a single `Error`
 * the UI can throw. Returns `null` when there is no error to surface.
 */
export const withError = (error: Error | null, data: unknown): Error | null => {
  if (error) return error
  if ((data as { error?: unknown })?.error) {
    return new Error((data as { message?: string }).message)
  }
  return null
}

/** Backend may respond with HTTP 200 and `{ error: true, message?: string }`. */
export function isApiErrorBody(body: unknown): boolean {
  if (!body || typeof body !== "object") return false
  const err = (body as Record<string, unknown>).error
  return err === true || err === "true" || err === 1
}

/**
 * Extracts a toast-friendly message from a `{ error, message?, code? }`
 * envelope. Falls back to the supplied default when no usable string
 * is present.
 */
export function getApiErrorMessageFromBody(
  body: unknown,
  fallback: string
): string {
  if (!body || typeof body !== "object") return fallback
  const o = body as Record<string, unknown>
  const msg = o.message
  if (typeof msg === "string") {
    const t = msg.trim()
    if (t.length > 0) return t
  } else if (
    msg != null &&
    typeof msg !== "object" &&
    typeof msg !== "undefined"
  ) {
    const t = String(msg).trim()
    if (t.length > 0) return t
  }
  const code = o.code
  if (typeof code === "string") {
    const t = code.trim()
    if (t.length > 0) return t
  }
  return fallback
}

/**
 * Message for failed requests (4xx/5xx, network errors) - safe for
 * toast copy. Strips HTML, truncates to 280 chars, and falls back
 * to the supplied default for empty/unknown error shapes.
 */
export function getRequestErrorMessage(
  error: unknown,
  fallback: string
): string {
  if (axios.isAxiosError(error)) {
    const status = error.response?.status
    const data = error.response?.data

    if (data != null && typeof data === "object" && !Array.isArray(data)) {
      const msg = getApiErrorMessageFromBody(data, "")
      if (msg) return msg
    }
    if (typeof data === "string" && data.trim().length > 0) {
      const text = data
        .replace(/<[^>]+>/g, " ")
        .replace(/\s+/g, " ")
        .trim()
      if (text.length > 0)
        return text.length > 4000 ? `${text.slice(0, 4000)}…` : text
    }
    if (status === 404) {
      return error.response?.statusText?.trim() || "Not found"
    }
    if (typeof error.message === "string" && error.message.trim().length > 0) {
      return error.message.trim()
    }
  }
  if (error instanceof Error && error.message.trim().length > 0) {
    return error.message.trim()
  }

  return fallback
}

export type RequestErrorInfo = {
  title: string
  description: string
  code?: string
  status?: number
}

/**
 * Splits a failed request into a short toast title and a details line
 * (backend message, error code, or HTTP status).
 */
export function getRequestErrorInfo(
  error: unknown,
  title: string
): RequestErrorInfo {
  let code: string | undefined
  let status: number | undefined
  let details: string | undefined

  if (axios.isAxiosError(error)) {
    status = error.response?.status
    const data = error.response?.data
    if (data != null && typeof data === "object" && !Array.isArray(data)) {
      const body = data as Record<string, unknown>
      const c = body.code
      if (typeof c === "string" && c.trim()) code = c.trim()
      const nested = body.data
      if (nested && typeof nested === "object" && !Array.isArray(nested)) {
        const n = nested as Record<string, unknown>
        const chunks: string[] = []
        if (typeof n.stderr === "string" && n.stderr.trim()) {
          chunks.push(n.stderr.trim())
        }
        if (typeof n.stdout === "string" && n.stdout.trim()) {
          chunks.push(n.stdout.trim())
        }
        if (typeof n.command === "string" && n.command.trim()) {
          chunks.push(`Command: ${n.command.trim()}`)
        }
        if (chunks.length) details = chunks.join("\n\n")
      }
    }
  }

  const message = getRequestErrorMessage(error, "")
  const parts: string[] = []
  if (message && message !== title) {
    parts.push(message.length > 4000 ? `${message.slice(0, 4000)}…` : message)
  } else if (status) {
    parts.push(`Request failed (HTTP ${status})`)
  } else {
    parts.push("Something went wrong")
  }
  if (details && !parts[0]?.includes(details.slice(0, 80))) {
    parts.push(details.length > 4000 ? `${details.slice(0, 4000)}…` : details)
  }
  if (code && !parts.some((p) => p.includes(code))) {
    parts.push(code)
  }

  return {
    title,
    description: parts.join("\n\n"),
    code,
    status,
  }
}

/** Bottom-right error toast: title = action, description = full API details. */
export function toastRequestError(error: unknown, title: string): void {
  const info = getRequestErrorInfo(error, title)
  toast.error(info.title, {
    description: info.description,
    duration: Math.min(20_000, 6_000 + info.description.length * 8),
  })
}

export type FieldError = { field: string; message: string }

function isFieldErrorItem(value: unknown): value is FieldError {
  if (!value || typeof value !== "object") return false
  const item = value as Record<string, unknown>
  return typeof item.field === "string" && typeof item.message === "string"
}

export function getFieldsErrorsFromData(error: unknown): FieldError[] {
  if (!axios.isAxiosError(error)) return []

  const payload = error.response?.data
  if (!payload || typeof payload !== "object" || Array.isArray(payload))
    return []

  const fields = (payload as Record<string, unknown>).data
  if (!Array.isArray(fields)) return []

  return fields.filter(isFieldErrorItem)
}
