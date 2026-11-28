<script setup lang="ts">
// Khách hàng quản lý nhân viên của mình: tạo tài khoản, chọn trang mỗi người được dùng, khoá / mở, đặt lại mật khẩu.
import { computed, onMounted, ref } from 'vue'
import { Loader2, Lock, Plus, Trash2, Unlock, UserCog, X } from 'lucide-vue-next'
import { api } from '../lib/api'
import { uiLocaleTag } from '../lib/uiI18n'
import { useAuthStore } from '../stores/auth.stores'
import { FEATURE_LABELS, type Feature, type User } from '../types/auth'

const auth = useAuthStore()

const staff = ref<User[]>([])
const myFeatures = ref<Feature[]>([])
const maxStaff = ref(0)
const loading = ref(false)
const errorMsg = ref('')
const okMsg = ref('')

async function load() {
  loading.value = true
  try {
    const { data } = await api.get('/team')
    staff.value = data.staff || []
    myFeatures.value = (data.features || []) as Feature[]
    maxStaff.value = data.max_staff || 0
  } catch (e: any) {
    errorMsg.value = e?.response?.data?.error || 'Không tải được danh sách nhân viên.'
  } finally {
    loading.value = false
  }
}
onMounted(load)

const full = computed(() => maxStaff.value > 0 && staff.value.length >= maxStaff.value)
const featureText = (s: User) => (s.features || []).map((f) => FEATURE_LABELS[f as Feature] || f).join(', ') || 'Chưa được cấp trang nào'

function toggle(f: Feature, list: Feature[]) {
  const i = list.indexOf(f)
  if (i >= 0) list.splice(i, 1)
  else list.push(f)
}

// ------------------------------------------------------------ Thêm nhân viên
const showCreate = ref(false)
const createForm = ref({ full_name: '', email: '', password: '', features: [] as Feature[] })
const createError = ref('')
const saving = ref(false)

function openCreate() {
  createForm.value = { full_name: '', email: '', password: '', features: [...myFeatures.value] }
  createError.value = ''
  showCreate.value = true
}

async function createStaff() {
  saving.value = true
  createError.value = ''
  try {
    await api.post('/team', createForm.value)
    showCreate.value = false
    okMsg.value = 'Đã tạo tài khoản nhân viên. Gửi email và mật khẩu này cho nhân viên để họ đăng nhập.'
    await load()
  } catch (e: any) {
    createError.value = e?.response?.data?.error || 'Không tạo được tài khoản.'
  } finally {
    saving.value = false
  }
}

// ------------------------------------------------------------ Sửa / khoá / xoá
const editing = ref<User | null>(null)
const editFeatures = ref<Feature[]>([])
const newPassword = ref('')

function openEdit(s: User) {
  editing.value = s
  editFeatures.value = [...((s.features || []) as Feature[])]
  newPassword.value = ''
}

async function saveEdit() {
  if (!editing.value) return
  saving.value = true
  try {
    await api.patch(`/team/${editing.value.id}`, { features: editFeatures.value })
    if (newPassword.value) {
      if (newPassword.value.length < 6) {
        errorMsg.value = 'Mật khẩu tối thiểu 6 ký tự.'
        return
      }
      await api.post(`/team/${editing.value.id}/password`, { password: newPassword.value })
    }
    okMsg.value = 'Đã lưu thay đổi cho nhân viên.'
    editing.value = null
    await load()
  } catch (e: any) {
    errorMsg.value = e?.response?.data?.error || 'Không lưu được thay đổi.'
  } finally {
    saving.value = false
  }
}

async function toggleActive(s: User) {
  try {
    await api.patch(`/team/${s.id}`, { is_active: !s.is_active })
    s.is_active = !s.is_active
  } catch (e: any) {
    errorMsg.value = e?.response?.data?.error || 'Không cập nhật được nhân viên.'
  }
}

async function remove(s: User) {
  if (!confirm(`Xoá tài khoản nhân viên ${s.full_name}?`)) return
  try {
    await api.delete(`/team/${s.id}`)
    await load()
  } catch (e: any) {
    errorMsg.value = e?.response?.data?.error || 'Không xoá được nhân viên.'
  }
}
</script>

