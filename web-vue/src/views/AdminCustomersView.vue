<script setup lang="ts">
// Admin: theo dõi toàn bộ khách hàng — số liệu, bật/tắt trang được dùng, giới hạn, khoá tài khoản, xem như khách.
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { Eye, Loader2, Lock, Plus, RefreshCw, Search, Unlock, UserCog, X } from 'lucide-vue-next'
import { api } from '../lib/api'
import { adminMoney, adminMoneyTitle, uiLocaleTag } from '../lib/uiI18n'
import { useAuthStore } from '../stores/auth.stores'
import { ALL_FEATURES, FEATURE_LABELS, type Feature, type User } from '../types/auth'

const router = useRouter()
const auth = useAuthStore()

interface Stats {
  pages: number
  conversations: number
  unread: number
  sites: number
  published_sites: number
  orders: number
  revenue: number
  posts: number
  staff: number
}
interface Customer extends User {
  stats: Stats
}
interface Overview {
  customers: number
  active_customers: number
  new_customers_7d: number
  active_logins_7d: number
  staff: number
  totals: Stats
}

const customers = ref<Customer[]>([])
const overview = ref<Overview | null>(null)
const loading = ref(false)
const q = ref('')
const errorMsg = ref('')
const okMsg = ref('')

async function load() {
  loading.value = true
  errorMsg.value = ''
  try {
    const [c, o] = await Promise.all([api.get('/admin/customers', { params: { q: q.value.trim() || undefined } }), api.get('/admin/overview')])
    customers.value = c.data.customers || []
    overview.value = o.data
  } catch (e: any) {
    errorMsg.value = e?.response?.data?.error || 'Không tải được danh sách khách hàng.'
  } finally {
    loading.value = false
  }
}
onMounted(load)

const nf = (n: number) => new Intl.NumberFormat(uiLocaleTag()).format(Math.round(n || 0))
function when(iso?: string | null): string {
  if (!iso) return 'Chưa đăng nhập'
  const s = Math.max(0, Math.round((Date.now() - new Date(iso).getTime()) / 1000))
  if (s < 3600) return `${Math.max(1, Math.round(s / 60))} phút trước`
  if (s < 86400) return `${Math.round(s / 3600)} giờ trước`
  return new Date(iso).toLocaleDateString(uiLocaleTag())
}
const featureCount = (c: User) => (c.features == null ? ALL_FEATURES.length : c.features.length)
const limitText = (n?: number) => (n ? String(n) : '∞')

// ------------------------------------------------------------ Xem như khách
async function viewAs(c: Customer) {
  await auth.startViewAs(c.id, c.full_name)
  router.push('/')
}

// ------------------------------------------------------------ Khoá / mở nhanh
async function toggleActive(c: Customer) {
  errorMsg.value = ''
  try {
    await api.patch(`/admin/customers/${c.id}`, { is_active: !c.is_active })
    c.is_active = !c.is_active
    okMsg.value = c.is_active ? `Đã mở khoá ${c.full_name}.` : `Đã khoá ${c.full_name}. Họ và nhân viên của họ không đăng nhập được nữa.`
  } catch (e: any) {
    errorMsg.value = e?.response?.data?.error || 'Không cập nhật được tài khoản.'
  }
}

// ------------------------------------------------------------ Quản lý một khách hàng
interface Detail {
  customer: Customer
  staff: User[]
  pages: { id: number; name: string; page_id: string; is_connected: boolean }[]
  sites: { id: number; slug: string; site_title: string; is_published: boolean }[]
}
const detail = ref<Detail | null>(null)
const form = ref({ full_name: '', features: [] as Feature[], max_pages: 0, max_sites: 0, max_staff: 0 })
const newPassword = ref('')
const saving = ref(false)

