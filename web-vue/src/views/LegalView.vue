<script setup lang="ts">
// Trang pháp lý công khai của ứng dụng (không cần đăng nhập): /privacy, /terms, /data-deletion.
// Song ngữ Việt / Anh: ?lang=vi|en, mặc định theo ngôn ngữ trình duyệt (người duyệt của Meta thường dùng tiếng Anh).
import { computed, watchEffect } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { MessageSquare } from 'lucide-vue-next'
import { APP_NAME, LEGAL_UPDATED, contact, legalDoc, legalPaths, legalTitles, type LegalKey, type LegalLang } from '../lib/legalContent'

const route = useRoute()
const router = useRouter()

const key = computed<LegalKey>(() => (route.meta.legal as LegalKey) || 'privacy')
const lang = computed<LegalLang>(() => {
  const q = String(route.query.lang || '').toLowerCase()
  if (q === 'vi' || q === 'en') return q
  return typeof navigator !== 'undefined' && /^vi/i.test(navigator.language || '') ? 'vi' : 'en'
})
const doc = computed(() => legalDoc(key.value, lang.value))
const keys: LegalKey[] = ['privacy', 'terms', 'deletion']

const ui = computed(() =>
  lang.value === 'vi'
    ? { updated: 'Cập nhật lần cuối', home: 'Về trang chủ', docs: 'Tài liệu pháp lý', rights: 'Bảo lưu mọi quyền.' }
    : { updated: 'Last updated', home: 'Back to home', docs: 'Legal documents', rights: 'All rights reserved.' }
)

function setLang(l: LegalLang) {
  router.replace({ query: { ...route.query, lang: l } })
}
const toPath = (k: LegalKey) => ({ path: legalPaths[k], query: { lang: lang.value } })

watchEffect(() => {
  document.title = `${doc.value.title} – ${APP_NAME}`
  document.documentElement.lang = lang.value
})
</script>

<template>
  <!-- data-i18n="skip": trang này tự quản lý ngôn ngữ, không để bộ dịch giao diện quản trị can thiệp -->
  <div data-i18n="skip" class="min-h-screen bg-slate-50 text-slate-800">
    <header class="border-b border-slate-200 bg-white">
      <div class="mx-auto flex max-w-3xl items-center justify-between gap-3 px-5 py-3">
        <router-link to="/" class="flex items-center gap-2 font-extrabold tracking-tight text-slate-900" data-testid="legal-home">
          <span class="flex h-8 w-8 items-center justify-center rounded-xl bg-emerald-600 text-white"><MessageSquare class="h-4 w-4" /></span>
          {{ APP_NAME }}
        </router-link>
        <div class="flex items-center gap-1 rounded-lg bg-slate-100 p-1 text-xs font-bold" role="group" aria-label="Language">
          <button
            v-for="l in (['vi', 'en'] as const)"
            :key="l"
            type="button"
            :data-testid="'legal-lang-' + l"
            class="rounded-md px-2.5 py-1"
            :class="lang === l ? 'bg-white text-emerald-700 shadow-sm' : 'text-slate-500 hover:text-slate-800'"
            @click="setLang(l)"
          >
            {{ l === 'vi' ? 'Tiếng Việt' : 'English' }}
          </button>
        </div>
      </div>
    </header>

    <main class="mx-auto max-w-3xl px-5 py-8 sm:py-12">
      <nav class="mb-6 flex flex-wrap gap-2 text-sm" :aria-label="ui.docs">
        <router-link
          v-for="k in keys"
          :key="k"
          :to="toPath(k)"
          :data-testid="'legal-tab-' + k"
          class="rounded-full border px-3.5 py-1.5 font-semibold"
          :class="k === key ? 'border-emerald-600 bg-emerald-600 text-white' : 'border-slate-200 bg-white text-slate-600 hover:border-emerald-300 hover:text-emerald-700'"
        >
          {{ legalTitles[k][lang] }}
        </router-link>
      </nav>

      <article class="rounded-3xl border border-slate-200 bg-white p-6 shadow-sm sm:p-10" data-testid="legal-article">
        <h1 class="text-2xl font-extrabold tracking-tight text-slate-900 sm:text-3xl">{{ doc.title }}</h1>
        <p class="mt-1 text-xs font-semibold text-slate-400">{{ ui.updated }}: {{ LEGAL_UPDATED[lang] }}</p>
        <p class="mt-5 leading-relaxed text-slate-600">{{ doc.summary }}</p>

        <section v-for="s in doc.sections" :key="s.h" class="mt-8">
          <h2 class="text-lg font-bold text-slate-900">{{ s.h }}</h2>
          <p v-for="(t, i) in s.p" :key="'p' + i" class="mt-2 leading-relaxed text-slate-600">{{ t }}</p>
          <ul v-if="s.ul" class="mt-2 list-disc space-y-1.5 pl-5 leading-relaxed text-slate-600">
            <li v-for="(t, i) in s.ul" :key="'u' + i">{{ t }}</li>
          </ul>
          <p v-for="(t, i) in s.after" :key="'a' + i" class="mt-2 leading-relaxed text-slate-600">{{ t }}</p>
        </section>

        <p v-if="contact.email" class="mt-8 border-t border-slate-100 pt-5 text-sm text-slate-500">
          <a :href="'mailto:' + contact.email" class="font-semibold text-emerald-700 hover:underline" data-testid="legal-email">{{ contact.email }}</a>
        </p>
      </article>

      <footer class="mt-6 flex flex-wrap items-center justify-between gap-2 text-xs text-slate-400">
        <span>© {{ new Date().getFullYear() }} {{ contact.operator || APP_NAME }}. {{ ui.rights }}</span>
        <router-link to="/" class="font-semibold text-slate-500 hover:text-emerald-700">{{ ui.home }}</router-link>
      </footer>
    </main>
  </div>
</template>
