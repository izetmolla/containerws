"use client"

import { type FC, useEffect, useMemo, useState } from "react"

import { ContentLoaderContext } from "./context"
import { ContentLoaderHeader } from "./header"
import { DefaultSpinner } from "./default-spinner"
import { buildLoaderSizeStyle } from "./loader-size-style"
import { ContentLoaderErrorView } from "./error-view"
import type { ContentLoaderProps } from "./types"

const META_DESCRIPTION_ID = "content-loader-meta-description"

/** Shared header props derived from ContentLoader props; avoids repetition. */
function useHeaderProps(
  props: ContentLoaderProps & {
    errorMessage?: string
    title: string
    description: string
  }
) {
  const {
    title,
    description,
    breadcrumb,
    rightComponent,
    customTitle,
    forMeta,
    errorMessage,
    header,
  } = props

  return useMemo(
    () => ({
      title,
      description,
      breadcrumb,
      rightComponent,
      customTitle,
      forMeta,
      error: errorMessage,
      showHeaderSeparator: props.showHeaderSeparator,
      headerSeparatorMarginY: props.headerSeparatorMarginY,
      headerClassName: props.headerClassName,
      header,
    }),
    [
      title,
      description,
      breadcrumb,
      rightComponent,
      customTitle,
      forMeta,
      errorMessage,
      props.showHeaderSeparator,
      props.headerSeparatorMarginY,
      props.headerClassName,
      header,
    ]
  )
}

/**
 * Sets document.title and optional meta description. When forMeta is true, only these are set
 * (header is not rendered). When forMeta is false, the header is also rendered with title and description.
 * Children can update title/description via useContentLoader().
 */
const ContentLoader: FC<ContentLoaderProps> = (props) => {
  const {
    title: titleProp,
    description: descriptionProp,
    isLoading,
    showHeaderOnLoader,
    error,
    minimalError,
    children,
    customLoader,
    breadcrumb,
    header,
    centered,
    minLoaderHeight,
    maxLoaderHeight,
  } = props

  const [title, setTitle] = useState(titleProp ?? "")
  const [description, setDescription] = useState(descriptionProp ?? "")
  const [prevTitleProp, setPrevTitleProp] = useState(titleProp)
  const [prevDescriptionProp, setPrevDescriptionProp] =
    useState(descriptionProp)

  // Sync from props when parent updates title/description (parent can still override).
  // Adjust during render instead of an effect to avoid cascading renders.
  if (titleProp !== prevTitleProp) {
    setPrevTitleProp(titleProp)
    if (titleProp !== undefined) setTitle(titleProp)
  }
  if (descriptionProp !== prevDescriptionProp) {
    setPrevDescriptionProp(descriptionProp)
    if (descriptionProp !== undefined) setDescription(descriptionProp)
  }

  const errorMessage = error?.message
  const effectiveBreadcrumb = breadcrumb

  // Apply title/description to document and meta (driven by state so children can update).
  useEffect(() => {
    if (title) document.title = title
  }, [title])

  useEffect(() => {
    let el = document.getElementById(
      META_DESCRIPTION_ID
    ) as HTMLMetaElement | null
    if (description) {
      if (!el) {
        el = document.createElement("meta")
        el.id = META_DESCRIPTION_ID
        el.name = "description"
        document.head.appendChild(el)
      }
      el.content = description
    } else if (el) {
      el.remove()
    }
    return () => {
      const existing = document.getElementById(META_DESCRIPTION_ID)
      if (existing) existing.remove()
    }
  }, [description])

  const contextValue = useMemo(
    () => ({
      title,
      setTitle,
      description,
      setDescription,
    }),
    [title, description]
  )

  const headerProps = useHeaderProps({
    ...props,
    title,
    description,
    breadcrumb: effectiveBreadcrumb,
    errorMessage,
  })

  const centeredState = centered && (isLoading || Boolean(error))

  const loaderWrapperStyle = buildLoaderSizeStyle(
    minLoaderHeight,
    maxLoaderHeight
  )

  const loadingBody = (
    <>
      {showHeaderOnLoader && <ContentLoaderHeader {...headerProps} />}
      {customLoader ? (
        <div className="overflow-auto" style={loaderWrapperStyle}>
          {customLoader}
        </div>
      ) : (
        <DefaultSpinner
          centered={centered}
          minLoaderHeight={minLoaderHeight}
          maxLoaderHeight={maxLoaderHeight}
        />
      )}
    </>
  )

  const errorBody = (
    <>
      <ContentLoaderHeader {...headerProps} />
      <ContentLoaderErrorView
        error={error!}
        minimal={minimalError}
        centered={centered}
      />
    </>
  )

  const successBody = (
    <>
      <ContentLoaderHeader {...headerProps} />
      {children}
    </>
  )

  const content = isLoading ? loadingBody : error ? errorBody : successBody

  return (
    <ContentLoaderContext.Provider value={contextValue}>
      {header && header}
      {centeredState ? (
        <div className="flex min-h-0 w-full flex-1 items-center justify-center">
          {content}
        </div>
      ) : (
        content
      )}
    </ContentLoaderContext.Provider>
  )
}

export default ContentLoader

/** @deprecated Use ContentLoaderErrorView for new code. Exported for backward compatibility. */
export const RenderError = ContentLoaderErrorView

export type {
  ContentLoaderProps,
  BreadcrumbItem,
  CustomBreadcrumbProps,
} from "./types"
