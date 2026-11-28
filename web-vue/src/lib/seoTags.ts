// SEO: từ thông tin SEO đã tính sẵn ở máy chủ (GET /storefront/public/seo) dựng ra các thẻ <title>, <meta>, <link rel=canonical>, JSON-LD.
// Hàm THUẦN (không dùng DOM) nên dùng chung cho:
//   - Cloudflare Pages Function (functions/_middleware.js): chèn thẻ thẳng vào HTML để Facebook / Zalo / Google xem trước đúng
//     dù không chạy JavaScript;
//   - trình duyệt (lib/seoHead.ts): cập nhật khi khách chuyển trang trong ứng dụng.

export interface SeoProduct {
  name: string
  description: string
  image: string
  sku: string
  price: number
  currency: string
  in_stock: boolean
}

export interface SeoMeta {
  title: string
  description: string
  image: string
  noindex: boolean
  type: string // website | product
  site_name: string
  path: string // đường dẫn trong website: "/", "/pages/<trang>", "/p/<id>"
  canonical_base: string // https://<tên miền riêng> khi đã kết nối xong; rỗng = dùng nơi đang truy cập
  product?: SeoProduct | null
}

export interface SeoTags {
  title: string
  metas: { name?: string; property?: string; content: string }[]
  links: { rel: string; href: string }[]
  jsonld: Record<string, unknown>[]
  canonical: string
}

export const SEO_ATTR = 'data-pp-seo'

// origin: https://host (không có dấu / cuối); prefix: "" hoặc "/s/<slug>"
export function buildSeoTags(m: SeoMeta, ctx: { origin: string; prefix: string }): SeoTags {
  const base = (m.canonical_base || ctx.origin + ctx.prefix).replace(/\/+$/, '')
  const canonical = base + (m.path === '/' ? '' : m.path) || ctx.origin + '/'
  const abs = (u: string) => {
    const v = (u || '').trim()
    if (/^https?:\/\//i.test(v)) return v
    if (v.startsWith('/') && !v.startsWith('//')) return ctx.origin + v
    return ''
  }
  const image = abs(m.image)
  const metas: SeoTags['metas'] = []
  const add = (key: 'name' | 'property', k: string, content: string) => {
    if (content) metas.push({ [key]: k, content } as SeoTags['metas'][number])
  }
  add('name', 'description', m.description)
  add('name', 'robots', m.noindex ? 'noindex, follow' : 'index, follow')
  add('property', 'og:type', m.type === 'product' ? 'product' : 'website')
  add('property', 'og:site_name', m.site_name)
  add('property', 'og:title', m.title)
  add('property', 'og:description', m.description)
  add('property', 'og:url', canonical)
  add('property', 'og:image', image)
  add('name', 'twitter:card', image ? 'summary_large_image' : 'summary')
  add('name', 'twitter:title', m.title)
  add('name', 'twitter:description', m.description)
  add('name', 'twitter:image', image)

  const jsonld: SeoTags['jsonld'] = []
  if (!m.noindex && m.path === '/') {
    jsonld.push({ '@context': 'https://schema.org', '@type': 'Organization', name: m.site_name, url: canonical, ...(image ? { logo: image } : {}) })
    jsonld.push({ '@context': 'https://schema.org', '@type': 'WebSite', name: m.site_name, url: canonical })
  }
  if (!m.noindex && m.product) {
    const p = m.product
    jsonld.push({
      '@context': 'https://schema.org',
      '@type': 'Product',
      name: p.name,
      ...(abs(p.image) ? { image: [abs(p.image)] } : {}),
      ...(p.description ? { description: p.description } : {}),
      ...(p.sku ? { sku: p.sku } : {}),
      offers: {
        '@type': 'Offer',
        url: canonical,
        price: String(p.price),
        priceCurrency: p.currency || 'VND',
        availability: p.in_stock ? 'https://schema.org/InStock' : 'https://schema.org/OutOfStock'
      }
    })
  }
  return { title: m.title, metas, links: [{ rel: 'canonical', href: canonical }], jsonld, canonical }
}

const esc = (s: string) => s.replace(/&/g, '&amp;').replace(/"/g, '&quot;').replace(/</g, '&lt;').replace(/>/g, '&gt;')

// Các thẻ dạng chuỗi HTML (đã escape), mỗi thẻ gắn data-pp-seo để trình duyệt thay lại khi chuyển trang
export function renderSeoHead(t: SeoTags): string {
  const out: string[] = []
  for (const m of t.metas) out.push(`<meta ${m.name ? `name="${esc(m.name)}"` : `property="${esc(m.property || '')}"`} content="${esc(m.content)}" ${SEO_ATTR}>`)
  for (const l of t.links) out.push(`<link rel="${esc(l.rel)}" href="${esc(l.href)}" ${SEO_ATTR}>`)
  for (const j of t.jsonld) out.push(`<script type="application/ld+json" ${SEO_ATTR}>${JSON.stringify(j).replace(/</g, '\\u003c')}</script>`)
  return out.join('\n')
}

// Chèn vào HTML của ứng dụng: thay <title>, bỏ <meta name="description"> / robots cũ, thêm thẻ SEO trước </head>
export function injectSeoIntoHtml(html: string, t: SeoTags): string {
  let out = html
  const titleTag = `<title>${esc(t.title)}</title>`
  out = /<title[^>]*>[\s\S]*?<\/title>/i.test(out) ? out.replace(/<title[^>]*>[\s\S]*?<\/title>/i, titleTag) : out.replace(/<\/head>/i, titleTag + '\n</head>')
  out = out.replace(/<meta\s+name=["'](description|robots)["'][^>]*>\s*/gi, '')
  return out.replace(/<\/head>/i, renderSeoHead(t) + '\n</head>')
}
