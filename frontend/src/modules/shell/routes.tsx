import type { RouteObject } from "react-router"
import { Fragment } from "react/jsx-runtime"

const shellRoutes: RouteObject[] = [
  {
    index: true,
    lazy: () =>
      import("./pages/shell").then((module) => ({
        Component: module.default,
      })),
    hydrateFallbackElement: <Fragment />,
  },
]

export default shellRoutes
