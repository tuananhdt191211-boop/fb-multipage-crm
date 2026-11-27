// Đa ngôn ngữ + đa tiền tệ cho website bán hàng công khai.
// - UI_VI: chữ cố định của giao diện (bản gốc tiếng Việt). Bản dịch nằm trong i18n.tr[lang].ui.
// - Nội dung trang (block) và sản phẩm: bản dịch trong i18n.tr[lang].b / .p, áp lên bản sao dữ liệu.
// - Tiền: giá gốc VND, đổi theo tỉ giá (ExchangeRate-API qua backend) và định dạng theo nước.
import { reactive } from 'vue'
import { api } from './api'
import { MARKET_BY_CODE, type Market } from './markets'
import type { PublicProduct, StoreBlock } from '../types/store'

export const UI_VI = {
  loading: 'Đang tải...',
  loading_store: 'Đang tải cửa hàng...',
  loading_order: 'Đang tải đơn hàng...',
  store_not_found: 'Không tìm thấy trang bán hàng đã xuất bản.',
  shop_not_found: 'Không tìm thấy cửa hàng.',
  home: 'Trang chủ',
  back_home: 'Về trang chủ',
  continue_shopping: 'Tiếp tục mua sắm',
  close: 'Đóng',
  decrease: 'Giảm',
  increase: 'Tăng',
  remove: 'Xoá',
  prev: 'Trước',
  next: 'Sau',
  search: 'Tìm kiếm',
  search_placeholder: 'Tìm sản phẩm...',
  search_close: 'Đóng tìm kiếm',
  search_empty: 'Không tìm thấy sản phẩm phù hợp',
  open_menu: 'Mở menu',
  cart: 'Giỏ hàng',
  cart_title: 'Giỏ hàng ({n})',
  cart_empty: 'Giỏ hàng đang trống',
  cart_empty_long: 'Giỏ hàng của bạn đang trống.',
  subtotal: 'Tạm tính',
  shipping_at_checkout: 'Phí vận chuyển tính ở bước thanh toán.',
  checkout: 'Thanh toán',
  // Tài khoản
  login: 'Đăng nhập',
  register: 'Đăng ký',
  logout: 'Đăng xuất',
  account: 'Tài khoản',
  account_of: 'Tài khoản của {name}',
  login_register: 'Đăng nhập / Đăng ký',
  my_orders: 'Đơn hàng của tôi',
  account_info: 'Thông tin tài khoản',
  create_account: 'Tạo tài khoản',
  auth_sub: '{shop} · theo dõi đơn hàng, đặt hàng nhanh hơn',
  login_id: 'Số điện thoại hoặc email',
  full_name: 'Họ và tên',
  phone: 'Số điện thoại',
  email_optional: 'Email (không bắt buộc)',
  password: 'Mật khẩu',
  password_new: 'Mật khẩu (ít nhất 6 ký tự)',
  show_password: 'Hiện mật khẩu',
  hide_password: 'Ẩn mật khẩu',
  no_account: 'Chưa có tài khoản?',
  register_now: 'Đăng ký ngay',
  have_account: 'Đã có tài khoản?',
  login_failed: 'Đăng nhập thất bại.',
  register_failed: 'Đăng ký thất bại.',
  your_account: 'Tài khoản của bạn',
  account_intro: 'Đăng nhập để xem đơn hàng đã đặt và lưu thông tin giao hàng cho lần sau.',
  hello: 'Xin chào, {name}',
  delivery_info: 'Thông tin giao hàng',
  no_orders: 'Bạn chưa có đơn hàng nào.',
  shop_now: 'Mua sắm ngay',
  email: 'Email',
  not_required: 'Không bắt buộc',
  save_info: 'Lưu thông tin',
  saved: 'Đã lưu thông tin.',
  save_failed: 'Không lưu được thông tin.',
  orders_failed: 'Không tải được đơn hàng.',
  items_count: '{n} sản phẩm',
  paid: 'Đã thanh toán',
  // Section mặc định
  cta_default: 'Đặt mua ngay',
  explore: 'Khám phá ngay',
  contact_now: 'Liên hệ ngay',
  buy: 'Đặt mua',
  add_to_cart: 'Thêm vào giỏ',
  out_of_stock: 'Hết hàng',
  products_heading: 'Sản phẩm nổi bật',
  products_soon: 'Sản phẩm sắp ra mắt.',
  features_heading: 'Cam kết dịch vụ',
  testimonials_heading: 'Khách hàng nói gì',
  call: 'Gọi điện',
  message: 'Nhắn tin',
  chat_zalo: 'Chat Zalo',
  address: 'Địa chỉ',
  subscribe: 'Đăng ký',
  your_email: 'Email của bạn',
  subscribe_thanks: 'Cảm ơn bạn đã đăng ký!',
  email_invalid: 'Email chưa đúng định dạng',
  subscribe_failed: 'Đăng ký chưa thành công, vui lòng thử lại',
  slide_prev: 'Ảnh trước',
  slide_next: 'Ảnh sau',
  slide_n: 'Ảnh {n}',
  // Trang sản phẩm
  product: 'Sản phẩm',
  product_missing: 'Sản phẩm không tồn tại hoặc đã ngừng bán.',
  save_amount: 'Tiết kiệm {amount}',
  in_stock: '● Còn hàng',
  sold_out: '● Tạm hết hàng',
  quantity: 'Số lượng',
  buy_now: 'Mua ngay',
  ship_nationwide: 'Giao hàng toàn quốc',
  check_on_delivery: 'Kiểm tra hàng khi nhận',
  description: 'Mô tả sản phẩm',
  related: 'Có thể bạn cũng thích',
  // Thanh toán
  checkout_failed_load: 'Không tải được trang thanh toán.',
  fill_required: 'Vui lòng nhập đủ họ tên, số điện thoại và địa chỉ.',
  cart_is_empty: 'Giỏ hàng đang trống.',
  order_failed: 'Đặt hàng chưa thành công, vui lòng thử lại.',
  shipping_info: 'Thông tin nhận hàng',
  full_name_req: 'Họ và tên *',
  phone_req: 'Số điện thoại *',
  address_req: 'Địa chỉ nhận hàng *',
  address_ph: 'Số nhà, đường, phường/xã, quận/huyện, tỉnh/thành',
  note: 'Ghi chú',
  note_ph: 'Thời gian nhận hàng, yêu cầu đặc biệt...',
  payment_method: 'Phương thức thanh toán',
  cod: 'Thanh toán khi nhận hàng (COD)',
  cod_desc: 'Kiểm tra hàng rồi trả tiền cho shipper',
  vnpay: 'Thanh toán online qua VNPay',
  vnpay_desc: 'Thẻ ATM, Visa/Master, QR ngân hàng',
  order_summary: 'Đơn hàng ({n} sản phẩm)',
  shipping_fee: 'Phí vận chuyển',
  free: 'Miễn phí',
  free_ship_more: 'Mua thêm {amount} để được miễn phí vận chuyển',
  total: 'Tổng cộng',
  pay_vnpay: 'Thanh toán qua VNPay',
  place_order: 'Đặt hàng',
  privacy_note: 'Thông tin của bạn chỉ dùng để giao hàng.',
  charged_in_vnd: 'Giá quy đổi để tham khảo. Đơn hàng được thanh toán bằng VND: {amount}.',
  // Đơn hàng
  order_not_found: 'Không tìm thấy đơn hàng.',
  pay_link_failed: 'Không tạo được liên kết thanh toán.',
  order_success: 'Đặt hàng thành công!',
  payment_failed: 'Thanh toán chưa thành công',
  payment_pending: 'Đang chờ xác nhận thanh toán',
  order_code: 'Mã đơn hàng:',
  will_call: 'Cửa hàng sẽ gọi cho bạn qua số {phone} để xác nhận và giao hàng.',
  order_kept: 'Đơn hàng vẫn được giữ. Bạn có thể thử thanh toán lại.',
  auto_update: 'Nếu bạn đã thanh toán, trang sẽ tự cập nhật trong giây lát.',
  redirecting: 'Đang chuyển...',
  retry_vnpay: 'Thanh toán lại qua VNPay',
  payment: 'Thanh toán',
  pay_on_delivery: 'Khi nhận hàng (COD)',
  paid_vnpay: 'Đã thanh toán qua VNPay',
  st_new: 'Chờ xác nhận',
  st_confirmed: 'Đã xác nhận',
  st_shipping: 'Đang giao',
  st_completed: 'Hoàn tất',
  st_cancelled: 'Đã huỷ',
  // Chọn thị trường
  choose_market: 'Quốc gia / ngôn ngữ',
  rates_note: 'Tỉ giá cập nhật hằng ngày'
} as const

