<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import AppShell from './components/AppShell.vue'

const route = useRoute()

// Trang quản trị dùng thanh menu chung; trang đăng nhập, web công khai,
// Tổng quan (có menu riêng) và trình chỉnh sửa web (toàn màn hình) thì không.
const useShell = computed(() => Boolean(route.meta.requiresAuth) && !route.meta.noShell)
</script>

<template>
  <AppShell v-if="useShell">
    <router-view />
  </AppShell>
  <router-view v-else />
</template>
