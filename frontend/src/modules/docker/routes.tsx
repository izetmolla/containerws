import type { RouteObject } from "react-router"
import { Fragment } from "react/jsx-runtime"
import { Navigate } from "react-router"

const dockerChildRoutes: RouteObject[] = [
  {
    index: true,
    element: <Navigate to="containers" replace />,
  },
  {
    path: "containers",
    children: [
      {
        index: true,
        lazy: () =>
          import("./pages/containers/list").then((module) => ({
            Component: module.default,
          })),
        hydrateFallbackElement: <Fragment />,
      },
      {
        path: "edit",
        lazy: () =>
          import("./pages/containers/edit").then((module) => ({
            Component: module.default,
          })),
        hydrateFallbackElement: <Fragment />,
      },
      {
        path: ":id",
        lazy: () =>
          import("./pages/containers/single/layout").then((module) => ({
            Component: module.default,
          })),
        hydrateFallbackElement: <Fragment />,
        children: [
          {
            index: true,
            lazy: () =>
              import("./pages/containers/single/overview").then((module) => ({
                Component: module.default,
              })),
            hydrateFallbackElement: <Fragment />,
          },
          {
            path: "logs",
            lazy: () =>
              import("./pages/containers/single/logs").then((module) => ({
                Component: module.default,
              })),
            hydrateFallbackElement: <Fragment />,
          },
          {
            path: "inspect",
            lazy: () =>
              import("./pages/containers/single/inspect").then((module) => ({
                Component: module.default,
              })),
            hydrateFallbackElement: <Fragment />,
          },
          {
            path: "stats",
            lazy: () =>
              import("./pages/containers/single/stats").then((module) => ({
                Component: module.default,
              })),
            hydrateFallbackElement: <Fragment />,
          },
          {
            path: "console",
            lazy: () =>
              import("./pages/containers/single/console").then((module) => ({
                Component: module.default,
              })),
            hydrateFallbackElement: <Fragment />,
          },
        ],
      },
    ],
  },
  {
    path: "images",
    lazy: () =>
      import("./pages/images").then((module) => ({
        Component: module.default,
      })),
    hydrateFallbackElement: <Fragment />,
  },
  {
    path: "networks",
    children: [
      {
        index: true,
        lazy: () =>
          import("./pages/networks").then((module) => ({
            Component: module.default,
          })),
        hydrateFallbackElement: <Fragment />,
      },
      {
        path: "edit",
        lazy: () =>
          import("./pages/networks/edit").then((module) => ({
            Component: module.default,
          })),
        hydrateFallbackElement: <Fragment />,
      },
    ],
  },
  {
    path: "volumes",
    lazy: () =>
      import("./pages/volumes").then((module) => ({
        Component: module.default,
      })),
    hydrateFallbackElement: <Fragment />,
  },
  {
    path: "stacks",
    children: [
      {
        index: true,
        lazy: () =>
          import("./pages/stacks").then((module) => ({
            Component: module.default,
          })),
        hydrateFallbackElement: <Fragment />,
      },
      {
        path: "edit",
        lazy: () =>
          import("./pages/stacks/edit").then((module) => ({
            Component: module.default,
          })),
        hydrateFallbackElement: <Fragment />,
      },
    ],
  },
  {
    path: "templates",
    lazy: () =>
      import("./pages/templates").then((module) => ({
        Component: module.default,
      })),
    hydrateFallbackElement: <Fragment />,
  },
  {
    path: "environments",
    lazy: () =>
      import("./pages/environments").then((module) => ({
        Component: module.default,
      })),
    hydrateFallbackElement: <Fragment />,
  },
]

const dockerRoutes: RouteObject[] = [
  {
    lazy: () =>
      import("./layout").then((module) => ({
        Component: module.default,
      })),
    hydrateFallbackElement: <Fragment />,
    children: dockerChildRoutes,
  },
]

export default dockerRoutes
