// SEO trong trình chỉnh sửa: cài đặt từng trang, cài đặt chung của website, xem trước kết quả Google, chấm điểm SEO theo luật (không cần AI).
import { collectBlockTexts } from './i18n'
import type { StoreBlock } from '../types/store'

// Cài đặt riêng của một trang (lưu trong bản nháp, sao sang bản công khai khi xuất bản)
export interface PageSettings {
  seo_title: string
  seo_description: string
  seo_image: string
  noindex: boolean // không cho Google lập chỉ mục trang này (và không đưa vào sitemap)
  hide_header: boolean // landing page: ẩn dải thông báo + đầu trang
  hide_footer: boolean // landing page: ẩn chân trang
}

// SEO mặc định của cả website
export interface SiteSeo {
  description: string
  image: string
  noindex: boolean
}

export const TITLE_MAX = 60
export const DESC_MAX = 155

export const emptySettings = (): PageSettings => ({ seo_title: '', seo_description: '', seo_image: '', noindex: false, hide_header: false, hide_footer: false })
export const emptySiteSeo = (): SiteSeo => ({ description: '', image: '', noindex: false })

export function parsePageSettings(raw: string | undefined | null): PageSettings {
  try {
    const v = JSON.parse(raw || '{}')
    return { ...emptySettings(), ...(v && typeof v === 'object' ? v : {}) }
  } catch {
    return emptySettings()
  }
}

export function parseSiteSeo(raw: string | undefined | null): SiteSeo {
  try {
    const v = JSON.parse(raw || '{}')
    return { ...emptySiteSeo(), ...(v && typeof v === 'object' ? v : {}) }
  } catch {
    return emptySiteSeo()
  }
}

// Văn bản của một trang (gom từ các section) — dùng để chấm điểm và đưa cho AI
export function pageText(blocks: StoreBlock[]): string {
  const parts: string[] = []
  for (const paths of Object.values(collectBlockTexts(blocks.filter((b) => b.enabled !== false)))) {
    for (const v of Object.values(paths)) parts.push(v.replace(/\s+/g, ' ').trim())
  }
  return parts.join('\n')
}

export function wordCount(text: string): number {
  return (text.match(/\p{L}[\p{L}\p{N}'’-]*/gu) || []).length
}

// Tiêu đề / mô tả thực tế khách thấy trên Google khi chưa tự đặt
export function effectiveTitle(s: PageSettings, pageTitle: string, siteTitle: string, isHome: boolean): string {
  return s.seo_title.trim() || (isHome ? siteTitle : `${pageTitle} – ${siteTitle}`)
}

export interface SeoCheck {
  id: string
  level: 'ok' | 'warn' | 'bad'
  text: string
}

export interface AuditInput {
  settings: PageSettings
  siteSeo: SiteSeo
  pageTitle: string
  siteTitle: string
  isHome: boolean
  blocks: StoreBlock[] // nội dung riêng của trang (không gồm đầu / chân trang)
  hasLogo: boolean
  otherTitles: string[] // tiêu đề thực tế của các trang khác (phát hiện trùng)
}

// Chấm điểm SEO của một trang theo luật phổ biến (độ dài tiêu đề / mô tả, có tiêu đề chính, đủ nội dung, ảnh có mô tả, ảnh chia sẻ, trùng tiêu đề...)
export function auditPage(i: AuditInput): { score: number; checks: SeoCheck[] } {
  const checks: SeoCheck[] = []
  const add = (id: string, level: SeoCheck['level'], text: string) => checks.push({ id, level, text })
  const title = effectiveTitle(i.settings, i.pageTitle, i.siteTitle, i.isHome)
  const tl = [...title].length
  if (!i.settings.seo_title.trim()) add('title', 'warn', 'Chưa đặt tiêu đề SEO riêng — đang dùng tên trang và tên website.')
  else if (tl < 25) add('title', 'warn', `Tiêu đề SEO hơi ngắn (${tl} ký tự). Nên dài 30–60 ký tự và có từ khoá chính.`)
  else if (tl > TITLE_MAX) add('title', 'warn', `Tiêu đề SEO dài ${tl} ký tự, Google có thể cắt bớt. Nên không quá ${TITLE_MAX}.`)
  else add('title', 'ok', `Tiêu đề SEO ${tl} ký tự — độ dài tốt.`)

  const desc = i.settings.seo_description.trim() || i.siteSeo.description.trim()
  const dl = [...desc].length
  if (!desc) add('desc', 'bad', 'Chưa có mô tả SEO — Google sẽ tự chọn một đoạn chữ bất kỳ trong trang.')
  else if (!i.settings.seo_description.trim()) add('desc', 'warn', 'Trang đang dùng mô tả mặc định của website. Mỗi trang nên có mô tả riêng.')
  else if (dl < 70) add('desc', 'warn', `Mô tả SEO hơi ngắn (${dl} ký tự). Nên dài 110–155 ký tự.`)
  else if (dl > 165) add('desc', 'warn', `Mô tả SEO dài ${dl} ký tự, Google có thể cắt bớt. Nên không quá ${DESC_MAX}.`)
  else add('desc', 'ok', `Mô tả SEO ${dl} ký tự — độ dài tốt.`)

  const hasHeading = i.blocks.some((b) => b.enabled !== false && ((b.type === 'hero' && String(b.data.title || '').trim()) || (b.type === 'richtext' && String(b.data.heading || '').trim())))
  add('h1', hasHeading ? 'ok' : 'warn', hasHeading ? 'Trang có tiêu đề chính.' : 'Trang chưa có tiêu đề chính. Thêm section Hero hoặc Văn bản có tiêu đề.')

  const words = wordCount(pageText(i.blocks))
  if (words < 60) add('words', i.isHome ? 'warn' : 'bad', `Nội dung chỉ có ${words} từ — quá ít để xếp hạng. Nên từ 150 từ trở lên.`)
  else if (words < 150) add('words', 'warn', `Nội dung có ${words} từ. Thêm nội dung (từ 150 từ) sẽ tốt hơn cho SEO.`)
  else add('words', 'ok', `Nội dung có ${words} từ.`)

  const noAlt = i.blocks.filter((b) => b.enabled !== false && b.type === 'image' && String(b.data.image_url || '').trim() && !String(b.data.alt || '').trim()).length
  add('alt', noAlt ? 'warn' : 'ok', noAlt ? `${noAlt} ảnh chưa có mô tả (alt). Thêm mô tả để Google hiểu ảnh.` : 'Ảnh đều có mô tả hoặc trang không có ảnh lớn cần mô tả.')

  const hasShare = !!(i.settings.seo_image.trim() || i.siteSeo.image.trim() || i.hasLogo)
  add('share', hasShare ? 'ok' : 'warn', hasShare ? 'Có ảnh chia sẻ khi gửi link qua Facebook / Zalo.' : 'Chưa có ảnh chia sẻ. Đặt ảnh chia sẻ (hoặc logo) để link đẹp hơn khi chia sẻ.')

  if (i.settings.noindex) add('noindex', 'warn', 'Trang đang bị ẩn khỏi Google (noindex) và không có trong sitemap.')
  const dup = i.otherTitles.some((t) => t.trim().toLowerCase() === title.trim().toLowerCase())
  if (dup) add('dup', 'warn', 'Trùng tiêu đề với một trang khác. Mỗi trang nên có tiêu đề riêng.')

  const weight = { ok: 1, warn: 0.5, bad: 0 }
  const score = Math.round((checks.reduce((n, c) => n + weight[c.level], 0) / checks.length) * 100)
  return { score, checks }
}
