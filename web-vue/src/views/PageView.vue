<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import {
  ArrowLeft,
  CheckCircle2,
  Facebook,
  KeyRound,
  Layers,
  Loader2,
  Plus,
  Power,
  RefreshCw,
  Search,
  Trash2,
  X
} from 'lucide-vue-next'
import { api } from '@/lib/api'
import type { FacebookPageItem } from '@/types/page'

const router = useRouter()

const pages = ref<FacebookPageItem[]>([])
const loading = ref(false)
const submitting = ref(false)
const syncing = ref(false)
const searchQuery = ref('')
const message = ref('')
const errorMsg = ref('')

const showAddModal = ref(false)
const showSyncModal = ref(false)
const userTokenInput = ref('')

const form = reactive({
  page_id: '',
  name: '',
  category: 'Bán hàng / Shop',
  avatar_url: '',
  access_token: ''
})

const filteredPages = computed(() => {
  const q = searchQuery.value.trim().toLowerCase()
  if (!q) return pages.value
  return pages.value.filter(
    (p) =>
      p.name.toLowerCase().includes(q) ||
      p.page_id.toLowerCase().includes(q) ||
      p.category.toLowerCase().includes(q)
  )
})

const activeCount = computed(() => pages.value.filter((p) => p.is_connected).length)

async function fetchPages() {
  loading.value = true
  errorMsg.value = ''
  try {
    const { data } = await api.get<{ pages: FacebookPageItem[] }>('/pages')
    pages.value = data.pages || []
  } catch (err: any) {
    errorMsg.value = err.response?.data?.error || 'Không thể tải danh sách Fanpage.'
  } finally {
    loading.value = false
  }
}

async function handleConnectPage() {
  submitting.value = true
  message.value = ''
  errorMsg.value = ''
  try {
    const { data } = await api.post<{ message: string; page: FacebookPageItem }>('/pages/connect', {
      page_id: form.page_id.trim(),
      name: form.name.trim(),
      category: form.category.trim(),
      avatar_url: form.avatar_url.trim(),
      access_token: form.access_token.trim()
    })
    message.value = data.message
    form.page_id = ''
    form.name = ''
    form.avatar_url = ''
    form.access_token = ''
    showAddModal.value = false
    await fetchPages()
  } catch (err: any) {
    errorMsg.value = err.response?.data?.error || 'Gộp Fanpage thất bại.'
  } finally {
    submitting.value = false
  }
}

async function handleSyncFromFb() {
  if (!userTokenInput.value.trim()) return
  syncing.value = true
  message.value = ''
  errorMsg.value = ''
  try {
    const { data } = await api.post<{ message: string }>('/pages/sync', {
      user_access_token: userTokenInput.value.trim()
    })
    message.value = data.message
    userTokenInput.value = ''
    showSyncModal.value = false
    await fetchPages()
  } catch (err: any) {
    errorMsg.value = err.response?.data?.error || 'Đồng bộ Fanpage thất bại.'
  } finally {
    syncing.value = false
  }
}

async function handleToggle(page: FacebookPageItem) {
  message.value = ''
  errorMsg.value = ''
  try {
    const { data } = await api.patch<{ message: string; page: FacebookPageItem }>(`/pages/${page.id}/toggle`)
    const idx = pages.value.findIndex((p) => p.id === page.id)
    if (idx !== -1) pages.value[idx] = data.page
    message.value = data.message
  } catch (err: any) {
    errorMsg.value = err.response?.data?.error || 'Không thể đổi trạng thái.'
  }
}

async function handleDelete(page: FacebookPageItem) {
  message.value = ''
  errorMsg.value = ''
  try {
    const { data } = await api.delete<{ message: string }>(`/pages/${page.id}`)
    pages.value = pages.value.filter((p) => p.id !== page.id)
    message.value = data.message
  } catch (err: any) {
    errorMsg.value = err.response?.data?.error || 'Không thể gỡ Fanpage.'
  }
}

onMounted(fetchPages)
</script>

