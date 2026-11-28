// Đa ngôn ngữ + đa tiền tệ cho giao diện QUẢN TRỊ (web công khai của khách có bộ riêng: lib/i18n.ts).
//
// Giao diện được viết bằng tiếng Việt. Script scripts/extract-ui-strings.mjs lập "danh mục" các câu chữ giao diện
// (và bản tiếng Anh nằm trong src/i18n/en.json). Ở đây trình duyệt dịch TẠI CHỖ các nút chữ trên trang quản trị:
//   - chỉ dịch khi nguyên văn một đoạn chữ / thuộc tính (placeholder, title...) trùng một câu trong danh mục
//     (câu có biến được ghi dạng "Đã chọn {0} trang" và so khớp bằng mẫu) → nội dung của khách (tin nhắn, tên,
//     sản phẩm...) không bao giờ bị đụng tới;
//   - vùng không được dịch đánh dấu bằng data-i18n="skip" (data-i18n="on" để bật lại bên trong);
//   - MutationObserver dịch ngay phần mới xuất hiện (trước khi trình duyệt vẽ nên không bị chớp chữ Việt);
//   - chữ gốc được nhớ lại để đổi qua lại giữa các ngôn ngữ / quay về tiếng Việt.
// Tiếng Anh nằm sẵn trong gói web; ngôn ngữ khác lấy từ máy chủ (bản AI dịch dùng chung) và được lưu cục bộ.
import { reactive } from 'vue'
import type { RouteLocationNormalized, Router } from 'vue-router'
import { api } from './api'
import { formatVnd } from './format'
import { onCustomDomain } from './storeHost'

// ----------------------------------------------------------------- Ngôn ngữ & tiền tệ

export interface UiLang {
  code: string
  name: string // tên bằng chính ngôn ngữ đó
  locale: string // định dạng ngày / số
  currency: string // tiền tệ mặc định khi chọn ngôn ngữ này
}

// Phải khớp uiLangNames ở backend (ui_i18n.go)
export const UI_LANGS: UiLang[] = [
  { code: 'vi', name: 'Tiếng Việt', locale: 'vi-VN', currency: 'VND' },
  { code: 'en', name: 'English', locale: 'en-US', currency: 'USD' },
  { code: 'de', name: 'Deutsch', locale: 'de-DE', currency: 'EUR' },
  { code: 'fr', name: 'Français', locale: 'fr-FR', currency: 'EUR' },
  { code: 'es', name: 'Español', locale: 'es-ES', currency: 'EUR' },
  { code: 'it', name: 'Italiano', locale: 'it-IT', currency: 'EUR' },
  { code: 'pt', name: 'Português', locale: 'pt-PT', currency: 'EUR' },
  { code: 'nl', name: 'Nederlands', locale: 'nl-NL', currency: 'EUR' },
  { code: 'pl', name: 'Polski', locale: 'pl-PL', currency: 'PLN' },
  { code: 'cs', name: 'Čeština', locale: 'cs-CZ', currency: 'CZK' },
  { code: 'sk', name: 'Slovenčina', locale: 'sk-SK', currency: 'EUR' },
  { code: 'sv', name: 'Svenska', locale: 'sv-SE', currency: 'SEK' },
  { code: 'da', name: 'Dansk', locale: 'da-DK', currency: 'DKK' },
  { code: 'fi', name: 'Suomi', locale: 'fi-FI', currency: 'EUR' },
  { code: 'nb', name: 'Norsk', locale: 'nb-NO', currency: 'NOK' },
  { code: 'el', name: 'Ελληνικά', locale: 'el-GR', currency: 'EUR' },
  { code: 'hu', name: 'Magyar', locale: 'hu-HU', currency: 'HUF' },
  { code: 'ro', name: 'Română', locale: 'ro-RO', currency: 'RON' },
  { code: 'bg', name: 'Български', locale: 'bg-BG', currency: 'EUR' },
  { code: 'hr', name: 'Hrvatski', locale: 'hr-HR', currency: 'EUR' },
  { code: 'sl', name: 'Slovenščina', locale: 'sl-SI', currency: 'EUR' },
  { code: 'et', name: 'Eesti', locale: 'et-EE', currency: 'EUR' },
  { code: 'lv', name: 'Latviešu', locale: 'lv-LV', currency: 'EUR' },
  { code: 'lt', name: 'Lietuvių', locale: 'lt-LT', currency: 'EUR' },
  { code: 'uk', name: 'Українська', locale: 'uk-UA', currency: 'UAH' },
  { code: 'zh', name: '简体中文', locale: 'zh-CN', currency: 'CNY' },
  { code: 'ja', name: '日本語', locale: 'ja-JP', currency: 'JPY' },
  { code: 'ko', name: '한국어', locale: 'ko-KR', currency: 'KRW' },
  { code: 'th', name: 'ไทย', locale: 'th-TH', currency: 'THB' },
  { code: 'id', name: 'Bahasa Indonesia', locale: 'id-ID', currency: 'IDR' },
  { code: 'ms', name: 'Bahasa Melayu', locale: 'ms-MY', currency: 'MYR' }
]
const LANG_BY_CODE = new Map(UI_LANGS.map((l) => [l.code, l]))

