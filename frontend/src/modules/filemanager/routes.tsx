import type { RouteObject } from "react-router"
import { Fragment } from "react/jsx-runtime"

const filemanagerRoutes: RouteObject[] = [
  {
    index: true,
    lazy: () =>
      import("./pages/browser").then((module) => ({
        Component: module.default,
      })),
    hydrateFallbackElement: <Fragment />,
  },
]

export default filemanagerRoutes
