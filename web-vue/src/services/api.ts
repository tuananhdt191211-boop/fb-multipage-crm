import axios from 'axios'

const api = axios.create({
  baseURL: import.meta.env.VITE_API_URL || 'http://localhost:8080/api/v1'
})

function extractJwtToken(): string {
  const candidateKeys = ['token', 'access_token', 'auth_token', 'auth']
  for (const key of candidateKeys) {
    const raw = localStorage.getItem(key)
    if (!raw) continue

    // Nếu lưu dạng JSON object (ví dụ Pinia persist): {"token": "eyJ..."}
    if (raw.startsWith('{')) {
      try {
        const parsed = JSON.parse(raw)
        if (typeof parsed.token === 'string' && parsed.token) return parsed.token
        if (typeof parsed.access_token === 'string' && parsed.access_token) return parsed.access_token
      } catch {
        // bỏ qua nếu không phải JSON
      }
    }

    // Loại bỏ dấu ngoặc kép thừa nếu có
    const cleaned = raw.replace(/^"+|"+$/g, '').trim()
    if (cleaned.split('.').length === 3) {
      return cleaned
    }
  }
  return ''
}

api.interceptors.request.use((config) => {
  const token = extractJwtToken()
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

export default api