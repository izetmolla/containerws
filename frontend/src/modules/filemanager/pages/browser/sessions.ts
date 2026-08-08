import { useCallback, useEffect, useState } from "react"

export type FileSession = {
  id: string
  path: string
  label: string
  /** Revision last seen when this tab was active. */
  seenRevision: number
}

const SESSIONS_KEY = "filemanager.sessions"
const ACTIVE_KEY = "filemanager.sessions.active"
const REVISION_KEY = "filemanager.sessions.revision"

function newId() {
  return `s_${Date.now().toString(36)}_${Math.random().toString(36).slice(2, 7)}`
}

function tabLabel(path: string) {
  if (!path || path === "/") return "/"
  const parts = path.replace(/\/+$/, "").split("/").filter(Boolean)
  return parts[parts.length - 1] || "/"
}

function loadSessions(): { sessions: FileSession[]; activeId: string } {
  try {
    const raw = localStorage.getItem(SESSIONS_KEY)
    const activeId = localStorage.getItem(ACTIVE_KEY) || ""
    if (!raw) {
      const id = newId()
      return {
        sessions: [{ id, path: "", label: "Home", seenRevision: 0 }],
        activeId: id,
      }
    }
    const sessions = JSON.parse(raw) as FileSession[]
    if (!Array.isArray(sessions) || sessions.length === 0) {
      const id = newId()
      return {
        sessions: [{ id, path: "", label: "Home", seenRevision: 0 }],
        activeId: id,
      }
    }
    const active =
      sessions.find((s) => s.id === activeId)?.id ?? sessions[0].id
    return { sessions, activeId: active }
  } catch {
    const id = newId()
    return {
      sessions: [{ id, path: "", label: "Home", seenRevision: 0 }],
      activeId: id,
    }
  }
}

function persist(sessions: FileSession[], activeId: string) {
  try {
    localStorage.setItem(SESSIONS_KEY, JSON.stringify(sessions))
    localStorage.setItem(ACTIVE_KEY, activeId)
  } catch {
    /* ignore */
  }
}

function readRevision() {
  try {
    return Number(localStorage.getItem(REVISION_KEY) || "0") || 0
  } catch {
    return 0
  }
}

function writeRevision(n: number) {
  try {
    localStorage.setItem(REVISION_KEY, String(n))
  } catch {
    /* ignore */
  }
}

export function useFileSessions() {
  const initial = loadSessions()
  const [sessions, setSessions] = useState<FileSession[]>(initial.sessions)
  const [activeId, setActiveId] = useState(initial.activeId)
  const [revision, setRevision] = useState(readRevision)

  const active = sessions.find((s) => s.id === activeId) ?? sessions[0]

  const bumpRevision = useCallback(() => {
    setRevision((prev) => {
      const next = prev + 1
      writeRevision(next)
      return next
    })
  }, [])

  const updateSessions = useCallback(
    (next: FileSession[], nextActive = activeId) => {
      setSessions(next)
      setActiveId(nextActive)
      persist(next, nextActive)
    },
    [activeId],
  )

  const setActivePath = useCallback(
    (path: string) => {
      setSessions((prev) => {
        const next = prev.map((s) =>
          s.id === activeId
            ? {
                ...s,
                path,
                label: tabLabel(path) || "Home",
                seenRevision: revision,
              }
            : s,
        )
        persist(next, activeId)
        return next
      })
    },
    [activeId, revision],
  )

  const switchSession = useCallback(
    (id: string) => {
      setSessions((prev) => {
        const next = prev.map((s) =>
          s.id === id ? { ...s, seenRevision: revision } : s,
        )
        persist(next, id)
        return next
      })
      setActiveId(id)
      try {
        localStorage.setItem(ACTIVE_KEY, id)
      } catch {
        /* ignore */
      }
    },
    [revision],
  )

  const addSession = useCallback(
    (path = "") => {
      const id = newId()
      const session: FileSession = {
        id,
        path,
        label: tabLabel(path) || "Home",
        seenRevision: revision,
      }
      setSessions((prev) => {
        const next = [...prev, session]
        persist(next, id)
        return next
      })
      setActiveId(id)
    },
    [revision],
  )

  const closeSession = useCallback(
    (id: string) => {
      setSessions((prev) => {
        if (prev.length <= 1) return prev
        const next = prev.filter((s) => s.id !== id)
        const nextActive =
          id === activeId
            ? next[Math.max(0, prev.findIndex((s) => s.id === id) - 1)]?.id ??
              next[0].id
            : activeId
        persist(next, nextActive)
        setActiveId(nextActive)
        return next
      })
    },
    [activeId],
  )

  const needsRefresh = Boolean(
    active && active.seenRevision < revision,
  )

  const markSeen = useCallback(() => {
    setSessions((prev) => {
      const next = prev.map((s) =>
        s.id === activeId ? { ...s, seenRevision: revision } : s,
      )
      persist(next, activeId)
      return next
    })
  }, [activeId, revision])

  useEffect(() => {
    const onStorage = (e: StorageEvent) => {
      if (e.key === REVISION_KEY) setRevision(readRevision())
      if (e.key === SESSIONS_KEY || e.key === ACTIVE_KEY) {
        const loaded = loadSessions()
        setSessions(loaded.sessions)
        setActiveId(loaded.activeId)
      }
    }
    window.addEventListener("storage", onStorage)
    return () => window.removeEventListener("storage", onStorage)
  }, [])

  return {
    sessions,
    activeId,
    active,
    revision,
    needsRefresh,
    bumpRevision,
    markSeen,
    setActivePath,
    switchSession,
    addSession,
    closeSession,
    updateSessions,
  }
}
