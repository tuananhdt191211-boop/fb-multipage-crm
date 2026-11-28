// Dịch website bằng AI: danh sách câu cần dịch, đọc/ghi bản dịch trong SiteI18n, và hàm chạy dịch
// dùng chung cho tab "Ngôn ngữ" và cho nút "Xuất bản" (tự dịch nốt câu còn thiếu trước khi xuất bản).
import { api } from './api'
import { UI_EN } from './i18nEn'
import { UI_VI, collectBlockTexts, collectProductTexts, type SiteI18n, type UiKey } from './i18n'
import { langsOf, LANG_VI } from './markets'
import type { PublicProduct, StoreBlock } from '../types/store'

export interface TrRow {
  key: string // b|blockId|path, p|id|name, p|id|o|chữ gốc (phân loại), u|key
  group: 'Trang' | 'Sản phẩm' | 'Giao diện'
  src: string
}

// Mọi câu tiếng Việt cần có bản dịch
export function translationRows(blocks: StoreBlock[], products: PublicProduct[]): TrRow[] {
  const out: TrRow[] = []
  for (const [bid, paths] of Object.entries(collectBlockTexts(blocks))) {
    for (const [p, v] of Object.entries(paths)) out.push({ key: `b|${bid}|${p}`, group: 'Trang', src: v })
  }
  for (const [pid, e] of Object.entries(collectProductTexts(products))) {
    if (e.name) out.push({ key: `p|${pid}|name`, group: 'Sản phẩm', src: e.name })
    if (e.description) out.push({ key: `p|${pid}|description`, group: 'Sản phẩm', src: e.description })
    for (const o of e.options || []) out.push({ key: `p|${pid}|o|${o}`, group: 'Sản phẩm', src: o })
  }
  for (const [k, v] of Object.entries(UI_VI)) out.push({ key: `u|${k}`, group: 'Giao diện', src: v })
  return out
}

// Khoá: kind|a|b[|c...] — phần sau "b" của khoá phân loại (p|id|o|chữ gốc) là chữ gốc, có thể chứa dấu "|"
export function parseTrKey(key: string): { kind: string; a: string; b: string; c: string } {
  const [kind = '', a = '', b = '', ...rest] = key.split('|')
  return { kind, a, b, c: rest.join('|') }
}

// Bản dịch đã lưu của shop (không tính bản tiếng Anh dựng sẵn)
export function savedTranslation(i18n: SiteI18n, lang: string, key: string): string {
  const tr = i18n.tr[lang]
  if (!tr) return ''
  const { kind, a, b, c } = parseTrKey(key)
  if (kind === 'b') return tr.b?.[a]?.[b] || ''
  if (kind === 'p') return b === 'o' ? tr.p?.[a]?.o?.[c] || '' : (tr.p?.[a] as any)?.[b] || ''
  return tr.ui?.[a] || ''
}

// Bản dịch đang dùng: bản của shop, nếu chưa có thì chữ cố định tiếng Anh dựng sẵn
export function getTranslation(i18n: SiteI18n, lang: string, key: string): string {
  const own = savedTranslation(i18n, lang, key)
  if (own) return own
  const { kind, a } = parseTrKey(key)
  return kind === 'u' && lang === 'en' ? UI_EN[a as UiKey] || '' : ''
}

// Ghi nhiều câu dịch của 1 ngôn ngữ, trả về SiteI18n mới
export function withTranslations(i18n: SiteI18n, lang: string, entries: Record<string, string>): SiteI18n {
  const tr = JSON.parse(JSON.stringify(i18n.tr[lang] || {}))
  for (const [key, value] of Object.entries(entries)) {
    const { kind, a, b, c } = parseTrKey(key)
    if (kind === 'b') ((tr.b ||= {})[a] ||= {})[b] = value
    else if (kind === 'p' && b === 'o') (((tr.p ||= {})[a] ||= {}).o ||= {})[c] = value
    else if (kind === 'p') ((tr.p ||= {})[a] ||= {})[b] = value
    else (tr.ui ||= {})[a] = value
  }
  return { ...i18n, tr: { ...i18n.tr, [lang]: tr } }
}

// Số câu còn thiếu bản dịch (cộng tất cả ngôn ngữ đang bật)
export function missingCount(i18n: SiteI18n, rows: TrRow[]): number {
  let n = 0
  for (const lang of langsOf(i18n.markets)) for (const r of rows) if (!getTranslation(i18n, lang, r.key)) n++
  return n
}

export interface TranslateResult {
  failed: number // số câu AI vẫn chưa dịch được
  reason?: string // lý do thật từ máy chủ (chưa cấu hình AI, hết lượt gọi...)
  i18n: SiteI18n
}

// Dịch các câu còn thiếu (hoặc tất cả nếu redo). Gọi máy chủ từng lô nhỏ; câu thất bại được thử lại thêm tối đa 2 vòng.
export async function translateRows(opts: {
  siteId: number | string
  i18n: SiteI18n
  rows: TrRow[]
  redo?: boolean
  onI18n: (v: SiteI18n) => void
  onProgress?: (text: string) => void
}): Promise<TranslateResult> {
  let cur = opts.i18n
  let reason = ''
  let failedNow = 0
  const langs = langsOf(cur.markets)
  const CHUNK = 40
  for (const [li, lang] of langs.entries()) {
    let todo = opts.rows.filter((r) => opts.redo || !getTranslation(cur, lang, r.key))
    const total = todo.length
    let lastFailed = Infinity
    for (let round = 0; round < 3 && todo.length; round++) {
      let failedRound = 0
      for (let i = 0; i < todo.length; i += CHUNK) {
        const chunk = todo.slice(i, i + CHUNK)
        const done = total - todo.length + Math.min(i + chunk.length, todo.length)
        opts.onProgress?.(`${LANG_VI[lang] || lang} (${li + 1}/${langs.length}) · ${done}/${total}`)
        const texts: Record<string, string> = {}
        for (const r of chunk) texts[r.key] = r.src
        const { data } = await api.post(`/storefront/sites/${opts.siteId}/translate`, { lang, texts }, { timeout: 180000 })
        cur = withTranslations(cur, lang, (data.translations || {}) as Record<string, string>)
        opts.onI18n(cur)
        failedRound += Number(data.failed || 0)
        if (data.reason) reason = String(data.reason)
      }
      // Chỉ thử lại phần còn thiếu, và chỉ khi vòng vừa rồi có tiến triển
      todo = opts.rows.filter((r) => !getTranslation(cur, lang, r.key))
      if (!failedRound || failedRound >= lastFailed) break
      lastFailed = failedRound
    }
    failedNow += todo.length
  }
  return { failed: failedNow, reason: reason || undefined, i18n: cur }
}
