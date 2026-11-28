// Làm sạch HTML mô tả sản phẩm ở trình duyệt. Máy chủ cũng lọc khi lưu; bộ lọc này dùng cho nội dung dán từ web khác vào
// trình soạn thảo và là lớp bảo vệ thứ hai khi hiển thị công khai. Cùng danh sách cho phép với backend (html_sanitize.go).
const ALLOWED = new Set([
  'P', 'BR', 'HR', 'STRONG', 'B', 'EM', 'I', 'U', 'S', 'STRIKE', 'H2', 'H3', 'H4', 'UL', 'OL', 'LI', 'BLOCKQUOTE', 'PRE', 'CODE',
  'A', 'IMG', 'TABLE', 'THEAD', 'TBODY', 'TR', 'TH', 'TD', 'SUB', 'SUP', 'FIGURE', 'FIGCAPTION'
])
// bỏ luôn cả nội dung bên trong
const DROP = new Set([
  'SCRIPT', 'STYLE', 'NOSCRIPT', 'IFRAME', 'OBJECT', 'EMBED', 'SVG', 'MATH', 'TEMPLATE', 'TEXTAREA', 'SELECT', 'OPTION', 'FORM', 'INPUT',
  'BUTTON', 'LINK', 'META', 'HEAD', 'TITLE', 'BASE', 'CANVAS', 'AUDIO', 'VIDEO', 'SOURCE'
])

function safeUrl(raw: string | null, allowMail: boolean): string {
  const v = (raw || '').trim()
  if (!v || v.length > 2000) return ''
  if (v.startsWith('#') && allowMail) return v
  if (v.startsWith('/') && !v.startsWith('//')) return v
  try {
    const u = new URL(v)
    if (u.protocol === 'http:' || u.protocol === 'https:') return v
    if (allowMail && (u.protocol === 'mailto:' || u.protocol === 'tel:')) return v
  } catch {
    /* không phải URL */
  }
  return ''
}

function alias(tag: string): string {
  if (tag === 'H1') return 'H2'
  if (tag === 'H5' || tag === 'H6') return 'H4'
  return tag
}

function copyInto(src: Node, out: Node, doc: Document) {
  src.childNodes.forEach((child) => {
    if (child.nodeType === Node.TEXT_NODE) {
      out.appendChild(doc.createTextNode(child.textContent || ''))
      return
    }
    if (child.nodeType !== Node.ELEMENT_NODE) return
    const el = child as Element
    let tag = el.tagName.toUpperCase()
    if (DROP.has(tag)) return
    tag = alias(tag)
    if (!ALLOWED.has(tag)) {
      copyInto(el, out, doc) // thẻ lạ (div, span, font...): bỏ thẻ, giữ chữ
      return
    }
    if (tag === 'IMG') {
      const src2 = safeUrl(el.getAttribute('src'), false)
      if (!src2) return
      const img = doc.createElement('img')
      img.setAttribute('src', src2)
      img.setAttribute('alt', (el.getAttribute('alt') || '').slice(0, 200))
      img.setAttribute('loading', 'lazy')
      img.setAttribute('referrerpolicy', 'no-referrer')
      out.appendChild(img)
      return
    }
    if (tag === 'A') {
      const href = safeUrl(el.getAttribute('href'), true)
      if (!href) {
        copyInto(el, out, doc)
        return
      }
      const a = doc.createElement('a')
      a.setAttribute('href', href)
      if (!href.startsWith('#')) {
        a.setAttribute('target', '_blank')
        a.setAttribute('rel', 'noopener noreferrer nofollow')
      }
      copyInto(el, a, doc)
      out.appendChild(a)
      return
    }
    const n = doc.createElement(tag.toLowerCase())
    if (tag === 'TD' || tag === 'TH') {
      for (const k of ['colspan', 'rowspan']) {
        const v = el.getAttribute(k) || ''
        if (/^\d{1,3}$/.test(v)) n.setAttribute(k, v)
      }
    }
    if (tag !== 'BR' && tag !== 'HR') copyInto(el, n, doc)
    out.appendChild(n)
  })
}

export function sanitizeHtml(html: string): string {
  if (!html) return ''
  const parsed = new DOMParser().parseFromString(`<body>${html}</body>`, 'text/html') // tài liệu "chết": không chạy script, không tải ảnh
  const holder = document.createElement('div')
  copyInto(parsed.body, holder, document)
  return holder.innerHTML.trim()
}

export function looksLikeHtml(s: string): boolean {
  return /<\/|<p>|<br|<img/i.test(s)
}

export function escapeHtml(s: string): string {
  return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;')
}

// Mô tả thuần văn bản của bản cũ → HTML đoạn văn
export function plainToHtml(text: string): string {
  const t = (text || '').replace(/\r\n/g, '\n').trim()
  if (!t) return ''
  return t
    .split(/\n{2,}/)
    .map((p) => `<p>${escapeHtml(p.trim()).replace(/\n/g, '<br>')}</p>`)
    .join('')
}

export function toRichHtml(s: string): string {
  return looksLikeHtml(s || '') ? sanitizeHtml(s) : sanitizeHtml(plainToHtml(s || ''))
}

// Chữ thuần (để tóm tắt trên thẻ sản phẩm / đếm độ dài)
export function textFromHtml(html: string): string {
  if (!html) return ''
  const parsed = new DOMParser().parseFromString(`<body>${looksLikeHtml(html) ? html : plainToHtml(html)}</body>`, 'text/html')
  parsed.body.querySelectorAll('script,style').forEach((n) => n.remove())
  parsed.body.querySelectorAll('p,br,li,h2,h3,h4,tr,blockquote,div').forEach((n) => n.append(' '))
  return (parsed.body.textContent || '').replace(/\s+/g, ' ').trim()
}

export function summaryFromHtml(html: string, max = 160): string {
  const t = textFromHtml(html)
  return t.length > max ? t.slice(0, max).trim() + '…' : t
}
