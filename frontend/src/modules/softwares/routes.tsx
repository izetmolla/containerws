import type { RouteObject } from "react-router"

const softwaresRoutes: RouteObject[] = [
  {
    index: true,
    lazy: () =>
      import("./pages/list").then((module) => ({
        Component: module.default,
      })),
    hydrateFallbackElement: <></>,
  },
  {
    path: "installing",
    lazy: () =>
      import("./pages/installing").then((module) => ({
        Component: module.default,
      })),
    hydrateFallbackElement: <></>,
  },
  {
    path: "installed",
    lazy: () =>
      import("./pages/installed").then((module) => ({
        Component: module.default,
      })),
    hydrateFallbackElement: <></>,
  },
  {
    path: "remotepkg",
    lazy: () =>
      import("./pages/remotepkg").then((module) => ({
        Component: module.default,
      })),
    hydrateFallbackElement: <></>,
  },
  {
    path: ":id/package",
    lazy: () =>
      import("./pages/package").then((module) => ({
        Component: module.default,
      })),
    hydrateFallbackElement: <></>,
  },
  {
    path: ":id",
    lazy: () =>
      import("./pages/single").then((module) => ({
        Component: module.default,
      })),
    hydrateFallbackElement: <></>,
  },
]

export default softwaresRoutes
