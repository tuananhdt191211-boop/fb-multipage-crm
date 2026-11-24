<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ArrowLeft, Check, Shield, Sliders, UserCheck, Users, X } from 'lucide-vue-next'
import { api } from '@/lib/api'
import { useAuthStore } from './../stores/auth.stores'
import type { SystemRole, User } from '@/types/auth'

const router = useRouter()
const auth = useAuthStore()

const users = ref<User[]>([])
const loading = ref(false)
const message = ref('')
const errorMsg = ref('')

const editingUser = ref<User | null>(null)
const selectedPermissions = ref<string[]>([])
const savingPerm = ref(false)

const AVAILABLE_PERMISSIONS = [
  { key: 'inbox.view', label: 'Xem hộp thư tin nhắn' },
  { key: 'inbox.reply', label: 'Trả lời tin nhắn khách' },
  { key: 'orders.manage', label: 'Tạo & cập nhật đơn hàng' },
  { key: 'content.create', label: 'Tạo / lên lịch nội dung' }
]

async function fetchUsers() {
  loading.value = true
  errorMsg.value = ''
  try {
    const { data } = await api.get<{ users: User[] }>('/roles/users')
    users.value = data.users || []
  } catch (err: any) {
    errorMsg.value = err.response?.data?.error || 'Không thể tải danh sách người dùng.'
  } finally {
    loading.value = false
  }
}

async function handleChangeRole(target: User, newRole: SystemRole) {
  if (!auth.isAdmin || target.system_role === newRole) return
  message.value = ''
  errorMsg.value = ''
  try {
    const { data } = await api.patch<{ user: User; message: string }>(
      `/roles/users/${target.id}/role`,
      { system_role: newRole }
    )
    const idx = users.value.findIndex((u) => u.id === target.id)
    if (idx !== -1) users.value[idx] = data.user
    message.value = `Đã cập nhật ${target.full_name} thành ${roleLabel(newRole)}.`
  } catch (err: any) {
    errorMsg.value = err.response?.data?.error || 'Không thể đổi cấp bậc.'
  }
}

function openPermissionModal(target: User) {
  editingUser.value = target
  selectedPermissions.value = [...(target.permissions || [])]
}

async function savePermissions() {
  if (!editingUser.value) return
  savingPerm.value = true
  message.value = ''
  errorMsg.value = ''
  try {
    const { data } = await api.patch<{ user: User }>(
      `/roles/users/${editingUser.value.id}/permissions`,
      { permissions: selectedPermissions.value }
    )
    const idx = users.value.findIndex((u) => u.id === editingUser.value!.id)
    if (idx !== -1) users.value[idx] = data.user
    message.value = `Đã lưu phân quyền cho ${editingUser.value.full_name}.`
    editingUser.value = null
  } catch (err: any) {
    errorMsg.value = err.response?.data?.error || 'Không thể lưu phân quyền.'
  } finally {
    savingPerm.value = false
  }
}

function roleLabel(role: SystemRole) {
  if (role === 'admin') return 'Admin'
  if (role === 'manager') return 'Quản lý'
  return 'Nhân viên'
}

function roleBadgeClass(role: SystemRole) {
  if (role === 'admin') return 'bg-purple-100 text-purple-700'
  if (role === 'manager') return 'bg-blue-100 text-blue-700'
  return 'bg-slate-100 text-slate-700'
}

onMounted(fetchUsers)
</script>

