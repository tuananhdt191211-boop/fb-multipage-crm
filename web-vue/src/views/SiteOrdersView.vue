<script setup lang="ts">
// Quản lý đơn hàng, cài đặt thanh toán & email đăng ký của một website
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ArrowLeft, Copy, CreditCard, Loader2, Mail, Package, Phone, RefreshCw, Search, X } from 'lucide-vue-next'
import { api } from '../lib/api'
import { adminMoney, adminMoneyTitle, uiLocaleTag } from '../lib/uiI18n'
import type { ShopOrder } from '../types/store'

const route = useRoute()
const router = useRouter()
const siteId = computed(() => String(route.params.id || ''))
const tab = ref<'orders' | 'payment' | 'subscribers'>('orders')

const statusLabels: Record<string, string> = {
  new: 'Mới',
  confirmed: 'Đã xác nhận',
  shipping: 'Đang giao',
  completed: 'Hoàn thành',
  cancelled: 'Đã huỷ'
}
const statusClass: Record<string, string> = {
  new: 'bg-sky-50 text-sky-700 ring-sky-200',
  confirmed: 'bg-violet-50 text-violet-700 ring-violet-200',
  shipping: 'bg-amber-50 text-amber-700 ring-amber-200',
  completed: 'bg-emerald-50 text-emerald-700 ring-emerald-200',
  cancelled: 'bg-slate-100 text-slate-500 ring-slate-200'
}
const payLabels: Record<string, string> = { unpaid: 'Chưa thu tiền', pending: 'Chờ thanh toán', paid: 'Đã thanh toán', failed: 'Thanh toán lỗi' }

// ------------------------------------------------------------ Đơn hàng
const orders = ref<ShopOrder[]>([])
const counts = ref<Record<string, number>>({})
const loadingOrders = ref(false)
const filter = ref('all')
const q = ref('')
const errorMsg = ref('')
const okMsg = ref('')
const detail = ref<ShopOrder | null>(null)
const updating = ref(false)

async function loadOrders() {
  loadingOrders.value = true
  errorMsg.value = ''
  try {
    const { data } = await api.get(`/storefront/sites/${siteId.value}/orders`, { params: { status: filter.value, q: q.value.trim() || undefined } })
    orders.value = data.orders || []
    counts.value = data.counts || {}
  } catch (e: any) {
    errorMsg.value = e?.response?.data?.error || 'Không tải được đơn hàng.'
  } finally {
    loadingOrders.value = false
  }
}

const totalCount = computed(() => Object.values(counts.value).reduce((a, b) => a + b, 0))

async function updateOrder(o: ShopOrder, body: Record<string, string>) {
  updating.value = true
  errorMsg.value = ''
  try {
    const { data } = await api.patch(`/storefront/sites/${siteId.value}/orders/${o.id}`, body)
    Object.assign(o, data.order)
    if (detail.value?.id === o.id) detail.value = { ...o }
    await loadOrders()
  } catch (e: any) {
    errorMsg.value = e?.response?.data?.error || 'Cập nhật đơn thất bại.'
  } finally {
    updating.value = false
  }
}

function fmtTime(s: string) {
  return new Date(s).toLocaleString(uiLocaleTag(), { hour: '2-digit', minute: '2-digit', day: '2-digit', month: '2-digit', year: 'numeric' })
}

let searchTimer: ReturnType<typeof setTimeout> | null = null
watch(q, () => {
  if (searchTimer) clearTimeout(searchTimer)
  searchTimer = setTimeout(loadOrders, 350)
})
watch(filter, loadOrders)

