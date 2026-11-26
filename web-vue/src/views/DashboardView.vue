<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import {
  ArrowUpRight,
  Bell,
  CalendarClock,
  CheckCircle2,
  Facebook,
  Layers,
  LayoutDashboard,
  LogOut,
  Menu,
  MessageSquare,
  RefreshCw,
  Send,
  ShieldCheck,
  Sparkles,
  TrendingUp,
  Users,
  X
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
const mobileMenuOpen = ref(false)

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
  conversations.value.filter((c) => c.unread_count > 0).slice(0, 5)
)

const navItems = computed(() => {
  const list = [
    { label: 'Tổng quan', path: '/', icon: LayoutDashboard, active: true, badge: 0 },
    { label: 'Hộp thư đa Page', path: '/inbox', icon: MessageSquare, active: false, badge: totalUnreadCount.value },
    { label: 'CSKH tự động', path: '/broadcasts', icon: Send, active: false, badge: 0 },
    { label: 'Phân tích đối thủ', path: '/competitors', icon: TrendingUp, active: false, badge: 0 },
    { label: 'Lên lịch đăng bài', path: '/posts', icon: CalendarClock, active: false, badge: 0 },
    { label: 'Gộp Fanpage', path: '/pages', icon: Layers, active: false, badge: 0 }
  ]
  if (canManageRoles.value) {
    list.push({ label: 'Phân cấp & Phân quyền', path: '/roles', icon: Users, active: false, badge: 0 })
  }
  return list
})

