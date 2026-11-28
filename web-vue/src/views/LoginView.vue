<script setup lang="ts">
import { ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  AlertCircle,
  ArrowRight,
  Lock,
  Mail,
  MessageSquare,
  Sparkles
} from 'lucide-vue-next'
import { useAuthStore } from '../stores/auth.stores'
import UiLangSwitcher from '../components/UiLangSwitcher.vue'

const router = useRouter()
const route = useRoute()
const auth = useAuthStore()

const email = ref('')
const password = ref('')

const DEFAULT_BACKEND_ORIGIN = 'https://fb-crm-backend.onrender.com'

function resolveBackendOrigin(): string {
  const rawEnv = String(
    import.meta.env.VITE_API_BASE_URL ||
      import.meta.env.VITE_BACKEND_URL ||
      DEFAULT_BACKEND_ORIGIN
  ).trim()

  const urlMatch = rawEnv.match(/https?:\/\/[^\s\]\)]+/i)
  const candidate = (urlMatch ? urlMatch[0] : rawEnv)
    .replace(/^[\[\(]+\vert{}[\]\)]+$/g, '')
    .replace(/\/+$/, '')
    .replace(/\/api\/v1$/i, '')

  if (!/^https?:\/\//i.test(candidate)) {
    return DEFAULT_BACKEND_ORIGIN
  }
  return candidate
}

const BACKEND_ORIGIN = resolveBackendOrigin()

async function handleLogin() {
  if (!email.value.trim() || !password.value) return
  const ok = await auth.login({
    email: email.value.trim(),
    password: password.value
  })
  if (ok) {
    const redirect =
      typeof route.query.redirect === 'string' && route.query.redirect.startsWith('/')
        ? route.query.redirect
        : '/'
    router.replace(redirect)
  }
}

function loginWithProvider(provider: 'google' | 'facebook') {
  window.location.href = `${BACKEND_ORIGIN}/api/v1/auth/${provider}`
}
</script>

<template>
  <div class="relative min-h-screen flex items-center justify-center bg-slate-50 px-4 py-10 text-slate-900">
    <div class="absolute right-4 top-4 z-10"><UiLangSwitcher variant="compact" /></div>
    <div class="w-full max-w-md space-y-6 rounded-3xl border border-slate-200/90 bg-white p-8 shadow-sm">
      <!-- Brand Header -->
      <div class="flex flex-col items-center text-center">
        <div class="flex h-12 w-12 items-center justify-center rounded-2xl bg-emerald-600 text-white shadow-sm">
          <MessageSquare class="h-6 w-6" />
        </div>
        <h1 class="mt-4 text-xl font-bold tracking-tight !text-slate-900">
          Đăng nhập PagePilot CRM
        </h1>
        <p class="mt-1 text-xs text-slate-500">
          Quản lý hộp thư gộp đa Fanpage & trợ lý AI tư vấn khách hàng
        </p>
      </div>

      <!-- Thông báo lỗi -->
      <div
        v-if="auth.error"
        class="flex items-start gap-2 rounded-2xl border border-red-200 bg-red-50 px-3.5 py-2.5 text-xs font-medium text-red-700"
      >
        <AlertCircle class="mt-0.5 h-4 w-4 shrink-0 text-red-600" />
        <span>{{ auth.error }}</span>
      </div>

      <!-- Form Email / Password -->
      <form @submit.prevent="handleLogin" class="space-y-4">
        <div>
          <label class="mb-1.5 block text-xs font-semibold text-slate-700">Email</label>
          <div class="relative">
            <Mail class="pointer-events-none absolute left-3.5 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400" />
            <input
              v-model="email"
              type="email"
              required
              autocomplete="email"
              placeholder="admin@pagepilot.vn"
              class="w-full rounded-xl border border-slate-200 bg-slate-50/60 py-2.5 pl-10 pr-3.5 text-sm text-slate-900 outline-none transition focus:border-emerald-600 focus:bg-white focus:ring-2 focus:ring-emerald-100"
            />
          </div>
        </div>

        <div>
          <label class="mb-1.5 block text-xs font-semibold text-slate-700">Mật khẩu</label>
          <div class="relative">
            <Lock class="pointer-events-none absolute left-3.5 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400" />
            <input
              v-model="password"
              type="password"
              required
              autocomplete="current-password"
              placeholder="••••••••"
              class="w-full rounded-xl border border-slate-200 bg-slate-50/60 py-2.5 pl-10 pr-3.5 text-sm text-slate-900 outline-none transition focus:border-emerald-600 focus:bg-white focus:ring-2 focus:ring-emerald-100"
            />
          </div>
        </div>

        <button
          type="submit"
          :disabled="auth.loading"
          class="flex w-full items-center justify-center gap-2 rounded-xl bg-emerald-600 px-4 py-2.5 text-sm font-semibold text-white shadow-xs transition hover:bg-emerald-700 disabled:opacity-60 cursor-pointer"
        >
          <span>{{ auth.loading ? 'Đang đăng nhập...' : 'Đăng nhập hệ thống' }}</span>
          <ArrowRight v-if="!auth.loading" class="h-4 w-4" />
        </button>
      </form>

      <div class="relative flex items-center py-1">
        <div class="flex-grow border-t border-slate-200" />
        <span class="mx-3 shrink text-[11px] font-medium uppercase tracking-wider text-slate-400">
          Hoặc tiếp tục với
        </span>
        <div class="flex-grow border-t border-slate-200" />
      </div>

      <!-- Social OAuth Buttons -->
      <div class="grid grid-cols-2 gap-3">
        <button
          type="button"
          @click="loginWithProvider('google')"
          class="flex items-center justify-center gap-2 rounded-xl border border-slate-200 bg-white px-3.5 py-2.5 text-xs font-semibold text-slate-700 transition hover:bg-slate-50 cursor-pointer"
        >
          <Sparkles class="h-4 w-4 text-amber-500" />
          <span>Google</span>
        </button>

        <button
          type="button"
          @click="loginWithProvider('facebook')"
          class="flex items-center justify-center gap-2 rounded-xl border border-blue-200 bg-blue-50/70 px-3.5 py-2.5 text-xs font-semibold text-blue-700 transition hover:bg-blue-100 cursor-pointer"
        >
          <span class="font-bold text-blue-600">f</span>
          <span>Facebook (Via)</span>
        </button>
      </div>

      <p class="text-center text-xs text-slate-500">
        Chưa có tài khoản?
        <router-link to="/register" class="font-semibold text-emerald-700 hover:underline">
          Đăng ký ngay
        </router-link>
      </p>
      <p class="text-center text-[11px] text-slate-400">
        <router-link to="/privacy" class="hover:underline">Chính sách quyền riêng tư</router-link>
        ·
        <router-link to="/terms" class="hover:underline">Điều khoản</router-link>
        ·
        <router-link to="/data-deletion" class="hover:underline">Xoá dữ liệu</router-link>
      </p>
    </div>
  </div>
</template>