<script setup lang="ts">
import { currentSlug, storePath } from '../lib/storeHost'
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { CreditCard, Globe, Loader2, Lock, Minus, Plus, Trash2, Truck, Wallet } from 'lucide-vue-next'
import PublicShell from '../components/store/PublicShell.vue'
import { api } from '../lib/api'
import { loadPublicProduct, loadPublicSite, localizeSite } from '../lib/publicSite'
import { formatVndPlain, isForeignCurrency, locale, money, t, variantLabel } from '../lib/i18n'
import { lineKey, useCart } from '../lib/cart'
import { customerHeaders, useCustomer } from '../lib/customer'
import { safeImg } from '../lib/safeUrl'
import type { PublicSiteData } from '../types/store'

const route = useRoute()
const router = useRouter()
const slug = computed(() => currentSlug(route.params.slug))
const cart = useCart(slug.value)

const rawSite = ref<PublicSiteData | null>(null)
const site = computed(() => (rawSite.value ? localizeSite(rawSite.value) : null))
// Tên sản phẩm theo ngôn ngữ đang chọn
function nameOf(l: { product_id: number; name: string }) {
  return site.value?.products.find((p) => p.id === l.product_id)?.name || l.name
}
const loading = ref(true)
const submitting = ref(false)
const errorMsg = ref('')
const cartNote = ref('') // giỏ vừa được làm mới: có dòng đã hết hàng / không còn bán
const config = ref({ cod: true, vnpay: false, paypal: false, stripe: false, shipping_fee: 0, free_ship_from: 0 })
type PayMethod = 'cod' | 'vnpay' | 'paypal' | 'stripe'
const intlMethod = computed(() => form.value.payment_method === 'paypal' || form.value.payment_method === 'stripe')

const form = ref({ customer_name: '', phone: '', email: '', address: '', note: '', payment_method: 'cod' as PayMethod })
const accent = computed(() => site.value?.primary_color || '#059669')

const shipping = computed(() => {
  const c = config.value
  if (c.shipping_fee <= 0) return 0
  if (c.free_ship_from > 0 && cart.subtotal.value >= c.free_ship_from) return 0
  return c.shipping_fee
})
const total = computed(() => cart.subtotal.value + shipping.value)

// Ghi nhớ thông tin giao hàng cho lần sau (chỉ trên máy của khách)
const infoKey = 'pp_checkout_info'
function restoreInfo() {
  try {
    const saved = JSON.parse(localStorage.getItem(infoKey) || '{}')
    for (const k of ['customer_name', 'phone', 'email', 'address'] as const) if (typeof saved[k] === 'string') form.value[k] = saved[k]
  } catch {
    /* bỏ qua */
  }
}
function rememberInfo() {
  try {
    const { customer_name, phone, email, address } = form.value
    localStorage.setItem(infoKey, JSON.stringify({ customer_name, phone, email, address }))
  } catch {
    /* bỏ qua */
  }
}

async function load() {
  loading.value = true
  try {
    rawSite.value = await loadPublicSite({ slug: slug.value })
    let removed = cart.syncWith(rawSite.value.products)
    // Dòng có biến thể: lấy giá / tồn kho mới nhất từ chi tiết sản phẩm (bỏ dòng đã hết hàng hoặc bị gỡ)
    removed += await cart.syncVariants(async (pid) => {
      try {
        return await loadPublicProduct(slug.value, pid)
      } catch (e: any) {
        if (e?.response?.status === 404) return null
        throw e
      }
    })
    if (removed > 0) cartNote.value = t('cart_items_removed')
    document.title = `${t('checkout')} – ${rawSite.value.site_title}`
    const { data } = await api.get('/storefront/public/checkout-config', { params: { slug: slug.value } })
    config.value = data
    // Khách đang xem bằng ngoại tệ -> ưu tiên thẻ quốc tế / PayPal
    if (locale.currency !== 'VND' && (data.stripe || data.paypal)) form.value.payment_method = data.stripe ? 'stripe' : 'paypal'
    else form.value.payment_method = data.cod ? 'cod' : data.vnpay ? 'vnpay' : data.stripe ? 'stripe' : 'paypal'
    restoreInfo()
    // Đã đăng nhập: điền thông tin tài khoản vào ô còn trống
    const c = useCustomer(slug.value).state.customer
    if (c) {
      if (!form.value.customer_name) form.value.customer_name = c.name
      if (!form.value.phone) form.value.phone = c.phone
      if (!form.value.email) form.value.email = c.email
      if (!form.value.address) form.value.address = c.address
    }
  } catch (e: any) {
    errorMsg.value = e?.response?.data?.error || t('checkout_failed_load')
  } finally {
    loading.value = false
  }
}