// Tiền tệ cho phép chọn hiển thị (chỉ hiện những loại có tỉ giá)
export const UI_CURRENCIES = [
  'VND', 'USD', 'EUR', 'GBP', 'AUD', 'CAD', 'NZD', 'SGD', 'HKD', 'JPY', 'KRW', 'CNY', 'TWD', 'THB', 'IDR', 'MYR', 'PHP', 'INR',
  'CHF', 'SEK', 'NOK', 'DKK', 'PLN', 'CZK', 'HUF', 'RON', 'UAH', 'TRY', 'AED', 'MXN', 'BRL'
]

export const ui = reactive({
  lang: 'vi',
  currency: 'VND',
  currencyAuto: true, // true = tiền tệ theo ngôn ngữ
  rates: {} as Record<string, number>, // 1 VND = ? ngoại tệ
  ratesUpdated: '',
  status: 'idle' as 'idle' | 'loading' | 'filling' | 'partial',
  count: 0, // số câu đã có bản dịch của ngôn ngữ đang chọn
  total: 0 // tổng số câu trong danh mục
})

const K_LANG = 'pp_ui_lang'
const K_CUR = 'pp_ui_cur'
const K_RATES = 'pp_ui_rates'
const K_TR = (lang: string) => `pp_ui_tr1_${lang}`
const K_FILL = (lang: string) => `pp_ui_fill_${lang}`

function lsGet(k: string): string | null {
  try {
    return localStorage.getItem(k)
  } catch {
    return null
  }
}
function lsSet(k: string, v: string | null) {
  try {
    if (v === null) localStorage.removeItem(k)
    else localStorage.setItem(k, v)
  } catch {
    /* bộ nhớ đầy hoặc bị chặn: bỏ qua */
  }
}

export function uiLocaleTag(): string {
  return LANG_BY_CODE.get(ui.lang)?.locale || 'vi-VN'
}

function detectLang(): string {
  const stored = lsGet(K_LANG)
  if (stored && LANG_BY_CODE.has(stored)) return stored
  let pick = ''
  if (lsGet('access_token')) {
    pick = 'vi' // người dùng cũ đã đăng nhập từ trước: giữ tiếng Việt như mọi khi
  } else {
    const list = (navigator.languages && navigator.languages.length ? [...navigator.languages] : [navigator.language]).map((l) => String(l).toLowerCase().split('-')[0]!)
    const alias: Record<string, string> = { no: 'nb', nn: 'nb', in: 'id', fil: 'en' }
    if (list.includes('vi')) pick = 'vi'
    else pick = list.map((c) => alias[c] || c).find((c) => LANG_BY_CODE.has(c)) || 'en'
  }
  lsSet(K_LANG, pick)
  return pick
}

// ----------------------------------------------------------------- Tiền tệ

