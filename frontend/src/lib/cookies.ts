function escapeRegExp(value: string) {
  return value.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")
}

export function getCookie(name: string): string | undefined {
  if (typeof document === "undefined") {
    return undefined
  }

  const match = document.cookie.match(
    new RegExp(`(?:^|; )${escapeRegExp(name)}=([^;]*)`)
  )
  if (!match) {
    return undefined
  }

  return decodeURIComponent(match[1])
}

export function getCookieJSON<T>(name: string): T | undefined {
  const value = getCookie(name)
  if (!value) {
    return undefined
  }

  try {
    return JSON.parse(value) as T
  } catch {
    return undefined
  }
}
