// Chèn thẻ SEO (title, description, Open Graph, canonical, robots, JSON-LD) thẳng vào HTML của web công khai.
// Web là ứng dụng một trang (JavaScript), nên các bot KHÔNG chạy JavaScript (Facebook, Zalo, Messenger xem trước link...) chỉ thấy
// index.html trống. Hàm này hỏi backend thông tin SEO của đúng trang đang mở rồi chèn vào trước khi trả về.
// An toàn: mọi lỗi / chậm đều trả nguyên bản HTML gốc (trình duyệt vẫn tự cập nhật thẻ SEO sau khi tải).
import { buildSeoTags, injectSeoIntoHtml } from '../src/lib/seoTags.ts'
import { apiBase, fetchWithTimeout, isCustomHost } from './lib/site.js'

export async function onRequest(context) {
  const { request, env, next } = context
  let res // đã gọi next() thì lỗi về sau trả đúng phản hồi này, không gọi next() lần hai
  try {
    if (request.method !== 'GET') return next()
    const url = new URL(request.url)
    if (/\.[a-z0-9]{1,8}$/i.test(url.pathname)) return next() // tệp tĩnh (js, css, ảnh...)

    let slug = ''
    let domain = ''
    let path = ''
    let prefix = ''
    const m = /^\/s\/([a-z0-9]+(?:-[a-z0-9]+)*)(\/.*)?$/.exec(url.pathname)
    if (m) {
      slug = m[1]
      path = m[2] || '/'
      prefix = '/s/' + slug
    } else if (isCustomHost(url.hostname, env)) {
      domain = url.hostname.toLowerCase()
      path = url.pathname || '/'
    } else {
      return next() // trang quản trị của ứng dụng
    }

    res = await next()
    if (res.status !== 200 || !String(res.headers.get('content-type') || '').includes('text/html')) return res

    const q = new URLSearchParams(slug ? { slug, path } : { domain, path })
    const api = await fetchWithTimeout(`${apiBase(env)}/storefront/public/seo?${q}`, 1800, { cf: { cacheTtl: 60, cacheEverything: true } })
    if (!api.ok) return res
    const meta = await api.json()
    const html = await res.text()
    const tags = buildSeoTags(meta, { origin: url.origin, prefix })
    const headers = new Headers(res.headers)
    headers.delete('content-length')
    headers.delete('etag')
    headers.set('Cache-Control', 'public, max-age=0, s-maxage=60, must-revalidate')
    return new Response(injectSeoIntoHtml(html, tags), { status: 200, headers })
  } catch {
    return res || next()
  }
}
