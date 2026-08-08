import { Label } from "@/components/ui/label"
import { useThemeConfig } from "../providers/use-theme"
import {
  ToggleGroup,
  ToggleGroupItem,
} from "@/components/ui/toggle-group"
import { BanIcon } from "lucide-react"
import { THEME_SCALES, type ThemeScale } from "../lib/themes"

export function ThemeScaleSelector() {
  const { theme, setTheme } = useThemeConfig()

  return (
    <div className="flex flex-col gap-3">
      <Label htmlFor="roundedCorner">Scale:</Label>
      <div>
        <ToggleGroup
          className="w-full"
          value={theme.scale}
          type="single"
          onValueChange={(value) => {
            if (!(THEME_SCALES as readonly string[]).includes(value)) return
            setTheme({ ...theme, scale: value as ThemeScale })
          }}
        >
          <ToggleGroupItem variant="outline" className="grow" value="none">
            <BanIcon />
          </ToggleGroupItem>
          <ToggleGroupItem variant="outline" className="grow" value="sm">
            XS
          </ToggleGroupItem>
          <ToggleGroupItem variant="outline" className="grow" value="lg">
            LG
          </ToggleGroupItem>
        </ToggleGroup>
      </div>
    </div>
  )
}
