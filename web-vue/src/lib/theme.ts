// Kiểu chữ toàn trang (Typography) -> CSS áp dụng trong khung .pp-store
import { fontStack, isKnownFont } from './fonts'
import type { SiteTheme, StoreBlock, ThemeTag, ThemeTypo } from '../types/store'

export const themeTags: { tag: ThemeTag; label: string; sample: string }[] = [
  { tag: 'h1', label: 'Tiêu đề H1', sample: 'Tiêu đề lớn nhất' },
  { tag: 'h2', label: 'Tiêu đề H2', sample: 'Tiêu đề mục' },
  { tag: 'h3', label: 'Tiêu đề H3', sample: 'Tiêu đề nhỏ' },
  { tag: 'h4', label: 'Tiêu đề H4', sample: 'Tiêu đề phụ' },
  { tag: 'p', label: 'Đoạn văn P', sample: 'Đoạn văn mô tả sản phẩm, giới thiệu cửa hàng.' }
]

export const themePresets: { name: string; desc: string; theme: SiteTheme }[] = [
  { name: 'Mặc định', desc: 'Be Vietnam Pro', theme: {} },
  { name: 'Hiện đại', desc: 'Montserrat · Inter', theme: { heading_font: 'Montserrat', body_font: 'Inter', tags: { h1: { weight: '800' }, h2: { weight: '800' } } } },
  { name: 'Sang trọng', desc: 'Playfair Display · Lora', theme: { heading_font: 'Playfair Display', body_font: 'Lora', tags: { h1: { weight: '700' }, h2: { weight: '700' } } } },
  { name: 'Mỹ phẩm', desc: 'Cormorant · Be Vietnam Pro', theme: { heading_font: 'Cormorant Garamond', body_font: 'Be Vietnam Pro', tags: { h1: { weight: '600', ls: 0.5 }, h2: { weight: '600', ls: 0.5 } } } },
  { name: 'Thân thiện', desc: 'Nunito · Nunito', theme: { heading_font: 'Nunito', body_font: 'Nunito', tags: { h1: { weight: '900' }, h2: { weight: '800' } } } },
  { name: 'Mạnh mẽ', desc: 'Oswald · Roboto', theme: { heading_font: 'Oswald', body_font: 'Roboto', tags: { h1: { weight: '700', upper: true }, h2: { weight: '600', upper: true } } } },
  { name: 'Cổ điển', desc: 'Merriweather · Open Sans', theme: { heading_font: 'Merriweather', body_font: 'Open Sans' } }
]

function decls(t: ThemeTypo | undefined, withSize: boolean): string[] {
  if (!t) return []
  const out: string[] = []
  if (isKnownFont(t.font)) out.push(`font-family: ${fontStack(t.font)}`)
  if (withSize && t.size) out.push(`font-size: ${t.size}px`)
  if (t.weight) out.push(`font-weight: ${t.weight}`)
  if (t.lh) out.push(`line-height: ${t.lh}`)
  if (t.ls !== undefined && t.ls !== null && Number.isFinite(Number(t.ls))) out.push(`letter-spacing: ${t.ls}px`)
  if (t.upper) out.push('text-transform: uppercase')
  if (t.color && /^#[0-9a-f]{6}$/i.test(t.color)) out.push(`color: ${t.color}`)
  return out
}

// Selector cho từng thẻ. Đoạn văn bỏ qua nhãn nhỏ (text-xs / chữ in hoa) để không làm vỡ bố cục.
const sel: Record<ThemeTag, string> = {
  h1: '.pp-store h1',
  h2: '.pp-store h2',
  h3: '.pp-store h3',
  h4: '.pp-store h4',
  p: '.pp-store p:not(.text-xs):not(.uppercase)'
}

export function themeCss(theme?: SiteTheme | null): string {
  if (!theme) return ''
  const parts: string[] = []
  if (isKnownFont(theme.body_font)) parts.push(`.pp-store { font-family: ${fontStack(theme.body_font)}; }`)
  if (isKnownFont(theme.heading_font)) parts.push(`.pp-store :is(h1, h2, h3, h4), .pp-store .pp-hfont { font-family: ${fontStack(theme.heading_font)}; }`)
  const mobile: string[] = []
  for (const { tag } of themeTags) {
    const t = theme.tags?.[tag]
    const d = decls(t, true)
    if (d.length) parts.push(`${sel[tag]} {\n  ${d.join(';\n  ')};\n}`)
    const m = t?.size_m || (t?.size && t.size > 28 ? Math.round(t.size * 0.75) : t?.size)
    if (m && m !== t?.size) mobile.push(`  ${sel[tag]} { font-size: ${m}px; }`)
  }
  if (mobile.length) parts.push(`/* Điện thoại */\n@container (max-width: 47.98rem) {\n${mobile.join('\n')}\n}`)
  return parts.join('\n\n')
}

export function parseTheme(raw?: string | null): SiteTheme {
  try {
    const v = JSON.parse(raw || '{}')
    return v && typeof v === 'object' && !Array.isArray(v) ? (v as SiteTheme) : {}
  } catch {
    return {}
  }
}

// Mọi font đang dùng (toàn trang + kiểu chữ riêng từng đoạn) để tải từ Google Fonts
export function usedFonts(theme: SiteTheme | null | undefined, blocks: StoreBlock[]): Set<string> {
  const s = new Set<string>()
  if (theme?.heading_font) s.add(theme.heading_font)
  if (theme?.body_font) s.add(theme.body_font)
  for (const t of Object.values(theme?.tags || {})) if (t?.font) s.add(t.font)
  for (const b of blocks) {
    for (const ts of Object.values((b.data?._text || {}) as Record<string, { font?: string }>)) if (ts?.font) s.add(ts.font)
  }
  return s
}
