// /robots.txt: không cho bot vào giỏ hàng / thanh toán / tài khoản / đơn hàng; tên miền riêng có thêm dòng Sitemap.
// (Không có hàm này thì /robots.txt rơi vào index.html của ứng dụng và bot đọc nhầm một trang HTML.)
import { isCustomHost } from './lib/site.js'

export function onRequestGet({ request, env }) {
  const url = new URL(request.url)
  const custom = isCustomHost(url.hostname, env)
  const lines = ['User-agent: *', 'Disallow: /checkout', 'Disallow: /account', 'Disallow: /order/', 'Disallow: /s/*/checkout', 'Disallow: /s/*/account', 'Disallow: /s/*/order/']
  if (!custom) lines.push('Disallow: /login', 'Disallow: /register')
  lines.push('Allow: /')
  if (custom) lines.push('', `Sitemap: ${url.origin}/sitemap.xml`)
  return new Response(lines.join('\n') + '\n', { headers: { 'Content-Type': 'text/plain; charset=utf-8', 'Cache-Control': 'public, max-age=3600' } })
}
