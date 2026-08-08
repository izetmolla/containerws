import * as React from "react"

import {
  ThemeConfigContext,
  ThemeProviderContext,
} from "./theme-context"

export type { Theme } from "./theme-context"

export const useTheme = () => {
  const context = React.useContext(ThemeProviderContext)

  if (context === undefined) {
    throw new Error("useTheme must be used within a ThemeProvider")
  }

  return context
}

export const useThemeConfig = () => {
  const context = React.useContext(ThemeConfigContext)

  if (context === undefined) {
    throw new Error("useThemeConfig must be used within a ThemeProvider")
  }

  return context
}
