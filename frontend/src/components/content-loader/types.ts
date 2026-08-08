/**
 * Shared types for the ContentLoader component and its subcomponents.
 */

import type { ReactNode } from "react"

/** Single breadcrumb link or page (current). */
export interface BreadcrumbItem {
  label: string
  to?: string
}

/** Props for the custom breadcrumb list. */
export interface CustomBreadcrumbProps {
  breadcrumb: BreadcrumbItem[]
}

/** Props for the page header (title, description, breadcrumb, actions). */
export interface ContentLoaderHeaderProps {
  title?: string
  description?: string
  /** Breadcrumb items; current page title is appended if not already present. */
  breadcrumb?: BreadcrumbItem[]
  rightComponent?: ReactNode
  customTitle?: ReactNode
  error?: string
  /**
   * When true, header is not rendered on the page.
   * Used with ContentLoader forMeta so only document.title (and meta description) are set.
   */
  forMeta?: boolean
  /** When true, a separator (border) is shown between the header and the body. Default false. */
  showHeaderSeparator?: boolean
  /**
   * Tailwind class(es) for vertical margin from the separator (space above/below the line).
   * Default "mb-6" (margin below). Use e.g. "mb-4", "mb-8", or "my-4" for both.
   */
  headerSeparatorMarginY?: string
  /** Merged onto the header row `<div>` (e.g. relative z-index above a dimmed backdrop). */
  headerClassName?: string
}

/** Props for the main ContentLoader wrapper. */
export interface ContentLoaderProps {
  /** Page title; sets document.title when provided. */
  title?: string
  /** Short description; shown in the page header and set as meta description when provided. */
  description?: string
  /** Show loading spinner or customLoader. */
  isLoading?: boolean
  header?: ReactNode
  /** When true, header is shown while loading; otherwise only when content or error is shown. */
  showHeaderOnLoader?: boolean
  /** When set, error state is shown with optional header. */
  error?: Error | null
  /**
   * When true, error UI is compact (e.g. sidebar) instead of a full centered block.
   */
  minimalError?: boolean
  children?: ReactNode
  /** Custom loading UI instead of default spinner. */
  customLoader?: ReactNode
  breadcrumb?: BreadcrumbItem[]
  customTitle?: ReactNode
  rightComponent?: ReactNode
  /**
   * When true: only set document.title (and meta description if description is set);
   * header is not rendered. Use for meta/SEO without visible title on the page.
   * When false (default): set document.title and render the header (title, description, breadcrumb).
   */
  forMeta?: boolean
  /** When true, a separator is shown between the header and the body. Default false. */
  showHeaderSeparator?: boolean
  /**
   * Tailwind class(es) for vertical margin from the separator. Default "mb-6".
   * E.g. "mb-4", "mb-8", "my-4".
   */
  headerSeparatorMarginY?: string
  /** Forwarded to the page header wrapper; see [ContentLoaderHeaderProps.headerClassName]. */
  headerClassName?: string
  /**
   * When true, loading and error states fill the available space and are centered
   * (e.g. dialogs or embedded panels). Default false uses a fixed viewport height.
   */
  centered?: boolean
  /**
   * Optional max height for the loading state (e.g. "220px" or 220).
   * Useful in dialogs or compact panels instead of the default 70vh spinner area.
   */
  maxLoaderHeight?: string | number
  /**
   * Optional min height for the loading state (e.g. "220px" or 220).
   * Useful in dialogs or compact panels to prevent the loader area from collapsing.
   */
  minLoaderHeight?: string | number
}

/** Shape of API error response (optional fields). */
export interface ApiErrorResponse {
  message?: string
  code?: string
  details?: unknown
}
