/** Raw PTY byte sequences for Cloud Shell special keys (mobile accessory bar). */

export const CLOUDSHELL_INJECT_EVENT = "cloudshell:inject"
export const CLOUDSHELL_TOGGLE_KEYS_EVENT = "cloudshell:toggle-keys"

export type CloudShellInjectDetail = {
  data: string
  /** Set synchronously by the active LiveTerminal when bytes were sent. */
  delivered?: boolean
}

/** Inject raw PTY bytes into the active Cloud Shell. Returns true if delivered. */
export function injectTerminalData(data: string): boolean {
  if (!data) return false
  const detail: CloudShellInjectDetail = { data }
  window.dispatchEvent(
    new CustomEvent<CloudShellInjectDetail>(CLOUDSHELL_INJECT_EVENT, {
      detail,
    }),
  )
  return Boolean(detail.delivered)
}

export function toggleSpecialKeysBar() {
  window.dispatchEvent(new Event(CLOUDSHELL_TOGGLE_KEYS_EVENT))
}

/** Ctrl+letter → ASCII control byte (a/A → 0x01 … z/Z → 0x1a). */
export function ctrlKey(letter: string): string {
  const ch = letter.trim().toLowerCase()
  if (ch.length !== 1 || ch < "a" || ch > "z") return ""
  return String.fromCharCode(ch.charCodeAt(0) - 96)
}

export const ESC = "\x1b"
export const TAB = "\t"
export const ENTER = "\r"
export const BACKSPACE = "\x7f"

export const ARROW = {
  up: "\x1b[A",
  down: "\x1b[B",
  right: "\x1b[C",
  left: "\x1b[D",
} as const

export const NAV = {
  home: "\x1b[H",
  end: "\x1b[F",
  pageUp: "\x1b[5~",
  pageDown: "\x1b[6~",
} as const

export type QuickCombo = {
  id: string
  label: string
  title: string
  data: string
}

/** One-tap combos most needed when the iOS keyboard has no Ctrl. */
export const QUICK_COMBOS: QuickCombo[] = [
  { id: "c", label: "Ctrl+C", title: "Interrupt (SIGINT)", data: ctrlKey("c") },
  { id: "d", label: "Ctrl+D", title: "EOF / logout", data: ctrlKey("d") },
  { id: "z", label: "Ctrl+Z", title: "Suspend (SIGTSTP)", data: ctrlKey("z") },
  { id: "l", label: "Ctrl+L", title: "Clear screen", data: ctrlKey("l") },
  { id: "a", label: "Ctrl+A", title: "Line start", data: ctrlKey("a") },
  { id: "e", label: "Ctrl+E", title: "Line end", data: ctrlKey("e") },
  { id: "u", label: "Ctrl+U", title: "Kill line", data: ctrlKey("u") },
  { id: "w", label: "Ctrl+W", title: "Delete word", data: ctrlKey("w") },
  { id: "r", label: "Ctrl+R", title: "Reverse search", data: ctrlKey("r") },
]

export const SYMBOL_KEYS: { label: string; data: string; title?: string }[] = [
  { label: "|", data: "|" },
  { label: "/", data: "/" },
  { label: "~", data: "~" },
  { label: "-", data: "-" },
  { label: "_", data: "_" },
  { label: ".", data: "." },
  { label: "$", data: "$" },
  { label: "&", data: "&" },
]
