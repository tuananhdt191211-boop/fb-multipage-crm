<script setup lang="ts">
// Kiểu chữ toàn trang (giống Typography của Shopify): bộ font gợi ý, font tiêu đề / nội dung,
// cỡ chữ – độ đậm – giãn dòng... cho từng thẻ H1–H4, P, và CSS sinh tự động.
import { computed, onMounted, ref } from 'vue'
import { Check, ChevronDown, Copy, RotateCcw } from 'lucide-vue-next'
import { ensureFonts, fontGroups, fontStack } from '../../lib/fonts'
import { themeCss, themePresets, themeTags } from '../../lib/theme'
import type { SiteTheme, ThemeTag, ThemeTypo } from '../../types/store'

const theme = defineModel<SiteTheme>({ required: true })

const inp = 'w-full rounded-lg border border-slate-200 bg-white px-2 py-1.5 text-xs focus:border-indigo-500 focus:outline-none'
const lbl = 'block text-[11px] font-bold text-slate-500'
const weights = ['300', '400', '500', '600', '700', '800', '900'] as const

const openTag = ref<ThemeTag | null>('h1')
const showCss = ref(false)
const copied = ref(false)

onMounted(() => {
  const s = new Set<string>()
  for (const p of themePresets) {
    if (p.theme.heading_font) s.add(p.theme.heading_font)
    if (p.theme.body_font) s.add(p.theme.body_font)
  }
  ensureFonts(s)
})

function setTheme(next: SiteTheme) {
  theme.value = JSON.parse(JSON.stringify(next))
  ensureFonts([theme.value.heading_font, theme.value.body_font, ...Object.values(theme.value.tags || {}).map((t) => t?.font)])
}

function setFont(key: 'heading_font' | 'body_font', v: string) {
  const next = { ...theme.value }
  if (v) next[key] = v
  else delete next[key]
  setTheme(next)
}

function tagOf(tag: ThemeTag): ThemeTypo {
  return theme.value.tags?.[tag] || {}
}

function patchTag(tag: ThemeTag, p: Partial<ThemeTypo>) {
  const cur: Record<string, unknown> = { ...tagOf(tag), ...p }
  for (const k of Object.keys(cur)) if (cur[k] === undefined || cur[k] === '' || cur[k] === false || cur[k] === null) delete cur[k]
  const tags = { ...(theme.value.tags || {}) }
  if (Object.keys(cur).length) tags[tag] = cur as ThemeTypo
  else delete tags[tag]
  const { tags: _old, ...rest } = theme.value
  setTheme(Object.keys(tags).length ? { ...rest, tags } : rest)
}

function num(v: string): number | undefined {
  const n = Number(v)
  return v.trim() !== '' && Number.isFinite(n) ? n : undefined
}

function isPreset(p: SiteTheme) {
  return JSON.stringify(p) === JSON.stringify(theme.value)
}

// Font hiển thị thực tế của một thẻ (để xem trước)
function previewStyle(tag: ThemeTag): Record<string, string> {
  const t = tagOf(tag)
  const base = tag === 'p' ? theme.value.body_font : theme.value.heading_font
  const s: Record<string, string> = {}
  const f = fontStack(t.font || base)
  if (f) s.fontFamily = f
  s.fontWeight = t.weight || (tag === 'p' ? '400' : '800')
  if (t.upper) s.textTransform = 'uppercase'
  if (t.color) s.color = t.color
  if (t.ls !== undefined) s.letterSpacing = `${t.ls}px`
  s.fontSize = tag === 'p' ? '13px' : `${Math.max(13, 22 - Number(tag[1]) * 2)}px`
  return s
}

const css = computed(() => themeCss(theme.value) || '/* Đang dùng kiểu chữ mặc định */')

async function copyCss() {
  try {
    await navigator.clipboard.writeText(css.value)
    copied.value = true
    setTimeout(() => (copied.value = false), 1200)
  } catch {
    /* trình duyệt chặn clipboard */
  }
}
</script>