async function submit() {
  errorMsg.value = ''
  const f = form.value
  if (!f.customer_name.trim() || !f.phone.trim() || !f.address.trim()) {
    errorMsg.value = t('fill_required')
    return
  }
  if (!cart.state.lines.length) {
    errorMsg.value = t('cart_is_empty')
    return
  }
  submitting.value = true
  try {
    const { data } = await api.post('/storefront/public/orders', {
      slug: slug.value,
      items: cart.state.lines.map((l) => ({ product_id: l.product_id, variant_id: l.variant_id || 0, qty: l.qty })),
      ...f,
      return_base: window.location.origin,
      currency: locale.currency
    }, { headers: customerHeaders(slug.value) })
    rememberInfo()
    if (data.payment_url) {
      // Giữ giỏ hàng tới khi VNPay báo thành công (khách có thể quay lại)
      window.location.href = data.payment_url
      return
    }
    cart.clear()
    router.replace({ name: 'public-order', params: { slug: slug.value, code: data.code }, query: { t: data.token } })
  } catch (e: any) {
    errorMsg.value = e?.response?.data?.error || t('order_failed')
  } finally {
    submitting.value = false
  }
}

onMounted(load)
</script>

<template>
  <div v-if="loading" class="flex min-h-screen items-center justify-center gap-2 text-sm text-slate-500"><Loader2 class="h-5 w-5 animate-spin" /> {{ t('loading') }}</div>
  <div v-else-if="!site" class="py-24 text-center text-sm font-semibold text-rose-600">{{ errorMsg }}</div>
  <PublicShell v-else :site="site">
    <div class="mx-auto max-w-6xl px-5 py-8 sm:px-8 sm:py-12">
      <h1 class="mb-8 text-2xl font-extrabold tracking-tight text-slate-900 sm:text-3xl">{{ t('checkout') }}</h1>
      <p v-if="cartNote" class="mb-6 rounded-xl bg-amber-50 px-4 py-3 text-sm font-semibold text-amber-800" data-testid="cart-note">{{ cartNote }}</p>

      <div v-if="!cart.state.lines.length" class="rounded-3xl bg-slate-50 py-16 text-center">
        <p class="font-semibold text-slate-700">{{ t('cart_empty_long') }}</p>
        <router-link :to="storePath(site.slug)" class="mt-4 inline-block rounded-full px-6 py-3 text-sm font-bold text-white" :style="{ backgroundColor: accent }">{{ t('continue_shopping') }}</router-link>
      </div>

      <form v-else class="grid grid-cols-1 gap-8 lg:grid-cols-[1fr_420px]" @submit.prevent="submit">
        <!-- Thông tin giao hàng -->
        <div class="space-y-6">
          <section class="space-y-4 rounded-3xl border border-slate-200 p-5 sm:p-6">
            <h2 class="text-lg font-bold text-slate-900">{{ t('shipping_info') }}</h2>
            <div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
              <label class="space-y-1">
                <span class="text-sm font-semibold text-slate-700">{{ t('full_name_req') }}</span>
                <input v-model="form.customer_name" autocomplete="name" required class="w-full rounded-xl border border-slate-300 px-4 py-3 text-[15px] outline-none focus:border-slate-500" />
              </label>
              <label class="space-y-1">
                <span class="text-sm font-semibold text-slate-700">{{ t('phone_req') }}</span>
                <input v-model="form.phone" type="tel" autocomplete="tel" inputmode="tel" required class="w-full rounded-xl border border-slate-300 px-4 py-3 text-[15px] outline-none focus:border-slate-500" />
              </label>
            </div>
            <label class="block space-y-1">
              <span class="text-sm font-semibold text-slate-700">{{ t('address_req') }}</span>
              <textarea v-model="form.address" rows="2" autocomplete="street-address" required :placeholder="t('address_ph')" class="w-full rounded-xl border border-slate-300 px-4 py-3 text-[15px] outline-none focus:border-slate-500" />
            </label>
            <label class="block space-y-1">
              <span class="text-sm font-semibold text-slate-700">{{ t('email_optional') }}</span>
              <input v-model="form.email" type="email" autocomplete="email" class="w-full rounded-xl border border-slate-300 px-4 py-3 text-[15px] outline-none focus:border-slate-500" />
            </label>
            <label class="block space-y-1">
              <span class="text-sm font-semibold text-slate-700">{{ t('note') }}</span>
              <textarea v-model="form.note" rows="2" :placeholder="t('note_ph')" class="w-full rounded-xl border border-slate-300 px-4 py-3 text-[15px] outline-none focus:border-slate-500" />
            </label>
          </section>

          <section class="space-y-3 rounded-3xl border border-slate-200 p-5 sm:p-6">
            <h2 class="text-lg font-bold text-slate-900">{{ t('payment_method') }}</h2>
            <label v-if="config.cod" class="flex cursor-pointer items-center gap-3 rounded-2xl border-2 p-4 transition" :style="form.payment_method === 'cod' ? { borderColor: accent } : undefined" :class="form.payment_method === 'cod' ? '' : 'border-slate-200'">
              <input v-model="form.payment_method" type="radio" value="cod" class="h-4 w-4" />
              <Truck class="h-5 w-5 text-slate-500" />
              <span><span class="block font-semibold text-slate-900">{{ t('cod') }}</span><span class="block text-xs text-slate-500">{{ t('cod_desc') }}</span></span>
            </label>
            <label v-if="config.vnpay" class="flex cursor-pointer items-center gap-3 rounded-2xl border-2 p-4 transition" :style="form.payment_method === 'vnpay' ? { borderColor: accent } : undefined" :class="form.payment_method === 'vnpay' ? '' : 'border-slate-200'">
              <input v-model="form.payment_method" type="radio" value="vnpay" class="h-4 w-4" />
              <CreditCard class="h-5 w-5 text-slate-500" />
              <span><span class="block font-semibold text-slate-900">{{ t('vnpay') }}</span><span class="block text-xs text-slate-500">{{ t('vnpay_desc') }}</span></span>
            </label>
            <label v-if="config.stripe" class="flex cursor-pointer items-center gap-3 rounded-2xl border-2 p-4 transition" :style="form.payment_method === 'stripe' ? { borderColor: accent } : undefined" :class="form.payment_method === 'stripe' ? '' : 'border-slate-200'">
              <input v-model="form.payment_method" type="radio" value="stripe" class="h-4 w-4" />
              <Globe class="h-5 w-5 text-slate-500" />
              <span><span class="block font-semibold text-slate-900">{{ t('card_intl') }}</span><span class="block text-xs text-slate-500">{{ t('card_intl_desc') }}</span></span>
            </label>
            <label v-if="config.paypal" class="flex cursor-pointer items-center gap-3 rounded-2xl border-2 p-4 transition" :style="form.payment_method === 'paypal' ? { borderColor: accent } : undefined" :class="form.payment_method === 'paypal' ? '' : 'border-slate-200'">
              <input v-model="form.payment_method" type="radio" value="paypal" class="h-4 w-4" />
              <Wallet class="h-5 w-5 text-slate-500" />
              <span><span class="block font-semibold text-slate-900">PayPal</span><span class="block text-xs text-slate-500">{{ t('paypal_desc') }}</span></span>
            </label>
          </section>
        </div>

        <!-- Tóm tắt đơn hàng -->
        <aside class="h-fit space-y-4 rounded-3xl bg-slate-50 p-5 sm:p-6 lg:sticky lg:top-24">
          <h2 class="text-lg font-bold text-slate-900">{{ t('order_summary', { n: cart.count.value }) }}</h2>
          <ul class="divide-y divide-slate-200">
            <li v-for="l in cart.state.lines" :key="lineKey(l)" class="flex gap-3 py-3" data-testid="checkout-line">
              <div class="h-16 w-16 shrink-0 overflow-hidden rounded-xl bg-white">
                <img v-if="safeImg(l.image_url)" :src="safeImg(l.image_url)" :alt="nameOf(l)" class="h-full w-full object-cover" />
              </div>
              <div class="min-w-0 flex-1">
                <div class="line-clamp-2 text-sm font-semibold text-slate-800">{{ nameOf(l) }}</div>
                <div v-if="l.variant_title" class="text-xs text-slate-500">{{ variantLabel(l.product_id, l.variant_title) }}</div>
                <div class="mt-1.5 flex items-center justify-between">
                  <div class="flex items-center rounded-full border border-slate-300 bg-white">
                    <button type="button" class="p-1.5" :aria-label="t('decrease')" @click="cart.setQty(l, l.qty - 1)"><Minus class="h-3 w-3" /></button>
                    <span class="w-6 text-center text-xs font-semibold">{{ l.qty }}</span>
                    <button type="button" class="p-1.5" :aria-label="t('increase')" @click="cart.setQty(l, l.qty + 1)"><Plus class="h-3 w-3" /></button>
                  </div>
                  <span class="text-sm font-bold">{{ money(l.price * l.qty) }}</span>
                </div>
              </div>
              <button type="button" class="self-start p-1 text-slate-400 hover:text-rose-600" :aria-label="t('remove')" @click="cart.remove(l)"><Trash2 class="h-4 w-4" /></button>
            </li>
          </ul>
          <div class="space-y-2 border-t border-slate-200 pt-4 text-sm">
            <div class="flex justify-between"><span class="text-slate-600">{{ t('subtotal') }}</span><span class="font-semibold">{{ money(cart.subtotal.value) }}</span></div>
            <div class="flex justify-between">
              <span class="text-slate-600">{{ t('shipping_fee') }}</span>
              <span class="font-semibold">{{ shipping ? money(shipping) : t('free') }}</span>
            </div>
            <p v-if="config.free_ship_from > 0 && shipping > 0" class="text-xs text-slate-500">{{ t('free_ship_more', { amount: money(config.free_ship_from - cart.subtotal.value) }) }}</p>
            <div class="flex items-baseline justify-between border-t border-slate-200 pt-3">
              <span class="font-bold text-slate-900">{{ t('total') }}</span>
              <span class="text-2xl font-extrabold" :style="{ color: accent }">{{ money(total) }}</span>
            </div>
          </div>
          <!-- Đang xem bằng ngoại tệ: ghi rõ số tiền VND thực thu -->
          <p v-if="isForeignCurrency() && intlMethod" class="rounded-xl bg-emerald-50 px-3 py-2 text-xs leading-relaxed text-emerald-800">{{ t('charged_in_currency', { amount: money(total) }) }}</p>
          <p v-else-if="isForeignCurrency()" class="rounded-xl bg-amber-50 px-3 py-2 text-xs leading-relaxed text-amber-800">{{ t('charged_in_vnd', { amount: formatVndPlain(total) }) }}</p>
          <p v-if="errorMsg" class="rounded-xl bg-rose-50 px-3 py-2 text-sm font-semibold text-rose-700">{{ errorMsg }}</p>
          <button type="submit" :disabled="submitting" class="flex w-full items-center justify-center gap-2 rounded-full py-4 text-base font-bold text-white shadow-lg transition hover:brightness-110 disabled:opacity-60" :style="{ backgroundColor: accent }">
            <Loader2 v-if="submitting" class="h-5 w-5 animate-spin" />
            <Lock v-else class="h-4 w-4" />
            {{ form.payment_method === 'vnpay' ? t('pay_vnpay') : form.payment_method === 'paypal' ? t('pay_paypal') : form.payment_method === 'stripe' ? t('pay_card') : t('place_order') }}
          </button>
          <p class="text-center text-xs text-slate-400">{{ t('privacy_note') }}</p>
        </aside>
      </form>
    </div>
  </PublicShell>
</template>
