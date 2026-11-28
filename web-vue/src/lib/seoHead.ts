// SEO ở trình duyệt: lấy thông tin SEO đã tính sẵn của một đường dẫn và cập nhật <title>, <meta>, canonical, JSON-LD.
// (Bản chèn sẵn trong HTML do Cloudflare Function làm cho các bot không chạy JavaScript; đây là bản cho khách chuyển trang trong ứng dụng.)
import { api } from './api'
import { onCustomDomain } from './storeHost'
import { buildSeoTags, SEO_ATTR, type SeoMeta, type SeoTags } from './seoTags'

export function applySeoTags(t: SeoTags) {
  document.title = t.title
  document.head.querySelectorAll(`[${SEO_ATTR}]`).forEach((el) => el.remove())
  document.head.querySelectorAll('meta[name="description"], meta[name="robots"]').forEach((el) => el.remove())
  const add = (tag: string, attrs: Record<string, string>, text = '') => {
    const el = document.createElement(tag)
    for (const [k, v] of Object.entries(attrs)) el.setAttribute(k, v)
    el.setAttribute(SEO_ATTR, '1')
    if (text) el.textContent = text
    document.head.appendChild(el)
  }
  for (const m of t.metas) add('meta', m.name ? { name: m.name, content: m.content } : { property: m.property || '', content: m.content })
  for (const l of t.links) add('link', { rel: l.rel, href: l.href })
  for (const j of t.jsonld) add('script', { type: 'application/ld+json' }, JSON.stringify(j))
}

// path: đường dẫn trong website ("/", "/pages/<trang>", "/p/<id>", "/checkout"...). Không báo lỗi nếu máy chủ chưa trả lời.
export async function applyPublicSeo(slug: string, path: string): Promise<SeoMeta | null> {
  try {
    const { data } = await api.get<SeoMeta>('/storefront/public/seo', { params: { slug, path } })
    applySeoTags(buildSeoTags(data, { origin: window.location.origin, prefix: onCustomDomain ? '' : `/s/${slug}` }))
    return data
  } catch {
    return null
  }
}
