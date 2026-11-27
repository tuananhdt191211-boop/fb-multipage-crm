<script setup lang="ts">
// Bán hàng quốc tế (tab "Ngôn ngữ" của trình chỉnh sửa):
// chọn thị trường, dịch bằng AI, sửa từng bản dịch, xem trước theo ngôn ngữ.
import { computed, nextTick, ref } from 'vue'
import { CheckCircle2, Languages, Loader2, Search, Sparkles } from 'lucide-vue-next'
import { api } from '../../lib/api'
import { LANG_NAMES, LANG_VI, MARKET_BY_CODE, MARKET_REGIONS, langsOf } from '../../lib/markets'
import { UI_VI, collectBlockTexts, collectProductTexts, type SiteI18n } from '../../lib/i18n'
import type { PublicProduct, StoreBlock } from '../../types/store'

const props = defineProps<{ siteId: number | string; blocks: StoreBlock[]; products: PublicProduct[] }>()
const i18n = defineModel<SiteI18n>('i18n', { required: true })
const previewLang = defineModel<string>('preview', { required: true })
const emit = defineEmits<{ changed: [] }>()

const enabled = computed(() => new Set(['vn', ...i18n.value.markets]))
const langs = computed(() => langsOf(i18n.value.markets))

// Lưu ý: với v-model, i18n.value chỉ cập nhật sau khi cha render lại -> mỗi thao tác chỉ gán 1 lần
function setMarkets(markets: string[]) {
  const def = i18n.value.default && (i18n.value.default === 'vn' || markets.includes(i18n.value.default)) ? i18n.value.default : 'vn'
  i18n.value = { ...i18n.value, markets, default: def }
  if (previewLang.value !== 'vi' && !langsOf(markets).includes(previewLang.value)) previewLang.value = 'vi'
  emit('changed')
}

function toggleMarket(code: string) {
  if (code === 'vn') return
  const set = new Set(i18n.value.markets)
  if (set.has(code)) set.delete(code)
  else set.add(code)
  setMarkets([...set])
}

function toggleRegion(region: string, on: boolean) {
  const set = new Set(i18n.value.markets)
  for (const m of MARKET_REGIONS.find((r) => r.region === region)?.markets || []) {
    if (m.code === 'vn') continue
    if (on) set.add(m.code)
    else set.delete(m.code)
  }
  setMarkets([...set])
}

function setDefault(code: string) {
  i18n.value = { ...i18n.value, default: code }
  emit('changed')
}

// ----------------------------------------------------------------- Nguồn tiếng Việt cần dịch

interface Row {
  key: string // b|blockId|path, p|id|name, u|key
  group: 'Trang' | 'Sản phẩm' | 'Giao diện'
  src: string
}

const rows = computed<Row[]>(() => {
  const out: Row[] = []
  for (const [bid, paths] of Object.entries(collectBlockTexts(props.blocks))) {
    for (const [p, v] of Object.entries(paths)) out.push({ key: `b|${bid}|${p}`, group: 'Trang', src: v })
  }
  for (const [pid, e] of Object.entries(collectProductTexts(props.products))) {
    if (e.name) out.push({ key: `p|${pid}|name`, group: 'Sản phẩm', src: e.name })
    if (e.description) out.push({ key: `p|${pid}|description`, group: 'Sản phẩm', src: e.description })
  }
  for (const [k, v] of Object.entries(UI_VI)) out.push({ key: `u|${k}`, group: 'Giao diện', src: v })
  return out
})

function getTr(lang: string, key: string): string {
  const tr = i18n.value.tr[lang]
  if (!tr) return ''
  const [kind, a, b] = key.split('|') as [string, string, string]
  if (kind === 'b') return tr.b?.[a]?.[b] || ''
  if (kind === 'p') return (tr.p?.[a] as any)?.[b] || ''
  return tr.ui?.[a] || ''
}