export type UiKey = keyof typeof UI_VI

export interface SiteI18n {
  markets: string[]
  default?: string
  tr: Record<string, { b?: Record<string, Record<string, string>>; p?: Record<string, { name?: string; description?: string }>; ui?: Record<string, string> }>
}

export function parseI18n(raw?: string | null): SiteI18n {
  try {
    const v = JSON.parse(raw || '{}')
    return { markets: Array.isArray(v.markets) ? v.markets : [], default: v.default, tr: v.tr && typeof v.tr === 'object' ? v.tr : {} }
  } catch {
    return { markets: [], tr: {} }
  }
}

// ----------------------------------------------------------------- Trạng thái ngôn ngữ / tiền tệ

export const locale = reactive({
  slug: '',
  market: 'vn',
  lang: 'vi',
  currency: 'VND',
  tag: 'vi-VN',
  markets: ['vn'] as string[],
  i18n: { markets: [], tr: {} } as SiteI18n,
  rates: {} as Record<string, number>,
  ratesUpdated: ''
})

let ratesLoading: Promise<void> | null = null
function loadRates() {
  if (ratesLoading) return ratesLoading
  ratesLoading = api
    .get('/storefront/public/rates')
    .then(({ data }) => {
      locale.rates = data.rates || {}
      locale.ratesUpdated = data.updated_at || ''
    })
    .catch(() => {
      ratesLoading = null // thử lại lần sau
    })
  return ratesLoading
}