async function openDetail(c: Customer) {
  errorMsg.value = ''
  try {
    const { data } = await api.get<Detail>(`/admin/customers/${c.id}`)
    data.staff = data.staff || []
    data.pages = data.pages || []
    data.sites = data.sites || []
    detail.value = data
    const u = data.customer
    form.value = {
      full_name: u.full_name,
      features: [...((u.features ?? ALL_FEATURES) as Feature[])],
      max_pages: u.max_pages || 0,
      max_sites: u.max_sites || 0,
      max_staff: u.max_staff || 0
    }
    newPassword.value = ''
  } catch (e: any) {
    errorMsg.value = e?.response?.data?.error || 'Không tải được chi tiết khách hàng.'
  }
}

function toggleFeature(f: Feature, list: Feature[]) {
  const i = list.indexOf(f)
  if (i >= 0) list.splice(i, 1)
  else list.push(f)
}

async function saveDetail() {
  if (!detail.value) return
  saving.value = true
  errorMsg.value = ''
  try {
    await api.patch(`/admin/customers/${detail.value.customer.id}`, {
      full_name: form.value.full_name,
      features: form.value.features,
      max_pages: Number(form.value.max_pages) || 0,
      max_sites: Number(form.value.max_sites) || 0,
      max_staff: Number(form.value.max_staff) || 0
    })
    okMsg.value = 'Đã lưu thay đổi cho khách hàng.'
    detail.value = null
    await load()
  } catch (e: any) {
    errorMsg.value = e?.response?.data?.error || 'Không lưu được thay đổi.'
  } finally {
    saving.value = false
  }
}

async function resetPassword() {
  if (!detail.value || newPassword.value.length < 6) {
    errorMsg.value = 'Mật khẩu tối thiểu 6 ký tự.'
    return
  }
  try {
    await api.post(`/admin/customers/${detail.value.customer.id}/password`, { password: newPassword.value })
    okMsg.value = 'Đã đặt lại mật khẩu.'
    newPassword.value = ''
  } catch (e: any) {
    errorMsg.value = e?.response?.data?.error || 'Không đặt lại được mật khẩu.'
  }
}

async function toggleStaff(s: User) {
  try {
    await api.patch(`/admin/customers/${s.id}`, { is_active: !s.is_active })
    s.is_active = !s.is_active
  } catch (e: any) {
    errorMsg.value = e?.response?.data?.error || 'Không cập nhật được nhân viên.'
  }
}

// ------------------------------------------------------------ Tạo tài khoản khách hàng
const showCreate = ref(false)
const createForm = ref({ full_name: '', email: '', password: '', features: [...ALL_FEATURES] as Feature[], max_pages: 0, max_sites: 0, max_staff: 0 })
const createError = ref('')

async function createCustomer() {
  createError.value = ''
  saving.value = true
  try {
    await api.post('/admin/customers', { ...createForm.value, max_pages: Number(createForm.value.max_pages) || 0, max_sites: Number(createForm.value.max_sites) || 0, max_staff: Number(createForm.value.max_staff) || 0 })
    showCreate.value = false
    createForm.value = { full_name: '', email: '', password: '', features: [...ALL_FEATURES], max_pages: 0, max_sites: 0, max_staff: 0 }
    okMsg.value = 'Đã tạo tài khoản khách hàng. Họ đăng nhập sẽ thấy dữ liệu trống để tự nhập.'
    await load()
  } catch (e: any) {
    createError.value = e?.response?.data?.error || 'Không tạo được tài khoản.'
  } finally {
    saving.value = false
  }
}

const cards = computed(() => {
  const o = overview.value
  if (!o) return []
  return [
    { k: 'customers', label: 'Khách hàng', value: nf(o.customers), sub: `${nf(o.active_customers)} đang hoạt động · ${nf(o.new_customers_7d)} mới 7 ngày` },
    { k: 'logins', label: 'Đăng nhập 7 ngày', value: nf(o.active_logins_7d), sub: `${nf(o.staff)} nhân viên` },
    { k: 'pages', label: 'Fanpage', value: nf(o.totals.pages), sub: `${nf(o.totals.conversations)} hội thoại · ${nf(o.totals.unread)} chưa đọc` },
    { k: 'sites', label: 'Website', value: nf(o.totals.sites), sub: `${nf(o.totals.published_sites)} đã xuất bản · ${nf(o.totals.orders)} đơn` },
    { k: 'revenue', label: 'Doanh thu (các shop)', value: adminMoney(o.totals.revenue), title: adminMoneyTitle(o.totals.revenue), sub: `${nf(o.totals.posts)} bài đăng` }
  ]
})
</script>

