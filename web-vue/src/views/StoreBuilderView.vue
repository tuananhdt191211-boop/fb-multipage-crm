<script setup lang="ts">
// Trình chỉnh sửa website kéo thả (kiểu Shopify):
// - Cột trái: thư viện section (kéo thả vào trang) / danh sách section / cài đặt chung
// - Giữa: trang thật, click để chọn, gõ chữ trực tiếp, kéo để đổi vị trí
// - Cột phải: cài đặt chi tiết của section đang chọn
import { computed, nextTick, onBeforeUnmount, onMounted, provide, ref, watch } from 'vue'
import { onBeforeRouteLeave, useRoute, useRouter } from 'vue-router'
import {
  ArrowLeft,
  Copy,
  ExternalLink,
  Eye,
  EyeOff,
  GripVertical,
  LayoutList,
  Loader2,
  Monitor,
  Languages,
  Palette,
  Plus,
  Redo2,
  Smartphone,
  Tablet,
  Trash2,
  Undo2,
  X
} from 'lucide-vue-next'
import { api } from '../lib/api'
import { blockCatalog, blockLabels, newBlock } from '../lib/blockFactory'
import StoreRenderer from '../components/store/StoreRenderer.vue'
import SectionSettings from '../components/store/SectionSettings.vue'
import TextStylePanel from '../components/store/TextStylePanel.vue'
import ImageField from '../components/store/ImageField.vue'
import type { AdminProduct, BlockType, PublicProduct, Site, SiteTheme, StoreBlock, TextFx } from '../types/store'
import { parseTheme } from '../lib/theme'
import { localizeBlocks, localizeProducts, parseI18n, previewLocale, type SiteI18n } from '../lib/i18n'
import LanguagePanel from '../components/store/LanguagePanel.vue'
import ThemeTypography from '../components/store/ThemeTypography.vue'

const route = useRoute()
const router = useRouter()
const siteId = computed(() => String(route.params.id || ''))

const loading = ref(true)
const saving = ref(false)
const publishing = ref(false)
const errorMsg = ref('')
const toast = ref<{ text: string; undo?: boolean } | null>(null)
let toastTimer: ReturnType<typeof setTimeout> | null = null

const site = ref<Site | null>(null)
const blocks = ref<StoreBlock[]>([])
// Kiểu chữ toàn trang (font, cỡ H1–P)
const theme = ref<SiteTheme>({})
const products = ref<PublicProduct[]>([])
const selectedId = ref<string | null>(null)
const leftTab = ref<'add' | 'sections' | 'theme' | 'lang'>('sections')

// Bán hàng quốc tế: thị trường + bản dịch (không đưa vào hoàn tác vì có thể rất lớn)
const i18n = ref<SiteI18n>({ markets: [], tr: {} })
const i18nDirty = ref(false)
const previewLang = ref('vi')
watch([previewLang, i18n], () => previewLocale(previewLang.value, i18n.value))
const previewing = computed(() => previewLang.value !== 'vi')
const canvasBlocks = computed(() => (previewing.value ? localizeBlocks(blocks.value, previewLang.value, i18n.value) : blocks.value))
const canvasProducts = computed(() => (previewing.value ? localizeProducts(products.value, previewLang.value, i18n.value) : products.value))
const device = ref<'desktop' | 'tablet' | 'mobile'>('desktop')

const deviceWidth = computed(() => (device.value === 'mobile' ? '390px' : device.value === 'tablet' ? '768px' : '100%'))
const selectedBlock = computed(() => blocks.value.find((b) => b.id === selectedId.value) || null)
const publicUrl = computed(() => (site.value ? `${window.location.origin}/s/${site.value.slug}` : ''))

const catalogGroups = computed(() => {
  const groups: { name: string; items: typeof blockCatalog }[] = []
  for (const m of blockCatalog) {
    let g = groups.find((x) => x.name === m.group)
    if (!g) groups.push((g = { name: m.group, items: [] }))
    g.items.push(m)
  }
  return groups
})

const colorPresets = ['#059669', '#2563eb', '#7c3aed', '#db2777', '#dc2626', '#ea580c', '#ca8a04', '#0f172a']

function showToast(text: string, undo = false) {
  toast.value = { text, undo }
  if (toastTimer) clearTimeout(toastTimer)
  toastTimer = setTimeout(() => (toast.value = null), undo ? 6000 : 2500)
}

function apiError(err: any, fallback: string) {
  return err?.response?.data?.error || fallback
}

