/**
 * Public auth-flow constants and the small string-set "vocabularies"
 * the request/response interceptors use to classify backend errors.
 *
 * These values are deliberately kept in their own module so they can
 * be imported by tests and other tooling without dragging in axios or
 * the auth store as transitive dependencies.
 */

/** Authorization scheme prefix used for Bearer tokens. */
export const TOKEN_TYPE = "Bearer "

/** HTTP header that carries the access token. */
export const REQUEST_HEADER_AUTH_KEY = "Authorization"

/**
 * Proactive refresh window before JWT `exp`. A new access token is minted
 * when the remaining lifetime drops to this value (2 seconds).
 */
export const REFRESH_SKEW_MS = 2 * 1000

/**
 * Upper bound on a refresh-token round trip. A wedged backend must not
 * keep every queued request blocked behind a single refresh call.
 */
export const REFRESH_TIMEOUT_MS = 15 * 1000

/**
 * Codes that mean the caller's session is unrecoverable and they must
 * re-authenticate. The interceptor will sign the user out without
 * attempting a refresh.
 */
export const FATAL_AUTH_CODES: ReadonlySet<string> = new Set([
  "INVALID_CREDENTIALS",
])

/**
 * Codes that look like an access-token problem and are usually fixed
 * by minting a new access token via the refresh flow.
 */
export const REFRESHABLE_AUTH_CODES: ReadonlySet<string> = new Set([
  "TOKEN_EXPIRED",
  "TOKEN_INVALID",
  "AUTH_REQUIRED",
])

/**
 * Codes that indicate the user IS signed in but lacks the rights for
 * this resource. They must NEVER trigger sign-out, even when paired
 * with a 401 status (some routes use 401 + INSUFFICIENT_PERMISSIONS
 * to distinguish "logged out" from "logged in but blocked").
 */
export const PERMISSION_CODES: ReadonlySet<string> = new Set([
  "INSUFFICIENT_PERMISSIONS",
  "ROLE_NOT_ALLOWED",
  "API_KEY_FORBIDDEN",
])
