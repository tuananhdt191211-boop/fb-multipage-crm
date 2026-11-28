// Theo dõi hành vi khách trên web bán hàng công khai → gửi lô nhỏ về POST /storefront/public/track.
// Riêng tư: chỉ dùng mã ẩn danh lưu trong trình duyệt; không gửi họ tên / số điện thoại / email / địa chỉ;
// đường dẫn đã bỏ tham số (mã đơn, token); tôn trọng "Do Not Track"; không theo dõi chủ shop đang đăng nhập.
// Không bao giờ làm hỏng trang: mọi lỗi ở đây đều bị nuốt.
import { api } from './api'

export type TrackType =
  | 'page_view'
  | 'product_view'
  | 'add_to_cart'
  | 'remove_from_cart'
  | 'cart_open'
  | 'checkout_start'
  | 'checkout_info'
  | 'checkout_info_done'
  | 'checkout_payment'
  | 'checkout_submit'
  | 'checkout_error'
  | 'click'
  | 'search'
  | 'market_change'
  | 'heartbeat'

interface TrackEvent {
  t: TrackType
  p: string // đường dẫn trang
  pid?: number
  l?: string // nhãn
  tg?: string // đích của click
  sec?: string // loại section
  v?: number
}

export interface CartSnapLine {
  product_id: number
  variant_id?: number
  qty: number
  price: number
  name: string
}

interface SessionRec {
  sid: string
  last: number
  ref: string
  us: string
  um: string
  uc: string
  fb: boolean
  landing: string
  ret: boolean
}

const SESSION_GAP = 30 * 60 * 1000 // im lặng quá 30 phút = phiên mới
const FLUSH_MS = 4000
const HEARTBEAT_MS = 40000
const IDLE_STOP_MS = 10 * 60 * 1000 // không thao tác 10 phút thì ngừng heartbeat
const MAX_QUEUE = 60
const MAX_CLICKS = 80 // số click tối đa gửi trong một lần mở trang

let slug = ''
let queue: TrackEvent[] = []
let timer: ReturnType<typeof setTimeout> | null = null
let heartbeat: ReturnType<typeof setInterval> | null = null
let started = false
let lastActivity = Date.now()
let clickCount = 0
let cartGetter: (() => CartSnapLine[]) | null = null
let cartDirty = false
let lastSearch = ''

function ls(): Storage | null {
  try {
    return window.localStorage
  } catch {
    return null
  }
}

// Có đang theo dõi không: bỏ qua khi khách bật "Do Not Track", khi chủ shop đang đăng nhập trên cùng trình duyệt, hoặc khi đã tắt thủ công
export function trackingEnabled(): boolean {
  if (typeof window === 'undefined') return false
  const nav = navigator as Navigator & { msDoNotTrack?: string }
  if (nav.doNotTrack === '1' || (window as any).doNotTrack === '1' || nav.msDoNotTrack === '1') return false
  const s = ls()
  if (!s) return false
  if (s.getItem('access_token') || s.getItem('pp_notrack') === '1') return false
  return true
}

const rand = (n: number) => {
  const a = new Uint8Array(n)
  try {
    crypto.getRandomValues(a)
  } catch {
    for (let i = 0; i < n; i++) a[i] = Math.floor(Math.random() * 256)
  }
  return [...a].map((b) => b.toString(16).padStart(2, '0')).join('')
}

function visitorId(): string {
  const s = ls()
  let v = s?.getItem('pp_vid') || ''
  if (!/^[A-Za-z0-9_-]{8,40}$/.test(v)) {
    v = rand(12)
    try {
      s?.setItem('pp_vid', v)
    } catch {
      /* bỏ qua */
    }
  }
  return v
}