// ---------------------------------------------------------------- Lưu / tải
function snapshot(): string {
  return JSON.stringify({
    t: site.value?.site_title,
    l: site.value?.logo_url,
    c: site.value?.primary_color,
    th: theme.value,
    b: blocks.value
  })
}
const savedSnapshot = ref('')
const dirty = computed(() => !!site.value && (currentSnap.value !== savedSnapshot.value || i18nDirty.value))
const currentSnap = ref('')

// Menu kiểu cũ (mảng chuỗi) -> {label, link}
function upgradeBlocks(arr: StoreBlock[]) {
  for (const b of arr) {
    if (!b.data) b.data = {}
    if (b.type === 'header' && Array.isArray(b.data.nav_items)) {
      b.data.nav_items = b.data.nav_items.map((x: any) => (typeof x === 'string' ? { label: x, link: '' } : x))
    }
  }
}

function applySite(s: Site) {
  site.value = s
  let parsed: StoreBlock[] = []
  try {
    const v = JSON.parse(s.draft_blocks_json || '[]')
    parsed = Array.isArray(v) ? v : []
  } catch {
    parsed = []
  }
  upgradeBlocks(parsed)
  blocks.value = parsed
  theme.value = parseTheme(s.draft_theme_json)
  i18n.value = parseI18n(s.draft_i18n_json)
  i18nDirty.value = false
  const snap = snapshot()
  savedSnapshot.value = snap
  currentSnap.value = snap
}

async function loadProducts() {
  try {
    const { data } = await api.get<{ products: AdminProduct[] }>(`/storefront/sites/${siteId.value}/products`)
    products.value = (data.products || [])
      .filter((p) => p.is_active)
      .map((p) => ({
        id: p.id,
        name: p.name,
        description: p.description,
        summary: p.summary || '',
        price: p.price,
        compare_price: p.compare_price,
        image_url: p.image_url,
        in_stock: p.stock !== 0
      }))
  } catch {
    products.value = []
  }
}

async function load() {
  loading.value = true
  errorMsg.value = ''
  try {
    const { data } = await api.get<{ site: Site }>(`/storefront/sites/${siteId.value}`)
    applySite(data.site)
    resetHistory()
    await loadProducts()
  } catch (err) {
    errorMsg.value = apiError(err, 'Không tải được website.')
  } finally {
    loading.value = false
  }
}

function cleanBlocks(): StoreBlock[] {
  const copy: StoreBlock[] = JSON.parse(JSON.stringify(blocks.value))
  for (const b of copy) {
    if (b.type === 'header' && Array.isArray(b.data.nav_items)) {
      b.data.nav_items = b.data.nav_items.filter((x: any) => String(x?.label || '').trim())
    }
    if (b.type === 'features' && Array.isArray(b.data.items)) {
      b.data.items = b.data.items.map((x: any) => String(x).trim()).filter(Boolean)
    }
  }
  return copy
}

async function saveDraft(silent = false): Promise<boolean> {
  if (!site.value || saving.value) return false
  saving.value = true
  errorMsg.value = ''
  const selected = selectedId.value
  try {
    const { data } = await api.put<{ site: Site }>(`/storefront/sites/${site.value.id}/draft`, {
      site_title: site.value.site_title,
      logo_url: site.value.logo_url,
      primary_color: site.value.primary_color,
      draft_blocks_json: JSON.stringify(cleanBlocks()),
      draft_theme_json: JSON.stringify(theme.value),
      draft_i18n_json: JSON.stringify(i18n.value)
    })
    applySite(data.site)
    resetHistory()
    selectedId.value = blocks.value.some((b) => b.id === selected) ? selected : null
    if (!silent) showToast('Đã lưu bản nháp')
    return true
  } catch (err) {
    errorMsg.value = apiError(err, 'Lưu bản nháp thất bại.')
    return false
  } finally {
    saving.value = false
  }
}

async function publish() {
  if (!site.value) return
  publishing.value = true
  try {
    if (!(await saveDraft(true))) return
    const { data } = await api.post<{ site: Site }>(`/storefront/sites/${site.value.id}/publish`)
    const selected = selectedId.value
    applySite(data.site)
    resetHistory()
    selectedId.value = selected
    showToast('Đã xuất bản. Khách hàng thấy bản mới ngay.')
  } catch (err) {
    errorMsg.value = apiError(err, 'Xuất bản thất bại.')
  } finally {
    publishing.value = false
  }
}

// ---------------------------------------------------------------- Hoàn tác / làm lại
const undoStack = ref<string[]>([])
const redoStack = ref<string[]>([])
let applyingHistory = false
let historyTimer: ReturnType<typeof setTimeout> | null = null

function resetHistory() {
  undoStack.value = [snapshot()]
  redoStack.value = []
}

