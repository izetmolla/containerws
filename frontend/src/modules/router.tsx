import { Fragment } from "react/jsx-runtime"
import { createBrowserRouter } from "react-router"
import authorizationRoutes from "./authorization/routes"
import softwaresRoutes from "./softwares/routes"
import cliRoutes from "./cli/routes"
import Layout from "@/components/layouts"
import NotFound404 from "@/components/errors/404"
import shellRoutes from "./shell/routes"
import vncnovncRoutes from "./vnc-novnc/routes"
import usersRoutes from "./users/routes"
import vscodeRoutes from "./vscode/routes"
import settingsRoutes from "./settings/routes"
import dockerRoutes from "./docker/routes"
import filemanagerRoutes from "./filemanager/routes"
import kubernetesRoutes from "./kubernetes/routes"
import proxymanagerRoutes from "./proxymanager/routes"

const router = createBrowserRouter([
  {
    path: "/",
    element: <Layout />,
    children: [
      {
        path: "/",
        lazy: () =>
          import("./dashboard").then((module) => ({
            Component: module.default,
          })),
        hydrateFallbackElement: <Fragment />,
      },
      { path: "/softwares", children: softwaresRoutes },
      { path: "/docker", children: dockerRoutes },
      { path: "/kubernetes", children: kubernetesRoutes },
      { path: "/proxymanager", children: proxymanagerRoutes },
      { path: "/filemanager", children: filemanagerRoutes },
      { path: "/cli", children: cliRoutes },
      { path: "/vnc-novnc", children: vncnovncRoutes },
      { path: "/users", children: usersRoutes },
      { path: "/vscode", children: vscodeRoutes },
      { path: "/settings", children: settingsRoutes },
      { path: "*", element: <NotFound404 /> },
    ],
  },
  { path: "/shell", element: <Layout cleanElement={true} />, children: shellRoutes },
  ...authorizationRoutes,
])

export default router
