// /s/<slug>/sitemap.xml — sơ đồ website của một cửa hàng chạy trên tên miền của hệ thống (gửi cho Google Search Console)
import { apiBase, fetchWithTimeout } from '../../lib/site.js'

export async function onRequestGet({ request, env, params }) {
  const slug = String(params.slug || '').toLowerCase()
  if (!/^[a-z0-9]+(-[a-z0-9]+)*$/.test(slug)) return new Response('Not found', { status: 404 })
  const url = new URL(request.url)
  const q = new URLSearchParams({ slug, origin: url.origin, prefix: '/s/' + slug })
  try {
    const r = await fetchWithTimeout(`${apiBase(env)}/storefront/public/sitemap.xml?${q}`, 8000)
    return new Response(await r.text(), { status: r.status, headers: { 'Content-Type': 'application/xml; charset=utf-8', 'Cache-Control': 'public, max-age=300' } })
  } catch {
    return new Response('Sitemap tạm thời chưa tải được', { status: 502 })
  }
}
