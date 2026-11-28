<script setup lang="ts">
// Dòng liên kết 3 trang pháp lý công khai (hiện ở trang đăng nhập / đăng ký)
</script>

<template>
  <p class="flex flex-wrap items-center justify-center gap-x-3 gap-y-1 text-[11px] text-slate-400" data-testid="legal-links">
    <router-link to="/privacy" class="hover:text-slate-600 hover:underline">Chính sách quyền riêng tư</router-link>
    <span aria-hidden="true">·</span>
    <router-link to="/terms" class="hover:text-slate-600 hover:underline">Điều khoản dịch vụ</router-link>
    <span aria-hidden="true">·</span>
    <router-link to="/data-deletion" class="hover:text-slate-600 hover:underline">Hướng dẫn xoá dữ liệu</router-link>
  </p>
</template>
