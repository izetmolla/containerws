"use client"

import { createContext, useContext } from "react"

export interface ContentLoaderContextValue {
  /** Current page title (document.title and header). */
  title: string
  /** Set the page title. Updates document.title and the header. */
  setTitle: (title: string) => void
  /** Current page description (header and meta description). */
  description: string
  /** Set the page description. Updates meta description and the header. */
  setDescription: (description: string) => void
}

export const ContentLoaderContext =
  createContext<ContentLoaderContextValue | null>(null)

/**
 * Hook to read and update the ContentLoader page title and description from any child.
 * Must be used inside a ContentLoader.
 *
 * @example
 * const { title, setTitle, description, setDescription } = useContentLoader();
 * setTitle("Edit: " + item.name);
 */
export function useContentLoader(): ContentLoaderContextValue {
  const ctx = useContext(ContentLoaderContext)
  if (ctx == null) {
    throw new Error("useContentLoader must be used within a ContentLoader")
  }
  return ctx
}

/**
 * Optional hook: returns the context value or null if outside ContentLoader.
 * Use when the component may render outside ContentLoader.
 */
export function useContentLoaderOptional(): ContentLoaderContextValue | null {
  return useContext(ContentLoaderContext)
}
