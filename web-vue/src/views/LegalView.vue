<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { CheckCircle2, FileText, Search, ShieldCheck, Trash2 } from 'lucide-vue-next'
import { api } from '../lib/api'
import { APP_NAME, CONTACT_EMAIL, LEGAL_UPDATED, legalContent, type LegalDoc, type LegalLang } from '../lib/legalContent'

const route = useRoute()
const doc = computed<LegalDoc>(() => (route.meta.legalDoc as LegalDoc) || 'privacy')

function initialLang(): LegalLang {
  const q = String(route.query.lang || '').toLowerCase()
  if (q === 'en' || q === 'vi') return q
  try {
    const saved = localStorage.getItem('pp_legal_lang')
    if (saved === 'en' || saved === 'vi') return saved
  } catch {
    /* bỏ qua */
  }
  return typeof navigator !== 'undefined' && navigator.language?.toLowerCase().startsWith('vi') ? 'vi' : 'en'
}

const lang = ref<LegalLang>(initialLang())
watch(lang, (v) => {
  try {
    localStorage.setItem('pp_legal_lang', v)
  } catch {
    /* bỏ qua */
  }
})

const page = computed(() => legalContent[lang.value][doc.value])

const ui = computed(() =>
  lang.value === 'vi'
    ? {
        updated: 'Cập nhật lần cuối',
        privacy: 'Quyền riêng tư',
        terms: 'Điều khoản',
        deletion: 'Xoá dữ liệu',
        contact: 'Liên hệ',
        statusTitle: 'Kiểm tra trạng thái yêu cầu xoá dữ liệu',
        statusHint: 'Nhập mã xác nhận Facebook hiển thị sau khi bạn gửi yêu cầu xoá dữ liệu.',
        placeholder: 'Mã xác nhận',
        check: 'Kiểm tra',
        checking: 'Đang kiểm tra...',
        notFound: 'Không tìm thấy mã xác nhận này.',
        done: 'Đã xử lý',
        noData: 'Không có dữ liệu',
        received: 'Thời điểm nhận yêu cầu'
      }
    : {
        updated: 'Last updated',
        privacy: 'Privacy',
        terms: 'Terms',
        deletion: 'Data deletion',
        contact: 'Contact',
        statusTitle: 'Check your data deletion request',
        statusHint: 'Enter the confirmation code Facebook showed after you requested data deletion.',
        placeholder: 'Confirmation code',
        check: 'Check',
        checking: 'Checking...',
        notFound: 'Confirmation code not found.',
        done: 'Completed',
        noData: 'No data found',
        received: 'Request received'
      }
)

const tabs = computed(() => [
  { doc: 'privacy' as LegalDoc, path: '/privacy', label: ui.value.privacy, icon: ShieldCheck },
  { doc: 'terms' as LegalDoc, path: '/terms', label: ui.value.terms, icon: FileText },
  { doc: 'deletion' as LegalDoc, path: '/data-deletion', label: ui.value.deletion, icon: Trash2 }
])

// Tra cứu trạng thái xoá dữ liệu (Meta chuyển người dùng tới /data-deletion?code=...)
interface DeletionStatus {
  confirmation_code: string
  status: string
  summary: string
  created_at: string
}
const code = ref(String(route.query.code || '').trim())
const checking = ref(false)
const status = ref<DeletionStatus | null>(null)
const statusError = ref('')

async function checkStatus() {
  const c = code.value.trim().toUpperCase()
  if (!c) return
  checking.value = true
  status.value = null
  statusError.value = ''
  try {
    const { data } = await api.get<DeletionStatus>(`/facebook/data-deletion/${encodeURIComponent(c)}`)
    status.value = data
  } catch {
    statusError.value = ui.value.notFound
  } finally {
    checking.value = false
  }
}

function formatDate(iso: string) {
  try {
    return new Date(iso).toLocaleString(lang.value === 'vi' ? 'vi-VN' : 'en-US')
  } catch {
    return iso
  }
}

watch(
  () => [doc.value, lang.value] as const,
  () => {
    document.title = `${page.value.title} – ${APP_NAME}`
  },
  { immediate: true }
)

onMounted(() => {
  if (doc.value === 'deletion' && code.value) checkStatus()
})
</script>

