import { useOutletContext } from "react-router"

import { ResourceEventsPanel } from "../../_shared/events-panel"
import type { DeploymentOutletContext } from "./layout"

export default function DeploymentEventsPage() {
  const { namespace, name } = useOutletContext<DeploymentOutletContext>()
  return (
    <ResourceEventsPanel
      namespace={namespace}
      kind="Deployment"
      name={name}
    />
  )
}
