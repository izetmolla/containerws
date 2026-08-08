import type { RouteObject } from "react-router"
import { Fragment } from "react/jsx-runtime"
import mcpSettingsRoutes from "./pages/mcp/routes"
import environmentsSettingsRoutes from "./pages/environments/routes"
import optionsSettingsRoutes from "./pages/options/routes"
import updateSettingsRoutes from "./pages/update/routes"

const settingsRoutes: RouteObject[] = [
  {
    index: true,
    lazy: () =>
      import("./pages/general").then((module) => ({
        Component: module.default,
      })),
    hydrateFallbackElement: <Fragment />,
  },
  {
    path: "environments",
    children: environmentsSettingsRoutes,
  },
  {
    path: "options",
    children: optionsSettingsRoutes,
  },
  {
    path: "mcp",
    children: mcpSettingsRoutes,
  },
  {
    path: "update",
    children: updateSettingsRoutes,
  },
]

export default settingsRoutes
