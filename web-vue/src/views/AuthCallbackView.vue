<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Loader2 } from 'lucide-vue-next'
import { useAuthStore } from './../stores/auth.stores'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const message = ref('Đang xác thực tài khoản...')

onMounted(async () => {
  const err = route.query.error
  if (typeof err === 'string' && err) {
    router.replace({ name: 'login', query: { error: err } })
    return
  }

  const token = route.query.token
  if (typeof token !== 'string' || !token) {
    router.replace({
      name: 'login',
      query: { error: 'Không nhận được mã xác thực từ máy chủ.' }
    })
    return
  }

  const ok = await auth.handleOAuthCallback(token)
  if (ok) {
    router.replace({ name: 'dashboard' })
  } else {
    message.value = 'Phiên đăng nhập không hợp lệ, đang chuyển hướng...'
    router.replace({
      name: 'login',
      query: { error: 'Không thể tải thông tin tài khoản. Vui lòng thử lại.' }
    })
  }
})
</script>

<template>
  <div class="flex min-h-screen flex-col items-center justify-center gap-3 bg-slate-50 text-slate-700">
    <Loader2 class="h-8 w-8 animate-spin text-blue-600" />
    <p class="text-sm font-medium">{{ message }}</p>
  </div>
</template>