<template>
  <div class="legal min-h-screen">
    <header class="legal-header">
      <div class="mx-auto flex max-w-3xl flex-wrap items-center justify-between gap-3 px-4 py-3">
        <router-link to="/" class="flex items-center gap-2 font-extrabold">
          <span class="legal-logo">P</span>
          <span>{{ APP_NAME }}</span>
        </router-link>
        <div class="legal-lang" role="group" aria-label="Language">
          <button type="button" :class="{ active: lang === 'vi' }" @click="lang = 'vi'">Tiếng Việt</button>
          <button type="button" :class="{ active: lang === 'en' }" @click="lang = 'en'">English</button>
        </div>
      </div>
      <nav class="mx-auto flex max-w-3xl gap-1 overflow-x-auto px-4 pb-2">
        <router-link
          v-for="t in tabs"
          :key="t.doc"
          :to="{ path: t.path, query: { lang } }"
          class="legal-tab"
          :class="{ active: t.doc === doc }"
        >
          <component :is="t.icon" class="h-4 w-4" />
          <span>{{ t.label }}</span>
        </router-link>
      </nav>
    </header>

    <main class="mx-auto max-w-3xl px-4 py-8">
      <h1 class="text-2xl font-extrabold sm:text-3xl">{{ page.title }}</h1>
      <p class="legal-muted mt-1 text-sm">{{ ui.updated }}: {{ LEGAL_UPDATED }}</p>

      <p v-for="(p, i) in page.intro" :key="'i' + i" class="mt-4 leading-relaxed">{{ p }}</p>

      <!-- Ô tra cứu trạng thái xoá dữ liệu -->
      <section v-if="doc === 'deletion'" class="legal-card mt-6">
        <h2 class="text-base font-bold">{{ ui.statusTitle }}</h2>
        <p class="legal-muted mt-1 text-sm">{{ ui.statusHint }}</p>
        <form class="mt-3 flex flex-col gap-2 sm:flex-row" @submit.prevent="checkStatus">
          <input v-model="code" :placeholder="ui.placeholder" class="legal-input flex-1" maxlength="40" />
          <button type="submit" class="legal-btn" :disabled="checking || !code.trim()">
            <Search class="h-4 w-4" />
            <span>{{ checking ? ui.checking : ui.check }}</span>
          </button>
        </form>
        <p v-if="statusError" class="legal-error mt-3 text-sm">{{ statusError }}</p>
        <div v-if="status" class="legal-ok mt-3 text-sm">
          <p class="flex items-center gap-1.5 font-bold">
            <CheckCircle2 class="h-4 w-4" />
            {{ status.status === 'completed' ? ui.done : ui.noData }} · {{ status.confirmation_code }}
          </p>
          <p class="mt-1">{{ status.summary }}</p>
          <p class="legal-muted mt-1">{{ ui.received }}: {{ formatDate(status.created_at) }}</p>
        </div>
      </section>

      <section v-for="(s, i) in page.sections" :key="'s' + i" class="mt-8">
        <h2 class="text-lg font-bold">{{ s.h }}</h2>
        <p v-for="(p, j) in s.p || []" :key="'p' + j" class="mt-2 leading-relaxed">{{ p }}</p>
        <ul v-if="s.list" class="mt-2 list-disc space-y-1.5 pl-5 leading-relaxed">
          <li v-for="(li, j) in s.list" :key="'l' + j">{{ li }}</li>
        </ul>
      </section>
    </main>

    <footer class="legal-footer">
      <div class="mx-auto flex max-w-3xl flex-wrap items-center justify-between gap-3 px-4 py-6 text-sm">
        <span>© {{ new Date().getFullYear() }} {{ APP_NAME }}</span>
        <span v-if="CONTACT_EMAIL">
          {{ ui.contact }}: <a :href="`mailto:${CONTACT_EMAIL}`" class="underline">{{ CONTACT_EMAIL }}</a>
        </span>
      </div>
    </footer>
  </div>
</template>

<style scoped>
.legal {
  --bg: #f8fafc;
  --surface: #ffffff;
  --text: #0f172a;
  --muted: #64748b;
  --border: #e2e8f0;
  --accent: #059669;
  --accent-soft: #ecfdf5;
  --danger: #be123c;
  background: var(--bg);
  color: var(--text);
}
@media (prefers-color-scheme: dark) {
  .legal {
    --bg: #0b1220;
    --surface: #111a2e;
    --text: #e2e8f0;
    --muted: #94a3b8;
    --border: #1f2a44;
    --accent: #34d399;
    --accent-soft: #0f2a24;
    --danger: #fb7185;
  }
}
.legal-header {
  position: sticky;
  top: 0;
  z-index: 10;
  background: var(--surface);
  border-bottom: 1px solid var(--border);
}
.legal-logo {
  display: inline-flex;
  height: 28px;
  width: 28px;
  align-items: center;
  justify-content: center;
  border-radius: 8px;
  background: var(--accent);
  color: #fff;
  font-size: 14px;
}
.legal-lang {
  display: inline-flex;
  border: 1px solid var(--border);
  border-radius: 999px;
  padding: 2px;
}
.legal-lang button {
  border-radius: 999px;
  padding: 4px 12px;
  font-size: 12px;
  font-weight: 600;
  color: var(--muted);
  cursor: pointer;
}
.legal-lang button.active {
  background: var(--accent);
  color: #fff;
}
.legal-tab {
  display: inline-flex;
  flex-shrink: 0;
  align-items: center;
  gap: 6px;
  border-radius: 10px;
  padding: 6px 12px;
  font-size: 14px;
  font-weight: 600;
  color: var(--muted);
}
.legal-tab.active {
  background: var(--accent-soft);
  color: var(--accent);
}
.legal-muted {
  color: var(--muted);
}
.legal-card {
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 16px;
  padding: 16px;
}
.legal-input {
  background: var(--bg);
  border: 1px solid var(--border);
  border-radius: 10px;
  padding: 8px 12px;
  color: var(--text);
  text-transform: uppercase;
  outline: none;
}
.legal-input:focus {
  border-color: var(--accent);
}
.legal-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  border-radius: 10px;
  background: var(--accent);
  color: #fff;
  padding: 8px 16px;
  font-weight: 700;
  cursor: pointer;
}
.legal-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
.legal-error {
  color: var(--danger);
}
.legal-ok {
  background: var(--accent-soft);
  border-radius: 10px;
  padding: 10px 12px;
}
.legal-ok p:first-child {
  color: var(--accent);
}
.legal-footer {
  border-top: 1px solid var(--border);
  color: var(--muted);
}
</style>
