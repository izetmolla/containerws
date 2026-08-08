import type { RouteObject } from "react-router";
import { Fragment } from "react/jsx-runtime";
import { Navigate } from "react-router";

const kubernetesChildRoutes: RouteObject[] = [
  {
    index: true,
    element: <Navigate to="cluster" replace />,
  },
  {
    path: "cluster",
    lazy: () =>
      import("./pages/cluster").then((module) => ({
        Component: module.default,
      })),
    hydrateFallbackElement: <Fragment />,
  },
  {
    path: "nodes",
    children: [
      {
        index: true,
        lazy: () =>
          import("./pages/nodes").then((module) => ({
            Component: module.default,
          })),
        hydrateFallbackElement: <Fragment />,
      },
      {
        path: ":name",
        lazy: () =>
          import("./pages/nodes/detail").then((module) => ({
            Component: module.default,
          })),
        hydrateFallbackElement: <Fragment />,
      },
    ],
  },
  {
    path: "namespaces",
    children: [
      {
        index: true,
        lazy: () =>
          import("./pages/namespaces").then((module) => ({
            Component: module.default,
          })),
        hydrateFallbackElement: <Fragment />,
      },
      {
        path: ":name",
        lazy: () =>
          import("./pages/namespaces/detail/layout").then((module) => ({
            Component: module.default,
          })),
        hydrateFallbackElement: <Fragment />,
        children: [
          {
            index: true,
            lazy: () =>
              import("./pages/namespaces/detail/overview").then((module) => ({
                Component: module.default,
              })),
          },
          {
            path: "yaml",
            lazy: () =>
              import("./pages/namespaces/detail/yaml").then((module) => ({
                Component: module.default,
              })),
          },
        ],
      },
    ],
  },
  {
    path: "pods",
    children: [
      {
        index: true,
        lazy: () =>
          import("./pages/pods").then((module) => ({
            Component: module.default,
          })),
        hydrateFallbackElement: <Fragment />,
      },
      {
        path: ":namespace/:name",
        lazy: () =>
          import("./pages/pods/detail/layout").then((module) => ({
            Component: module.default,
          })),
        hydrateFallbackElement: <Fragment />,
        children: [
          {
            index: true,
            lazy: () =>
              import("./pages/pods/detail/overview").then((module) => ({
                Component: module.default,
              })),
          },
          {
            path: "logs",
            lazy: () =>
              import("./pages/pods/detail/logs").then((module) => ({
                Component: module.default,
              })),
          },
          {
            path: "exec",
            lazy: () =>
              import("./pages/pods/detail/exec").then((module) => ({
                Component: module.default,
              })),
          },
          {
            path: "events",
            lazy: () =>
              import("./pages/pods/detail/events").then((module) => ({
                Component: module.default,
              })),
          },
          {
            path: "yaml",
            lazy: () =>
              import("./pages/pods/detail/yaml").then((module) => ({
                Component: module.default,
              })),
          },
        ],
      },
    ],
  },
  {
    path: "deployments",
    children: [
      {
        index: true,
        lazy: () =>
          import("./pages/deployments").then((module) => ({
            Component: module.default,
          })),
        hydrateFallbackElement: <Fragment />,
      },
      {
        path: "create",
        lazy: () =>
          import("./pages/deployments/create").then((module) => ({
            Component: module.default,
          })),
        hydrateFallbackElement: <Fragment />,
      },
      {
        path: ":namespace/:name",
        lazy: () =>
          import("./pages/deployments/detail/layout").then((module) => ({
            Component: module.default,
          })),
        hydrateFallbackElement: <Fragment />,
        children: [
          {
            index: true,
            lazy: () =>
              import("./pages/deployments/detail/overview").then((module) => ({
                Component: module.default,
              })),
          },
          {
            path: "pods",
            lazy: () =>
              import("./pages/deployments/detail/pods").then((module) => ({
                Component: module.default,
              })),
          },
          {
            path: "events",
            lazy: () =>
              import("./pages/deployments/detail/events").then((module) => ({
                Component: module.default,
              })),
          },
          {
            path: "yaml",
            lazy: () =>
              import("./pages/deployments/detail/yaml").then((module) => ({
                Component: module.default,
              })),
          },
        ],
      },
    ],
  },
  {
    path: "statefulsets",
    children: [
      {
        index: true,
        lazy: () =>
          import("./pages/statefulsets").then((module) => ({
            Component: module.default,
          })),
        hydrateFallbackElement: <Fragment />,
      },
      {
        path: "create",
        lazy: () =>
          import("./pages/statefulsets/create").then((module) => ({
            Component: module.default,
          })),
        hydrateFallbackElement: <Fragment />,
      },
      {
        path: ":namespace/:name",
        lazy: () =>
          import("./pages/statefulsets/detail/layout").then((module) => ({
            Component: module.default,
          })),
        hydrateFallbackElement: <Fragment />,
        children: [
          {
            index: true,
            lazy: () =>
              import("./pages/statefulsets/detail/overview").then((module) => ({
                Component: module.default,
              })),
          },
          {
            path: "pods",
            lazy: () =>
              import("./pages/statefulsets/detail/pods").then((module) => ({
                Component: module.default,
              })),
          },
          {
            path: "yaml",
            lazy: () =>
              import("./pages/statefulsets/detail/yaml").then((module) => ({
                Component: module.default,
              })),
          },
        ],
      },
    ],
  },
  {
    path: "daemonsets",
    children: [
      {
        index: true,
        lazy: () =>
          import("./pages/daemonsets").then((module) => ({
            Component: module.default,
          })),
        hydrateFallbackElement: <Fragment />,
      },
      {
        path: "create",
        lazy: () =>
          import("./pages/daemonsets/create").then((module) => ({
            Component: module.default,
          })),
        hydrateFallbackElement: <Fragment />,
      },
      {
        path: ":namespace/:name",
        lazy: () =>
          import("./pages/daemonsets/detail/layout").then((module) => ({
            Component: module.default,
          })),
        hydrateFallbackElement: <Fragment />,
        children: [
          {
            index: true,
            lazy: () =>
              import("./pages/daemonsets/detail/overview").then((module) => ({
                Component: module.default,
              })),
          },
          {
            path: "pods",
            lazy: () =>
              import("./pages/daemonsets/detail/pods").then((module) => ({
                Component: module.default,
              })),
          },
          {
            path: "yaml",
            lazy: () =>
              import("./pages/daemonsets/detail/yaml").then((module) => ({
                Component: module.default,
              })),
          },
        ],
      },
    ],
  },
  {
    path: "jobs",
    children: [
      {
        index: true,
        lazy: () =>
          import("./pages/jobs").then((module) => ({
            Component: module.default,
          })),
        hydrateFallbackElement: <Fragment />,
      },
      {
        path: "create",
        lazy: () =>
          import("./pages/jobs/create").then((module) => ({
            Component: module.default,
          })),
        hydrateFallbackElement: <Fragment />,
      },
      {
        path: "cron/:namespace/:name",
        lazy: () =>
          import("./pages/jobs/cron-detail/layout").then((module) => ({
            Component: module.default,
          })),
        hydrateFallbackElement: <Fragment />,
        children: [
          {
            index: true,
            lazy: () =>
              import("./pages/jobs/cron-detail/overview").then((module) => ({
                Component: module.default,
              })),
          },
          {
            path: "yaml",
            lazy: () =>
              import("./pages/jobs/cron-detail/yaml").then((module) => ({
                Component: module.default,
              })),
          },
        ],
      },
      {
        path: ":namespace/:name",
        lazy: () =>
          import("./pages/jobs/detail/layout").then((module) => ({
            Component: module.default,
          })),
        hydrateFallbackElement: <Fragment />,
        children: [
          {
            index: true,
            lazy: () =>
              import("./pages/jobs/detail/overview").then((module) => ({
                Component: module.default,
              })),
          },
          {
            path: "yaml",
            lazy: () =>
              import("./pages/jobs/detail/yaml").then((module) => ({
                Component: module.default,
              })),
          },
        ],
      },
    ],
  },
  {
    path: "services",
    children: [
      {
        index: true,
        lazy: () =>
          import("./pages/services").then((module) => ({
            Component: module.default,
          })),
        hydrateFallbackElement: <Fragment />,
      },
      {
        path: ":namespace/:name",
        lazy: () =>
          import("./pages/services/detail/layout").then((module) => ({
            Component: module.default,
          })),
        hydrateFallbackElement: <Fragment />,
        children: [
          {
            index: true,
            lazy: () =>
              import("./pages/services/detail/overview").then((module) => ({
                Component: module.default,
              })),
          },
          {
            path: "yaml",
            lazy: () =>
              import("./pages/services/detail/yaml").then((module) => ({
                Component: module.default,
              })),
          },
        ],
      },
    ],
  },
  {
    path: "ingresses",
    children: [
      {
        index: true,
        lazy: () =>
          import("./pages/ingresses").then((module) => ({
            Component: module.default,
          })),
        hydrateFallbackElement: <Fragment />,
      },
      {
        path: ":namespace/:name",
        lazy: () =>
          import("./pages/ingresses/detail/layout").then((module) => ({
            Component: module.default,
          })),
        hydrateFallbackElement: <Fragment />,
        children: [
          {
            index: true,
            lazy: () =>
              import("./pages/ingresses/detail/overview").then((module) => ({
                Component: module.default,
              })),
          },
          {
            path: "yaml",
            lazy: () =>
              import("./pages/ingresses/detail/yaml").then((module) => ({
                Component: module.default,
              })),
          },
        ],
      },
    ],
  },
  {
    path: "network-policies",
    children: [
      {
        index: true,
        lazy: () =>
          import("./pages/network-policies").then((module) => ({
            Component: module.default,
          })),
        hydrateFallbackElement: <Fragment />,
      },
      {
        path: "create",
        lazy: () =>
          import("./pages/network-policies/create").then((module) => ({
            Component: module.default,
          })),
        hydrateFallbackElement: <Fragment />,
      },
      {
        path: ":namespace/:name",
        lazy: () =>
          import("./pages/network-policies/detail/layout").then((module) => ({
            Component: module.default,
          })),
        hydrateFallbackElement: <Fragment />,
        children: [
          {
            index: true,
            lazy: () =>
              import("./pages/network-policies/detail/overview").then(
                (module) => ({ Component: module.default }),
              ),
          },
          {
            path: "yaml",
            lazy: () =>
              import("./pages/network-policies/detail/yaml").then((module) => ({
                Component: module.default,
              })),
          },
        ],
      },
    ],
  },
  {
    path: "configmaps",
    children: [
      {
        index: true,
        lazy: () =>
          import("./pages/configmaps").then((module) => ({
            Component: module.default,
          })),
        hydrateFallbackElement: <Fragment />,
      },
      {
        path: ":namespace/:name",
        lazy: () =>
          import("./pages/configmaps/detail/layout").then((module) => ({
            Component: module.default,
          })),
        hydrateFallbackElement: <Fragment />,
        children: [
          {
            index: true,
            lazy: () =>
              import("./pages/configmaps/detail/overview").then((module) => ({
                Component: module.default,
              })),
          },
          {
            path: "yaml",
            lazy: () =>
              import("./pages/configmaps/detail/yaml").then((module) => ({
                Component: module.default,
              })),
          },
        ],
      },
    ],
  },
  {
    path: "secrets",
    children: [
      {
        index: true,
        lazy: () =>
          import("./pages/secrets").then((module) => ({
            Component: module.default,
          })),
        hydrateFallbackElement: <Fragment />,
      },
      {
        path: "create",
        lazy: () =>
          import("./pages/secrets/create").then((module) => ({
            Component: module.default,
          })),
        hydrateFallbackElement: <Fragment />,
      },
      {
        path: ":namespace/:name",
        lazy: () =>
          import("./pages/secrets/detail/layout").then((module) => ({
            Component: module.default,
          })),
        hydrateFallbackElement: <Fragment />,
        children: [
          {
            index: true,
            lazy: () =>
              import("./pages/secrets/detail/overview").then((module) => ({
                Component: module.default,
              })),
          },
          {
            path: "yaml",
            lazy: () =>
              import("./pages/secrets/detail/yaml").then((module) => ({
                Component: module.default,
              })),
          },
        ],
      },
    ],
  },
  {
    path: "persistentvolumeclaims",
    children: [
      {
        index: true,
        lazy: () =>
          import("./pages/persistentvolumeclaims").then((module) => ({
            Component: module.default,
          })),
        hydrateFallbackElement: <Fragment />,
      },
      {
        path: "create",
        lazy: () =>
          import("./pages/persistentvolumeclaims/create").then((module) => ({
            Component: module.default,
          })),
        hydrateFallbackElement: <Fragment />,
      },
      {
        path: ":namespace/:name",
        lazy: () =>
          import("./pages/persistentvolumeclaims/detail/layout").then(
            (module) => ({ Component: module.default }),
          ),
        hydrateFallbackElement: <Fragment />,
        children: [
          {
            index: true,
            lazy: () =>
              import("./pages/persistentvolumeclaims/detail/overview").then(
                (module) => ({ Component: module.default }),
              ),
          },
          {
            path: "yaml",
            lazy: () =>
              import("./pages/persistentvolumeclaims/detail/yaml").then(
                (module) => ({ Component: module.default }),
              ),
          },
        ],
      },
    ],
  },
  {
    path: "storageclasses",
    lazy: () =>
      import("./pages/storageclasses").then((module) => ({
        Component: module.default,
      })),
    hydrateFallbackElement: <Fragment />,
  },
  {
    path: "applications",
    children: [
      {
        index: true,
        lazy: () =>
          import("./pages/applications").then((module) => ({
            Component: module.default,
          })),
        hydrateFallbackElement: <Fragment />,
      },
      {
        path: "edit",
        lazy: () =>
          import("./pages/applications/edit").then((module) => ({
            Component: module.default,
          })),
        hydrateFallbackElement: <Fragment />,
      },
    ],
  },
  {
    path: "settings",
    lazy: () =>
      import("./pages/settings").then((module) => ({
        Component: module.default,
      })),
    hydrateFallbackElement: <Fragment />,
  },
];

const kubernetesRoutes: RouteObject[] = [
  {
    lazy: () =>
      import("./layout").then((module) => ({
        Component: module.default,
      })),
    hydrateFallbackElement: <Fragment />,
    children: kubernetesChildRoutes,
  },
];

export default kubernetesRoutes;