const mkKey = (slug: string) => `pp_mk_${slug}`

function applyMarket(code: string) {
  const mk = MARKET_BY_CODE.get(code) || MARKET_BY_CODE.get('vn')!
  locale.market = mk.code
  locale.lang = mk.lang
  locale.currency = mk.currency
  locale.tag = mk.locale
  if (typeof document !== 'undefined') document.documentElement.lang = mk.lang
  if (mk.currency !== 'VND') loadRates()
}

// Gọi khi tải website: chọn thị trường đã lưu, hoặc theo ngôn ngữ trình duyệt, hoặc mặc định
export function initLocale(slug: string, i18n: SiteI18n) {
  const enabled = ['vn', ...i18n.markets.filter((c) => c !== 'vn' && MARKET_BY_CODE.has(c))]
  locale.markets = enabled
  locale.i18n = i18n
  if (locale.slug === slug && enabled.includes(locale.market)) return
  locale.slug = slug
  let pick = ''
  try {
    pick = localStorage.getItem(mkKey(slug)) || ''
  } catch {
    /* bỏ qua */
  }
  if (!enabled.includes(pick)) {
    pick = ''
    const nav = typeof navigator !== 'undefined' ? navigator.languages || [navigator.language] : []
    for (const l of nav) {
      const [lang, country] = String(l).toLowerCase().split('-')
      const cands = enabled.map((c) => MARKET_BY_CODE.get(c)!).filter((m) => m.lang === lang || (lang === 'no' && m.lang === 'nb'))
      const exact = cands.find((m) => country && (m.code === country || m.code.startsWith(country + '-') || m.locale.toLowerCase().endsWith('-' + country)))
      const found = exact || cands[0]
      if (found) {
        pick = found.code
        break
      }
    }
  }
  if (!pick) pick = i18n.default && enabled.includes(i18n.default) ? i18n.default : 'vn'
  applyMarket(pick)
}

export function setMarket(code: string) {
  if (!locale.markets.includes(code)) return
  applyMarket(code)
  try {
    localStorage.setItem(mkKey(locale.slug), code)
  } catch {
    /* bỏ qua */
  }
}

export function currentMarket(): Market {
  return MARKET_BY_CODE.get(locale.market) || MARKET_BY_CODE.get('vn')!
}

// ----------------------------------------------------------------- Chữ & tiền

export function t(key: UiKey, vars?: Record<string, string | number>): string {
  const tr = locale.lang !== 'vi' ? locale.i18n.tr[locale.lang]?.ui?.[key] : undefined
  let s: string = tr || UI_VI[key] || key
  if (vars) for (const [k, v] of Object.entries(vars)) s = s.split(`{${k}}`).join(String(v))
  return s
}

export function formatVndPlain(n: number): string {
  return new Intl.NumberFormat('vi-VN').format(Math.max(0, Math.round(n || 0))) + 'đ'
}

// Giá (gốc VND) theo tiền tệ đang chọn
export function money(vnd: number): string {
  const cur = locale.currency
  const rate = locale.rates[cur]
  if (cur === 'VND' || !rate) return formatVndPlain(vnd)
  try {
    return new Intl.NumberFormat(locale.tag, { style: 'currency', currency: cur }).format(Math.max(0, vnd || 0) * rate)
  } catch {
    return formatVndPlain(vnd)
  }
}

export function isForeignCurrency(): boolean {
  return locale.currency !== 'VND' && !!locale.rates[locale.currency]
}

export function formatDate(s: string): string {
  const d = new Date(s)
  return isNaN(d.getTime()) ? '' : d.toLocaleString(locale.tag, { day: '2-digit', month: '2-digit', year: 'numeric', hour: '2-digit', minute: '2-digit' })
}

