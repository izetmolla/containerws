"use client"

import { type FC, useMemo } from "react"

import { ContentLoaderBreadcrumb } from "./breadcrumb"
import type { BreadcrumbItem, ContentLoaderHeaderProps } from "./types"
import { cn } from "@/lib/utils"

const HOME_ITEM: BreadcrumbItem = { label: "Home", to: "/" }

/**
 * Prepends "Home" as the first breadcrumb item if the list is non-empty and does not already start with Home.
 * Does not mutate the incoming array.
 */
function breadcrumbWithHome(
  breadcrumb: BreadcrumbItem[] | undefined
): BreadcrumbItem[] {
  if (!breadcrumb?.length) return breadcrumb ?? []
  const first = breadcrumb[0]
  if (first?.label === "Home" && (first?.to === "/" || first?.to === undefined))
    return breadcrumb
  return [HOME_ITEM, ...breadcrumb]
}

/**
 * Builds the breadcrumb array with the current page title appended if missing.
 * Does not mutate the incoming array.
 */
function breadcrumbWithTitle(
  breadcrumb: BreadcrumbItem[] | undefined,
  title: string | undefined
): BreadcrumbItem[] {
  if (!title || !breadcrumb?.length) return breadcrumb ?? []
  const hasTitle = breadcrumb.some((item) => item.label === title)
  if (hasTitle) return breadcrumb
  return [...breadcrumb, { label: title }]
}

/**
 * Page header with optional title, description, breadcrumb trail, and right-side actions.
 * Used by ContentLoader for consistent layout above loading, error, or content.
 */
export const ContentLoaderHeader: FC<ContentLoaderHeaderProps> = ({
  title,
  description,
  breadcrumb,
  rightComponent,
  customTitle,
  error,
  forMeta,
  showHeaderSeparator = false,
  headerSeparatorMarginY = "mb-6",
  headerClassName,
}) => {
  const breadcrumbWithCurrent = useMemo(() => {
    const withHome = breadcrumbWithHome(breadcrumb)
    return breadcrumbWithTitle(withHome, title)
  }, [breadcrumb, title])

  if (forMeta) return null

  const showHeader = Boolean(
    title ??
    description ??
    customTitle ??
    rightComponent ??
    breadcrumbWithCurrent.length > 0
  )
  if (!showHeader) return null

  return (
    <header
      className={cn(
        "flex flex-col gap-3",
        showHeaderSeparator
          ? cn("border-b border-border/80 pb-5", headerSeparatorMarginY)
          : "mb-4",
        headerClassName
      )}
    >
      {breadcrumbWithCurrent.length > 0 ? (
        <ContentLoaderBreadcrumb breadcrumb={breadcrumbWithCurrent} />
      ) : null}

      <div className="flex flex-col gap-4 lg:flex-row lg:items-end lg:justify-between">
        <div className="min-w-0 flex-1 space-y-1.5">
          {customTitle && !error ? (
            customTitle
          ) : title ? (
            <h1 className="text-2xl font-semibold tracking-tight text-foreground">
              {title}
            </h1>
          ) : null}
          {description ? (
            <p className="max-w-2xl text-sm leading-relaxed text-muted-foreground">
              {description}
            </p>
          ) : null}
        </div>

        {rightComponent ? (
          <div className="flex shrink-0 flex-wrap items-center gap-2 lg:justify-end">
            {rightComponent}
          </div>
        ) : null}
      </div>
    </header>
  )
}
