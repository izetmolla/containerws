/**
 * Public façade for the network layer.
 *
 * The actual implementation is split across small focused modules
 * (constants, jwt, refresh, interceptors, etc.) in this folder. This
 * barrel exists for two reasons:
 *
 *   1. Importing it triggers the `./interceptors` side effect that
 *      wires the auth pipeline onto `BaseService`. Every consumer
 *      already imports something from `@/lib/network`,
 *      so the side effect is guaranteed to run.
 *   2. It preserves the historical import surface so downstream apps
 *      keep working without per-file changes.
 */

import "./interceptors"

import ApiService from "./api-service"

export { REQUEST_HEADER_AUTH_KEY, TOKEN_TYPE } from "./constants"
export { exceptedPaths, baseApiURL, isDev, getModuleName } from "./env"
export { getTokenExpiryMs, isAccessTokenStale, isValidJwtFormat } from "./jwt"
export { BaseService } from "./client"
export { refreshAccessToken } from "./refresh"
export type { ResponseWithError, ResponseWithPagination } from "./api-service"
export {
  getApiErrorMessageFromBody,
  getRequestErrorInfo,
  getRequestErrorMessage,
  isApiErrorBody,
  toastRequestError,
  withError,
} from "./errors"
export {
  type WithPagination,
  withAPI,
  withInitialData,
  withModule,
} from "./helpers"
export { queryClient } from "./query-client"

export default ApiService
