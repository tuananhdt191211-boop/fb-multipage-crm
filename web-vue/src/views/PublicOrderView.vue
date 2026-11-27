<script setup lang="ts">
// Trang kết quả đặt hàng (cũng là trang VNPay chuyển khách về sau khi thanh toán)
import { currentSlug, storePath } from '../lib/storeHost'
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { CheckCircle2, Clock, Loader2, XCircle } from 'lucide-vue-next'
import PublicShell from '../components/store/PublicShell.vue'
import { api } from '../lib/api'
import { loadPublicSite, localizeSite } from '../lib/publicSite'
import { money, t } from '../lib/i18n'
import { useCart } from '../lib/cart'
import type { PublicSiteData, ShopOrder } from '../types/store'

const route = useRoute()
const slug = computed(() => currentSlug(route.params.slug))
const code = computed(() => String(route.params.code || ''))
const token = computed(() => String(route.query.t || ''))

const rawSite = ref<PublicSiteData | null>(null)
const site = computed(() => (rawSite.value ? localizeSite(rawSite.value) : null))
function nameOf(it: { product_id: number; name: string }) {
  return site.value?.products.find((p) => p.id === it.product_id)?.name || it.name
}
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
    rawSite.value = await loadPublicSite({ slug: slug.value })
    const vnp: Record<string, string> = {}
    for (const [k, v] of Object.entries(route.query)) if (k.startsWith('vnp_') && typeof v === 'string') vnp[k] = v
    if (Object.keys(vnp).length) {
      const { data } = await api.get('/storefront/public/vnpay-return', { params: vnp })
      vnpResult.value = data
    }
    await fetchOrder()
  } catch (e: any) {
    errorMsg.value = e?.response?.data?.error || t('order_not_found')
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
    errorMsg.value = e?.response?.data?.error || t('pay_link_failed')
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
  <div v-if="loading" class="flex min-h-screen items-center justify-center gap-2 text-sm text-slate-500"><Loader2 class="h-5 w-5 animate-spin" /> {{ t('loading_order') }}</div>
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
            {{ state === 'ok' ? t('order_success') : state === 'failed' ? t('payment_failed') : t('payment_pending') }}
          </h1>
          <p class="text-slate-600">
            {{ t('order_code') }} <b class="text-slate-900">{{ order.code }}</b>
          </p>
          <p class="text-sm text-slate-500">
            <template v-if="state === 'ok'">{{ t('will_call', { phone: order.phone }) }}</template>
            <template v-else-if="state === 'failed'">{{ t('order_kept') }}</template>
            <template v-else>{{ t('auto_update') }}</template>
          </p>
          <button
            v-if="state !== 'ok' && order.payment_method === 'vnpay'"
            :disabled="retrying"
            class="mt-2 rounded-full px-6 py-3 text-sm font-bold text-white disabled:opacity-60"
            :style="{ backgroundColor: accent }"
            @click="retryPay"
          >
            {{ retrying ? t('redirecting') : t('retry_vnpay') }}
          </button>
        </div>

        <div class="mt-10 space-y-3 rounded-3xl bg-slate-50 p-5 sm:p-6">
          <div v-for="it in order.items" :key="it.product_id" class="flex justify-between gap-3 text-sm">
            <span class="text-slate-700">{{ nameOf(it) }} <span class="text-slate-400">× {{ it.qty }}</span></span>
            <span class="font-semibold">{{ money(it.line_total) }}</span>
          </div>
          <div class="space-y-1.5 border-t border-slate-200 pt-3 text-sm">
            <div class="flex justify-between"><span class="text-slate-500">{{ t('shipping_fee') }}</span><span>{{ order.shipping_fee ? money(order.shipping_fee) : t('free') }}</span></div>
            <div class="flex justify-between text-base"><span class="font-bold">{{ t('total') }}</span><span class="font-extrabold" :style="{ color: accent }">{{ money(order.total) }}</span></div>
            <div class="flex justify-between"><span class="text-slate-500">{{ t('payment') }}</span><span>{{ order.payment_method === 'cod' ? t('pay_on_delivery') : order.payment_status === 'paid' ? t('paid_vnpay') : 'VNPay' }}</span></div>
          </div>
          <div class="border-t border-slate-200 pt-3 text-sm text-slate-600">
            <div class="font-semibold text-slate-800">{{ order.customer_name }} · {{ order.phone }}</div>
            <div>{{ order.address }}</div>
          </div>
        </div>

        <div class="mt-8 text-center">
          <router-link :to="storePath(site.slug)" class="inline-block rounded-full border-2 px-6 py-3 text-sm font-bold" :style="{ borderColor: accent, color: accent }">{{ t('continue_shopping') }}</router-link>
        </div>
      </template>
    </div>
  </PublicShell>
</template>
