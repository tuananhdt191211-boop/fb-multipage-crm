export type SystemRole = 'admin' | 'manager' | 'staff'

export interface User {
  id: number
  full_name: string
  email: string
  avatar_url?: string
  auth_provider?: 'local' | 'google' | 'facebook'
  system_role: SystemRole
  manager_id?: number | null
  permissions?: string[]
  is_active: boolean
  created_at?: string
}

export interface AuthResponse {
  token: string
  user: User
}

export interface LoginPayload {
  email: string
  password: string
}

export interface RegisterPayload {
  full_name: string
  email: string
  password: string
}