// Kiểu chữ tuỳ chỉnh (giống Shopify): chọn trên giao diện -> sinh CSS / HTML tương ứng.
import type { TextStyle } from '../types/store'

export const fontChoices = [
  { v: '', t: 'Mặc định' },
  { v: "'Be Vietnam Pro', sans-serif", t: 'Be Vietnam Pro' },
  { v: 'Arial, Helvetica, sans-serif', t: 'Arial' },
  { v: "Georgia, 'Times New Roman', serif", t: 'Georgia (có chân)' },
  { v: "'Times New Roman', Times, serif", t: 'Times New Roman' },
  { v: "'Courier New', monospace", t: 'Courier (đơn cách)' }
]

export const tagChoices = [
  { v: 'h1', t: 'H1' },
  { v: 'h2', t: 'H2' },
  { v: 'h3', t: 'H3' },
  { v: 'h4', t: 'H4' },
  { v: 'p', t: 'P' },
  { v: 'div', t: 'DIV' }
] as const

export function pathKey(path: (string | number)[]): string {
  return path.join('.')
}

const clampNum = (n: unknown, min: number, max: number) => {
  const v = Number(n)
  return Number.isFinite(v) && v > 0 ? Math.min(max, Math.max(min, v)) : 0
}

const safeColor = (c?: string) => (c && /^(#[0-9a-f]{3,8}|rgba?\([\d\s.,%]+\)|[a-z]+)$/i.test(c.trim()) ? c.trim() : '')

// Khai báo CSS tự viết: "prop: value; ..." -> chỉ giữ cú pháp an toàn
export function parseDecls(css?: string): [string, string][] {
  const out: [string, string][] = []
  for (const part of String(css || '').slice(0, 2000).split(';')) {
    const i = part.indexOf(':')
    if (i < 1) continue
    const prop = part.slice(0, i).trim().toLowerCase()
    const val = part.slice(i + 1).trim()
    if (!/^-{0,2}[a-z][a-z0-9-]*$/.test(prop) || !val) continue
    if (/[{}<>]|expression\s*\(|javascript:|@import/i.test(val)) continue
    out.push([prop, val])
  }
  return out
}

// CSS (dạng khai báo) mà lựa chọn sinh ra — dùng để hiển thị "code" và để render
export function styleDecls(ts?: TextStyle): { base: [string, string][]; desktop?: string; mobile?: string } {
  const base: [string, string][] = []
  if (!ts) return { base }
  const color = safeColor(ts.color)
  if (color) base.push(['color', color])
  if (ts.weight) base.push(['font-weight', ts.weight])
  if (ts.italic) base.push(['font-style', 'italic'])
  if (ts.underline) base.push(['text-decoration', 'underline'])
  if (ts.upper) base.push(['text-transform', 'uppercase'])
  if (ts.align) base.push(['text-align', ts.align])
  const lh = clampNum(ts.lh, 0.8, 3)
  if (lh) base.push(['line-height', String(lh)])
  if (ts.ls !== undefined && ts.ls !== null && String(ts.ls) !== '' && Number.isFinite(Number(ts.ls))) {
    base.push(['letter-spacing', `${Math.min(20, Math.max(-5, Number(ts.ls)))}px`])
  }
  if (ts.font && fontChoices.some((f) => f.v === ts.font)) base.push(['font-family', ts.font])
  const d = clampNum(ts.size, 8, 160)
  let m = clampNum(ts.size_m, 8, 120)
  // Không nhập cỡ điện thoại -> tự thu nhỏ chữ lớn cho vừa màn hình
  if (d && !m) m = d > 28 ? Math.round(d * 0.75) : d
  return { base, desktop: d ? `${d}px` : undefined, mobile: m ? `${m}px` : undefined }
}

// Style + class gắn lên phần tử chữ
export function textStyleAttrs(ts?: TextStyle): { style: Record<string, string>; cls: string[] } {
  const style: Record<string, string> = {}
  const cls: string[] = []
  if (!ts) return { style, cls }
  const { base, desktop, mobile } = styleDecls(ts)
  for (const [k, v] of base) style[k] = v
  if (desktop) {
    style['--pp-fs'] = desktop
    cls.push('pp-fx-fsd')
  }
  if (mobile) {
    style['--pp-fs-m'] = mobile
    cls.push('pp-fx-fsm')
  }
  for (const [k, v] of parseDecls(ts.css)) style[k] = v
  return { style, cls }
}

// Đoạn code hiển thị cho người dùng
export function cssPreview(ts?: TextStyle): string {
  const { base, desktop, mobile } = styleDecls(ts)
  const lines: string[] = []
  const extra = parseDecls(ts?.css)
  for (const [k, v] of base) lines.push(`  ${k}: ${v};`)
  for (const [k, v] of extra) lines.push(`  ${k}: ${v};`)
  let out = `.chu-nay {\n${desktop ? `  font-size: ${desktop};\n` : ''}${lines.join('\n')}${lines.length ? '\n' : ''}}`
  if (mobile && mobile !== desktop) out += `\n\n/* Điện thoại */\n@media (max-width: 767px) {\n  .chu-nay { font-size: ${mobile}; }\n}`
  return out
}

export function htmlPreview(tag: string, text: string, ts?: TextStyle): string {
  const { style } = textStyleAttrs(ts)
  const inline = Object.entries(style)
    .filter(([k]) => !k.startsWith('--'))
    .map(([k, v]) => `${k}: ${v}`)
    .join('; ')
  const size = styleDecls(ts).desktop
  const all = [size ? `font-size: ${size}` : '', inline].filter(Boolean).join('; ')
  const esc = (s: string) => s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;')
  return `<${tag}${all ? ` style="${esc(all)}"` : ''}>${esc(text)}</${tag}>`
}

// CSS tuỳ chỉnh của section, bọc trong selector của section (CSS lồng nhau)
export function scopedSectionCss(blockId: string, css?: string): string {
  let c = String(css || '').slice(0, 20000)
  if (!c.trim()) return ''
  c = c.replace(/<\/?style/gi, '').replace(/@import[^;]*;?/gi, '').replace(/expression\s*\(/gi, '')
  const id = blockId.replace(/[^a-zA-Z0-9_-]/g, '')
  return `[data-pp-sec="${id}"] {\n${c}\n}`
}

export function cleanTextStyle(ts: TextStyle): TextStyle | undefined {
  const out: Record<string, unknown> = {}
  for (const [k, v] of Object.entries(ts)) {
    if (v === undefined || v === null || v === '' || v === false) continue
    out[k] = v
  }
  return Object.keys(out).length ? (out as TextStyle) : undefined
}
