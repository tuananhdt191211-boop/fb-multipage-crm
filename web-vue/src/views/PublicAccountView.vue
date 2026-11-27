<script setup lang="ts">
// Trang tài khoản của khách: đơn hàng của tôi + thông tin giao hàng
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { ChevronRight, Loader2, LogOut, Package, User } from 'lucide-vue-next'
import PublicShell from '../components/store/PublicShell.vue'
import { loadPublicSite, localizeSite } from '../lib/publicSite'
import { apiError, customerState, useCustomer, type CustomerOrder } from '../lib/customer'
import { currentSlug, storePath } from '../lib/storeHost'
import { formatDate, money, t, type UiKey } from '../lib/i18n'
import type { PublicSiteData } from '../types/store'

const route = useRoute()
const slug = computed(() => currentSlug(route.params.slug))
const cust = useCustomer(slug.value)

const rawSite = ref<PublicSiteData | null>(null)
const site = computed(() => (rawSite.value ? localizeSite(rawSite.value) : null))
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

const statusText: Record<string, { k: UiKey; cls: string }> = {
  new: { k: 'st_new', cls: 'bg-amber-100 text-amber-700' },
  confirmed: { k: 'st_confirmed', cls: 'bg-violet-100 text-violet-700' },
  shipping: { k: 'st_shipping', cls: 'bg-sky-100 text-sky-700' },
  completed: { k: 'st_completed', cls: 'bg-emerald-100 text-emerald-700' },
  cancelled: { k: 'st_cancelled', cls: 'bg-slate-100 text-slate-500' }
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
    errorMsg.value = apiError(e, t('orders_failed'))
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
    saveMsg.value = t('saved')
  } catch (e) {
    saveErr.value = apiError(e, t('save_failed'))
  } finally {
    saving.value = false
  }
}

function orderLink(o: CustomerOrder) {
  return { path: storePath(slug.value, `/order/${o.code}`), query: { t: o.token } }
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
    rawSite.value = await loadPublicSite({ slug: slug.value })
    document.title = `${t('account')} – ${rawSite.value.site_title}`
    await cust.refresh()
    fillProfile()
    await loadOrders()
  } catch (e) {
    errorMsg.value = apiError(e, t('shop_not_found'))
  } finally {
    loading.value = false
  }
})
</script>

