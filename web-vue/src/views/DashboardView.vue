<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import {
  ArrowRight,
  Bell,
  CalendarClock,
  CheckCircle2,
  Facebook,
  Layers,
  LogOut,
  MessageSquare,
  RefreshCw,
  Send,
  ShieldCheck,
  Sparkles,
  TrendingUp,
  Users
} from 'lucide-vue-next'
import { api } from '../lib/api'
import { useAuthStore } from './../stores/auth.stores'
import type { FacebookPageItem } from '../types/page'

interface ConversationSummary {
  id: number
  page_id: string
  page_name: string
  customer_name: string
  last_message: string
  last_message_time: string
  unread_count: number
}

const router = useRouter()
const authStore = useAuthStore()

const pages = ref<FacebookPageItem[]>([])
const conversations = ref<ConversationSummary[]>([])
const loadingStats = ref(false)
const syncingAll = ref(false)
const toastMessage = ref('')

const currentUser = computed(() => (authStore.user || {}) as Record<string, any>)
const userName = computed(() => currentUser.value.full_name || currentUser.value.name || 'Quản trị viên')
const userEmail = computed(() => currentUser.value.email || '')
const userRole = computed(() => String(currentUser.value.role || 'admin').toUpperCase())

const canManageRoles = computed(() => {
  const role = String(currentUser.value.role || 'admin').toLowerCase()
  return role === 'admin' || role === 'manager'
})

const activePagesCount = computed(() => pages.value.filter((p) => p.is_connected).length)
const totalUnreadCount = computed(() =>
  conversations.value.reduce((sum, item) => sum + (item.unread_count || 0), 0)
)
const recentUnreadConversations = computed(() =>
  conversations.value.filter((c) => c.unread_count > 0).slice(0, 4)
)

const featureModules = computed(() => {
  const items = [
    {
      title: 'Hộp thư chat gộp đa Page',
      shortLabel: 'Hộp thư đa Page',
      path: '/inbox',
      badge: totalUnreadCount.value > 0 ? `${totalUnreadCount.value} mới` : 'Trực tiếp',
      badgeClass: 'bg-emerald-100 text-emerald-800',
      icon: MessageSquare,
      iconWrap: 'bg-emerald-600 text-white',
      description:
        'Xem và trả lời tin nhắn Messenger từ tất cả Fanpage trên một màn hình duy nhất, hỗ trợ gửi ảnh và phân loại khách hàng.',
      cta: 'Vào khung chat'
    },
    {
      title: 'Chăm sóc khách cũ tự động',
      shortLabel: 'CSKH tự động',
      path: '/broadcasts',
      badge: 'Conversion',
      badgeClass: 'bg-amber-100 text-amber-800',
      icon: Send,
      iconWrap: 'bg-amber-500 text-white',
      description:
        'Quét toàn bộ khách đã nhắn tin trên Meta Business và tự động gửi tin nhắn chăm sóc lại theo nhịp giãn cách an toàn.',
      cta: 'Tạo chiến dịch gửi'
    },
    {
      title: 'Phân tích đối thủ cạnh tranh',
      shortLabel: 'Đối thủ',
      path: '/competitors',
      badge: 'Competitor',
      badgeClass: 'bg-indigo-100 text-indigo-800',
      icon: TrendingUp,
      iconWrap: 'bg-indigo-600 text-white',
      description:
        'Theo dõi bài đăng, tần suất hoạt động và nội dung nổi bật của các Fanpage đối thủ để tối ưu chiến lược nội dung.',
      cta: 'Xem phân tích đối thủ'
    },
    {
      title: 'Lên lịch & Quản lý bài đăng',
      shortLabel: 'Đăng bài tự động',
      path: '/posts',
      badge: 'Auto Post',
      badgeClass: 'bg-sky-100 text-sky-800',
      icon: CalendarClock,
      iconWrap: 'bg-sky-600 text-white',
      description:
        'Soạn nội dung, đính kèm hình ảnh và hẹn giờ đăng bài tự động lên nhiều Fanpage cùng lúc.',
      cta: 'Quản lý lịch đăng'
    },
    {
      title: 'Quản lý & Gộp Fanpage',
      shortLabel: 'Gộp Fanpage',
      path: '/pages',
      badge: `${activePagesCount.value}/${pages.value.length} bật`,
      badgeClass: 'bg-teal-100 text-teal-800',
      icon: Layers,
      iconWrap: 'bg-teal-600 text-white',
      description:
        'Đồng bộ tự động danh sách Fanpage từ Facebook, bật/tắt trạng thái hoạt động từng Page và làm mới Page Access Token.',
      cta: 'Quản lý danh sách Page'
    }
  ]

  if (canManageRoles.value) {
    items.push({
      title: 'Phân cấp & Phân quyền nhân sự',
      shortLabel: 'Phân quyền',
      path: '/roles',
      badge: 'Admin / Manager',
      badgeClass: 'bg-slate-200 text-slate-800',
      icon: Users,
      iconWrap: 'bg-slate-800 text-white',
      description:
        'Chia vai trò Quản lý và Nhân viên trực Page, giới hạn quyền truy cập dữ liệu theo từng Fanpage được giao.',
      cta: 'Thiết lập đội ngũ'
    })
  }

  return items
})

