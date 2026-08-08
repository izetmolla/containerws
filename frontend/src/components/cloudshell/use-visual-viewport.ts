import { useEffect, useState } from "react"

export type VisualViewportState = {
  /** Visible height (shrinks when soft keyboard opens). */
  height: number
  width: number
  /** Offset of the visual viewport from the layout viewport top (iOS pan). */
  offsetTop: number
  /** Space below the visual viewport (≈ keyboard height on many browsers). */
  bottomInset: number
  /** True when the visible viewport is meaningfully shorter than the layout viewport. */
  keyboardOpen: boolean
  /** Coarse pointer / narrow width — treat as phone-like. */
  isCompact: boolean
}

function readVisualViewport(): VisualViewportState {
  if (typeof window === "undefined") {
    return {
      height: 800,
      width: 400,
      offsetTop: 0,
      bottomInset: 0,
      keyboardOpen: false,
      isCompact: true,
    }
  }

  const vp = window.visualViewport
  const innerH = window.innerHeight || 0
  const innerW = window.innerWidth || 0
  const height = vp?.height ?? innerH
  const width = vp?.width ?? innerW
  const offsetTop = vp?.offsetTop ?? 0
  const bottomInset = Math.max(0, Math.round(innerH - offsetTop - height))
  // Soft keyboard typically eats >120px; ignore tiny chrome/UI chrome changes.
  const keyboardOpen = bottomInset > 120 || height < innerH * 0.72
  const isCompact =
    innerW < 768 ||
    (typeof window.matchMedia === "function" &&
      window.matchMedia("(hover: none) and (pointer: coarse)").matches)

  return {
    height: Math.round(height),
    width: Math.round(width),
    offsetTop: Math.round(offsetTop),
    bottomInset,
    keyboardOpen,
    isCompact,
  }
}

/** Tracks visualViewport so UIs can stay above the mobile soft keyboard. */
export function useVisualViewport(): VisualViewportState {
  const [state, setState] = useState<VisualViewportState>(() =>
    readVisualViewport()
  )

  useEffect(() => {
    let frame = 0
    const update = () => {
      cancelAnimationFrame(frame)
      frame = requestAnimationFrame(() => {
        setState(readVisualViewport())
      })
    }

    update()
    const vp = window.visualViewport
    window.addEventListener("resize", update)
    window.addEventListener("orientationchange", update)
    vp?.addEventListener("resize", update)
    vp?.addEventListener("scroll", update)

    return () => {
      cancelAnimationFrame(frame)
      window.removeEventListener("resize", update)
      window.removeEventListener("orientationchange", update)
      vp?.removeEventListener("resize", update)
      vp?.removeEventListener("scroll", update)
    }
  }, [])

  return state
}
