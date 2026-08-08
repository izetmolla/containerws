/**
 * Pure JWT helpers used by the auth pipeline.
 *
 * The frontend never verifies the signature - that's the backend's
 * job - so we only ever read the payload. Keeping these functions
 * pure (no I/O, no globals) makes them trivial to unit-test and
 * reuse outside the interceptor.
 */

import { jwtDecode } from "jwt-decode"

import { REFRESH_SKEW_MS } from "./constants"

interface AccessTokenPayload {
  exp?: number
  iat?: number
}

/**
 * Type guard for "this string is a syntactically valid JWT we can
 * decode". Empty strings, undefined, and arbitrary text all fall
 * through as `false` rather than throwing.
 */
export function isValidJwtFormat(token?: string): token is string {
  if (typeof token !== "string" || token.length === 0) return false
  try {
    jwtDecode<AccessTokenPayload>(token)
    return true
  } catch {
    return false
  }
}

/** Returns the JWT `exp` claim in milliseconds, or null when unparseable. */
export function getTokenExpiryMs(token?: string): number | null {
  if (!isValidJwtFormat(token)) return null
  try {
    const decoded = jwtDecode<AccessTokenPayload>(token)
    if (typeof decoded.exp !== "number") return null
    return decoded.exp * 1000
  } catch {
    return null
  }
}

/** Returns issued lifetime (`exp - iat`) in milliseconds, or null when unknown. */
export function getTokenLifetimeMs(token?: string): number | null {
  if (!isValidJwtFormat(token)) return null
  try {
    const decoded = jwtDecode<AccessTokenPayload>(token)
    if (typeof decoded.exp !== "number" || typeof decoded.iat !== "number") {
      return null
    }
    const lifetimeMs = (decoded.exp - decoded.iat) * 1000
    return lifetimeMs > 0 ? lifetimeMs : null
  } catch {
    return null
  }
}

/**
 * Resolves the refresh window, capped so tokens shorter than the skew are
 * not treated as stale for their entire lifetime.
 */
export function resolveEffectiveRefreshSkewMs(
  token: string | undefined,
  skewMs: number = REFRESH_SKEW_MS
): number {
  const lifetimeMs = getTokenLifetimeMs(token)
  if (lifetimeMs === null) return skewMs

  return Math.min(skewMs, Math.max(0, lifetimeMs - 1_000))
}

/**
 * True when the token is expired or within `REFRESH_SKEW_MS` (2s) of expiry.
 */
export function isAccessTokenStale(
  token: string | undefined,
  skewMs: number = REFRESH_SKEW_MS
): boolean {
  const exp = getTokenExpiryMs(token)
  if (exp === null) return false

  const remainingMs = exp - Date.now()
  if (remainingMs <= 0) return true

  const effectiveSkew = resolveEffectiveRefreshSkewMs(token, skewMs)
  return remainingMs <= effectiveSkew
}