let ratesPromise: Promise<void> | null = null
export function ensureRates(): Promise<void> {
  if (Object.keys(ui.rates).length) return Promise.resolve()
  try {
    const c = JSON.parse(lsGet(K_RATES) || 'null')
    if (c && c.rates && Date.now() - c.at < 6 * 3600e3) {
      ui.rates = c.rates
      ui.ratesUpdated = c.updated || ''
      return Promise.resolve()
    }
  } catch {
    /* bỏ qua */
  }
  if (!ratesPromise) {
    ratesPromise = api
      .get('/storefront/public/rates')
      .then(({ data }) => {
        ui.rates = data.rates || {}
        ui.ratesUpdated = data.updated_at || ''
        lsSet(K_RATES, JSON.stringify({ at: Date.now(), rates: ui.rates, updated: ui.ratesUpdated }))
      })
      .catch(() => {
        ratesPromise = null // thử lại lần sau
      })
  }
  return ratesPromise
}

export function setUiCurrency(code: string) {
  if (code === 'auto') {
    ui.currencyAuto = true
    ui.currency = LANG_BY_CODE.get(ui.lang)?.currency || 'VND'
    lsSet(K_CUR, null)
  } else if (/^[A-Z]{3}$/.test(code)) {
    ui.currencyAuto = false
    ui.currency = code
    lsSet(K_CUR, code)
  }
  if (ui.currency !== 'VND') void ensureRates()
}

// Tên tiền tệ theo ngôn ngữ đang chọn (vd. "US Dollar")
export function currencyName(code: string): string {
  try {
    return new Intl.DisplayNames([uiLocaleTag()], { type: 'currency' }).of(code) || code
  } catch {
    return code
  }
}

// Giá trong hệ thống luôn là VND; các tiền tệ khác chỉ là quy đổi tham khảo theo tỉ giá hằng ngày.
export function adminMoney(vnd: number): string {
  const rate = ui.rates[ui.currency]
  if (ui.currency === 'VND' || !rate) return formatVnd(vnd)
  try {
    return '≈ ' + new Intl.NumberFormat(uiLocaleTag(), { style: 'currency', currency: ui.currency }).format(Math.max(0, vnd || 0) * rate)
  } catch {
    return formatVnd(vnd)
  }
}

// Gợi ý (tooltip) hiện giá gốc VND khi đang xem bằng tiền tệ khác
export function adminMoneyTitle(vnd: number): string | undefined {
  return ui.currency !== 'VND' && ui.rates[ui.currency] ? formatVnd(vnd) : undefined
}

// ----------------------------------------------------------------- Bộ dịch chữ

const VI_RE = /[àáạảãâầấậẩẫăằắặẳẵèéẹẻẽêềếệểễìíịỉĩòóọỏõôồốộổỗơờớợởỡùúụủũưừứựửữỳýỵỷỹđ]/i
const ATTRS = ['placeholder', 'title', 'alt', 'aria-label', 'label', 'data-placeholder']
const SKIP_TAGS = new Set(['SCRIPT', 'STYLE', 'TEXTAREA', 'NOSCRIPT', 'TITLE'])

interface Pat {
  re: RegExp
  groups: number[] // số thứ tự {n} theo vị trí xuất hiện trong câu tiếng Việt
  out: string
  prefix: string
  suffix: string
  pieces: string[]
}

let exact = new Map<string, string>()
let pats: Pat[] = []
const miss = new Set<string>() // chữ chắc chắn không có trong danh mục (đỡ thử lại)
let hasDict = false
let dictVer = 0 // tăng mỗi khi từ điển đổi

const norm = (s: string) => s.replace(/\s+/g, ' ').trim()

function compilePat(vi: string, out: string): Pat {
  const groups = [...vi.matchAll(/\{(\d+)\}/g)].map((m) => Number(m[1]))
  const pieces = vi.split(/\{\d+\}/)
  const esc = (s: string) => s.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
  const src = '^' + pieces.map((p, i) => esc(p) + (i < groups.length ? '([\\s\\S]{1,120}?)' : '')).join('') + '$'
  return { re: new RegExp(src), groups, out, prefix: pieces[0]!, suffix: pieces[pieces.length - 1]!, pieces: pieces.filter(Boolean) }
}