function getInitials(name: string) {
  if (!name) return 'U'
  const parts = name.trim().split(/\s+/)
  if (parts.length === 1) return parts[0].charAt(0).toUpperCase()
  return (parts[0].charAt(0) + parts[parts.length - 1].charAt(0)).toUpperCase()
}

async function fetchOverviewData() {
  loadingStats.value = true
  try {
    const [pagesRes, inboxRes] = await Promise.allSettled([
      api.get<{ pages: FacebookPageItem[] }>('/pages'),
      api.get<{ conversations: ConversationSummary[] }>('/inbox/conversations')
    ])

    if (pagesRes.status === 'fulfilled') {
      pages.value = pagesRes.value.data.pages || []
    }
    if (inboxRes.status === 'fulfilled') {
      conversations.value = inboxRes.value.data.conversations || []
    }
  } finally {
    loadingStats.value = false
  }
}

async function handleQuickSync() {
  syncingAll.value = true
  toastMessage.value = ''
  try {
    const { data } = await api.post<{ message: string }>('/inbox/sync')
    toastMessage.value = data.message || 'Đã đồng bộ tin nhắn mới nhất!'
    await fetchOverviewData()
    setTimeout(() => {
      toastMessage.value = ''
    }, 3500)
  } catch {
    router.push('/inbox')
  } finally {
    syncingAll.value = false
  }
}

async function handleLogout() {
  try {
    await authStore.logout()
  } finally {
    router.push('/login')
  }
}

onMounted(fetchOverviewData)
</script>

