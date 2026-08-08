/**
 * Legacy /cli page — Cloud Shell UI lives at /shell and in the dock.
 * This route keeps backwards-compatible deep links working.
 */
import { Navigate } from "react-router"

export default function CliTerminalRedirect() {
  return <Navigate to="/shell" replace />
}
