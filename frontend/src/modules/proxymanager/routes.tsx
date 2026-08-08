import type { RouteObject } from "react-router"
import { Fragment } from "react/jsx-runtime"
import { Navigate } from "react-router"
import ProxyManagerLayout from "./layout"

const proxymanagerRoutes: RouteObject[] = [
  {
    element: <ProxyManagerLayout />,
    children: [
      {
        index: true,
        element: <Navigate to="overview" replace />,
      },
      {
        path: "overview",
        lazy: () =>
          import("./pages/overview").then((module) => ({
            Component: module.default,
          })),
        hydrateFallbackElement: <Fragment />,
      },
      {
        path: "hosts",
        lazy: () =>
          import("./pages/hosts").then((module) => ({
            Component: module.default,
          })),
        hydrateFallbackElement: <Fragment />,
      },
      {
        path: "ssl",
        lazy: () =>
          import("./pages/ssl").then((module) => ({
            Component: module.default,
          })),
        hydrateFallbackElement: <Fragment />,
      },
      {
        path: "status",
        lazy: () =>
          import("./pages/status").then((module) => ({
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
    ],
  },
]

export default proxymanagerRoutes
