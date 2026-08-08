/**
 * Single shared react-query client for the workspace. Lives in its
 * own module so it can be imported by the providers without dragging
 * in the axios stack, and so test setups can swap it out cleanly.
 */

import { QueryClient } from "@tanstack/react-query"

export const queryClient = new QueryClient()
