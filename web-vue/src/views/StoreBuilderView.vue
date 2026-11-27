<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  ArrowDown,
  ArrowLeft,
  ArrowUp,
  ChevronDown,
  ChevronUp,
  Copy,
  ExternalLink,
  Eye,
  EyeOff,
  GripVertical,
  Loader2,
  Plus,
  Trash2,
  X
} from 'lucide-vue-next'
import { api } from '../lib/api'
import { blockCatalog, blockLabels, newBlock } from '../lib/blockFactory'
import StoreRenderer from '../components/store/StoreRenderer.vue'
import type { AdminProduct, BlockType, PublicProduct, Site, StoreBlock } from '../types/store'

const route = useRoute()
const router = useRouter()
const siteId = computed(() => String(route.params.id || ''))

const inp = 'w-full rounded-xl border border-slate-200 px-3 py-2 text-sm'
const inpSm = 'w-full rounded-lg border border-slate-200 px-2 py-1.5 text-sm'

const loading = ref(true)
const saving = ref(false)
const publishing = ref(false)
const errorMsg = ref('')
const okMsg = ref('')

const site = ref<Site | null>(null)
const blocks = ref<StoreBlock[]>([])
const products = ref<PublicProduct[]>([])

const expandedId = ref<string | null>(null)
const showPicker = ref(false)

// "Chưa lưu" = khác với bản đã tải/lưu gần nhất
const baseline = ref('')
function snapshot() {
  return JSON.stringify([site.value?.site_title, site.value?.logo_url, site.value?.primary_color, blocks.value])
}
const dirty = computed(() => !!site.value && snapshot() !== baseline.value)

function apiError(err: any, fallback: string) {
  return err?.response?.data?.error || fallback
}

// Menu kiểu cũ (mảng chuỗi) -> {label, link}
function upgradeBlocks(arr: StoreBlock[]) {
  for (const b of arr) {
    if (b.type === 'header' && Array.isArray(b.data.nav_items)) {
      b.data.nav_items = b.data.nav_items.map((x: any) => (typeof x === 'string' ? { label: x, link: '' } : x))
    }
  }
}

function applySite(s: Site) {
  site.value = s
  try {
    const parsed = JSON.parse(s.draft_blocks_json || '[]')
    blocks.value = Array.isArray(parsed) ? parsed : []
  } catch {
     const parsed = JSON.parse(s.draft_blocks_json || '[]')
    blocks.value = Array.isArray(parsed) ? parsed : []
    upgradeBlocks(blocks.value)
  }
  baseline.value = snapshot()
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
        if (Array.isArray(b.data.nav_items)) b.data.nav_items = b.data.nav_items.map((x: string) => x.trim()).filter(Boolean)

    if (b.type === 'features' && Array.isArray(b.data.items)) {
      b.data.items = b.data.items.map((x: string) => x.trim()).filter(Boolean)
    }
  }
  return copy
}

async function saveDraft(): Promise<boolean> {
  if (!site.value) return false
  saving.value = true
  errorMsg.value = ''
  okMsg.value = ''
  try {
    const { data } = await api.put<{ site: Site }>(`/storefront/sites/${site.value.id}/draft`, {
      site_title: site.value.site_title,
      logo_url: site.value.logo_url,
      primary_color: site.value.primary_color,
      draft_blocks_json: JSON.stringify(cleanBlocks())
    })
    applySite(data.site)
    okMsg.value = 'Đã lưu bản nháp.'
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
    if (!(await saveDraft())) return
    const { data } = await api.post<{ site: Site }>(`/storefront/sites/${site.value.id}/publish`)
    applySite(data.site)
    okMsg.value = 'Đã xuất bản. Khách hàng sẽ thấy bản mới ngay.'
  } catch (err) {
    errorMsg.value = apiError(err, 'Xuất bản thất bại.')
  } finally {
    publishing.value = false
  }
}

// ---------- Quản lý section ----------
function summaryOf(b: StoreBlock): string {
  return String(b.data.title || b.data.heading || b.data.brand_name || '').slice(0, 40)
}

