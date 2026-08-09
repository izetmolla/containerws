import axios from "axios"

import useAuthorizationStore, {
  getCurrentTokens,
} from "../../store/authorization"

import { BaseService } from "./client"
import { baseApiURL } from "./env"
import { getApiErrorMessageFromBody } from "./errors"
import "./interceptors"

function parseFilenameFromDisposition(header?: string): string | null {
  if (!header) {
    return null
  }
  const utf8Match = header.match(/filename\*=UTF-8''([^;]+)/i)
  if (utf8Match?.[1]) {
    return decodeURIComponent(utf8Match[1])
  }
  const plainMatch = header.match(/filename="?([^";]+)"?/i)
  return plainMatch?.[1] ?? null
}

async function readBlobErrorMessage(blob: Blob): Promise<string | null> {
  try {
    const text = await blob.text()
    if (!text.trim()) {
      return null
    }
    const parsed = JSON.parse(text) as unknown
    return getApiErrorMessageFromBody(parsed, "")
  } catch {
    return null
  }
}

function triggerBrowserDownload(blob: Blob, filename: string) {
  const url = URL.createObjectURL(blob)
  const link = document.createElement("a")
  link.href = url
  link.download = filename
  link.rel = "noopener"
  link.style.display = "none"
  document.body.appendChild(link)
  link.click()

  window.setTimeout(() => {
    link.remove()
    URL.revokeObjectURL(url)
  }, 1000)
}

export function buildAuthenticatedDownloadUrl(
  path: string,
  params: Record<string, string | number>
) {
  const tokens = getCurrentTokens(useAuthorizationStore.getState())
  const accessToken = tokens?.access_token ?? ""
  const search = new URLSearchParams()

  for (const [key, value] of Object.entries(params)) {
    search.set(key, String(value))
  }
  if (accessToken) {
    search.set("access_token", accessToken)
  }

  const normalizedPath = path.startsWith("/") ? path : `/${path}`
  return `${baseApiURL()}${normalizedPath}?${search.toString()}`
}

const recentDownloads = new Map<string, number>()
const downloadDedupeMs = 1500

export function openAuthenticatedDownload(
  path: string,
  params: Record<string, string | number>
) {
  const url = buildAuthenticatedDownloadUrl(path, params)
  const now = Date.now()
  const lastStarted = recentDownloads.get(url) ?? 0
  if (now - lastStarted < downloadDedupeMs) {
    return
  }
  recentDownloads.set(url, now)

  const iframe = document.createElement("iframe")
  iframe.style.display = "none"
  iframe.style.position = "fixed"
  iframe.style.width = "0"
  iframe.style.height = "0"
  iframe.style.border = "0"
  iframe.setAttribute("aria-hidden", "true")
  iframe.src = url
  document.body.appendChild(iframe)

  window.setTimeout(() => {
    iframe.remove()
  }, 120_000)
}

export async function downloadAuthenticatedBlob(
  path: string,
  params: Record<string, string | number> | undefined,
  suggestedFilename: string,
  options?: {
    method?: "get" | "post"
    data?: unknown
  }
) {
  try {
    const method = options?.method || "get"
    const response =
      method === "post"
        ? await BaseService.post(path, options?.data ?? {}, {
            params,
            responseType: "blob",
          })
        : await BaseService.get(path, {
            params,
            responseType: "blob",
          })

    const blob = response.data as Blob
    const contentType = String(
      response.headers["content-type"] ?? blob.type ?? ""
    )

    if (contentType.includes("json") || contentType.includes("text/html")) {
      const message = await readBlobErrorMessage(blob)
      throw new Error(message || "Download failed.")
    }

    const headerFilename = parseFilenameFromDisposition(
      String(response.headers["content-disposition"] ?? "")
    )
    const filename = headerFilename || suggestedFilename || "download"
    const downloadBlob =
      blob.type || !contentType
        ? blob
        : new Blob([blob], {
            type: contentType.split(";")[0] || "application/octet-stream",
          })

    triggerBrowserDownload(downloadBlob, filename)
  } catch (error) {
    if (axios.isAxiosError(error) && error.response?.data instanceof Blob) {
      const message = await readBlobErrorMessage(error.response.data)
      if (message) {
        throw new Error(message, { cause: error })
      }
    }
    throw error
  }
}
