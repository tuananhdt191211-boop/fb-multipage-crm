<script setup lang="ts">
// Nhập sản phẩm từ AliExpress / 1688 / Taobao / Shopify / website bất kỳ.
//  - "Dán link": máy chủ tự đọc trang (website thường lấy được khá đủ; AliExpress chỉ có tiêu đề + ảnh; 1688/Taobao thường bị chặn).
//  - "Dán nội dung trang": mở trang sản phẩm bằng trình duyệt của bạn → Ctrl+A → Ctrl+C → dán vào đây. Chạy với mọi nguồn
//    (trình duyệt của bạn đã vượt được captcha); AI đọc chữ, bỏ phần rác, dịch sang tiếng Việt, tách giá / phân loại.
// Kết quả chỉ là BẢN NHÁP: bấm "Dùng dữ liệu này" sẽ mở trình sửa sản phẩm đã điền sẵn để bạn xem lại rồi mới lưu.
import { computed, ref } from 'vue'
import { AlertTriangle, ArrowLeft, CheckCircle2, ClipboardPaste, Link2, Loader2, Sparkles, X } from 'lucide-vue-next'
import { api } from '../../lib/api'
import { escapeHtml, sanitizeHtml } from '../../lib/richHtml'
import { importImagesToLibrary, type ProductMedia } from '../../lib/productMedia'
import { safeImg } from '../../lib/safeUrl'

interface Draft {
  name: string
  description: string
  images: string[]
  videos: string[]
  price: number
  compare_price: number
  currency: string
  sku?: string
  options?: { name: string; values: string[] }[]
  source_url: string
  source: string
  fx?: { currency: string; vnd_per_unit: number }
  warnings: string[]
  blocked: boolean
  translated: boolean
}

export interface ImportResult {
  name: string
  description: string
  media: ProductMedia[]
  price: number
  compare_price: number
  cost_price: number
  sku: string
  source_url: string
}

const props = defineProps<{ siteId: number | string }>()
const emit = defineEmits<{ (e: 'close'): void; (e: 'apply', r: ImportResult): void }>()

const tab = ref<'link' | 'paste'>('link')
const url = ref('')
const sourceLink = ref('')
const pasted = ref<{ html: string; text: string; imgs: number } | null>(null)
const translate = ref(true)
const lang = ref('vi')
const rehost = ref(true)
const loading = ref(false)
const applying = ref('')
const error = ref('')
const draft = ref<Draft | null>(null)
const selected = ref<Set<string>>(new Set())
const selectedVideos = ref<Set<string>>(new Set())
const markup = ref(1)
const finalPrice = ref(0)
const finalCompare = ref(0)
const includeOptions = ref(true)

const SOURCE_NAMES: Record<string, string> = { aliexpress: 'AliExpress', '1688': '1688', taobao: 'Taobao / Tmall', shopify: 'Shopify', generic: 'website', paste: 'nội dung đã dán' }

// Gọn HTML dán từ clipboard: bỏ script / style / svg và mọi thuộc tính trừ ảnh & link, để gửi nhẹ (cả trang có thể nặng vài MB)
function slimHtml(html: string): string {
  const doc = new DOMParser().parseFromString(html, 'text/html')
  doc.querySelectorAll('script,style,svg,link,meta,noscript,iframe,canvas,template').forEach((n) => n.remove())
  const keep = new Set(['src', 'srcset', 'data-src', 'data-original', 'data-lazy-src', 'data-image', 'alt', 'href', 'poster'])
  doc.body.querySelectorAll('*').forEach((el) => {
    for (const a of Array.from(el.attributes)) if (!keep.has(a.name)) el.removeAttribute(a.name)
  })
  const out = doc.body.innerHTML
  return out.length > 2_500_000 ? out.slice(0, 2_500_000) : out
}