function restore(snap: string) {
  applyingHistory = true
  const s = JSON.parse(snap)
  if (site.value) {
    site.value.site_title = s.t
    site.value.logo_url = s.l
    site.value.primary_color = s.c
  }
  theme.value = s.th || {}
  blocks.value = s.b
  if (selectedId.value && !blocks.value.some((b) => b.id === selectedId.value)) selectedId.value = null
  currentSnap.value = snap
  nextTick(() => (applyingHistory = false))
}

watch(
  [blocks, theme, () => site.value?.site_title, () => site.value?.logo_url, () => site.value?.primary_color],
  () => {
    if (!site.value) return
    currentSnap.value = snapshot()
    if (applyingHistory) return
    if (historyTimer) clearTimeout(historyTimer)
    historyTimer = setTimeout(() => {
      const snap = snapshot()
      if (undoStack.value[undoStack.value.length - 1] !== snap) {
        undoStack.value.push(snap)
        if (undoStack.value.length > 100) undoStack.value.shift()
        redoStack.value = []
      }
    }, 350)
  },
  { deep: true }
)

function flushHistory() {
  if (!historyTimer) return
  clearTimeout(historyTimer)
  historyTimer = null
  const snap = snapshot()
  if (undoStack.value[undoStack.value.length - 1] !== snap) {
    undoStack.value.push(snap)
    redoStack.value = []
  }
}

function undo() {
  flushHistory()
  if (undoStack.value.length < 2) return
  redoStack.value.push(undoStack.value.pop()!)
  restore(undoStack.value[undoStack.value.length - 1]!)
}

function redo() {
  if (!redoStack.value.length) return
  const snap = redoStack.value.pop()!
  undoStack.value.push(snap)
  restore(snap)
}

// ---------------------------------------------------------------- Thao tác section
function insertBlock(type: BlockType, index: number) {
  const b = newBlock(type)
  blocks.value.splice(Math.max(0, Math.min(index, blocks.value.length)), 0, b)
  selectedId.value = b.id
  nextTick(() => scrollToBlock(b.id))
}

function scrollToBlock(id: string) {
  const el = document.querySelector(`[data-block-id="${id}"]`)
  el?.scrollIntoView({ behavior: 'smooth', block: 'center' })
}

function selectBlock(id: string | null) {
  selectedId.value = id
  if (activeText.value && activeText.value.block !== id) activeText.value = null
}

// Bấm vào một đoạn chữ trên trang -> mở bảng "Kiểu chữ" cho đoạn đó
const activeText = ref<{ block: string; key: string; baseTag: string } | null>(null)
provide('ppTextFocus', (fx: TextFx, baseTag: string) => {
  selectedId.value = fx.block
  activeText.value = { block: fx.block, key: fx.key, baseTag }
})
// Nội dung hiện tại của đoạn chữ đang chọn (để xem trước code)
const activeTextValue = computed(() => {
  const t = activeText.value
  const b = t && blocks.value.find((x) => x.id === t.block)
  if (!t || !b) return ''
  let v: any = b.data
  for (const k of t.key.split('.')) v = v?.[/^\d+$/.test(k) ? Number(k) : k]
  if (v && typeof v === 'object') v = v.label
  return v === undefined || v === null ? '' : String(v)
})

function selectFromList(id: string) {
  selectedId.value = id
  scrollToBlock(id)
}

function onSectionAction(kind: 'up' | 'down' | 'duplicate' | 'hide' | 'delete', index: number) {
  const arr = blocks.value
  const b = arr[index]
  if (!b) return
  switch (kind) {
    case 'up':
    case 'down': {
      const j = kind === 'up' ? index - 1 : index + 1
      if (j < 0 || j >= arr.length) return
      ;[arr[index], arr[j]] = [arr[j]!, arr[index]!]
      nextTick(() => scrollToBlock(b.id))
      break
    }
    case 'duplicate': {
      const copy: StoreBlock = JSON.parse(JSON.stringify(b))
      copy.id = `${b.type}-${Date.now().toString(36)}${Math.random().toString(36).slice(2, 7)}`
      arr.splice(index + 1, 0, copy)
      selectedId.value = copy.id
      break
    }
    case 'hide':
      b.enabled = !b.enabled
      showToast(b.enabled ? 'Đã hiện section' : 'Đã ẩn section (khách sẽ không thấy)')
      break
    case 'delete':
      arr.splice(index, 1)
      if (selectedId.value === b.id) selectedId.value = null
      showToast(`Đã xoá "${blockLabels[b.type] || b.type}"`, true)
      break
  }
}

