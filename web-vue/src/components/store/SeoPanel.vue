<script setup lang="ts">
// SEO + landing page của TRANG ĐANG SỬA, và SEO mặc định của cả website.
//  - Xem trước kết quả Google, đếm ký tự, ảnh chia sẻ, noindex
//  - AI viết tiêu đề + mô tả từ nội dung trang (từng trang hoặc cả website)
//  - Chấm điểm SEO theo luật (không cần AI)
//  - Landing page: ẩn đầu trang / chân trang
import { computed, ref } from 'vue'
import { AlertTriangle, CheckCircle2, Loader2, Sparkles, XCircle } from 'lucide-vue-next'
import { api } from '../../lib/api'
import ImageField from './ImageField.vue'
import { auditPage, DESC_MAX, effectiveTitle, TITLE_MAX, type PageSettings, type SiteSeo } from '../../lib/seo'
import { pageSubPath, type SitePage } from '../../lib/pages'
import type { StoreBlock } from '../../types/store'

const props = defineProps<{
  siteId: number | string
  siteTitle: string
  siteSlug: string
  hasLogo: boolean
  page: SitePage
  settings: PageSettings
  siteSeo: SiteSeo
  blocks: StoreBlock[] // nội dung riêng của trang đang sửa (không gồm đầu / chân trang)
  pages: SitePage[]
  allSettings: Record<number, PageSettings>
  getText: (id: number) => string // văn bản của trang bất kỳ (AI viết hàng loạt)
  pageText: string // văn bản của trang đang sửa (kể cả phần chưa lưu)
}>()

const isHome = computed(() => props.page.kind === 'home')
const title = computed(() => effectiveTitle(props.settings, props.page.title, props.siteTitle, isHome.value))
const desc = computed(() => props.settings.seo_description.trim() || props.siteSeo.description.trim())
const urlShown = computed(() => `${window.location.host} › s › ${props.siteSlug}${pageSubPath(props.page).replace(/\//g, ' › ')}`)

const audit = computed(() =>
  auditPage({
    settings: props.settings,
    siteSeo: props.siteSeo,
    pageTitle: props.page.title,
    siteTitle: props.siteTitle,
    isHome: isHome.value,
    blocks: props.blocks,
    hasLogo: props.hasLogo,
    otherTitles: props.pages.filter((p) => p.id !== props.page.id).map((p) => effectiveTitle(props.allSettings[p.id] || props.settings, p.title, props.siteTitle, p.kind === 'home'))
  })
)
const scoreColor = computed(() => (audit.value.score >= 80 ? 'text-emerald-600' : audit.value.score >= 50 ? 'text-amber-600' : 'text-rose-600'))

// ---------------------------------------------------------------- AI
const keywords = ref('')
const aiBusy = ref(false)
const aiError = ref('')
const aiResult = ref<{ title: string; description: string; keywords: string[] } | null>(null)

function apiError(e: any, fb: string) {
  return e?.response?.data?.error || fb
}

async function callAI(body: { title: string; content: string; is_home: boolean; keywords?: string }) {
  const { data } = await api.post<{ title: string; description: string; keywords: string[] }>(`/storefront/sites/${props.siteId}/seo/suggest`, body, { timeout: 120000 })
  return data
}

async function suggest() {
  if (aiBusy.value) return
  aiBusy.value = true
  aiError.value = ''
  aiResult.value = null
  try {
    aiResult.value = await callAI({ title: props.page.title, content: props.pageText, is_home: isHome.value, keywords: keywords.value })
  } catch (e) {
    aiError.value = apiError(e, 'AI chưa trả lời được. Hãy thử lại.')
  } finally {
    aiBusy.value = false
  }
}

function useResult(which: 'title' | 'description' | 'both') {
  const r = aiResult.value
  if (!r) return
  if (which !== 'description') props.settings.seo_title = r.title
  if (which !== 'title') props.settings.seo_description = r.description
}

// Viết SEO cho mọi trang còn thiếu tiêu đề hoặc mô tả (không ghi đè phần đã có)
const bulkBusy = ref(false)
const bulkProgress = ref('') // "2/5 · Tên trang"
const bulkDone = ref(0)
const bulkSkipped = ref(0)
const missingPages = computed(() => props.pages.filter((p) => !(props.allSettings[p.id]?.seo_title || '').trim() || !(props.allSettings[p.id]?.seo_description || '').trim()))

async function bulk() {
  if (bulkBusy.value) return
  bulkBusy.value = true
  bulkProgress.value = ''
  bulkDone.value = 0
  bulkSkipped.value = 0
  aiError.value = ''
  const list = [...missingPages.value]
  try {
    for (const [i, p] of list.entries()) {
      const text = p.id === props.page.id ? props.pageText : props.getText(p.id)
      if (!text.trim()) {
        bulkSkipped.value++
        continue
      }
      bulkProgress.value = `${i + 1}/${list.length} · ${p.title}`
      const r = await callAI({ title: p.title, content: text, is_home: p.kind === 'home' })
      const st = props.allSettings[p.id]
      if (!st) continue
      if (!st.seo_title.trim() && r.title) st.seo_title = r.title
      if (!st.seo_description.trim() && r.description) st.seo_description = r.description
      bulkDone.value++
    }
  } catch (e) {
    aiError.value = apiError(e, 'AI chưa trả lời được. Hãy thử lại.')
  } finally {
    bulkProgress.value = ''
    bulkBusy.value = false
  }
}

