import { useOutletContext } from "react-router"

import { ResourceEventsPanel } from "../../_shared/events-panel"
import type { PodOutletContext } from "./layout"

export default function PodEventsPage() {
  const { namespace, name } = useOutletContext<PodOutletContext>()
  return (
    <ResourceEventsPanel namespace={namespace} kind="Pod" name={name} />
  )
}
