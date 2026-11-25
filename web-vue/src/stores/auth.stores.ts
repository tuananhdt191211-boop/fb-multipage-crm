import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import { api } from './../lib/api'
import type { AuthResponse, LoginPayload, RegisterPayload, User } from './../types/auth'

const TOKEN_KEY = 'access_token'
const USER_KEY = 'auth_user'
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

export const useAuthStore = defineStore('auth', () => {
  const token = ref<string | null>(localStorage.getItem(TOKEN_KEY))
  const user = ref<User | null>(loadStoredUser())
  const loading = ref(false)
  const error = ref<string | null>(null)

  const isAuthenticated = computed(() => Boolean(token.value))
  const isAdmin = computed(() => user.value?.system_role === 'admin')
  const isManager = computed(() => user.value?.system_role === 'manager')
  const canManageRoles = computed(() => isAdmin.value || isManager.value)

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
    localStorage.removeItem(TOKEN_KEY)
    localStorage.removeItem(USER_KEY)
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
      const { data } = await api.get<{ user: User }>('/auth/me')
      user.value = data.user
      localStorage.setItem(USER_KEY, JSON.stringify(data.user))
      return true
    } catch {
      clearSession()
      return false
    }
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
    loading,
    error,
    isAuthenticated,
    isAdmin,
    isManager,
    canManageRoles,
    login,
    register,
    fetchMe,
    handleOAuthCallback,
    logout,
    clearSession
  }
})