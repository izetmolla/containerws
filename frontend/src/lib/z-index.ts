/** Shared overlay stacking order for modals, confirmations, and in-dialog popovers. */
export const Z_INDEX = {
  dialog: 50,
  dialogElevated: 70,
  dialogNested: 80,
  dialogConfirm: 100,
  dialogPopover: 110,
} as const

export type DialogLayer = "default" | "elevated" | "nested"

export const DIALOG_LAYER_CLASSES: Record<
  DialogLayer,
  { overlay: string; content: string }
> = {
  default: { overlay: "z-[50]", content: "z-[50]" },
  elevated: { overlay: "z-[70]", content: "z-[70]" },
  nested: { overlay: "z-[80]", content: "z-[80]" },
}

export const ALERT_DIALOG_LAYER_CLASSES = {
  overlay: "z-[100]",
  content: "z-[100]",
} as const

export function dialogLayerClass(layer: DialogLayer = "default") {
  return DIALOG_LAYER_CLASSES[layer]
}
