import { Toaster } from "sonner"

import { useTheme } from "@/components/layouts/dashboard1/providers/use-theme"

/** Global Sonner host — must be mounted or toast.* calls are silent. */
export function AppToaster() {
  const { theme } = useTheme()

  return (
    <Toaster
      theme={theme}
      position="bottom-right"
      richColors
      closeButton
      expand
      visibleToasts={5}
      toastOptions={{
        classNames: {
          description: "whitespace-pre-wrap break-words text-xs leading-relaxed",
          toast: "items-start",
        },
      }}
    />
  )
}
