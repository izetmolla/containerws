export interface User {
  id: string
  email: string
  first_name?: string
  last_name?: string
  username?: string
  image?: string
  created_at: string
  roles: string[]
}

export interface Tokens {
  access_token: string
  refresh_token: string
}

export interface AuthSession {
  session_id: string
  trusted: boolean
  user: User
  tokens: Tokens
}

export interface Confirmation {
  message?: string
  type: "email" | "phone" | "device"
  email?: string
  phone?: string
  device?: string
}
