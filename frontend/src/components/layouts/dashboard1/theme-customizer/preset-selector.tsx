"use client"

import { DEFAULT_THEME, THEMES, type ThemePreset } from "../lib/themes"
import { useThemeConfig } from "../providers/use-theme"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { Label } from "@/components/ui/label"

export function PresetSelector() {
  const { theme, setTheme } = useThemeConfig()

  const handlePreset = (value: string) => {
    const preset = THEMES.find((item) => item.value === value)?.value
    if (!preset) return
    setTheme({ ...theme, ...DEFAULT_THEME, preset: preset as ThemePreset })
  }

  return (
    <div className="flex flex-col gap-3">
      <Label>Theme preset:</Label>
      <Select
        value={theme.preset}
        onValueChange={(value) => handlePreset(value)}
      >
        <SelectTrigger className="w-full">
          <SelectValue placeholder="Select a theme" />
        </SelectTrigger>
        <SelectContent align="end">
          {THEMES.map((theme) => (
            <SelectItem key={theme.name} value={theme.value}>
              <div className="flex shrink-0 gap-1">
                {theme.colors.map((color, key) => (
                  <span
                    key={key}
                    className="size-2 rounded-full"
                    style={{ backgroundColor: color }}
                  ></span>
                ))}
              </div>
              {theme.name}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </div>
  )
}
