<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { CreditCard, Loader2, Lock, Minus, Plus, Trash2, Truck } from 'lucide-vue-next'
import PublicShell from '../components/store/PublicShell.vue'
import { api } from '../lib/api'
import { loadPublicSite } from '../lib/publicSite'
import { useCart } from '../lib/cart'
import { formatVnd } from '../lib/format'
import { safeImg } from '../lib/safeUrl'
import type { PublicSiteData } from '../types/store'

const route = useRoute()
const router = useRouter()
const slug = computed(() => String(route.params.slug || ''))
const cart = useCart(slug.value)

const site = ref<PublicSiteData | null>(null)
const loading = ref(true)
const submitting = ref(false)
const errorMsg = ref('')
const config = ref({ cod: true, vnpay: false, shipping_fee: 0, free_ship_from: 0 })

const form = ref({ customer_name: '', phone: '', email: '', address: '', note: '', payment_method: 'cod' as 'cod' | 'vnpay' })
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
    site.value = await loadPublicSite({ slug: slug.value })
    cart.syncWith(site.value.products)
    document.title = `Thanh toán – ${site.value.site_title}`
    const { data } = await api.get('/storefront/public/checkout-config', { params: { slug: slug.value } })
    config.value = data
    form.value.payment_method = data.cod ? 'cod' : 'vnpay'
    restoreInfo()
  } catch (e: any) {
    errorMsg.value = e?.response?.data?.error || 'Không tải được trang thanh toán.'
  } finally {
    loading.value = false
  }
}

async function submit() {
  errorMsg.value = ''
  const f = form.value
  if (!f.customer_name.trim() || !f.phone.trim() || !f.address.trim()) {
    errorMsg.value = 'Vui lòng nhập đủ họ tên, số điện thoại và địa chỉ.'
    return
  }
  if (!cart.state.lines.length) {
    errorMsg.value = 'Giỏ hàng đang trống.'
    return
  }
  submitting.value = true
  try {
    const { data } = await api.post('/storefront/public/orders', {
      slug: slug.value,
      items: cart.state.lines.map((l) => ({ product_id: l.product_id, qty: l.qty })),
      ...f,
      return_base: window.location.origin
    })
    rememberInfo()
    if (data.payment_url) {
      // Giữ giỏ hàng tới khi VNPay báo thành công (khách có thể quay lại)
      window.location.href = data.payment_url
      return
    }
    cart.clear()
    router.replace({ name: 'public-order', params: { slug: slug.value, code: data.code }, query: { t: data.token } })
  } catch (e: any) {
    errorMsg.value = e?.response?.data?.error || 'Đặt hàng chưa thành công, vui lòng thử lại.'
  } finally {
    submitting.value = false
  }
}

onMounted(load)
</script>

