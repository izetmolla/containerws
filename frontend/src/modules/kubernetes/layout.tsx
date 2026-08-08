import { Outlet } from "react-router"

/** Kubernetes module shell — full content width like Docker. */
export default function KubernetesLayout() {
  return (
    <div data-content-width="full" className="flex w-full min-w-0 flex-col">
      <Outlet />
    </div>
  )
}
