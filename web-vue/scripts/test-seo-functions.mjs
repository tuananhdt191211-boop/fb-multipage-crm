// Kiểm tra logic SEO chạy ở Cloudflare Function (không cần Cloudflare): node scripts/test-seo-functions.mjs
// Dựng thẻ SEO từ thông tin máy chủ trả về, chèn vào index.html, và chạy thử _middleware / sitemap / robots với fetch giả.
import assert from 'node:assert/strict'
import { buildSeoTags, injectSeoIntoHtml } from '../src/lib/seoTags.ts'
import { onRequest } from '../functions/_middleware.js'
import { onRequestGet as sitemap } from '../functions/sitemap.xml.js'
import { onRequestGet as slugSitemap } from '../functions/s/[slug]/sitemap.xml.js'
import { onRequestGet as robots } from '../functions/robots.txt.js'
import { apiBase, isCustomHost } from '../functions/lib/site.js'

const INDEX = `<!doctype html><html lang="vi"><head><meta charset="UTF-8" /><meta name="description" content="Cũ" /><title>PagePilot CRM</title></head><body><div id="app"></div></body></html>`
const META = { title: 'Giới thiệu "thương hiệu" <b>', description: 'Mô tả & ngắn', image: '/api/v1/storefront/media/9', noindex: false, type: 'website', site_name: 'Shop A', path: '/pages/gioi-thieu', canonical_base: '' }
let pass = 0
const ok = (name, fn) => { fn(); pass++; console.log('PASS ', name) }
const okAsync = async (name, fn) => { await fn(); pass++; console.log('PASS ', name) }

ok('buildSeoTags: canonical, og, robots, ảnh tương đối thành tuyệt đối', () => {
  const t = buildSeoTags(META, { origin: 'https://app.pages.dev', prefix: '/s/shop-a' })
  assert.equal(t.canonical, 'https://app.pages.dev/s/shop-a/pages/gioi-thieu')
  const m = Object.fromEntries(t.metas.map((x) => [x.name || x.property, x.content]))
  assert.equal(m['og:image'], 'https://app.pages.dev/api/v1/storefront/media/9')
  assert.equal(m['og:url'], t.canonical)
  assert.equal(m.robots, 'index, follow')
  assert.equal(m['twitter:card'], 'summary_large_image')
  assert.equal(t.jsonld.length, 0) // trang thường không có JSON-LD
})
ok('canonical dùng tên miền riêng khi máy chủ báo đã kết nối', () => {
  const t = buildSeoTags({ ...META, canonical_base: 'https://www.cuahang.vn' }, { origin: 'https://app.pages.dev', prefix: '/s/shop-a' })
  assert.equal(t.canonical, 'https://www.cuahang.vn/pages/gioi-thieu')
})
ok('trang chủ: Organization + WebSite; sản phẩm: Product + Offer; noindex: không JSON-LD', () => {
  const home = buildSeoTags({ ...META, path: '/' }, { origin: 'https://x.com', prefix: '' })
  assert.deepEqual(home.jsonld.map((j) => j['@type']), ['Organization', 'WebSite'])
  assert.equal(home.canonical, 'https://x.com')
  const prod = buildSeoTags({ ...META, type: 'product', path: '/p/5', product: { name: 'Kem', description: 'd', image: 'https://c.com/a.jpg', sku: 'K1', price: 250000, currency: 'VND', in_stock: true } }, { origin: 'https://x.com', prefix: '' })
  const p = prod.jsonld.find((j) => j['@type'] === 'Product')
  assert.equal(p.offers.price, '250000')
  assert.equal(p.offers.availability, 'https://schema.org/InStock')
  const no = buildSeoTags({ ...META, noindex: true, path: '/', product: null }, { origin: 'https://x.com', prefix: '' })
  assert.equal(no.jsonld.length, 0)
  assert.equal(Object.fromEntries(no.metas.map((x) => [x.name, x.content])).robots, 'noindex, follow')
})
ok('injectSeoIntoHtml: thay title, bỏ meta cũ, escape ký tự nguy hiểm, không chèn script từ dữ liệu', () => {
  const html = injectSeoIntoHtml(INDEX, buildSeoTags(META, { origin: 'https://x.com', prefix: '' }))
  assert.match(html, /<title>Giới thiệu &quot;thương hiệu&quot; &lt;b&gt;<\/title>/)
  assert.equal((html.match(/name="description"/g) || []).length, 1)
  assert.ok(!html.includes('content="Cũ"'))
  assert.ok(html.includes('<link rel="canonical" href="https://x.com/pages/gioi-thieu" data-pp-seo>'))
  const evil = injectSeoIntoHtml(INDEX, buildSeoTags({ ...META, type: 'product', path: '/p/1', product: { name: '</script><script>alert(1)</script>', description: '', image: '', sku: '', price: 1, currency: 'VND', in_stock: true } }, { origin: 'https://x.com', prefix: '' }))
  assert.ok(!evil.includes('</script><script>alert(1)'))
})

