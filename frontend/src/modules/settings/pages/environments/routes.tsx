import type { RouteObject } from "react-router"
import { Fragment } from "react/jsx-runtime"

const environmentsSettingsRoutes: RouteObject[] = [
  {
    index: true,
    lazy: () =>
      import("./index").then((module) => ({
        Component: module.default,
      })),
    hydrateFallbackElement: <Fragment />,
  },
]

export default environmentsSettingsRoutes
