import { Outlet } from "react-router"

export default function ProxyManagerLayout() {
  return (
    <div data-content-width="full" className="flex w-full min-w-0 flex-col">
      <Outlet />
    </div>
  )
}
