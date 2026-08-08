"use client"

import { Fragment, type FC } from "react"
import { Link } from "react-router"

import {
  Breadcrumb,
  BreadcrumbItem,
  BreadcrumbLink,
  BreadcrumbList,
  BreadcrumbPage,
  BreadcrumbSeparator,
} from "@/components/ui/breadcrumb"

import type { CustomBreadcrumbProps } from "./types"

const getBasePath = (path: string) => {
  return path.startsWith("/") ? path : `/${path}`
}
/**
 * Renders a breadcrumb trail from an array of { name, to? } items.
 * Items with `to` render as links; the last or without `to` render as current page.
 */
export const ContentLoaderBreadcrumb: FC<CustomBreadcrumbProps> = ({
  breadcrumb,
}) => {
  if (!breadcrumb.length) return null

  return (
    <Breadcrumb>
      <BreadcrumbList>
        {breadcrumb.map((item, index) => {
          const isLast = index === breadcrumb.length - 1
          return (
            <Fragment key={index}>
              <BreadcrumbItem>
                {item.to ? (
                  <BreadcrumbLink asChild>
                    <Link to={getBasePath(item.to) ?? "#"}>{item.label}</Link>
                  </BreadcrumbLink>
                ) : (
                  <BreadcrumbPage>{item.label}</BreadcrumbPage>
                )}
              </BreadcrumbItem>
              {!isLast && <BreadcrumbSeparator />}
            </Fragment>
          )
        })}
      </BreadcrumbList>
    </Breadcrumb>
  )
}
