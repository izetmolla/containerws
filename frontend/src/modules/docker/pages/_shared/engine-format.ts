export function stateBadgeClass(state: string) {
  const s = state.toLowerCase()
  if (s === "running") {
    return "bg-emerald-500/15 text-emerald-700 dark:text-emerald-300"
  }
  if (s === "paused" || s === "restarting" || s === "partial" || s === "pending") {
    return "bg-amber-500/15 text-amber-800 dark:text-amber-300"
  }
  if (s === "error" || s === "dead") {
    return "bg-destructive/15 text-destructive"
  }
  if (s === "exited" || s === "created" || s === "stopped") {
    return "bg-muted text-muted-foreground"
  }
  return "bg-muted text-muted-foreground"
}

export function formatBytes(n?: number) {
  if (n == null || Number.isNaN(n)) return "—"
  const units = ["B", "KB", "MB", "GB", "TB"]
  let v = n
  let i = 0
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024
    i += 1
  }
  return `${v.toFixed(i === 0 ? 0 : 1)} ${units[i]}`
}

export function formatPorts(
  ports?: { private_port: number; public_port?: number; type?: string; ip?: string }[]
) {
  if (!ports?.length) return "—"
  return ports
    .map((p) =>
      p.public_port
        ? `${p.public_port}→${p.private_port}/${p.type || "tcp"}`
        : `${p.private_port}/${p.type || "tcp"}`
    )
    .join(", ")
}
