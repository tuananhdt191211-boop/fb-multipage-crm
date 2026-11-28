<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { AlertCircle, Eye, EyeOff, Loader2, Lock, Mail, MessageSquare, User } from 'lucide-vue-next'
import { useAuthStore } from './../stores/auth.stores'
import SocialAuthButtons from '@/components/SocialAuthButtons.vue'
import UiLangSwitcher from '@/components/UiLangSwitcher.vue'
import LegalLinks from '@/components/LegalLinks.vue'

const router = useRouter()
const auth = useAuthStore()

const showPassword = ref(false)
const localError = ref('')
const form = reactive({
  full_name: '',
  email: '',
  password: '',
  confirm_password: ''
})

onMounted(() => {
  auth.error = null
})

async function handleSubmit() {
  localError.value = ''
  if (form.password.length < 6) {
    localError.value = 'Mật khẩu phải có ít nhất 6 ký tự.'
    return
  }
  if (form.password !== form.confirm_password) {
    localError.value = 'Mật khẩu nhập lại không khớp.'
    return
  }

  const ok = await auth.register({
    full_name: form.full_name.trim(),
    email: form.email.trim(),
    password: form.password
  })
  if (ok) {
    router.push('/')
  }
}
</script>

<template>
  <div class="relative flex min-h-screen items-center justify-center bg-slate-50 p-6">
    <div class="absolute right-4 top-4 z-10"><UiLangSwitcher variant="compact" /></div>
    <div class="w-full max-w-md space-y-6 rounded-2xl bg-white p-8 shadow-sm border border-slate-200">
      <div class="space-y-1.5 text-center">
        <div class="mx-auto mb-3 flex h-11 w-11 items-center justify-center rounded-xl bg-blue-600 text-white">
          <MessageSquare class="h-5 w-5" />
        </div>
        <h2 class="text-2xl font-bold tracking-tight text-slate-900">Tạo tài khoản mới</h2>
        <p class="text-sm text-slate-500">Tài khoản đăng ký đầu tiên sẽ tự động có quyền Admin.</p>
      </div>

      <div
        v-if="localError || auth.error"
        class="flex items-start gap-2.5 rounded-xl border border-red-200 bg-red-50 p-3.5 text-sm text-red-700"
      >
        <AlertCircle class="h-5 w-5 shrink-0 mt-0.5" />
        <span>{{ localError || auth.error }}</span>
      </div>

      <SocialAuthButtons />

      <div class="relative flex items-center">
        <div class="flex-grow border-t border-slate-200"></div>
        <span class="mx-3 shrink-0 text-xs font-medium uppercase text-slate-400">hoặc đăng ký bằng email</span>
        <div class="flex-grow border-t border-slate-200"></div>
      </div>

      <form @submit.prevent="handleSubmit" class="space-y-4">
        <div class="space-y-1.5">
          <label for="full_name" class="block text-sm font-medium text-slate-700">Họ và tên</label>
          <div class="relative">
            <User class="pointer-events-none absolute left-3.5 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400" />
            <input
              id="full_name"
              v-model="form.full_name"
              type="text"
              required
              placeholder="Nguyễn Văn A"
              class="w-full rounded-xl border border-slate-300 py-2.5 pl-10 pr-3.5 text-sm outline-none transition focus:border-blue-600 focus:ring-2 focus:ring-blue-100"
            />
          </div>
        </div>

        <div class="space-y-1.5">
          <label for="reg_email" class="block text-sm font-medium text-slate-700">Email</label>
          <div class="relative">
            <Mail class="pointer-events-none absolute left-3.5 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400" />
            <input
              id="reg_email"
              v-model="form.email"
              type="email"
              required
              placeholder="tenban@congty.com"
              class="w-full rounded-xl border border-slate-300 py-2.5 pl-10 pr-3.5 text-sm outline-none transition focus:border-blue-600 focus:ring-2 focus:ring-blue-100"
            />
          </div>
        </div>

        <div class="space-y-1.5">
          <label for="reg_password" class="block text-sm font-medium text-slate-700">Mật khẩu</label>
          <div class="relative">
            <Lock class="pointer-events-none absolute left-3.5 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400" />
            <input
              id="reg_password"
              v-model="form.password"
              :type="showPassword ? 'text' : 'password'"
              required
              minlength="6"
              placeholder="Tối thiểu 6 ký tự"
              class="w-full rounded-xl border border-slate-300 py-2.5 pl-10 pr-10 text-sm outline-none transition focus:border-blue-600 focus:ring-2 focus:ring-blue-100"
            />
            <button
              type="button"
              @click="showPassword = !showPassword"
              class="absolute right-3 top-1/2 -translate-y-1/2 text-slate-400 hover:text-slate-600 cursor-pointer"
              tabindex="-1"
            >
              <EyeOff v-if="showPassword" class="h-4 w-4" />
              <Eye v-else class="h-4 w-4" />
            </button>
          </div>
        </div>

        <div class="space-y-1.5">
          <label for="confirm_password" class="block text-sm font-medium text-slate-700">Nhập lại mật khẩu</label>
          <div class="relative">
            <Lock class="pointer-events-none absolute left-3.5 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400" />
            <input
              id="confirm_password"
              v-model="form.confirm_password"
              :type="showPassword ? 'text' : 'password'"
              required
              minlength="6"
              placeholder="Nhập lại mật khẩu"
              class="w-full rounded-xl border border-slate-300 py-2.5 pl-10 pr-3.5 text-sm outline-none transition focus:border-blue-600 focus:ring-2 focus:ring-blue-100"
            />
          </div>
        </div>

        <button
          type="submit"
          :disabled="auth.loading"
          class="flex w-full items-center justify-center gap-2 rounded-xl bg-blue-600 px-4 py-2.5 text-sm font-semibold text-white shadow-xs transition hover:bg-blue-700 disabled:opacity-60 cursor-pointer"
        >
          <Loader2 v-if="auth.loading" class="h-4 w-4 animate-spin" />
          <span>{{ auth.loading ? 'Đang tạo tài khoản...' : 'Đăng ký tài khoản' }}</span>
        </button>
      </form>

      <p class="text-center text-sm text-slate-600">
        Đã có tài khoản?
        <router-link to="/login" class="font-semibold text-blue-600 hover:text-blue-700">
          Đăng nhập
        </router-link>
      </p>
      <LegalLinks />
    </div>
  </div>
</template>