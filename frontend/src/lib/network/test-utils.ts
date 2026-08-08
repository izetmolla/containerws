/**
 * Shared test fixtures for the network layer.
 *
 * Filename intentionally does not end in `.test.ts` so vitest does
 * NOT pick it up as a test file - it's pure helper code consumed by
 * the real tests.
 */

import type {
  AxiosAdapter,
  AxiosResponse,
  InternalAxiosRequestConfig,
} from "axios"

/**
 * Builds a JWT string with the supplied expiry, in milliseconds since
 * the epoch. The signing key doesn't matter because the frontend
 * never verifies the signature; it only decodes the payload.
 */
export function makeJwt(expMs: number): string {
  const header = { alg: "HS256", typ: "JWT" }
  const payload = {
    exp: Math.floor(expMs / 1000),
    iat: Math.floor(Date.now() / 1000),
  }
  const base64 = (obj: object): string =>
    btoa(JSON.stringify(obj))
      .replace(/\+/g, "-")
      .replace(/\//g, "_")
      .replace(/=+$/g, "")
  return `${base64(header)}.${base64(payload)}.signature-not-checked`
}

export interface MockedRequest {
  url?: string
  method?: string
  headers: Record<string, string>
  data?: unknown
}

export interface MockedReply {
  status: number
  data: unknown
}

export interface MockAdapter {
  adapter: AxiosAdapter
  calls: MockedRequest[]
  push: (
    handler: (req: MockedRequest) => MockedReply | Promise<MockedReply>
  ) => void
  reset: () => void
}

/**
 * A minimal axios adapter that records each outbound request and
 * lets tests script the response. We avoid `axios-mock-adapter`
 * because its version skew with axios v1 has caused intermittent
 * test failures in the past.
 */
export function createMockAdapter(): MockAdapter {
  const calls: MockedRequest[] = []
  let queue: Array<(req: MockedRequest) => MockedReply | Promise<MockedReply>> =
    []

  const adapter: AxiosAdapter = async (config) => {
    const headers: Record<string, string> = {}
    for (const [k, v] of Object.entries(config.headers ?? {})) {
      if (typeof v === "string") headers[k] = v
    }
    const req: MockedRequest = {
      url: config.url,
      method: config.method,
      headers,
      data: config.data,
    }
    calls.push(req)

    const handler = queue.shift()
    if (!handler) {
      throw new Error(
        `MockAdapter: no scripted reply for ${req.method?.toUpperCase()} ${req.url}`
      )
    }
    const reply = await handler(req)
    const response: AxiosResponse = {
      status: reply.status,
      statusText: "",
      headers: {},
      config: config as InternalAxiosRequestConfig,
      data: reply.data,
      request: {},
    }
    if (reply.status >= 200 && reply.status < 300) {
      return response
    }
    // Match axios's behaviour: throw a wrapped AxiosError so the
    // response interceptor's error branch fires.
    const err = new Error(
      `Request failed with status code ${reply.status}`
    ) as Error & {
      isAxiosError: boolean
      response: AxiosResponse
      config: InternalAxiosRequestConfig
    }
    err.isAxiosError = true
    err.response = response
    err.config = config as InternalAxiosRequestConfig
    throw err
  }

  return {
    adapter,
    calls,
    push(handler) {
      queue.push(handler)
    },
    reset() {
      queue = []
      calls.length = 0
    },
  }
}