// Ghi nhiều câu dịch của 1 ngôn ngữ trong 1 lần gán
function setTrMany(lang: string, entries: Record<string, string>) {
  const tr = JSON.parse(JSON.stringify(i18n.value.tr[lang] || {}))
  for (const [key, value] of Object.entries(entries)) {
    const [kind, a, b] = key.split('|') as [string, string, string]
    if (kind === 'b') ((tr.b ||= {})[a] ||= {})[b] = value
    else if (kind === 'p') ((tr.p ||= {})[a] ||= {})[b] = value
    else (tr.ui ||= {})[a] = value
  }
  i18n.value = { ...i18n.value, tr: { ...i18n.value.tr, [lang]: tr } }
}

function setTr(lang: string, key: string, value: string) {
  setTrMany(lang, { [key]: value })
}

function doneCount(lang: string) {
  return rows.value.filter((r) => getTr(lang, r.key)).length
}

// ----------------------------------------------------------------- Dịch bằng AI

const running = ref(false)
const progress = ref('')
const errorMsg = ref('')
const warnMsg = ref('')
const redoAll = ref(false)

async function translateAll() {
  if (!langs.value.length || running.value) return
  running.value = true
  errorMsg.value = ''
  warnMsg.value = ''
  let failed = 0
  try {
    for (const [li, lang] of langs.value.entries()) {
      const todo = rows.value.filter((r) => redoAll.value || !getTr(lang, r.key))
      for (let i = 0; i < todo.length; i += 150) {
        const chunk = todo.slice(i, i + 150)
        progress.value = `${LANG_VI[lang]} (${li + 1}/${langs.value.length}) · ${Math.min(i + chunk.length, todo.length)}/${todo.length} câu`
        const texts: Record<string, string> = {}
        for (const r of chunk) texts[r.key] = r.src
        const { data } = await api.post(`/storefront/sites/${props.siteId}/translate`, { lang, texts }, { timeout: 180000 })
        setTrMany(lang, (data.translations || {}) as Record<string, string>)
        await nextTick() // đợi v-model cập nhật trước nhóm câu tiếp theo
        failed += Number(data.failed || 0)
      }
    }
    progress.value = ''
    if (failed) warnMsg.value = `Còn ${failed} câu AI chưa dịch được (thường do giới hạn lượt gọi AI). Bấm "Dịch bằng AI" lần nữa để dịch tiếp phần còn thiếu.`
    emit('changed')
  } catch (e: any) {
    errorMsg.value = e?.response?.data?.error || 'Dịch chưa xong, vui lòng thử lại. Phần đã dịch vẫn được giữ.'
    emit('changed')
  } finally {
    running.value = false
    progress.value = ''
  }
}

// ----------------------------------------------------------------- Sửa bản dịch

const editLang = ref('')
const query = ref('')
const groupFilter = ref<'all' | Row['group']>('all')
const onlyMissing = ref(false)
const shown = computed(() => {
  const q = query.value.trim().toLowerCase()
  const lang = editLang.value
  return rows.value
    .filter((r) => groupFilter.value === 'all' || r.group === groupFilter.value)
    .filter((r) => !onlyMissing.value || !getTr(lang, r.key))
    .filter((r) => !q || r.src.toLowerCase().includes(q) || getTr(lang, r.key).toLowerCase().includes(q))
    .slice(0, 300)
})

function onEdit(key: string, ev: Event) {
  setTr(editLang.value, key, (ev.target as HTMLTextAreaElement).value)
  emit('changed')
}
</script>

