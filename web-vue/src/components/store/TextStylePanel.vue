<script setup lang="ts">
// Bảng "Kiểu chữ" (cột phải của trình chỉnh sửa): chọn thẻ, cỡ, màu, độ đậm...
// và xem / sửa CSS, HTML được sinh tự động — kiểu Shopify.
import { computed, ref } from 'vue'
import { AlignCenter, AlignJustify, AlignLeft, AlignRight, Bold, Check, Copy, Italic, RotateCcw, Underline, X } from 'lucide-vue-next'
import { cleanTextStyle, cssPreview, htmlPreview, tagChoices } from '../../lib/textStyle'
import { fontGroups } from '../../lib/fonts'
import type { StoreBlock, TextStyle } from '../../types/store'

const props = defineProps<{ block: StoreBlock; fxKey: string; baseTag: string; text: string }>()
const emit = defineEmits<{ close: [] }>()

const inp = 'w-full rounded-lg border border-slate-200 bg-white px-2.5 py-1.5 text-sm focus:border-indigo-500 focus:outline-none'
const lbl = 'block text-xs font-bold text-slate-600'

const colorPresets = ['#0f172a', '#334155', '#64748b', '#ffffff', '#dc2626', '#ea580c', '#ca8a04', '#059669', '#2563eb', '#7c3aed', '#db2777']
const weights = [
  { v: '', t: 'Mặc định' },
  { v: '300', t: 'Mảnh (300)' },
  { v: '400', t: 'Thường (400)' },
  { v: '500', t: 'Vừa (500)' },
  { v: '600', t: 'Hơi đậm (600)' },
  { v: '700', t: 'Đậm (700)' },
  { v: '800', t: 'Rất đậm (800)' },
  { v: '900', t: 'Đen (900)' }
]
const aligns = [
  { v: 'left', icon: AlignLeft, t: 'Trái' },
  { v: 'center', icon: AlignCenter, t: 'Giữa' },
  { v: 'right', icon: AlignRight, t: 'Phải' },
  { v: 'justify', icon: AlignJustify, t: 'Đều hai bên' }
] as const

// Chỉ đổi thẻ cho chữ dạng khối (tiêu đề, đoạn văn), không đổi chữ trong nút / menu
const allowTag = computed(() => ['h1', 'h2', 'h3', 'h4', 'p', 'div'].includes(props.baseTag))

const st = computed<TextStyle>(() => (props.block.data?._text?.[props.fxKey] as TextStyle) || {})
const currentTag = computed(() => st.value.tag || props.baseTag)

function patch(p: Partial<TextStyle>) {
  const d = props.block.data
  const all: Record<string, TextStyle> = { ...(d._text || {}) }
  const next = cleanTextStyle({ ...(all[props.fxKey] || {}), ...p } as TextStyle)
  if (next) all[props.fxKey] = next
  else delete all[props.fxKey]
  if (Object.keys(all).length) d._text = all
  else delete d._text
}

function num(v: string): number | undefined {
  const n = Number(v)
  return v.trim() !== '' && Number.isFinite(n) ? n : undefined
}

function reset() {
  patch({ tag: undefined, size: undefined, size_m: undefined, color: undefined, weight: undefined, italic: undefined, underline: undefined, upper: undefined, align: undefined, lh: undefined, ls: undefined, font: undefined, css: undefined })
}

const hasStyle = computed(() => Object.keys(st.value).length > 0)
const codeTab = ref<'css' | 'html'>('css')
const css = computed(() => cssPreview(st.value))
const html = computed(() => htmlPreview(currentTag.value, props.text || '…', st.value))
const copied = ref(false)
async function copyCode() {
  try {
    await navigator.clipboard.writeText(codeTab.value === 'css' ? css.value : html.value)
    copied.value = true
    setTimeout(() => (copied.value = false), 1200)
  } catch {
    /* trình duyệt chặn clipboard */
  }
}
</script>

