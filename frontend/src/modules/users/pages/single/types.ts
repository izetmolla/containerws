import type { UserDetail } from "../list/api"

export type UserSingleOutletContext = {
  user: UserDetail
  id: string
  invalidate: () => void
}