<template>
  <div class="min-h-screen bg-[#f6f9f8] text-slate-900">
    <!-- Top Navigation Bar: Đầy đủ tất cả chức năng -->
    <header class="sticky top-0 z-20 border-b border-slate-200/80 bg-white/95 backdrop-blur-xs">
      <div class="mx-auto flex h-16 max-w-7xl items-center justify-between gap-3 px-4 sm:px-6 lg:px-8">
        <!-- Brand Logo -->
        <button
          @click="router.push('/')"
          class="flex items-center gap-2.5 shrink-0 text-left cursor-pointer"
        >
          <div class="flex h-10 w-10 items-center justify-center rounded-xl bg-gradient-to-br from-emerald-600 to-teal-600 text-white shadow-sm">
            <MessageSquare class="h-5 w-5" />
          </div>
          <div class="leading-tight">
            <span class="block text-base font-extrabold tracking-tight !text-slate-900">
              PagePilot CRM
            </span>
            <span class="block text-[11px] font-medium text-emerald-700">
              Quản lý & CSKH đa Fanpage
            </span>
          </div>
        </button>

        <!-- Center Navigation Pills (Đủ tất cả 6 chức năng) -->
        <nav class="hidden lg:flex items-center gap-1.5 overflow-x-auto py-1">
          <button
            @click="router.push('/inbox')"
            class="inline-flex items-center gap-1.5 whitespace-nowrap rounded-xl bg-emerald-600 px-3.5 py-2 text-xs font-semibold text-white shadow-xs transition hover:bg-emerald-700 cursor-pointer"
          >
            <MessageSquare class="h-3.5 w-3.5" />
            <span>Hộp thư</span>
            <span
              v-if="totalUnreadCount > 0"
              class="rounded-full bg-white px-1.5 py-0.2 text-[10px] font-bold text-emerald-700"
            >
              {{ totalUnreadCount }}
            </span>
          </button>

          <button
            @click="router.push('/broadcasts')"
            class="inline-flex items-center gap-1.5 whitespace-nowrap rounded-xl border border-amber-200 bg-amber-50/70 px-3 py-2 text-xs font-semibold text-amber-900 transition hover:border-amber-300 hover:bg-amber-100 cursor-pointer"
          >
            <Send class="h-3.5 w-3.5 text-amber-600" />
            <span>CSKH tự động</span>
          </button>

          <button
            @click="router.push('/competitors')"
            class="inline-flex items-center gap-1.5 whitespace-nowrap rounded-xl border border-slate-200 bg-white px-3 py-2 text-xs font-semibold text-slate-700 transition hover:border-indigo-300 hover:bg-indigo-50 hover:text-indigo-800 cursor-pointer"
          >
            <TrendingUp class="h-3.5 w-3.5 text-indigo-600" />
            <span>Đối thủ</span>
          </button>

          <button
            @click="router.push('/posts')"
            class="inline-flex items-center gap-1.5 whitespace-nowrap rounded-xl border border-slate-200 bg-white px-3 py-2 text-xs font-semibold text-slate-700 transition hover:border-sky-300 hover:bg-sky-50 hover:text-sky-800 cursor-pointer"
          >
            <CalendarClock class="h-3.5 w-3.5 text-sky-600" />
            <span>Đăng bài</span>
          </button>

          <button
            @click="router.push('/pages')"
            class="inline-flex items-center gap-1.5 whitespace-nowrap rounded-xl border border-slate-200 bg-white px-3 py-2 text-xs font-semibold text-slate-700 transition hover:border-emerald-300 hover:bg-emerald-50 hover:text-emerald-800 cursor-pointer"
          >
            <Layers class="h-3.5 w-3.5 text-emerald-600" />
            <span>Gộp Page</span>
          </button>

          <button
            v-if="canManageRoles"
            @click="router.push('/roles')"
            class="inline-flex items-center gap-1.5 whitespace-nowrap rounded-xl border border-slate-200 bg-white px-3 py-2 text-xs font-semibold text-slate-700 transition hover:border-emerald-300 hover:bg-emerald-50 hover:text-emerald-800 cursor-pointer"
          >
            <Users class="h-3.5 w-3.5 text-slate-600" />
            <span>Phân quyền</span>
          </button>
        </nav>

        <!-- Right User Profile & Logout -->
        <div class="flex items-center gap-2.5 shrink-0">
          <div class="hidden sm:flex items-center gap-2 rounded-2xl border border-slate-100 bg-slate-50/80 px-2.5 py-1.5">
            <div class="flex h-8 w-8 items-center justify-center rounded-full bg-emerald-100 text-xs font-bold text-emerald-800">
              {{ getInitials(userName) }}
            </div>
            <div class="text-left leading-tight">
              <div class="flex items-center gap-1.5">
                <span class="text-xs font-bold !text-slate-900">{{ userName }}</span>
                <span class="inline-flex items-center gap-0.5 rounded-md bg-emerald-100 px-1.5 py-0.5 text-[10px] font-bold text-emerald-800">
                  <ShieldCheck class="h-3 w-3" />
                  {{ userRole }}
                </span>
              </div>
              <p v-if="userEmail" class="text-[10px] text-slate-500 max-w-[140px] truncate">{{ userEmail }}</p>
            </div>
          </div>

          <button
            @click="handleLogout"
            class="inline-flex items-center gap-1.5 whitespace-nowrap rounded-xl border border-slate-200 bg-white px-3 py-2 text-xs font-semibold text-slate-700 transition hover:border-red-200 hover:bg-red-50 hover:text-red-600 cursor-pointer"
          >
            <LogOut class="h-3.5 w-3.5" />
            <span class="hidden sm:inline">Đăng xuất</span>
          </button>
        </div>
      </div>

      <!-- Mobile/Tablet Sub-Nav Bar (Hiển thị đủ nút trên màn hình nhỏ) -->
      <div class="flex lg:hidden items-center gap-2 overflow-x-auto border-t border-slate-100 bg-slate-50/70 px-4 py-2">
        <button
          v-for="mod in featureModules"
          :key="mod.path"
          @click="router.push(mod.path)"
          class="inline-flex items-center gap-1.5 whitespace-nowrap rounded-lg border border-slate-200 bg-white px-3 py-1.5 text-xs font-semibold text-slate-700 shadow-2xs"
        >
          <component :is="mod.icon" class="h-3.5 w-3.5 text-emerald-600" />
          <span>{{ mod.shortLabel }}</span>
        </button>
      </div>
    </header>

    <!-- Toast Thông báo đồng bộ -->
    <div
      v-if="toastMessage"
      class="border-b border-emerald-200 bg-emerald-50 px-4 py-2.5 text-center text-xs font-semibold text-emerald-800"
    >
      {{ toastMessage }}
    </div>

    <!-- Main Content -->
    <main class="mx-auto max-w-7xl space-y-6 px-4 py-7 sm:px-6 lg:px-8">
      <!-- Hero Welcome Banner -->
      <section class="relative overflow-hidden rounded-3xl border border-emerald-200/70 bg-gradient-to-r from-emerald-700 via-emerald-600 to-teal-600 p-6 sm:p-8 text-white shadow-sm">
        <div class="relative z-10 flex flex-col justify-between gap-6 lg:flex-row lg:items-center">
          <div class="max-w-2xl space-y-2">
            <div class="inline-flex items-center gap-1.5 rounded-full bg-white/15 px-3 py-1 text-xs font-semibold text-emerald-50 backdrop-blur-xs">
              <Sparkles class="h-3.5 w-3.5" />
              <span>Không gian làm việc đã sẵn sàng</span>
            </div>
            <h1 class="text-2xl sm:text-3xl font-extrabold tracking-tight !text-white">
              Xin chào, {{ userName }}!
            </h1>
            <p class="text-sm text-emerald-50/90 leading-relaxed">
              Trung tâm điều hành đa Fanpage: trực tin nhắn gộp, gửi chăm sóc khách cũ tự động, theo dõi đối thủ và lên lịch đăng bài trên một giao diện duy nhất.
            </p>
          </div>

          <div class="flex flex-wrap items-center gap-3 shrink-0">
            <button
              @click="router.push('/inbox')"
              class="inline-flex items-center gap-2 whitespace-nowrap rounded-2xl bg-white px-4 py-3 text-sm font-bold text-emerald-800 shadow-sm transition hover:bg-emerald-50 cursor-pointer"
            >
              <MessageSquare class="h-4 w-4 text-emerald-600" />
              <span>Mở Hộp thư</span>
            </button>

            <button
              @click="router.push('/broadcasts')"
              class="inline-flex items-center gap-2 whitespace-nowrap rounded-2xl bg-amber-400 px-4 py-3 text-sm font-bold text-slate-950 shadow-sm transition hover:bg-amber-300 cursor-pointer"
            >
              <Send class="h-4 w-4" />
              <span>Nhắn tin tự động</span>
            </button>

            <button
              @click="handleQuickSync"
              :disabled="syncingAll"
              class="inline-flex items-center gap-2 whitespace-nowrap rounded-2xl border border-white/30 bg-white/10 px-4 py-3 text-sm font-semibold text-white backdrop-blur-xs transition hover:bg-white/20 disabled:opacity-60 cursor-pointer"
            >
              <RefreshCw class="h-4 w-4" :class="{ 'animate-spin': syncingAll }" />
              <span>{{ syncingAll ? 'Đang kéo tin...' : 'Kéo tin mới' }}</span>
            </button>
          </div>
        </div>
      </section>

      <!-- 4 Thẻ thống kê tổng quan -->
      <section class="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <div class="rounded-2xl border border-slate-200/80 bg-white p-5 shadow-2xs">
          <div class="flex items-center justify-between">
            <span class="text-xs font-semibold text-slate-500">Tổng Fanpage đã gộp</span>
            <div class="flex h-9 w-9 items-center justify-center rounded-xl bg-emerald-50 text-emerald-600">
              <Layers class="h-4 w-4" />
            </div>
          </div>
          <p class="mt-3 text-2xl font-extrabold !text-slate-900">
            {{ loadingStats ? '...' : pages.length }}
          </p>
          <p class="mt-1 text-xs text-slate-500">Kết nối đồng bộ qua Graph API</p>
        </div>

        <div class="rounded-2xl border border-slate-200/80 bg-white p-5 shadow-2xs">
          <div class="flex items-center justify-between">
            <span class="text-xs font-semibold text-slate-500">Page đang hoạt động</span>
            <div class="flex h-9 w-9 items-center justify-center rounded-xl bg-teal-50 text-teal-600">
              <CheckCircle2 class="h-4 w-4" />
            </div>
          </div>
          <p class="mt-3 text-2xl font-extrabold !text-emerald-700">
            {{ loadingStats ? '...' : activePagesCount }}
          </p>
          <p class="mt-1 text-xs text-slate-500">Sẵn sàng nhận & gửi tin nhắn</p>
        </div>

        <div class="rounded-2xl border border-slate-200/80 bg-white p-5 shadow-2xs">
          <div class="flex items-center justify-between">
            <span class="text-xs font-semibold text-slate-500">Tổng hội thoại gộp</span>
            <div class="flex h-9 w-9 items-center justify-center rounded-xl bg-blue-50 text-blue-600">
              <MessageSquare class="h-4 w-4" />
            </div>
          </div>
          <p class="mt-3 text-2xl font-extrabold !text-slate-900">
            {{ loadingStats ? '...' : conversations.length }}
          </p>
          <p class="mt-1 text-xs text-slate-500">Khách hàng trên toàn hệ thống</p>
        </div>

        <div class="rounded-2xl border border-slate-200/80 bg-white p-5 shadow-2xs">
          <div class="flex items-center justify-between">
            <span class="text-xs font-semibold text-slate-500">Tin nhắn chưa đọc</span>
            <div class="flex h-9 w-9 items-center justify-center rounded-xl bg-amber-50 text-amber-600">
              <Bell class="h-4 w-4" />
            </div>
          </div>
          <p class="mt-3 text-2xl font-extrabold" :class="totalUnreadCount > 0 ? '!text-amber-600' : '!text-slate-900'">
            {{ loadingStats ? '...' : totalUnreadCount }}
          </p>
          <p class="mt-1 text-xs text-slate-500">Cần nhân viên phản hồi sớm</p>
        </div>
      </section>

      <!-- Lưới điều hướng tất cả chức năng hệ thống -->
      <section class="space-y-3">
        <div class="flex items-center justify-between">
          <div>
            <h2 class="text-base font-extrabold !text-slate-900">Tất cả chức năng hệ thống</h2>
            <p class="text-xs text-slate-500">Chọn nhanh công cụ bạn muốn làm việc</p>
          </div>
        </div>

        <div class="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
          <button
            v-for="mod in featureModules"
            :key="mod.path"
            @click="router.push(mod.path)"
            class="group flex flex-col justify-between rounded-2xl border border-slate-200/80 bg-white p-5 text-left shadow-2xs transition hover:-translate-y-0.5 hover:border-emerald-300 hover:shadow-md cursor-pointer"
          >
            <div>
              <div class="flex items-center justify-between gap-2">
                <div
                  class="flex h-11 w-11 items-center justify-center rounded-2xl shadow-2xs"
                  :class="mod.iconWrap"
                >
                  <component :is="mod.icon" class="h-5 w-5" />
                </div>
                <span
                  class="rounded-full px-2.5 py-0.5 text-[11px] font-bold"
                  :class="mod.badgeClass"
                >
                  {{ mod.badge }}
                </span>
              </div>

              <h3 class="mt-4 text-base font-bold !text-slate-900 group-hover:text-emerald-700">
                {{ mod.title }}
              </h3>
              <p class="mt-1.5 text-xs leading-relaxed text-slate-500">
                {{ mod.description }}
              </p>
            </div>

            <div class="mt-5 inline-flex items-center gap-1.5 text-xs font-bold text-emerald-700">
              <span>{{ mod.cta }}</span>
              <ArrowRight class="h-3.5 w-3.5 transition-transform group-hover:translate-x-1" />
            </div>
          </button>
        </div>
      </section>

      <!-- Danh sách Fanpage đang kết nối & Tin nhắn chưa đọc mới nhất -->
      <section class="grid grid-cols-1 gap-6 lg:grid-cols-12">
        <!-- Cột trái: Fanpage đang kết nối -->
        <div class="rounded-2xl border border-slate-200/80 bg-white p-5 shadow-2xs lg:col-span-7">
          <div class="flex items-center justify-between border-b border-slate-100 pb-3.5">
            <div>
              <h3 class="text-sm font-bold !text-slate-900">Fanpage đang kết nối</h3>
              <p class="text-xs text-slate-500">Danh sách các trang đang đồng bộ vào PagePilot CRM</p>
            </div>
            <button
              @click="router.push('/pages')"
              class="text-xs font-semibold text-emerald-700 hover:underline cursor-pointer"
            >
              Xem tất cả →
            </button>
          </div>

          <div v-if="pages.length === 0" class="py-10 text-center text-xs text-slate-500">
            Chưa có Fanpage nào được gộp. Bấm <strong>Gộp Fanpage</strong> để bắt đầu.
          </div>

          <div v-else class="mt-3 divide-y divide-slate-100">
            <div
              v-for="page in pages.slice(0, 5)"
              :key="page.id"
              class="flex items-center justify-between gap-3 py-3"
            >
              <div class="flex min-w-0 items-center gap-3">
                <div class="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-blue-50 text-blue-600">
                  <Facebook class="h-4 w-4" />
                </div>
                <div class="min-w-0">
                  <p class="truncate text-xs font-bold !text-slate-900">{{ page.name }}</p>
                  <p class="text-[11px] text-slate-400">ID: {{ page.page_id }}</p>
                </div>
              </div>

              <span
                class="shrink-0 rounded-full px-2.5 py-1 text-[11px] font-semibold"
                :class="
                  page.is_connected
                    ? 'bg-emerald-50 text-emerald-700 border border-emerald-200'
                    : 'bg-slate-100 text-slate-500'
                "
              >
                {{ page.is_connected ? 'Đang hoạt động' : 'Tạm ngưng' }}
              </span>
            </div>
          </div>
        </div>

        <!-- Cột phải: Tin nhắn chưa đọc mới nhất -->
        <div class="rounded-2xl border border-slate-200/80 bg-white p-5 shadow-2xs lg:col-span-5">
          <div class="flex items-center justify-between border-b border-slate-100 pb-3.5">
            <div>
              <h3 class="text-sm font-bold !text-slate-900">Khách hàng chờ phản hồi</h3>
              <p class="text-xs text-slate-500">Các hội thoại có tin nhắn chưa đọc</p>
            </div>
            <button
              @click="router.push('/inbox')"
              class="text-xs font-semibold text-emerald-700 hover:underline cursor-pointer"
            >
              Mở hộp thư →
            </button>
          </div>

          <div v-if="recentUnreadConversations.length === 0" class="py-10 text-center text-xs text-slate-500">
            Tuyệt vời! Hiện không có tin nhắn nào bị bỏ sót.
          </div>

          <div v-else class="mt-3 divide-y divide-slate-100">
            <button
              v-for="conv in recentUnreadConversations"
              :key="conv.id"
              @click="router.push('/inbox')"
              class="flex w-full items-center justify-between gap-3 py-3 text-left transition hover:bg-emerald-50/50 rounded-xl px-2 cursor-pointer"
            >
              <div class="min-w-0 flex-1">
                <div class="flex items-center gap-2">
                  <p class="truncate text-xs font-bold !text-slate-900">{{ conv.customer_name }}</p>
                  <span class="rounded-full bg-emerald-600 px-1.5 py-0.2 text-[10px] font-bold text-white">
                    {{ conv.unread_count }}
                  </span>
                </div>
                <p class="mt-0.5 truncate text-xs text-slate-600">{{ conv.last_message }}</p>
                <p class="mt-0.5 truncate text-[10px] font-medium text-emerald-700">{{ conv.page_name }}</p>
              </div>
              <ArrowRight class="h-4 w-4 shrink-0 text-slate-400" />
            </button>
          </div>
        </div>
      </section>
    </main>
  </div>
</template>