<template>
  <div class="space-y-4 rounded-xl border border-indigo-200 bg-indigo-50/40 p-3">
    <div class="flex items-start justify-between gap-2">
      <div class="min-w-0">
        <div class="text-xs font-extrabold uppercase tracking-wide text-indigo-700">Kiểu chữ</div>
        <div class="truncate text-[11px] text-slate-500">“{{ text || '(trống)' }}”</div>
      </div>
      <div class="flex shrink-0 items-center gap-1">
        <button v-if="hasStyle" type="button" class="rounded-lg p-1 text-slate-500 hover:bg-white" title="Đặt lại mặc định" @click="reset"><RotateCcw class="h-3.5 w-3.5" /></button>
        <button type="button" class="rounded-lg p-1 text-slate-500 hover:bg-white" title="Đóng" @click="emit('close')"><X class="h-4 w-4" /></button>
      </div>
    </div>

    <!-- Thẻ HTML -->
    <div v-if="allowTag" class="space-y-1">
      <span :class="lbl">Thẻ HTML</span>
      <div class="grid grid-cols-6 gap-1">
        <button
          v-for="t in tagChoices"
          :key="t.v"
          type="button"
          class="rounded-md border py-1 text-[11px] font-bold"
          :class="currentTag === t.v ? 'border-indigo-600 bg-indigo-600 text-white' : 'border-slate-200 bg-white text-slate-600 hover:border-indigo-300'"
          @click="patch({ tag: t.v === baseTag ? undefined : t.v })"
        >
          {{ t.t }}
        </button>
      </div>
      <p class="text-[10px] text-slate-400">H1 chỉ nên dùng 1 lần mỗi trang (tốt cho SEO).</p>
    </div>

    <!-- Cỡ chữ -->
    <div class="space-y-1.5">
      <span :class="lbl">Cỡ chữ (px)</span>
      <div class="grid grid-cols-2 gap-2">
        <label class="space-y-0.5">
          <span class="text-[10px] font-semibold text-slate-500">Máy tính</span>
          <input type="number" min="8" max="160" :value="st.size ?? ''" placeholder="Mặc định" :class="inp" @input="patch({ size: num(($event.target as HTMLInputElement).value) })" />
        </label>
        <label class="space-y-0.5">
          <span class="text-[10px] font-semibold text-slate-500">Điện thoại</span>
          <input type="number" min="8" max="120" :value="st.size_m ?? ''" placeholder="Tự động" :class="inp" @input="patch({ size_m: num(($event.target as HTMLInputElement).value) })" />
        </label>
      </div>
      <input type="range" min="10" max="96" :value="st.size ?? 16" class="w-full accent-indigo-600" @input="patch({ size: Number(($event.target as HTMLInputElement).value) })" />
    </div>

    <!-- Màu chữ -->
    <div class="space-y-1.5">
      <span :class="lbl">Màu chữ</span>
      <div class="flex flex-wrap items-center gap-1.5">
        <button
          v-for="c in colorPresets"
          :key="c"
          type="button"
          class="h-6 w-6 rounded-full border border-slate-200 ring-offset-1"
          :class="st.color?.toLowerCase() === c ? 'ring-2 ring-indigo-500' : ''"
          :style="{ backgroundColor: c }"
          :title="c"
          @click="patch({ color: c })"
        />
        <input type="color" :value="st.color && st.color.startsWith('#') ? st.color : '#0f172a'" class="h-6 w-8 rounded border border-slate-200" title="Màu khác" @input="patch({ color: ($event.target as HTMLInputElement).value })" />
        <button v-if="st.color" type="button" class="text-[11px] text-slate-500 underline" @click="patch({ color: undefined })">Bỏ màu</button>
      </div>
    </div>

    <!-- Độ đậm, kiểu -->
    <div class="grid grid-cols-2 gap-2">
      <label class="space-y-1">
        <span :class="lbl">Độ đậm</span>
        <select :value="st.weight || ''" :class="inp" @change="patch({ weight: (($event.target as HTMLSelectElement).value || undefined) as TextStyle['weight'] })">
          <option v-for="w in weights" :key="w.v" :value="w.v">{{ w.t }}</option>
        </select>
      </label>
      <label class="space-y-1">
        <span :class="lbl">Phông chữ</span>
        <select :value="st.font || ''" :class="inp" @change="patch({ font: ($event.target as HTMLSelectElement).value || undefined })">
          <option value="">Theo toàn trang</option>
          <optgroup v-for="g in fontGroups" :key="g.group" :label="g.group">
            <option v-for="f in g.fonts" :key="f.name" :value="f.name">{{ f.name }}</option>
          </optgroup>
        </select>
      </label>
    </div>

    <div class="flex flex-wrap items-center gap-1">
      <button type="button" class="rounded-md border p-1.5" :class="st.weight === '700' ? 'border-indigo-600 bg-indigo-600 text-white' : 'border-slate-200 bg-white text-slate-600'" title="In đậm" @click="patch({ weight: st.weight === '700' ? undefined : '700' })"><Bold class="h-3.5 w-3.5" /></button>
      <button type="button" class="rounded-md border p-1.5" :class="st.italic ? 'border-indigo-600 bg-indigo-600 text-white' : 'border-slate-200 bg-white text-slate-600'" title="In nghiêng" @click="patch({ italic: !st.italic })"><Italic class="h-3.5 w-3.5" /></button>
      <button type="button" class="rounded-md border p-1.5" :class="st.underline ? 'border-indigo-600 bg-indigo-600 text-white' : 'border-slate-200 bg-white text-slate-600'" title="Gạch chân" @click="patch({ underline: !st.underline })"><Underline class="h-3.5 w-3.5" /></button>
      <button type="button" class="rounded-md border px-2 py-1 text-[11px] font-extrabold" :class="st.upper ? 'border-indigo-600 bg-indigo-600 text-white' : 'border-slate-200 bg-white text-slate-600'" title="Viết hoa toàn bộ" @click="patch({ upper: !st.upper })">AA</button>
      <span class="mx-1 h-5 w-px bg-slate-200" />
      <button
        v-for="a in aligns"
        :key="a.v"
        type="button"
        class="rounded-md border p-1.5"
        :class="st.align === a.v ? 'border-indigo-600 bg-indigo-600 text-white' : 'border-slate-200 bg-white text-slate-600'"
        :title="`Căn ${a.t}`"
        @click="patch({ align: st.align === a.v ? undefined : a.v })"
      >
        <component :is="a.icon" class="h-3.5 w-3.5" />
      </button>
    </div>

    <div class="grid grid-cols-2 gap-2">
      <label class="space-y-1">
        <span :class="lbl">Giãn dòng</span>
        <input type="number" step="0.1" min="0.8" max="3" :value="st.lh ?? ''" placeholder="vd. 1.5" :class="inp" @input="patch({ lh: num(($event.target as HTMLInputElement).value) })" />
      </label>
      <label class="space-y-1">
        <span :class="lbl">Giãn chữ (px)</span>
        <input type="number" step="0.5" min="-5" max="20" :value="st.ls ?? ''" placeholder="vd. 1" :class="inp" @input="patch({ ls: num(($event.target as HTMLInputElement).value) })" />
      </label>
    </div>

    <!-- Code sinh tự động -->
    <div class="space-y-1.5">
      <div class="flex items-center justify-between">
        <div class="flex gap-1">
          <button type="button" class="rounded-md px-2 py-0.5 text-[11px] font-bold" :class="codeTab === 'css' ? 'bg-slate-900 text-white' : 'text-slate-500 hover:bg-white'" @click="codeTab = 'css'">CSS</button>
          <button type="button" class="rounded-md px-2 py-0.5 text-[11px] font-bold" :class="codeTab === 'html' ? 'bg-slate-900 text-white' : 'text-slate-500 hover:bg-white'" @click="codeTab = 'html'">HTML</button>
        </div>
        <button type="button" class="inline-flex items-center gap-1 text-[11px] font-semibold text-slate-500 hover:text-indigo-700" @click="copyCode">
          <Check v-if="copied" class="h-3 w-3 text-emerald-600" /><Copy v-else class="h-3 w-3" /> {{ copied ? 'Đã chép' : 'Sao chép' }}
        </button>
      </div>
      <pre class="max-h-40 overflow-auto whitespace-pre-wrap break-all rounded-lg bg-slate-900 p-2.5 font-mono text-[11px] leading-relaxed text-emerald-200">{{ codeTab === 'css' ? css : html }}</pre>
      <p class="text-[10px] text-slate-400">Code được tạo tự động từ các lựa chọn ở trên.</p>
    </div>

    <label class="block space-y-1">
      <span :class="lbl">CSS viết thêm cho chữ này</span>
      <textarea
        :value="st.css || ''"
        rows="3"
        spellcheck="false"
        placeholder="text-shadow: 0 2px 8px rgba(0,0,0,.3);&#10;background: #fef3c7;"
        class="w-full rounded-lg border border-slate-200 bg-white px-2.5 py-1.5 font-mono text-xs focus:border-indigo-500 focus:outline-none"
        @input="patch({ css: ($event.target as HTMLTextAreaElement).value || undefined })"
      />
      <span class="block text-[10px] text-slate-400">Viết dạng <code>thuộc-tính: giá-trị;</code> — áp dụng ngay khi gõ.</span>
    </label>
  </div>
</template>
