import * as React from "react"

import { type ThemeType } from "../lib/themes"

export type Theme = "dark" | "light" | "system"

export type ThemeProviderState = {
  theme: Theme
  setTheme: (theme: Theme) => void
}

export type ThemeConfigContextType = {
  theme: ThemeType
  setTheme: (theme: ThemeType) => void
}

export const ThemeProviderContext = React.createContext<
  ThemeProviderState | undefined
>(undefined)

export const ThemeConfigContext = React.createContext<
  ThemeConfigContextType | undefined
>(undefined)
