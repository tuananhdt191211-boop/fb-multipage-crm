// Vai trò: admin theo dõi toàn bộ; customer là một khách hàng với dữ liệu riêng; staff là nhân viên của một khách hàng
export type SystemRole = 'admin' | 'customer' | 'staff'

// Các trang có thể bật / tắt cho từng tài khoản
export type Feature = 'pages' | 'inbox' | 'ai' | 'posts' | 'competitors' | 'broadcasts' | 'sites'

export const FEATURE_LABELS: Record<Feature, string> = {
  pages: 'Fanpage',
  inbox: 'Hộp thư đa Page',
  ai: 'AI trả lời & tin nhắn mẫu',
  posts: 'Lên lịch đăng bài',
  competitors: 'Phân tích đối thủ',
  broadcasts: 'CSKH tự động (chăm sóc khách cũ)',
  sites: 'Website bán hàng'
}

export const ALL_FEATURES = Object.keys(FEATURE_LABELS) as Feature[]

export interface User {
  id: number
  full_name: string
  email: string
  avatar_url?: string
  auth_provider?: 'local' | 'google' | 'facebook'
  system_role: SystemRole
  owner_id?: number | null
  features?: Feature[] | null
  max_pages?: number
  max_sites?: number
  max_staff?: number
  last_login_at?: string | null
  is_active: boolean
  created_at?: string
}

// Không gian dữ liệu đang dùng (do máy chủ tính cho từng yêu cầu)
export interface Tenant {
  workspace_id: number
  owner_name: string
  is_owner: boolean
  viewing: boolean // admin đang xem như khách (chỉ đọc)
  features: Feature[]
  limits: { max_pages: number; max_sites: number; max_staff: number }
  usage: { pages: number; sites: number; staff: number }
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
