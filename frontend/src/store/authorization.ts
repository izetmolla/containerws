import axios from "axios"
import ApiService from "../lib/network/api-service"
import type { AuthSession, Tokens, User } from "@/types"
import { create, type StateCreator } from "zustand"
import { devtools } from "zustand/middleware"
import {
  persist,
  createJSONStorage,
  type StateStorage,
} from "zustand/middleware"

export type { AuthSession }

export interface AuthorizationState {
  current_session: string
  sessions: AuthSession[]
  redirectUrl?: string
  accessDenied: boolean
  getCurrentUser: () => User | undefined
  signIn: (input: {
    session_id?: string
    user: Pick<User, "id"> | User
    tokens: Tokens
    trusted?: boolean
  }) => void
  signOut: () => void
  signOutAll: () => void
  setTrusted: (session_id: string, trusted: boolean) => void
  setCurrentSession: (session_id: string) => void
  setAccessToken: (access_token: string) => void
  setRedirectUrl: (url: string) => void
  setAccessDenied: (accessDenied: boolean) => void
  clearAccessDenied: () => void
}

export function getCurrentSession(
  state: Pick<AuthorizationState, "sessions" | "current_session">
): AuthSession | undefined {
  if (!state.current_session) return undefined
  return state.sessions.find((s) => s.session_id === state.current_session)
}

export function getCurrentTokens(
  state: Pick<AuthorizationState, "sessions" | "current_session">
): Tokens | undefined {
  return getCurrentSession(state)?.tokens
}

export function isSignedIn(
  state: Pick<AuthorizationState, "sessions" | "current_session">
): boolean {
  const tokens = getCurrentTokens(state)
  return Boolean(tokens?.access_token || tokens?.refresh_token)
}

function resolveSessionId(user: Pick<User, "id">, session_id?: string): string {
  if (user.id) return user.id
  if (session_id) return session_id
  if (typeof crypto !== "undefined" && "randomUUID" in crypto) {
    return crypto.randomUUID()
  }
  return `session-${Date.now()}`
}

function normalizeSession(raw: unknown): AuthSession | null {
  if (!raw || typeof raw !== "object") return null
  const record = raw as Record<string, unknown>
  const userRecord = record.user
  if (!userRecord || typeof userRecord !== "object") return null
  const userId = (userRecord as { id?: unknown }).id
  if (typeof userId !== "string" || !userId) return null

  const session_id = userId

  const tokensRecord = record.tokens
  if (!tokensRecord || typeof tokensRecord !== "object") return null
  const access_token = (tokensRecord as { access_token?: unknown }).access_token
  const refresh_token = (tokensRecord as { refresh_token?: unknown })
    .refresh_token
  if (typeof access_token !== "string" || typeof refresh_token !== "string") {
    return null
  }
  if (!access_token && !refresh_token) return null

  return {
    session_id,
    trusted: Boolean(record.trusted),
    user: { id: userId, ...userRecord } as User,
    tokens: { access_token, refresh_token },
  }
}

function migratePersistedState(
  persisted:
    Partial<AuthorizationState & { user?: User; tokens?: Tokens }> | undefined
): Pick<AuthorizationState, "sessions" | "current_session"> {
  if (!persisted) {
    return { sessions: [], current_session: "" }
  }

  const rawSessions = persisted.sessions
  if (Array.isArray(rawSessions)) {
    const sessions = [
      ...new Map(
        rawSessions
          .map(normalizeSession)
          .filter((session): session is AuthSession => session !== null)
          .map((session) => [session.session_id, session] as const)
      ).values(),
    ]
    const current =
      persisted.current_session &&
      sessions.some((s) => s.session_id === persisted.current_session)
        ? persisted.current_session
        : (sessions[0]?.session_id ?? "")
    return { sessions, current_session: current }
  }

  if (persisted.user && persisted.tokens) {
    const session = normalizeSession({
      session_id: persisted.current_session,
      user: persisted.user,
      tokens: persisted.tokens,
      trusted: false,
    })
    if (!session) {
      return { sessions: [], current_session: "" }
    }
    return {
      sessions: [session],
      current_session: session.session_id,
    }
  }

  return {
    sessions: [],
    current_session: persisted.current_session ?? "",
  }
}

interface PersistedAuthorizationSnapshot {
  sessions: AuthSession[]
  current_session: string
  redirectUrl?: string
}

function isZustandPersistEnvelope(
  value: unknown
): value is { state: PersistedAuthorizationSnapshot; version?: number } {
  return (
    typeof value === "object" &&
    value !== null &&
    "state" in value &&
    typeof (value as { state: unknown }).state === "object"
  )
}