const inp = 'w-full rounded-lg border border-slate-200 bg-white px-2.5 py-1.5 text-sm focus:border-indigo-500 focus:outline-none'
const lbl = 'block text-xs font-bold text-slate-600'
const counter = (n: number, max: number) => (n === 0 ? 'text-slate-400' : n > max ? 'text-amber-600' : 'text-emerald-600')
</script>

<template>
  <div class="space-y-5" data-testid="seo-panel">
    <!-- Xem trước kết quả Google -->
    <div class="space-y-1 rounded-xl border border-slate-200 bg-white p-3" data-testid="serp">
      <div class="truncate text-[11px] text-emerald-700">{{ urlShown }}</div>
      <div class="line-clamp-2 text-[15px] font-medium leading-snug text-blue-700" data-testid="serp-title">{{ title }}</div>
      <div class="line-clamp-3 text-xs leading-relaxed text-slate-600" data-testid="serp-desc">{{ desc || 'Chưa có mô tả — Google sẽ tự chọn một đoạn chữ trong trang.' }}</div>
    </div>

    <!-- Điểm SEO -->
    <div class="space-y-2">
      <div class="flex items-center justify-between">
        <span :class="lbl">Điểm SEO của trang</span>
        <span class="text-lg font-extrabold" :class="scoreColor" data-testid="seo-score">{{ audit.score }}/100</span>
      </div>
      <ul class="space-y-1">
        <li v-for="c in audit.checks" :key="c.id" class="flex items-start gap-1.5 text-[11px] leading-snug" :class="c.level === 'ok' ? 'text-slate-500' : 'text-slate-700'">
          <CheckCircle2 v-if="c.level === 'ok'" class="mt-px h-3.5 w-3.5 shrink-0 text-emerald-500" />
          <AlertTriangle v-else-if="c.level === 'warn'" class="mt-px h-3.5 w-3.5 shrink-0 text-amber-500" />
          <XCircle v-else class="mt-px h-3.5 w-3.5 shrink-0 text-rose-500" />
          <span>{{ c.text }}</span>
        </li>
      </ul>
    </div>

    <!-- Tiêu đề / mô tả -->
    <div class="space-y-3">
      <label class="block space-y-1">
        <span class="flex items-center justify-between"><span :class="lbl">Tiêu đề SEO</span><span class="text-[11px] font-semibold" :class="counter([...settings.seo_title].length, TITLE_MAX)">{{ [...settings.seo_title].length }}/{{ TITLE_MAX }}</span></span>
        <input v-model="settings.seo_title" data-testid="seo-title" maxlength="120" :placeholder="title" :class="inp" />
      </label>
      <label class="block space-y-1">
        <span class="flex items-center justify-between"><span :class="lbl">Mô tả SEO</span><span class="text-[11px] font-semibold" :class="counter([...settings.seo_description].length, DESC_MAX)">{{ [...settings.seo_description].length }}/{{ DESC_MAX }}</span></span>
        <textarea v-model="settings.seo_description" data-testid="seo-desc" rows="3" maxlength="320" placeholder="1–2 câu tóm tắt trang, có từ khoá chính, kết thúc bằng lời mời hành động" :class="inp" />
      </label>
      <ImageField v-model="settings.seo_image" :site-id="siteId" label="Ảnh chia sẻ (khi gửi link qua Facebook, Zalo)" compact />
      <label class="flex items-start gap-2 text-xs font-bold text-slate-600">
        <input v-model="settings.noindex" data-testid="seo-noindex" type="checkbox" class="mt-0.5" />
        <span>Ẩn trang này khỏi Google (noindex)<br /><span class="font-normal text-slate-400">Cũng không đưa vào sitemap. Dùng cho trang cảm ơn, trang nội bộ.</span></span>
      </label>
    </div>

    <!-- AI viết SEO -->
    <div class="space-y-2 rounded-xl border border-indigo-100 bg-indigo-50/60 p-3">
      <div class="flex items-center gap-1.5 text-xs font-extrabold text-indigo-800"><Sparkles class="h-4 w-4" /> AI viết SEO</div>
      <input v-model="keywords" data-testid="seo-keywords" maxlength="200" placeholder="Từ khoá chính muốn nhắm tới (không bắt buộc)" :class="inp" />
      <button type="button" data-testid="seo-ai" class="inline-flex w-full items-center justify-center gap-1.5 rounded-lg bg-indigo-600 px-3 py-2 text-xs font-bold text-white hover:bg-indigo-700 disabled:opacity-50" :disabled="aiBusy || bulkBusy" @click="suggest">
        <Loader2 v-if="aiBusy" class="h-3.5 w-3.5 animate-spin" /> Viết tiêu đề + mô tả cho trang này
      </button>
      <div v-if="aiResult" class="space-y-2 rounded-lg bg-white p-2.5 text-xs" data-testid="seo-ai-result">
        <div><span class="font-bold text-slate-500">Tiêu đề:</span> <span data-testid="seo-ai-title">{{ aiResult.title }}</span></div>
        <div><span class="font-bold text-slate-500">Mô tả:</span> <span data-testid="seo-ai-desc">{{ aiResult.description }}</span></div>
        <div v-if="aiResult.keywords.length" class="text-slate-400">Từ khoá gợi ý: {{ aiResult.keywords.join(', ') }}</div>
        <div class="flex flex-wrap gap-1.5">
          <button type="button" data-testid="seo-ai-use" class="rounded-md bg-indigo-600 px-2.5 py-1 font-bold text-white hover:bg-indigo-700" @click="useResult('both')">Dùng cả hai</button>
          <button type="button" class="rounded-md border border-slate-200 px-2.5 py-1 font-bold text-slate-600 hover:bg-slate-50" @click="useResult('title')">Chỉ tiêu đề</button>
          <button type="button" class="rounded-md border border-slate-200 px-2.5 py-1 font-bold text-slate-600 hover:bg-slate-50" @click="useResult('description')">Chỉ mô tả</button>
        </div>
      </div>
      <div class="border-t border-indigo-100 pt-2">
        <button type="button" data-testid="seo-bulk" class="inline-flex w-full items-center justify-center gap-1.5 rounded-lg border border-indigo-200 bg-white px-3 py-2 text-xs font-bold text-indigo-700 hover:bg-indigo-50 disabled:opacity-50" :disabled="bulkBusy || aiBusy || !missingPages.length" @click="bulk">
          <Loader2 v-if="bulkBusy" class="h-3.5 w-3.5 animate-spin" /> Viết SEO cho mọi trang còn thiếu ({{ missingPages.length }})
        </button>
        <p v-if="bulkProgress" class="mt-1.5 text-[11px] text-slate-600" data-testid="seo-bulk-progress">{{ bulkProgress }}</p>
        <p v-if="bulkDone && !bulkBusy" class="mt-1.5 text-[11px] text-slate-600" data-testid="seo-bulk-note">Đã viết SEO cho {{ bulkDone }} trang. Xem lại rồi bấm Lưu nháp.</p>
        <p v-if="bulkSkipped && !bulkBusy" class="mt-1 text-[11px] text-slate-500">Bỏ qua {{ bulkSkipped }} trang chưa có nội dung.</p>
      </div>
      <p v-if="aiError" class="text-[11px] font-semibold text-rose-600" data-testid="seo-ai-error">{{ aiError }}</p>
      <p class="text-[10px] leading-snug text-slate-400">AI chỉ viết dựa trên nội dung trang. Hãy đọc lại trước khi dùng, đặc biệt các thông tin về giá và cam kết.</p>
    </div>

    <!-- Landing page -->
    <div class="space-y-2 border-t border-slate-100 pt-4">
      <span :class="lbl">Landing page</span>
      <label class="flex items-start gap-2 text-xs font-bold text-slate-600">
        <input v-model="settings.hide_header" data-testid="landing-hide-header" type="checkbox" class="mt-0.5" />
        <span>Ẩn dải thông báo và đầu trang ở trang này</span>
      </label>
      <label class="flex items-start gap-2 text-xs font-bold text-slate-600">
        <input v-model="settings.hide_footer" data-testid="landing-hide-footer" type="checkbox" class="mt-0.5" />
        <span>Ẩn chân trang ở trang này</span>
      </label>
      <p class="text-[11px] text-slate-400">Chỉ ẩn ở trang này, các trang khác vẫn hiện bình thường. Dùng cho trang bán hàng một sản phẩm, trang đăng ký, chiến dịch quảng cáo.</p>
    </div>

    <!-- SEO mặc định của cả website -->
    <div class="space-y-3 border-t border-slate-100 pt-4">
      <span :class="lbl">Cả website</span>
      <label class="block space-y-1">
        <span class="text-[11px] font-semibold text-slate-500">Mô tả mặc định (khi trang không có mô tả riêng)</span>
        <textarea v-model="siteSeo.description" data-testid="site-seo-desc" rows="2" maxlength="320" :class="inp" />
      </label>
      <ImageField v-model="siteSeo.image" :site-id="siteId" label="Ảnh chia sẻ mặc định (không chọn thì dùng logo)" compact />
      <label class="flex items-start gap-2 text-xs font-bold text-slate-600">
        <input v-model="siteSeo.noindex" data-testid="site-noindex" type="checkbox" class="mt-0.5" />
        <span>Ẩn CẢ website khỏi Google<br /><span class="font-normal text-slate-400">Bật khi đang làm thử, chưa muốn công khai trên tìm kiếm.</span></span>
      </label>
      <p class="text-[11px] text-slate-400">Sơ đồ website (sitemap) tự tạo ở <code>/s/{{ siteSlug }}/sitemap.xml</code> — gửi địa chỉ này cho Google Search Console.</p>
    </div>
  </div>
</template>
