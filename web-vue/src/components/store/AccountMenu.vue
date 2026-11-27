<script setup lang="ts">
// Menu thả xuống của khách đã đăng nhập (đầu trang): tên, Đơn hàng của tôi, Thông tin, Đăng xuất
import { LogOut, Package, User } from 'lucide-vue-next'

defineProps<{ name: string; phone?: string; accountPath: string }>()
const emit = defineEmits<{ close: []; logout: [] }>()
</script>

<template>
  <div class="fixed inset-0 z-30" @click.stop="emit('close')" />
  <div class="absolute right-0 top-full z-40 mt-2 w-56 overflow-hidden rounded-2xl border border-slate-200 bg-white py-1.5 text-left text-sm text-slate-700 shadow-xl">
    <div class="border-b border-slate-100 px-4 pb-2 pt-1">
      <div class="truncate font-bold text-slate-900">{{ name }}</div>
      <div v-if="phone" class="truncate text-xs text-slate-500">{{ phone }}</div>
    </div>
    <router-link :to="accountPath" class="flex items-center gap-2.5 px-4 py-2.5 hover:bg-slate-50" @click="emit('close')"><Package class="h-4 w-4 text-slate-400" /> Đơn hàng của tôi</router-link>
    <router-link :to="accountPath" class="flex items-center gap-2.5 px-4 py-2.5 hover:bg-slate-50" @click="emit('close')"><User class="h-4 w-4 text-slate-400" /> Thông tin tài khoản</router-link>
    <button type="button" class="flex w-full items-center gap-2.5 border-t border-slate-100 px-4 py-2.5 text-left text-rose-600 hover:bg-rose-50" @click="emit('logout')"><LogOut class="h-4 w-4" /> Đăng xuất</button>
  </div>
</template>
