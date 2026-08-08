import { Palette } from "lucide-react"
import { Button } from "@/components/ui/button"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { PresetSelector } from "./preset-selector"
import { ThemeScaleSelector } from "./scale-selector"
import { ColorModeSelector } from "./color-mode-selector"
import { ContentLayoutSelector } from "./content-layout-selector"
import { ThemeRadiusSelector } from "./radius-selector"
import { ResetThemeButton } from "./reset-theme"
import { useIsMobile } from "@/hooks/use-mobile"
import { SidebarModeSelector } from "./sidebar-mode-selector"

export function ThemeCustomizerPanel() {
  const isMobile = useIsMobile()

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button size="icon-sm" variant="ghost">
          <Palette />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent
        className="me-4 w-80 p-4 shadow-xl lg:me-0"
        align={isMobile ? "center" : "end"}
      >
        <div className="grid space-y-4">
          <PresetSelector />
          <ThemeScaleSelector />
          <ThemeRadiusSelector />
          <ColorModeSelector />
          <ContentLayoutSelector />
          <SidebarModeSelector />
        </div>
        <ResetThemeButton />
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