// ----------------------------------------------------------------- Áp bản dịch lên nội dung

function setPath(obj: any, path: string, value: string) {
  const parts = path.split('.')
  let cur = obj
  for (let i = 0; i < parts.length - 1; i++) {
    const k = /^\d+$/.test(parts[i]!) ? Number(parts[i]) : parts[i]!
    if (cur == null || typeof cur !== 'object') return
    cur = cur[k as any]
  }
  const last = parts[parts.length - 1]!
  const k = /^\d+$/.test(last) ? Number(last) : last
  if (cur && typeof cur === 'object' && typeof cur[k as any] === 'string') cur[k as any] = value
}

export function localizeBlocks(blocks: StoreBlock[], lang = locale.lang, i18n = locale.i18n): StoreBlock[] {
  const tb = lang !== 'vi' ? i18n.tr[lang]?.b : undefined
  if (!tb) return blocks
  return blocks.map((b) => {
    const tr = tb[b.id]
    if (!tr) return b
    const copy: StoreBlock = { ...b, data: JSON.parse(JSON.stringify(b.data || {})) }
    for (const [path, text] of Object.entries(tr)) if (text) setPath(copy.data, path, text)
    return copy
  })
}

export function localizeProducts<T extends { id: number; name: string; description?: string }>(products: T[], lang = locale.lang, i18n = locale.i18n): T[] {
  const tp = lang !== 'vi' ? i18n.tr[lang]?.p : undefined
  if (!tp) return products
  return products.map((p) => {
    const tr = tp[String(p.id)]
    return tr ? { ...p, name: tr.name || p.name, description: tr.description ?? p.description } : p
  })
}

// ----------------------------------------------------------------- Thu thập chữ cần dịch (trình chỉnh sửa)

const SKIP_KEYS = new Set([
  'style', '_text', 'layout', 'type', 'icon', 'source', 'buy_mode', 'right_mode', 'account_style', 'brand_size', 'height',
  'image_side', 'align', 'html', 'embed', 'pad', 'bg', 'text_color', 'color', 'css', 'id', 'socials', 'price', 'compare_price',
  'phone', 'email', 'url', 'link', 'href', 'video', 'map', 'map_embed', 'logo_height', 'columns_count',
  // tên riêng / địa chỉ: giữ nguyên
  'brand_name', 'company', 'address'
])

function isTranslatable(key: string, v: string): boolean {
  if (SKIP_KEYS.has(key) || /(_url|_link|_embed|_id|_color)$/.test(key)) return false
  const s = v.trim()
  if (!s || s.length > 4000) return false
  if (/^(https?:|mailto:|tel:|#|\/)/i.test(s)) return false
  if (/^#[0-9a-f]{3,8}$/i.test(s)) return false
  return /\p{L}/u.test(s)
}

// {blockId: {path: text}}
export function collectBlockTexts(blocks: StoreBlock[]): Record<string, Record<string, string>> {
  const out: Record<string, Record<string, string>> = {}
  const walk = (v: any, path: string[], blockId: string, key: string) => {
    if (typeof v === 'string') {
      if (isTranslatable(key, v)) (out[blockId] ||= {})[path.join('.')] = v
    } else if (Array.isArray(v)) {
      v.forEach((x, i) => walk(x, [...path, String(i)], blockId, key))
    } else if (v && typeof v === 'object') {
      for (const [k, x] of Object.entries(v)) {
        if (SKIP_KEYS.has(k)) continue
        walk(x, [...path, k], blockId, k)
      }
    }
  }
  for (const b of blocks) walk(b.data || {}, [], b.id, '')
  return out
}

export function collectProductTexts(products: PublicProduct[]): Record<string, { name?: string; description?: string }> {
  const out: Record<string, { name?: string; description?: string }> = {}
  for (const p of products) {
    const e: { name?: string; description?: string } = {}
    if (p.name?.trim()) e.name = p.name
    if ((p as any).description?.trim()) e.description = (p as any).description
    if (e.name || e.description) out[String(p.id)] = e
  }
  return out
}

// Trình chỉnh sửa: xem trước website bằng một ngôn ngữ (tiền tệ = thị trường đầu tiên dùng ngôn ngữ đó)
export function previewLocale(lang: string, i18n: SiteI18n) {
  locale.i18n = i18n
  locale.lang = lang
  const mk = [...i18n.markets, 'vn'].map((c) => MARKET_BY_CODE.get(c)).find((x) => x && x.lang === lang)
  locale.market = mk?.code || 'vn'
  locale.currency = mk?.currency || 'VND'
  locale.tag = mk?.locale || 'vi-VN'
  if (locale.currency !== 'VND') loadRates()
}
