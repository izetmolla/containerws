import { create } from "zustand"

import type { InstallTerminalLine } from "./api"
import type { InstallTerminalStatus } from "./components/install-terminal"

/** Persists the VNC install terminal across navigations within the SPA. */
type VncInstallSessionState = {
  terminalOpen: boolean
  terminalStatus: InstallTerminalStatus
  terminalLines: InstallTerminalLine[]
  jobId: string | null
  installing: boolean
  cancelling: boolean
  updateMode: boolean
  hydratedJobId: string | null
  setTerminalOpen: (open: boolean) => void
  setTerminalStatus: (status: InstallTerminalStatus) => void
  setTerminalLines: (
    lines:
      | InstallTerminalLine[]
      | ((prev: InstallTerminalLine[]) => InstallTerminalLine[])
  ) => void
  appendLine: (line: InstallTerminalLine) => void
  setJobId: (id: string | null) => void
  setInstalling: (v: boolean) => void
  setCancelling: (v: boolean) => void
  setUpdateMode: (v: boolean) => void
  setHydratedJobId: (id: string | null) => void
  resetTerminal: () => void
}

export const useVncInstallSession = create<VncInstallSessionState>((set) => ({
  terminalOpen: false,
  terminalStatus: "idle",
  terminalLines: [],
  jobId: null,
  installing: false,
  cancelling: false,
  updateMode: false,
  hydratedJobId: null,
  setTerminalOpen: (terminalOpen) => set({ terminalOpen }),
  setTerminalStatus: (terminalStatus) => set({ terminalStatus }),
  setTerminalLines: (lines) =>
    set((s) => ({
      terminalLines: typeof lines === "function" ? lines(s.terminalLines) : lines,
    })),
  appendLine: (line) =>
    set((s) => ({ terminalLines: [...s.terminalLines, line] })),
  setJobId: (jobId) => set({ jobId }),
  setInstalling: (installing) => set({ installing }),
  setCancelling: (cancelling) => set({ cancelling }),
  setUpdateMode: (updateMode) => set({ updateMode }),
  setHydratedJobId: (hydratedJobId) => set({ hydratedJobId }),
  resetTerminal: () =>
    set({
      terminalOpen: false,
      terminalStatus: "idle",
      terminalLines: [],
      jobId: null,
      installing: false,
      cancelling: false,
      hydratedJobId: null,
    }),
}))