function buildDict(en: Record<string, string> | null, tr: Record<string, string> | null) {
  exact = new Map()
  pats = []
  miss.clear()
  hasDict = false
  dictVer++
  if (!en && !tr) return
  hasDict = true
  const all: Record<string, string> = { ...(en || {}), ...(tr || {}) }
  for (const [vi, out] of Object.entries(all)) {
    if (!out || typeof out !== 'string') continue
    if (/\{\d+\}/.test(vi)) pats.push(compilePat(vi, out))
    else exact.set(vi, out)
  }
  // Mẫu cụ thể (nhiều chữ cố định) thử trước
  pats.sort((a, b) => b.pieces.join('').length - a.pieces.join('').length)
}

function renderPat(p: Pat, m: RegExpExecArray): string {
  return p.out.replace(/\{(\d+)\}/g, (_, n: string) => {
    const i = p.groups.indexOf(Number(n))
    const v = i >= 0 ? m[i + 1] ?? '' : ''
    return exact.get(norm(v)) ?? v // phần biến cũng là chữ giao diện (vd. "Đang chạy") thì dịch luôn
  })
}

// Trả về chữ đã dịch (giữ khoảng trắng đầu/cuối), hoặc null nếu không phải chữ giao diện
export function translateString(raw: string): string | null {
  if (!hasDict) return null
  const n = norm(raw)
  if (n.length < 2 || !VI_RE.test(n)) return null
  let out = exact.get(n)
  if (out === undefined && !miss.has(n)) {
    for (const p of pats) {
      if (!n.startsWith(p.prefix) || !n.endsWith(p.suffix) || p.pieces.some((x) => !n.includes(x))) continue
      const m = p.re.exec(n)
      if (m) {
        out = renderPat(p, m)
        break
      }
    }
    if (out === undefined) {
      if (miss.size > 6000) miss.clear()
      miss.add(n)
    }
  }
  if (out === undefined) return null
  const lead = /^\s*/.exec(raw)![0]
  const trail = /\s*$/.exec(raw)![0]
  return lead + out + trail
}

// ----------------------------------------------------------------- Áp bản dịch lên trang

interface Rec {
  orig: string
  out: string
  v: number // phiên bản từ điển lúc dịch (không đổi thì khỏi dịch lại)
}
const textRecs = new WeakMap<Text, Rec>()
const attrRecs = new WeakMap<Element, Record<string, Rec>>()
let titleRec: Rec | null = null

function doText(node: Text) {
  const cur = node.nodeValue ?? ''
  const rec = textRecs.get(node)
  if (rec && cur === rec.out) {
    // chữ do bộ dịch ghi: dịch lại từ bản gốc khi từ điển đổi (đổi ngôn ngữ / có thêm bản dịch mới)
    if (rec.v === dictVer) return
    const out = translateString(rec.orig)
    if (out === null) {
      node.nodeValue = rec.orig
      textRecs.delete(node)
      return
    }
    rec.v = dictVer
    if (out !== rec.out) {
      rec.out = out
      node.nodeValue = out
    }
    return
  }
  // chữ mới (hoặc Vue vừa ghi đè): coi là bản gốc
  const out = translateString(cur)
  if (out === null) {
    if (rec) textRecs.delete(node)
    return
  }
  textRecs.set(node, { orig: cur, out, v: dictVer })
  node.nodeValue = out
}

function doAttr(el: Element, name: string) {
  const cur = el.getAttribute(name)
  if (cur === null) return
  let recs = attrRecs.get(el)
  const rec = recs?.[name]
  if (rec && cur === rec.out) {
    if (rec.v === dictVer) return
    const out = translateString(rec.orig)
    if (out === null) {
      el.setAttribute(name, rec.orig)
      delete recs![name]
      return
    }
    rec.v = dictVer
    if (out !== rec.out) {
      rec.out = out
      el.setAttribute(name, out)
    }
    return
  }
  const out = translateString(cur)
  if (out === null) {
    if (rec) delete recs![name]
    return
  }
  if (!recs) {
    recs = {}
    attrRecs.set(el, recs)
  }
  recs[name] = { orig: cur, out, v: dictVer }
  el.setAttribute(name, out)
}

function doTitle() {
  const cur = document.title
  if (titleRec && cur === titleRec.out) {
    if (titleRec.v === dictVer) return
    const out = translateString(titleRec.orig)
    if (out === null) {
      document.title = titleRec.orig
      titleRec = null
      return
    }
    titleRec.v = dictVer
    if (out !== titleRec.out) {
      titleRec.out = out
      document.title = out
    }
    return
  }
  const out = translateString(cur)
  if (out === null) {
    titleRec = null
    return
  }
  titleRec = { orig: cur, out, v: dictVer }
  document.title = out
}

