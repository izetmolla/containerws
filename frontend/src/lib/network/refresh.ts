/**
 * Single-flight access-token refresh.
 *
 * The refresh flow is the trickiest part of the auth pipeline because
 * it's racing on three axes at once:
 *   1. Many pending requests may all notice the access token is stale
 *      at the same time and try to refresh in parallel.
 *   2. The refresh request itself goes through the same axios client
 *      whose interceptors triggered it - we have to break that loop.
 *   3. The store is the source of truth for tokens, but reading from
 *      it after the refresh request returns races with persistence.
 *
 * The mitigations:
 *   - A module-level `inFlight` promise ensures all callers share one
 *     network round-trip (point 1).
 *   - The internal `_isRefresh` flag on the axios config tells the
 *     interceptors to skip the auth pipeline (point 2).
 *   - The new access token is returned directly to the caller rather
 *     than re-read from the store (point 3).
 */

import type { AxiosRequestConfig, InternalAxiosRequestConfig } from "axios"

import useAuthorizationStore from "@/store/authorization"

import { BaseService } from "./client"
import {
  REFRESH_TIMEOUT_MS,
  REQUEST_HEADER_AUTH_KEY,
  TOKEN_TYPE,
} from "./constants"
import { isDev } from "./env"
import { isValidJwtFormat } from "./jwt"

/**
 * Internal config shape used by the interceptors and the refresh
 * helper. Both `_authRetried` and `_isRefresh` are kept as private
 * properties on the axios config so a single request never gets
 * looped through the auth pipeline more than once.
 */
export interface RetryableConfig extends InternalAxiosRequestConfig {
  _authRetried?: boolean
  _isRefresh?: boolean
}

/**
 * One in-flight refresh shared by every concurrent caller. When it
 * resolves it carries the *new* access token (or null when refresh
 * failed) so callers can decide between retrying and giving up
 * without having to re-read the store.
 */
let inFlight: Promise<string | null> | null = null

/**
 * Sets the Authorization header on a config in a way that survives
 * axios's `AxiosHeaders` instance and plain-object headers alike.
 */
export function applyAuthHeader(
  config: InternalAxiosRequestConfig,
  token: string
): void {
  if (!config.headers) return
  config.headers[REQUEST_HEADER_AUTH_KEY] = `${TOKEN_TYPE}${token}`
}

/**
 * Performs a single refresh-token round-trip. Implementation notes:
 *
 *   - We go through `BaseService` (instead of a bare axios call) so
 *     features like the configured baseURL and any platform-injected
 *     adapters (in tests, the mock adapter) apply uniformly. The
 *     interceptors short-circuit on `_isRefresh = true` to avoid
 *     re-entering the auth logic for the refresh request itself.
 *   - The request is sent to the same path the original request was
 *     going to. The Go middleware short-circuits on the `cft` header
 *     and never reaches the route handler, so the path itself is
 *     irrelevant; using the original path keeps debugging easier.
 *   - A timeout is enforced so a wedged backend can't keep every
 *     queued request blocked indefinitely.
 */
async function performRefresh(
  config: InternalAxiosRequestConfig,
  refreshToken: string
): Promise<string | null> {
  try {
    const refreshConfig: AxiosRequestConfig & { _isRefresh: true } = {
      method: config.method ?? "get",
      url: config.url,
      baseURL: config.baseURL ?? BaseService.defaults.baseURL,
      timeout: REFRESH_TIMEOUT_MS,
      headers: {
        accept: "application/json",
        Authorization: `${TOKEN_TYPE}${refreshToken}`,
        cft: "t",
      },
      _isRefresh: true,
    }
    const { data } = await BaseService.request(refreshConfig)

    if (typeof data === "string" && isValidJwtFormat(data)) {
      useAuthorizationStore.getState().setAccessToken(data)
      return data
    }
    if (isDev()) {
      console.warn("Refresh response was not a JWT string:", data)
    }
    return null
  } catch (err) {
    if (isDev()) {
      console.warn("Access token refresh failed:", err)
    }
    return null
  }
}

/**
 * Returns a promise resolving to the new access token (or null on
 * failure), guaranteeing only a single underlying network call even
 * when many concurrent requests all need a refresh at once.
 */
export function refreshAccessToken(
  config: InternalAxiosRequestConfig,
  refreshToken: string
): Promise<string | null> {
  if (inFlight) {
    return inFlight
  }
  inFlight = performRefresh(config, refreshToken).finally(() => {
    inFlight = null
  })
  return inFlight
}