function parsePersistedSnapshot(
  raw: string | null
): PersistedAuthorizationSnapshot | null {
  if (!raw) return null
  try {
    const parsed: unknown = JSON.parse(raw)
    if (isZustandPersistEnvelope(parsed)) {
      return parsed.state
    }
    if (
      typeof parsed === "object" &&
      parsed !== null &&
      "sessions" in parsed &&
      Array.isArray((parsed as PersistedAuthorizationSnapshot).sessions)
    ) {
      return parsed as PersistedAuthorizationSnapshot
    }
    return null
  } catch {
    return null
  }
}

function createAuthorizationPersistStorage(): StateStorage {
  return {
    getItem: (name) => {
      const snapshot = parsePersistedSnapshot(localStorage.getItem(name))
      if (!snapshot) return null
      return JSON.stringify({ state: snapshot, version: 0 })
    },
    setItem: (name, value) => {
      try {
        const parsed: unknown = JSON.parse(value)
        const snapshot = isZustandPersistEnvelope(parsed)
          ? parsed.state
          : (parsed as PersistedAuthorizationSnapshot)
        localStorage.setItem(name, JSON.stringify(snapshot))
      } catch {
        localStorage.setItem(name, value)
      }
    },
    removeItem: (name) => {
      localStorage.removeItem(name)
    },
  }
}

async function callSignOutEndpoint(): Promise<void> {
  try {
    await axios({
      method: "post",
      url: "/api/authorization/sign-out",
      withCredentials: true,
      timeout: 5000,
    })
  } catch {
    /* idempotent: already signed out / network down / cookie missing */
  }
}

const authorizationStore: StateCreator<AuthorizationState> = (set, get) => ({
  current_session: "",
  sessions: [],
  redirectUrl: "",
  accessDenied: false,
  getCurrentUser: () =>
    (get().sessions.find((s) => s.session_id === get().current_session)
      ?.user as User | undefined) ?? undefined,
  setRedirectUrl: (url) => set({ redirectUrl: url }),
  setAccessDenied: (accessDenied) => set({ accessDenied }),
  clearAccessDenied: () => set({ accessDenied: false }),
  signIn: ({ session_id, user, tokens, trusted = false }) => {
    const id = resolveSessionId(user, session_id)
    set((state) => {
      const sessions = [
        ...state.sessions.filter((s) => s.session_id !== id),
        {
          session_id: id,
          user: { ...user, id: user.id } as User,
          tokens,
          trusted,
        },
      ]
      return {
        sessions,
        current_session: id,
        accessDenied: false,
      }
    })
  },
  signOut: () => {
    const { current_session } = get()
    void callSignOutEndpoint()
    set((state) => {
      const sessions = state.sessions.filter(
        (s) => s.session_id !== current_session
      )
      return {
        sessions,
        current_session: sessions[0]?.session_id ?? "",
        accessDenied: false,
      }
    })
  },
  signOutAll: () => {
    void callSignOutEndpoint()
    set({
      sessions: [],
      current_session: "",
      accessDenied: false,
    })
  },
  setTrusted: (session_id, trusted) =>
    set((state) => ({
      sessions: state.sessions.map((s) =>
        s.session_id === session_id ? { ...s, trusted } : s
      ),
    })),
  setCurrentSession: (session_id) => {
    const { sessions } = get()
    if (!sessions.some((s) => s.session_id === session_id)) return
    set({ current_session: session_id, accessDenied: false })
  },
  setAccessToken: (access_token) =>
    set((state) => ({
      sessions: state.sessions.map((s) =>
        s.session_id === state.current_session
          ? {
              ...s,
              tokens: {
                ...s.tokens,
                access_token,
              },
            }
          : s
      ),
    })),
})

export function signOutApi() {
  return ApiService.fetchData({
    url: "/api/authorization/sign-out",
    method: "post",
  })
}

/** @deprecated Use signOutApi — kept for call-site migration. */
export function useSignOutApi() {
  return signOutApi()
}

const useAuthorizationStore = create<AuthorizationState>()(
  devtools(
    persist(authorizationStore, {
      name: "authorization-storage",
      storage: createJSONStorage(() => createAuthorizationPersistStorage()),
      partialize: (state) => ({
        sessions: state.sessions,
        current_session: state.current_session,
        redirectUrl: state.redirectUrl,
      }),
      merge: (persistedState, currentState) => {
        const migrated = migratePersistedState(
          persistedState as Partial<AuthorizationState>
        )
        let currentSession = migrated.current_session
        if (
          currentSession &&
          !migrated.sessions.some((s) => s.session_id === currentSession)
        ) {
          currentSession = migrated.sessions[0]?.session_id ?? ""
        }
        return {
          ...currentState,
          sessions: migrated.sessions,
          current_session: currentSession,
          redirectUrl:
            (persistedState as Partial<AuthorizationState>)?.redirectUrl ??
            currentState.redirectUrl,
        }
      },
    }),
    {
      name: "authorization-storage",
      enabled: import.meta.env.DEV,
    }
  )
)

export function useCurrentUser() {
  return useAuthorizationStore((state) => state.getCurrentUser())
}

export default useAuthorizationStore
