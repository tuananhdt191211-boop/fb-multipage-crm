import axios from 'axios'

const DEFAULT_API_BASE_URL = 'https://fb-crm-backend.onrender.com/api/v1'

function resolveApiBaseUrl(): string {
  const rawEnv = String(
    import.meta.env.VITE_API_BASE_URL ||
      import.meta.env.VITE_BACKEND_URL ||
      DEFAULT_API_BASE_URL
  ).trim()

  // Nếu giá trị bị dính dạng Markdown [url](url) thì trích URL đầu tiên hợp lệ
  const urlMatch = rawEnv.match(/https?:\/\/[^\s\]\)]+/i)
  const cleaned = (urlMatch ? urlMatch[0] : rawEnv)
    .replace(/^[\[\(]+|[\]\)]+$/g, '')
    .replace(/\/+$/, '')

  if (!/^https?:\/\//i.test(cleaned)) {
    return DEFAULT_API_BASE_URL
  }

  return /\/api\/v1$/i.test(cleaned) ? cleaned : `${cleaned}/api/v1`
}

export const api = axios.create({
  baseURL: resolveApiBaseUrl(),
  timeout: 65000,
  headers: {
    'Content-Type': 'application/json'
  }
})

let isRedirectingToLogin = false

api.interceptors.request.use((config) => {
  const token = localStorage.getItem('access_token')
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  // Admin đang "xem như khách": các API dữ liệu trả về dữ liệu của khách đó (chỉ đọc). Khu quản trị không dùng header này.
  const viewAs = localStorage.getItem('pp_view_as')
  if (viewAs && token && !String(config.url || '').startsWith('/admin')) {
    config.headers['X-Workspace'] = viewAs
  }
  return config
})

api.interceptors.response.use(
  (response) => response,
  (error) => {
    const status = error.response?.status
    const requestUrl = String(error.config?.url || '')
    const isAuthRequest =
      requestUrl.includes('/auth/login') || requestUrl.includes('/auth/register')

    if (status === 401 && !isAuthRequest) {
      localStorage.removeItem('access_token')
      localStorage.removeItem('token')
      localStorage.removeItem('auth_user')

      const onAuthScreen =
        window.location.pathname.startsWith('/login') ||
        window.location.pathname.startsWith('/register') ||
        window.location.pathname.startsWith('/auth/callback')

      if (!onAuthScreen && !isRedirectingToLogin) {
        isRedirectingToLogin = true
        window.location.replace('/login')
      }
    }
    return Promise.reject(error)
  }
)