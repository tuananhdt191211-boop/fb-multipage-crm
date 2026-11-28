// /sitemap.xml của website chạy trên TÊN MIỀN RIÊNG của khách (xem thêm /s/<slug>/sitemap.xml cho địa chỉ của hệ thống)
import { apiBase, fetchWithTimeout, isCustomHost } from './lib/site.js'

export async function onRequestGet({ request, env }) {
  const url = new URL(request.url)
  if (!isCustomHost(url.hostname, env)) return new Response('Not found', { status: 404 })
  const q = new URLSearchParams({ domain: url.hostname.toLowerCase(), origin: url.origin, prefix: '' })
  try {
    const r = await fetchWithTimeout(`${apiBase(env)}/storefront/public/sitemap.xml?${q}`, 8000)
    return new Response(await r.text(), { status: r.status, headers: { 'Content-Type': 'application/xml; charset=utf-8', 'Cache-Control': 'public, max-age=300' } })
  } catch {
    return new Response('Sitemap tạm thời chưa tải được', { status: 502 })
  }
}