const realFetch = globalThis.fetch
const calls = []
globalThis.fetch = async (url, init) => {
  calls.push(String(url))
  if (String(url).includes('/storefront/public/seo')) {
    if (String(url).includes('slug=hong')) return new Response('{}', { status: 404 })
    if (String(url).includes('slug=cham')) {
      await new Promise((resolve, reject) => {
        const timer = setTimeout(resolve, 4000)
        init.signal.addEventListener('abort', () => { clearTimeout(timer); reject(new DOMException('aborted', 'AbortError')) })
      })
      return new Response('{}')
    }
    return Response.json({ ...META, path: new URL(String(url)).searchParams.get('path') })
  }
  if (String(url).includes('/sitemap.xml')) return new Response('<urlset/>', { status: 200 })
  return realFetch(url, init)
}
const htmlRes = () => new Response(INDEX, { status: 200, headers: { 'content-type': 'text/html; charset=utf-8' } })
const run = (u, env = {}, next = async () => htmlRes()) => onRequest({ request: new Request(u), env, next })

await okAsync('middleware: /s/<slug>/pages/x được chèn SEO, gọi API đúng tham số', async () => {
  calls.length = 0
  const res = await run('https://app.pages.dev/s/shop-a/pages/gioi-thieu')
  const body = await res.text()
  assert.match(body, /<link rel="canonical" href="https:\/\/app\.pages\.dev\/s\/shop-a\/pages\/gioi-thieu"/)
  assert.match(calls[0], /\/api\/v1\/storefront\/public\/seo\?slug=shop-a&path=%2Fpages%2Fgioi-thieu/)
})
await okAsync('middleware: tên miền riêng dùng domain=, không có tiền tố /s/', async () => {
  calls.length = 0
  const body = await (await run('https://www.cuahang.vn/p/12')).text()
  assert.match(calls[0], /domain=www\.cuahang\.vn&path=%2Fp%2F12/)
  assert.match(body, /href="https:\/\/www\.cuahang\.vn\/p\/12"/)
})
await okAsync('middleware: trang quản trị / tệp tĩnh / POST không bị đụng, không gọi API', async () => {
  calls.length = 0
  for (const u of ['https://fb-multipage-crm.pages.dev/inbox', 'https://fb-multipage-crm.pages.dev/', 'https://app.pages.dev/s/shop-a/assets.js', 'https://x.pages.dev/s/shop-a/logo.png']) {
    const body = await (await run(u)).text()
    assert.equal(body, INDEX, u)
  }
  const r = await onRequest({ request: new Request('https://app.pages.dev/s/shop-a', { method: 'POST' }), env: {}, next: async () => htmlRes() })
  assert.equal(await r.text(), INDEX)
  assert.equal(calls.length, 0)
})
await okAsync('middleware: API 404 / API chậm / lỗi → trả nguyên HTML gốc, không treo', async () => {
  assert.equal(await (await run('https://app.pages.dev/s/hong/pages/x')).text(), INDEX)
  const t0 = Date.now()
  assert.equal(await (await run('https://app.pages.dev/s/cham')).text(), INDEX)
  assert.ok(Date.now() - t0 < 3000, 'phải bỏ qua sau ~1.8 giây')
  // API trả JSON hỏng → vẫn trả đúng một lần next() với HTML gốc
  let nextCalls = 0
  const bad = globalThis.fetch
  globalThis.fetch = async () => new Response('không phải json', { status: 200 })
  const r = await onRequest({ request: new Request('https://app.pages.dev/s/shop-a'), env: {}, next: async () => { nextCalls++; return htmlRes() } })
  globalThis.fetch = bad
  assert.equal(await r.text(), INDEX)
  assert.equal(nextCalls, 1, 'chỉ gọi next() một lần')
})
await okAsync('middleware: phản hồi không phải HTML (JSON, 404) giữ nguyên', async () => {
  const res = await onRequest({ request: new Request('https://app.pages.dev/s/shop-a'), env: {}, next: async () => new Response('{"a":1}', { headers: { 'content-type': 'application/json' } }) })
  assert.equal(await res.text(), '{"a":1}')
  const nf = await onRequest({ request: new Request('https://app.pages.dev/s/shop-a'), env: {}, next: async () => new Response('x', { status: 404, headers: { 'content-type': 'text/html' } }) })
  assert.equal(nf.status, 404)
})
await okAsync('sitemap: tên miền riêng → gọi backend với domain/origin; tên miền hệ thống → 404; /s/<slug>/sitemap.xml đúng prefix', async () => {
  calls.length = 0
  const a = await sitemap({ request: new Request('https://www.cuahang.vn/sitemap.xml'), env: {} })
  assert.equal(a.status, 200)
  assert.match(calls[0], /domain=www\.cuahang\.vn&origin=https%3A%2F%2Fwww\.cuahang\.vn&prefix=$/)
  assert.equal((await sitemap({ request: new Request('https://app.pages.dev/sitemap.xml'), env: {} })).status, 404)
  const b = await slugSitemap({ request: new Request('https://app.pages.dev/s/shop-a/sitemap.xml'), env: {}, params: { slug: 'shop-a' } })
  assert.equal(b.headers.get('content-type'), 'application/xml; charset=utf-8')
  assert.match(calls[1], /slug=shop-a&origin=https%3A%2F%2Fapp\.pages\.dev&prefix=%2Fs%2Fshop-a/)
  assert.equal((await slugSitemap({ request: new Request('https://app.pages.dev/s/Xấu/sitemap.xml'), env: {}, params: { slug: 'Xấu' } })).status, 404)
})
{
  const own = await robots({ request: new Request('https://www.cuahang.vn/robots.txt'), env: {} }).text()
  assert.match(own, /Sitemap: https:\/\/www\.cuahang\.vn\/sitemap\.xml/)
  assert.match(own, /Disallow: \/checkout/)
  const app = await robots({ request: new Request('https://app.pages.dev/robots.txt'), env: {} }).text()
  assert.ok(!app.includes('Sitemap:') && app.includes('Disallow: /login'))
  pass++
  console.log('PASS  robots.txt thật')
}
ok('apiBase / isCustomHost', () => {
  assert.equal(apiBase({}), 'https://fb-crm-backend.onrender.com/api/v1')
  assert.equal(apiBase({ API_BASE_URL: 'https://api.x.com/' }), 'https://api.x.com/api/v1')
  assert.equal(apiBase({ VITE_API_BASE_URL: 'https://api.x.com/api/v1' }), 'https://api.x.com/api/v1')
  assert.equal(isCustomHost('www.cuahang.vn', {}), true)
  assert.equal(isCustomHost('fb-multipage-crm.pages.dev', {}), false)
  assert.equal(isCustomHost('localhost', {}), false)
  assert.equal(isCustomHost('app.mydomain.com', { APP_HOSTS: 'app.mydomain.com' }), false)
})
console.log(`\nKẾT QUẢ: ${pass} đạt`)