function toggleExpand(id: string) {
  expandedId.value = expandedId.value === id ? null : id
}

function addBlock(type: BlockType) {
  const b = newBlock(type)
  blocks.value.push(b)
  expandedId.value = b.id
  showPicker.value = false
}

function duplicateBlock(i: number) {
  const src = blocks.value[i]
  if (!src) return
  const copy: StoreBlock = JSON.parse(JSON.stringify(src))
  copy.id = `${src.type}-${Date.now().toString(36)}${Math.random().toString(36).slice(2, 7)}`
  blocks.value.splice(i + 1, 0, copy)
  expandedId.value = copy.id
}

function removeBlock(i: number) {
  const b = blocks.value[i]
  if (!b) return
  if (!window.confirm(`Xóa section "${blockLabels[b.type] || b.type}"?`)) return
  blocks.value.splice(i, 1)
}

function move(i: number, dir: -1 | 1) {
  const j = i + dir
  if (j < 0 || j >= blocks.value.length) return
  const arr = blocks.value
  ;[arr[i], arr[j]] = [arr[j]!, arr[i]!]
}

// ---------- Kéo-thả (HTML5 native) ----------
const dragIndex = ref<number | null>(null)
const overIndex = ref<number | null>(null)

function onDragStart(i: number, ev: DragEvent) {
  dragIndex.value = i
  if (ev.dataTransfer) {
    ev.dataTransfer.effectAllowed = 'move'
    ev.dataTransfer.setData('text/plain', String(i))
  }
}
function onDragOver(i: number, ev: DragEvent) {
  if (dragIndex.value === null) return
  ev.preventDefault()
  overIndex.value = i
}
function onDrop(i: number) {
  const from = dragIndex.value
  dragIndex.value = null
  overIndex.value = null
  if (from === null || from === i) return
  const [item] = blocks.value.splice(from, 1)
  if (item) blocks.value.splice(i, 0, item)
}
function onDragEnd() {
  dragIndex.value = null
  overIndex.value = null
}

// ---------- Danh sách con (items) ----------
function addItem(b: StoreBlock, tpl: Record<string, any> | string) {
  if (!Array.isArray(b.data.items)) b.data.items = []
  b.data.items.push(typeof tpl === 'string' ? tpl : { ...tpl })
}
function removeAt(b: StoreBlock, idx: number) {
  b.data.items.splice(idx, 1)
}

function addNavItem(b: StoreBlock) {
  if (!Array.isArray(b.data.nav_items)) b.data.nav_items = []
  b.data.nav_items.push({ label: 'Mục mới', link: '' })
}
function moveItem(b: StoreBlock, idx: number, dir: -1 | 1) {
  const arr = b.data.nav_items
  const j = idx + dir
  if (!Array.isArray(arr) || j < 0 || j >= arr.length) return
  ;[arr[idx], arr[j]] = [arr[j], arr[idx]]
}

function addManualProduct(b: StoreBlock) {
  addItem(b, { id: `p${Date.now()}`, name: 'Sản phẩm mới', price: '0đ', image_url: '', desc: '' })
}

function linesOf(arr: unknown): string {
  return Array.isArray(arr) ? arr.join('\n') : ''
}
function setLines(b: StoreBlock, key: string, ev: Event) {
  b.data[key] = (ev.target as HTMLTextAreaElement).value.split('\n')
}

// ---------- Nền & khoảng cách của section ----------
function setStyle(b: StoreBlock, key: 'bg' | 'pad', val: string) {
  const st: Record<string, string> = { ...(b.data.style || {}) }
  if (!val || (key === 'pad' && val === 'none')) delete st[key]
  else st[key] = val
  b.data.style = st
}

const publicUrl = computed(() => (site.value ? `${window.location.origin}/s/${site.value.slug}` : ''))

onMounted(load)
</script>

