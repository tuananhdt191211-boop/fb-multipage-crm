// Dùng chung cho các Cloudflare Pages Function của web công khai (SEO chèn vào HTML, sitemap, robots.txt)
export const DEFAULT_API = 'https://fb-crm-backend.onrender.com/api/v1'

// API công khai của backend: đặt biến môi trường API_BASE_URL (hoặc VITE_API_BASE_URL) trên Cloudflare Pages
export function apiBase(env) {
  const raw = String((env && (env.API_BASE_URL || env.VITE_API_BASE_URL)) || DEFAULT_API).trim().replace(/\/+$/, '')
  return /\/api\/v1$/i.test(raw) ? raw : raw + '/api/v1'
}

// Tên miền của chính ứng dụng (không phải tên miền riêng của khách): APP_HOSTS="a.com,b.com" (mặc định gồm *.pages.dev và localhost)
export function isCustomHost(host, env) {
  const h = String(host || '').toLowerCase()
  if (!h || h === 'localhost' || h.endsWith('.localhost') || h.endsWith('.local') || h.endsWith('.pages.dev')) return false
  if (/^\d{1,3}(\.\d{1,3}){3}$/.test(h) || h.includes(':')) return false
  const extra = String((env && env.APP_HOSTS) || '').split(',').map((x) => x.trim().toLowerCase()).filter(Boolean)
  return !extra.includes(h)
}

// fetch có giới hạn thời gian: backend ngủ đông (Render miễn phí) thì bỏ qua, trang vẫn chạy bình thường không có thẻ SEO chèn sẵn
export async function fetchWithTimeout(url, ms, init) {
  const ctl = new AbortController()
  const timer = setTimeout(() => ctl.abort(), ms)
  try {
    return await fetch(url, { ...(init || {}), signal: ctl.signal })
  } finally {
    clearTimeout(timer)
  }
}
