// Nhận biết web đang chạy trên tên miền riêng của khách (vd. www.cuahang.vn)
// thay vì tên miền của hệ thống (*.pages.dev, localhost, VITE_APP_HOSTS).
// Trên tên miền riêng: "/" là trang chủ cửa hàng, đường dẫn ngắn /p/:pid, /checkout, /order/:code.
import { loadPublicSite } from './publicSite'

const extraAppHosts = String(import.meta.env.VITE_APP_HOSTS || '')
  .split(',')
  .map((s) => s.trim().toLowerCase())
  .filter(Boolean)

export function isCustomDomainHost(host = window.location.hostname): boolean {
  const h = host.toLowerCase()
  if (!h || h === 'localhost' || h.endsWith('.localhost') || h.endsWith('.local')) return false
  if (/^\d{1,3}(\.\d{1,3}){3}$/.test(h) || h.includes(':')) return false // IP
  if (h.endsWith('.pages.dev')) return false
  return !extraAppHosts.includes(h)
}

export const onCustomDomain = isCustomDomainHost()

let domainSlug = ''

// Gọi trước khi mount app: đọc slug của website gắn với tên miền hiện tại
export async function resolveDomainSlug(): Promise<void> {
  if (!onCustomDomain) return
  try {
    const site = await loadPublicSite({ domain: window.location.hostname })
    domainSlug = site.slug
  } catch {
    domainSlug = '' // trang chủ sẽ tự hiển thị "chưa xuất bản"
  }
}

export function currentSlug(routeSlug: unknown): string {
  return (typeof routeSlug === 'string' && routeSlug) || domainSlug
}

// Đường dẫn trong cửa hàng: "/s/<slug>/p/1" trên hệ thống, "/p/1" trên tên miền riêng
export function storePath(slug: string, sub = ''): string {
  return onCustomDomain ? sub || '/' : `/s/${slug}${sub}`
}