// Bỏ tham số, bỏ tiền tố /s/<slug>, gộp trang đơn hàng — máy chủ cũng làm lại bước này
export function normPath(path: string): string {
  let p = String(path || '/').split(/[?#]/)[0] || '/'
  if (p[0] !== '/') p = '/'
  if (slug) {
    const prefix = '/s/' + slug
    if (p === prefix || p.startsWith(prefix + '/')) p = p.slice(prefix.length) || '/'
  }
  if (/^\/order\b/.test(p)) return '/order/*'
  return p.slice(0, 120)
}

function currentSession(): SessionRec | null {
  const s = ls()
  if (!s || !slug) return null
  const key = 'pp_sess_' + slug
  const now = Date.now()
  let rec: SessionRec | null = null
  try {
    rec = JSON.parse(s.getItem(key) || 'null')
  } catch {
    rec = null
  }
  if (!rec || !rec.sid || now - rec.last > SESSION_GAP) {
    const hadVisitor = !!s.getItem('pp_vid')
    const q = new URLSearchParams(location.search)
    rec = {
      sid: rand(12),
      last: now,
      ref: (document.referrer || '').slice(0, 300),
      us: (q.get('utm_source') || '').slice(0, 60),
      um: (q.get('utm_medium') || '').slice(0, 20),
      uc: (q.get('utm_campaign') || '').slice(0, 80),
      fb: q.has('fbclid'),
      landing: normPath(location.pathname),
      ret: hadVisitor
    }
  }
  rec.last = now
  try {
    s.setItem(key, JSON.stringify(rec))
  } catch {
    /* bỏ qua */
  }
  return rec
}

// Mã phiên hiện tại để gắn vào đơn hàng (rỗng nếu không theo dõi)
export function trackSid(): string {
  if (!trackingEnabled()) return ''
  return currentSession()?.sid || ''
}

// Nước (ước lượng, không dùng IP)
const TZ_COUNTRY: Record<string, string> = {
  'Asia/Ho_Chi_Minh': 'VN', 'Asia/Saigon': 'VN', 'Asia/Bangkok': 'TH', 'Asia/Jakarta': 'ID', 'Asia/Manila': 'PH', 'Asia/Singapore': 'SG',
  'Asia/Kuala_Lumpur': 'MY', 'Asia/Tokyo': 'JP', 'Asia/Seoul': 'KR', 'Asia/Shanghai': 'CN', 'Asia/Hong_Kong': 'HK', 'Asia/Taipei': 'TW',
  'Asia/Kolkata': 'IN', 'Asia/Calcutta': 'IN', 'Asia/Dubai': 'AE', 'Asia/Phnom_Penh': 'KH', 'Asia/Vientiane': 'LA', 'Europe/London': 'GB',
  'Europe/Paris': 'FR', 'Europe/Berlin': 'DE', 'Europe/Madrid': 'ES', 'Europe/Rome': 'IT', 'Europe/Amsterdam': 'NL', 'Europe/Warsaw': 'PL',
  'Europe/Stockholm': 'SE', 'Europe/Moscow': 'RU', 'Europe/Kiev': 'UA', 'Europe/Kyiv': 'UA', 'Australia/Sydney': 'AU', 'Australia/Melbourne': 'AU',
  'Pacific/Auckland': 'NZ', 'America/New_York': 'US', 'America/Chicago': 'US', 'America/Denver': 'US', 'America/Los_Angeles': 'US',
  'America/Toronto': 'CA', 'America/Vancouver': 'CA', 'America/Mexico_City': 'MX', 'America/Sao_Paulo': 'BR'
}
const LANG_COUNTRY: Record<string, string> = {
  vi: 'VN', th: 'TH', id: 'ID', ms: 'MY', ja: 'JP', ko: 'KR', zh: 'CN', hi: 'IN', fr: 'FR', de: 'DE', es: 'ES', it: 'IT', nl: 'NL', pl: 'PL',
  sv: 'SE', ru: 'RU', uk: 'UA', pt: 'BR', tr: 'TR'
}
function countryGuess(): string {
  // 1) vùng ghi trong ngôn ngữ trình duyệt (vi-VN → VN); 2) nước hay dùng ngôn ngữ đó (vi → VN); 3) múi giờ của máy
  const first = (navigator.languages?.length ? navigator.languages[0] : navigator.language) || ''
  const m = /^[a-z]{2,3}-([A-Za-z]{2})$/.exec(first)
  if (m) return m[1]!.toUpperCase()
  const byLang = LANG_COUNTRY[first.slice(0, 2).toLowerCase()]
  if (byLang) return byLang
  try {
    return TZ_COUNTRY[Intl.DateTimeFormat().resolvedOptions().timeZone] || ''
  } catch {
    return ''
  }
}

// Ngôn ngữ / thị trường khách đang chọn (do lib/i18n.ts báo mỗi khi đổi)
let mctx = { lang: '', market: '' }
export function setMarketCtx(lang: string, market: string) {
  mctx = { lang, market }
}

function endpoint(): string {
  return String(api.defaults.baseURL || '').replace(/\/+$/, '') + '/storefront/public/track'
}

function flush() {
  if (timer) {
    clearTimeout(timer)
    timer = null
  }
  if (!slug || !queue.length || !trackingEnabled()) {
    if (!trackingEnabled()) queue = []
    return
  }
  const rec = currentSession()
  if (!rec) return
  const events = queue.splice(0, 25)
  const mk = mctx
  const body: Record<string, unknown> = {
    slug,
    sid: rec.sid,
    vid: visitorId(),
    ctx: { ref: rec.ref, us: rec.us, um: rec.um, uc: rec.uc, fb: rec.fb, landing: rec.landing, lang: mk.lang, market: mk.market, country: countryGuess(), ret: rec.ret },
    events
  }
  if (cartGetter && (cartDirty || events.some((e) => e.t.startsWith('checkout') || e.t === 'add_to_cart'))) {
    body.cart = {
      items: cartGetter()
        .slice(0, 10)
        .map((l) => ({ p: l.product_id, v: l.variant_id || 0, q: l.qty, pr: l.price, nm: (l.name || '').slice(0, 60) }))
    }
    cartDirty = false
  }
  const json = JSON.stringify(body)
  try {
    // text/plain: không cần preflight CORS, và sendBeacon vẫn gửi được khi khách đóng tab
    const blob = new Blob([json], { type: 'text/plain;charset=UTF-8' })
    if (!(navigator.sendBeacon && navigator.sendBeacon(endpoint(), blob))) {
      void fetch(endpoint(), { method: 'POST', body: json, keepalive: true, headers: { 'Content-Type': 'text/plain;charset=UTF-8' } }).catch(() => {})
    }
  } catch {
    /* bỏ qua */
  }
  if (queue.length) schedule(300)
}

function schedule(ms = FLUSH_MS) {
  if (timer || !slug) return
  timer = setTimeout(() => flush(), ms)
}

// Ghi một sự kiện. Chưa biết website (đang tải) thì giữ trong hàng đợi, gửi sau khi startTracking.
export function track(type: TrackType, props: { pid?: number; l?: string; tg?: string; sec?: string; v?: number; path?: string } = {}) {
  try {
    if (!trackingEnabled()) return
    if (queue.length >= MAX_QUEUE) queue.shift()
    queue.push({
      t: type,
      p: normPath(props.path ?? location.pathname),
      ...(props.pid ? { pid: props.pid } : {}),
      ...(props.l ? { l: props.l.slice(0, 120) } : {}),
      ...(props.tg ? { tg: props.tg.slice(0, 160) } : {}),
      ...(props.sec ? { sec: props.sec.slice(0, 24) } : {}),
      ...(props.v ? { v: Math.round(props.v) } : {})
    })
    // Sự kiện quan trọng gửi gần như ngay (khách có thể chuyển sang cổng thanh toán)
    schedule(type === 'checkout_submit' || type === 'checkout_start' || type === 'add_to_cart' ? 250 : FLUSH_MS)
  } catch {
    /* bỏ qua */
  }
}

export function trackPage(fullPath: string) {
  track('page_view', { path: fullPath })
}

// Giỏ hàng gần nhất được gửi kèm lô tiếp theo
export function setCartProvider(fn: () => CartSnapLine[]) {
  cartGetter = fn
}
export function markCartDirty() {
  cartDirty = true
}

export function flushNow() {
  flush()
}

function onClick(ev: MouseEvent) {
  lastActivity = Date.now()
  try {
    if (!trackingEnabled() || clickCount >= MAX_CLICKS) return
    const target = ev.target as HTMLElement | null
    const el = target?.closest?.('a,button,[role="button"],[data-track]') as HTMLElement | null
    if (!el || el.closest('[data-no-track]')) return
    let label = el.getAttribute('data-track') || el.getAttribute('aria-label') || ''
    if (!label) label = (el.textContent || '').replace(/\s+/g, ' ').trim()
    if (!label) label = el.getAttribute('title') || ''
    if (!label) return
    clickCount++
    const href = el instanceof HTMLAnchorElement ? el.getAttribute('href') || '' : ''
    track('click', { l: label.slice(0, 60), tg: href, sec: el.closest('[data-pp-type]')?.getAttribute('data-pp-type') || '' })
  } catch {
    /* bỏ qua */
  }
}

// Tìm kiếm sản phẩm ở đầu trang: gọi khi khách ngừng gõ
export function trackSearch(term: string, results: number) {
  const q = term.trim().toLowerCase()
  if (q.length < 2 || q === lastSearch) return
  lastSearch = q
  track('search', { l: q, v: results })
}

// Gọi khi đã biết website (sau khi tải dữ liệu công khai)
export function startTracking(siteSlug: string) {
  if (typeof window === 'undefined') return
  slug = siteSlug
  if (!started) {
    started = true
    document.addEventListener('click', onClick, { capture: true, passive: true })
    for (const e of ['keydown', 'scroll', 'pointerdown', 'touchstart'] as const) window.addEventListener(e, () => (lastActivity = Date.now()), { passive: true })
    document.addEventListener('visibilitychange', () => {
      if (document.visibilityState === 'hidden') flush()
    })
    window.addEventListener('pagehide', () => flush())
    heartbeat = setInterval(() => {
      if (document.visibilityState !== 'visible' || Date.now() - lastActivity > IDLE_STOP_MS || !trackingEnabled()) return
      track('heartbeat')
    }, HEARTBEAT_MS)
  }
  if (queue.length) schedule(300)
}

export function stopTrackingForTests() {
  if (heartbeat) clearInterval(heartbeat)
}