<template>
  <div class="space-y-4">
    <div class="flex items-center justify-between">
      <span class="text-xs font-extrabold uppercase tracking-wide text-slate-500">Kiểu chữ toàn trang</span>
      <button v-if="Object.keys(theme).length" type="button" class="inline-flex items-center gap-1 text-[11px] font-semibold text-slate-500 hover:text-indigo-700" @click="setTheme({})">
        <RotateCcw class="h-3 w-3" /> Mặc định
      </button>
    </div>

    <!-- Bộ font gợi ý -->
    <div class="grid grid-cols-2 gap-1.5">
      <button
        v-for="p in themePresets"
        :key="p.name"
        type="button"
        class="rounded-xl border p-2 text-left transition"
        :class="isPreset(p.theme) ? 'border-indigo-600 bg-indigo-50' : 'border-slate-200 bg-white hover:border-indigo-300'"
        @click="setTheme(p.theme)"
      >
        <div class="truncate text-lg leading-tight text-slate-900" :style="{ fontFamily: fontStack(p.theme.heading_font) || undefined, fontWeight: 700 }">Aa Bb</div>
        <div class="mt-0.5 text-[11px] font-bold text-slate-700">{{ p.name }}</div>
        <div class="truncate text-[10px] text-slate-400">{{ p.desc }}</div>
      </button>
    </div>

    <!-- Font tiêu đề / nội dung -->
    <div class="space-y-2.5 rounded-xl border border-slate-200 bg-white p-3">
      <label class="block space-y-1">
        <span :class="lbl">Font tiêu đề (H1–H4, tên thương hiệu)</span>
        <select :value="theme.heading_font || ''" :class="inp" @change="setFont('heading_font', ($event.target as HTMLSelectElement).value)">
          <option value="">Mặc định (Be Vietnam Pro)</option>
          <optgroup v-for="g in fontGroups" :key="g.group" :label="g.group">
            <option v-for="f in g.fonts" :key="f.name" :value="f.name">{{ f.name }}</option>
          </optgroup>
        </select>
      </label>
      <label class="block space-y-1">
        <span :class="lbl">Font nội dung (đoạn văn, menu, nút)</span>
        <select :value="theme.body_font || ''" :class="inp" @change="setFont('body_font', ($event.target as HTMLSelectElement).value)">
          <option value="">Mặc định (Be Vietnam Pro)</option>
          <optgroup v-for="g in fontGroups" :key="g.group" :label="g.group">
            <option v-for="f in g.fonts" :key="f.name" :value="f.name">{{ f.name }}</option>
          </optgroup>
        </select>
      </label>
    </div>

    <!-- Từng thẻ -->
    <div class="overflow-hidden rounded-xl border border-slate-200 bg-white">
      <div v-for="t in themeTags" :key="t.tag" class="border-b border-slate-100 last:border-b-0">
        <button type="button" class="flex w-full items-center gap-2 px-3 py-2.5 text-left hover:bg-slate-50" @click="openTag = openTag === t.tag ? null : t.tag">
          <span class="w-7 shrink-0 rounded bg-slate-900 py-0.5 text-center text-[10px] font-extrabold uppercase text-white">{{ t.tag }}</span>
          <span class="min-w-0 flex-1 truncate text-slate-900" :style="previewStyle(t.tag)">{{ t.sample }}</span>
          <span v-if="theme.tags?.[t.tag]" class="h-1.5 w-1.5 shrink-0 rounded-full bg-indigo-500" title="Đã tuỳ chỉnh" />
          <ChevronDown class="h-3.5 w-3.5 shrink-0 text-slate-400 transition" :class="openTag === t.tag && 'rotate-180'" />
        </button>

        <div v-if="openTag === t.tag" class="space-y-2.5 bg-slate-50/60 px-3 pb-3 pt-1">
          <label class="block space-y-1">
            <span :class="lbl">Font</span>
            <select :value="tagOf(t.tag).font || ''" :class="inp" @change="patchTag(t.tag, { font: ($event.target as HTMLSelectElement).value || undefined })">
              <option value="">Theo font {{ t.tag === 'p' ? 'nội dung' : 'tiêu đề' }}</option>
              <optgroup v-for="g in fontGroups" :key="g.group" :label="g.group">
                <option v-for="f in g.fonts" :key="f.name" :value="f.name">{{ f.name }}</option>
              </optgroup>
            </select>
          </label>
          <div class="grid grid-cols-2 gap-2">
            <label class="space-y-1">
              <span :class="lbl">Cỡ máy tính (px)</span>
              <input type="number" min="8" max="160" :value="tagOf(t.tag).size ?? ''" placeholder="Mặc định" :class="inp" @input="patchTag(t.tag, { size: num(($event.target as HTMLInputElement).value) })" />
            </label>
            <label class="space-y-1">
              <span :class="lbl">Cỡ điện thoại (px)</span>
              <input type="number" min="8" max="120" :value="tagOf(t.tag).size_m ?? ''" placeholder="Tự động" :class="inp" @input="patchTag(t.tag, { size_m: num(($event.target as HTMLInputElement).value) })" />
            </label>
            <label class="space-y-1">
              <span :class="lbl">Độ đậm</span>
              <select :value="tagOf(t.tag).weight || ''" :class="inp" @change="patchTag(t.tag, { weight: (($event.target as HTMLSelectElement).value || undefined) as ThemeTypo['weight'] })">
                <option value="">Mặc định</option>
                <option v-for="w in weights" :key="w" :value="w">{{ w }}</option>
              </select>
            </label>
            <label class="space-y-1">
              <span :class="lbl">Giãn dòng</span>
              <input type="number" step="0.1" min="0.8" max="3" :value="tagOf(t.tag).lh ?? ''" placeholder="vd. 1.3" :class="inp" @input="patchTag(t.tag, { lh: num(($event.target as HTMLInputElement).value) })" />
            </label>
            <label class="space-y-1">
              <span :class="lbl">Giãn chữ (px)</span>
              <input type="number" step="0.5" min="-5" max="20" :value="tagOf(t.tag).ls ?? ''" placeholder="vd. 0.5" :class="inp" @input="patchTag(t.tag, { ls: num(($event.target as HTMLInputElement).value) })" />
            </label>
            <div class="space-y-1">
              <span :class="lbl">Màu chữ</span>
              <div class="flex items-center gap-1.5">
                <input type="color" :value="tagOf(t.tag).color || '#0f172a'" class="h-7 w-9 rounded border border-slate-200" @input="patchTag(t.tag, { color: ($event.target as HTMLInputElement).value })" />
                <button v-if="tagOf(t.tag).color" type="button" class="text-[11px] text-slate-500 underline" @click="patchTag(t.tag, { color: undefined })">Bỏ</button>
              </div>
            </div>
          </div>
          <div class="flex items-center justify-between">
            <label class="flex items-center gap-1.5 text-xs font-semibold text-slate-600">
              <input type="checkbox" :checked="!!tagOf(t.tag).upper" class="accent-indigo-600" @change="patchTag(t.tag, { upper: ($event.target as HTMLInputElement).checked })" />
              VIẾT HOA
            </label>
            <button v-if="theme.tags?.[t.tag]" type="button" class="text-[11px] font-semibold text-slate-500 hover:text-rose-600" @click="patchTag(t.tag, { font: undefined, size: undefined, size_m: undefined, weight: undefined, lh: undefined, ls: undefined, upper: undefined, color: undefined })">
              Đặt lại {{ t.tag.toUpperCase() }}
            </button>
          </div>
          <p v-if="t.tag !== 'p' && tagOf(t.tag).color" class="text-[10px] text-amber-700">Màu áp cho mọi {{ t.tag.toUpperCase() }}, kể cả tiêu đề trên ảnh banner. Muốn riêng từng chữ, bấm vào chữ đó trên trang.</p>
        </div>
      </div>
    </div>

    <!-- CSS sinh tự động -->
    <div class="space-y-1.5">
      <div class="flex items-center justify-between">
        <button type="button" class="inline-flex items-center gap-1 text-[11px] font-bold text-slate-600 hover:text-indigo-700" @click="showCss = !showCss">
          <ChevronDown class="h-3 w-3 transition" :class="showCss ? 'rotate-180' : ''" /> Xem CSS được tạo
        </button>
        <button v-if="showCss" type="button" class="inline-flex items-center gap-1 text-[11px] font-semibold text-slate-500 hover:text-indigo-700" @click="copyCss">
          <Check v-if="copied" class="h-3 w-3 text-emerald-600" /><Copy v-else class="h-3 w-3" /> {{ copied ? 'Đã chép' : 'Sao chép' }}
        </button>
      </div>
      <pre v-if="showCss" class="max-h-56 overflow-auto whitespace-pre-wrap break-all rounded-lg bg-slate-900 p-2.5 font-mono text-[10.5px] leading-relaxed text-emerald-200">{{ css }}</pre>
    </div>
  </div>
</template>
