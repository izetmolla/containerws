import { useNavigate } from "react-router"

import { CloudShell } from "@/components/cloudshell"

/**
 * Full-viewport Cloud Shell (same sessions as the docked panel).
 * Mounted under Layout with `cleanElement` so dashboard chrome is hidden.
 */
export default function ShellPage() {
  const navigate = useNavigate()

  return (
    <CloudShell
      variant="fullscreen"
      onRequestClose={() => {
        if (window.history.length > 1) {
          navigate(-1)
        } else {
          navigate("/")
        }
      }}
    />
  )
}