// Sửa chữ trực tiếp trên trang
function onInlineEdit(id: string, path: (string | number)[], value: string | boolean) {
  const b = blocks.value.find((x) => x.id === id)
  if (!b) return
  let target: any = b.data
  for (let i = 0; i < path.length - 1; i++) {
    const k = path[i]!
    if (target[k] === undefined || target[k] === null) target[k] = typeof path[i + 1] === 'number' ? [] : {}
    if (typeof target[k] === 'string' && i === path.length - 2) target[k] = { label: target[k] }
    target = target[k]
  }
  target[path[path.length - 1]!] = value
}

// ---------------------------------------------------------------- Kéo thả
type DragPayload = { kind: 'new'; type: BlockType } | { kind: 'move'; index: number }
const drag = ref<DragPayload | null>(null)
const dragActive = computed(() => drag.value !== null)

function startNewDrag(type: BlockType, e: DragEvent) {
  drag.value = { kind: 'new', type }
  e.dataTransfer?.setData('text/plain', 'pp-section')
  if (e.dataTransfer) e.dataTransfer.effectAllowed = 'copyMove'
}

function startMoveDrag(index: number, e: DragEvent) {
  drag.value = { kind: 'move', index }
  e.dataTransfer?.setData('text/plain', 'pp-section')
  if (e.dataTransfer) e.dataTransfer.effectAllowed = 'move'
  const el = document.querySelector(`[data-block-id="${blocks.value[index]?.id}"]`) as HTMLElement | null
  if (el && e.dataTransfer) e.dataTransfer.setDragImage(el, 20, 20)
}

function endDrag() {
  drag.value = null
  listDropIndex.value = null
}

function dropAt(index: number) {
  const p = drag.value
  endDrag()
  if (!p) return
  if (p.kind === 'new') {
    insertBlock(p.type, index)
    return
  }
  const from = p.index
  let to = index
  if (to > from) to--
  if (to === from) return
  const [item] = blocks.value.splice(from, 1)
  if (item) {
    blocks.value.splice(to, 0, item)
    selectedId.value = item.id
  }
}

// Danh sách section ở cột trái cũng kéo thả được
const listDropIndex = ref<number | null>(null)
function onListDragOver(i: number, e: DragEvent) {
  if (!drag.value) return
  e.preventDefault()
  const rect = (e.currentTarget as HTMLElement).getBoundingClientRect()
  listDropIndex.value = e.clientY < rect.top + rect.height / 2 ? i : i + 1
}
function onListDrop() {
  if (listDropIndex.value !== null) dropAt(listDropIndex.value)
}

// ---------------------------------------------------------------- Cửa sổ chọn section
const pickerAt = ref<number | null>(null)
function openPicker(index: number) {
  pickerAt.value = index
}
function pickType(type: BlockType) {
  const at = pickerAt.value ?? blocks.value.length
  pickerAt.value = null
  insertBlock(type, at)
}

// ---------------------------------------------------------------- Phím tắt & rời trang
function isTyping(t: EventTarget | null) {
  const el = t as HTMLElement | null
  return !!el && (el.isContentEditable || ['INPUT', 'TEXTAREA', 'SELECT'].includes(el.tagName))
}

function onKeydown(e: KeyboardEvent) {
  const mod = e.ctrlKey || e.metaKey
  if (mod && e.key.toLowerCase() === 's') {
    e.preventDefault()
    saveDraft()
    return
  }
  if (isTyping(e.target)) return
  if (mod && e.key.toLowerCase() === 'z' && !e.shiftKey) {
    e.preventDefault()
    undo()
  } else if (mod && (e.key.toLowerCase() === 'y' || (e.key.toLowerCase() === 'z' && e.shiftKey))) {
    e.preventDefault()
    redo()
  } else if (e.key === 'Escape') {
    selectedId.value = null
  }
}

function onBeforeUnload(e: BeforeUnloadEvent) {
  if (dirty.value) {
    e.preventDefault()
    e.returnValue = ''
  }
}

onBeforeRouteLeave(() => {
  if (dirty.value && !window.confirm('Bạn có thay đổi chưa lưu. Rời trang và bỏ các thay đổi?')) return false
})

onMounted(() => {
  load()
  window.addEventListener('keydown', onKeydown)
  window.addEventListener('beforeunload', onBeforeUnload)
})
onBeforeUnmount(() => {
  previewLocale('vi', { markets: [], tr: {} })
  window.removeEventListener('keydown', onKeydown)
  window.removeEventListener('beforeunload', onBeforeUnload)
})

// Chọn section thì tự mở bảng cài đặt; trên trang danh sách thì hiện "Section"
watch(selectedId, (id) => {
  if (id && leftTab.value === 'add') leftTab.value = 'sections'
})
</script>

