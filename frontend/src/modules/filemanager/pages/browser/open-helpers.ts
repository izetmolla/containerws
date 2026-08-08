import { toast } from "sonner"

import { useCloudShellStore } from "@/components/cloudshell/store"
import { injectTerminalData } from "@/components/cloudshell/special-keys"
import { getRequestErrorMessage } from "@/lib/network"
import { openCodeserverEditor } from "@/modules/vscode/pages/list/api"

/** POSIX single-quote escape for shell paths. */
export function shellSingleQuote(value: string): string {
  return `'${value.replace(/'/g, `'\\''`)}'`
}

/** Open Cloud Shell (if needed) and `cd` into the folder in the active terminal. */
export function openFolderInTerminal(path: string) {
  const target = path.trim()
  if (!target) return

  useCloudShellStore.getState().openShell()
  window.dispatchEvent(new Event("cloudshell:focus"))

  const payload = `cd -- ${shellSingleQuote(target)}\r`
  let attempts = 0
  const maxAttempts = 40

  const tryInject = () => {
    attempts += 1
    if (injectTerminalData(payload)) {
      window.dispatchEvent(new Event("cloudshell:focus"))
      return
    }
    if (attempts >= maxAttempts) {
      toast.error("Terminal is not ready yet — open Cloud Shell and try again")
      return
    }
    window.setTimeout(tryInject, 100)
  }

  window.setTimeout(tryInject, 50)
}

/** Open the folder in VS Code Server in a new browser tab. */
export async function openFolderInVSCode(path: string) {
  const target = path.trim()
  if (!target) return

  // Open synchronously so popup blockers allow the tab after the async start.
  const tab = window.open("about:blank", "_blank")
  try {
    const res = await openCodeserverEditor({ path: target })
    const url =
      res.connect_url ||
      res.data?.connect_url ||
      (res.data?.id ? `/codeserver/${res.data.id}/?folder=${encodeURIComponent(target)}` : "")
    if (!url) {
      throw new Error("VS Code connect URL missing")
    }
    if (tab && !tab.closed) {
      tab.location.href = url
    } else {
      window.open(url, "_blank", "noopener,noreferrer")
    }
    toast.success(res.message || "Opening VS Code Server")
  } catch (err) {
    if (tab && !tab.closed) tab.close()
    toast.error(getRequestErrorMessage(err, "Could not open VS Code Server"))
  }
}
