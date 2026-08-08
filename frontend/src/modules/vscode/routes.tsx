import type { RouteObject } from "react-router"
import { Fragment } from "react/jsx-runtime"

const vscodeRoutes: RouteObject[] = [
  {
    index: true,
    lazy: () =>
      import("./pages/list").then((module) => ({
        Component: module.default,
      })),
    hydrateFallbackElement: <Fragment />,
  },
]

export default vscodeRoutes
