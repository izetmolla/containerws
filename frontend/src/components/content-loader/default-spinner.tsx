"use client"

import { type FC } from "react"

import { cn } from "@/lib/utils"

import { buildLoaderSizeStyle } from "./loader-size-style"

interface DefaultSpinnerProps {
  centered?: boolean
  minLoaderHeight?: string | number
  maxLoaderHeight?: string | number
}

/**
 * Default loading spinner shown by ContentLoader when no customLoader is provided.
 * Kept in a separate component so it can be memoized and avoid unnecessary re-renders.
 */
export const DefaultSpinner: FC<DefaultSpinnerProps> = ({
  centered = false,
  minLoaderHeight,
  maxLoaderHeight,
}) => {
  const sizeStyle = buildLoaderSizeStyle(minLoaderHeight, maxLoaderHeight)
  const hasCustomHeight = sizeStyle !== undefined

  return (
    <div
      className={cn(
        "flex items-center justify-center",
        centered ? "w-full" : hasCustomHeight ? undefined : "h-[70vh]"
      )}
      style={sizeStyle}
      role="status"
      aria-label="Loading"
    >
      <span className="relative flex h-5 w-5">
        <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-blue-200 opacity-50" />
        <span className="relative inline-flex h-5 w-5 rounded-full bg-blue-300" />
      </span>
    </div>
  )
}
