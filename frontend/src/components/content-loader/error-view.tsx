"use client"

import { type FC, useMemo } from "react"

import { cn } from "@/lib/utils"
import type { ApiErrorResponse } from "./types"

interface ContentLoaderErrorViewProps {
  error: Error
  /** Compact layout for constrained areas (e.g. sidebar). */
  minimal?: boolean
  /** Fill parent and center content instead of using a fixed viewport height. */
  centered?: boolean
}

/**
 * Renders error message and optional API response details (message, code, details).
 * Used by ContentLoader in the error state and exported for reuse.
 */
export const ContentLoaderErrorView: FC<ContentLoaderErrorViewProps> = ({
  error,
  minimal = false,
  centered = false,
}) => {
  const apiResponse = useMemo((): ApiErrorResponse | null => {
    const response = (
      error as Error & { response?: { data?: ApiErrorResponse } }
    ).response?.data
    return response ?? null
  }, [error])

  if (minimal) {
    return (
      <div className="px-2 py-1.5 text-xs text-destructive" role="alert">
        <p className="leading-tight font-medium break-words">{error.message}</p>
        {apiResponse?.message && apiResponse.message !== error.message && (
          <p className="mt-0.5 line-clamp-2 text-muted-foreground">
            {apiResponse.message}
          </p>
        )}
      </div>
    )
  }

  return (
    <div
      className={cn(
        "flex flex-col items-center justify-center gap-2 text-center text-destructive",
        centered ? "w-full px-4" : "h-[70vh]"
      )}
      role="alert"
    >
      <p className="font-medium">{error.message}</p>
      {apiResponse?.message && <p className="text-sm">{apiResponse.message}</p>}
      {apiResponse?.code && (
        <p className="text-xs text-muted-foreground">{apiResponse.code}</p>
      )}
      {apiResponse?.details != null && (
        <pre className="max-w-full overflow-auto rounded bg-muted px-2 py-1 text-left text-xs">
          {JSON.stringify(apiResponse.details, null, 2)}
        </pre>
      )}
    </div>
  )
}
