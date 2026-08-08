import { HardDrive } from "lucide-react"
import { useOutletContext } from "react-router"

import type { UserSingleOutletContext } from "../types"

export default function UserStoragePage() {
  const { user } = useOutletContext<UserSingleOutletContext>()

  return (
    <section className="flex min-h-[280px] flex-col items-center justify-center rounded-xl border border-dashed bg-muted/20 px-6 py-16 text-center">
      <div className="grid size-12 place-items-center rounded-2xl bg-muted">
        <HardDrive className="size-5 text-muted-foreground" />
      </div>
      <h2 className="mt-4 text-base font-semibold tracking-tight">Storage</h2>
      <p className="mt-1 max-w-sm text-sm text-muted-foreground">
        Coming soon. Home and workspace volumes for{" "}
        <span className="font-medium text-foreground">
          {user.full_name || user.username || "this user"}
        </span>{" "}
        will appear here.
      </p>
    </section>
  )
}
