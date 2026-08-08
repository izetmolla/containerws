import type { FC } from "react"
import { ThemeProvider } from "@/components/layouts/dashboard1"
import { QueryClientProvider } from "@tanstack/react-query"
import { queryClient } from "@/lib/network"
import { TooltipProvider } from "@/components/ui/tooltip"
// import { TimePickerFloatingBoundary } from "@/components/DateTimePickers/TimePickerPortal"
import { NuqsAdapter } from "nuqs/adapters/react-router/v8"
import { I18NextProvider } from "./i18n"
import { AppToaster } from "./toaster"
// import { AlertNotifyProvider } from "../alert-notify"

interface ProvidersProps {
    children: React.ReactNode
}

const Providers: FC<ProvidersProps> = ({ children }) => {
    return (
        <NuqsAdapter>
            <I18NextProvider>
                <ThemeProvider>
                    <QueryClientProvider client={queryClient}>
                        <TooltipProvider>
                            {/* <TimePickerFloatingBoundary /> */}
                            {/* <AlertNotifyProvider> */}
                            {children}
                            <AppToaster />
                            {/* </AlertNotifyProvider> */}
                        </TooltipProvider>
                    </QueryClientProvider>
                </ThemeProvider>
            </I18NextProvider>
        </NuqsAdapter>
    )
}

export default Providers
