import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import { api } from './../lib/api'
import type { AuthResponse, Feature, LoginPayload, RegisterPayload, Tenant, User } from './../types/auth'

const TOKEN_KEY = 'access_token'
const USER_KEY = 'auth_user'
const TENANT_KEY = 'auth_tenant'
const VIEW_AS_KEY = 'pp_view_as' // id khách hàng mà admin đang xem như (chỉ đọc); api.ts gắn vào header X-Workspace
const VIEW_AS_NAME_KEY = 'pp_view_as_name'
const DEFAULT_API_BASE_URL = 'https://fb-crm-backend.onrender.com/api/v1'

function normalizeApiBaseUrl(raw?: string): string {
  const value = String(raw || '').trim()
  if (!value) return DEFAULT_API_BASE_URL

  const urlMatch = value.match(/https?:\/\/[^\s\]\)]+/i)
  const cleaned = (urlMatch ? urlMatch[0] : value)
    .replace(/^[\[\(]+|[\]\)]+$/g, '')
    .replace(/\/+$/, '')

  if (!/^https?:\/\//i.test(cleaned)) {
    return DEFAULT_API_BASE_URL
  }

  return /\/api\/v1$/i.test(cleaned) ? cleaned : `${cleaned}/api/v1`
}

// Đảm bảo instance `api` không bao giờ dùng baseURL bị lỗi ngoặc hoặc trỏ nhầm sang pages.dev
if (api?.defaults) {
  const currentBase = String(api.defaults.baseURL || '')
  if (!currentBase || currentBase.includes('[') || currentBase.includes('pages.dev')) {
    api.defaults.baseURL = normalizeApiBaseUrl(
      import.meta.env.VITE_API_BASE_URL || import.meta.env.VITE_BACKEND_URL
    )
  }
}

function loadStoredUser(): User | null {
  try {
    const raw = localStorage.getItem(USER_KEY)
    return raw ? (JSON.parse(raw) as User) : null
  } catch {
    return null
  }
}

function loadStoredTenant(): Tenant | null {
  try {
    const raw = localStorage.getItem(TENANT_KEY)
    return raw ? (JSON.parse(raw) as Tenant) : null
  } catch {
    return null
  }
}

export const useAuthStore = defineStore('auth', () => {
  const token = ref<string | null>(localStorage.getItem(TOKEN_KEY))
  const user = ref<User | null>(loadStoredUser())
  const tenant = ref<Tenant | null>(loadStoredTenant())
  const viewAsId = ref<number>(Number(localStorage.getItem(VIEW_AS_KEY)) || 0)
  const viewAsName = ref<string>(localStorage.getItem(VIEW_AS_NAME_KEY) || '')
  const loading = ref(false)
  const error = ref<string | null>(null)

  const isAuthenticated = computed(() => Boolean(token.value))
  const isAdmin = computed(() => user.value?.system_role === 'admin')
  const isStaff = computed(() => user.value?.system_role === 'staff')
  // Chủ không gian dữ liệu (khách hàng, hoặc admin trong không gian của mình): được quản lý nhân viên, kết nối Fanpage...
  const isOwner = computed(() => (tenant.value ? tenant.value.is_owner || tenant.value.viewing : !isStaff.value))
  const viewing = computed(() => Boolean(tenant.value?.viewing))
  // Trang được dùng; chưa tải xong (chưa biết) thì tạm cho phép — máy chủ vẫn kiểm tra thật
  function can(feature: Feature): boolean {
    return tenant.value ? tenant.value.features.includes(feature) : true
  }

  function setSession(newToken: string, newUser?: User | null) {
    token.value = newToken
    localStorage.setItem(TOKEN_KEY, newToken)
    if (newUser) {
      user.value = newUser
      localStorage.setItem(USER_KEY, JSON.stringify(newUser))
    }
  }

  function clearSession() {
    token.value = null
    user.value = null
    tenant.value = null
    viewAsId.value = 0
    viewAsName.value = ''
    localStorage.removeItem(TOKEN_KEY)
    localStorage.removeItem(USER_KEY)
    localStorage.removeItem(TENANT_KEY)
    localStorage.removeItem(VIEW_AS_KEY)
    localStorage.removeItem(VIEW_AS_NAME_KEY)
  }

  async function login(payload: LoginPayload) {
    loading.value = true
    error.value = null
    try {
      const { data } = await api.post<AuthResponse>('/auth/login', payload)
      setSession(data.token, data.user)
      return true
    } catch (err: any) {
      error.value =
        err.response?.data?.error || 'Không thể kết nối máy chủ. Kiểm tra lại Backend Golang.'
      return false
    } finally {
      loading.value = false
    }
  }

  async function register(payload: RegisterPayload) {
    loading.value = true
    error.value = null
    try {
      const { data } = await api.post<AuthResponse>('/auth/register', payload)
      setSession(data.token, data.user)
      return true
    } catch (err: any) {
      error.value = err.response?.data?.error || 'Đăng ký thất bại. Vui lòng thử lại.'
      return false
    } finally {
      loading.value = false
    }
  }

  async function fetchMe() {
    if (!token.value) return false
    try {
      const { data } = await api.get<{ user: User; tenant?: Tenant }>('/auth/me')
      user.value = data.user
      localStorage.setItem(USER_KEY, JSON.stringify(data.user))
      tenant.value = data.tenant || null
      if (data.tenant) localStorage.setItem(TENANT_KEY, JSON.stringify(data.tenant))
      else localStorage.removeItem(TENANT_KEY)
      return true
    } catch (err: any) {
      const status = err?.response?.status
      // Khách hàng đang xem không tồn tại / không xem được: bỏ chế độ xem thay vì đăng xuất admin
      if (viewAsId.value && status && status !== 401) {
        stopViewAs()
        return fetchMe()
      }
      // Chỉ đăng xuất khi máy chủ báo phiên không hợp lệ; lỗi mạng / bảo trì thì giữ phiên
      if (status === 401) clearSession()
      return status === 401 ? false : Boolean(user.value)
    }
  }

  // Admin xem như khách hàng (chỉ đọc)
  async function startViewAs(id: number, name: string) {
    viewAsId.value = id
    viewAsName.value = name
    localStorage.setItem(VIEW_AS_KEY, String(id))
    localStorage.setItem(VIEW_AS_NAME_KEY, name)
    return fetchMe()
  }

  function stopViewAs() {
    viewAsId.value = 0
    viewAsName.value = ''
    localStorage.removeItem(VIEW_AS_KEY)
    localStorage.removeItem(VIEW_AS_NAME_KEY)
  }

  async function handleOAuthCallback(oauthToken: string) {
    setSession(oauthToken, null)
    return await fetchMe()
  }

  async function logout() {
    try {
      if (token.value) {
        await api.post('/auth/logout')
      }
    } catch {
      // Vẫn xóa phiên phía client nếu token hết hạn hoặc server ngắt
    } finally {
      clearSession()
    }
  }

  return {
    token,
    user,
    tenant,
    viewAsId,
    viewAsName,
    loading,
    error,
    isAuthenticated,
    isAdmin,
    isStaff,
    isOwner,
    viewing,
    can,
    startViewAs,
    stopViewAs,
    login,
    register,
    fetchMe,
    handleOAuthCallback,
    logout,
    clearSession
  }
})