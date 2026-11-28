<script setup lang="ts">
// Thống kê website bán hàng (kiểu Shopify / ShopBase): khách đang online + đang ở bước nào của thanh toán,
// doanh thu, đơn hàng, chuyển đổi, phễu thanh toán, giỏ hàng bỏ dở, nguồn truy cập, thiết bị, sản phẩm, click...
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Activity, ArrowDown, ArrowLeft, ArrowUp, Loader2, RefreshCw, ShoppingCart } from 'lucide-vue-next'
import { api } from '../lib/api'
import { adminMoney, adminMoneyTitle, uiLocaleTag } from '../lib/uiI18n'
import { blockLabels } from '../lib/blockFactory'

const route = useRoute()
const router = useRouter()
const siteId = computed(() => String(route.params.id || ''))
const siteTitle = ref('')

// ------------------------------------------------------------------ Khoảng ngày
type Preset = 'today' | 'yesterday' | '7d' | '30d' | '90d' | 'custom'
const presets: { v: Preset; l: string }[] = [
  { v: 'today', l: 'Hôm nay' },
  { v: 'yesterday', l: 'Hôm qua' },
  { v: '7d', l: '7 ngày' },
  { v: '30d', l: '30 ngày' },
  { v: '90d', l: '90 ngày' },
  { v: 'custom', l: 'Tuỳ chọn' }
]
const preset = ref<Preset>('7d')
const ymd = (d: Date) => `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
const addDays = (d: Date, n: number) => new Date(d.getFullYear(), d.getMonth(), d.getDate() + n)
const today = () => new Date()
const customFrom = ref(ymd(addDays(today(), -6)))
const customTo = ref(ymd(today()))
const tzo = -new Date().getTimezoneOffset() // phút lệch so với UTC (Việt Nam = 420)

const range = computed(() => {
  const t = today()
  switch (preset.value) {
    case 'today':
      return { from: ymd(t), to: ymd(t) }
    case 'yesterday':
      return { from: ymd(addDays(t, -1)), to: ymd(addDays(t, -1)) }
    case '30d':
      return { from: ymd(addDays(t, -29)), to: ymd(t) }
    case '90d':
      return { from: ymd(addDays(t, -89)), to: ymd(t) }
    case 'custom':
      return { from: customFrom.value, to: customTo.value }
    default:
      return { from: ymd(addDays(t, -6)), to: ymd(t) }
  }
})

// ------------------------------------------------------------------ Dữ liệu
interface Totals {
  sessions: number
  visitors: number
  new_visitors: number
  returning_visitors: number
  page_views: number
  orders: number
  cancelled_orders: number
  paid_orders: number
  revenue: number
  aov: number
  conversion_rate: number
  add_to_cart_rate: number
  checkout_rate: number
  bounce_rate: number
  avg_session_sec: number
  abandoned_carts: number
  abandoned_checkouts: number
  abandoned_value: number
}
interface Row {
  name: string
  extra?: string
  sessions: number
  orders: number
  revenue: number
  conversion_rate: number
}
interface Overview {
  totals: Totals
  prev: Totals
  series: { date: string; sessions: number; visitors: number; orders: number; revenue: number }[]
  funnel: { key: string; count: number }[]
  hours: number[]
  sources: Row[]
  devices: Row[]
  browsers: Row[]
  countries: Row[]
  landing: Row[]
  pages: { path: string; views: number }[]
  products: { product_id: number; name: string; views: number; adds: number; orders: number; qty: number; revenue: number }[]
  clicks: { section: string; label: string; target: string; count: number }[]
  searches: { term: string; count: number; avg_results: number }[]
  events: Record<string, number>
  methods: { method: string; orders: number; paid: number; revenue: number }[]
}
interface CartLine {
  p: number
  q: number
  pr: number
  nm: string
}
interface Visitor {
  id: number
  sid: string
  step: string
  stage: number
  last_event: string
  last_path: string
  device: string
  country: string
  source: string
  medium: string
  returning: boolean
  cart_value: number
  cart_items: number
  cart: CartLine[]
  orders: number
  started_at: string
  last_seen: string
}
interface Live {
  online: number
  steps: Record<string, number>
  sessions: Visitor[]
}
interface Abandoned {
  id: number
  stage: number
  last_event: string
  device: string
  country: string
  source: string
  medium: string
  returning: boolean
  cart_value: number
  cart_items: number
  cart: CartLine[]
  started_at: string
  last_seen: string
}

const data = ref<Overview | null>(null)
const live = ref<Live | null>(null)
const abandoned = ref<Abandoned[]>([])
const loading = ref(false)
const errorMsg = ref('')
const metric = ref<'revenue' | 'orders' | 'sessions' | 'visitors'>('revenue')

async function load() {
  loading.value = true
  errorMsg.value = ''
  try {
    const params = { from: range.value.from, to: range.value.to, tzo }
    const [o, a] = await Promise.all([
      api.get(`/storefront/sites/${siteId.value}/analytics`, { params }),
      api.get(`/storefront/sites/${siteId.value}/analytics/abandoned`, { params })
    ])
    // Máy chủ có thể trả null cho danh sách rỗng: chuẩn hoá để bảng không lỗi
    const d = o.data as Overview
    for (const k of ['series', 'funnel', 'hours', 'sources', 'devices', 'browsers', 'countries', 'landing', 'pages', 'products', 'clicks', 'searches', 'methods'] as const) (d as any)[k] = d[k] || []
    d.events = d.events || {}
    data.value = d
    abandoned.value = a.data.abandoned || []
  } catch (e: any) {
    errorMsg.value = e?.response?.data?.error || 'Không tải được thống kê.'
  } finally {
    loading.value = false
  }
}

async function loadLive() {
  try {
    const { data: d } = await api.get(`/storefront/sites/${siteId.value}/analytics/live`)
    live.value = d
  } catch {
    /* lần sau thử lại */
  }
}

let liveTimer: ReturnType<typeof setInterval> | null = null
function startLive() {
  stopLive()
  void loadLive()
  liveTimer = setInterval(() => document.visibilityState === 'visible' && loadLive(), 10000)
}
function stopLive() {
  if (liveTimer) clearInterval(liveTimer)
  liveTimer = null
}

onMounted(async () => {
  try {
    const { data: d } = await api.get(`/storefront/sites/${siteId.value}`)
    siteTitle.value = d.site?.site_title || ''
  } catch {
    /* bỏ qua */
  }
  await load()
  startLive()
})
onBeforeUnmount(stopLive)
watch([preset, customFrom, customTo], () => {
  if (preset.value !== 'custom' || (customFrom.value && customTo.value && customFrom.value <= customTo.value)) void load()
})

// ------------------------------------------------------------------ Hiển thị
const nf = (n: number) => new Intl.NumberFormat(uiLocaleTag()).format(Math.round(n || 0))
const pct = (r: number, d = 1) => `${((r || 0) * 100).toFixed(d).replace(/\.0+$/, '')}%`
function dur(sec: number): string {
  if (!sec) return '0s'
  const m = Math.floor(sec / 60)
  return m ? `${m}m ${sec % 60}s` : `${sec}s`
}
function ago(iso: string): string {
  const s = Math.max(0, Math.round((Date.now() - new Date(iso).getTime()) / 1000))
  if (s < 45) return 'vừa xong'
  if (s < 3600) return `${Math.round(s / 60)} phút trước`
  if (s < 86400) return `${Math.round(s / 3600)} giờ trước`
  return new Date(iso).toLocaleDateString(uiLocaleTag())
}
function delta(cur: number, prev: number): { text: string; up: boolean; flat: boolean } {
  if (!prev && !cur) return { text: '—', up: false, flat: true }
  if (!prev) return { text: 'mới', up: true, flat: false }
  const d = ((cur - prev) / prev) * 100
  return { text: `${Math.abs(d).toFixed(Math.abs(d) < 10 ? 1 : 0)}%`, up: d >= 0, flat: Math.abs(d) < 0.05 }
}

const cards = computed(() => {
  const d = data.value
  if (!d) return []
  const t = d.totals
  const p = d.prev
  return [
    { k: 'revenue', label: 'Doanh thu', value: adminMoney(t.revenue), title: adminMoneyTitle(t.revenue), d: delta(t.revenue, p.revenue), sub: `${nf(t.paid_orders)} đơn đã thanh toán online` },
    { k: 'orders', label: 'Đơn hàng', value: nf(t.orders), d: delta(t.orders, p.orders), sub: t.cancelled_orders ? `${nf(t.cancelled_orders)} đơn đã huỷ` : 'Không tính đơn đã huỷ' },
    { k: 'sessions', label: 'Lượt truy cập', value: nf(t.sessions), d: delta(t.sessions, p.sessions), sub: `${nf(t.page_views)} lượt xem trang` },
    { k: 'visitors', label: 'Khách truy cập', value: nf(t.visitors), d: delta(t.visitors, p.visitors), sub: `${nf(t.new_visitors)} mới · ${nf(t.returning_visitors)} quay lại` },
    { k: 'conv', label: 'Tỷ lệ chuyển đổi', value: pct(t.conversion_rate, 2), d: delta(t.conversion_rate, p.conversion_rate), sub: 'Đơn hàng / lượt truy cập' },
    { k: 'aov', label: 'Giá trị đơn trung bình', value: adminMoney(t.aov), title: adminMoneyTitle(t.aov), d: delta(t.aov, p.aov), sub: 'Doanh thu / số đơn' },
    { k: 'cart', label: 'Tỷ lệ thêm vào giỏ', value: pct(t.add_to_cart_rate), d: null, sub: `${pct(t.checkout_rate)} vào trang thanh toán` },
    { k: 'bounce', label: 'Tỷ lệ thoát', value: pct(t.bounce_rate), d: null, sub: `Thời gian TB mỗi lượt: ${dur(t.avg_session_sec)}` }
  ]
})

// Biểu đồ cột theo ngày
const chart = computed(() => {
  const d = data.value
  if (!d) return { bars: [] as { x: number; y: number; w: number; h: number; label: string; v: number }[], max: 0, ticks: [] as number[], labels: [] as { x: number; t: string }[] }
  const W = 640
  const H = 170
  const vals = d.series.map((s) => s[metric.value])
  const max = Math.max(1, ...vals)
  const n = Math.max(1, vals.length)
  const gap = n > 40 ? 1 : 3
  // Ít ngày thì cột không bè ra hết chiều ngang: giới hạn bề rộng cột và căn giữa
  const w = Math.min(n <= 14 ? 44 : 9999, Math.max(2, (W - gap * (n - 1)) / n))
  const x0 = (W - (n * w + gap * (n - 1))) / 2
  const bars = vals.map((v, i) => {
    const h = v > 0 ? Math.max(2, (v / max) * H) : 0
    return { x: x0 + i * (w + gap), y: H - h, w, h, label: d.series[i]!.date, v }
  })
  const idx = n <= 8 ? bars.map((_, i) => i) : [0, Math.floor(n / 4), Math.floor(n / 2), Math.floor((3 * n) / 4), n - 1]
  return {
    bars,
    max,
    ticks: [0.25, 0.5, 0.75, 1].map((f) => max * f),
    labels: idx.map((i) => ({ x: bars[i]!.x + w / 2, t: d.series[i]!.date.slice(5) }))
  }
})
const metricLabel: Record<string, string> = { revenue: 'Doanh thu', orders: 'Đơn hàng', sessions: 'Lượt truy cập', visitors: 'Khách truy cập' }
const fmtMetric = (v: number) => (metric.value === 'revenue' ? adminMoney(v) : nf(v))

// Phễu
const funnelLabels: Record<string, string> = {
  sessions: 'Lượt truy cập',
  product_view: 'Xem sản phẩm',
  add_to_cart: 'Thêm vào giỏ hàng',
  checkout: 'Vào trang thanh toán',
  info: 'Bắt đầu nhập thông tin giao hàng',
  info_done: 'Điền đủ thông tin giao hàng',
  payment: 'Chọn phương thức thanh toán',
  submit: 'Bấm đặt hàng / thanh toán',
  ordered: 'Đặt hàng thành công'
}
const funnel = computed(() => {
  const f = data.value?.funnel || []
  const top = Math.max(1, f[0]?.count || 0)
  return f.map((r, i) => {
    const prev = i ? f[i - 1]!.count : r.count
    return { ...r, label: funnelLabels[r.key] || r.key, width: Math.max(r.count ? 2 : 0, (r.count / top) * 100), ofTotal: r.count / top, drop: i && prev > 0 ? 1 - r.count / prev : 0, lost: i ? Math.max(0, prev - r.count) : 0 }
  })
})
// Bước khách rời đi nhiều nhất
const worstStep = computed(() => {
  const rows = funnel.value.slice(1).filter((r) => r.lost > 0)
  return rows.length ? rows.reduce((a, b) => (b.lost > a.lost ? b : a)) : null
})

// Bước hiện tại của khách đang online
const stepMeta: Record<string, { l: string; c: string }> = {
  browsing: { l: 'Đang xem trang', c: 'bg-slate-100 text-slate-700 ring-slate-200' },
  product: { l: 'Đang xem sản phẩm', c: 'bg-sky-50 text-sky-700 ring-sky-200' },
  cart: { l: 'Có hàng trong giỏ', c: 'bg-amber-50 text-amber-700 ring-amber-200' },
  checkout: { l: 'Mở trang thanh toán', c: 'bg-indigo-50 text-indigo-700 ring-indigo-200' },
  info: { l: 'Đang nhập thông tin giao hàng', c: 'bg-violet-50 text-violet-700 ring-violet-200' },
  info_done: { l: 'Đã điền đủ thông tin', c: 'bg-fuchsia-50 text-fuchsia-700 ring-fuchsia-200' },
  payment: { l: 'Đang chọn thanh toán', c: 'bg-pink-50 text-pink-700 ring-pink-200' },
  submit: { l: 'Đang đặt hàng', c: 'bg-rose-50 text-rose-700 ring-rose-200' },
  ordered: { l: 'Vừa đặt hàng', c: 'bg-emerald-50 text-emerald-700 ring-emerald-200' }
}
const stepOrder = ['browsing', 'product', 'cart', 'checkout', 'info', 'info_done', 'payment', 'submit', 'ordered']
const stageLabels: Record<number, string> = {
  2: 'Thêm vào giỏ rồi rời đi',
  3: 'Rời ở trang thanh toán',
  4: 'Rời khi đang nhập thông tin',
  5: 'Rời sau khi điền đủ thông tin',
  6: 'Rời ở bước chọn thanh toán',
  7: 'Rời khi bấm đặt hàng'
}

const deviceLabels: Record<string, string> = { desktop: 'Máy tính', mobile: 'Điện thoại', tablet: 'Máy tính bảng', '-': 'Không rõ' }
const mediumLabels: Record<string, string> = { direct: 'Trực tiếp', search: 'Tìm kiếm', social: 'Mạng xã hội', email: 'Email', paid: 'Quảng cáo', referral: 'Giới thiệu' }
const payLabels: Record<string, string> = { cod: 'Thanh toán khi nhận hàng', vnpay: 'VNPay', stripe: 'Thẻ quốc tế', paypal: 'PayPal' }
const clickLabels: Record<string, string> = {
  header_cart: 'Biểu tượng giỏ hàng',
  header_search: 'Biểu tượng tìm kiếm',
  hero_cta: 'Nút chính ở banner',
  product_add_to_cart: 'Nút thêm vào giỏ ở thẻ sản phẩm',
  product_choose_options: 'Nút chọn phân loại ở thẻ sản phẩm'
}
const clickLabel = (l: string) => clickLabels[l] || l
const pageLabel = (p: string) => (p === '/' ? 'Trang chủ' : p === '/checkout' ? 'Thanh toán' : p === '/order/*' ? 'Trang đơn hàng' : p === '/account' ? 'Tài khoản khách' : p.startsWith('/p/') ? `Sản phẩm #${p.slice(3)}` : p)
const sourceLabel = (s: string) => (s === 'direct' ? 'Truy cập trực tiếp' : s)
function countryName(code: string): string {
  if (!code || code === '-') return 'Không rõ'
  try {
    return new Intl.DisplayNames([uiLocaleTag()], { type: 'region' }).of(code) || code
  } catch {
    return code
  }
}
const flag = (code: string) => (/^[A-Z]{2}$/.test(code) ? String.fromCodePoint(...[...code].map((c) => 0x1f1e6 + c.charCodeAt(0) - 65)) : '🌐')

