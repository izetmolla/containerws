import type { NavigateFunction } from "react-router"

import {
  check,
  type SignInResponse,
} from "@/modules/authorization/pages/sign-in/api"
import useAuthorizationStore from "@/store/authorization"
import type { AuthSession } from "@/types"

/** Client-side switch account: avoids a full reload to SSR /sign-in (504 on prod). */
export function navigateToSwitchAccount(navigate: NavigateFunction) {
  const returnTo = `${window.location.pathname}${window.location.search}`
  const params = new URLSearchParams({
    redirectUrl: returnTo,
    switchAccount: "1",
  })
  navigate(`/sign-in?${params.toString()}`, { replace: true })
}

export function redirectToSignIn(navigate?: NavigateFunction) {
  if (navigate) {
    navigateToSwitchAccount(navigate)
    return
  }
  window.location.replace("/sign-in")
}

export function removeStoredSession(sessionId: string) {
  useAuthorizationStore.setState((state) => {
    const nextSessions = state.sessions.filter(
      (session) => session.session_id !== sessionId
    )
    return {
      sessions: nextSessions,
      current_session:
        state.current_session === sessionId
          ? (nextSessions[0]?.session_id ?? "")
          : state.current_session,
    }
  })
}

export type VerifySessionResult =
  { ok: true; data: SignInResponse } | { ok: false; shouldSignIn: boolean }

export async function verifyStoredSession(
  session: AuthSession,
  previousSessionId: string
): Promise<VerifySessionResult> {
  const refreshToken = session.tokens?.refresh_token
  const { setCurrentSession, signIn } = useAuthorizationStore.getState()

  if (!refreshToken) {
    removeStoredSession(session.session_id)
    restorePreviousSession(previousSessionId, session.session_id)
    return {
      ok: false,
      shouldSignIn: useAuthorizationStore.getState().sessions.length === 0,
    }
  }

  setCurrentSession(session.session_id)

  try {
    const data = await check(refreshToken)
    signIn({
      session_id: data.session_id,
      user: data.user,
      tokens: data.tokens,
    })
    return { ok: true, data }
  } catch {
    removeStoredSession(session.session_id)
    restorePreviousSession(previousSessionId, session.session_id)

    const remaining = useAuthorizationStore.getState().sessions
    if (remaining.length === 0) {
      return { ok: false, shouldSignIn: true }
    }

    const hasPrevious = remaining.some(
      (item) => item.session_id === previousSessionId
    )
    return { ok: false, shouldSignIn: !hasPrevious }
  }
}

function restorePreviousSession(
  previousSessionId: string,
  attemptedSessionId: string
) {
  if (!previousSessionId || previousSessionId === attemptedSessionId) {
    return
  }

  const { sessions, setCurrentSession } = useAuthorizationStore.getState()
  if (sessions.some((session) => session.session_id === previousSessionId)) {
    setCurrentSession(previousSessionId)
  }
}
