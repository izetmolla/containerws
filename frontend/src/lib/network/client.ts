/**
 * The shared axios instance.
 *
 * Interceptors are registered separately (see `./interceptors.ts`)
 * so this module can be imported in tests that want to drive the
 * raw client without the auth pipeline running.
 */

import axios from "axios"

import { baseApiURL, isDev } from "./env"

/**
 * Single axios instance used by the entire app. A 60s timeout is
 * generous enough for slow report endpoints but short enough to
 * surface a stuck backend without locking the UI forever.
 */
export const BaseService = axios.create({
  timeout: 60_000,
  baseURL: baseApiURL(),
})

if (isDev()) {
  console.log("API Base URL:", BaseService.defaults.baseURL)
}