const maxHour = computed(() => Math.max(1, ...(data.value?.hours || [0])))
const maxSessions = (rows: Row[]) => Math.max(1, ...rows.map((r) => r.sessions))
const cartText = (c: CartLine[]) => c.map((l) => `${l.nm || '#' + l.p} ×${l.q}`).join(', ')
const hasData = computed(() => !!data.value && (data.value.totals.sessions > 0 || data.value.totals.orders > 0))
</script>

<template>
  <div class="min-h-screen bg-slate-50 text-slate-900">
    <div class="mx-auto max-w-6xl space-y-5 px-4 py-6">
      <!-- Đầu trang -->
      <div class="flex flex-wrap items-center justify-between gap-3">
        <div class="flex items-center gap-3">
          <button class="rounded-xl border border-slate-200 bg-white p-2 hover:bg-slate-100" @click="router.push('/sites')"><ArrowLeft class="h-4 w-4" /></button>
          <div>
            <h1 class="text-xl font-extrabold">Thống kê website</h1>
            <p class="text-xs text-slate-500">{{ siteTitle }}</p>
          </div>
        </div>
        <div class="flex flex-wrap items-center gap-2">
          <div class="flex rounded-xl bg-white p-1 shadow-sm ring-1 ring-slate-200" data-testid="range-presets">
            <button
              v-for="p in presets"
              :key="p.v"
              class="rounded-lg px-3 py-1.5 text-xs font-bold"
              :class="preset === p.v ? 'bg-emerald-600 text-white' : 'text-slate-600 hover:bg-slate-50'"
              :data-preset="p.v"
              @click="preset = p.v"
            >
              {{ p.l }}
            </button>
          </div>
          <template v-if="preset === 'custom'">
            <input v-model="customFrom" type="date" class="rounded-lg border border-slate-200 bg-white px-2 py-1.5 text-xs" />
            <span class="text-xs text-slate-400">→</span>
            <input v-model="customTo" type="date" class="rounded-lg border border-slate-200 bg-white px-2 py-1.5 text-xs" />
          </template>
          <button class="rounded-xl border border-slate-200 bg-white p-2 hover:bg-slate-100" title="Tải lại" @click="load(); loadLive()">
            <RefreshCw class="h-4 w-4" :class="loading ? 'animate-spin' : ''" />
          </button>
        </div>
      </div>

      <p v-if="errorMsg" class="rounded-xl bg-rose-50 px-4 py-2 text-sm font-semibold text-rose-700">{{ errorMsg }}</p>

      <!-- ===================== XEM TRỰC TIẾP ===================== -->
      <section class="rounded-2xl bg-white p-5 shadow-sm ring-1 ring-slate-200" data-testid="live-panel">
        <div class="flex flex-wrap items-center justify-between gap-2">
          <h2 class="flex items-center gap-2 text-sm font-extrabold">
            <span class="relative flex h-2.5 w-2.5"><span class="absolute inline-flex h-full w-full animate-ping rounded-full bg-emerald-400 opacity-60" /><span class="relative inline-flex h-2.5 w-2.5 rounded-full bg-emerald-500" /></span>
            Khách đang online
            <span class="rounded-full bg-emerald-50 px-2 py-0.5 text-xs font-extrabold text-emerald-700" data-testid="live-online">{{ live?.online ?? 0 }}</span>
          </h2>
          <span class="text-[11px] text-slate-400">Cập nhật mỗi 10 giây · tính khách hoạt động trong 5 phút gần nhất</span>
        </div>
        <div class="mt-3 flex flex-wrap gap-2">
          <span v-for="k in stepOrder" :key="k" class="inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 text-xs font-bold ring-1" :class="[stepMeta[k]!.c, live?.steps?.[k] ? '' : 'opacity-40']" :data-step="k">
            {{ stepMeta[k]!.l }} <b class="text-sm" :data-testid="'live-step-' + k">{{ live?.steps?.[k] || 0 }}</b>
          </span>
        </div>
        <div v-if="live?.sessions?.length" class="mt-4 overflow-x-auto">
          <table class="w-full min-w-[640px] text-left text-xs">
            <thead class="text-[11px] uppercase tracking-wide text-slate-400">
              <tr><th class="py-1.5 pr-3">Khách</th><th class="pr-3">Đang ở bước</th><th class="pr-3">Giỏ hàng</th><th class="pr-3">Nguồn</th><th class="pr-3">Thiết bị</th><th>Hoạt động</th></tr>
            </thead>
            <tbody class="divide-y divide-slate-100">
              <tr v-for="v in live.sessions" :key="v.id" data-testid="live-row">
                <td class="py-2 pr-3 font-mono text-[11px] text-slate-500">#{{ v.sid }} <span v-if="v.returning" class="ml-1 rounded bg-slate-100 px-1 font-sans text-[10px] font-bold text-slate-500">quay lại</span></td>
                <td class="pr-3"><span class="rounded-full px-2 py-0.5 text-[11px] font-bold ring-1" :class="stepMeta[v.step]?.c">{{ stepMeta[v.step]?.l || v.step }}</span></td>
                <td class="pr-3">
                  <template v-if="v.cart_items"><span class="font-bold">{{ adminMoney(v.cart_value) }}</span> <span class="text-slate-400">· {{ cartText(v.cart) }}</span></template>
                  <span v-else class="text-slate-300">—</span>
                </td>
                <td class="pr-3">{{ sourceLabel(v.source) }}</td>
                <td class="pr-3">{{ flag(v.country) }} {{ deviceLabels[v.device] || v.device }}</td>
                <td class="text-slate-500">{{ ago(v.last_seen) }}</td>
              </tr>
            </tbody>
          </table>
        </div>
        <p v-else class="mt-4 text-xs text-slate-400">Chưa có khách nào đang xem website lúc này.</p>
      </section>

      <div v-if="loading && !data" class="flex items-center justify-center gap-2 py-16 text-sm text-slate-500"><Loader2 class="h-5 w-5 animate-spin" /> Đang tải...</div>

      <template v-else-if="data">
        <p v-if="!hasData" class="rounded-2xl bg-white p-6 text-center text-sm text-slate-500 shadow-sm ring-1 ring-slate-200" data-testid="no-data">
          Chưa có dữ liệu trong khoảng ngày này. Thống kê bắt đầu ghi nhận từ khi khách truy cập website đã xuất bản (lượt xem của bạn khi đang đăng nhập không được tính).
        </p>

        <!-- ===================== CHỈ SỐ CHÍNH ===================== -->
        <section class="grid grid-cols-2 gap-3 lg:grid-cols-4" data-testid="kpis">
          <div v-for="c in cards" :key="c.k" class="rounded-2xl bg-white p-4 shadow-sm ring-1 ring-slate-200" :data-kpi="c.k">
            <div class="text-[11px] font-bold uppercase tracking-wide text-slate-400">{{ c.label }}</div>
            <div class="mt-1 flex items-baseline gap-2">
              <span class="text-xl font-extrabold sm:text-2xl" :title="c.title" data-testid="kpi-value">{{ c.value }}</span>
              <span v-if="c.d && !c.d.flat" class="inline-flex items-center text-[11px] font-bold" :class="c.d.up ? 'text-emerald-600' : 'text-rose-600'">
                <component :is="c.d.up ? ArrowUp : ArrowDown" class="h-3 w-3" />{{ c.d.text }}
              </span>
            </div>
            <div class="mt-1 text-[11px] text-slate-500">{{ c.sub }}</div>
          </div>
        </section>

        <!-- ===================== BIỂU ĐỒ ===================== -->
        <section class="rounded-2xl bg-white p-5 shadow-sm ring-1 ring-slate-200">
          <div class="flex flex-wrap items-center justify-between gap-2">
            <h2 class="text-sm font-extrabold">Theo ngày</h2>
            <div class="flex rounded-lg bg-slate-100 p-0.5">
              <button v-for="m in (['revenue', 'orders', 'sessions', 'visitors'] as const)" :key="m" class="rounded-md px-2.5 py-1 text-[11px] font-bold" :class="metric === m ? 'bg-white text-slate-900 shadow-sm' : 'text-slate-500'" :data-metric="m" @click="metric = m">
                {{ metricLabel[m] }}
              </button>
            </div>
          </div>
          <div class="mt-4">
            <svg viewBox="0 0 640 200" class="h-48 w-full" role="img" data-testid="chart">
              <g v-for="(tk, i) in chart.ticks" :key="i">
                <line x1="0" x2="640" :y1="170 - (tk / chart.max) * 170" :y2="170 - (tk / chart.max) * 170" stroke="#e2e8f0" stroke-dasharray="3 4" />
              </g>
              <rect v-for="b in chart.bars" :key="b.label" :x="b.x" :y="b.y" :width="b.w" :height="b.h" rx="2" fill="#10b981" opacity="0.9" data-testid="chart-bar">
                <title>{{ b.label }}: {{ fmtMetric(b.v) }}</title>
              </rect>
              <text v-for="l in chart.labels" :key="l.x" :x="l.x" y="192" text-anchor="middle" font-size="11" fill="#94a3b8">{{ l.t }}</text>
            </svg>
            <p class="mt-1 text-right text-[11px] text-slate-400">Cao nhất: {{ fmtMetric(chart.max) }}</p>
          </div>
        </section>

        <!-- ===================== PHỄU THANH TOÁN ===================== -->
        <section class="rounded-2xl bg-white p-5 shadow-sm ring-1 ring-slate-200" data-testid="funnel">
          <h2 class="text-sm font-extrabold">Phễu mua hàng: khách dừng lại ở bước nào?</h2>
          <p class="mt-0.5 text-xs text-slate-500">Mỗi thanh là số lượt truy cập đã đi tới bước đó; phần trăm đỏ là tỉ lệ khách rời đi so với bước liền trước.</p>
          <div class="mt-4 space-y-2.5">
            <div v-for="(r, i) in funnel" :key="r.key" data-testid="funnel-row" :data-key="r.key">
              <div class="flex items-baseline justify-between gap-3 text-xs">
                <span class="font-semibold text-slate-700">{{ i + 1 }}. {{ r.label }}</span>
                <span class="shrink-0"><b class="text-sm">{{ nf(r.count) }}</b> <span class="text-slate-400">({{ pct(r.ofTotal) }})</span> <span v-if="i && r.drop > 0" class="ml-1 font-bold text-rose-500">−{{ pct(r.drop, 0) }}</span></span>
              </div>
              <div class="mt-1 h-3 overflow-hidden rounded-full bg-slate-100">
                <div class="h-full rounded-full" :style="{ width: r.width + '%', backgroundColor: r.key === 'ordered' ? '#10b981' : '#6366f1' }" />
              </div>
            </div>
          </div>
          <p v-if="worstStep" class="mt-4 rounded-xl bg-amber-50 px-3 py-2 text-xs font-semibold text-amber-800" data-testid="worst-step">
            Khách rời đi nhiều nhất ở bước "{{ worstStep.label }}": mất {{ nf(worstStep.lost) }} lượt so với bước trước (−{{ pct(worstStep.drop, 0) }}).
          </p>
        </section>

        <!-- ===================== BỎ DỞ ===================== -->
        <section class="rounded-2xl bg-white p-5 shadow-sm ring-1 ring-slate-200" data-testid="abandoned">
          <div class="flex flex-wrap items-center justify-between gap-2">
            <h2 class="flex items-center gap-2 text-sm font-extrabold"><ShoppingCart class="h-4 w-4" /> Giỏ hàng & thanh toán bỏ dở</h2>
            <div class="text-xs text-slate-500">
              <b data-testid="ab-checkouts">{{ nf(data.totals.abandoned_checkouts) }}</b> bỏ ở thanh toán ·
              <b data-testid="ab-carts">{{ nf(data.totals.abandoned_carts) }}</b> bỏ ở giỏ hàng · giá trị
              <b :title="adminMoneyTitle(data.totals.abandoned_value)">{{ adminMoney(data.totals.abandoned_value) }}</b>
            </div>
          </div>
          <p class="mt-0.5 text-xs text-slate-500">Khách đã thêm hàng vào giỏ / vào thanh toán nhưng không đặt hàng và im lặng quá 30 phút. Hệ thống không lưu họ tên hay số điện thoại của khách bỏ dở.</p>
          <div v-if="abandoned.length" class="mt-3 overflow-x-auto">
            <table class="w-full min-w-[640px] text-left text-xs">
              <thead class="text-[11px] uppercase tracking-wide text-slate-400">
                <tr><th class="py-1.5 pr-3">Thời gian</th><th class="pr-3">Khách rời đi ở</th><th class="pr-3">Giỏ hàng</th><th class="pr-3">Nguồn</th><th>Thiết bị</th></tr>
              </thead>
              <tbody class="divide-y divide-slate-100">
                <tr v-for="a in abandoned" :key="a.id" data-testid="abandoned-row">
                  <td class="py-2 pr-3 text-slate-500">{{ ago(a.last_seen) }}</td>
                  <td class="pr-3 font-semibold">{{ stageLabels[a.stage] || 'Rời đi' }}</td>
                  <td class="pr-3"><b>{{ adminMoney(a.cart_value) }}</b> <span class="text-slate-400">· {{ cartText(a.cart) }}</span></td>
                  <td class="pr-3">{{ sourceLabel(a.source) }}</td>
                  <td>{{ flag(a.country) }} {{ deviceLabels[a.device] || a.device }}</td>
                </tr>
              </tbody>
            </table>
          </div>
          <p v-else class="mt-4 text-xs text-slate-400">Chưa có giỏ hàng bỏ dở trong khoảng ngày này.</p>
        </section>

        <!-- ===================== PHÂN TÍCH ===================== -->
        <div class="grid grid-cols-1 gap-5 lg:grid-cols-2">
          <!-- Nguồn truy cập -->
          <section class="rounded-2xl bg-white p-5 shadow-sm ring-1 ring-slate-200" data-testid="sources">
            <h2 class="text-sm font-extrabold">Nguồn truy cập</h2>
            <table v-if="data.sources.length" class="mt-3 w-full text-left text-xs">
              <thead class="text-[11px] uppercase tracking-wide text-slate-400"><tr><th class="py-1.5">Nguồn</th><th class="text-right">Lượt</th><th class="text-right">Đơn</th><th class="text-right">Doanh thu</th><th class="text-right">CR</th></tr></thead>
              <tbody class="divide-y divide-slate-100">
                <tr v-for="r in data.sources" :key="r.name" data-testid="source-row">
                  <td class="py-1.5">
                    <div class="font-semibold">{{ sourceLabel(r.name) }}</div>
                    <div class="text-[10px] text-slate-400">{{ mediumLabels[r.extra || ''] || r.extra }}</div>
                    <div class="mt-1 h-1 rounded bg-slate-100"><div class="h-1 rounded bg-indigo-400" :style="{ width: (r.sessions / maxSessions(data.sources)) * 100 + '%' }" /></div>
                  </td>
                  <td class="text-right">{{ nf(r.sessions) }}</td><td class="text-right">{{ nf(r.orders) }}</td><td class="text-right" :title="adminMoneyTitle(r.revenue)">{{ adminMoney(r.revenue) }}</td><td class="text-right">{{ pct(r.conversion_rate) }}</td>
                </tr>
              </tbody>
            </table>
            <p v-else class="mt-3 text-xs text-slate-400">Chưa có dữ liệu.</p>
          </section>

          <!-- Sản phẩm -->
          <section class="rounded-2xl bg-white p-5 shadow-sm ring-1 ring-slate-200" data-testid="products">
            <h2 class="text-sm font-extrabold">Sản phẩm</h2>
            <table v-if="data.products.length" class="mt-3 w-full text-left text-xs">
              <thead class="text-[11px] uppercase tracking-wide text-slate-400"><tr><th class="py-1.5">Sản phẩm</th><th class="text-right">Xem</th><th class="text-right">Thêm giỏ</th><th class="text-right">Đã bán</th><th class="text-right">Doanh thu</th></tr></thead>
              <tbody class="divide-y divide-slate-100">
                <tr v-for="p in data.products" :key="p.product_id" data-testid="product-row">
                  <td class="max-w-[180px] truncate py-1.5 font-semibold" :title="p.name">{{ p.name }}</td>
                  <td class="text-right">{{ nf(p.views) }}</td><td class="text-right">{{ nf(p.adds) }}</td><td class="text-right">{{ nf(p.qty) }}</td><td class="text-right" :title="adminMoneyTitle(p.revenue)">{{ adminMoney(p.revenue) }}</td>
                </tr>
              </tbody>
            </table>
            <p v-else class="mt-3 text-xs text-slate-400">Chưa có dữ liệu.</p>
          </section>

          <!-- Click nhiều nhất -->
          <section class="rounded-2xl bg-white p-5 shadow-sm ring-1 ring-slate-200" data-testid="clicks">
            <div class="flex items-center justify-between">
              <h2 class="flex items-center gap-2 text-sm font-extrabold"><Activity class="h-4 w-4" /> Lượt click nhiều nhất</h2>
              <span class="text-[11px] text-slate-400">{{ nf(data.events.click || 0) }} lượt click</span>
            </div>
            <table v-if="data.clicks.length" class="mt-3 w-full text-left text-xs">
              <thead class="text-[11px] uppercase tracking-wide text-slate-400"><tr><th class="py-1.5">Nút / liên kết</th><th>Section</th><th class="text-right">Lượt</th></tr></thead>
              <tbody class="divide-y divide-slate-100">
                <tr v-for="(c, i) in data.clicks" :key="i" data-testid="click-row">
                  <td class="max-w-[200px] truncate py-1.5 font-semibold" :title="c.target">{{ clickLabel(c.label) }}</td>
                  <td class="text-slate-500">{{ blockLabels[c.section] || c.section || '—' }}</td>
                  <td class="text-right font-bold">{{ nf(c.count) }}</td>
                </tr>
              </tbody>
            </table>
            <p v-else class="mt-3 text-xs text-slate-400">Chưa có lượt click nào.</p>
          </section>

          <!-- Thiết bị & Quốc gia -->
          <section class="rounded-2xl bg-white p-5 shadow-sm ring-1 ring-slate-200" data-testid="devices">
            <h2 class="text-sm font-extrabold">Thiết bị</h2>
            <div class="mt-3 space-y-2">
              <div v-for="r in data.devices" :key="r.name" data-testid="device-row">
                <div class="flex justify-between text-xs"><span class="font-semibold">{{ deviceLabels[r.name] || r.name }}</span><span>{{ nf(r.sessions) }} <span class="text-slate-400">· CR {{ pct(r.conversion_rate) }}</span></span></div>
                <div class="mt-1 h-2 rounded-full bg-slate-100"><div class="h-2 rounded-full bg-sky-400" :style="{ width: (r.sessions / maxSessions(data.devices)) * 100 + '%' }" /></div>
              </div>
              <p v-if="!data.devices.length" class="text-xs text-slate-400">Chưa có dữ liệu.</p>
            </div>
            <h2 class="mt-5 text-sm font-extrabold">Quốc gia <span class="text-[11px] font-normal text-slate-400">(theo ngôn ngữ trình duyệt của khách)</span></h2>
            <div class="mt-3 space-y-1.5">
              <div v-for="r in data.countries.slice(0, 8)" :key="r.name" class="flex justify-between text-xs" data-testid="country-row"><span>{{ flag(r.name) }} {{ countryName(r.name) }}</span><span>{{ nf(r.sessions) }} <span class="text-slate-400">· {{ nf(r.orders) }} đơn</span></span></div>
              <p v-if="!data.countries.length" class="text-xs text-slate-400">Chưa có dữ liệu.</p>
            </div>
          </section>

          <!-- Trang & tìm kiếm -->
          <section class="rounded-2xl bg-white p-5 shadow-sm ring-1 ring-slate-200">
            <h2 class="text-sm font-extrabold">Trang được xem nhiều</h2>
            <div class="mt-3 space-y-1.5">
              <div v-for="p in data.pages" :key="p.path" class="flex justify-between text-xs"><span class="truncate">{{ pageLabel(p.path) }}</span><b>{{ nf(p.views) }}</b></div>
              <p v-if="!data.pages.length" class="text-xs text-slate-400">Chưa có dữ liệu.</p>
            </div>
            <h2 class="mt-5 text-sm font-extrabold">Từ khoá khách tìm kiếm</h2>
            <div class="mt-3 space-y-1.5" data-testid="searches">
              <div v-for="s in data.searches" :key="s.term" class="flex justify-between text-xs"><span class="truncate">“{{ s.term }}”</span><span><b>{{ nf(s.count) }}</b> <span class="text-slate-400">· {{ s.avg_results ? 'có kết quả' : 'không có kết quả' }}</span></span></div>
              <p v-if="!data.searches.length" class="text-xs text-slate-400">Chưa có ai tìm kiếm.</p>
            </div>
          </section>

          <!-- Thanh toán & giờ cao điểm -->
          <section class="rounded-2xl bg-white p-5 shadow-sm ring-1 ring-slate-200">
            <h2 class="text-sm font-extrabold">Phương thức thanh toán</h2>
            <div class="mt-3 space-y-1.5" data-testid="methods">
              <div v-for="m in data.methods" :key="m.method" class="flex justify-between text-xs"><span class="font-semibold">{{ payLabels[m.method] || m.method }}</span><span>{{ nf(m.orders) }} đơn <span class="text-slate-400">· {{ nf(m.paid) }} đã thanh toán · {{ adminMoney(m.revenue) }}</span></span></div>
              <p v-if="!data.methods.length" class="text-xs text-slate-400">Chưa có đơn hàng.</p>
            </div>
            <h2 class="mt-5 text-sm font-extrabold">Giờ khách truy cập nhiều</h2>
            <div class="mt-3 flex h-20 items-end gap-0.5" data-testid="hours">
              <div v-for="(n, h) in data.hours" :key="h" class="group relative flex-1 rounded-t bg-violet-300 hover:bg-violet-500" :style="{ height: Math.max(n ? 6 : 1, (n / maxHour) * 100) + '%' }" :title="`${h}:00 — ${n}`" />
            </div>
            <div class="mt-1 flex justify-between text-[10px] text-slate-400"><span>0h</span><span>6h</span><span>12h</span><span>18h</span><span>23h</span></div>
          </section>
        </div>

        <p class="pb-6 text-center text-[11px] leading-relaxed text-slate-400">
          Thống kê dùng mã ẩn danh trong trình duyệt khách, không lưu địa chỉ IP, họ tên hay số điện thoại. Dữ liệu chi tiết giữ 60 ngày, số liệu theo phiên giữ 180 ngày. Khách bật "Do Not Track" và chính bạn khi đang đăng nhập sẽ không được tính.
        </p>
      </template>
    </div>
  </div>
</template>
