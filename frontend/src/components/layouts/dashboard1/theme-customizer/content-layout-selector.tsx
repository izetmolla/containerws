"use client"

import { Label } from "@/components/ui/label"
import { useThemeConfig } from "../providers/use-theme"
import {
  ToggleGroup,
  ToggleGroupItem,
} from "@/components/ui/toggle-group"
import type { ThemeContentLayout } from "../lib/themes"

export function ContentLayoutSelector() {
  const { theme, setTheme } = useThemeConfig()

  return (
    <div className="hidden flex-col gap-3 lg:flex">
      <Label>Content layout</Label>
      <ToggleGroup
        className="w-full"
        value={theme.contentLayout}
        type="single"
        onValueChange={(value) => {
          if (value !== "full" && value !== "centered") return
          setTheme({
            ...theme,
            contentLayout: value as ThemeContentLayout,
          })
        }}
      >
        <ToggleGroupItem variant="outline" className="grow" value="full">
          Full
        </ToggleGroupItem>
        <ToggleGroupItem variant="outline" className="grow" value="centered">
          Centered
        </ToggleGroupItem>
      </ToggleGroup>
    </div>
  )
}