<template>
  <div class="min-h-screen bg-slate-50">
    <header class="border-b border-slate-200 bg-white">
      <div class="mx-auto flex max-w-6xl items-center justify-between px-6 py-4">
        <div class="flex items-center gap-3">
          <button
            @click="router.push('/')"
            class="inline-flex items-center gap-1.5 rounded-xl border border-slate-200 px-3 py-2 text-sm font-medium text-slate-700 hover:bg-slate-50 cursor-pointer"
          >
            <ArrowLeft class="h-4 w-4" />
            <span>Trang chủ</span>
          </button>
          <div class="flex items-center gap-2">
            <Users class="h-5 w-5 text-blue-600" />
            <h1 class="text-lg font-bold text-slate-900">Phân cấp bậc & Phân quyền</h1>
          </div>
        </div>

        <span class="rounded-full bg-slate-100 px-3 py-1 text-xs font-semibold text-slate-700">
          Vai trò của bạn: {{ roleLabel(auth.user?.system_role || 'staff') }}
        </span>
      </div>
    </header>

    <main class="mx-auto max-w-6xl space-y-4 p-6">
      <div class="rounded-2xl border border-blue-100 bg-blue-50/70 p-4 text-sm text-blue-900">
        <p v-if="auth.isAdmin">
          <strong>Quyền Admin:</strong> Bạn có thể phân cấp bậc thành <strong>Quản lý</strong> hoặc <strong>Nhân viên</strong> và phân quyền chi tiết cho tài khoản.
        </p>
        <p v-else>
          <strong>Quyền Quản lý:</strong> Bạn chỉ có thể xem và phân quyền cho tài khoản thuộc cấp bậc <strong>Nhân viên</strong>.
        </p>
      </div>

      <div v-if="message" class="rounded-xl border border-emerald-200 bg-emerald-50 px-4 py-3 text-sm text-emerald-800">
        {{ message }}
      </div>
      <div v-if="errorMsg" class="rounded-xl border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">
        {{ errorMsg }}
      </div>

      <div class="overflow-hidden rounded-2xl border border-slate-200 bg-white shadow-xs">
        <table class="w-full text-left text-sm">
          <thead class="border-b border-slate-200 bg-slate-50 text-xs uppercase text-slate-500">
            <tr>
              <th class="px-5 py-3.5">Người dùng</th>
              <th class="px-5 py-3.5">Cấp bậc (Role)</th>
              <th class="px-5 py-3.5">Quyền được cấp</th>
              <th class="px-5 py-3.5 text-right">Thao tác</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-slate-100">
            <tr v-if="loading">
              <td colspan="4" class="px-5 py-8 text-center text-slate-500">Đang tải danh sách...</td>
            </tr>
            <tr v-else-if="users.length === 0">
              <td colspan="4" class="px-5 py-8 text-center text-slate-500">Chưa có tài khoản nào để quản lý.</td>
            </tr>
            <tr v-for="u in users" :key="u.id" class="hover:bg-slate-50/70">
              <td class="px-5 py-4">
                <p class="font-semibold text-slate-900">{{ u.full_name }}</p>
                <p class="text-xs text-slate-500">{{ u.email }}</p>
              </td>
              <td class="px-5 py-4">
                <select
                  v-if="auth.isAdmin && u.system_role !== 'admin'"
                  :value="u.system_role"
                  @change="handleChangeRole(u, ($event.target as HTMLSelectElement).value as SystemRole)"
                  class="rounded-lg border border-slate-300 bg-white px-2.5 py-1.5 text-xs font-semibold text-slate-800 outline-none focus:border-blue-600"
                >
                  <option value="manager">Quản lý (Manager)</option>
                  <option value="staff">Nhân viên (Staff)</option>
                </select>
                <span
                  v-else
                  class="inline-flex items-center gap-1 rounded-full px-2.5 py-1 text-xs font-semibold"
                  :class="roleBadgeClass(u.system_role)"
                >
                  <Shield class="h-3.5 w-3.5" />
                  {{ roleLabel(u.system_role) }}
                </span>
              </td>
              <td class="px-5 py-4">
                <div v-if="u.system_role === 'admin'" class="text-xs text-slate-500">Toàn quyền hệ thống</div>
                <div v-else-if="u.permissions && u.permissions.length" class="flex flex-wrap gap-1.5">
                  <span
                    v-for="p in u.permissions"
                    :key="p"
                    class="rounded-md bg-blue-50 px-2 py-0.5 text-xs font-medium text-blue-700"
                  >
                    {{ p }}
                  </span>
                </div>
                <span v-else class="text-xs text-slate-400">Chưa gán quyền chi tiết</span>
              </td>
              <td class="px-5 py-4 text-right">
                <button
                  v-if="u.system_role === 'staff' || (auth.isAdmin && u.system_role === 'manager')"
                  @click="openPermissionModal(u)"
                  class="inline-flex items-center gap-1.5 rounded-xl border border-slate-200 px-3 py-1.5 text-xs font-semibold text-slate-700 hover:bg-slate-100 cursor-pointer"
                >
                  <Sliders class="h-3.5 w-3.5" />
                  <span>Phân quyền</span>
                </button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </main>

    <div
      v-if="editingUser"
      class="fixed inset-0 z-50 flex items-center justify-center bg-slate-900/40 p-4"
    >
      <div class="w-full max-w-md rounded-2xl bg-white p-6 shadow-lg border border-slate-200">
        <div class="flex items-center justify-between border-b border-slate-100 pb-3">
          <div class="flex items-center gap-2">
            <UserCheck class="h-5 w-5 text-blue-600" />
            <h2 class="font-bold text-slate-900">Phân quyền: {{ editingUser.full_name }}</h2>
          </div>
          <button @click="editingUser = null" class="text-slate-400 hover:text-slate-600 cursor-pointer">
            <X class="h-5 w-5" />
          </button>
        </div>

        <div class="my-4 space-y-2.5">
          <label
            v-for="item in AVAILABLE_PERMISSIONS"
            :key="item.key"
            class="flex items-center justify-between rounded-xl border border-slate-200 p-3 text-sm hover:bg-slate-50 cursor-pointer"
          >
            <span>{{ item.label }}</span>
            <input
              type="checkbox"
              :value="item.key"
              v-model="selectedPermissions"
              class="h-4 w-4 rounded border-slate-300 text-blue-600"
            />
          </label>
        </div>

        <div class="flex justify-end gap-2 pt-2">
          <button
            @click="editingUser = null"
            class="rounded-xl border border-slate-200 px-4 py-2 text-sm font-medium text-slate-600 hover:bg-slate-50 cursor-pointer"
          >
            Hủy
          </button>
          <button
            @click="savePermissions"
            :disabled="savingPerm"
            class="inline-flex items-center gap-1.5 rounded-xl bg-blue-600 px-4 py-2 text-sm font-semibold text-white hover:bg-blue-700 disabled:opacity-60 cursor-pointer"
          >
            <Check class="h-4 w-4" />
            <span>{{ savingPerm ? 'Đang lưu...' : 'Lưu phân quyền' }}</span>
          </button>
        </div>
      </div>
    </div>
  </div>
</template>