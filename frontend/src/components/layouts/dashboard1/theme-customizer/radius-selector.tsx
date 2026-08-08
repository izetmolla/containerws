import { Label } from "@/components/ui/label"
import { useThemeConfig } from "../providers/use-theme"
import {
  ToggleGroup,
  ToggleGroupItem,
} from "@/components/ui/toggle-group"
import { BanIcon } from "lucide-react"
import { THEME_RADII, type ThemeRadius } from "../lib/themes"

export function ThemeRadiusSelector() {
  const { theme, setTheme } = useThemeConfig()

  return (
    <div className="flex flex-col gap-3">
      <Label htmlFor="roundedCorner">Radius:</Label>
      <ToggleGroup
        className="w-full"
        value={theme.radius}
        type="single"
        onValueChange={(value) => {
          if (!(THEME_RADII as readonly string[]).includes(value)) return
          setTheme({ ...theme, radius: value as ThemeRadius })
        }}
      >
        <ToggleGroupItem variant="outline" className="grow" value="none">
          <BanIcon />
        </ToggleGroupItem>
        <ToggleGroupItem variant="outline" className="grow" value="sm">
          SM
        </ToggleGroupItem>
        <ToggleGroupItem variant="outline" className="grow" value="md">
          MD
        </ToggleGroupItem>
        <ToggleGroupItem variant="outline" className="grow" value="lg">
          LG
        </ToggleGroupItem>
        <ToggleGroupItem variant="outline" className="grow" value="xl">
          XL
        </ToggleGroupItem>
      </ToggleGroup>
    </div>
  )
}