function onPaste(e: ClipboardEvent) {
  const cd = e.clipboardData
  if (!cd) return
  e.preventDefault()
  const html = cd.getData('text/html')
  const text = cd.getData('text/plain')
  if (!html && !text.trim()) {
    error.value = 'Không đọc được nội dung đã sao chép. Hãy chọn tất cả (Ctrl+A) trên trang sản phẩm, sao chép (Ctrl+C) rồi dán lại.'
    return
  }
  error.value = ''
  const slim = html ? slimHtml(html) : ''
  pasted.value = { html: slim, text: text.slice(0, 120000), imgs: (slim.match(/<img\b/gi) || []).length }
}

async function analyze() {
  error.value = ''
  draft.value = null
  loading.value = true
  try {
    const body: Record<string, unknown> = { translate: translate.value, lang: lang.value }
    if (tab.value === 'link') {
      if (!url.value.trim()) throw new Error('Hãy dán link sản phẩm.')
      body.url = url.value.trim()
    } else {
      if (!pasted.value) throw new Error('Hãy dán nội dung trang vào ô bên dưới (Ctrl+V).')
      body.html = pasted.value.html
      body.text = pasted.value.text
      if (sourceLink.value.trim()) body.url = sourceLink.value.trim()
    }
    const { data } = await api.post<{ draft: Draft }>(`/storefront/sites/${props.siteId}/products/import`, body, { timeout: 120000 })
    const d = data.draft
    draft.value = d
    selected.value = new Set(d.images.slice(0, 8))
    selectedVideos.value = new Set(d.videos.slice(0, 1))
    const isVnd = d.currency === 'VND' || !d.fx
    markup.value = isVnd ? 1 : 2.5
    recalc()
    includeOptions.value = true
  } catch (e: any) {
    error.value = e?.response?.data?.error || e?.message || 'Không phân tích được nội dung.'
  } finally {
    loading.value = false
  }
}

function roundVnd(x: number): number {
  if (!isFinite(x) || x <= 0) return 0
  return x >= 100000 ? Math.round(x / 1000) * 1000 : Math.round(x / 100) * 100
}

// Giá nhập quy đổi sang VND (chưa nhân hệ số)
const costVnd = computed(() => {
  const d = draft.value
  if (!d || !d.price) return 0
  if (d.currency === 'VND') return Math.round(d.price)
  return d.fx ? Math.round(d.price * d.fx.vnd_per_unit) : 0
})
const compareBaseVnd = computed(() => {
  const d = draft.value
  if (!d || !d.compare_price) return 0
  if (d.currency === 'VND') return d.compare_price
  return d.fx ? d.compare_price * d.fx.vnd_per_unit : 0
})

function recalc() {
  finalPrice.value = roundVnd(costVnd.value * (markup.value || 1))
  finalCompare.value = compareBaseVnd.value ? roundVnd(compareBaseVnd.value * (markup.value || 1)) : 0
}

function vnd(n: number) {
  return new Intl.NumberFormat('vi-VN').format(Math.round(n)) + 'đ'
}

function toggle(set: Set<string>, u: string, max: number) {
  if (set.has(u)) set.delete(u)
  else if (set.size < max) set.add(u)
}

const needsRehost = computed(() => rehost.value && selected.value.size > 0)

async function apply() {
  const d = draft.value
  if (!d) return
  applying.value = ''
  const imgs = d.images.filter((u) => selected.value.has(u))
  let finalImgs = imgs
  if (needsRehost.value) {
    applying.value = `Đang tải ${imgs.length} ảnh về thư viện của bạn...`
    try {
      const { map, failed } = await importImagesToLibrary(props.siteId, imgs)
      finalImgs = imgs.map((u) => map[u] || u) // ảnh không tải được thì giữ link gốc
      if (failed.length === imgs.length) error.value = 'Không tải được ảnh nào về thư viện (nguồn có thể chặn) — dùng link gốc của ảnh.'
    } catch {
      finalImgs = imgs
    }
  }
  const media: ProductMedia[] = [
    ...finalImgs.map((u) => ({ type: 'image' as const, url: u })),
    ...d.videos.filter((u) => selectedVideos.value.has(u)).map((u) => ({ type: 'video' as const, url: u }))
  ]
  let desc = d.description || ''
  if (includeOptions.value && d.options?.length) {
    desc += '<h3>Phân loại</h3><ul>' + d.options.map((o) => `<li><strong>${escapeHtml(o.name)}:</strong> ${escapeHtml(o.values.join(', '))}</li>`).join('') + '</ul>'
  }
  applying.value = ''
  emit('apply', {
    name: d.name,
    description: desc,
    media,
    price: finalPrice.value,
    compare_price: finalCompare.value > finalPrice.value ? finalCompare.value : 0,
    cost_price: costVnd.value,
    sku: d.sku || '',
    source_url: d.source_url || (tab.value === 'paste' ? sourceLink.value.trim() : '')
  })
}

