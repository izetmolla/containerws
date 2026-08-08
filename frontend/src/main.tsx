import { StrictMode } from "react"
import { createRoot } from "react-dom/client"


import "./global.css"
import "@/components/layouts/dashboard1/styles/index.css"
import "sonner/dist/styles.css"
import Providers from "@/components/providers/index.tsx"

import { RouterProvider } from "react-router"
import router from "./modules/router"


createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <Providers>
     <RouterProvider router={router} />
    </Providers>
  </StrictMode>
)
