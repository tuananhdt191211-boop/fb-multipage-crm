<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Eye } from 'lucide-vue-next'
import AppShell from './components/AppShell.vue'
import { useAuthStore } from './stores/auth.stores'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()

// Admin đang "xem như khách": thoát về trang quản lý khách hàng
async function exitViewAs() {
  auth.stopViewAs()
  await auth.fetchMe()
  router.push('/admin/customers')
}

// Trang quản trị dùng thanh menu chung; trang đăng nhập, web công khai,
// Tổng quan (có menu riêng) và trình chỉnh sửa web (toàn màn hình) thì không.
const useShell = computed(() => Boolean(route.meta.requiresAuth) && !route.meta.noShell)
</script>

<template>
  <!-- Banner chế độ xem như khách hàng (chỉ đọc) -->
  <div v-if="auth.viewAsId && auth.isAuthenticated" data-testid="view-as-banner" class="sticky top-0 z-[300] flex flex-wrap items-center justify-center gap-x-3 gap-y-1 bg-amber-500 px-4 py-1.5 text-xs font-bold text-white shadow">
    <Eye class="h-3.5 w-3.5" />
    <span>Đang xem như khách hàng <b data-testid="view-as-name">{{ auth.viewAsName }}</b> — chỉ xem, không thể thay đổi dữ liệu.</span>
    <button type="button" data-testid="view-as-exit" class="rounded-full bg-white/25 px-3 py-0.5 hover:bg-white/40" @click="exitViewAs">Thoát chế độ xem</button>
  </div>
  <AppShell v-if="useShell">
    <router-view />
  </AppShell>
  <router-view v-else />
</template>