<template>
  <div v-if="loading" class="flex min-h-screen items-center justify-center gap-2 text-sm text-slate-500"><Loader2 class="h-5 w-5 animate-spin" /> Đang tải...</div>
  <div v-else-if="!site" class="py-24 text-center text-sm font-semibold text-rose-600">{{ errorMsg }}</div>
  <PublicShell v-else :site="site">
    <div class="mx-auto max-w-6xl px-5 py-8 sm:px-8 sm:py-12">
      <h1 class="mb-8 text-2xl font-extrabold tracking-tight text-slate-900 sm:text-3xl">Thanh toán</h1>

      <div v-if="!cart.state.lines.length" class="rounded-3xl bg-slate-50 py-16 text-center">
        <p class="font-semibold text-slate-700">Giỏ hàng của bạn đang trống.</p>
        <router-link :to="`/s/${site.slug}`" class="mt-4 inline-block rounded-full px-6 py-3 text-sm font-bold text-white" :style="{ backgroundColor: accent }">Tiếp tục mua sắm</router-link>
      </div>

      <form v-else class="grid grid-cols-1 gap-8 lg:grid-cols-[1fr_420px]" @submit.prevent="submit">
        <!-- Thông tin giao hàng -->
        <div class="space-y-6">
          <section class="space-y-4 rounded-3xl border border-slate-200 p-5 sm:p-6">
            <h2 class="text-lg font-bold text-slate-900">Thông tin nhận hàng</h2>
            <div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
              <label class="space-y-1">
                <span class="text-sm font-semibold text-slate-700">Họ và tên *</span>
                <input v-model="form.customer_name" autocomplete="name" required class="w-full rounded-xl border border-slate-300 px-4 py-3 text-[15px] outline-none focus:border-slate-500" />
              </label>
              <label class="space-y-1">
                <span class="text-sm font-semibold text-slate-700">Số điện thoại *</span>
                <input v-model="form.phone" type="tel" autocomplete="tel" inputmode="tel" required class="w-full rounded-xl border border-slate-300 px-4 py-3 text-[15px] outline-none focus:border-slate-500" />
              </label>
            </div>
            <label class="block space-y-1">
              <span class="text-sm font-semibold text-slate-700">Địa chỉ nhận hàng *</span>
              <textarea v-model="form.address" rows="2" autocomplete="street-address" required placeholder="Số nhà, đường, phường/xã, quận/huyện, tỉnh/thành" class="w-full rounded-xl border border-slate-300 px-4 py-3 text-[15px] outline-none focus:border-slate-500" />
            </label>
            <label class="block space-y-1">
              <span class="text-sm font-semibold text-slate-700">Email (không bắt buộc)</span>
              <input v-model="form.email" type="email" autocomplete="email" class="w-full rounded-xl border border-slate-300 px-4 py-3 text-[15px] outline-none focus:border-slate-500" />
            </label>
            <label class="block space-y-1">
              <span class="text-sm font-semibold text-slate-700">Ghi chú</span>
              <textarea v-model="form.note" rows="2" placeholder="Thời gian nhận hàng, yêu cầu đặc biệt..." class="w-full rounded-xl border border-slate-300 px-4 py-3 text-[15px] outline-none focus:border-slate-500" />
            </label>
          </section>

          <section class="space-y-3 rounded-3xl border border-slate-200 p-5 sm:p-6">
            <h2 class="text-lg font-bold text-slate-900">Phương thức thanh toán</h2>
            <label v-if="config.cod" class="flex cursor-pointer items-center gap-3 rounded-2xl border-2 p-4 transition" :style="form.payment_method === 'cod' ? { borderColor: accent } : undefined" :class="form.payment_method === 'cod' ? '' : 'border-slate-200'">
              <input v-model="form.payment_method" type="radio" value="cod" class="h-4 w-4" />
              <Truck class="h-5 w-5 text-slate-500" />
              <span><span class="block font-semibold text-slate-900">Thanh toán khi nhận hàng (COD)</span><span class="block text-xs text-slate-500">Kiểm tra hàng rồi trả tiền cho shipper</span></span>
            </label>
            <label v-if="config.vnpay" class="flex cursor-pointer items-center gap-3 rounded-2xl border-2 p-4 transition" :style="form.payment_method === 'vnpay' ? { borderColor: accent } : undefined" :class="form.payment_method === 'vnpay' ? '' : 'border-slate-200'">
              <input v-model="form.payment_method" type="radio" value="vnpay" class="h-4 w-4" />
              <CreditCard class="h-5 w-5 text-slate-500" />
              <span><span class="block font-semibold text-slate-900">Thanh toán online qua VNPay</span><span class="block text-xs text-slate-500">Thẻ ATM, Visa/Master, QR ngân hàng</span></span>
            </label>
          </section>
        </div>

        <!-- Tóm tắt đơn hàng -->
        <aside class="h-fit space-y-4 rounded-3xl bg-slate-50 p-5 sm:p-6 lg:sticky lg:top-24">
          <h2 class="text-lg font-bold text-slate-900">Đơn hàng ({{ cart.count.value }} sản phẩm)</h2>
          <ul class="divide-y divide-slate-200">
            <li v-for="l in cart.state.lines" :key="l.product_id" class="flex gap-3 py-3">
              <div class="h-16 w-16 shrink-0 overflow-hidden rounded-xl bg-white">
                <img v-if="safeImg(l.image_url)" :src="safeImg(l.image_url)" :alt="l.name" class="h-full w-full object-cover" />
              </div>
              <div class="min-w-0 flex-1">
                <div class="line-clamp-2 text-sm font-semibold text-slate-800">{{ l.name }}</div>
                <div class="mt-1.5 flex items-center justify-between">
                  <div class="flex items-center rounded-full border border-slate-300 bg-white">
                    <button type="button" class="p-1.5" aria-label="Giảm" @click="cart.setQty(l.product_id, l.qty - 1)"><Minus class="h-3 w-3" /></button>
                    <span class="w-6 text-center text-xs font-semibold">{{ l.qty }}</span>
                    <button type="button" class="p-1.5" aria-label="Tăng" @click="cart.setQty(l.product_id, l.qty + 1)"><Plus class="h-3 w-3" /></button>
                  </div>
                  <span class="text-sm font-bold">{{ formatVnd(l.price * l.qty) }}</span>
                </div>
              </div>
              <button type="button" class="self-start p-1 text-slate-400 hover:text-rose-600" aria-label="Xoá" @click="cart.remove(l.product_id)"><Trash2 class="h-4 w-4" /></button>
            </li>
          </ul>
          <div class="space-y-2 border-t border-slate-200 pt-4 text-sm">
            <div class="flex justify-between"><span class="text-slate-600">Tạm tính</span><span class="font-semibold">{{ formatVnd(cart.subtotal.value) }}</span></div>
            <div class="flex justify-between">
              <span class="text-slate-600">Phí vận chuyển</span>
              <span class="font-semibold">{{ shipping ? formatVnd(shipping) : 'Miễn phí' }}</span>
            </div>
            <p v-if="config.free_ship_from > 0 && shipping > 0" class="text-xs text-slate-500">Mua thêm {{ formatVnd(config.free_ship_from - cart.subtotal.value) }} để được miễn phí vận chuyển</p>
            <div class="flex items-baseline justify-between border-t border-slate-200 pt-3">
              <span class="font-bold text-slate-900">Tổng cộng</span>
              <span class="text-2xl font-extrabold" :style="{ color: accent }">{{ formatVnd(total) }}</span>
            </div>
          </div>
          <p v-if="errorMsg" class="rounded-xl bg-rose-50 px-3 py-2 text-sm font-semibold text-rose-700">{{ errorMsg }}</p>
          <button type="submit" :disabled="submitting" class="flex w-full items-center justify-center gap-2 rounded-full py-4 text-base font-bold text-white shadow-lg transition hover:brightness-110 disabled:opacity-60" :style="{ backgroundColor: accent }">
            <Loader2 v-if="submitting" class="h-5 w-5 animate-spin" />
            <Lock v-else class="h-4 w-4" />
            {{ form.payment_method === 'vnpay' ? 'Thanh toán qua VNPay' : 'Đặt hàng' }}
          </button>
          <p class="text-center text-xs text-slate-400">Thông tin của bạn chỉ dùng để giao hàng.</p>
        </aside>
      </form>
    </div>
  </PublicShell>
</template>