// ------------------------------------------------------------ Thanh toán
const pay = ref({
  cod_enabled: true,
  vnp_enabled: false,
  vnp_tmn_code: '',
  vnp_hash_secret: '',
  vnp_sandbox: true,
  shipping_fee: 0,
  free_ship_from: 0,
  // Quốc tế
  paypal_enabled: false,
  paypal_client_id: '',
  paypal_secret: '',
  paypal_sandbox: true,
  stripe_enabled: false,
  stripe_secret: '',
  stripe_webhook_secret: ''
})
const secretSet = ref(false)
const intl = ref({ paypal_secret_set: false, stripe_secret_set: false, stripe_webhook_set: false, stripe_webhook_url: '' })
function applyPayResp(data: any) {
  Object.assign(pay.value, data.settings, { vnp_hash_secret: '', paypal_secret: '', stripe_secret: '', stripe_webhook_secret: '' })
  secretSet.value = !!data.vnp_secret_set
  intl.value = {
    paypal_secret_set: !!data.paypal_secret_set,
    stripe_secret_set: !!data.stripe_secret_set,
    stripe_webhook_set: !!data.stripe_webhook_set,
    stripe_webhook_url: data.stripe_webhook_url || intl.value.stripe_webhook_url
  }
}
// Số tiền ngoại tệ (khớp currencyDecimals ở backend)
function fmtForeign(o: { currency?: string; foreign_minor?: number; payment_method: string }) {
  if (!o.currency || !o.foreign_minor) return ''
  const zero = ['JPY', 'CLP'].includes(o.currency) || (o.payment_method === 'paypal' && ['HUF', 'TWD'].includes(o.currency))
  return ' · ' + (zero ? String(o.foreign_minor) : (o.foreign_minor / 100).toFixed(2)) + ' ' + o.currency
}
function payLabel(m: string) {
  return m === 'cod' ? 'COD' : m === 'paypal' ? 'PayPal' : m === 'stripe' ? 'Thẻ QT (Stripe)' : 'VNPay'
}
const ipnUrl = ref('')
const savingPay = ref(false)

async function loadPayment() {
  try {
    const { data } = await api.get(`/storefront/sites/${siteId.value}/payment-settings`)
    applyPayResp(data)
    ipnUrl.value = data.vnp_ipn_url || ''
  } catch (e: any) {
    errorMsg.value = e?.response?.data?.error || 'Không tải được cài đặt thanh toán.'
  }
}

async function savePayment() {
  savingPay.value = true
  errorMsg.value = ''
  okMsg.value = ''
  try {
    const { data } = await api.put(`/storefront/sites/${siteId.value}/payment-settings`, pay.value)
    applyPayResp(data)
    okMsg.value = 'Đã lưu cài đặt thanh toán.'
  } catch (e: any) {
    errorMsg.value = e?.response?.data?.error || 'Lưu cài đặt thất bại.'
  } finally {
    savingPay.value = false
  }
}

async function copy(text: string) {
  try {
    await navigator.clipboard.writeText(text)
    okMsg.value = 'Đã sao chép.'
  } catch {
    /* trình duyệt chặn */
  }
}

// ------------------------------------------------------------ Email đăng ký
const subscribers = ref<{ id: number; email: string; created_at: string }[]>([])
async function loadSubscribers() {
  try {
    const { data } = await api.get(`/storefront/sites/${siteId.value}/subscribers`)
    subscribers.value = data.subscribers || []
  } catch (e: any) {
    errorMsg.value = e?.response?.data?.error || 'Không tải được danh sách email.'
  }
}

watch(tab, (t) => {
  okMsg.value = ''
  if (t === 'payment') loadPayment()
  if (t === 'subscribers') loadSubscribers()
})

onMounted(loadOrders)
</script>

