import { useContext } from "react"

import { SoftwareCtx } from "./software-context"

export function useSoftware() {
  const ctx = useContext(SoftwareCtx)
  if (!ctx) throw new Error("useSoftware must be used within SoftwareProvider")
  return ctx
}