const featureModules = computed(() => {
  const items = [
    {
      title: 'Hộp thư chat gộp đa Page',
      tag: 'Messenger Live',
      path: '/inbox',
      stat: totalUnreadCount.value > 0 ? `${totalUnreadCount.value} tin chưa đọc` : 'Đã phản hồi hết',
      icon: MessageSquare,
      iconBg: 'bg-emerald-600 text-white',
      description: 'Tập trung toàn bộ tin nhắn Messenger từ nhiều Fanpage về một màn hình, hỗ trợ gửi ảnh và phân loại đơn.'
    },
    {
      title: 'Chăm sóc khách cũ tự động',
      tag: 'Conversion',
      path: '/broadcasts',
      stat: `${conversations.value.length} khách tiềm năng`,
      icon: Send,
      iconBg: 'bg-teal-600 text-white',
      description: 'Quét toàn bộ khách đã nhắn tin trên Meta Business và tự động nhắn lại theo kịch bản xoay vòng an toàn.'
    },
    {
      title: 'Phân tích đối thủ cạnh tranh',
      tag: 'Competitor',
      path: '/competitors',
      stat: 'Theo dõi nội dung',
      icon: TrendingUp,
      iconBg: 'bg-emerald-700 text-white',
      description: 'Giám sát tần suất đăng bài, nội dung tương tác cao và chiến lược truyền thông của các Fanpage đối thủ.'
    },
    {
      title: 'Lên lịch & Đăng bài đa Page',
      tag: 'Auto Post',
      path: '/posts',
      stat: 'Tự động hóa nội dung',
      icon: CalendarClock,
      iconBg: 'bg-teal-700 text-white',
      description: 'Soạn thảo bài viết, đính kèm hình ảnh và hẹn giờ xuất bản đồng loạt lên nhiều Fanpage cùng lúc.'
    },
    {
      title: 'Quản lý & Kết nối Fanpage',
      tag: 'Pages Sync',
      path: '/pages',
      stat: `${activePagesCount.value}/${pages.value.length} Page bật`,
      icon: Layers,
      iconBg: 'bg-emerald-600 text-white',
      description: 'Đồng bộ danh sách Fanpage từ Facebook, bật/tắt trạng thái kết nối và làm mới Page Access Token.'
    }
  ]

  if (canManageRoles.value) {
    items.push({
      title: 'Phân cấp & Phân quyền nhân sự',
      tag: 'Team & Roles',
      path: '/roles',
      stat: 'Quản lý truy cập',
      icon: Users,
      iconBg: 'bg-slate-800 text-white',
      description: 'Phân chia vai trò Quản lý và Nhân viên trực Page, kiểm soát quyền thao tác theo từng Fanpage.'
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

function goTo(path: string) {
  mobileMenuOpen.value = false
  router.push(path)
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
  <div class="min-h-screen bg-[#f4f7f6] text-slate-900 antialiased flex">
    <!-- Backdrop mobile -->
    <div
      v-if="mobileMenuOpen"
      @click="mobileMenuOpen = false"
      class="fixed inset-0 z-40 bg-slate-900/40 backdrop-blur-xs lg:hidden"
    ></div>

    <!-- Left Sidebar Navigation (Cố định bên trái, rõ ràng, không bao giờ cắt chữ) -->
    <aside
      class="fixed inset-y-0 left-0 z-50 flex w-64 flex-col border-r border-slate-200/80 bg-white transition-transform duration-200 lg:static lg:translate-x-0"
      :class="mobileMenuOpen ? 'translate-x-0' : '-translate-x-full'"
    >
      <!-- Brand Header -->
      <div class="flex h-16 items-center justify-between border-b border-slate-100 px-5">
        <button @click="goTo('/')" class="flex items-center gap-3 text-left cursor-pointer">
          <div class="flex h-9 w-9 items-center justify-center rounded-xl bg-emerald-600 text-white shadow-xs">
            <MessageSquare class="h-4.5 w-4.5" />
          </div>
          <div class="leading-none">
            <span class="block text-[15px] font-extrabold tracking-tight !text-slate-900">
              PagePilot
            </span>
            <span class="mt-1 block text-[10px] font-bold uppercase tracking-wider text-emerald-600">
              Multi-Page CRM
            </span>
          </div>
        </button>

        <button
          @click="mobileMenuOpen = false"
          class="rounded-lg p-1.5 text-slate-400 hover:bg-slate-100 lg:hidden"
        >
          <X class="h-5 w-5" />
        </button>
      </div>

      <!-- Navigation List -->
      <div class="flex-1 overflow-y-auto px-3.5 py-5 space-y-1">
        <p class="px-3 pb-2 text-[10px] font-bold uppercase tracking-wider text-slate-400">
          Điều hướng chức năng
        </p>

        <button
          v-for="item in navItems"
          :key="item.path"
          @click="goTo(item.path)"
          class="flex w-full items-center justify-between rounded-xl px-3.5 py-2.5 text-[13px] font-semibold transition-all cursor-pointer"
          :class="
            item.active
              ? 'bg-emerald-50 text-emerald-800 ring-1 ring-emerald-200/80'
              : 'text-slate-600 hover:bg-slate-50 hover:text-slate-900'
          "
        >
          <span class="flex items-center gap-3">
            <component
              :is="item.icon"
              class="h-4 w-4 shrink-0"
              :class="item.active ? 'text-emerald-600' : 'text-slate-400'"
            />
            <span>{{ item.label }}</span>
          </span>

          <span
            v-if="item.badge > 0"
            class="rounded-full bg-emerald-600 px-2 py-0.5 text-[10px] font-bold text-white"
          >
            {{ item.badge }}
          </span>
        </button>
      </div>

      <!-- Bottom User Card + Logout -->
      <div class="border-t border-slate-100 p-3.5 space-y-2.5">
        <div class="flex items-center gap-2.5 rounded-xl bg-slate-50 p-2.5 ring-1 ring-slate-200/60">
          <div class="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-emerald-600 text-xs font-bold text-white">
            {{ getInitials(userName) }}
          </div>
          <div class="min-w-0 flex-1 leading-tight">
            <div class="flex items-center gap-1.5">
              <span class="truncate text-xs font-bold !text-slate-900">{{ userName }}</span>
              <span class="shrink-0 rounded bg-emerald-100 px-1.5 py-0.5 text-[9px] font-extrabold text-emerald-800">
                {{ userRole }}
              </span>
            </div>
            <p v-if="userEmail" class="mt-0.5 truncate text-[11px] text-slate-400">{{ userEmail }}</p>
          </div>
        </div>

        <button
          @click="handleLogout"
          class="flex w-full items-center justify-center gap-2 rounded-xl border border-slate-200 bg-white px-3 py-2 text-xs font-semibold text-slate-600 transition hover:border-rose-200 hover:bg-rose-50 hover:text-rose-600 cursor-pointer"
        >
          <LogOut class="h-3.5 w-3.5" />
          <span>Đăng xuất tài khoản</span>
        </button>
      </div>
    </aside>

    <!-- Right Workspace Area -->
    <div class="flex min-w-0 flex-1 flex-col">
      <!-- Clean Topbar -->
      <header class="sticky top-0 z-20 flex h-16 items-center justify-between border-b border-slate-200/70 bg-white/90 px-4 backdrop-blur-md sm:px-8">
        <div class="flex items-center gap-3">
          <button
            @click="mobileMenuOpen = true"
            class="rounded-xl border border-slate-200 p-2 text-slate-600 hover:bg-slate-50 lg:hidden"
          >
            <Menu class="h-5 w-5" />
          </button>
          <div>
            <h2 class="text-sm font-bold !text-slate-900">Tổng quan hệ thống</h2>
            <p class="hidden sm:block text-[11px] text-slate-400">
              Theo dõi hiệu suất Fanpage, hội thoại & chiến dịch chăm sóc khách hàng
            </p>
          </div>
        </div>

        <div class="flex items-center gap-2.5">
          <span class="hidden md:inline-flex items-center gap-1.5 rounded-full bg-emerald-50 px-3 py-1 text-[11px] font-semibold text-emerald-700 ring-1 ring-emerald-200/70">
            <ShieldCheck class="h-3.5 w-3.5" />
            <span>Hệ thống ổn định</span>
          </span>

          <button
            @click="handleQuickSync"
            :disabled="syncingAll"
            class="inline-flex items-center gap-1.5 rounded-xl border border-slate-200 bg-white px-3.5 py-2 text-xs font-semibold text-slate-700 shadow-2xs transition hover:border-emerald-300 hover:bg-emerald-50 hover:text-emerald-800 disabled:opacity-60 cursor-pointer"
          >
            <RefreshCw class="h-3.5 w-3.5 text-emerald-600" :class="{ 'animate-spin': syncingAll }" />
            <span>{{ syncingAll ? 'Đang đồng bộ...' : 'Đồng bộ tin mới' }}</span>
          </button>
        </div>
      </header>

      <!-- Toast thông báo -->
      <div
        v-if="toastMessage"
        class="border-b border-emerald-200 bg-emerald-50 px-4 py-2 text-center text-xs font-semibold text-emerald-800"
      >
        {{ toastMessage }}
      </div>

      <!-- Main Dashboard Content -->
      <main class="mx-auto w-full max-w-6xl space-y-6 px-4 py-6 sm:px-8">
        <!-- Compact & Balanced Hero Banner -->
        <section class="relative overflow-hidden rounded-2xl bg-gradient-to-r from-emerald-800 via-emerald-700 to-teal-700 p-6 sm:p-7 text-white shadow-sm">
          <div class="pointer-events-none absolute -right-10 -top-10 h-48 w-48 rounded-full bg-white/10 blur-2xl"></div>

          <div class="relative z-10 flex flex-col justify-between gap-5 lg:flex-row lg:items-center">
            <div class="space-y-1.5 text-left">
              <div class="inline-flex items-center gap-1.5 rounded-full bg-white/15 px-2.5 py-0.5 text-[11px] font-semibold text-emerald-100">
                <Sparkles class="h-3 w-3" />
                <span>Không gian làm việc đã sẵn sàng</span>
              </div>
              <h1 class="text-xl sm:text-2xl font-extrabold tracking-tight !text-white">
                Xin chào, {{ userName }}!
              </h1>
              <p class="max-w-xl text-xs sm:text-[13px] text-emerald-50/90 leading-relaxed">
                Điều hành tập trung tin nhắn đa Fanpage, nhắn tin chăm sóc khách cũ tự động, theo dõi đối thủ và lên lịch đăng bài tại một nơi duy nhất.
              </p>
            </div>

            <div class="flex flex-wrap items-center gap-2.5 shrink-0">
              <button
                @click="goTo('/inbox')"
                class="inline-flex items-center gap-2 whitespace-nowrap rounded-xl bg-white px-4 py-2.5 text-xs font-bold text-emerald-800 shadow-xs transition hover:bg-emerald-50 cursor-pointer"
              >
                <MessageSquare class="h-4 w-4 text-emerald-600" />
                <span>Mở Hộp thư</span>
              </button>

              <button
                @click="goTo('/broadcasts')"
                class="inline-flex items-center gap-2 whitespace-nowrap rounded-xl bg-emerald-950/35 px-4 py-2.5 text-xs font-semibold text-white ring-1 ring-white/25 transition hover:bg-emerald-950/50 cursor-pointer"
              >
                <Send class="h-4 w-4 text-emerald-200" />
                <span>CSKH tự động</span>
              </button>
            </div>
          </div>
        </section>

        <!-- 4 KPI Cards (Căn trái chuẩn xác, sắc nét) -->
        <section class="grid grid-cols-2 gap-4 lg:grid-cols-4">
          <div class="rounded-2xl border border-slate-200/80 bg-white p-5 text-left shadow-2xs">
            <div class="flex items-center justify-between">
              <span class="text-xs font-semibold text-slate-500">Tổng Fanpage</span>
              <div class="flex h-8 w-8 items-center justify-center rounded-lg bg-emerald-50 text-emerald-600">
                <Layers class="h-4 w-4" />
              </div>
            </div>
            <p class="mt-3 text-2xl font-extrabold tracking-tight !text-slate-900">
              {{ loadingStats ? '...' : pages.length }}
            </p>
            <p class="mt-1 text-[11px] text-slate-400">Đã kết nối qua Graph API</p>
          </div>

          <div class="rounded-2xl border border-slate-200/80 bg-white p-5 text-left shadow-2xs">
            <div class="flex items-center justify-between">
              <span class="text-xs font-semibold text-slate-500">Page hoạt động</span>
              <div class="flex h-8 w-8 items-center justify-center rounded-lg bg-teal-50 text-teal-600">
                <CheckCircle2 class="h-4 w-4" />
              </div>
            </div>
            <p class="mt-3 text-2xl font-extrabold tracking-tight !text-emerald-700">
              {{ loadingStats ? '...' : activePagesCount }}
            </p>
            <p class="mt-1 text-[11px] text-slate-400">Sẵn sàng nhận & gửi tin</p>
          </div>

          <div class="rounded-2xl border border-slate-200/80 bg-white p-5 text-left shadow-2xs">
            <div class="flex items-center justify-between">
              <span class="text-xs font-semibold text-slate-500">Tổng hội thoại</span>
              <div class="flex h-8 w-8 items-center justify-center rounded-lg bg-emerald-50 text-emerald-600">
                <MessageSquare class="h-4 w-4" />
              </div>
            </div>
            <p class="mt-3 text-2xl font-extrabold tracking-tight !text-slate-900">
              {{ loadingStats ? '...' : conversations.length }}
            </p>
            <p class="mt-1 text-[11px] text-slate-400">Khách hàng trên hệ thống</p>
          </div>

          <div class="rounded-2xl border border-slate-200/80 bg-white p-5 text-left shadow-2xs">
            <div class="flex items-center justify-between">
              <span class="text-xs font-semibold text-slate-500">Tin chưa đọc</span>
              <div class="flex h-8 w-8 items-center justify-center rounded-lg bg-amber-50 text-amber-600">
                <Bell class="h-4 w-4" />
              </div>
            </div>
            <p class="mt-3 text-2xl font-extrabold tracking-tight" :class="totalUnreadCount > 0 ? '!text-amber-600' : '!text-slate-900'">
              {{ loadingStats ? '...' : totalUnreadCount }}
            </p>
            <p class="mt-1 text-[11px] text-slate-400">Cần nhân viên phản hồi</p>
          </div>
        </section>

        <!-- 6 Thẻ truy cập nhanh chức năng -->
        <section class="space-y-3">
          <div class="text-left">
            <h2 class="text-sm font-extrabold uppercase tracking-wider !text-slate-700">
              Truy cập nhanh chức năng
            </h2>
          </div>

          <div class="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
            <button
              v-for="mod in featureModules"
              :key="mod.path"
              @click="goTo(mod.path)"
              class="group flex flex-col justify-between rounded-2xl border border-slate-200/80 bg-white p-5 text-left shadow-2xs transition-all duration-200 hover:-translate-y-0.5 hover:border-emerald-300 hover:shadow-md cursor-pointer"
            >
              <div>
                <div class="flex items-center justify-between gap-2">
                  <div
                    class="flex h-10 w-10 items-center justify-center rounded-xl shadow-2xs"
                    :class="mod.iconBg"
                  >
                    <component :is="mod.icon" class="h-5 w-5" />
                  </div>
                  <span class="rounded-md bg-slate-100 px-2 py-0.5 text-[10px] font-bold uppercase tracking-wide text-slate-600">
                    {{ mod.tag }}
                  </span>
                </div>

                <h3 class="mt-4 text-[15px] font-bold !text-slate-900 group-hover:text-emerald-700 transition-colors">
                  {{ mod.title }}
                </h3>
                <p class="mt-1.5 text-xs leading-relaxed text-slate-500">
                  {{ mod.description }}
                </p>
              </div>

              <div class="mt-5 flex items-center justify-between border-t border-slate-100 pt-3">
                <span class="text-[11px] font-semibold text-slate-400">{{ mod.stat }}</span>
                <span class="inline-flex items-center gap-1 text-xs font-bold text-emerald-700">
                  <span>Mở tính năng</span>
                  <ArrowUpRight class="h-3.5 w-3.5 transition-transform group-hover:translate-x-0.5 group-hover:-translate-y-0.5" />
                </span>
              </div>
            </button>
          </div>
        </section>

        <!-- 2 Cột dữ liệu thực tế bên dưới -->
        <section class="grid grid-cols-1 gap-6 lg:grid-cols-12">
          <!-- Danh sách Fanpage -->
          <div class="rounded-2xl border border-slate-200/80 bg-white p-5 text-left shadow-2xs lg:col-span-7">
            <div class="flex items-center justify-between border-b border-slate-100 pb-3.5">
              <div>
                <h3 class="text-sm font-bold !text-slate-900">Fanpage đang kết nối</h3>
                <p class="text-xs text-slate-500">Các trang đang đồng bộ vào PagePilot CRM</p>
              </div>
              <button
                @click="goTo('/pages')"
                class="text-xs font-semibold text-emerald-700 hover:underline cursor-pointer"
              >
                Tất cả Page →
              </button>
            </div>

            <div v-if="pages.length === 0" class="py-10 text-center text-xs text-slate-500">
              Chưa có Fanpage nào được gộp. Bấm <strong>Gộp Fanpage</strong> để bắt đầu.
            </div>

            <div v-else class="mt-2 divide-y divide-slate-100">
              <div
                v-for="page in pages.slice(0, 5)"
                :key="page.id"
                class="flex items-center justify-between gap-3 py-3"
              >
                <div class="flex min-w-0 items-center gap-3">
                  <div class="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-emerald-50 text-emerald-600">
                    <Facebook class="h-4 w-4" />
                  </div>
                  <div class="min-w-0">
                    <p class="truncate text-xs font-bold !text-slate-900">{{ page.name }}</p>
                    <p class="text-[11px] text-slate-400">ID: {{ page.page_id }}</p>
                  </div>
                </div>

                <span
                  class="shrink-0 rounded-full px-2.5 py-0.5 text-[11px] font-semibold"
                  :class="
                    page.is_connected
                      ? 'bg-emerald-50 text-emerald-700 ring-1 ring-emerald-200'
                      : 'bg-slate-100 text-slate-500'
                  "
                >
                  {{ page.is_connected ? 'Hoạt động' : 'Tạm ngưng' }}
                </span>
              </div>
            </div>
          </div>

          <!-- Tin nhắn chưa đọc -->
          <div class="rounded-2xl border border-slate-200/80 bg-white p-5 text-left shadow-2xs lg:col-span-5">
            <div class="flex items-center justify-between border-b border-slate-100 pb-3.5">
              <div>
                <h3 class="text-sm font-bold !text-slate-900">Khách hàng chờ phản hồi</h3>
                <p class="text-xs text-slate-500">Hội thoại có tin nhắn chưa đọc</p>
              </div>
              <button
                @click="goTo('/inbox')"
                class="text-xs font-semibold text-emerald-700 hover:underline cursor-pointer"
              >
                Mở hộp thư →
              </button>
            </div>

            <div v-if="recentUnreadConversations.length === 0" class="py-10 text-center text-xs text-slate-500">
              Tuyệt vời! Hiện không có tin nhắn nào bị bỏ sót.
            </div>

            <div v-else class="mt-2 divide-y divide-slate-100">
              <button
                v-for="conv in recentUnreadConversations"
                :key="conv.id"
                @click="goTo('/inbox')"
                class="flex w-full items-center justify-between gap-3 rounded-xl px-2 py-3 text-left transition hover:bg-emerald-50/60 cursor-pointer"
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
                <ArrowUpRight class="h-4 w-4 shrink-0 text-slate-400" />
              </button>
            </div>
          </div>
        </section>
      </main>
    </div>
  </div>
</template>