<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import {
  ArrowLeft,
  Check,
  CheckCircle2,
  Copy,
  Facebook,
  KeyRound,
  Layers,
  Loader2,
  PauseCircle,
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
const statusFilter = ref<'all' | 'active' | 'paused'>('all')
const message = ref('')
const errorMsg = ref('')
const copiedId = ref('')
const brokenAvatars = ref<Record<string, boolean>>({})

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

const activeCount = computed(() => pages.value.filter((p) => p.is_connected).length)
const pausedCount = computed(() => pages.value.length - activeCount.value)

const filteredPages = computed(() => {
  const q = searchQuery.value.trim().toLowerCase()
  return pages.value.filter((p) => {
    if (statusFilter.value === 'active' && !p.is_connected) return false
    if (statusFilter.value === 'paused' && p.is_connected) return false
    if (!q) return true
    return (
      p.name.toLowerCase().includes(q) ||
      p.page_id.toLowerCase().includes(q) ||
      (p.category || '').toLowerCase().includes(q)
    )
  })
})

function copyPageId(pageId: string) {
  navigator.clipboard?.writeText(pageId)
  copiedId.value = pageId
  setTimeout(() => {
    if (copiedId.value === pageId) copiedId.value = ''
  }, 1800)
}

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
  <div translate="no" class="notranslate min-h-screen bg-[#f4f7f6] text-slate-900">
    <!-- Top Header -->
    <header class="sticky top-0 z-20 border-b border-slate-200 bg-white">
      <div class="mx-auto flex max-w-7xl flex-wrap items-center justify-between gap-3 px-4 py-3 sm:px-6 lg:px-8">
        <div class="flex min-w-0 items-center gap-3">
          <button
            type="button"
            @click="router.push('/')"
            class="inline-flex shrink-0 items-center gap-1.5 rounded-xl border border-slate-200 bg-white px-3 py-2 text-xs font-semibold text-slate-700 transition hover:border-emerald-300 hover:bg-emerald-50 hover:text-emerald-800 cursor-pointer"
          >
            <ArrowLeft class="h-3.5 w-3.5" />
            <span>Trang chủ</span>
          </button>

          <div class="flex min-w-0 items-center gap-2.5 border-l border-slate-200 pl-3">
            <div class="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-emerald-600 text-white">
              <Layers class="h-4 w-4" />
            </div>
            <div class="min-w-0 text-left">
              <h1 class="truncate text-sm sm:text-base font-extrabold tracking-tight !text-slate-900">
                Gộp Fanpage Facebook
              </h1>
              <p class="hidden sm:block truncate text-xs text-slate-500">
                Quản lý tập trung toàn bộ Fanpage kết nối trên một nơi
              </p>
            </div>
          </div>
        </div>

        <div class="flex shrink-0 items-center gap-2">
          <button
            type="button"
            @click="showSyncModal = true"
            class="inline-flex items-center gap-1.5 whitespace-nowrap rounded-xl border border-emerald-200 bg-emerald-50 px-3.5 py-2 text-xs font-bold text-emerald-800 transition hover:bg-emerald-100 cursor-pointer"
          >
            <RefreshCw class="h-3.5 w-3.5" />
            <span>Đồng bộ từ FB</span>
          </button>

          <button
            type="button"
            @click="showAddModal = true"
            class="inline-flex items-center gap-1.5 whitespace-nowrap rounded-xl bg-emerald-600 px-4 py-2 text-xs font-bold text-white transition hover:bg-emerald-700 cursor-pointer"
          >
            <Plus class="h-3.5 w-3.5" />
            <span>Gộp Page mới</span>
          </button>
        </div>
      </div>
    </header>

    <!-- Main Content -->
    <main class="mx-auto max-w-7xl space-y-5 px-4 py-6 sm:px-6 lg:px-8">
      <!-- 3 Thẻ thống kê -->
      <div class="grid grid-cols-1 gap-4 sm:grid-cols-3">
        <div class="rounded-2xl border border-slate-200 bg-white p-5 text-left shadow-sm">
          <div class="flex items-center justify-between">
            <span class="text-xs font-semibold text-slate-500">Tổng Page đã gộp</span>
            <div class="flex h-8 w-8 items-center justify-center rounded-lg bg-emerald-50 text-emerald-600">
              <Layers class="h-4 w-4" />
            </div>
          </div>
          <p class="mt-2.5 text-2xl font-extrabold !text-slate-900">{{ pages.length }}</p>
          <p class="mt-1 text-xs text-slate-400">Tất cả Fanpage trong hệ thống</p>
        </div>

        <div class="rounded-2xl border border-slate-200 bg-white p-5 text-left shadow-sm">
          <div class="flex items-center justify-between">
            <span class="text-xs font-semibold text-slate-500">Đang hoạt động</span>
            <div class="flex h-8 w-8 items-center justify-center rounded-lg bg-teal-50 text-teal-600">
              <CheckCircle2 class="h-4 w-4" />
            </div>
          </div>
          <p class="mt-2.5 text-2xl font-extrabold !text-emerald-700">{{ activeCount }}</p>
          <p class="mt-1 text-xs text-slate-400">Đang đồng bộ tin nhắn & tự động CSKH</p>
        </div>

        <div class="rounded-2xl border border-slate-200 bg-white p-5 text-left shadow-sm">
          <div class="flex items-center justify-between">
            <span class="text-xs font-semibold text-slate-500">Đang tạm ngưng</span>
            <div class="flex h-8 w-8 items-center justify-center rounded-lg bg-slate-100 text-slate-500">
              <PauseCircle class="h-4 w-4" />
            </div>
          </div>
          <p class="mt-2.5 text-2xl font-extrabold !text-slate-700">{{ pausedCount }}</p>
          <p class="mt-1 text-xs text-slate-400">Tạm tắt nhận tin nhắn mới</p>
        </div>
      </div>

      <!-- Thông báo -->
      <div
        v-if="message"
        class="flex items-center gap-2 rounded-xl border border-emerald-200 bg-emerald-50 px-4 py-3 text-left text-xs font-semibold text-emerald-800"
      >
        <CheckCircle2 class="h-4 w-4 shrink-0 text-emerald-600" />
        <span>{{ message }}</span>
      </div>
      <div
        v-if="errorMsg"
        class="rounded-xl border border-rose-200 bg-rose-50 px-4 py-3 text-left text-xs font-medium text-rose-700"
      >
        {{ errorMsg }}
      </div>

      <!-- Tìm kiếm & Bộ lọc -->
      <div class="flex flex-col justify-between gap-3 sm:flex-row sm:items-center">
        <div class="relative w-full sm:max-w-md">
          <Search class="pointer-events-none absolute left-3.5 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400" />
          <input
            v-model="searchQuery"
            type="text"
            placeholder="Tìm theo tên Page, Page ID hoặc danh mục..."
            class="w-full rounded-xl border border-slate-200 bg-white py-2.5 pl-10 pr-4 text-sm !text-slate-900 placeholder:text-slate-400 shadow-sm outline-none focus:border-emerald-500 focus:ring-2 focus:ring-emerald-100"
          />
        </div>

        <div class="inline-flex shrink-0 items-center rounded-xl border border-slate-200 bg-white p-1 shadow-sm self-start">
          <button
            type="button"
            @click="statusFilter = 'all'"
            class="whitespace-nowrap rounded-lg px-3 py-1.5 text-xs font-semibold transition cursor-pointer"
            :class="statusFilter === 'all' ? 'bg-emerald-600 text-white' : 'text-slate-600 hover:text-slate-900'"
          >
            Tất cả ({{ pages.length }})
          </button>
          <button
            type="button"
            @click="statusFilter = 'active'"
            class="whitespace-nowrap rounded-lg px-3 py-1.5 text-xs font-semibold transition cursor-pointer"
            :class="statusFilter === 'active' ? 'bg-emerald-600 text-white' : 'text-slate-600 hover:text-slate-900'"
          >
            Hoạt động ({{ activeCount }})
          </button>
          <button
            type="button"
            @click="statusFilter = 'paused'"
            class="whitespace-nowrap rounded-lg px-3 py-1.5 text-xs font-semibold transition cursor-pointer"
            :class="statusFilter === 'paused' ? 'bg-emerald-600 text-white' : 'text-slate-600 hover:text-slate-900'"
          >
            Tạm ngưng ({{ pausedCount }})
          </button>
        </div>
      </div>

      <!-- Danh sách Fanpage -->
      <div v-if="loading" class="rounded-2xl border border-slate-200 bg-white p-12 text-center text-xs font-medium text-slate-500">
        Đang tải danh sách Fanpage...
      </div>

      <div
        v-else-if="filteredPages.length === 0"
        class="rounded-2xl border border-dashed border-emerald-200 bg-white p-12 text-center"
      >
        <div class="mx-auto flex h-12 w-12 items-center justify-center rounded-2xl bg-emerald-50 text-emerald-600">
          <Facebook class="h-6 w-6" />
        </div>
        <h2 class="mt-3 text-sm font-bold !text-slate-900">Không tìm thấy Fanpage phù hợp</h2>
        <p class="mt-1 text-xs text-slate-500">
          Thử đổi từ khóa tìm kiếm hoặc bấm <strong>Gộp Page mới</strong> / <strong>Đồng bộ từ FB</strong>.
        </p>
      </div>

      <div v-else class="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        <div
          v-for="page in filteredPages"
          :key="page.id"
          class="flex flex-col justify-between rounded-2xl border border-slate-200 bg-white p-5 text-left shadow-sm transition hover:border-emerald-300"
        >
          <div class="flex items-start gap-3.5">
            <div class="flex h-12 w-12 shrink-0 items-center justify-center overflow-hidden rounded-xl border border-slate-100 bg-emerald-50 text-emerald-600">
              <img
                v-if="page.avatar_url && !brokenAvatars[page.page_id]"
                :src="page.avatar_url"
                :alt="page.name"
                class="h-full w-full object-cover"
                @error="brokenAvatars[page.page_id] = true"
              />
              <Facebook v-else class="h-5 w-5" />
            </div>

            <div class="min-w-0 flex-1">
              <div class="flex items-center justify-between gap-2">
                <h3 class="truncate text-sm font-bold !text-slate-900" :title="page.name">
                  {{ page.name }}
                </h3>
                <span
                  class="shrink-0 rounded-full px-2.5 py-0.5 text-[11px] font-bold"
                  :class="
                    page.is_connected
                      ? 'bg-emerald-50 text-emerald-700 ring-1 ring-emerald-200'
                      : 'bg-slate-100 text-slate-500'
                  "
                >
                  {{ page.is_connected ? 'Hoạt động' : 'Tạm ngưng' }}
                </span>
              </div>

              <button
                type="button"
                @click="copyPageId(page.page_id)"
                title="Sao chép Page ID"
                class="mt-1 inline-flex items-center gap-1 text-xs text-slate-400 hover:text-emerald-700 cursor-pointer"
              >
                <span>ID: {{ page.page_id }}</span>
                <Check v-if="copiedId === page.page_id" class="h-3 w-3 text-emerald-600" />
                <Copy v-else class="h-3 w-3" />
              </button>

              <div class="mt-2">
                <span class="inline-block rounded-md bg-emerald-50 px-2 py-0.5 text-[11px] font-semibold text-emerald-700">
                  {{ page.category || 'Fanpage' }}
                </span>
              </div>
            </div>
          </div>

          <div class="mt-4 flex items-center justify-between border-t border-slate-100 pt-3.5">
            <button
              type="button"
              @click="handleToggle(page)"
              class="inline-flex items-center gap-1.5 rounded-xl border border-slate-200 bg-white px-3 py-1.5 text-xs font-semibold text-slate-700 transition hover:border-emerald-300 hover:bg-emerald-50 hover:text-emerald-800 cursor-pointer"
            >
              <Power class="h-3.5 w-3.5" />
              <span>{{ page.is_connected ? 'Tạm ngưng' : 'Bật lại' }}</span>
            </button>

            <button
              type="button"
              @click="handleDelete(page)"
              class="inline-flex items-center gap-1.5 rounded-xl border border-slate-200 bg-white px-3 py-1.5 text-xs font-semibold text-slate-500 transition hover:border-rose-200 hover:bg-rose-50 hover:text-rose-600 cursor-pointer"
            >
              <Trash2 class="h-3.5 w-3.5" />
              <span>Gỡ Page</span>
            </button>
          </div>
        </div>
      </div>
    </main>

    <!-- Modal Gộp Fanpage mới -->
    <div v-if="showAddModal" class="fixed inset-0 z-50 flex items-center justify-center bg-slate-900/40 p-4">
      <div class="w-full max-w-md rounded-2xl border border-slate-200 bg-white p-6 text-left shadow-xl">
        <div class="flex items-center justify-between border-b border-slate-100 pb-3">
          <h2 class="text-base font-extrabold !text-slate-900">Gộp Fanpage mới</h2>
          <button @click="showAddModal = false" class="rounded-lg p-1 text-slate-400 hover:bg-slate-100 hover:text-slate-600 cursor-pointer">
            <X class="h-5 w-5" />
          </button>
        </div>

        <form @submit.prevent="handleConnectPage" class="mt-4 space-y-3.5">
          <div>
            <label class="block text-xs font-bold !text-slate-700">Page ID Facebook *</label>
            <input
              v-model="form.page_id"
              type="text"
              required
              placeholder="VD: 1453898437804281"
              class="mt-1 w-full rounded-xl border border-slate-200 bg-slate-50 px-3.5 py-2 text-sm !text-slate-900 outline-none focus:border-emerald-600 focus:bg-white"
            />
          </div>
          <div>
            <label class="block text-xs font-bold !text-slate-700">Tên Fanpage *</label>
            <input
              v-model="form.name"
              type="text"
              required
              placeholder="Nhập tên hiển thị của Page"
              class="mt-1 w-full rounded-xl border border-slate-200 bg-slate-50 px-3.5 py-2 text-sm !text-slate-900 outline-none focus:border-emerald-600 focus:bg-white"
            />
          </div>
          <div>
            <label class="block text-xs font-bold !text-slate-700">Danh mục</label>
            <input
              v-model="form.category"
              type="text"
              class="mt-1 w-full rounded-xl border border-slate-200 bg-slate-50 px-3.5 py-2 text-sm !text-slate-900 outline-none focus:border-emerald-600 focus:bg-white"
            />
          </div>
          <div>
            <label class="block text-xs font-bold !text-slate-700">Page Access Token *</label>
            <input
              v-model="form.access_token"
              type="password"
              required
              placeholder="EAA..."
              class="mt-1 w-full rounded-xl border border-slate-200 bg-slate-50 px-3.5 py-2 text-sm !text-slate-900 outline-none focus:border-emerald-600 focus:bg-white"
            />
          </div>
          <div class="flex justify-end gap-2 pt-2">
            <button
              type="button"
              @click="showAddModal = false"
              class="rounded-xl border border-slate-200 px-4 py-2 text-xs font-semibold text-slate-600 hover:bg-slate-50 cursor-pointer"
            >
              Hủy
            </button>
            <button
              type="submit"
              :disabled="submitting"
              class="inline-flex items-center gap-1.5 rounded-xl bg-emerald-600 px-4 py-2 text-xs font-bold text-white hover:bg-emerald-700 cursor-pointer"
            >
              <Loader2 v-if="submitting" class="h-4 w-4 animate-spin" />
              <span>{{ submitting ? 'Đang lưu...' : 'Xác nhận gộp Page' }}</span>
            </button>
          </div>
        </form>
      </div>
    </div>

    <!-- Modal Đồng bộ từ Facebook -->
    <div v-if="showSyncModal" class="fixed inset-0 z-50 flex items-center justify-center bg-slate-900/40 p-4">
      <div class="w-full max-w-md rounded-2xl border border-slate-200 bg-white p-6 text-left shadow-xl">
        <div class="flex items-center justify-between border-b border-slate-100 pb-3">
          <div class="flex items-center gap-2">
            <KeyRound class="h-4 w-4 text-emerald-600" />
            <h2 class="text-base font-extrabold !text-slate-900">Đồng bộ từ Facebook</h2>
          </div>
          <button @click="showSyncModal = false" class="rounded-lg p-1 text-slate-400 hover:bg-slate-100 hover:text-slate-600 cursor-pointer">
            <X class="h-5 w-5" />
          </button>
        </div>

        <form @submit.prevent="handleSyncFromFb" class="mt-4 space-y-4">
          <div>
            <label class="block text-xs font-bold !text-slate-700">Facebook User Access Token</label>
            <input
              v-model="userTokenInput"
              type="password"
              required
              placeholder="EAA..."
              class="mt-1 w-full rounded-xl border border-slate-200 bg-slate-50 px-3.5 py-2 text-sm !text-slate-900 outline-none focus:border-emerald-600 focus:bg-white"
            />
          </div>
          <div class="flex justify-end gap-2">
            <button
              type="button"
              @click="showSyncModal = false"
              class="rounded-xl border border-slate-200 px-4 py-2 text-xs font-semibold text-slate-600 hover:bg-slate-50 cursor-pointer"
            >
              Hủy
            </button>
            <button
              type="submit"
              :disabled="syncing"
              class="inline-flex items-center gap-1.5 rounded-xl bg-emerald-600 px-4 py-2 text-xs font-bold text-white hover:bg-emerald-700 cursor-pointer"
            >
              <Loader2 v-if="syncing" class="h-4 w-4 animate-spin" />
              <span>{{ syncing ? 'Đang đồng bộ...' : 'Bắt đầu đồng bộ' }}</span>
            </button>
          </div>
        </form>
      </div>
    </div>
  </div>
</template>