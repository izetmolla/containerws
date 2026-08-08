import type { DashboardTool } from "../api"

export function isGpuTool(tool: DashboardTool): boolean {
  return tool.sub_category?.toLowerCase() === "gpu"
}
