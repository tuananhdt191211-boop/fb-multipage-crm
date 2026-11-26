// Lọc URL do khách hàng nhập để tránh chèn javascript:/data: vào href/src của trang công khai.
export function safeHref(raw?: string, fallback = '#'): string {
  const v = (raw || '').trim()
  if (!v) return fallback
  if (v.startsWith('#') || (v.startsWith('/') && !v.startsWith('//'))) return v
  try {
    const u = new URL(v)
    if (['http:', 'https:', 'mailto:', 'tel:'].includes(u.protocol)) return v
  } catch {
    /* URL không hợp lệ */
  }
  return fallback
}

export function safeImg(raw?: string): string {
  const v = (raw || '').trim()
  if (v.startsWith('/') && !v.startsWith('//')) return v
  try {
    const u = new URL(v)
    if (u.protocol === 'http:' || u.protocol === 'https:') return v
  } catch {
    /* bỏ qua */
  }
  return ''
}