// data-i18n="skip" (gần nhất) loại cả cây khỏi việc dịch (nội dung của khách: tin nhắn, tên, nội dung website...);
// data-i18n="on" bật lại bên trong. Ô đang gõ (contenteditable) cũng bỏ qua.
// Cố ý không dùng translate="no" của HTML vì nó chặn luôn tính năng "Dịch trang" của trình duyệt —
// người dùng nước ngoài vẫn cần nó để đọc tin nhắn tiếng Việt của khách.
// forAttr: đang xét thuộc tính của chính phần tử này (vd. placeholder của <textarea>, data-placeholder của ô đang sửa)
// — khác với chữ bên trong, thuộc tính vẫn được dịch.
function ancestorSkip(el: Element | null, forAttr = false): boolean {
  if (!el) return true
  if (!forAttr && (el as HTMLElement).isContentEditable) return true
  for (let p: Element | null = el; p; p = p.parentElement) {
    if (SKIP_TAGS.has(p.tagName) && !(forAttr && p === el)) return true
    const t = p.getAttribute('data-i18n')
    if (t !== null) return t === 'skip'
  }
  return false
}

function walk(node: Node, skip: boolean) {
  if (node.nodeType === 3) {
    if (!skip) doText(node as Text)
    return
  }
  if (node.nodeType !== 1) return
  const el = node as Element
  let s = skip
  const t = el.getAttribute('data-i18n')
  if (t !== null) s = t === 'skip'
  if (!s) for (const a of ATTRS) if (el.hasAttribute(a)) doAttr(el, a)
  if (SKIP_TAGS.has(el.tagName)) return // <textarea>, <script>...: chỉ dịch thuộc tính, không đụng nội dung bên trong
  const childSkip = s || (el as HTMLElement).isContentEditable
  for (let c = el.firstChild; c; c = c.nextSibling) walk(c, childSkip)
}

function retranslateAll() {
  if (typeof document === 'undefined' || !document.body) return
  walk(document.body, ancestorSkip(document.body))
  doTitle()
}

let mo: MutationObserver | null = null
let titleMo: MutationObserver | null = null

function startObservers() {
  if (mo || typeof MutationObserver === 'undefined') return
  mo = new MutationObserver((records) => {
    for (const r of records) {
      if (r.type === 'childList') {
        r.addedNodes.forEach((n) => {
          if (n.nodeType === 3) {
            if (!ancestorSkip(n.parentElement)) doText(n as Text)
          } else if (n.nodeType === 1) {
            walk(n, ancestorSkip((n as Element).parentElement))
          }
        })
      } else if (r.type === 'characterData') {
        if (r.target.nodeType === 3 && !ancestorSkip(r.target.parentElement)) doText(r.target as Text)
      } else if (r.type === 'attributes' && r.attributeName) {
        const el = r.target as Element
        if (!ancestorSkip(el, true)) doAttr(el, r.attributeName)
      }
    }
  })
  mo.observe(document.body, { subtree: true, childList: true, characterData: true, attributes: true, attributeFilter: ATTRS })
  const titleEl = document.querySelector('title')
  if (titleEl) {
    titleMo = new MutationObserver(() => doTitle())
    titleMo.observe(titleEl, { childList: true, characterData: true, subtree: true })
  }
}

function stopObservers() {
  mo?.disconnect()
  titleMo?.disconnect()
  mo = null
  titleMo = null
}

// ----------------------------------------------------------------- Nạp bản dịch

let enMap: Record<string, string> | null = null
const langMaps = new Map<string, Record<string, string>>()
let active = false // đang ở trang quản trị

async function loadEnglish(): Promise<Record<string, string>> {
  if (!enMap) {
    const mod = await import('../i18n/en.json')
    enMap = ((mod as { default?: unknown }).default ?? mod) as Record<string, string>
    if (!ui.total) ui.total = Object.keys(enMap).length
  }
  return enMap
}