<template>
  <div class="min-h-screen bg-slate-50 text-slate-900">
    <div class="mx-auto max-w-4xl space-y-5 px-4 py-6">
      <div class="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 class="text-xl font-extrabold">Nhân viên</h1>
          <p class="text-xs text-slate-500">Nhân viên làm việc chung trên dữ liệu của bạn ({{ auth.user?.full_name }}); mỗi người chỉ dùng các trang bạn cấp.</p>
        </div>
        <button
          class="inline-flex items-center gap-1.5 rounded-xl bg-emerald-600 px-3.5 py-2 text-xs font-bold text-white hover:bg-emerald-700 disabled:opacity-50"
          :disabled="full || auth.viewing"
          data-testid="add-staff"
          @click="openCreate"
        >
          <Plus class="h-3.5 w-3.5" /> Thêm nhân viên
        </button>
      </div>

      <p v-if="maxStaff > 0" class="text-xs text-slate-500" data-testid="staff-limit">Đã tạo {{ staff.length }}/{{ maxStaff }} nhân viên được phép.</p>
      <p v-if="errorMsg" class="rounded-xl bg-rose-50 px-4 py-2 text-sm font-semibold text-rose-700">{{ errorMsg }}</p>
      <p v-if="okMsg" class="rounded-xl bg-emerald-50 px-4 py-2 text-sm font-semibold text-emerald-700" data-testid="ok-msg">{{ okMsg }}</p>

      <div v-if="loading && !staff.length" class="flex items-center justify-center gap-2 py-16 text-sm text-slate-500"><Loader2 class="h-5 w-5 animate-spin" /> Đang tải...</div>
      <section v-else class="space-y-2.5" data-testid="staff-list">
        <div v-for="s in staff" :key="s.id" class="flex flex-wrap items-center justify-between gap-3 rounded-2xl bg-white p-4 shadow-sm ring-1 ring-slate-200" data-testid="staff-row" :class="s.is_active ? '' : 'opacity-60'">
          <div class="min-w-0">
            <div class="font-bold">{{ s.full_name }} <span v-if="!s.is_active" class="ml-1 rounded bg-rose-100 px-1.5 py-0.5 text-[10px] font-extrabold text-rose-700">ĐÃ KHOÁ</span></div>
            <div class="text-xs text-slate-500">{{ s.email }} · {{ s.last_login_at ? 'đăng nhập ' + new Date(s.last_login_at).toLocaleDateString(uiLocaleTag()) : 'chưa đăng nhập' }}</div>
            <div class="mt-1 text-[11px] text-slate-500">Được dùng: {{ featureText(s) }}</div>
          </div>
          <div v-if="!auth.viewing" class="flex gap-1.5">
            <button class="inline-flex items-center gap-1 rounded-lg border border-slate-200 px-2.5 py-1.5 text-xs font-bold hover:bg-slate-50" data-testid="edit-staff" @click="openEdit(s)"><UserCog class="h-3.5 w-3.5" /> Sửa</button>
            <button class="rounded-lg border border-slate-200 p-1.5 hover:bg-slate-50" :title="s.is_active ? 'Khoá' : 'Mở khoá'" @click="toggleActive(s)"><component :is="s.is_active ? Lock : Unlock" class="h-3.5 w-3.5" /></button>
            <button class="rounded-lg border border-slate-200 p-1.5 text-rose-600 hover:bg-rose-50" title="Xoá" @click="remove(s)"><Trash2 class="h-3.5 w-3.5" /></button>
          </div>
        </div>
        <p v-if="!staff.length" class="rounded-2xl bg-white p-8 text-center text-sm text-slate-400 ring-1 ring-slate-200">Bạn chưa có nhân viên nào. Bấm "Thêm nhân viên" để tạo tài khoản cho người làm chung.</p>
      </section>
    </div>

    <div v-if="showCreate" class="fixed inset-0 z-[400] flex items-start justify-center overflow-y-auto bg-slate-900/40 p-4" @click.self="showCreate = false">
      <div class="my-10 w-full max-w-lg space-y-3 rounded-2xl bg-white p-5 shadow-xl">
        <div class="flex items-center justify-between"><h2 class="text-base font-extrabold">Thêm nhân viên</h2><button class="rounded-lg p-1.5 hover:bg-slate-100" @click="showCreate = false"><X class="h-4 w-4" /></button></div>
        <label class="block space-y-1"><span class="text-xs font-bold text-slate-600">Họ tên</span><input v-model="createForm.full_name" data-testid="staff-name" class="w-full rounded-lg border border-slate-200 px-3 py-2 text-sm" /></label>
        <label class="block space-y-1"><span class="text-xs font-bold text-slate-600">Email đăng nhập</span><input v-model="createForm.email" type="email" data-testid="staff-email" class="w-full rounded-lg border border-slate-200 px-3 py-2 text-sm" /></label>
        <label class="block space-y-1"><span class="text-xs font-bold text-slate-600">Mật khẩu</span><input v-model="createForm.password" type="text" data-testid="staff-password" class="w-full rounded-lg border border-slate-200 px-3 py-2 text-sm" /></label>
        <div class="space-y-1.5">
          <span class="text-xs font-bold text-slate-600">Trang được dùng</span>
          <div class="grid grid-cols-1 gap-1.5 sm:grid-cols-2">
            <label v-for="f in myFeatures" :key="f" class="flex cursor-pointer items-center gap-2 rounded-lg border px-3 py-2 text-xs font-semibold" :class="createForm.features.includes(f) ? 'border-emerald-300 bg-emerald-50 text-emerald-800' : 'border-slate-200 text-slate-500'">
              <input type="checkbox" :checked="createForm.features.includes(f)" @change="toggle(f, createForm.features)" /> {{ FEATURE_LABELS[f] }}
            </label>
          </div>
          <p class="text-[11px] text-slate-400">Nhân viên không kết nối / gỡ Fanpage, không tạo / xoá website và không quản lý nhân viên khác.</p>
        </div>
        <p v-if="createError" class="rounded-lg bg-rose-50 px-3 py-2 text-xs font-semibold text-rose-700">{{ createError }}</p>
        <div class="flex justify-end gap-2">
          <button class="rounded-xl border border-slate-200 px-4 py-2 text-xs font-bold hover:bg-slate-50" @click="showCreate = false">Huỷ</button>
          <button class="rounded-xl bg-emerald-600 px-4 py-2 text-xs font-bold text-white hover:bg-emerald-700 disabled:opacity-60" :disabled="saving" data-testid="staff-submit" @click="createStaff">Tạo tài khoản</button>
        </div>
      </div>
    </div>

    <div v-if="editing" class="fixed inset-0 z-[400] flex items-start justify-center overflow-y-auto bg-slate-900/40 p-4" @click.self="editing = null">
      <div class="my-10 w-full max-w-lg space-y-3 rounded-2xl bg-white p-5 shadow-xl">
        <div class="flex items-center justify-between"><h2 class="text-base font-extrabold">{{ editing.full_name }}</h2><button class="rounded-lg p-1.5 hover:bg-slate-100" @click="editing = null"><X class="h-4 w-4" /></button></div>
        <div class="grid grid-cols-1 gap-1.5 sm:grid-cols-2">
          <label v-for="f in myFeatures" :key="f" class="flex cursor-pointer items-center gap-2 rounded-lg border px-3 py-2 text-xs font-semibold" :class="editFeatures.includes(f) ? 'border-emerald-300 bg-emerald-50 text-emerald-800' : 'border-slate-200 text-slate-500'">
            <input type="checkbox" :checked="editFeatures.includes(f)" :data-feature="f" @change="toggle(f, editFeatures)" /> {{ FEATURE_LABELS[f] }}
          </label>
        </div>
        <label class="block space-y-1"><span class="text-xs font-bold text-slate-600">Đặt lại mật khẩu (để trống nếu không đổi)</span><input v-model="newPassword" type="text" class="w-full rounded-lg border border-slate-200 px-3 py-2 text-sm" /></label>
        <div class="flex justify-end gap-2">
          <button class="rounded-xl border border-slate-200 px-4 py-2 text-xs font-bold hover:bg-slate-50" @click="editing = null">Đóng</button>
          <button class="rounded-xl bg-emerald-600 px-4 py-2 text-xs font-bold text-white hover:bg-emerald-700 disabled:opacity-60" :disabled="saving" data-testid="edit-submit" @click="saveEdit">Lưu</button>
        </div>
      </div>
    </div>
  </div>
</template>