<template>
  <div class="min-h-screen bg-slate-50 text-slate-900">
    <div class="mx-auto max-w-6xl space-y-5 px-4 py-6">
      <div class="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 class="text-xl font-extrabold">Khách hàng</h1>
          <p class="text-xs text-slate-500">Mỗi tài khoản là một khách hàng với dữ liệu Facebook và website riêng. Bạn theo dõi toàn bộ ở đây.</p>
        </div>
        <div class="flex items-center gap-2">
          <div class="flex items-center gap-1.5 rounded-xl border border-slate-200 bg-white px-2.5">
            <Search class="h-3.5 w-3.5 text-slate-400" />
            <input v-model="q" data-testid="customer-search" placeholder="Tìm tên hoặc email..." class="w-44 bg-transparent py-2 text-xs outline-none" @keyup.enter="load" />
          </div>
          <button class="rounded-xl border border-slate-200 bg-white p-2 hover:bg-slate-100" title="Tải lại" @click="load"><RefreshCw class="h-4 w-4" :class="loading ? 'animate-spin' : ''" /></button>
          <button class="inline-flex items-center gap-1.5 rounded-xl bg-emerald-600 px-3.5 py-2 text-xs font-bold text-white hover:bg-emerald-700" data-testid="create-customer" @click="showCreate = true">
            <Plus class="h-3.5 w-3.5" /> Tạo tài khoản
          </button>
        </div>
      </div>

      <p v-if="errorMsg" class="rounded-xl bg-rose-50 px-4 py-2 text-sm font-semibold text-rose-700">{{ errorMsg }}</p>
      <p v-if="okMsg" class="rounded-xl bg-emerald-50 px-4 py-2 text-sm font-semibold text-emerald-700" data-testid="ok-msg">{{ okMsg }}</p>

      <section class="grid grid-cols-2 gap-3 lg:grid-cols-5" data-testid="overview">
        <div v-for="c in cards" :key="c.k" class="rounded-2xl bg-white p-4 shadow-sm ring-1 ring-slate-200" :data-kpi="c.k">
          <div class="text-[11px] font-bold uppercase tracking-wide text-slate-400">{{ c.label }}</div>
          <div class="mt-1 text-xl font-extrabold" :title="c.title" data-testid="kpi-value">{{ c.value }}</div>
          <div class="mt-1 text-[11px] text-slate-500">{{ c.sub }}</div>
        </div>
      </section>

      <div v-if="loading && !customers.length" class="flex items-center justify-center gap-2 py-16 text-sm text-slate-500"><Loader2 class="h-5 w-5 animate-spin" /> Đang tải...</div>
      <section v-else class="overflow-x-auto rounded-2xl bg-white shadow-sm ring-1 ring-slate-200">
        <table class="w-full min-w-[900px] text-left text-xs" data-testid="customers-table">
          <thead class="border-b border-slate-100 text-[11px] uppercase tracking-wide text-slate-400">
            <tr>
              <th class="px-4 py-3">Khách hàng</th>
              <th class="px-2">Fanpage</th>
              <th class="px-2">Hội thoại</th>
              <th class="px-2">Website</th>
              <th class="px-2">Đơn</th>
              <th class="px-2">Doanh thu</th>
              <th class="px-2">Nhân viên</th>
              <th class="px-2">Trang được dùng</th>
              <th class="px-2">Đăng nhập gần nhất</th>
              <th class="px-4 text-right">Thao tác</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-slate-100">
            <tr v-for="c in customers" :key="c.id" data-testid="customer-row" :data-id="c.id" :class="c.is_active ? '' : 'bg-slate-50 text-slate-400'">
              <td class="px-4 py-3">
                <div class="font-bold text-slate-900">{{ c.full_name }} <span v-if="!c.is_active" class="ml-1 rounded bg-rose-100 px-1.5 py-0.5 text-[10px] font-extrabold text-rose-700">ĐÃ KHOÁ</span></div>
                <div class="text-[11px] text-slate-500">{{ c.email }}</div>
              </td>
              <td class="px-2">{{ nf(c.stats.pages) }}<span class="text-slate-400"> / {{ limitText(c.max_pages) }}</span></td>
              <td class="px-2">{{ nf(c.stats.conversations) }}<span v-if="c.stats.unread" class="ml-1 rounded bg-amber-100 px-1 text-[10px] font-bold text-amber-700">{{ c.stats.unread }} chưa đọc</span></td>
              <td class="px-2">{{ nf(c.stats.sites) }}<span class="text-slate-400"> / {{ limitText(c.max_sites) }}</span></td>
              <td class="px-2">{{ nf(c.stats.orders) }}</td>
              <td class="px-2" :title="adminMoneyTitle(c.stats.revenue)">{{ adminMoney(c.stats.revenue) }}</td>
              <td class="px-2">{{ nf(c.stats.staff) }}<span class="text-slate-400"> / {{ limitText(c.max_staff) }}</span></td>
              <td class="px-2"><span class="rounded-full bg-slate-100 px-2 py-0.5 font-bold text-slate-600">{{ featureCount(c) }}/{{ ALL_FEATURES.length }} trang</span></td>
              <td class="px-2 text-slate-500">{{ when(c.last_login_at) }}</td>
              <td class="px-4">
                <div class="flex justify-end gap-1.5 whitespace-nowrap">
                  <button class="inline-flex items-center gap-1 rounded-lg border border-slate-200 px-2.5 py-1.5 font-bold text-amber-700 hover:bg-amber-50" data-testid="view-as" :disabled="!c.is_active" @click="viewAs(c)"><Eye class="h-3.5 w-3.5" /> Xem như khách</button>
                  <button class="inline-flex items-center gap-1 rounded-lg border border-slate-200 px-2.5 py-1.5 font-bold text-slate-700 hover:bg-slate-50" data-testid="manage" @click="openDetail(c)"><UserCog class="h-3.5 w-3.5" /> Quản lý</button>
                  <button class="rounded-lg border border-slate-200 p-1.5 hover:bg-slate-50" :title="c.is_active ? 'Khoá tài khoản' : 'Mở khoá'" data-testid="toggle-active" @click="toggleActive(c)">
                    <component :is="c.is_active ? Lock : Unlock" class="h-3.5 w-3.5" :class="c.is_active ? 'text-rose-500' : 'text-emerald-600'" />
                  </button>
                </div>
              </td>
            </tr>
            <tr v-if="!customers.length"><td colspan="10" class="px-4 py-10 text-center text-slate-400">Chưa có khách hàng nào. Khi có người đăng ký tài khoản, họ sẽ hiện ở đây.</td></tr>
          </tbody>
        </table>
      </section>
    </div>

    <!-- Quản lý một khách hàng -->
    <div v-if="detail" class="fixed inset-0 z-[400] flex items-start justify-center overflow-y-auto bg-slate-900/40 p-4" data-testid="detail-modal" @click.self="detail = null">
      <div class="my-6 w-full max-w-2xl space-y-4 rounded-2xl bg-white p-5 shadow-xl">
        <div class="flex items-start justify-between gap-3">
          <div>
            <h2 class="text-base font-extrabold">{{ detail.customer.full_name }}</h2>
            <p class="text-xs text-slate-500">{{ detail.customer.email }} · tham gia {{ new Date(detail.customer.created_at || '').toLocaleDateString(uiLocaleTag()) }}</p>
          </div>
          <button class="rounded-lg p-1.5 hover:bg-slate-100" @click="detail = null"><X class="h-4 w-4" /></button>
        </div>

        <label class="block space-y-1"><span class="text-xs font-bold text-slate-600">Tên</span><input v-model="form.full_name" class="w-full rounded-lg border border-slate-200 px-3 py-2 text-sm" /></label>

        <div class="space-y-1.5">
          <span class="text-xs font-bold text-slate-600">Trang được dùng</span>
          <div class="grid grid-cols-1 gap-1.5 sm:grid-cols-2">
            <label v-for="f in ALL_FEATURES" :key="f" class="flex cursor-pointer items-center gap-2 rounded-lg border px-3 py-2 text-xs font-semibold" :class="form.features.includes(f) ? 'border-emerald-300 bg-emerald-50 text-emerald-800' : 'border-slate-200 text-slate-500'">
              <input type="checkbox" :checked="form.features.includes(f)" :data-feature="f" @change="toggleFeature(f, form.features)" /> {{ FEATURE_LABELS[f] }}
            </label>
          </div>
        </div>

        <div class="grid grid-cols-3 gap-3">
          <label class="space-y-1"><span class="text-xs font-bold text-slate-600">Tối đa Fanpage</span><input v-model.number="form.max_pages" type="number" min="0" data-testid="max-pages" class="w-full rounded-lg border border-slate-200 px-3 py-2 text-sm" /></label>
          <label class="space-y-1"><span class="text-xs font-bold text-slate-600">Tối đa website</span><input v-model.number="form.max_sites" type="number" min="0" data-testid="max-sites" class="w-full rounded-lg border border-slate-200 px-3 py-2 text-sm" /></label>
          <label class="space-y-1"><span class="text-xs font-bold text-slate-600">Tối đa nhân viên</span><input v-model.number="form.max_staff" type="number" min="0" class="w-full rounded-lg border border-slate-200 px-3 py-2 text-sm" /></label>
        </div>
        <p class="-mt-2 text-[11px] text-slate-400">Số 0 = không giới hạn.</p>

        <div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
          <div class="rounded-xl bg-slate-50 p-3">
            <div class="text-xs font-bold text-slate-600">Fanpage ({{ detail.pages.length }})</div>
            <ul class="mt-1 space-y-0.5 text-xs text-slate-600">
              <li v-for="p in detail.pages" :key="p.id" class="truncate">{{ p.name }} <span v-if="!p.is_connected" class="text-slate-400">(đã tắt)</span></li>
              <li v-if="!detail.pages.length" class="text-slate-400">Chưa kết nối Fanpage nào.</li>
            </ul>
          </div>
          <div class="rounded-xl bg-slate-50 p-3">
            <div class="text-xs font-bold text-slate-600">Website ({{ detail.sites.length }})</div>
            <ul class="mt-1 space-y-0.5 text-xs text-slate-600">
              <li v-for="s in detail.sites" :key="s.id" class="truncate">{{ s.site_title }} <span class="text-slate-400">/{{ s.slug }}</span> <span v-if="!s.is_published" class="text-slate-400">(chưa xuất bản)</span></li>
              <li v-if="!detail.sites.length" class="text-slate-400">Chưa tạo website nào.</li>
            </ul>
          </div>
        </div>

        <div class="rounded-xl bg-slate-50 p-3">
          <div class="text-xs font-bold text-slate-600">Nhân viên ({{ detail.staff.length }})</div>
          <ul class="mt-1 space-y-1 text-xs">
            <li v-for="s in detail.staff" :key="s.id" class="flex items-center justify-between gap-2">
              <span class="truncate" :class="s.is_active ? '' : 'text-slate-400 line-through'">{{ s.full_name }} · {{ s.email }}</span>
              <button class="shrink-0 rounded-md border border-slate-200 px-2 py-0.5 font-bold hover:bg-white" @click="toggleStaff(s)">{{ s.is_active ? 'Khoá' : 'Mở khoá' }}</button>
            </li>
            <li v-if="!detail.staff.length" class="text-slate-400">Chưa có nhân viên.</li>
          </ul>
        </div>

        <div class="flex items-end gap-2">
          <label class="min-w-0 flex-1 space-y-1"><span class="text-xs font-bold text-slate-600">Đặt lại mật khẩu</span><input v-model="newPassword" type="text" placeholder="Mật khẩu mới (tối thiểu 6 ký tự)" class="w-full rounded-lg border border-slate-200 px-3 py-2 text-sm" /></label>
          <button class="rounded-lg border border-slate-200 px-3 py-2 text-xs font-bold hover:bg-slate-50" @click="resetPassword">Đặt lại</button>
        </div>

        <div class="flex justify-end gap-2 border-t border-slate-100 pt-3">
          <button class="rounded-xl border border-slate-200 px-4 py-2 text-xs font-bold hover:bg-slate-50" @click="detail = null">Đóng</button>
          <button class="inline-flex items-center gap-1.5 rounded-xl bg-emerald-600 px-4 py-2 text-xs font-bold text-white hover:bg-emerald-700 disabled:opacity-60" :disabled="saving" data-testid="save-customer" @click="saveDetail">
            <Loader2 v-if="saving" class="h-3.5 w-3.5 animate-spin" /> Lưu thay đổi
          </button>
        </div>
      </div>
    </div>

    <!-- Tạo tài khoản khách hàng -->
    <div v-if="showCreate" class="fixed inset-0 z-[400] flex items-start justify-center overflow-y-auto bg-slate-900/40 p-4" @click.self="showCreate = false">
      <div class="my-10 w-full max-w-lg space-y-3 rounded-2xl bg-white p-5 shadow-xl">
        <div class="flex items-center justify-between"><h2 class="text-base font-extrabold">Tạo tài khoản khách hàng</h2><button class="rounded-lg p-1.5 hover:bg-slate-100" @click="showCreate = false"><X class="h-4 w-4" /></button></div>
        <p class="text-xs text-slate-500">Khách hàng đăng nhập lần đầu sẽ thấy dữ liệu trống và tự nhập Fanpage, website của họ.</p>
        <label class="block space-y-1"><span class="text-xs font-bold text-slate-600">Họ tên</span><input v-model="createForm.full_name" data-testid="new-name" class="w-full rounded-lg border border-slate-200 px-3 py-2 text-sm" /></label>
        <label class="block space-y-1"><span class="text-xs font-bold text-slate-600">Email</span><input v-model="createForm.email" type="email" data-testid="new-email" class="w-full rounded-lg border border-slate-200 px-3 py-2 text-sm" /></label>
        <label class="block space-y-1"><span class="text-xs font-bold text-slate-600">Mật khẩu</span><input v-model="createForm.password" type="text" data-testid="new-password" class="w-full rounded-lg border border-slate-200 px-3 py-2 text-sm" /></label>
        <div class="grid grid-cols-1 gap-1.5 sm:grid-cols-2">
          <label v-for="f in ALL_FEATURES" :key="f" class="flex cursor-pointer items-center gap-2 rounded-lg border px-3 py-2 text-xs font-semibold" :class="createForm.features.includes(f) ? 'border-emerald-300 bg-emerald-50 text-emerald-800' : 'border-slate-200 text-slate-500'">
            <input type="checkbox" :checked="createForm.features.includes(f)" @change="toggleFeature(f, createForm.features)" /> {{ FEATURE_LABELS[f] }}
          </label>
        </div>
        <p v-if="createError" class="rounded-lg bg-rose-50 px-3 py-2 text-xs font-semibold text-rose-700">{{ createError }}</p>
        <div class="flex justify-end gap-2">
          <button class="rounded-xl border border-slate-200 px-4 py-2 text-xs font-bold hover:bg-slate-50" @click="showCreate = false">Huỷ</button>
          <button class="rounded-xl bg-emerald-600 px-4 py-2 text-xs font-bold text-white hover:bg-emerald-700 disabled:opacity-60" :disabled="saving" data-testid="create-submit" @click="createCustomer">Tạo tài khoản</button>
        </div>
      </div>
    </div>
  </div>
</template>