function readCache(lang: string): Record<string, string> | null {
  try {
    const v = JSON.parse(lsGet(K_TR(lang)) || 'null')
    return v && typeof v === 'object' ? v : null
  } catch {
    return null
  }
}

let cacheTimer: number | null = null
function writeCacheSoon(lang: string) {
  if (cacheTimer) clearTimeout(cacheTimer)
  cacheTimer = window.setTimeout(() => {
    cacheTimer = null
    const m = langMaps.get(lang)
    if (m) lsSet(K_TR(lang), JSON.stringify(m))
  }, 800)
}

// Nạp những gì có sẵn tại máy (tiếng Anh trong gói + bản dịch đã lưu), không chờ mạng
async function loadDictionary(lang: string) {
  if (lang === 'vi') {
    buildDict(null, null)
    ui.count = ui.total
    return
  }
  const en = await loadEnglish()
  let tr: Record<string, string> | null = null
  if (lang !== 'en') {
    tr = langMaps.get(lang) || readCache(lang)
    if (tr) langMaps.set(lang, tr)
  }
  buildDict(en, tr)
  ui.count = lang === 'en' ? ui.total : Object.keys(tr || {}).length
}

function wantObservers(): boolean {
  return active && ui.lang !== 'vi' && hasDict
}

// Gọi mỗi khi từ điển / trang / ngôn ngữ đổi: bật-tắt bộ theo dõi và dịch lại toàn trang
function applyNow() {
  if (wantObservers()) {
    startObservers()
    retranslateAll()
  } else {
    // về tiếng Việt hoặc rời trang quản trị: trả chữ gốc rồi dừng theo dõi
    if (mo || titleRec) retranslateAll()
    stopObservers()
  }
}

let retranslateTimer: number | null = null
function applySoon() {
  if (retranslateTimer) return
  retranslateTimer = window.setTimeout(() => {
    retranslateTimer = null
    applyNow()
  }, 150)
}

function mergeTranslations(lang: string, incoming: Record<string, string>) {
  const cur = { ...(langMaps.get(lang) || {}) }
  for (const [k, v] of Object.entries(incoming)) if (typeof v === 'string' && v.trim()) cur[k] = v
  langMaps.set(lang, cur)
  writeCacheSoon(lang)
  if (lang === ui.lang) {
    ui.count = Object.keys(cur).length
    buildDict(enMap, cur)
    applySoon()
  }
}

const sleep = (ms: number) => new Promise((r) => setTimeout(r, ms))

// Lấy bản dịch mới nhất từ máy chủ rồi (nếu còn thiếu và đã đăng nhập) nhờ AI dịch tiếp
async function refreshFromServer(force = false) {
  const lang = ui.lang
  if (lang === 'vi' || lang === 'en') {
    ui.status = 'idle'
    return
  }
  ui.status = 'loading'
  try {
    const { data } = await api.get(`/ui-i18n/${lang}`, { timeout: 25000 })
    if (lang !== ui.lang) return
    if (data?.translations) mergeTranslations(lang, data.translations)
    if (data?.total) ui.total = Number(data.total)
  } catch {
    /* máy chủ đang ngủ / mất mạng: dùng bản đã lưu, câu thiếu hiện tiếng Anh */
  }
  if (lang !== ui.lang) return
  ui.status = 'idle'
  if (ui.total && ui.count < ui.total) void fillMissing(lang, force)
}

