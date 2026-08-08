import { createContext } from "react"

import { type Software, type SoftwareStatus } from "./software-data"

export type SoftwareStoreContext = {
  items: Software[]
  getById: (id: string) => Software | undefined
  install: (id: string, version?: string) => Promise<void>
  update: (id: string, version?: string) => Promise<void>
  uninstall: (id: string) => Promise<void>
  updateAll: () => Promise<void>
}

export const SoftwareCtx = createContext<SoftwareStoreContext | null>(null)

export type { SoftwareStatus }
