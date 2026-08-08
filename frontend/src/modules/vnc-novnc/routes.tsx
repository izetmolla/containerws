import type { RouteObject } from "react-router"
import { Fragment } from "react/jsx-runtime"

const vncnovncRoutes: RouteObject[] = [
  {
    index: true,
    lazy: () =>
      import("./pages/settings").then((module) => ({
        Component: module.default,
      })),
    hydrateFallbackElement: <Fragment />,
  },
  {
    path: "logs",
    lazy: () =>
      import("./pages/logs").then((module) => ({
        Component: module.default,
      })),
    hydrateFallbackElement: <Fragment />,
  },
]

export default vncnovncRoutes