const safeDescription = computed(() => sanitizeHtml(draft.value?.description || '') || '<p>(Chưa có mô tả)</p>')
const sourceLabel = computed(() => (draft.value ? SOURCE_NAMES[draft.value.source] || 'website' : ''))
</script>

<template>
  <div class="fixed inset-0 z-[60] flex items-start justify-center overflow-auto bg-black/50 p-4" @click.self="emit('close')">
    <div class="my-6 w-full max-w-3xl rounded-2xl bg-white shadow-2xl">
      <div class="flex items-center justify-between border-b border-slate-100 px-6 py-4">
        <div>
          <h2 class="text-base font-extrabold">Nhập sản phẩm từ AliExpress / 1688 / website khác</h2>
          <p class="text-xs text-slate-500">Kết quả chỉ là bản nháp — bạn xem lại và chỉnh sửa trước khi lưu.</p>
        </div>
        <button type="button" class="rounded-lg p-1.5 hover:bg-slate-100" @click="emit('close')"><X class="h-4 w-4" /></button>
      </div>

      <!-- Bước 1: lấy dữ liệu -->
      <div v-if="!draft || draft.blocked" class="space-y-4 p-6">
        <div class="inline-flex rounded-xl bg-slate-100 p-1 text-xs font-bold">
          <button type="button" class="inline-flex items-center gap-1.5 rounded-lg px-3 py-1.5" :class="tab === 'link' ? 'bg-white text-emerald-700 shadow-sm' : 'text-slate-500'" @click="tab = 'link'">
            <Link2 class="h-3.5 w-3.5" /> Dán link
          </button>
          <button type="button" class="inline-flex items-center gap-1.5 rounded-lg px-3 py-1.5" :class="tab === 'paste' ? 'bg-white text-emerald-700 shadow-sm' : 'text-slate-500'" @click="tab = 'paste'">
            <ClipboardPaste class="h-3.5 w-3.5" /> Dán nội dung trang
          </button>
        </div>

        <div v-if="tab === 'link'" class="space-y-2">
          <input v-model="url" class="w-full rounded-xl border border-slate-200 px-3 py-2.5 text-sm" placeholder="https://www.aliexpress.com/item/... hoặc link sản phẩm bất kỳ" @keydown.enter="analyze" />
          <p class="rounded-lg bg-slate-50 px-3 py-2 text-[11px] leading-relaxed text-slate-500">
            Website bán hàng thông thường (Shopify, WooCommerce...) lấy được khá đủ. <strong>AliExpress</strong> chỉ cho máy chủ lấy tiêu đề và ảnh (giá, mô tả nạp bằng JavaScript).
            <strong>1688 / Taobao</strong> chặn máy chủ bằng xác thực trượt — hãy dùng tab “Dán nội dung trang”.
          </p>
        </div>

        <div v-else class="space-y-3">
          <ol class="list-decimal space-y-1 pl-5 text-xs leading-relaxed text-slate-600">
            <li>Mở trang sản phẩm (AliExpress, 1688...) bằng trình duyệt của bạn và <strong>cuộn xuống hết</strong> để ảnh tải xong.</li>
            <li>Nhấn <kbd class="rounded border bg-slate-50 px-1">Ctrl</kbd>+<kbd class="rounded border bg-slate-50 px-1">A</kbd> rồi <kbd class="rounded border bg-slate-50 px-1">Ctrl</kbd>+<kbd class="rounded border bg-slate-50 px-1">C</kbd> để sao chép cả trang.</li>
            <li>Quay lại đây, bấm vào khung bên dưới rồi nhấn <kbd class="rounded border bg-slate-50 px-1">Ctrl</kbd>+<kbd class="rounded border bg-slate-50 px-1">V</kbd>.</li>
          </ol>
          <div
            tabindex="0"
            class="flex min-h-[110px] cursor-text flex-col items-center justify-center gap-1 rounded-xl border-2 border-dashed px-4 py-5 text-center text-sm outline-none focus:border-emerald-500 focus:bg-emerald-50/40"
            :class="pasted ? 'border-emerald-300 bg-emerald-50/50' : 'border-slate-300 text-slate-500'"
            @paste="onPaste"
          >
            <template v-if="pasted">
              <CheckCircle2 class="h-6 w-6 text-emerald-600" />
              <span class="font-bold text-emerald-700">Đã nhận nội dung: {{ pasted.text.length.toLocaleString() }} ký tự, {{ pasted.imgs }} ảnh</span>
              <span class="text-xs text-slate-500">Dán lại để thay bằng nội dung khác</span>
            </template>
            <template v-else>
              <ClipboardPaste class="h-6 w-6 text-slate-400" />
              <span class="font-semibold">Bấm vào đây rồi nhấn Ctrl+V</span>
            </template>
          </div>
          <input v-model="sourceLink" class="w-full rounded-xl border border-slate-200 px-3 py-2 text-xs" placeholder="Link gốc của sản phẩm (không bắt buộc — để lưu nguồn hàng và nhận diện AliExpress / 1688)" />
        </div>

        <div class="flex flex-wrap items-center gap-x-5 gap-y-2 rounded-xl border border-slate-200 px-3 py-2.5 text-xs font-semibold text-slate-600">
          <label class="inline-flex items-center gap-2"><input v-model="translate" type="checkbox" /> <Sparkles class="h-3.5 w-3.5 text-emerald-600" /> Dùng AI dịch &amp; làm sạch mô tả</label>
          <label v-if="translate" class="inline-flex items-center gap-2">
            Dịch sang
            <select v-model="lang" class="rounded-lg border border-slate-200 px-2 py-1 text-xs">
              <option value="vi">Tiếng Việt</option>
              <option value="en">English</option>
            </select>
          </label>
          <label class="inline-flex items-center gap-2"><input v-model="rehost" type="checkbox" /> Tải ảnh về thư viện của tôi (không phụ thuộc nguồn)</label>
        </div>

        <div v-if="draft?.blocked" class="space-y-1.5 rounded-xl border border-amber-200 bg-amber-50 px-3.5 py-3 text-xs text-amber-900">
          <p class="flex items-center gap-1.5 font-bold"><AlertTriangle class="h-4 w-4" /> Chưa lấy được dữ liệu</p>
          <p v-for="w in draft.warnings" :key="w" class="leading-relaxed">{{ w }}</p>
          <button v-if="tab === 'link'" type="button" class="mt-1 rounded-lg bg-white px-3 py-1.5 font-bold text-amber-800 ring-1 ring-amber-300 hover:bg-amber-100" @click="tab = 'paste'">Chuyển sang “Dán nội dung trang”</button>
        </div>
        <p v-if="error" class="rounded-xl bg-rose-50 px-3.5 py-2.5 text-xs font-semibold text-rose-700">{{ error }}</p>

        <div class="flex justify-end">
          <button type="button" class="inline-flex items-center gap-2 rounded-xl bg-emerald-600 px-5 py-2.5 text-sm font-bold text-white hover:bg-emerald-700 disabled:opacity-50" :disabled="loading" @click="analyze">
            <Loader2 v-if="loading" class="h-4 w-4 animate-spin" />
            {{ loading ? 'Đang phân tích...' : 'Lấy dữ liệu sản phẩm' }}
          </button>
        </div>
      </div>

      <!-- Bước 2: xem lại -->
      <div v-else class="space-y-5 p-6">
        <div class="flex flex-wrap items-center gap-2 text-xs">
          <button type="button" class="inline-flex items-center gap-1 rounded-lg border border-slate-200 px-2.5 py-1.5 font-bold text-slate-600 hover:bg-slate-50" @click="draft = null">
            <ArrowLeft class="h-3.5 w-3.5" /> Làm lại
          </button>
          <span class="rounded-full bg-slate-100 px-2.5 py-1 font-bold text-slate-600">Nguồn: {{ sourceLabel }}</span>
          <span v-if="draft.translated" class="inline-flex items-center gap-1 rounded-full bg-emerald-50 px-2.5 py-1 font-bold text-emerald-700"><Sparkles class="h-3 w-3" /> Đã dịch &amp; làm sạch bằng AI</span>
        </div>

        <ul v-if="draft.warnings.length" class="space-y-1 rounded-xl border border-amber-200 bg-amber-50 px-3.5 py-3 text-xs text-amber-900">
          <li v-for="w in draft.warnings" :key="w" class="flex gap-1.5"><AlertTriangle class="mt-0.5 h-3.5 w-3.5 shrink-0" /> {{ w }}</li>
        </ul>

        <label class="block space-y-1">
          <span class="text-xs font-bold text-slate-600">Tên sản phẩm</span>
          <input v-model="draft.name" class="w-full rounded-xl border border-slate-200 px-3 py-2 text-sm" />
        </label>

        <!-- Giá -->
        <div class="space-y-3 rounded-xl border border-slate-200 p-4">
          <div class="flex flex-wrap items-baseline justify-between gap-2">
            <span class="text-xs font-bold text-slate-600">Giá</span>
            <span v-if="draft.price" class="text-xs text-slate-500">
              Giá nguồn: <strong>{{ draft.price.toLocaleString() }} {{ draft.currency }}</strong>
              <template v-if="draft.currency !== 'VND' && draft.fx"> ≈ <strong>{{ vnd(costVnd) }}</strong> (tỉ giá hôm nay)</template>
            </span>
            <span v-else class="text-xs font-semibold text-amber-700">Chưa nhận ra giá — hãy nhập giá bán bên dưới</span>
          </div>
          <div class="grid grid-cols-1 gap-3 sm:grid-cols-3">
            <label class="block space-y-1">
              <span class="text-[11px] font-bold text-slate-500">Hệ số nhân (giá bán ÷ giá nhập)</span>
              <input v-model.number="markup" type="number" min="0.1" step="0.1" class="w-full rounded-xl border border-slate-200 px-3 py-2 text-sm" @input="recalc" />
            </label>
            <label class="block space-y-1">
              <span class="text-[11px] font-bold text-slate-500">Giá bán (đ)</span>
              <input v-model.number="finalPrice" type="number" min="0" class="w-full rounded-xl border border-slate-200 px-3 py-2 text-sm" />
            </label>
            <label class="block space-y-1">
              <span class="text-[11px] font-bold text-slate-500">Giá gốc gạch ngang (đ)</span>
              <input v-model.number="finalCompare" type="number" min="0" class="w-full rounded-xl border border-slate-200 px-3 py-2 text-sm" />
            </label>
          </div>
          <p v-if="costVnd && finalPrice" class="text-[11px] text-slate-500">
            Giá nhập lưu để tính lợi nhuận: <strong>{{ vnd(costVnd) }}</strong> · Lợi nhuận dự kiến:
            <strong :class="finalPrice - costVnd >= 0 ? 'text-emerald-700' : 'text-rose-600'">{{ vnd(finalPrice - costVnd) }}</strong>
            ({{ Math.round(((finalPrice - costVnd) / finalPrice) * 100) }}%)
          </p>
        </div>

        <!-- Ảnh -->
        <div class="space-y-2">
          <div class="flex items-center justify-between">
            <span class="text-xs font-bold text-slate-600">Ảnh ({{ selected.size }}/{{ draft.images.length }} được chọn — tối đa 20)</span>
            <span class="flex gap-3 text-[11px] font-semibold text-emerald-700">
              <button type="button" @click="selected = new Set(draft.images.slice(0, 20))">Chọn tất cả</button>
              <button type="button" class="text-slate-500" @click="selected = new Set()">Bỏ chọn</button>
            </span>
          </div>
          <div v-if="draft.images.length" class="grid max-h-72 grid-cols-4 gap-2 overflow-y-auto sm:grid-cols-6">
            <button
              v-for="im in draft.images"
              :key="im"
              type="button"
              class="relative aspect-square overflow-hidden rounded-lg border-2 bg-slate-100"
              :class="selected.has(im) ? 'border-emerald-500' : 'border-transparent opacity-60 hover:opacity-100'"
              @click="toggle(selected, im, 20)"
            >
              <img :src="safeImg(im)" alt="" referrerpolicy="no-referrer" class="h-full w-full object-cover" />
              <span v-if="selected.has(im)" class="absolute right-1 top-1 rounded-full bg-emerald-600 p-0.5 text-white"><CheckCircle2 class="h-3.5 w-3.5" /></span>
            </button>
          </div>
          <p v-else class="rounded-lg bg-slate-50 px-3 py-2 text-xs text-slate-500">Không tìm thấy ảnh. Bạn có thể thêm ảnh sau trong trình sửa sản phẩm.</p>
        </div>

        <!-- Video -->
        <div v-if="draft.videos.length" class="space-y-1.5">
          <span class="text-xs font-bold text-slate-600">Video tìm thấy</span>
          <label v-for="v in draft.videos" :key="v" class="flex items-center gap-2 break-all rounded-lg border border-slate-200 px-3 py-2 text-[11px] text-slate-600">
            <input type="checkbox" :checked="selectedVideos.has(v)" @change="toggle(selectedVideos, v, 3)" /> {{ v }}
          </label>
          <p class="text-[11px] text-slate-400">Link video của AliExpress / 1688 thường hết hạn hoặc bị chặn khi nhúng; nên tải video về rồi “Tải video lên” trong trình sửa sản phẩm.</p>
        </div>

        <!-- Phân loại -->
        <div v-if="draft.options?.length" class="space-y-1.5 rounded-xl border border-slate-200 p-4">
          <span class="text-xs font-bold text-slate-600">Phân loại tìm thấy</span>
          <p v-for="o in draft.options" :key="o.name" class="text-xs text-slate-600"><strong>{{ o.name }}:</strong> {{ o.values.join(', ') }}</p>
          <label class="inline-flex items-center gap-2 pt-1 text-[11px] font-semibold text-slate-500">
            <input v-model="includeOptions" type="checkbox" /> Ghi các phân loại này vào cuối mô tả (hệ thống chưa quản lý biến thể riêng cho từng phân loại)
          </label>
        </div>

        <!-- Mô tả -->
        <div class="space-y-1">
          <span class="text-xs font-bold text-slate-600">Mô tả (sẽ chỉnh tiếp trong trình sửa)</span>
          <div data-i18n="skip" class="pp-prose max-h-56 overflow-y-auto rounded-xl border border-slate-200 bg-slate-50/60 px-4 py-3 text-sm" v-html="safeDescription" />
        </div>

        <p v-if="error" class="rounded-xl bg-rose-50 px-3.5 py-2.5 text-xs font-semibold text-rose-700">{{ error }}</p>
        <div class="flex items-center justify-end gap-3 border-t border-slate-100 pt-4">
          <span v-if="applying" class="mr-auto inline-flex items-center gap-2 text-xs font-semibold text-emerald-700"><Loader2 class="h-4 w-4 animate-spin" /> {{ applying }}</span>
          <button type="button" class="rounded-xl border border-slate-200 px-4 py-2 text-sm font-bold" @click="emit('close')">Huỷ</button>
          <button type="button" class="rounded-xl bg-emerald-600 px-5 py-2 text-sm font-bold text-white hover:bg-emerald-700 disabled:opacity-50" :disabled="!!applying" @click="apply">Dùng dữ liệu này</button>
        </div>
      </div>
    </div>
  </div>
</template>
