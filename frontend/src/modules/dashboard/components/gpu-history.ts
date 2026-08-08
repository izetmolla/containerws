export type GpuHistorySample = {
  t: number
  util: number
  mem: number
  memUtil: number
  clock: number
  clockMax: number
}

const MAX_SAMPLES = 720

export function gpuHistoryKey(gpu: {
  pci_slot?: string
  drm_card?: string
  uuid?: string
  index?: number
}): string {
  return gpu.pci_slot || gpu.drm_card || gpu.uuid || `gpu-${gpu.index ?? 0}`
}

export function appendGpuSample(
  prev: Record<string, GpuHistorySample[]>,
  key: string,
  sample: GpuHistorySample
): Record<string, GpuHistorySample[]> {
  const list = [...(prev[key] ?? []), sample]
  return {
    ...prev,
    [key]: list.length > MAX_SAMPLES ? list.slice(-MAX_SAMPLES) : list,
  }
}
