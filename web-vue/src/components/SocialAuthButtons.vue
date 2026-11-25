<script setup lang="ts">
const rawBase =
  import.meta.env.VITE_API_BASE_URL ||
  import.meta.env.VITE_BACKEND_URL ||
  'https://fb-crm-backend.onrender.com'

// Loại bỏ dấu '/' thừa ở cuối và bỏ hậu tố '/api/v1' nếu đã có để tránh lặp '/api/v1/api/v1'
const backendOrigin = rawBase
  .trim()
  .replace(/\/+$/, '')
  .replace(/\/api\/v1$/, '')

function loginWithProvider(provider: 'google' | 'facebook') {
  window.location.href = `${backendOrigin}/api/v1/auth/${provider}`
}
</script>

<template>
  <div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
    <button
      type="button"
      @click="loginWithProvider('google')"
      class="flex w-full items-center justify-center gap-2.5 rounded-xl border border-slate-200 bg-white px-4 py-2.5 text-sm font-medium text-slate-700 shadow-xs transition hover:bg-slate-50 active:scale-[0.99] cursor-pointer"
    >
      <svg class="h-5 w-5" viewBox="0 0 24 24" aria-hidden="true">
        <path
          fill="#4285F4"
          d="M23.49 12.27c0-.79-.07-1.54-.2-2.27H12v4.51h6.47a5.54 5.54 0 0 1-2.4 3.63v3h3.88c2.27-2.09 3.54-5.17 3.54-8.87z"
        />
        <path
          fill="#34A853"
          d="M12 24c3.24 0 5.95-1.07 7.94-2.91l-3.88-3c-1.07.72-2.45 1.15-4.06 1.15-3.12 0-5.77-2.11-6.72-4.96H1.27v3.1A12 12 0 0 0 12 24z"
        />
        <path
          fill="#FBBC05"
          d="M5.28 14.28A7.2 7.2 0 0 1 4.9 12c0-.79.14-1.56.38-2.28v-3.1H1.27A12 12 0 0 0 0 12c0 1.94.46 3.77 1.27 5.38l4.01-3.1z"
        />
        <path
          fill="#EA4335"
          d="M12 4.76c1.76 0 3.34.61 4.58 1.8l3.44-3.44C17.95 1.19 15.24 0 12 0A12 12 0 0 0 1.27 6.62l4.01 3.1c.95-2.85 3.6-4.96 6.72-4.96z"
        />
      </svg>
      <span>Google</span>
    </button>

    <button
      type="button"
      @click="loginWithProvider('facebook')"
      class="flex w-full items-center justify-center gap-2.5 rounded-xl bg-[#1877F2] px-4 py-2.5 text-sm font-medium text-white shadow-xs transition hover:bg-[#166FE5] active:scale-[0.99] cursor-pointer"
    >
      <svg class="h-5 w-5 fill-current" viewBox="0 0 24 24" aria-hidden="true">
        <path
          d="M24 12.073C24 5.405 18.627 0 12 0S0 5.405 0 12.073C0 18.1 4.388 23.094 10.125 24v-8.437H7.078v-3.49h3.047V9.41c0-3.025 1.792-4.697 4.533-4.697 1.312 0 2.686.236 2.686.236v2.971H15.83c-1.491 0-1.956.931-1.956 1.886v2.267h3.328l-.532 3.49h-2.796V24C19.612 23.094 24 18.1 24 12.073z"
        />
      </svg>
      <span>Facebook</span>
    </button>
  </div>
</template>