<template>
  <div class="min-h-screen bg-slate-50 text-slate-900">
    <div v-if="loading" class="flex items-center justify-center gap-2 py-24 text-sm text-slate-500">
      <Loader2 class="h-5 w-5 animate-spin text-emerald-600" /> Đang tải...
    </div>

    <div v-else-if="!site" class="mx-auto max-w-lg py-24 text-center">
      <p class="text-sm font-semibold text-rose-600">{{ errorMsg || 'Không tìm thấy website.' }}</p>
      <button class="mt-4 rounded-xl border px-4 py-2 text-sm font-bold" @click="router.push('/sites')">Về danh sách</button>
    </div>

    <template v-else>
      <!-- Thanh trên cùng -->
      <div class="sticky top-0 z-20 flex flex-wrap items-center justify-between gap-3 border-b border-slate-200 bg-white px-4 py-3">
        <div class="flex items-center gap-3">
          <button class="rounded-xl border border-slate-200 p-2 hover:bg-slate-100" @click="router.push('/sites')">
            <ArrowLeft class="h-4 w-4" />
          </button>
          <div class="text-sm font-extrabold">{{ site.site_title }}</div>
          <span v-if="dirty" class="text-xs font-semibold text-amber-600">Chưa lưu</span>
        </div>
        <div class="flex items-center gap-2">
          <a
            v-if="site.is_published"
            :href="publicUrl"
            target="_blank"
            rel="noopener"
            class="inline-flex items-center gap-1 text-xs font-bold text-emerald-700 hover:underline"
          >
            Xem trang <ExternalLink class="h-3 w-3" />
          </a>
          <button
            class="rounded-xl border border-slate-200 px-4 py-2 text-xs font-bold hover:bg-slate-50 disabled:opacity-50"
            :disabled="saving || publishing"
            @click="saveDraft"
          >
            <Loader2 v-if="saving" class="mr-1 inline h-3.5 w-3.5 animate-spin" /> Lưu nháp
          </button>
          <button
            class="rounded-xl bg-emerald-600 px-4 py-2 text-xs font-bold text-white hover:bg-emerald-700 disabled:opacity-50"
            :disabled="saving || publishing"
            @click="publish"
          >
            <Loader2 v-if="publishing" class="mr-1 inline h-3.5 w-3.5 animate-spin" /> Xuất bản
          </button>
        </div>
      </div>

      <p v-if="errorMsg" class="mx-4 mt-3 rounded-xl bg-rose-50 px-4 py-2 text-sm font-semibold text-rose-600">{{ errorMsg }}</p>
      <p v-if="okMsg" class="mx-4 mt-3 rounded-xl bg-emerald-50 px-4 py-2 text-sm font-semibold text-emerald-700">{{ okMsg }}</p>

      <div class="grid grid-cols-1 gap-4 p-4 lg:grid-cols-[440px_1fr]">
        <!-- ===== Cột chỉnh sửa ===== -->
        <div class="space-y-3">
          <section class="space-y-3 rounded-2xl border border-slate-200 bg-white p-4">
            <h2 class="text-sm font-extrabold">Thông tin chung</h2>
            <label class="block space-y-1">
              <span class="text-xs font-bold text-slate-600">Tên website</span>
              <input v-model="site.site_title" :class="inp" />
            </label>
            <label class="block space-y-1">
              <span class="text-xs font-bold text-slate-600">Logo (URL ảnh)</span>
              <input v-model="site.logo_url" :class="inp" />
            </label>
            <label class="flex items-center gap-3">
              <span class="text-xs font-bold text-slate-600">Màu chủ đạo</span>
              <input v-model="site.primary_color" type="color" class="h-8 w-12 rounded border border-slate-200" />
              <span class="text-xs text-slate-400">{{ site.primary_color }}</span>
            </label>
          </section>

          <div class="flex items-center justify-between pt-1">
            <h2 class="text-sm font-extrabold">Các section ({{ blocks.length }})</h2>
            <button
              class="inline-flex items-center gap-1 rounded-xl bg-emerald-600 px-3 py-1.5 text-xs font-bold text-white hover:bg-emerald-700"
              @click="showPicker = true"
            >
              <Plus class="h-3.5 w-3.5" /> Thêm section
            </button>
          </div>

          <p v-if="!blocks.length" class="rounded-2xl border border-dashed border-slate-300 bg-white py-8 text-center text-sm text-slate-500">
            Trang đang trống. Bấm “Thêm section” để bắt đầu.
          </p>

          <section
            v-for="(b, i) in blocks"
            :key="b.id"
            class="rounded-2xl border bg-white transition-colors"
            :class="[
              overIndex === i && dragIndex !== null && dragIndex !== i ? 'border-emerald-500 ring-2 ring-emerald-200' : 'border-slate-200',
              dragIndex === i ? 'opacity-50' : '',
              !b.enabled ? 'opacity-70' : ''
            ]"
            @dragover="onDragOver(i, $event)"
            @drop.prevent="onDrop(i)"
          >
            <!-- Hàng tiêu đề (kéo bằng tay nắm) -->
            <div
              class="flex items-center gap-1 px-2 py-2"
              draggable="true"
              @dragstart="onDragStart(i, $event)"
              @dragend="onDragEnd"
            >
              <span class="cursor-grab rounded-lg p-1 text-slate-400 hover:bg-slate-100" title="Kéo để đổi vị trí">
                <GripVertical class="h-4 w-4" />
              </span>
              <button class="flex min-w-0 flex-1 items-center gap-2 text-left" @click="toggleExpand(b.id)">
                <span class="text-sm font-extrabold">{{ blockLabels[b.type] || b.type }}</span>
                <span class="truncate text-xs text-slate-400">{{ summaryOf(b) }}</span>
                <component :is="expandedId === b.id ? ChevronUp : ChevronDown" class="ml-auto h-4 w-4 shrink-0 text-slate-400" />
              </button>
              <button class="rounded-lg p-1 hover:bg-slate-100 disabled:opacity-30" :disabled="i === 0" title="Lên" @click="move(i, -1)">
                <ArrowUp class="h-4 w-4" />
              </button>
              <button class="rounded-lg p-1 hover:bg-slate-100 disabled:opacity-30" :disabled="i === blocks.length - 1" title="Xuống" @click="move(i, 1)">
                <ArrowDown class="h-4 w-4" />
              </button>
              <button class="rounded-lg p-1 hover:bg-slate-100" :title="b.enabled ? 'Ẩn' : 'Hiện'" @click="b.enabled = !b.enabled">
                <component :is="b.enabled ? Eye : EyeOff" class="h-4 w-4" />
              </button>
              <button class="rounded-lg p-1 hover:bg-slate-100" title="Nhân đôi" @click="duplicateBlock(i)">
                <Copy class="h-4 w-4" />
              </button>
              <button class="rounded-lg p-1 text-rose-600 hover:bg-rose-50" title="Xóa" @click="removeBlock(i)">
                <Trash2 class="h-4 w-4" />
              </button>
            </div>

            <!-- Nội dung chỉnh sửa -->
            <div v-if="expandedId === b.id" class="space-y-3 border-t border-slate-100 p-4">
              <!-- header -->
                           <template v-if="b.type === 'header'">
                <!-- Logo -->
                <div class="space-y-2">
                  <span class="text-xs font-bold text-slate-600">Logo</span>
                  <input v-model="b.data.logo_url" placeholder="URL ảnh logo (để trống = dùng logo ở Thông tin chung)" :class="inp" />
                  <label class="flex items-center gap-3 text-xs font-bold text-slate-600">
                    Chiều cao logo
                    <input v-model.number="b.data.logo_height" type="range" min="20" max="120" step="2" class="flex-1" />
                    <span class="w-12 text-right font-normal text-slate-500">{{ b.data.logo_height || 40 }}px</span>
                  </label>
                </div>

                <!-- Tên chữ -->
                <input v-model="b.data.brand_name" placeholder="Tên thương hiệu (chữ)" :class="inp" />
                <div class="flex flex-wrap items-center gap-4">
                  <label class="flex items-center gap-2 text-xs font-bold text-slate-600">
                    <input type="checkbox" :checked="b.data.show_brand !== false" @change="b.data.show_brand = ($event.target as HTMLInputElement).checked" />
                    Hiện tên chữ
                  </label>
                  <label class="flex items-center gap-2 text-xs font-bold text-slate-600">
                    Cỡ chữ
                    <select v-model="b.data.brand_size" class="rounded-lg border border-slate-200 px-2 py-1 text-sm">
                      <option value="sm">Nhỏ</option>
                      <option value="md">Vừa</option>
                      <option value="lg">Lớn</option>
                    </select>
                  </label>
                </div>

                <!-- Bố cục -->
                <label class="block space-y-1">
                  <span class="text-xs font-bold text-slate-600">Bố cục</span>
                  <select v-model="b.data.layout" :class="inp">
                    <option value="left">Logo trái · Menu và nút bên phải</option>
                    <option value="split">Logo trái · Menu giữa · Nút phải</option>
                    <option value="center">Logo giữa · Menu bên dưới</option>
                    <option value="center_split">Menu trái · Logo giữa · Nút phải</option>
                  </select>
                </label>

                <!-- Menu -->
                <div class="space-y-2">
                  <span class="text-xs font-bold text-slate-600">Menu</span>
                  <div v-for="(n, ni) in b.data.nav_items || []" :key="ni" class="flex items-center gap-1">
                    <input v-model="n.label" placeholder="Chữ" class="w-1/3 rounded-lg border border-slate-200 px-2 py-1.5 text-sm" />
                    <input v-model="n.link" placeholder="#products, /s/..., https://..." class="min-w-0 flex-1 rounded-lg border border-slate-200 px-2 py-1.5 text-sm" />
                    <button class="rounded-lg p-1 hover:bg-slate-100 disabled:opacity-30" :disabled="ni === 0" @click="moveItem(b, Number(ni), -1)"><ArrowUp class="h-4 w-4" /></button>
                    <button class="rounded-lg p-1 hover:bg-slate-100 disabled:opacity-30" :disabled="ni === (b.data.nav_items || []).length - 1" @click="moveItem(b, Number(ni), 1)"><ArrowDown class="h-4 w-4" /></button>
                    <button class="rounded-lg p-1 text-rose-600 hover:bg-rose-50" @click="removeAt(b, Number(ni))"><Trash2 class="h-4 w-4" /></button>
                  </div>
                  <button class="inline-flex items-center gap-1 text-xs font-bold text-emerald-700" @click="addNavItem(b)">
                    <Plus class="h-3.5 w-3.5" /> Thêm mục menu
                  </button>
                </div>

                <!-- Nút -->
                <label class="flex items-center gap-2 text-xs font-bold text-slate-600">
                  <input type="checkbox" :checked="b.data.show_cta !== false" @change="b.data.show_cta = ($event.target as HTMLInputElement).checked" />
                  Hiện nút
                </label>
                <div v-if="b.data.show_cta !== false" class="grid grid-cols-2 gap-2">
                  <input v-model="b.data.cta_label" placeholder="Chữ trên nút" :class="inp" />
                  <input v-model="b.data.cta_link" placeholder="Liên kết nút" :class="inp" />
                </div>

                <!-- Hiển thị -->
                <div class="flex flex-wrap items-center gap-4 border-t border-slate-100 pt-3">
                  <label class="flex items-center gap-2 text-xs font-bold text-slate-600">
                    <input v-model="b.data.sticky" type="checkbox" /> Ghim khi cuộn
                  </label>
                  <label class="flex items-center gap-2 text-xs font-bold text-slate-600">
                    <input type="checkbox" :checked="b.data.border !== false" @change="b.data.border = ($event.target as HTMLInputElement).checked" />
                    Đường viền dưới
                  </label>
                  <label class="flex items-center gap-2 text-xs font-bold text-slate-600">
                    Màu chữ
                    <input
                      type="color"
                      :value="b.data.text_color || '#0f172a'"
                      class="h-7 w-10 rounded border border-slate-200"
                      @input="b.data.text_color = ($event.target as HTMLInputElement).value"
                    />
                  </label>
                  <button v-if="b.data.text_color" class="text-xs text-slate-500 underline" @click="b.data.text_color = ''">Màu mặc định</button>
                </div>
              </template>

              <!-- hero -->
              <template v-else-if="b.type === 'hero'">
                <input v-model="b.data.badge" placeholder="Nhãn nhỏ" :class="inp" />
                <input v-model="b.data.title" placeholder="Tiêu đề" :class="inp" />
                <textarea v-model="b.data.subtitle" rows="3" placeholder="Mô tả ngắn" :class="inp" />
                <input v-model="b.data.image_url" placeholder="URL ảnh banner" :class="inp" />
                <div class="grid grid-cols-2 gap-2">
                  <input v-model="b.data.cta_label" placeholder="Chữ trên nút" :class="inp" />
                  <input v-model="b.data.cta_link" placeholder="Liên kết nút" :class="inp" />
                </div>
              </template>

              <!-- products -->
              <template v-else-if="b.type === 'products'">
                <input v-model="b.data.heading" placeholder="Tiêu đề mục" :class="inp" />
                <label class="block space-y-1">
                  <span class="text-xs font-bold text-slate-600">Nguồn sản phẩm</span>
                  <select v-model="b.data.source" :class="inp">
                    <option value="catalog">Lấy từ danh sách sản phẩm của shop</option>
                    <option value="manual">Nhập tay trong khối này</option>
                  </select>
                </label>
                <p v-if="b.data.source === 'catalog'" class="text-xs text-slate-500">
                  Sản phẩm được quản lý ở trang riêng.
                  <router-link :to="{ name: 'site-products', params: { id: site.id } }" class="font-bold text-emerald-700 hover:underline">
                    Quản lý sản phẩm
                  </router-link>
                </p>
                <template v-else>
                  <div v-for="(p, pi) in b.data.items || []" :key="p.id || pi" class="space-y-2 rounded-xl border border-slate-100 bg-slate-50 p-3">
                    <div class="flex gap-2">
                      <input v-model="p.name" placeholder="Tên sản phẩm" :class="inpSm" />
                      <input v-model="p.price" placeholder="Giá" class="w-28 rounded-lg border border-slate-200 px-2 py-1.5 text-sm" />
                      <button class="rounded-lg p-1.5 text-rose-600 hover:bg-rose-50" @click="removeAt(b, Number(pi))"><Trash2 class="h-4 w-4" /></button>
                    </div>
                    <input v-model="p.image_url" placeholder="URL ảnh" :class="inpSm" />
                    <textarea v-model="p.desc" rows="2" placeholder="Mô tả" :class="inpSm" />
                  </div>
                  <button class="inline-flex items-center gap-1 text-xs font-bold text-emerald-700" @click="addManualProduct(b)">
                    <Plus class="h-3.5 w-3.5" /> Thêm sản phẩm
                  </button>
                </template>
              </template>

              <!-- features -->
              <template v-else-if="b.type === 'features'">
                <input v-model="b.data.heading" placeholder="Tiêu đề mục" :class="inp" />
                <textarea :value="linesOf(b.data.items)" rows="4" placeholder="Mỗi dòng một cam kết" :class="inp" @input="setLines(b, 'items', $event)" />
              </template>

              <!-- testimonials -->
              <template v-else-if="b.type === 'testimonials'">
                <input v-model="b.data.heading" placeholder="Tiêu đề mục" :class="inp" />
                <div v-for="(t, ti) in b.data.items || []" :key="ti" class="space-y-2 rounded-xl border border-slate-100 bg-slate-50 p-3">
                  <div class="flex gap-2">
                    <input v-model="t.name" placeholder="Tên khách" :class="inpSm" />
                    <button class="rounded-lg p-1.5 text-rose-600 hover:bg-rose-50" @click="removeAt(b, Number(ti))"><Trash2 class="h-4 w-4" /></button>
                  </div>
                  <textarea v-model="t.quote" rows="2" placeholder="Nội dung đánh giá" :class="inpSm" />
                </div>
                <button class="inline-flex items-center gap-1 text-xs font-bold text-emerald-700" @click="addItem(b, { name: '', quote: '' })">
                  <Plus class="h-3.5 w-3.5" /> Thêm đánh giá
                </button>
              </template>

              <!-- cta -->
              <template v-else-if="b.type === 'cta'">
                <input v-model="b.data.title" placeholder="Tiêu đề" :class="inp" />
                <textarea v-model="b.data.subtitle" rows="2" placeholder="Mô tả" :class="inp" />
                <div class="grid grid-cols-2 gap-2">
                  <input v-model="b.data.button_text" placeholder="Chữ trên nút" :class="inp" />
                  <input v-model="b.data.button_link" placeholder="Liên kết (tel:, mailto:, https://...)" :class="inp" />
                </div>
                <input v-model="b.data.footer_note" placeholder="Ghi chú chân trang" :class="inp" />
              </template>

              <!-- richtext -->
              <template v-else-if="b.type === 'richtext'">
                <input v-model="b.data.heading" placeholder="Tiêu đề (tùy chọn)" :class="inp" />
                <textarea v-model="b.data.body" rows="6" placeholder="Nội dung (xuống dòng để tách đoạn)" :class="inp" />
                <label class="flex items-center gap-2 text-xs font-bold text-slate-600">
                  Căn lề
                  <select v-model="b.data.align" class="rounded-lg border border-slate-200 px-2 py-1 text-sm">
                    <option value="left">Trái</option>
                    <option value="center">Giữa</option>
                    <option value="right">Phải</option>
                  </select>
                </label>
              </template>

              <!-- image -->
              <template v-else-if="b.type === 'image'">
                <input v-model="b.data.image_url" placeholder="URL ảnh" :class="inp" />
                <input v-model="b.data.alt" placeholder="Mô tả ảnh (cho người khiếm thị và SEO)" :class="inp" />
                <input v-model="b.data.link" placeholder="Liên kết khi bấm vào ảnh (tùy chọn)" :class="inp" />
              </template>

              <!-- gallery -->
              <template v-else-if="b.type === 'gallery'">
                <input v-model="b.data.heading" placeholder="Tiêu đề mục" :class="inp" />
                <div v-for="(g, gi) in b.data.items || []" :key="gi" class="space-y-2 rounded-xl border border-slate-100 bg-slate-50 p-3">
                  <div class="flex gap-2">
                    <input v-model="g.image_url" placeholder="URL ảnh" :class="inpSm" />
                    <button class="rounded-lg p-1.5 text-rose-600 hover:bg-rose-50" @click="removeAt(b, Number(gi))"><Trash2 class="h-4 w-4" /></button>
                  </div>
                  <input v-model="g.caption" placeholder="Chú thích (tùy chọn)" :class="inpSm" />
                </div>
                <button class="inline-flex items-center gap-1 text-xs font-bold text-emerald-700" @click="addItem(b, { image_url: '', caption: '' })">
                  <Plus class="h-3.5 w-3.5" /> Thêm ảnh
                </button>
              </template>

              <!-- faq -->
              <template v-else-if="b.type === 'faq'">
                <input v-model="b.data.heading" placeholder="Tiêu đề mục" :class="inp" />
                <div v-for="(f, fi) in b.data.items || []" :key="fi" class="space-y-2 rounded-xl border border-slate-100 bg-slate-50 p-3">
                  <div class="flex gap-2">
                    <input v-model="f.q" placeholder="Câu hỏi" :class="inpSm" />
                    <button class="rounded-lg p-1.5 text-rose-600 hover:bg-rose-50" @click="removeAt(b, Number(fi))"><Trash2 class="h-4 w-4" /></button>
                  </div>
                  <textarea v-model="f.a" rows="2" placeholder="Câu trả lời" :class="inpSm" />
                </div>
                <button class="inline-flex items-center gap-1 text-xs font-bold text-emerald-700" @click="addItem(b, { q: '', a: '' })">
                  <Plus class="h-3.5 w-3.5" /> Thêm câu hỏi
                </button>
              </template>

              <!-- contact -->
              <template v-else-if="b.type === 'contact'">
                <input v-model="b.data.heading" placeholder="Tiêu đề mục" :class="inp" />
                <input v-model="b.data.phone" placeholder="Số điện thoại" :class="inp" />
                <input v-model="b.data.email" placeholder="Email" :class="inp" />
                <input v-model="b.data.address" placeholder="Địa chỉ" :class="inp" />
                <input v-model="b.data.zalo_link" placeholder="Link Zalo (https://zalo.me/...)" :class="inp" />
              </template>

              <!-- html -->
              <template v-else-if="b.type === 'html'">
                <textarea v-model="b.data.html" rows="10" spellcheck="false" placeholder="<div>...</div>" class="w-full rounded-xl border border-slate-200 px-3 py-2 font-mono text-xs" />
                <label class="flex items-center gap-2 text-xs font-bold text-slate-600">
                  Chiều cao (px)
                  <input v-model.number="b.data.height" type="number" min="80" max="2000" class="w-24 rounded-lg border border-slate-200 px-2 py-1 text-sm" />
                </label>
                <p class="rounded-lg bg-amber-50 px-3 py-2 text-xs text-amber-800">
                  Mã chạy trong khung cách ly: không đọc được dữ liệu đăng nhập hay chuyển hướng trang chính.
                  Không đưa thông tin nhạy cảm của khách vào khối này.
                </p>
              </template>

              <!-- Nền & khoảng cách -->
              <div class="flex flex-wrap items-center gap-4 border-t border-slate-100 pt-3">
                <label class="flex items-center gap-2 text-xs font-bold text-slate-600">
                  Màu nền
                  <input
                    type="color"
                    :value="b.data.style?.bg || '#ffffff'"
                    class="h-7 w-10 rounded border border-slate-200"
                    @input="setStyle(b, 'bg', ($event.target as HTMLInputElement).value)"
                  />
                </label>
                <button v-if="b.data.style?.bg" class="text-xs text-slate-500 underline" @click="setStyle(b, 'bg', '')">Bỏ màu nền</button>
                <label class="flex items-center gap-2 text-xs font-bold text-slate-600">
                  Khoảng cách thêm
                  <select
                    :value="b.data.style?.pad || 'none'"
                    class="rounded-lg border border-slate-200 px-2 py-1 text-sm"
                    @change="setStyle(b, 'pad', ($event.target as HTMLSelectElement).value)"
                  >
                    <option value="none">Mặc định</option>
                    <option value="sm">Nhỏ</option>
                    <option value="md">Vừa</option>
                    <option value="lg">Lớn</option>
                  </select>
                </label>
              </div>
            </div>
          </section>
        </div>

        <!-- ===== Xem trước ===== -->
        <div class="lg:sticky lg:top-20 lg:h-[calc(100vh-6rem)] overflow-auto rounded-2xl border border-slate-200 bg-white">
          <div class="border-b border-slate-100 px-4 py-2 text-xs font-bold text-slate-400">Xem trước (bản nháp)</div>
          <StoreRenderer :site-title="site.site_title" :primary-color="site.primary_color" :blocks="blocks" :products="products" />
        </div>
      </div>
    </template>

    <!-- Cửa sổ chọn loại section -->
    <div v-if="showPicker" class="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4" @click.self="showPicker = false">
      <div class="w-full max-w-2xl space-y-4 rounded-2xl bg-white p-6">
        <div class="flex items-center justify-between">
          <h2 class="text-base font-extrabold">Thêm section</h2>
          <button class="rounded-lg p-1 hover:bg-slate-100" @click="showPicker = false"><X class="h-4 w-4" /></button>
        </div>
        <div class="grid max-h-[60vh] grid-cols-1 gap-3 overflow-auto sm:grid-cols-2">
          <button
            v-for="m in blockCatalog"
            :key="m.type"
            class="rounded-xl border border-slate-200 p-3 text-left hover:border-emerald-500 hover:bg-emerald-50"
            @click="addBlock(m.type)"
          >
            <div class="text-sm font-extrabold">{{ m.label }}</div>
            <div class="mt-0.5 text-xs text-slate-500">{{ m.desc }}</div>
          </button>
        </div>
      </div>
    </div>
  </div>
</template>