<template>
  <div class="min-h-screen bg-slate-50 text-slate-900">
    <div class="mx-auto max-w-6xl space-y-5 px-4 py-6">
      <div class="flex flex-wrap items-center justify-between gap-3">
        <div class="flex items-center gap-3">
          <button class="rounded-xl border border-slate-200 bg-white p-2 hover:bg-slate-100" @click="router.push('/sites')"><ArrowLeft class="h-4 w-4" /></button>
          <div>
            <h1 class="text-xl font-extrabold">Đơn hàng & thanh toán</h1>
            <p class="text-xs text-slate-500">{{ totalCount }} đơn hàng</p>
          </div>
        </div>
        <div class="flex rounded-xl bg-white p-1 shadow-sm ring-1 ring-slate-200">
          <button
            v-for="t in [{ v: 'orders', l: 'Đơn hàng' }, { v: 'payment', l: 'Thanh toán & ship' }, { v: 'subscribers', l: 'Email đăng ký' }] as const"
            :key="t.v"
            class="rounded-lg px-3 py-1.5 text-xs font-bold"
            :class="tab === t.v ? 'bg-emerald-600 text-white' : 'text-slate-600 hover:bg-slate-50'"
            @click="tab = t.v"
          >
            {{ t.l }}
          </button>
        </div>
      </div>

      <p v-if="errorMsg" class="rounded-xl bg-rose-50 px-4 py-2 text-sm font-semibold text-rose-700">{{ errorMsg }}</p>
      <p v-if="okMsg" class="rounded-xl bg-emerald-50 px-4 py-2 text-sm font-semibold text-emerald-700">{{ okMsg }}</p>

      <!-- ===================== ĐƠN HÀNG ===================== -->
      <template v-if="tab === 'orders'">
        <div class="flex flex-wrap items-center gap-2">
          <button
            v-for="f in ['all', 'new', 'confirmed', 'shipping', 'completed', 'cancelled']"
            :key="f"
            class="rounded-full px-3 py-1.5 text-xs font-bold ring-1"
            :class="filter === f ? 'bg-slate-900 text-white ring-slate-900' : 'bg-white text-slate-600 ring-slate-200 hover:bg-slate-50'"
            @click="filter = f"
          >
            {{ f === 'all' ? 'Tất cả' : statusLabels[f] }}
            <span v-if="f !== 'all' && counts[f]" class="ml-0.5 opacity-70">({{ counts[f] }})</span>
          </button>
          <div class="ml-auto flex items-center gap-2 rounded-xl bg-white px-3 py-1.5 ring-1 ring-slate-200">
            <Search class="h-4 w-4 text-slate-400" />
            <input v-model="q" placeholder="Mã đơn, SĐT, tên khách" class="w-44 bg-transparent text-sm outline-none" />
          </div>
          <button class="rounded-xl bg-white p-2 ring-1 ring-slate-200 hover:bg-slate-50" title="Tải lại" @click="loadOrders"><RefreshCw class="h-4 w-4" :class="{ 'animate-spin': loadingOrders }" /></button>
        </div>

        <div class="overflow-hidden rounded-2xl bg-white ring-1 ring-slate-200">
          <div v-if="loadingOrders && !orders.length" class="flex items-center justify-center gap-2 py-16 text-sm text-slate-500"><Loader2 class="h-4 w-4 animate-spin" /> Đang tải...</div>
          <div v-else-if="!orders.length" class="flex flex-col items-center gap-2 py-16 text-center text-sm text-slate-500">
            <Package class="h-10 w-10 text-slate-300" />
            <p class="font-semibold">Chưa có đơn hàng</p>
            <p class="text-xs">Đơn khách đặt trên website sẽ hiện ở đây.</p>
          </div>
          <!-- Điện thoại: dạng thẻ -->
          <ul v-if="orders.length" class="divide-y divide-slate-100 md:hidden">
            <li v-for="o in orders" :key="'m' + o.id" class="cursor-pointer space-y-1.5 px-4 py-3.5 active:bg-slate-50" @click="detail = { ...o }">
              <div class="flex items-center justify-between gap-2">
                <span class="text-sm font-bold">{{ o.code }}</span>
                <span class="rounded-full px-2 py-0.5 text-[11px] font-bold ring-1" :class="statusClass[o.status]">{{ statusLabels[o.status] }}</span>
              </div>
              <div class="flex items-center justify-between gap-2 text-sm">
                <span class="min-w-0 truncate">{{ o.customer_name }} · <span class="text-slate-500">{{ o.phone }}</span></span>
                <span class="shrink-0 font-bold" :title="adminMoneyTitle(o.total)">{{ adminMoney(o.total) }}</span>
              </div>
              <div class="flex items-center justify-between text-xs text-slate-500">
                <span>
                  {{ payLabel(o.payment_method) }} ·
                  <span :class="o.payment_status === 'paid' ? 'text-emerald-600' : o.payment_status === 'failed' ? 'text-rose-600' : ''">{{ payLabels[o.payment_status] }}</span>
                </span>
                <span>{{ fmtTime(o.created_at) }}</span>
              </div>
            </li>
          </ul>
          <div v-if="orders.length" class="hidden overflow-x-auto md:block">
            <table class="w-full min-w-[760px] text-sm">
              <thead class="bg-slate-50 text-left text-xs font-bold uppercase tracking-wide text-slate-500">
                <tr>
                  <th class="px-4 py-3">Mã đơn</th>
                  <th class="px-4 py-3">Khách hàng</th>
                  <th class="px-4 py-3 text-right">Tổng tiền</th>
                  <th class="px-4 py-3">Thanh toán</th>
                  <th class="px-4 py-3">Trạng thái</th>
                  <th class="px-4 py-3">Thời gian</th>
                </tr>
              </thead>
              <tbody class="divide-y divide-slate-100">
                <tr v-for="o in orders" :key="o.id" class="cursor-pointer hover:bg-slate-50" @click="detail = { ...o }">
                  <td class="px-4 py-3 font-bold">{{ o.code }}</td>
                  <td class="px-4 py-3">
                    <div class="font-semibold">{{ o.customer_name }}</div>
                    <div class="text-xs text-slate-500">{{ o.phone }}</div>
                  </td>
                  <td class="px-4 py-3 text-right font-bold" :title="adminMoneyTitle(o.total)">{{ adminMoney(o.total) }}</td>
                  <td class="px-4 py-3">
                    <div class="text-xs font-semibold">{{ payLabel(o.payment_method) }}</div>
                    <div class="text-xs" :class="o.payment_status === 'paid' ? 'text-emerald-600' : o.payment_status === 'failed' ? 'text-rose-600' : 'text-slate-500'">{{ payLabels[o.payment_status] }}</div>
                  </td>
                  <td class="px-4 py-3">
                    <span class="rounded-full px-2 py-0.5 text-xs font-bold ring-1" :class="statusClass[o.status]">{{ statusLabels[o.status] }}</span>
                  </td>
                  <td class="px-4 py-3 text-xs text-slate-500">{{ fmtTime(o.created_at) }}</td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
      </template>

      <!-- ===================== THANH TOÁN ===================== -->
      <template v-else-if="tab === 'payment'">
        <form class="grid grid-cols-1 gap-5 lg:grid-cols-2" @submit.prevent="savePayment">
          <section class="space-y-4 rounded-2xl bg-white p-5 ring-1 ring-slate-200">
            <h2 class="flex items-center gap-2 font-extrabold"><Package class="h-4 w-4" /> Giao hàng & COD</h2>
            <label class="flex items-center gap-2 text-sm font-semibold">
              <input v-model="pay.cod_enabled" type="checkbox" /> Cho phép thanh toán khi nhận hàng (COD)
            </label>
            <div class="grid grid-cols-2 gap-3">
              <label class="space-y-1">
                <span class="text-xs font-bold text-slate-600">Phí vận chuyển (đ)</span>
                <input v-model.number="pay.shipping_fee" type="number" min="0" step="1000" class="w-full rounded-lg border border-slate-200 px-3 py-2 text-sm" />
              </label>
              <label class="space-y-1">
                <span class="text-xs font-bold text-slate-600">Miễn phí ship từ (đ)</span>
                <input v-model.number="pay.free_ship_from" type="number" min="0" step="1000" class="w-full rounded-lg border border-slate-200 px-3 py-2 text-sm" />
              </label>
            </div>
            <p class="text-xs text-slate-500">Để 0 nếu không thu phí ship / không có mức miễn phí.</p>
          </section>

          <section class="space-y-4 rounded-2xl bg-white p-5 ring-1 ring-slate-200">
            <h2 class="flex items-center gap-2 font-extrabold"><CreditCard class="h-4 w-4" /> Thanh toán online VNPay</h2>
            <p class="text-xs leading-relaxed text-slate-500">
              Cần tài khoản merchant VNPay của chính shop. Đăng ký tại vnpay.vn để nhận <b>TmnCode</b> và <b>Hash Secret</b>
              (có tài khoản thử nghiệm sandbox để test trước).
            </p>
            <div class="grid grid-cols-2 gap-3">
              <label class="space-y-1">
                <span class="text-xs font-bold text-slate-600">TmnCode</span>
                <input v-model="pay.vnp_tmn_code" autocomplete="off" class="w-full rounded-lg border border-slate-200 px-3 py-2 font-mono text-sm" />
              </label>
              <label class="space-y-1">
                <span class="text-xs font-bold text-slate-600">Hash Secret</span>
                <input v-model="pay.vnp_hash_secret" type="password" autocomplete="new-password" :placeholder="secretSet ? '•••••• (đã lưu, để trống = giữ nguyên)' : 'Dán khoá bí mật'" class="w-full rounded-lg border border-slate-200 px-3 py-2 font-mono text-sm" />
              </label>
            </div>
            <div class="flex flex-wrap items-center gap-4">
              <label class="flex items-center gap-2 text-sm font-semibold"><input v-model="pay.vnp_sandbox" type="checkbox" /> Chế độ thử nghiệm (sandbox)</label>
              <label class="flex items-center gap-2 text-sm font-semibold"><input v-model="pay.vnp_enabled" type="checkbox" /> Bật VNPay cho khách</label>
            </div>
            <div class="space-y-1 rounded-xl bg-amber-50 p-3 text-xs text-amber-900">
              <p class="font-bold">Bắt buộc: khai báo IPN URL trong trang quản trị VNPay</p>
              <div class="flex items-center gap-2">
                <code class="min-w-0 flex-1 break-all rounded bg-white/70 px-2 py-1">{{ ipnUrl }}</code>
                <button type="button" class="rounded p-1 hover:bg-amber-100" title="Sao chép" @click="copy(ipnUrl)"><Copy class="h-3.5 w-3.5" /></button>
              </div>
              <p>Không khai báo thì đơn VNPay sẽ không tự chuyển sang "Đã thanh toán".</p>
            </div>
          </section>

          <!-- ===== Thanh toán quốc tế: PayPal ===== -->
          <section class="space-y-3 rounded-2xl bg-white p-5 ring-1 ring-slate-200">
            <h2 class="font-extrabold">PayPal · khách nước ngoài trả bằng ngoại tệ</h2>
            <p class="text-xs text-slate-500">
              Cần tài khoản <b>PayPal Business</b> (đăng ký được ở Việt Nam). Vào developer.paypal.com → Apps &amp; Credentials → tạo App để lấy <b>Client ID</b> và <b>Secret</b>.
              Khách trả bằng tiền tệ đang xem (USD, EUR, GBP, AUD, CAD, PLN…); tiền tệ PayPal không nhận sẽ quy sang USD.
            </p>
            <label class="block space-y-1">
              <span class="text-xs font-bold text-slate-600">Client ID</span>
              <input v-model="pay.paypal_client_id" autocomplete="off" class="w-full rounded-lg border border-slate-200 px-3 py-2 font-mono text-xs" />
            </label>
            <label class="block space-y-1">
              <span class="text-xs font-bold text-slate-600">Secret</span>
              <input v-model="pay.paypal_secret" type="password" autocomplete="new-password" :placeholder="intl.paypal_secret_set ? '•••••• (đã lưu, để trống = giữ nguyên)' : 'Dán Secret'" class="w-full rounded-lg border border-slate-200 px-3 py-2 font-mono text-xs" />
            </label>
            <div class="flex flex-wrap items-center gap-4">
              <label class="flex items-center gap-2 text-sm font-semibold"><input v-model="pay.paypal_sandbox" type="checkbox" /> Chế độ thử nghiệm (Sandbox)</label>
              <label class="flex items-center gap-2 text-sm font-semibold"><input v-model="pay.paypal_enabled" type="checkbox" /> Bật PayPal cho khách</label>
            </div>
            <p class="text-[11px] text-slate-400">Khi lưu, hệ thống thử đăng nhập PayPal bằng khoá vừa nhập để báo ngay nếu sai.</p>
          </section>

          <!-- ===== Thanh toán quốc tế: Stripe ===== -->
          <section class="space-y-3 rounded-2xl bg-white p-5 ring-1 ring-slate-200">
            <h2 class="font-extrabold">Stripe · thẻ quốc tế (Visa, Mastercard, Amex)</h2>
            <p class="text-xs text-slate-500">
              Stripe chưa mở cho doanh nghiệp đăng ký tại Việt Nam — cần công ty ở nước Stripe hỗ trợ (Singapore, Mỹ, EU…).
              Lấy <b>Secret key</b> (sk_live_… / sk_test_…) ở dashboard.stripe.com → Developers → API keys.
            </p>
            <label class="block space-y-1">
              <span class="text-xs font-bold text-slate-600">Secret key</span>
              <input v-model="pay.stripe_secret" type="password" autocomplete="new-password" :placeholder="intl.stripe_secret_set ? '•••••• (đã lưu, để trống = giữ nguyên)' : 'sk_live_... hoặc sk_test_...'" class="w-full rounded-lg border border-slate-200 px-3 py-2 font-mono text-xs" />
            </label>
            <label class="flex items-center gap-2 text-sm font-semibold"><input v-model="pay.stripe_enabled" type="checkbox" /> Bật thanh toán thẻ quốc tế (Stripe)</label>
            <div class="space-y-1.5 rounded-xl bg-sky-50 p-3 text-xs text-sky-900">
              <p class="font-bold">Nên làm: thêm Webhook trong Stripe (Developers → Webhooks → Add endpoint)</p>
              <div class="flex items-center gap-2">
                <code class="min-w-0 flex-1 break-all rounded bg-white/70 px-2 py-1">{{ intl.stripe_webhook_url }}</code>
                <button type="button" class="rounded p-1 hover:bg-sky-100" title="Sao chép" @click="copy(intl.stripe_webhook_url)"><Copy class="h-3.5 w-3.5" /></button>
              </div>
              <p>Chọn sự kiện <code>checkout.session.completed</code>, rồi dán <b>Signing secret</b> (whsec_…) vào ô dưới. Có webhook thì đơn vẫn được ghi nhận dù khách đóng trình duyệt sau khi trả tiền.</p>
              <input v-model="pay.stripe_webhook_secret" type="password" autocomplete="new-password" :placeholder="intl.stripe_webhook_set ? '•••••• (đã lưu)' : 'whsec_...'" class="w-full rounded-lg border border-sky-200 bg-white px-3 py-2 font-mono text-xs" />
            </div>
          </section>

          <div class="lg:col-span-2">
            <button type="submit" :disabled="savingPay" class="rounded-xl bg-emerald-600 px-5 py-2.5 text-sm font-bold text-white hover:bg-emerald-700 disabled:opacity-50">
              <Loader2 v-if="savingPay" class="mr-1 inline h-4 w-4 animate-spin" /> Lưu cài đặt
            </button>
          </div>
        </form>
      </template>

      <!-- ===================== EMAIL ĐĂNG KÝ ===================== -->
      <template v-else>
        <div class="rounded-2xl bg-white ring-1 ring-slate-200">
          <div class="flex items-center justify-between border-b border-slate-100 px-5 py-3">
            <span class="text-sm font-bold">{{ subscribers.length }} email</span>
            <button v-if="subscribers.length" class="inline-flex items-center gap-1 rounded-lg px-2 py-1 text-xs font-bold text-emerald-700 hover:bg-emerald-50" @click="copy(subscribers.map((s) => s.email).join('\n'))">
              <Copy class="h-3.5 w-3.5" /> Sao chép tất cả
            </button>
          </div>
          <p v-if="!subscribers.length" class="py-14 text-center text-sm text-slate-500">Chưa có ai đăng ký. Thêm section "Đăng ký email" vào trang để thu thập.</p>
          <ul v-else class="divide-y divide-slate-100">
            <li v-for="s in subscribers" :key="s.id" class="flex items-center justify-between px-5 py-2.5 text-sm">
              <span class="flex items-center gap-2"><Mail class="h-4 w-4 text-slate-400" /> {{ s.email }}</span>
              <span class="text-xs text-slate-400">{{ fmtTime(s.created_at) }}</span>
            </li>
          </ul>
        </div>
      </template>
    </div>

    <!-- Chi tiết đơn -->
    <div v-if="detail" class="fixed inset-0 z-50 flex justify-end bg-black/40" @click.self="detail = null">
      <aside class="flex h-full w-full max-w-lg flex-col overflow-y-auto bg-white">
        <div class="sticky top-0 flex items-center justify-between border-b border-slate-100 bg-white px-5 py-4">
          <div>
            <div class="text-lg font-extrabold">{{ detail.code }}</div>
            <div class="text-xs text-slate-500">{{ fmtTime(detail.created_at) }}</div>
          </div>
          <button class="rounded-lg p-1.5 hover:bg-slate-100" @click="detail = null"><X class="h-5 w-5" /></button>
        </div>
        <div class="space-y-5 p-5">
          <section class="space-y-1 text-sm">
            <div class="font-bold">{{ detail.customer_name }}</div>
            <a :href="`tel:${detail.phone}`" class="inline-flex items-center gap-1 font-semibold text-emerald-700"><Phone class="h-3.5 w-3.5" /> {{ detail.phone }}</a>
            <div v-if="detail.email" class="text-slate-600">{{ detail.email }}</div>
            <div class="text-slate-700">{{ detail.address }}</div>
            <div v-if="detail.note" class="rounded-lg bg-amber-50 px-3 py-2 text-amber-900">Ghi chú: {{ detail.note }}</div>
          </section>
          <section class="space-y-2 rounded-xl bg-slate-50 p-4 text-sm">
            <div v-for="it in detail.items" :key="`${it.product_id}:${it.variant_id || 0}`" class="flex justify-between gap-3" data-testid="admin-order-line">
              <span>{{ it.name }}<span v-if="it.variant_title" class="font-semibold text-slate-600"> · {{ it.variant_title }}</span><span v-if="it.sku" class="text-xs text-slate-400"> ({{ it.sku }})</span> <span class="text-slate-400">× {{ it.qty }}</span></span>
              <span class="font-semibold" :title="adminMoneyTitle(it.line_total)">{{ adminMoney(it.line_total) }}</span>
            </div>
            <div class="flex justify-between border-t border-slate-200 pt-2 text-slate-500"><span>Phí ship</span><span :title="adminMoneyTitle(detail.shipping_fee)">{{ adminMoney(detail.shipping_fee) }}</span></div>
            <div class="flex justify-between text-base font-extrabold"><span>Tổng</span><span :title="adminMoneyTitle(detail.total)">{{ adminMoney(detail.total) }}</span></div>
            <div class="flex justify-between text-xs">
              <span>{{ payLabel(detail.payment_method) }}{{ detail.vnp_transaction_no ? ' · GD ' + detail.vnp_transaction_no : '' }}{{ detail.gateway_txn ? ' · GD ' + detail.gateway_txn : '' }}{{ fmtForeign(detail) }}</span>
              <span :class="detail.payment_status === 'paid' ? 'font-bold text-emerald-600' : 'text-slate-500'">{{ payLabels[detail.payment_status] }}</span>
            </div>
          </section>
          <section class="space-y-2">
            <span class="text-xs font-bold text-slate-600">Cập nhật trạng thái</span>
            <div class="grid grid-cols-2 gap-2 sm:grid-cols-3">
              <button
                v-for="s in ['new', 'confirmed', 'shipping', 'completed', 'cancelled']"
                :key="s"
                :disabled="updating || detail.status === s"
                class="rounded-xl px-3 py-2 text-xs font-bold ring-1 disabled:cursor-default"
                :class="detail.status === s ? statusClass[s] : 'bg-white text-slate-600 ring-slate-200 hover:bg-slate-50'"
                @click="updateOrder(orders.find((o) => o.id === detail!.id) || detail!, { status: s })"
              >
                {{ statusLabels[s] }}
              </button>
            </div>
            <button
              v-if="detail.payment_method === 'cod' && detail.payment_status !== 'paid'"
              :disabled="updating"
              class="w-full rounded-xl bg-emerald-600 px-3 py-2 text-xs font-bold text-white hover:bg-emerald-700"
              @click="updateOrder(orders.find((o) => o.id === detail!.id) || detail!, { payment_status: 'paid' })"
            >
              Đánh dấu đã thu tiền
            </button>
            <p class="text-[11px] text-slate-400">Huỷ đơn sẽ tự hoàn lại tồn kho.</p>
          </section>
        </div>
      </aside>
    </div>
  </div>
</template>
