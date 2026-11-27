<script setup lang="ts">
// Trang tài khoản của khách: đơn hàng của tôi + thông tin giao hàng
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { ChevronRight, Loader2, LogOut, Package, User } from 'lucide-vue-next'
import PublicShell from '../components/store/PublicShell.vue'
import { loadPublicSite } from '../lib/publicSite'
import { apiError, customerState, useCustomer, type CustomerOrder } from '../lib/customer'
import { currentSlug, storePath } from '../lib/storeHost'
import { formatVnd } from '../lib/format'
import type { PublicSiteData } from '../types/store'

const route = useRoute()
const slug = computed(() => currentSlug(route.params.slug))
const cust = useCustomer(slug.value)

const site = ref<PublicSiteData | null>(null)
const loading = ref(true)
const errorMsg = ref('')
const orders = ref<CustomerOrder[]>([])
const ordersLoading = ref(false)
const tab = ref<'orders' | 'profile'>('orders')
const accent = computed(() => site.value?.primary_color || '#059669')

const profile = ref({ name: '', email: '', address: '' })
const saving = ref(false)
const saveMsg = ref('')
const saveErr = ref('')

const statusText: Record<string, { t: string; cls: string }> = {
  new: { t: 'Chờ xác nhận', cls: 'bg-amber-100 text-amber-700' },
  confirmed: { t: 'Đã xác nhận', cls: 'bg-violet-100 text-violet-700' },
  shipping: { t: 'Đang giao', cls: 'bg-sky-100 text-sky-700' },
  completed: { t: 'Hoàn tất', cls: 'bg-emerald-100 text-emerald-700' },
  cancelled: { t: 'Đã huỷ', cls: 'bg-slate-100 text-slate-500' }
}

function fillProfile() {
  const c = customerState.customer
  profile.value = { name: c?.name || '', email: c?.email || '', address: c?.address || '' }
}

async function loadOrders() {
  if (!customerState.customer) return
  ordersLoading.value = true
  try {
    orders.value = await cust.orders()
  } catch (e) {
    errorMsg.value = apiError(e, 'Không tải được đơn hàng.')
  } finally {
    ordersLoading.value = false
  }
}

async function saveProfile() {
  saving.value = true
  saveMsg.value = ''
  saveErr.value = ''
  try {
    await cust.update({ name: profile.value.name.trim(), email: profile.value.email.trim(), address: profile.value.address.trim() })
    saveMsg.value = 'Đã lưu thông tin.'
  } catch (e) {
    saveErr.value = apiError(e, 'Không lưu được thông tin.')
  } finally {
    saving.value = false
  }
}

function orderLink(o: CustomerOrder) {
  return { path: storePath(slug.value, `/order/${o.code}`), query: { t: o.token } }
}

function fmtDate(s: string) {
  const d = new Date(s)
  return isNaN(d.getTime()) ? '' : d.toLocaleString('vi-VN', { day: '2-digit', month: '2-digit', year: 'numeric', hour: '2-digit', minute: '2-digit' })
}

// Đăng nhập / đăng xuất ngay trên trang này -> tải lại dữ liệu
watch(
  () => customerState.customer?.id,
  () => {
    fillProfile()
    orders.value = []
    loadOrders()
  }
)

onMounted(async () => {
  try {
    site.value = await loadPublicSite({ slug: slug.value })
    document.title = `Tài khoản – ${site.value.site_title}`
    await cust.refresh()
    fillProfile()
    await loadOrders()
  } catch (e) {
    errorMsg.value = apiError(e, 'Không tìm thấy cửa hàng.')
  } finally {
    loading.value = false
  }
})
</script>

