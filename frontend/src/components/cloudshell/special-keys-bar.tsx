"use client"

import { useState } from "react"
import {
  ChevronDown,
  ChevronLeft,
  ChevronRight,
  ChevronUp,
  Delete,
} from "lucide-react"

import { cn } from "@/lib/utils"

import {
  ARROW,
  BACKSPACE,
  ENTER,
  ESC,
  NAV,
  QUICK_COMBOS,
  SYMBOL_KEYS,
  TAB,
  ctrlKey,
  injectTerminalData,
} from "./special-keys"

type SpecialKeysBarProps = {
  open: boolean
  onClose?: () => void
  className?: string
}

function KeyBtn({
  label,
  title,
  active,
  wide,
  onClick,
  children,
}: {
  label?: string
  title?: string
  active?: boolean
  wide?: boolean
  onClick: () => void
  children?: React.ReactNode
}) {
  return (
    <button
      type="button"
      title={title || label}
      aria-label={title || label}
      aria-pressed={active}
      onClick={(e) => {
        e.preventDefault()
        e.stopPropagation()
        onClick()
        // Keep soft keyboard / xterm focused after tapping the bar.
        window.dispatchEvent(new Event("cloudshell:focus"))
      }}
      onPointerDown={(e) => {
        // Prevent the bar from stealing focus from the terminal helper textarea.
        e.preventDefault()
      }}
      className={cn(
        "inline-flex h-9 min-w-9 shrink-0 items-center justify-center rounded-md border px-2",
        "text-[12px] font-medium tabular-nums transition-colors select-none",
        "border-white/15 bg-[#2b2c2f] text-zinc-200",
        "active:bg-sky-600 active:text-white",
        active && "border-sky-500/80 bg-sky-600/90 text-white",
        wide && "min-w-14 px-2.5"
      )}
    >
      {children ?? label}
    </button>
  )
}

/**
 * Mobile accessory bar inspired by Termius / Blink / JuiceSSH / vmux:
 * sticky Ctrl + one-tap combos, Esc/Tab, arrows, and path symbols.
 */
export function SpecialKeysBar({ open, onClose, className }: SpecialKeysBarProps) {
  const [ctrlArmed, setCtrlArmed] = useState(false)
  const [ctrlLocked, setCtrlLocked] = useState(false)
  const [prevOpen, setPrevOpen] = useState(open)

  if (open !== prevOpen) {
    setPrevOpen(open)
    if (!open) {
      setCtrlArmed(false)
      setCtrlLocked(false)
    }
  }

  if (!open) return null

  const send = (data: string) => {
    if (!data) return
    injectTerminalData(data)
  }

  const sendLetter = (letter: string) => {
    if (ctrlArmed || ctrlLocked) {
      send(ctrlKey(letter))
      if (!ctrlLocked) setCtrlArmed(false)
      return
    }
    send(letter)
  }

  const toggleCtrl = () => {
    if (ctrlLocked) {
      setCtrlLocked(false)
      setCtrlArmed(false)
      return
    }
    if (ctrlArmed) {
      // Second tap locks Ctrl (Blink/Moshi-style chain).
      setCtrlLocked(true)
      setCtrlArmed(true)
      return
    }
    setCtrlArmed(true)
  }

  return (
    <div
      className={cn(
        "shrink-0 border-t border-white/10 bg-[#1a1b1e] text-zinc-200",
        "pb-[max(0.35rem,env(safe-area-inset-bottom,0px))]",
        className
      )}
      role="toolbar"
      aria-label="Terminal special keys"
    >
      <div className="flex items-center justify-between gap-2 px-2 pt-1.5 pb-1">
        <p className="text-[10px] font-medium tracking-wide text-zinc-500 uppercase">
          Special keys
          {ctrlLocked ? (
            <span className="ml-1.5 text-sky-400 normal-case">Ctrl locked</span>
          ) : ctrlArmed ? (
            <span className="ml-1.5 text-sky-400 normal-case">Ctrl…</span>
          ) : null}
        </p>
        {onClose ? (
          <button
            type="button"
            className="rounded px-1.5 py-0.5 text-[11px] text-zinc-400 hover:bg-white/10 hover:text-white"
            onClick={onClose}
            onPointerDown={(e) => e.preventDefault()}
          >
            Hide
          </button>
        ) : null}
      </div>

      <div className="flex flex-col gap-1.5 px-2 pb-2">
        {/* Modifiers + navigation */}
        <div className="flex gap-1.5 overflow-x-auto [-ms-overflow-style:none] [scrollbar-width:none] [&::-webkit-scrollbar]:hidden">
          <KeyBtn
            label="Ctrl"
            title="Arm Ctrl (tap twice to lock)"
            active={ctrlArmed || ctrlLocked}
            wide
            onClick={toggleCtrl}
          />
          <KeyBtn label="Esc" title="Escape" onClick={() => send(ESC)} />
          <KeyBtn label="Tab" title="Tab" onClick={() => send(TAB)} />
          <KeyBtn label="↵" title="Enter" onClick={() => send(ENTER)} />
          <KeyBtn
            label="⌫"
            title="Backspace"
            onClick={() => send(BACKSPACE)}
          >
            <Delete className="size-3.5" />
          </KeyBtn>
          <div className="mx-0.5 w-px shrink-0 self-stretch bg-white/10" />
          <KeyBtn
            title="Up"
            onClick={() => send(ARROW.up)}
          >
            <ChevronUp className="size-4" />
          </KeyBtn>
          <KeyBtn title="Down" onClick={() => send(ARROW.down)}>
            <ChevronDown className="size-4" />
          </KeyBtn>
          <KeyBtn title="Left" onClick={() => send(ARROW.left)}>
            <ChevronLeft className="size-4" />
          </KeyBtn>
          <KeyBtn title="Right" onClick={() => send(ARROW.right)}>
            <ChevronRight className="size-4" />
          </KeyBtn>
          <KeyBtn label="Home" title="Home" onClick={() => send(NAV.home)} />
          <KeyBtn label="End" title="End" onClick={() => send(NAV.end)} />
        </div>

        {/* One-tap Ctrl combos — primary need on iPhone */}
        <div className="flex gap-1.5 overflow-x-auto [-ms-overflow-style:none] [scrollbar-width:none] [&::-webkit-scrollbar]:hidden">
          {QUICK_COMBOS.map((combo) => (
            <KeyBtn
              key={combo.id}
              label={combo.label}
              title={combo.title}
              wide
              onClick={() => send(combo.data)}
            />
          ))}
        </div>

        {/* Letters when Ctrl is armed — tap C after Ctrl for Ctrl+C */}
        {ctrlArmed || ctrlLocked ? (
          <div className="flex gap-1 overflow-x-auto [-ms-overflow-style:none] [scrollbar-width:none] [&::-webkit-scrollbar]:hidden">
            {"abcdefghijklmnopqrstuvwxyz".split("").map((letter) => (
              <KeyBtn
                key={letter}
                label={letter.toUpperCase()}
                title={`Ctrl+${letter.toUpperCase()}`}
                onClick={() => sendLetter(letter)}
              />
            ))}
          </div>
        ) : (
          <div className="flex gap-1.5 overflow-x-auto [-ms-overflow-style:none] [scrollbar-width:none] [&::-webkit-scrollbar]:hidden">
            {SYMBOL_KEYS.map((sym) => (
              <KeyBtn
                key={sym.label}
                label={sym.label}
                title={sym.title || sym.label}
                onClick={() => send(sym.data)}
              />
            ))}
          </div>
        )}
      </div>
    </div>
  )
}
