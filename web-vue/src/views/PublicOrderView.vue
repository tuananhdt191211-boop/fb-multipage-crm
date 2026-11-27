<script setup lang="ts">
// Trang kết quả đặt hàng (cũng là trang VNPay chuyển khách về sau khi thanh toán)
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { CheckCircle2, Clock, Loader2, XCircle } from 'lucide-vue-next'
import PublicShell from '../components/store/PublicShell.vue'
import { api } from '../lib/api'
import { loadPublicSite } from '../lib/publicSite'
import { useCart } from '../lib/cart'
import { formatVnd } from '../lib/format'
import type { PublicSiteData, ShopOrder } from '../types/store'

const route = useRoute()
const slug = computed(() => String(route.params.slug || ''))
const code = computed(() => String(route.params.code || ''))
const token = computed(() => String(route.query.t || ''))

const site = ref<PublicSiteData | null>(null)
const order = ref<ShopOrder | null>(null)
const loading = ref(true)
const errorMsg = ref('')
const vnpResult = ref<{ valid: boolean; success: boolean } | null>(null)
const retrying = ref(false)
const accent = computed(() => site.value?.primary_color || '#059669')

let pollTimer: ReturnType<typeof setTimeout> | null = null
let polls = 0

async function fetchOrder() {
  const { data } = await api.get<{ order: ShopOrder }>(`/storefront/public/orders/${code.value}`, { params: { t: token.value } })
  order.value = data.order
  if (order.value.payment_status === 'paid' || order.value.payment_method === 'cod') useCart(slug.value).clear()
  // VNPay báo kết quả cho máy chủ (IPN) có thể chậm vài giây -> hỏi lại
  if (order.value.payment_method === 'vnpay' && order.value.payment_status === 'pending' && vnpResult.value?.success && polls < 10) {
    polls++
    pollTimer = setTimeout(fetchOrder, 3000)
  }
}

async function load() {
  loading.value = true
  try {
    site.value = await loadPublicSite({ slug: slug.value })
    const vnp: Record<string, string> = {}
    for (const [k, v] of Object.entries(route.query)) if (k.startsWith('vnp_') && typeof v === 'string') vnp[k] = v
    if (Object.keys(vnp).length) {
      const { data } = await api.get('/storefront/public/vnpay-return', { params: vnp })
      vnpResult.value = data
    }
    await fetchOrder()
  } catch (e: any) {
    errorMsg.value = e?.response?.data?.error || 'Không tìm thấy đơn hàng.'
  } finally {
    loading.value = false
  }
}

async function retryPay() {
  retrying.value = true
  try {
    const { data } = await api.post(`/storefront/public/orders/${code.value}/pay`, null, { params: { t: token.value, return_base: window.location.origin } })
    window.location.href = data.payment_url
  } catch (e: any) {
    errorMsg.value = e?.response?.data?.error || 'Không tạo được liên kết thanh toán.'
    retrying.value = false
  }
}

const state = computed<'ok' | 'pending' | 'failed'>(() => {
  const o = order.value
  if (!o) return 'pending'
  if (o.payment_method === 'cod' || o.payment_status === 'paid') return 'ok'
  if (o.payment_status === 'failed' || (vnpResult.value && !vnpResult.value.success)) return 'failed'
  return 'pending'
})

onMounted(load)
onBeforeUnmount(() => pollTimer && clearTimeout(pollTimer))
</script>

<template>
  <div v-if="loading" class="flex min-h-screen items-center justify-center gap-2 text-sm text-slate-500"><Loader2 class="h-5 w-5 animate-spin" /> Đang tải đơn hàng...</div>
  <div v-else-if="!site" class="py-24 text-center text-sm font-semibold text-rose-600">{{ errorMsg }}</div>
  <PublicShell v-else :site="site">
    <div class="mx-auto max-w-2xl px-5 py-10 sm:py-16">
      <p v-if="!order" class="text-center font-semibold text-rose-600">{{ errorMsg }}</p>
      <template v-else>
        <div class="space-y-3 text-center">
          <CheckCircle2 v-if="state === 'ok'" class="mx-auto h-16 w-16" :style="{ color: accent }" />
          <XCircle v-else-if="state === 'failed'" class="mx-auto h-16 w-16 text-rose-500" />
          <Clock v-else class="mx-auto h-16 w-16 text-amber-500" />
          <h1 class="text-2xl font-extrabold text-slate-900 sm:text-3xl">
            {{ state === 'ok' ? 'Đặt hàng thành công!' : state === 'failed' ? 'Thanh toán chưa thành công' : 'Đang chờ xác nhận thanh toán' }}
          </h1>
          <p class="text-slate-600">
            Mã đơn hàng: <b class="text-slate-900">{{ order.code }}</b>
          </p>
          <p class="text-sm text-slate-500">
            <template v-if="state === 'ok'">Cửa hàng sẽ gọi cho bạn qua số {{ order.phone }} để xác nhận và giao hàng.</template>
            <template v-else-if="state === 'failed'">Đơn hàng vẫn được giữ. Bạn có thể thử thanh toán lại.</template>
            <template v-else>Nếu bạn đã thanh toán, trang sẽ tự cập nhật trong giây lát.</template>
          </p>
          <button
            v-if="state !== 'ok' && order.payment_method === 'vnpay'"
            :disabled="retrying"
            class="mt-2 rounded-full px-6 py-3 text-sm font-bold text-white disabled:opacity-60"
            :style="{ backgroundColor: accent }"
            @click="retryPay"
          >
            {{ retrying ? 'Đang chuyển...' : 'Thanh toán lại qua VNPay' }}
          </button>
        </div>

        <div class="mt-10 space-y-3 rounded-3xl bg-slate-50 p-5 sm:p-6">
          <div v-for="it in order.items" :key="it.product_id" class="flex justify-between gap-3 text-sm">
            <span class="text-slate-700">{{ it.name }} <span class="text-slate-400">× {{ it.qty }}</span></span>
            <span class="font-semibold">{{ formatVnd(it.line_total) }}</span>
          </div>
          <div class="space-y-1.5 border-t border-slate-200 pt-3 text-sm">
            <div class="flex justify-between"><span class="text-slate-500">Phí vận chuyển</span><span>{{ order.shipping_fee ? formatVnd(order.shipping_fee) : 'Miễn phí' }}</span></div>
            <div class="flex justify-between text-base"><span class="font-bold">Tổng cộng</span><span class="font-extrabold" :style="{ color: accent }">{{ formatVnd(order.total) }}</span></div>
            <div class="flex justify-between"><span class="text-slate-500">Thanh toán</span><span>{{ order.payment_method === 'cod' ? 'Khi nhận hàng (COD)' : order.payment_status === 'paid' ? 'Đã thanh toán qua VNPay' : 'VNPay' }}</span></div>
          </div>
          <div class="border-t border-slate-200 pt-3 text-sm text-slate-600">
            <div class="font-semibold text-slate-800">{{ order.customer_name }} · {{ order.phone }}</div>
            <div>{{ order.address }}</div>
          </div>
        </div>

        <div class="mt-8 text-center">
          <router-link :to="`/s/${site.slug}`" class="inline-block rounded-full border-2 px-6 py-3 text-sm font-bold" :style="{ borderColor: accent, color: accent }">Tiếp tục mua sắm</router-link>
        </div>
      </template>
    </div>
  </PublicShell>
</template>