<template>
  <div v-if="loading" class="flex min-h-screen items-center justify-center gap-2 text-sm text-slate-500">
    <Loader2 class="h-5 w-5 animate-spin" /> Đang tải...
  </div>
  <div v-else-if="!site" class="flex min-h-screen items-center justify-center p-6 text-center text-sm text-slate-500">{{ errorMsg }}</div>
  <PublicShell v-else :site="site">
    <div class="mx-auto w-full max-w-4xl px-5 py-10 text-slate-800">
      <!-- Chưa đăng nhập -->
      <div v-if="!customerState.customer" class="mx-auto max-w-md space-y-4 rounded-3xl border border-slate-200 p-8 text-center">
        <div class="mx-auto flex h-14 w-14 items-center justify-center rounded-full" :style="{ backgroundColor: `color-mix(in srgb, ${accent} 12%, white)`, color: accent }">
          <User class="h-7 w-7" />
        </div>
        <h1 class="text-2xl font-extrabold">Tài khoản của bạn</h1>
        <p class="text-sm text-slate-500">Đăng nhập để xem đơn hàng đã đặt và lưu thông tin giao hàng cho lần sau.</p>
        <div class="grid grid-cols-2 gap-2 pt-2">
          <button type="button" class="rounded-full border border-slate-300 py-3 text-sm font-bold hover:bg-slate-50" @click="customerState.dialog = 'login'">Đăng nhập</button>
          <button type="button" class="rounded-full py-3 text-sm font-bold text-white hover:opacity-90" :style="{ backgroundColor: accent }" @click="customerState.dialog = 'register'">Đăng ký</button>
        </div>
      </div>

      <!-- Đã đăng nhập -->
      <template v-else>
        <div class="flex flex-wrap items-center justify-between gap-4">
          <div class="flex items-center gap-4">
            <span class="flex h-14 w-14 items-center justify-center rounded-full text-xl font-extrabold text-white" :style="{ backgroundColor: accent }">
              {{ customerState.customer.name.trim().charAt(0).toUpperCase() }}
            </span>
            <div>
              <h1 class="text-2xl font-extrabold">Xin chào, {{ customerState.customer.name }}</h1>
              <p class="text-sm text-slate-500">{{ customerState.customer.phone }}<template v-if="customerState.customer.email"> · {{ customerState.customer.email }}</template></p>
            </div>
          </div>
          <button type="button" class="inline-flex items-center gap-2 rounded-full border border-slate-200 px-4 py-2 text-sm font-semibold text-rose-600 hover:bg-rose-50" @click="cust.logout()">
            <LogOut class="h-4 w-4" /> Đăng xuất
          </button>
        </div>

        <div class="mt-8 flex gap-1 border-b border-slate-200">
          <button
            v-for="t in [{ v: 'orders', l: 'Đơn hàng của tôi', i: Package }, { v: 'profile', l: 'Thông tin giao hàng', i: User }] as const"
            :key="t.v"
            type="button"
            class="-mb-px inline-flex items-center gap-2 border-b-2 px-4 py-3 text-sm font-bold transition"
            :class="tab === t.v ? '' : 'border-transparent text-slate-500 hover:text-slate-800'"
            :style="tab === t.v ? { borderColor: accent, color: accent } : undefined"
            @click="tab = t.v"
          >
            <component :is="t.i" class="h-4 w-4" /> {{ t.l }}
          </button>
        </div>

        <!-- Đơn hàng -->
        <div v-if="tab === 'orders'" class="mt-6">
          <div v-if="ordersLoading" class="flex items-center justify-center gap-2 py-12 text-sm text-slate-500"><Loader2 class="h-5 w-5 animate-spin" /> Đang tải đơn hàng...</div>
          <p v-else-if="errorMsg" class="rounded-xl bg-rose-50 px-4 py-3 text-sm text-rose-600">{{ errorMsg }}</p>
          <div v-else-if="!orders.length" class="rounded-3xl border border-dashed border-slate-300 py-14 text-center">
            <Package class="mx-auto h-8 w-8 text-slate-300" />
            <p class="mt-3 text-sm text-slate-500">Bạn chưa có đơn hàng nào.</p>
            <router-link :to="storePath(slug)" class="mt-4 inline-block rounded-full px-6 py-2.5 text-sm font-bold text-white" :style="{ backgroundColor: accent }">Mua sắm ngay</router-link>
          </div>
          <ul v-else class="divide-y divide-slate-100 overflow-hidden rounded-2xl border border-slate-200">
            <li v-for="o in orders" :key="o.code">
              <router-link :to="orderLink(o)" class="flex items-center gap-4 px-5 py-4 transition hover:bg-slate-50">
                <div class="min-w-0 flex-1">
                  <div class="flex flex-wrap items-center gap-2">
                    <span class="font-bold">#{{ o.code }}</span>
                    <span class="rounded-full px-2 py-0.5 text-[11px] font-bold" :class="statusText[o.status]?.cls || 'bg-slate-100 text-slate-600'">{{ statusText[o.status]?.t || o.status }}</span>
                    <span v-if="o.payment_status === 'paid'" class="rounded-full bg-emerald-50 px-2 py-0.5 text-[11px] font-bold text-emerald-700">Đã thanh toán</span>
                  </div>
                  <div class="mt-1 text-xs text-slate-500">{{ fmtDate(o.created_at) }} · {{ o.qty }} sản phẩm · {{ o.payment_method === 'vnpay' ? 'VNPay' : 'Thanh toán khi nhận hàng' }}</div>
                </div>
                <div class="text-right font-extrabold" :style="{ color: accent }">{{ formatVnd(o.total) }}</div>
                <ChevronRight class="h-4 w-4 shrink-0 text-slate-400" />
              </router-link>
            </li>
          </ul>
        </div>

        <!-- Thông tin giao hàng -->
        <form v-else class="mt-6 max-w-xl space-y-4" @submit.prevent="saveProfile">
          <label class="block space-y-1.5">
            <span class="text-sm font-semibold">Họ và tên</span>
            <input v-model="profile.name" class="w-full rounded-xl border border-slate-200 px-4 py-3 text-sm outline-none focus:border-slate-400" required />
          </label>
          <label class="block space-y-1.5">
            <span class="text-sm font-semibold">Số điện thoại</span>
            <input :value="customerState.customer.phone" disabled class="w-full rounded-xl border border-slate-200 bg-slate-50 px-4 py-3 text-sm text-slate-500" />
          </label>
          <label class="block space-y-1.5">
            <span class="text-sm font-semibold">Email</span>
            <input v-model="profile.email" type="email" class="w-full rounded-xl border border-slate-200 px-4 py-3 text-sm outline-none focus:border-slate-400" placeholder="Không bắt buộc" />
          </label>
          <label class="block space-y-1.5">
            <span class="text-sm font-semibold">Địa chỉ nhận hàng</span>
            <textarea v-model="profile.address" rows="3" class="w-full rounded-xl border border-slate-200 px-4 py-3 text-sm outline-none focus:border-slate-400" placeholder="Số nhà, đường, phường/xã, quận/huyện, tỉnh/thành" />
          </label>
          <p v-if="saveErr" class="rounded-xl bg-rose-50 px-4 py-2.5 text-sm text-rose-600">{{ saveErr }}</p>
          <p v-if="saveMsg" class="rounded-xl bg-emerald-50 px-4 py-2.5 text-sm text-emerald-700">{{ saveMsg }}</p>
          <button type="submit" class="inline-flex items-center gap-2 rounded-full px-7 py-3 text-sm font-bold text-white hover:opacity-90 disabled:opacity-60" :style="{ backgroundColor: accent }" :disabled="saving">
            <Loader2 v-if="saving" class="h-4 w-4 animate-spin" /> Lưu thông tin
          </button>
        </form>
      </template>
    </div>
  </PublicShell>
</template>
