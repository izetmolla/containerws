import { Label } from "@/components/ui/label"
import {
  ToggleGroup,
  ToggleGroupItem,
} from "@/components/ui/toggle-group"
import { useTheme, type Theme } from "../providers/use-theme"

export function ColorModeSelector() {
  const { theme, setTheme } = useTheme()

  return (
    <div className="flex flex-col gap-3">
      <Label htmlFor="roundedCorner">Color mode:</Label>
      <ToggleGroup
        className="w-full"
        value={theme}
        type="single"
        onValueChange={(value: Theme) => setTheme(value)}
      >
        <ToggleGroupItem variant="outline" className="grow" value="light">
          Light
        </ToggleGroupItem>
        <ToggleGroupItem variant="outline" className="grow" value="dark">
          Dark
        </ToggleGroupItem>
      </ToggleGroup>
    </div>
  )
}
