import type { RouteObject } from "react-router"

const cliRoutes: RouteObject[] = [
  {
    index: true,
    lazy: () =>
      import("./pages/terminal").then((module) => ({
        Component: module.default,
      })),
    hydrateFallbackElement: <></>,
  },
]

export default cliRoutes
