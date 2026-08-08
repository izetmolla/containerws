import { Outlet } from "react-router"

/**
 * Docker module shell — always use the full content width (tables, editors).
 * Marks the subtree so the dashboard layout skips the centered max-width container.
 */
export default function DockerLayout() {
  return (
    <div data-content-width="full" className="flex w-full min-w-0 flex-col">
      <Outlet />
    </div>
  )
}
