import { useCallback, useEffect, useState } from "react"

export type ClipboardMode = "copy" | "cut"

export type FileClipboard = {
  mode: ClipboardMode
  paths: string[]
}

const STORAGE_KEY = "filemanager.clipboard"

function readClipboard(): FileClipboard | null {
  try {
    const raw = sessionStorage.getItem(STORAGE_KEY)
    if (!raw) return null
    const parsed = JSON.parse(raw) as FileClipboard
    if (
      (parsed.mode !== "copy" && parsed.mode !== "cut") ||
      !Array.isArray(parsed.paths) ||
      parsed.paths.length === 0
    ) {
      return null
    }
    return parsed
  } catch {
    return null
  }
}

function writeClipboard(value: FileClipboard | null) {
  try {
    if (!value) sessionStorage.removeItem(STORAGE_KEY)
    else sessionStorage.setItem(STORAGE_KEY, JSON.stringify(value))
  } catch {
    /* ignore */
  }
}

export function useFileClipboard() {
  const [clipboard, setClipboardState] = useState<FileClipboard | null>(() =>
    readClipboard(),
  )

  const setClipboard = useCallback((value: FileClipboard | null) => {
    setClipboardState(value)
    writeClipboard(value)
  }, [])

  const copyPaths = useCallback(
    (paths: string[]) => {
      const unique = Array.from(new Set(paths.filter(Boolean)))
      if (!unique.length) return
      setClipboard({ mode: "copy", paths: unique })
    },
    [setClipboard],
  )

  const cutPaths = useCallback(
    (paths: string[]) => {
      const unique = Array.from(new Set(paths.filter(Boolean)))
      if (!unique.length) return
      setClipboard({ mode: "cut", paths: unique })
    },
    [setClipboard],
  )

  const clearClipboard = useCallback(() => setClipboard(null), [setClipboard])

  useEffect(() => {
    const onStorage = (e: StorageEvent) => {
      if (e.key === STORAGE_KEY) setClipboardState(readClipboard())
    }
    window.addEventListener("storage", onStorage)
    return () => window.removeEventListener("storage", onStorage)
  }, [])

  return {
    clipboard,
    copyPaths,
    cutPaths,
    clearClipboard,
    setClipboard,
  }
}