<template>
  <div class="flex h-screen flex-col bg-slate-100 text-slate-900">
    <div v-if="loading" class="flex flex-1 items-center justify-center gap-2 text-sm text-slate-500">
      <Loader2 class="h-5 w-5 animate-spin text-indigo-600" /> Đang tải trình chỉnh sửa...
    </div>

    <div v-else-if="!site" class="mx-auto max-w-lg py-24 text-center">
      <p class="text-sm font-semibold text-rose-600">{{ errorMsg || 'Không tìm thấy website.' }}</p>
      <button class="mt-4 rounded-xl border bg-white px-4 py-2 text-sm font-bold" @click="router.push('/sites')">Về danh sách</button>
    </div>

    <template v-else>
      <!-- ================= Thanh trên cùng ================= -->
      <header class="flex shrink-0 flex-wrap items-center justify-between gap-2 border-b border-slate-200 bg-white px-3 py-2">
        <div class="flex min-w-0 items-center gap-2">
          <button class="rounded-lg border border-slate-200 p-1.5 hover:bg-slate-100" title="Về danh sách website" @click="router.push('/sites')">
            <ArrowLeft class="h-4 w-4" />
          </button>
          <div class="min-w-0">
            <div class="truncate text-sm font-extrabold">{{ site.site_title }}</div>
            <div class="text-[11px]" :class="dirty ? 'text-amber-600' : 'text-slate-400'">
              {{ saving ? 'Đang lưu...' : dirty ? 'Có thay đổi chưa lưu' : 'Đã lưu' }}
            </div>
          </div>
        </div>

        <div class="flex items-center gap-1 rounded-xl bg-slate-100 p-1">
          <button
            v-for="d in [{ v: 'desktop', i: Monitor, t: 'Máy tính' }, { v: 'tablet', i: Tablet, t: 'Máy tính bảng' }, { v: 'mobile', i: Smartphone, t: 'Điện thoại' }] as const"
            :key="d.v"
            class="rounded-lg p-1.5"
            :class="device === d.v ? 'bg-white text-indigo-700 shadow-sm' : 'text-slate-500 hover:text-slate-800'"
            :title="d.t"
            @click="device = d.v"
          >
            <component :is="d.i" class="h-4 w-4" />
          </button>
        </div>

        <div class="flex items-center gap-1.5">
          <button class="rounded-lg p-1.5 text-slate-600 hover:bg-slate-100 disabled:opacity-30" :disabled="undoStack.length < 2" title="Hoàn tác (Ctrl+Z)" @click="undo">
            <Undo2 class="h-4 w-4" />
          </button>
          <button class="rounded-lg p-1.5 text-slate-600 hover:bg-slate-100 disabled:opacity-30" :disabled="!redoStack.length" title="Làm lại (Ctrl+Y)" @click="redo">
            <Redo2 class="h-4 w-4" />
          </button>
          <a
            v-if="site.is_published"
            :href="publicUrl"
            target="_blank"
            rel="noopener"
            class="hidden items-center gap-1 rounded-lg px-2 py-1.5 text-xs font-bold text-slate-600 hover:bg-slate-100 sm:inline-flex"
          >
            Xem trang <ExternalLink class="h-3 w-3" />
          </a>
          <button
            class="rounded-xl border border-slate-200 bg-white px-3 py-1.5 text-xs font-bold hover:bg-slate-50 disabled:opacity-50"
            :disabled="saving || publishing || !dirty"
            title="Lưu nháp (Ctrl+S)"
            @click="saveDraft()"
          >
            <Loader2 v-if="saving && !publishing" class="mr-1 inline h-3.5 w-3.5 animate-spin" /> Lưu nháp
          </button>
          <button
            class="rounded-xl bg-indigo-600 px-3.5 py-1.5 text-xs font-bold text-white hover:bg-indigo-700 disabled:opacity-50"
            :disabled="saving || publishing"
            @click="publish"
          >
            <Loader2 v-if="publishing" class="mr-1 inline h-3.5 w-3.5 animate-spin" /> Xuất bản
          </button>
        </div>
      </header>

      <p v-if="errorMsg" class="flex items-center justify-between gap-2 bg-rose-50 px-4 py-2 text-xs font-semibold text-rose-700">
        {{ errorMsg }} <button class="rounded p-0.5 hover:bg-rose-100" @click="errorMsg = ''"><X class="h-3.5 w-3.5" /></button>
      </p>

      <div class="flex min-h-0 flex-1 flex-col lg:flex-row">
        <!-- ================= Cột trái ================= -->
        <aside class="flex max-h-[40vh] w-full shrink-0 flex-col border-b border-slate-200 bg-white lg:max-h-none lg:w-72 lg:border-b-0 lg:border-r">
          <div class="flex shrink-0 border-b border-slate-100 p-1.5">
            <button
              v-for="t in [{ v: 'sections', i: LayoutList, t: 'Section' }, { v: 'add', i: Plus, t: 'Thêm' }, { v: 'theme', i: Palette, t: 'Giao diện' }, { v: 'lang', i: Languages, t: 'Ngôn ngữ' }] as const"
              :key="t.v"
              class="flex flex-1 flex-col items-center justify-center gap-0.5 rounded-lg py-1.5 text-[11px] font-bold leading-none"
              :class="leftTab === t.v ? 'bg-indigo-50 text-indigo-700' : 'text-slate-500 hover:bg-slate-50'"
              @click="leftTab = t.v"
            >
              <component :is="t.i" class="h-3.5 w-3.5" /> {{ t.t }}
            </button>
          </div>

          <div class="min-h-0 flex-1 overflow-y-auto p-3">
            <!-- Danh sách section -->
            <template v-if="leftTab === 'sections'">
              <p v-if="!blocks.length" class="rounded-xl border border-dashed border-slate-300 p-4 text-center text-xs text-slate-500">
                Trang trống. Mở tab <b>Thêm</b> và kéo section vào trang.
              </p>
              <div class="space-y-1" @drop.prevent="onListDrop">
                <div
                  v-for="(b, i) in blocks"
                  :key="b.id"
                  class="relative"
                  @dragover="onListDragOver(i, $event)"
                >
                  <div v-if="dragActive && listDropIndex === i" class="absolute inset-x-0 -top-0.5 h-1 rounded bg-indigo-500" />
                  <div
                    class="group flex cursor-pointer items-center gap-1.5 rounded-lg border px-2 py-2 text-xs"
                    :class="[
                      selectedId === b.id ? 'border-indigo-500 bg-indigo-50' : 'border-transparent hover:bg-slate-50',
                      drag?.kind === 'move' && drag.index === i ? 'opacity-40' : '',
                      !b.enabled ? 'text-slate-400' : ''
                    ]"
                    @click="selectFromList(b.id)"
                  >
                    <span class="cursor-grab text-slate-300 group-hover:text-slate-500" draggable="true" @dragstart="startMoveDrag(i, $event)" @dragend="endDrag">
                      <GripVertical class="h-4 w-4" />
                    </span>
                    <span class="min-w-0 flex-1">
                      <span class="block truncate font-bold">{{ blockLabels[b.type] || b.type }}</span>
                      <span class="block truncate text-[11px] text-slate-400">{{ b.data.title || b.data.heading || b.data.brand_name || '' }}</span>
                    </span>
                    <button class="rounded p-1 opacity-0 hover:bg-slate-200 group-hover:opacity-100" :title="b.enabled ? 'Ẩn' : 'Hiện'" @click.stop="onSectionAction('hide', i)">
                      <component :is="b.enabled ? Eye : EyeOff" class="h-3.5 w-3.5" />
                    </button>
                    <button class="rounded p-1 opacity-0 hover:bg-slate-200 group-hover:opacity-100" title="Nhân đôi" @click.stop="onSectionAction('duplicate', i)">
                      <Copy class="h-3.5 w-3.5" />
                    </button>
                    <button class="rounded p-1 text-rose-600 opacity-0 hover:bg-rose-50 group-hover:opacity-100" title="Xoá" @click.stop="onSectionAction('delete', i)">
                      <Trash2 class="h-3.5 w-3.5" />
                    </button>
                  </div>
                  <div v-if="dragActive && listDropIndex === i + 1 && i === blocks.length - 1" class="absolute inset-x-0 -bottom-0.5 h-1 rounded bg-indigo-500" />
                </div>
              </div>
              <button
                class="mt-3 flex w-full items-center justify-center gap-1 rounded-xl border border-dashed border-slate-300 py-2 text-xs font-bold text-slate-600 hover:border-indigo-400 hover:text-indigo-700"
                @click="leftTab = 'add'"
              >
                <Plus class="h-3.5 w-3.5" /> Thêm section
              </button>
            </template>

            <!-- Thư viện section: kéo thả vào trang -->
            <template v-else-if="leftTab === 'lang'">
              <LanguagePanel v-model:i18n="i18n" v-model:preview="previewLang" :site-id="site.id" :blocks="blocks" :products="products" @changed="i18nDirty = true" />
            </template>

            <template v-else-if="leftTab === 'add'">
              <p class="mb-2 text-[11px] text-slate-500">Kéo section thả vào vị trí bất kỳ trên trang, hoặc bấm để thêm vào cuối.</p>
              <div v-for="g in catalogGroups" :key="g.name" class="mb-3 space-y-1.5">
                <div class="pt-1 text-[11px] font-extrabold uppercase tracking-wide text-slate-400">{{ g.name }}</div>
                <div
                  v-for="m in g.items"
                  :key="m.type"
                  draggable="true"
                  class="cursor-grab rounded-xl border border-slate-200 bg-white p-2.5 transition hover:border-indigo-400 hover:shadow-sm active:cursor-grabbing"
                  @dragstart="startNewDrag(m.type, $event)"
                  @dragend="endDrag"
                  @click="insertBlock(m.type, blocks.length)"
                >
                  <div class="flex items-center gap-1.5 text-xs font-extrabold">
                    <GripVertical class="h-3.5 w-3.5 text-slate-300" /> {{ m.label }}
                  </div>
                  <div class="mt-0.5 pl-5 text-[11px] text-slate-500">{{ m.desc }}</div>
                </div>
              </div>
            </template>

            <!-- Cài đặt chung -->
            <template v-else>
              <div class="space-y-4">
                <label class="block space-y-1">
                  <span class="text-xs font-bold text-slate-600">Tên website</span>
                  <input v-model="site.site_title" class="w-full rounded-lg border border-slate-200 px-2.5 py-1.5 text-sm" />
                </label>
                <ImageField v-model="site.logo_url" :site-id="site.id" label="Logo mặc định" compact />
                <div class="space-y-1.5">
                  <span class="block text-xs font-bold text-slate-600">Màu chủ đạo (nút, giá, nhãn)</span>
                  <div class="flex flex-wrap items-center gap-1.5">
                    <button
                      v-for="c in colorPresets"
                      :key="c"
                      class="h-7 w-7 rounded-full ring-offset-2"
                      :class="site.primary_color?.toLowerCase() === c ? 'ring-2 ring-indigo-500' : ''"
                      :style="{ backgroundColor: c }"
                      :title="c"
                      @click="site.primary_color = c"
                    />
                    <input v-model="site.primary_color" type="color" class="h-7 w-9 rounded border border-slate-200" title="Màu khác" />
                  </div>
                </div>
                <div class="border-t border-slate-100 pt-4">
                  <ThemeTypography v-model="theme" />
                </div>
                <div class="space-y-1 rounded-xl bg-slate-50 p-3 text-xs">
                  <div class="font-bold text-slate-600">Địa chỉ trang</div>
                  <a :href="publicUrl" target="_blank" rel="noopener" class="break-all text-indigo-700 hover:underline">{{ publicUrl }}</a>
                  <div class="text-slate-400">{{ site.is_published ? 'Đang xuất bản' : 'Chưa xuất bản' }}</div>
                </div>
                <router-link
                  :to="{ name: 'site-products', params: { id: site.id } }"
                  class="block rounded-xl border border-slate-200 px-3 py-2 text-center text-xs font-bold text-slate-700 hover:border-indigo-400 hover:text-indigo-700"
                >
                  Quản lý sản phẩm
                </router-link>
                <router-link
                  :to="{ name: 'site-orders', params: { id: site.id } }"
                  class="block rounded-xl border border-slate-200 px-3 py-2 text-center text-xs font-bold text-slate-700 hover:border-indigo-400 hover:text-indigo-700"
                >
                  Đơn hàng & thanh toán
                </router-link>
              </div>
            </template>
          </div>
        </aside>

        <!-- ================= Khung xem trước / chỉnh sửa ================= -->
        <div class="min-h-0 flex-1 overflow-auto p-3 sm:p-5" @click.self="selectBlock(null)">
          <div
            class="mx-auto min-h-full overflow-hidden rounded-xl bg-white shadow-sm ring-1 ring-slate-200 transition-[max-width] duration-300"
            :style="{ maxWidth: deviceWidth === '100%' ? '1200px' : deviceWidth }"
          >
            <div v-if="previewing" class="flex items-center justify-between gap-2 bg-indigo-600 px-4 py-2 text-xs font-semibold text-white">
              <span>Đang xem trước bản dịch — trang chỉ xem, không sửa trực tiếp. Sửa câu dịch ở tab Ngôn ngữ.</span>
              <button type="button" class="shrink-0 rounded-md bg-white/20 px-2 py-1 hover:bg-white/30" @click="previewLang = 'vi'">Về tiếng Việt</button>
            </div>
            <StoreRenderer
              :site-title="site.site_title"
              :primary-color="site.primary_color"
              :theme="theme"
              :logo-url="site.logo_url"
              :blocks="canvasBlocks"
              :products="canvasProducts"
              :editable="!previewing"
              :selected-id="selectedId"
              :drag-active="dragActive"
              @select="selectBlock"
              @edit="onInlineEdit"
              @action="onSectionAction"
              @drop="dropAt"
              @section-dragstart="startMoveDrag"
              @section-dragend="endDrag"
              @add-at="openPicker"
            />
          </div>
        </div>

        <!-- ================= Cột phải: cài đặt section ================= -->
        <aside class="w-full shrink-0 overflow-y-auto border-t border-slate-200 bg-white lg:w-80 lg:border-l lg:border-t-0">
          <template v-if="selectedBlock">
            <div class="sticky top-0 z-10 flex items-center justify-between border-b border-slate-100 bg-white px-4 py-3">
              <div>
                <div class="text-sm font-extrabold">{{ blockLabels[selectedBlock.type] || selectedBlock.type }}</div>
                <div class="text-[11px] text-slate-400">Bấm vào chữ trên trang để sửa trực tiếp</div>
              </div>
              <button class="rounded-lg p-1 hover:bg-slate-100" title="Bỏ chọn (Esc)" @click="selectBlock(null)"><X class="h-4 w-4" /></button>
            </div>
            <div class="space-y-4 p-4">
              <TextStylePanel
                v-if="activeText && activeText.block === selectedBlock.id"
                :key="activeText.key"
                :block="selectedBlock"
                :fx-key="activeText.key"
                :base-tag="activeText.baseTag"
                :text="activeTextValue"
                @close="activeText = null"
              />
              <p v-else class="rounded-lg bg-slate-50 px-3 py-2 text-[11px] text-slate-500">
                Mẹo: bấm vào một dòng chữ trên trang để chỉnh cỡ chữ, màu, thẻ H1/H2… và xem code CSS/HTML.
              </p>
              <SectionSettings :key="selectedBlock.id" :block="selectedBlock" :site-id="site.id" :site-logo="site.logo_url" :site-color="site.primary_color" />
            </div>
          </template>
          <div v-else class="space-y-3 p-5 text-center text-xs text-slate-500">
            <div class="mx-auto flex h-10 w-10 items-center justify-center rounded-full bg-indigo-50 text-indigo-600">
              <LayoutList class="h-5 w-5" />
            </div>
            <p class="font-bold text-slate-700">Chọn một section để chỉnh</p>
            <p>Bấm vào bất kỳ phần nào trên trang. Gõ chữ trực tiếp, kéo tay nắm <GripVertical class="inline h-3 w-3" /> để đổi vị trí.</p>
            <p class="text-[11px] text-slate-400">Phím tắt: Ctrl+S lưu · Ctrl+Z hoàn tác · Ctrl+Y làm lại · Esc bỏ chọn</p>
          </div>
        </aside>
      </div>
    </template>

    <!-- Cửa sổ chọn loại section (khi bấm "+ Thêm" trên trang) -->
    <div v-if="pickerAt !== null" class="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4" @click.self="pickerAt = null">
      <div class="w-full max-w-2xl space-y-4 rounded-2xl bg-white p-5">
        <div class="flex items-center justify-between">
          <h2 class="text-base font-extrabold">Thêm section</h2>
          <button class="rounded-lg p-1 hover:bg-slate-100" @click="pickerAt = null"><X class="h-4 w-4" /></button>
        </div>
        <div class="grid max-h-[60vh] grid-cols-1 gap-2.5 overflow-auto sm:grid-cols-2">
          <button
            v-for="m in blockCatalog"
            :key="m.type"
            class="rounded-xl border border-slate-200 p-3 text-left hover:border-indigo-500 hover:bg-indigo-50"
            @click="pickType(m.type)"
          >
            <div class="text-sm font-extrabold">{{ m.label }}</div>
            <div class="mt-0.5 text-xs text-slate-500">{{ m.desc }}</div>
          </button>
        </div>
      </div>
    </div>

    <!-- Thông báo nhỏ -->
    <div
      v-if="toast"
      class="fixed bottom-5 left-1/2 z-[80] flex -translate-x-1/2 items-center gap-3 rounded-xl bg-slate-900 px-4 py-2.5 text-xs font-semibold text-white shadow-xl"
    >
      {{ toast.text }}
      <button v-if="toast.undo" class="font-bold text-indigo-300 hover:text-indigo-200" @click="undo(); toast = null">Hoàn tác</button>
    </div>
  </div>
</template>
