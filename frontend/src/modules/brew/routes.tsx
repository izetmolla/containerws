import type { RouteObject } from "react-router"

const brewRoutes: RouteObject[] = [
  {
    index: true,
    lazy: () =>
      import("./pages/discover").then((m) => ({ Component: m.default })),
  },
  {
    path: "installed",
    lazy: () =>
      import("./pages/installed").then((m) => ({ Component: m.default })),
  },
  {
    path: ":name",
    lazy: () =>
      import("./pages/detail").then((m) => ({ Component: m.default })),
  },
]

export default brewRoutes