<template>
  <div class="min-h-screen bg-white">
    <header class="border-b border-emerald-100 bg-white">
      <div class="mx-auto flex max-w-6xl flex-wrap items-center justify-between gap-3 px-6 py-4">
        <div class="flex items-center gap-3">
          <button
            @click="router.push('/')"
            class="inline-flex items-center gap-1.5 rounded-xl border border-slate-200 bg-white px-3 py-2 text-sm font-medium text-slate-700 hover:bg-emerald-50 cursor-pointer"
          >
            <ArrowLeft class="h-4 w-4" />
            <span>Trang chủ</span>
          </button>
          <div class="flex items-center gap-2">
            <div class="flex h-9 w-9 items-center justify-center rounded-xl bg-emerald-600 text-white">
              <Layers class="h-5 w-5" />
            </div>
            <div>
              <h1 class="text-lg font-bold text-slate-900">Gộp Fanpage Facebook</h1>
              <p class="text-xs text-slate-500">Quản lý tập trung nhiều Page trên một nơi</p>
            </div>
          </div>
        </div>

        <div class="flex items-center gap-2.5">
          <button
            @click="showSyncModal = true"
            class="inline-flex items-center gap-1.5 rounded-xl border border-emerald-200 bg-emerald-50 px-3.5 py-2 text-sm font-semibold text-emerald-800 hover:bg-emerald-100 cursor-pointer"
          >
            <RefreshCw class="h-4 w-4" />
            <span>Đồng bộ từ FB</span>
          </button>
          <button
            @click="showAddModal = true"
            class="inline-flex items-center gap-1.5 rounded-xl bg-emerald-600 px-4 py-2 text-sm font-semibold text-white hover:bg-emerald-700 cursor-pointer"
          >
            <Plus class="h-4 w-4" />
            <span>Gộp Page mới</span>
          </button>
        </div>
      </div>
    </header>

    <main class="mx-auto max-w-6xl space-y-6 p-6">
      <div class="grid gap-4 sm:grid-cols-3">
        <div class="rounded-2xl border border-emerald-100 bg-emerald-50/40 p-5">
          <p class="text-xs font-semibold uppercase text-emerald-700">Tổng Page đã gộp</p>
          <p class="mt-1.5 text-2xl font-bold text-slate-900">{{ pages.length }}</p>
        </div>
        <div class="rounded-2xl border border-slate-200 bg-white p-5">
          <p class="text-xs font-semibold uppercase text-emerald-700">Đang hoạt động</p>
          <p class="mt-1.5 text-2xl font-bold text-emerald-700">{{ activeCount }}</p>
        </div>
        <div class="rounded-2xl border border-slate-200 bg-white p-5">
          <p class="text-xs font-semibold uppercase text-slate-500">Đang tạm ngưng</p>
          <p class="mt-1.5 text-2xl font-bold text-slate-700">{{ pages.length - activeCount }}</p>
        </div>
      </div>

      <div v-if="message" class="flex items-center gap-2 rounded-xl border border-emerald-200 bg-emerald-50 px-4 py-3 text-sm font-medium text-emerald-800">
        <CheckCircle2 class="h-4 w-4 shrink-0" />
        <span>{{ message }}</span>
      </div>
      <div v-if="errorMsg" class="rounded-xl border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">
        {{ errorMsg }}
      </div>

      <div class="relative w-full max-w-md">
        <Search class="pointer-events-none absolute left-3.5 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400" />
        <input
          v-model="searchQuery"
          type="text"
          placeholder="Tìm theo tên Page, Page ID hoặc danh mục..."
          class="w-full rounded-xl border border-slate-200 bg-white py-2.5 pl-10 pr-4 text-sm outline-none focus:border-emerald-600 focus:ring-2 focus:ring-emerald-100"
        />
      </div>

      <div v-if="loading" class="rounded-2xl border border-slate-200 bg-white p-12 text-center text-sm text-slate-500">
        Đang tải danh sách Fanpage...
      </div>

      <div v-else-if="filteredPages.length === 0" class="rounded-2xl border border-dashed border-emerald-200 bg-emerald-50/20 p-12 text-center">
        <Facebook class="mx-auto h-10 w-10 text-emerald-600/80" />
        <h2 class="mt-3 text-base font-bold text-slate-900">Chưa có Fanpage nào được gộp</h2>
        <p class="mt-1 text-sm text-slate-500">Bấm <strong>Gộp Page mới</strong> để thêm hoặc <strong>Đồng bộ từ FB</strong>.</p>
      </div>

      <div v-else class="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
        <div
          v-for="page in filteredPages"
          :key="page.id"
          class="flex flex-col justify-between rounded-2xl border border-slate-200 bg-white p-5 shadow-xs"
        >
          <div class="flex items-start gap-3.5">
            <img
              :src="page.avatar_url"
              :alt="page.name"
              class="h-12 w-12 rounded-xl border border-slate-100 object-cover bg-emerald-50 shrink-0"
              @error="($event.target as HTMLImageElement).style.display = 'none'"
            />
            <div class="min-w-0 flex-1">
              <div class="flex items-center justify-between gap-2">
                <h3 class="truncate text-base font-bold text-slate-900">{{ page.name }}</h3>
                <span
                  class="shrink-0 rounded-full px-2.5 py-0.5 text-xs font-semibold"
                  :class="page.is_connected ? 'bg-emerald-100 text-emerald-800' : 'bg-slate-100 text-slate-600'"
                >
                  {{ page.is_connected ? 'Hoạt động' : 'Tạm ngưng' }}
                </span>
              </div>
              <p class="mt-0.5 text-xs text-slate-500">ID: {{ page.page_id }}</p>
              <p class="mt-1 inline-block rounded-md bg-emerald-50 px-2 py-0.5 text-xs font-medium text-emerald-700">
                {{ page.category }}
              </p>
            </div>
          </div>

          <div class="mt-5 flex items-center justify-between border-t border-slate-100 pt-3.5">
            <button
              @click="handleToggle(page)"
              class="inline-flex items-center gap-1.5 rounded-xl border border-slate-200 px-3 py-1.5 text-xs font-semibold text-slate-700 hover:bg-emerald-50 cursor-pointer"
            >
              <Power class="h-3.5 w-3.5" />
              <span>{{ page.is_connected ? 'Tạm ngưng' : 'Bật lại' }}</span>
            </button>
            <button
              @click="handleDelete(page)"
              class="inline-flex items-center gap-1.5 rounded-xl border border-slate-200 px-3 py-1.5 text-xs font-semibold text-slate-600 hover:border-red-200 hover:bg-red-50 hover:text-red-600 cursor-pointer"
            >
              <Trash2 class="h-3.5 w-3.5" />
              <span>Gỡ Page</span>
            </button>
          </div>
        </div>
      </div>
    </main>

    <div v-if="showAddModal" class="fixed inset-0 z-50 flex items-center justify-center bg-slate-900/35 p-4">
      <div class="w-full max-w-md rounded-2xl border border-emerald-100 bg-white p-6 shadow-lg">
        <div class="flex items-center justify-between border-b border-slate-100 pb-3">
          <h2 class="text-base font-bold text-slate-900">Gộp Fanpage mới</h2>
          <button @click="showAddModal = false" class="text-slate-400 hover:text-slate-600 cursor-pointer">
            <X class="h-5 w-5" />
          </button>
        </div>
        <form @submit.prevent="handleConnectPage" class="mt-4 space-y-3.5">
          <div>
            <label class="block text-xs font-semibold text-slate-700">Page ID Facebook *</label>
            <input v-model="form.page_id" type="text" required class="mt-1 w-full rounded-xl border border-slate-200 px-3.5 py-2 text-sm outline-none focus:border-emerald-600" />
          </div>
          <div>
            <label class="block text-xs font-semibold text-slate-700">Tên Fanpage *</label>
            <input v-model="form.name" type="text" required class="mt-1 w-full rounded-xl border border-slate-200 px-3.5 py-2 text-sm outline-none focus:border-emerald-600" />
          </div>
          <div>
            <label class="block text-xs font-semibold text-slate-700">Danh mục</label>
            <input v-model="form.category" type="text" class="mt-1 w-full rounded-xl border border-slate-200 px-3.5 py-2 text-sm outline-none focus:border-emerald-600" />
          </div>
          <div>
            <label class="block text-xs font-semibold text-slate-700">Page Access Token *</label>
            <input v-model="form.access_token" type="password" required class="mt-1 w-full rounded-xl border border-slate-200 px-3.5 py-2 text-sm outline-none focus:border-emerald-600" />
          </div>
          <div class="flex justify-end gap-2 pt-2">
            <button type="button" @click="showAddModal = false" class="rounded-xl border border-slate-200 px-4 py-2 text-sm text-slate-600 cursor-pointer">Hủy</button>
            <button type="submit" :disabled="submitting" class="inline-flex items-center gap-1.5 rounded-xl bg-emerald-600 px-4 py-2 text-sm font-semibold text-white hover:bg-emerald-700 cursor-pointer">
              <Loader2 v-if="submitting" class="h-4 w-4 animate-spin" />
              <span>{{ submitting ? 'Đang lưu...' : 'Xác nhận gộp Page' }}</span>
            </button>
          </div>
        </form>
      </div>
    </div>

    <div v-if="showSyncModal" class="fixed inset-0 z-50 flex items-center justify-center bg-slate-900/35 p-4">
      <div class="w-full max-w-md rounded-2xl border border-emerald-100 bg-white p-6 shadow-lg">
        <div class="flex items-center justify-between border-b border-slate-100 pb-3">
          <div class="flex items-center gap-2">
            <KeyRound class="h-4 w-4 text-emerald-600" />
            <h2 class="text-base font-bold text-slate-900">Đồng bộ từ Facebook</h2>
          </div>
          <button @click="showSyncModal = false" class="text-slate-400 hover:text-slate-600 cursor-pointer">
            <X class="h-5 w-5" />
          </button>
        </div>
        <form @submit.prevent="handleSyncFromFb" class="mt-4 space-y-4">
          <div>
            <label class="block text-xs font-semibold text-slate-700">Facebook User Access Token</label>
            <input v-model="userTokenInput" type="password" required placeholder="EAA..." class="mt-1 w-full rounded-xl border border-slate-200 px-3.5 py-2 text-sm outline-none focus:border-emerald-600" />
          </div>
          <div class="flex justify-end gap-2">
            <button type="button" @click="showSyncModal = false" class="rounded-xl border border-slate-200 px-4 py-2 text-sm text-slate-600 cursor-pointer">Hủy</button>
            <button type="submit" :disabled="syncing" class="inline-flex items-center gap-1.5 rounded-xl bg-emerald-600 px-4 py-2 text-sm font-semibold text-white hover:bg-emerald-700 cursor-pointer">
              <Loader2 v-if="syncing" class="h-4 w-4 animate-spin" />
              <span>{{ syncing ? 'Đang đồng bộ...' : 'Bắt đầu đồng bộ' }}</span>
            </button>
          </div>
        </form>
      </div>
    </div>
  </div>
</template>