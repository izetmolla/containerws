import type { RouteObject } from "react-router"
import { Fragment } from "react/jsx-runtime"

const usersRoutes: RouteObject[] = [
  {
    index: true,
    lazy: () =>
      import("./pages/list").then((module) => ({
        Component: module.default,
      })),
    hydrateFallbackElement: <Fragment />,
  },
  {
    path: ":id",
    lazy: () =>
      import("./pages/single/layout").then((module) => ({
        Component: module.default,
      })),
    hydrateFallbackElement: <Fragment />,
    children: [
      {
        index: true,
        lazy: () =>
          import("./pages/single/general").then((module) => ({
            Component: module.default,
          })),
        hydrateFallbackElement: <Fragment />,
      },
      {
        path: "vnc",
        lazy: () =>
          import("./pages/single/vnc").then((module) => ({
            Component: module.default,
          })),
        hydrateFallbackElement: <Fragment />,
      },
      {
        path: "shell",
        lazy: () =>
          import("./pages/single/shell").then((module) => ({
            Component: module.default,
          })),
        hydrateFallbackElement: <Fragment />,
      },
      {
        path: "novnc",
        lazy: () =>
          import("./pages/single/novnc").then((module) => ({
            Component: module.default,
          })),
        hydrateFallbackElement: <Fragment />,
      },
      {
        path: "keys",
        lazy: () =>
          import("./pages/single/keys").then((module) => ({
            Component: module.default,
          })),
        hydrateFallbackElement: <Fragment />,
      },
      {
        path: "logs",
        lazy: () =>
          import("./pages/single/logs").then((module) => ({
            Component: module.default,
          })),
        hydrateFallbackElement: <Fragment />,
      },
      {
        path: "storage",
        lazy: () =>
          import("./pages/single/storage").then((module) => ({
            Component: module.default,
          })),
        hydrateFallbackElement: <Fragment />,
      },
    ],
  },
]

export default usersRoutes
