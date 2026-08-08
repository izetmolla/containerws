import { type CSSProperties } from "react"

function toCssLength(value: string | number): string {
  return typeof value === "number" ? `${value}px` : value
}

export function buildLoaderSizeStyle(
  minLoaderHeight?: string | number,
  maxLoaderHeight?: string | number
): CSSProperties | undefined {
  if (minLoaderHeight === undefined && maxLoaderHeight === undefined) {
    return undefined
  }

  const style: CSSProperties = {}

  if (minLoaderHeight !== undefined) {
    style.minHeight = toCssLength(minLoaderHeight)
  }
  if (maxLoaderHeight !== undefined) {
    style.maxHeight = toCssLength(maxLoaderHeight)
    if (minLoaderHeight === undefined) {
      style.height = toCssLength(maxLoaderHeight)
    }
  }

  return style
}