let filling: string | null = null
async function fillMissing(lang: string, force: boolean) {
  if (filling || !lsGet('access_token')) {
    if (ui.count < ui.total) ui.status = 'partial'
    return
  }
  // Lần trước dịch dở mà thất bại thì chờ 2 giờ (trừ khi người dùng vừa chủ động chọn lại ngôn ngữ)
  if (!force && Date.now() - Number(lsGet(K_FILL(lang)) || 0) < 2 * 3600e3) {
    ui.status = 'partial'
    return
  }
  filling = lang
  ui.status = 'filling'
  let stalls = 0
  let busy = 0
  let finished = false
  try {
    for (let i = 0; i < 80 && lang === ui.lang; i++) {
      const { data } = await api.post(`/ui-i18n/${lang}/fill`, null, { timeout: 120000 })
      if (data.busy) {
        // người khác đang dịch ngôn ngữ này: chờ rồi lấy kết quả của họ
        if (++busy > 12) break
        await sleep(5000)
        const r = await api.get(`/ui-i18n/${lang}`, { timeout: 25000 }).catch(() => null)
        if (r?.data?.translations) mergeTranslations(lang, r.data.translations)
        if (ui.count >= ui.total) {
          finished = true
          break
        }
        continue
      }
      const added = (data.added || {}) as Record<string, string>
      if (Object.keys(added).length) {
        stalls = 0
        mergeTranslations(lang, added)
      } else if (++stalls >= 2) {
        break
      }
      if (data.total) ui.total = Number(data.total)
      if (!data.remaining) {
        finished = true
        break
      }
    }
  } catch {
    /* hết lượt gọi AI, chưa có API key, mất mạng... — lần sau thử lại */
  }
  filling = null
  if (finished) {
    lsSet(K_FILL(lang), null)
    if (lang === ui.lang) ui.status = 'idle'
  } else {
    lsSet(K_FILL(lang), String(Date.now()))
    if (lang === ui.lang) ui.status = ui.count < ui.total ? 'partial' : 'idle'
  }
}

// ----------------------------------------------------------------- Điều khiển công khai

export async function setUiLang(code: string) {
  if (!LANG_BY_CODE.has(code) || code === ui.lang) return
  ui.lang = code
  lsSet(K_LANG, code)
  document.documentElement.lang = code
  if (ui.currencyAuto) ui.currency = LANG_BY_CODE.get(code)!.currency
  if (ui.currency !== 'VND') void ensureRates()
  try {
    await loadDictionary(code)
  } catch {
    buildDict(null, null) // không nạp được tiếng Anh (mất mạng): giữ nguyên chữ Việt
  }
  if (code !== ui.lang) return // người dùng đã đổi tiếp
  applyNow()
  void refreshFromServer(true)
}

function isAdminRoute(to: RouteLocationNormalized): boolean {
  return Boolean(to.meta.requiresAuth || to.meta.guestOnly || to.name === 'auth-callback')
}

function installDialogs() {
  // alert / confirm / prompt là hộp thoại của trình duyệt, không nằm trong DOM → dịch chữ trước khi hiện
  const w = window as unknown as Record<string, (...a: unknown[]) => unknown>
  for (const name of ['alert', 'confirm', 'prompt']) {
    const orig = w[name]!.bind(window)
    w[name] = (msg?: unknown, ...rest: unknown[]) => orig(typeof msg === 'string' && active ? translateString(msg) ?? msg : msg, ...rest)
  }
}

// Vào / rời trang quản trị. Lần đầu vào từ trang công khai (chưa nạp từ điển) thì nạp rồi mới dịch.
async function setActive(on: boolean) {
  active = on
  if (on && ui.lang !== 'vi' && !hasDict) {
    const lang = ui.lang
    try {
      await loadDictionary(lang)
    } catch {
      buildDict(null, null)
    }
    if (lang === ui.lang) void refreshFromServer()
  }
  applyNow()
}

// Gọi một lần trước khi mount app
export async function initUiI18n(router: Router): Promise<void> {
  if (onCustomDomain) return // tên miền riêng của khách chỉ có cửa hàng công khai, không có giao diện quản trị
  ui.lang = detectLang()
  const cur = lsGet(K_CUR)
  if (cur && /^[A-Z]{3}$/.test(cur)) {
    ui.currency = cur
    ui.currencyAuto = false
  } else {
    ui.currency = LANG_BY_CODE.get(ui.lang)?.currency || 'VND'
  }
  document.documentElement.lang = ui.lang
  installDialogs()

  router.afterEach((to) => {
    void setActive(isAdminRoute(to))
  })
  void router.isReady().then(() => setActive(isAdminRoute(router.currentRoute.value)))

  const p = window.location.pathname
  const publicPath = p.startsWith('/s/') || p === '/store'
  if (ui.lang !== 'vi' && !publicPath) {
    try {
      await loadDictionary(ui.lang)
    } catch {
      buildDict(null, null)
    }
  }
  if (ui.currency !== 'VND' && !publicPath) void ensureRates()
  if (!publicPath) void refreshFromServer()
}