<template>
  <div v-if="loading" class="flex min-h-screen items-center justify-center gap-2 text-sm text-slate-500">
    <Loader2 class="h-5 w-5 animate-spin" /> {{ t('loading') }}
  </div>
  <div v-else-if="!site" class="flex min-h-screen items-center justify-center p-6 text-center text-sm text-slate-500">{{ errorMsg }}</div>
  <PublicShell v-else :site="site">
    <div class="mx-auto w-full max-w-4xl px-5 py-10 text-slate-800">
      <!-- Chưa đăng nhập -->
      <div v-if="!customerState.customer" class="mx-auto max-w-md space-y-4 rounded-3xl border border-slate-200 p-8 text-center">
        <div class="mx-auto flex h-14 w-14 items-center justify-center rounded-full" :style="{ backgroundColor: `color-mix(in srgb, ${accent} 12%, white)`, color: accent }">
          <User class="h-7 w-7" />
        </div>
        <h1 class="text-2xl font-extrabold">{{ t('your_account') }}</h1>
        <p class="text-sm text-slate-500">{{ t('account_intro') }}</p>
        <div class="grid grid-cols-2 gap-2 pt-2">
          <button type="button" class="rounded-full border border-slate-300 py-3 text-sm font-bold hover:bg-slate-50" @click="customerState.dialog = 'login'">{{ t('login') }}</button>
          <button type="button" class="rounded-full py-3 text-sm font-bold text-white hover:opacity-90" :style="{ backgroundColor: accent }" @click="customerState.dialog = 'register'">{{ t('register') }}</button>
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
              <h1 class="text-2xl font-extrabold">{{ t('hello', { name: customerState.customer.name }) }}</h1>
              <p class="text-sm text-slate-500">{{ customerState.customer.phone }}<template v-if="customerState.customer.email"> · {{ customerState.customer.email }}</template></p>
            </div>
          </div>
          <button type="button" class="inline-flex items-center gap-2 rounded-full border border-slate-200 px-4 py-2 text-sm font-semibold text-rose-600 hover:bg-rose-50" @click="cust.logout()">
            <LogOut class="h-4 w-4" /> {{ t('logout') }}
          </button>
        </div>

        <div class="mt-8 flex gap-1 border-b border-slate-200">
          <button
            v-for="tb in [{ v: 'orders', l: 'my_orders', i: Package }, { v: 'profile', l: 'delivery_info', i: User }] as const"
            :key="tb.v"
            type="button"
            class="-mb-px inline-flex items-center gap-2 border-b-2 px-4 py-3 text-sm font-bold transition"
            :class="tab === tb.v ? '' : 'border-transparent text-slate-500 hover:text-slate-800'"
            :style="tab === tb.v ? { borderColor: accent, color: accent } : undefined"
            @click="tab = tb.v"
          >
            <component :is="tb.i" class="h-4 w-4" /> {{ t(tb.l) }}
          </button>
        </div>

        <!-- Đơn hàng -->
        <div v-if="tab === 'orders'" class="mt-6">
          <div v-if="ordersLoading" class="flex items-center justify-center gap-2 py-12 text-sm text-slate-500"><Loader2 class="h-5 w-5 animate-spin" /> {{ t('loading_order') }}</div>
          <p v-else-if="errorMsg" class="rounded-xl bg-rose-50 px-4 py-3 text-sm text-rose-600">{{ errorMsg }}</p>
          <div v-else-if="!orders.length" class="rounded-3xl border border-dashed border-slate-300 py-14 text-center">
            <Package class="mx-auto h-8 w-8 text-slate-300" />
            <p class="mt-3 text-sm text-slate-500">{{ t('no_orders') }}</p>
            <router-link :to="storePath(slug)" class="mt-4 inline-block rounded-full px-6 py-2.5 text-sm font-bold text-white" :style="{ backgroundColor: accent }">{{ t('shop_now') }}</router-link>
          </div>
          <ul v-else class="divide-y divide-slate-100 overflow-hidden rounded-2xl border border-slate-200">
            <li v-for="o in orders" :key="o.code">
              <router-link :to="orderLink(o)" class="flex items-center gap-4 px-5 py-4 transition hover:bg-slate-50">
                <div class="min-w-0 flex-1">
                  <div class="flex flex-wrap items-center gap-2">
                    <span class="font-bold">#{{ o.code }}</span>
                    <span class="rounded-full px-2 py-0.5 text-[11px] font-bold" :class="statusText[o.status]?.cls || 'bg-slate-100 text-slate-600'">{{ statusText[o.status] ? t(statusText[o.status]!.k) : o.status }}</span>
                    <span v-if="o.payment_status === 'paid'" class="rounded-full bg-emerald-50 px-2 py-0.5 text-[11px] font-bold text-emerald-700">{{ t('paid') }}</span>
                  </div>
                  <div class="mt-1 text-xs text-slate-500">{{ formatDate(o.created_at) }} · {{ t('items_count', { n: o.qty }) }} · {{ o.payment_method === 'vnpay' ? 'VNPay' : t('pay_on_delivery') }}</div>
                </div>
                <div class="text-right font-extrabold" :style="{ color: accent }">{{ money(o.total) }}</div>
                <ChevronRight class="h-4 w-4 shrink-0 text-slate-400" />
              </router-link>
            </li>
          </ul>
        </div>

        <!-- Thông tin giao hàng -->
        <form v-else class="mt-6 max-w-xl space-y-4" @submit.prevent="saveProfile">
          <label class="block space-y-1.5">
            <span class="text-sm font-semibold">{{ t('full_name') }}</span>
            <input v-model="profile.name" class="w-full rounded-xl border border-slate-200 px-4 py-3 text-sm outline-none focus:border-slate-400" required />
          </label>
          <label class="block space-y-1.5">
            <span class="text-sm font-semibold">{{ t('phone') }}</span>
            <input :value="customerState.customer.phone" disabled class="w-full rounded-xl border border-slate-200 bg-slate-50 px-4 py-3 text-sm text-slate-500" />
          </label>
          <label class="block space-y-1.5">
            <span class="text-sm font-semibold">{{ t('email') }}</span>
            <input v-model="profile.email" type="email" class="w-full rounded-xl border border-slate-200 px-4 py-3 text-sm outline-none focus:border-slate-400" :placeholder="t('not_required')" />
          </label>
          <label class="block space-y-1.5">
            <span class="text-sm font-semibold">{{ t('address_req').replace(' *', '') }}</span>
            <textarea v-model="profile.address" rows="3" class="w-full rounded-xl border border-slate-200 px-4 py-3 text-sm outline-none focus:border-slate-400" :placeholder="t('address_ph')" />
          </label>
          <p v-if="saveErr" class="rounded-xl bg-rose-50 px-4 py-2.5 text-sm text-rose-600">{{ saveErr }}</p>
          <p v-if="saveMsg" class="rounded-xl bg-emerald-50 px-4 py-2.5 text-sm text-emerald-700">{{ saveMsg }}</p>
          <button type="submit" class="inline-flex items-center gap-2 rounded-full px-7 py-3 text-sm font-bold text-white hover:opacity-90 disabled:opacity-60" :style="{ backgroundColor: accent }" :disabled="saving">
            <Loader2 v-if="saving" class="h-4 w-4 animate-spin" /> {{ t('save_info') }}
          </button>
        </form>
      </template>
    </div>
  </PublicShell>
</template>
