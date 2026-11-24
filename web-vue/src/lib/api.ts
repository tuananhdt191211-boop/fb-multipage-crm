import axios from 'axios'

export const api = axios.create({
  baseURL: import.meta.env.VITE_API_BASE_URL || '/api/v1',
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