/**
 * Wires the request and response interceptors onto `BaseService`.
 *
 * Importing this module is the SIDE EFFECT that activates the auth
 * pipeline. The barrel re-export in `../network.ts` triggers it,
 * which is why every consumer of the package gets the configured
 * client transparently.
 */

import type { AxiosError } from "axios"

import { toast } from "sonner"

import useAuthorizationStore, {
  getCurrentTokens,
  isSignedIn as isUserSignedIn,
} from "../../store/authorization"

import { BaseService } from "./client"
import {
  FATAL_AUTH_CODES,
  PERMISSION_CODES,
  REFRESHABLE_AUTH_CODES,
} from "./constants"
import { isDev } from "./env"
import { isAccessTokenStale } from "./jwt"
import {
  applyAuthHeader,
  refreshAccessToken,
  type RetryableConfig,
} from "./refresh"

/**
 * Reads the optional `code` field from a JSON error envelope. The
 * backend uses this to disambiguate auth failures (`TOKEN_EXPIRED`,
 * `INSUFFICIENT_PERMISSIONS`, etc.) from generic 4xx noise.
 */
function getResponseCode(data: unknown): string | undefined {
  if (data && typeof data === "object") {
    const c = (data as Record<string, unknown>).code
    if (typeof c === "string") return c
  }
  return undefined
}

/**
 * Performs a hard sign-out on the client: clears the local store and
 * surfaces a single toast. The backend session is invalidated by the
 * dedicated `/api/authorization/sign-out` endpoint, which is fired
 * separately from the store and tolerates a missing session.
 */
function forceSignOut(message?: string): void {
  const { signOut } = useAuthorizationStore.getState()
  signOut()
  if (message) {
    toast.error(message)
  }
}

/** User is signed in but lacks rights; show the app unauthorized page, keep session. */
function showUnauthorizedPage(): void {
  useAuthorizationStore.getState().setAccessDenied(true)
}

BaseService.interceptors.request.use(
  async (config) => {
    const cfg = config as RetryableConfig

    // The refresh request brings its own Authorization header
    // (the refresh token) and must skip the rest of the auth
    // pipeline; otherwise we'd recurse into refreshAccessToken().
    if (cfg._isRefresh) {
      return config
    }

    const authState = useAuthorizationStore.getState()
    const tokens = getCurrentTokens(authState)
    const accessToken = tokens?.access_token ?? ""
    const refreshToken = tokens?.refresh_token ?? ""

    // The reauthorize header (`cra`) is set when we have neither a
    // valid access nor refresh token, so the server can hand back
    // refreshed credentials in-band for protected endpoints that
    // support it.
    if (!accessToken && !refreshToken) {
      config.headers["cra"] = "t"
      return config
    }
    config.headers["cra"] = "f"

    if (accessToken && !isAccessTokenStale(accessToken)) {
      applyAuthHeader(config, accessToken)
      return config
    }

    if (refreshToken) {
      const newAccess = await refreshAccessToken(config, refreshToken)
      if (newAccess) {
        applyAuthHeader(config, newAccess)
        return config
      }
    }

    // Last resort: send the (possibly expired) access token. The
    // response interceptor will pick up the 401 and either retry
    // once after a fresh refresh or, if that also fails, sign the
    // user out cleanly.
    if (accessToken) {
      applyAuthHeader(config, accessToken)
    }
    return config
  },
  (error) => Promise.reject(error)
)

BaseService.interceptors.response.use(
  (response) => {
    if (response && response.status === 200 && response.data?.reauthorize) {
      const { signIn } = useAuthorizationStore.getState()
      signIn({
        user: response.data.user,
        tokens: response?.data?.tokens,
      })
      const cfg = response.config as RetryableConfig
      // Guard against an infinite loop in case the server keeps
      // setting `reauthorize: true` on every response.
      if (!cfg._authRetried) {
        cfg._authRetried = true
        return BaseService(cfg)
      }
    }
    return response
  },
  async (error: AxiosError) => {
    const response = error.response
    const cfg = error.config as RetryableConfig | undefined

    // Refresh requests own their own error handling; let the
    // caller decide what to do. Forcing sign-out from inside the
    // response interceptor on a refresh failure would race with
    // the original request's retry logic.
    if (cfg?._isRefresh) {
      return Promise.reject(error)
    }

    if (!response) {
      // Network error / timeout / cancellation. NEVER sign out
      // here: the user is still authenticated, they just lost
      // connectivity. Letting the caller see the original error
      // is enough.
      return Promise.reject(error)
    }

    const status = response.status
    const code = getResponseCode(response.data)
    const authState = useAuthorizationStore.getState()
    const signedIn = isUserSignedIn(authState)

    // Permission failures are not auth failures. The user IS signed in,
    // they simply lack rights. Match by code first so a backend bug that
    // returns the wrong HTTP status (e.g. 500 with INSUFFICIENT_PERMISSIONS)
    // still shows the access-denied page.
    const isAccessDenied =
      (code !== undefined && PERMISSION_CODES.has(code)) ||
      status === 403 ||
      (signedIn && code === "UNAUTHORIZED")

    if (isAccessDenied) {
      showUnauthorizedPage()
      return Promise.reject(error)
    }

    if (status === 401) {
      const isRefreshable = !code || REFRESHABLE_AUTH_CODES.has(code)
      const isFatal = code !== undefined && FATAL_AUTH_CODES.has(code)

      // One retry only. If we already attempted to refresh and
      // the second 401 came back, we know the refresh token is
      // dead too and we have to re-authenticate.
      if (cfg && isRefreshable && !isFatal && !cfg._authRetried) {
        const tokens = getCurrentTokens(useAuthorizationStore.getState())
        const refreshToken = tokens?.refresh_token ?? ""
        if (refreshToken) {
          cfg._authRetried = true
          const newAccess = await refreshAccessToken(cfg, refreshToken)
          if (newAccess) {
            applyAuthHeader(cfg, newAccess)
            return BaseService(cfg)
          }
        }
      }

      // No refresh token, refresh failed, fatal code, or this is
      // the second 401 in a row: the session is genuinely gone.
      forceSignOut(
        code === "INVALID_CREDENTIALS"
          ? "Invalid credentials"
          : "Session expired. Please sign in again."
      )
      return Promise.reject(error)
    }

    if (status >= 500 && status < 600 && isDev()) {
      console.warn(`Server error ${status}:`, response.data)
    }

    return Promise.reject(error)
  }
)