<template>
  <div class="space-y-4">
    <div class="rounded-xl bg-indigo-50 p-3 text-[11px] leading-relaxed text-indigo-900">
      <div class="mb-1 flex items-center gap-1.5 text-xs font-extrabold"><Languages class="h-4 w-4" /> Bán hàng quốc tế</div>
      Chọn các nước bán hàng. Khách chọn nước ở đầu trang → web đổi ngôn ngữ và giá tiền theo tỉ giá cập nhật hằng ngày.
    </div>

    <!-- Thị trường -->
    <div class="space-y-3">
      <div v-for="g in MARKET_REGIONS" :key="g.region" class="space-y-1">
        <div class="flex items-center justify-between">
          <span class="text-[11px] font-extrabold uppercase tracking-wide text-slate-400">{{ g.region }}</span>
          <span v-if="g.region !== 'Châu Á'" class="flex gap-2 text-[10px] font-semibold">
            <button type="button" class="text-indigo-600 hover:underline" @click="toggleRegion(g.region, true)">Chọn hết</button>
            <button type="button" class="text-slate-400 hover:underline" @click="toggleRegion(g.region, false)">Bỏ</button>
          </span>
        </div>
        <div class="grid grid-cols-1 gap-0.5">
          <label
            v-for="mk in g.markets"
            :key="mk.code"
            class="flex cursor-pointer items-center gap-2 rounded-lg px-2 py-1 text-xs hover:bg-slate-50"
            :class="mk.code === 'vn' ? 'cursor-default opacity-70' : ''"
          >
            <input type="checkbox" class="accent-indigo-600" :checked="enabled.has(mk.code)" :disabled="mk.code === 'vn'" @change="toggleMarket(mk.code)" />
            <span class="text-sm leading-none">{{ mk.flag }}</span>
            <span class="min-w-0 flex-1 truncate font-semibold text-slate-700">{{ mk.country }}</span>
            <span class="shrink-0 text-[10px] text-slate-400">{{ mk.lang.toUpperCase() }} · {{ mk.currency }}</span>
          </label>
        </div>
      </div>
    </div>

    <label v-if="i18n.markets.length" class="block space-y-1">
      <span class="text-xs font-bold text-slate-600">Thị trường mặc định (khi không đoán được nước của khách)</span>
      <select :value="i18n.default || 'vn'" class="w-full rounded-lg border border-slate-200 px-2.5 py-1.5 text-sm" @change="setDefault(($event.target as HTMLSelectElement).value)">
        <option v-for="c in ['vn', ...i18n.markets]" :key="c" :value="c">{{ MARKET_BY_CODE.get(c)?.flag }} {{ MARKET_BY_CODE.get(c)?.country }} · {{ MARKET_BY_CODE.get(c)?.currency }}</option>
      </select>
      <span class="block text-[10px] text-slate-400">Khách mở web lần đầu: hệ thống tự chọn theo ngôn ngữ trình duyệt của họ.</span>
    </label>

    <!-- Dịch bằng AI -->
    <div v-if="langs.length" class="space-y-2 rounded-xl border border-slate-200 p-3">
      <div class="text-xs font-extrabold text-slate-700">Bản dịch ({{ langs.length }} ngôn ngữ · {{ rows.length }} câu mỗi ngôn ngữ)</div>
      <div class="space-y-1">
        <div v-for="l in langs" :key="l" class="flex items-center gap-2 text-[11px]">
          <span class="w-28 shrink-0 truncate font-semibold text-slate-600">{{ LANG_VI[l] }}</span>
          <span class="h-1.5 flex-1 overflow-hidden rounded-full bg-slate-100">
            <span class="block h-full rounded-full bg-indigo-500 transition-all" :style="{ width: `${Math.round((doneCount(l) / Math.max(1, rows.length)) * 100)}%` }" />
          </span>
          <CheckCircle2 v-if="doneCount(l) >= rows.length" class="h-3.5 w-3.5 shrink-0 text-emerald-600" />
          <span v-else class="w-12 shrink-0 text-right text-slate-400">{{ doneCount(l) }}/{{ rows.length }}</span>
        </div>
      </div>
      <label class="flex items-center gap-1.5 text-[11px] text-slate-500">
        <input v-model="redoAll" type="checkbox" class="accent-indigo-600" /> Dịch lại cả những câu đã có (ghi đè bản đã sửa)
      </label>
      <button
        type="button"
        class="flex w-full items-center justify-center gap-1.5 rounded-lg bg-indigo-600 py-2 text-xs font-bold text-white hover:bg-indigo-700 disabled:opacity-60"
        :disabled="running"
        @click="translateAll"
      >
        <Loader2 v-if="running" class="h-3.5 w-3.5 animate-spin" /><Sparkles v-else class="h-3.5 w-3.5" />
        {{ running ? 'Đang dịch...' : 'Dịch bằng AI' }}
      </button>
      <p v-if="progress" class="text-center text-[11px] text-indigo-700">{{ progress }}</p>
      <p v-if="errorMsg" class="rounded-lg bg-rose-50 px-2 py-1.5 text-[11px] text-rose-600">{{ errorMsg }}</p>
      <p v-if="warnMsg" class="rounded-lg bg-amber-50 px-2 py-1.5 text-[11px] text-amber-800">{{ warnMsg }}</p>
      <p class="text-[10px] leading-snug text-slate-400">Sửa chữ tiếng Việt hoặc thêm sản phẩm mới thì bấm dịch lại — chỉ câu mới được dịch. Nhớ bấm “Lưu nháp” / “Xuất bản”.</p>
    </div>

    <!-- Xem trước -->
    <label v-if="langs.length" class="block space-y-1">
      <span class="text-xs font-bold text-slate-600">Xem trước trang bằng</span>
      <select v-model="previewLang" class="w-full rounded-lg border border-slate-200 px-2.5 py-1.5 text-sm">
        <option value="vi">Tiếng Việt (đang chỉnh sửa)</option>
        <option v-for="l in langs" :key="l" :value="l">{{ LANG_NAMES[l] }} — {{ LANG_VI[l] }}</option>
      </select>
    </label>

    <!-- Sửa bản dịch -->
    <div v-if="langs.length" class="space-y-2">
      <label class="block space-y-1">
        <span class="text-xs font-bold text-slate-600">Sửa bản dịch</span>
        <select v-model="editLang" class="w-full rounded-lg border border-slate-200 px-2.5 py-1.5 text-sm">
          <option value="">— Chọn ngôn ngữ —</option>
          <option v-for="l in langs" :key="l" :value="l">{{ LANG_VI[l] }}</option>
        </select>
      </label>
      <template v-if="editLang">
        <div class="flex items-center gap-1.5 rounded-lg border border-slate-200 px-2">
          <Search class="h-3.5 w-3.5 text-slate-400" />
          <input v-model="query" placeholder="Tìm câu..." class="min-w-0 flex-1 py-1.5 text-xs outline-none" />
        </div>
        <div class="flex flex-wrap gap-1">
          <button
            v-for="g in ['all', 'Trang', 'Sản phẩm', 'Giao diện'] as const"
            :key="g"
            type="button"
            class="rounded-full border px-2 py-0.5 text-[10px] font-bold"
            :class="groupFilter === g ? 'border-indigo-600 bg-indigo-50 text-indigo-700' : 'border-slate-200 text-slate-500'"
            @click="groupFilter = g"
          >
            {{ g === 'all' ? 'Tất cả' : g }}
          </button>
          <label class="ml-auto flex items-center gap-1 text-[10px] text-slate-500"><input v-model="onlyMissing" type="checkbox" class="accent-indigo-600" /> Chưa dịch</label>
        </div>
        <div class="max-h-[50vh] space-y-2 overflow-y-auto pr-1">
          <div v-for="r in shown" :key="r.key" class="space-y-1 rounded-lg bg-slate-50 p-2">
            <div class="flex items-start gap-1.5 text-[11px] text-slate-500">
              <span class="shrink-0 rounded bg-white px-1 text-[9px] font-bold uppercase text-slate-400">{{ r.group }}</span>
              <span class="line-clamp-3">{{ r.src }}</span>
            </div>
            <textarea
              :value="getTr(editLang, r.key)"
              rows="2"
              class="w-full rounded-md border border-slate-200 bg-white px-2 py-1 text-xs focus:border-indigo-500 focus:outline-none"
              :class="getTr(editLang, r.key) ? '' : 'border-amber-300'"
              placeholder="Chưa dịch — đang hiện tiếng Việt"
              @change="onEdit(r.key, $event)"
            />
          </div>
          <p v-if="!shown.length" class="py-4 text-center text-[11px] text-slate-400">Không có câu nào.</p>
        </div>
      </template>
    </div>
  </div>
</template>
