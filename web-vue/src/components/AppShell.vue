<script setup lang="ts">
// Khung chung cho các trang quản trị: thanh menu bên trái (máy tính), thu gọn được.
// Trên điện thoại mỗi trang tự có thanh trên cùng riêng nên khung này không hiện.
import { computed, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  CalendarClock,
  ChevronsLeft,
  ChevronsRight,
  Globe,
  Layers,
  LayoutDashboard,
  LogOut,
  MessageSquare,
  Send,
  TrendingUp,
  Users
} from 'lucide-vue-next'
import { useAuthStore } from '../stores/auth.stores'
import UiLangSwitcher from './UiLangSwitcher.vue'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()

const collapsed = ref(false)
try {
  collapsed.value = localStorage.getItem('pp_nav_collapsed') === '1'
} catch {
  /* bỏ qua */
}
watch(collapsed, (v) => {
  try {
    localStorage.setItem('pp_nav_collapsed', v ? '1' : '0')
  } catch {
    /* bỏ qua */
  }
})

const user = computed(() => (auth.user || {}) as Record<string, any>)
const userName = computed(() => user.value.full_name || user.value.name || 'Quản trị viên')
const role = computed(() => String(user.value.role || '').toLowerCase())

const items = computed(() => {
  const list = [
    { label: 'Tổng quan', path: '/', icon: LayoutDashboard },
    { label: 'Hộp thư đa Page', path: '/inbox', icon: MessageSquare },
    { label: 'CSKH tự động', path: '/broadcasts', icon: Send },
    { label: 'Lên lịch đăng bài', path: '/posts', icon: CalendarClock },
    { label: 'Phân tích đối thủ', path: '/competitors', icon: TrendingUp },
    { label: 'Website bán hàng', path: '/sites', icon: Globe },
    { label: 'Fanpage', path: '/pages', icon: Layers }
  ]
  if (role.value === 'admin' || role.value === 'manager') list.push({ label: 'Phân quyền', path: '/roles', icon: Users })
  return list
})

function isActive(path: string) {
  return path === '/' ? route.path === '/' : route.path === path || route.path.startsWith(path + '/')
}

function initials(name: string) {
  const parts = name.trim().split(/\s+/)
  return ((parts[0]?.[0] || 'U') + (parts.length > 1 ? parts[parts.length - 1]![0] : '')).toUpperCase()
}

async function logout() {
  try {
    await auth.logout()
  } finally {
    router.push('/login')
  }
}
</script>

<template>
  <div class="flex min-h-dvh">
    <aside
      class="sticky top-0 hidden h-dvh shrink-0 flex-col border-r border-slate-200 bg-white transition-[width] duration-200 lg:flex"
      :class="collapsed ? 'w-[68px]' : 'w-60'"
    >
      <!-- Thương hiệu -->
      <router-link to="/" class="flex h-14 shrink-0 items-center gap-2.5 border-b border-slate-100 px-4">
        <span class="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-emerald-600 text-white">
          <MessageSquare class="h-4 w-4" />
        </span>
        <span v-if="!collapsed" class="leading-none">
          <span class="block text-sm font-extrabold tracking-tight text-slate-900">PagePilot</span>
          <span class="mt-0.5 block text-[10px] font-bold uppercase tracking-wider text-emerald-600">Multi-Page CRM</span>
        </span>
      </router-link>

      <!-- Menu -->
      <nav class="flex-1 space-y-0.5 overflow-y-auto p-2.5">
        <router-link
          v-for="it in items"
          :key="it.path"
          :to="it.path"
          :title="collapsed ? it.label : undefined"
          class="group flex items-center gap-3 rounded-lg px-3 py-2 text-[13px] font-semibold transition-colors"
          :class="[
            isActive(it.path) ? 'bg-emerald-50 text-emerald-800' : 'text-slate-600 hover:bg-slate-50 hover:text-slate-900',
            collapsed ? 'justify-center px-0' : ''
          ]"
        >
          <component :is="it.icon" class="h-[18px] w-[18px] shrink-0" :class="isActive(it.path) ? 'text-emerald-600' : 'text-slate-400 group-hover:text-slate-600'" />
          <span v-if="!collapsed" class="truncate">{{ it.label }}</span>
        </router-link>
      </nav>

      <!-- Tài khoản -->
      <div class="shrink-0 space-y-1 border-t border-slate-100 p-2.5">
        <div class="flex items-center gap-2.5 rounded-lg px-2 py-1.5" :class="collapsed ? 'justify-center px-0' : ''" :title="collapsed ? userName : undefined">
          <span class="flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-emerald-600 text-[11px] font-bold text-white">{{ initials(userName) }}</span>
          <span v-if="!collapsed" class="min-w-0 flex-1">
            <span class="block truncate text-xs font-bold text-slate-900">{{ userName }}</span>
            <span class="block truncate text-[11px] uppercase text-slate-400">{{ role || 'admin' }}</span>
          </span>
        </div>
        <UiLangSwitcher variant="sidebar" :collapsed="collapsed" />
        <button
          class="flex w-full items-center gap-3 rounded-lg px-3 py-2 text-[13px] font-semibold text-slate-500 hover:bg-rose-50 hover:text-rose-600"
          :class="collapsed ? 'justify-center px-0' : ''"
          :title="collapsed ? 'Đăng xuất' : undefined"
          @click="logout"
        >
          <LogOut class="h-[18px] w-[18px] shrink-0" />
          <span v-if="!collapsed">Đăng xuất</span>
        </button>
        <button
          class="flex w-full items-center gap-3 rounded-lg px-3 py-2 text-[13px] font-semibold text-slate-400 hover:bg-slate-50 hover:text-slate-700"
          :class="collapsed ? 'justify-center px-0' : ''"
          :title="collapsed ? 'Mở rộng menu' : 'Thu gọn menu'"
          @click="collapsed = !collapsed"
        >
          <component :is="collapsed ? ChevronsRight : ChevronsLeft" class="h-[18px] w-[18px] shrink-0" />
          <span v-if="!collapsed">Thu gọn</span>
        </button>
      </div>
    </aside>

    <div class="min-w-0 flex-1">
      <slot />
    </div>
  </div>
</template>
