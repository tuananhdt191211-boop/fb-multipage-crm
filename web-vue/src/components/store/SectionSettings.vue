<script setup lang="ts">
// Bảng cài đặt cho section đang chọn (cột phải của trình chỉnh sửa)
import { computed, ref } from 'vue'
import { GripVertical, Plus, Trash2 } from 'lucide-vue-next'
import ImageField from './ImageField.vue'
import { iconChoices } from '../../lib/blockFactory'
import { menuById, newMenuId, type SiteMenu } from '../../lib/menus'
import type { PublicProduct, StoreBlock } from '../../types/store'

const props = defineProps<{ block: StoreBlock; siteId: number | string; siteLogo?: string; siteColor?: string; products?: PublicProduct[]; menus?: SiteMenu[] }>()

// ----- Menu quản lý riêng: đầu trang / cột chân trang chọn menu thay vì tự nhập danh sách -----
const menuOf = (id: unknown) => menuById(props.menus, id)
function uniqueMenuName(base: string) {
  let name = base
  for (let i = 2; (props.menus || []).some((m) => m.name === name); i++) name = base + ' ' + i
  return name
}
// Tạo menu từ danh sách tự nhập hiện có rồi chuyển section sang dùng menu đó
function navToMenu() {
  if (!props.menus) return
  const items = (Array.isArray(d().nav_items) ? d().nav_items : [])
    .map((n: any) => ({ label: String(n?.label ?? n ?? '').trim(), link: String(n?.link || '') }))
    .filter((n: { label: string }) => n.label)
  const id = newMenuId()
  props.menus.push({ id, name: uniqueMenuName('Menu chính'), items })
  d().menu_id = id
}
function colToMenu(col: any) {
  if (!props.menus) return
  const items = (col.links || []).map((l: any) => ({ label: String(l?.label || '').trim(), link: String(l?.link || '') })).filter((l: { label: string }) => l.label)
  const id = newMenuId()
  props.menus.push({ id, name: uniqueMenuName(String(col.title || 'Menu chân trang')), items })
  col.menu_id = id
}

// ----- Sản phẩm: chọn thẻ để chỉ hiện các sản phẩm có thẻ đó trong section -----
const tagKey = (s: string) => s.trim().toLowerCase()
const selectedTags = computed<string[]>(() => (Array.isArray(d().tags) ? (d().tags as string[]) : []))
// Mọi thẻ đang dùng ở các sản phẩm đang bán (kèm số sản phẩm); thẻ đã chọn mà không còn sản phẩm nào vẫn hiện để bỏ chọn
const tagChoices = computed(() => {
  const m = new Map<string, { name: string; count: number }>()
  for (const p of props.products || []) {
    for (const t of p.tags || []) {
      const e = m.get(tagKey(t))
      if (e) e.count++
      else m.set(tagKey(t), { name: t, count: 1 })
    }
  }
  for (const t of selectedTags.value) if (!m.has(tagKey(t))) m.set(tagKey(t), { name: t, count: 0 })
  return [...m.values()].sort((a, b) => b.count - a.count || a.name.localeCompare(b.name))
})
const tagOn = (name: string) => selectedTags.value.some((t) => tagKey(t) === tagKey(name))
function toggleTag(name: string) {
  d().tags = tagOn(name) ? selectedTags.value.filter((t) => tagKey(t) !== tagKey(name)) : [...selectedTags.value, name]
}
const matchingCount = computed(() => {
  const want = new Set(selectedTags.value.map(tagKey))
  if (!want.size) return (props.products || []).length
  return (props.products || []).filter((p) => (p.tags || []).some((t) => want.has(tagKey(t)))).length
})

const inp = 'w-full rounded-lg border border-slate-200 bg-white px-2.5 py-1.5 text-sm focus:border-indigo-500 focus:outline-none'
const lbl = 'block text-xs font-bold text-slate-600'

const d = () => props.block.data

// ----- Đầu trang: hiện logo, tên chữ hay cả hai -----
const hasLogo = computed(() => !!String(d().logo_url || props.siteLogo || '').trim())
const brandModes = [
  { v: 'both', t: 'Logo + tên' },
  { v: 'logo', t: 'Chỉ logo' },
  { v: 'name', t: 'Chỉ tên' }
] as const
type BrandMode = (typeof brandModes)[number]['v']
const brandMode = computed<BrandMode>(() => {
  if (!hasLogo.value || d().show_logo === false) return 'name'
  return d().show_brand === false ? 'logo' : 'both'
})
function setBrandMode(m: BrandMode) {
  d().show_logo = m !== 'name'
  d().show_brand = m !== 'logo'
}

function list(key: string): any[] {
  if (!Array.isArray(d()[key])) d()[key] = []
  return d()[key]
}
function addTo(key: string, item: any) {
  list(key).push(typeof item === 'object' ? { ...item } : item)
}
function removeFrom(key: string, i: number) {
  list(key).splice(i, 1)
}

// Kéo thả để đổi thứ tự phần tử trong danh sách
const dragKey = ref<string | null>(null)
const dragFrom = ref<number | null>(null)
const dragOver = ref<number | null>(null)
function itemDragStart(key: string, i: number, e: DragEvent) {
  dragKey.value = key
  dragFrom.value = i
  e.dataTransfer?.setData('text/x-pp-item', String(i))
  if (e.dataTransfer) e.dataTransfer.effectAllowed = 'move'
}
function itemDragOver(key: string, i: number, e: DragEvent) {
  if (dragKey.value !== key) return
  e.preventDefault()
  dragOver.value = i
}
function itemDrop(key: string, i: number) {
  const from = dragFrom.value
  if (dragKey.value === key && from !== null && from !== i) {
    const arr = list(key)
    const [it] = arr.splice(from, 1)
    arr.splice(i, 0, it)
  }
  itemDragEnd()
}
function itemDragEnd() {
  dragKey.value = null
  dragFrom.value = null
  dragOver.value = null
}
function rowClass(key: string, i: number) {
  return [
    'rounded-xl border bg-slate-50 p-2.5 transition',
    dragKey.value === key && dragOver.value === i && dragFrom.value !== i ? 'border-indigo-500 ring-2 ring-indigo-100' : 'border-slate-200',
    dragKey.value === key && dragFrom.value === i ? 'opacity-40' : ''
  ]
}

// ----- Đầu trang: kiểu có sẵn -----
// Màu chủ đạo của website làm đậm (trộn với đen) -> nền đầu trang theo thương hiệu của chính shop
function darken(hex: string | undefined, keep = 0.35): string {
  const m = /^#?([0-9a-f]{6})$/i.exec(String(hex || '').trim())
  if (!m) return '#1e293b'
  const n = parseInt(m[1]!, 16)
  const ch = (shift: number) => Math.round(((n >> shift) & 255) * keep).toString(16).padStart(2, '0')
  return `#${ch(16)}${ch(8)}${ch(0)}`
}

const headerPresets = computed(() => [
  { key: 'brand' as const, name: 'Tối giản · màu thương hiệu', bg: darken(props.siteColor), fg: '#ffffff' },
  { key: 'black' as const, name: 'Tối giản · đen', bg: '#111111', fg: '#ffffff' },
  { key: 'light' as const, name: 'Tối giản · trắng', bg: '#ffffff', fg: '#0f172a' },
  { key: 'classic' as const, name: 'Cổ điển · nút đặt mua', bg: '#f8fafc', fg: '#334155' }
])

function applyHeaderPreset(key: 'brand' | 'black' | 'light' | 'classic') {
  const p = headerPresets.value.find((x) => x.key === key)!
  const data = d()
  const st: Record<string, string> = { ...(data.style || {}) }
  if (key === 'classic') {
    Object.assign(data, { layout: 'left', right_mode: 'cta', show_cta: true, border: true, text_color: '' })
    delete st.bg
  } else {
    // Logo + menu sát logo bên trái, biểu tượng 🔍 👤 🛍 bên phải
    Object.assign(data, { layout: 'left_menu', right_mode: 'account', account_style: 'icon', show_search: true, sticky: true })
    if (key === 'light') {
      Object.assign(data, { border: true, text_color: '' })
      delete st.bg
    } else {
      Object.assign(data, { border: false, text_color: p.fg })
      st.bg = p.bg
    }
    // Có logo thì chỉ hiện logo cho gọn
    if (hasLogo.value) {
      data.show_logo = true
      data.show_brand = false
    }
  }
  data.style = st
}

// Màu nền gợi ý cho đầu trang (có màu thương hiệu của shop)
const headerBgSwatches = computed(() => [...new Set(['#ffffff', '#f8fafc', '#0f172a', '#111111', darken(props.siteColor), String(props.siteColor || '#059669')])])

// ----- Banner chữ + ảnh: chiều cao -----
const heroHeights = [
  { v: 0, t: 'Tự động' },
  { v: 320, t: 'Thấp' },
  { v: 480, t: 'Vừa' },
  { v: 640, t: 'Cao' },
  { v: 800, t: 'Rất cao' }
]

// ----- Sản phẩm: kiểu hiển thị -----
type ProductLayout = 'grid' | 'slide' | 'dots' | 'center' | 'marquee'
const productLayouts: { v: ProductLayout; t: string; d: string }[] = [
  { v: 'grid', t: 'Lưới', d: 'Xếp thành lưới nhiều hàng' },
  { v: 'slide', t: 'Trượt ngang', d: 'Vuốt hoặc bấm mũi tên' },
  { v: 'dots', t: 'Chia trang', d: 'Có chấm tròn, tự chuyển trang' },
  { v: 'center', t: 'Tâm điểm', d: 'Sản phẩm ở giữa to nhất' },
  { v: 'marquee', t: 'Chạy liên tục', d: 'Dải tự chạy, rê chuột để dừng' }
]
const productLayout = computed<ProductLayout>(() => {
  const l = d().layout as ProductLayout
  return productLayouts.some((o) => o.v === l) ? l : 'grid'
})
function setProductLayout(v: ProductLayout) {
  d().layout = v
  // Kiểu "tâm điểm" đẹp nhất với số sản phẩm lẻ; kiểu khác giữ số shop đã chọn
  if (v === 'center' && !d().per_view) d().per_view = 3
}
// Hình minh hoạ nhỏ của từng kiểu (khung 60 × 30)
function layoutPreview(v: ProductLayout): { rects: { x: number; y: number; w: number; h: number; o: number }[]; dots: number } {
  const row = (xs: number[], w: number, h: number, y: number, o: number[] = []) => xs.map((x, i) => ({ x, y, w, h, o: o[i] ?? 1 }))
  switch (v) {
    case 'slide':
      return { rects: row([2, 21, 40], 17, 22, 4, [1, 1, 0.55]), dots: 0 }
    case 'dots':
      return { rects: row([2, 21, 40], 17, 18, 2), dots: 3 }
    case 'center':
      return { rects: [...row([3], 12, 14, 8, [0.45]), ...row([19], 22, 24, 3), ...row([45], 12, 14, 8, [0.45])], dots: 0 }
    case 'marquee':
      return { rects: row([-5, 14, 33, 52], 15, 20, 5, [0.35, 1, 1, 0.35]), dots: 0 }
    default:
      return { rects: [...row([2, 17, 32, 47], 12, 11, 2), ...row([2, 17, 32, 47], 12, 11, 16)], dots: 0 }
  }
}

const cssPlaceholder ='border-radius: 24px;\n\nh2 {\n  color: #be123c;\n  font-size: 48px;\n}\n\nbutton:hover {\n  transform: scale(1.03);\n}'

function setStyle(key: 'bg' | 'pad' | 'css', val: string) {
  const st: Record<string, string> = { ...(d().style || {}) }
  if (!val.trim() || (key === 'pad' && val === 'none')) delete st[key]
  else st[key] = val
  d().style = st
}
</script>

<template>
  <div class="space-y-4 text-left">
    <!-- ===== Đầu trang ===== -->
    <template v-if="block.type === 'header'">
      <!-- Kiểu có sẵn: áp cả bộ trong 1 lần bấm -->
      <div class="space-y-1.5">
        <span :class="lbl">Kiểu đầu trang có sẵn</span>
        <div class="grid grid-cols-2 gap-1.5">
          <button
            v-for="p in headerPresets"
            :key="p.key"
            type="button"
            class="overflow-hidden rounded-xl border border-slate-200 text-left transition hover:border-indigo-400 hover:shadow-sm"
            @click="applyHeaderPreset(p.key)"
          >
            <div class="flex h-8 items-center gap-1.5 px-2" :style="{ backgroundColor: p.bg, color: p.fg }">
              <span class="h-3.5 w-3.5 rounded-sm" :style="{ backgroundColor: p.fg, opacity: 0.85 }" />
              <span class="h-1 w-4 rounded-full" :style="{ backgroundColor: p.fg, opacity: 0.6 }" />
              <span class="h-1 w-4 rounded-full" :style="{ backgroundColor: p.fg, opacity: 0.6 }" />
              <span class="ml-auto flex gap-1">
                <span v-for="n in 3" :key="n" class="h-2 w-2 rounded-full border" :style="{ borderColor: p.fg }" />
              </span>
            </div>
            <div class="px-2 py-1.5 text-[11px] font-bold text-slate-700">{{ p.name }}</div>
          </button>
        </div>
      </div>
      <ImageField v-model="block.data.logo_url" :site-id="siteId" label="Logo" compact />
      <label class="flex items-center gap-3 text-xs font-bold text-slate-600">
        Cao logo
        <input v-model.number="block.data.logo_height" type="range" min="20" max="120" step="2" class="flex-1 accent-indigo-600" />
        <span class="w-10 text-right font-normal text-slate-500">{{ block.data.logo_height || 40 }}px</span>
      </label>
      <label class="space-y-1">
        <span :class="lbl">Tên thương hiệu</span>
        <input v-model="block.data.brand_name" :class="inp" />
      </label>
      <div class="space-y-1">
        <span :class="lbl">Hiển thị ở đầu trang</span>
        <div class="grid grid-cols-3 gap-1">
          <button
            v-for="m in brandModes"
            :key="m.v"
            type="button"
            class="rounded-lg border px-1 py-1.5 text-[11px] font-bold disabled:cursor-not-allowed disabled:opacity-40"
            :class="brandMode === m.v ? 'border-indigo-600 bg-indigo-50 text-indigo-700' : 'border-slate-200 text-slate-600 hover:border-slate-300'"
            :disabled="m.v !== 'name' && !hasLogo"
            :title="m.v !== 'name' && !hasLogo ? 'Hãy chọn ảnh logo trước' : ''"
            @click="setBrandMode(m.v)"
          >
            {{ m.t }}
          </button>
        </div>
        <p v-if="!hasLogo" class="text-[10px] text-slate-400">Chưa có logo nên đầu trang hiện tên chữ.</p>
      </div>
      <div class="flex flex-wrap items-center gap-4">
        <label v-if="brandMode !== 'logo'" class="flex items-center gap-2 text-xs font-bold text-slate-600">
          Cỡ chữ
          <select :value="block.data.brand_size || 'md'" class="rounded-lg border border-slate-200 px-2 py-1 text-sm" @change="block.data.brand_size = ($event.target as HTMLSelectElement).value">
            <option value="sm">Nhỏ</option>
            <option value="md">Vừa</option>
            <option value="lg">Lớn</option>
          </select>
        </label>
      </div>
      <label class="space-y-1">
        <span :class="lbl">Bố cục</span>
        <select v-model="block.data.layout" :class="inp">
          <option value="left_menu">Logo · Menu sát logo · Biểu tượng bên phải</option>
          <option value="left">Logo trái · Menu và nút bên phải</option>
          <option value="split">Logo trái · Menu giữa · Nút phải</option>
          <option value="center">Logo giữa · Menu bên dưới</option>
          <option value="center_split">Menu trái · Logo giữa · Nút phải</option>
        </select>
      </label>

      <div class="space-y-1.5">
        <span :class="lbl">Menu điều hướng</span>
        <select :value="block.data.menu_id || ''" data-testid="header-menu-select" class="w-full rounded-lg border border-slate-200 bg-white px-2 py-1.5 text-sm" @change="block.data.menu_id = ($event.target as HTMLSelectElement).value">
          <option value="">Tự nhập (danh sách bên dưới)</option>
          <option v-for="m in menus || []" :key="m.id" :value="m.id">{{ m.name }}</option>
        </select>
        <p v-if="menuOf(block.data.menu_id)" class="rounded-lg bg-indigo-50 px-2.5 py-1.5 text-[11px] text-indigo-800" data-testid="header-menu-note">Đang dùng menu "{{ menuOf(block.data.menu_id)?.name }}" — sửa mục menu ở tab <b>Menu</b>.</p>
        <button v-else-if="list('nav_items').length" type="button" data-testid="header-nav-to-menu" class="text-[11px] font-bold text-indigo-700 hover:underline" @click="navToMenu">Tạo menu từ danh sách này (dùng lại được ở chân trang)</button>
      </div>

      <div v-if="!menuOf(block.data.menu_id)" class="space-y-2">
        <span :class="lbl">Menu (kéo để đổi thứ tự)</span>
        <div
          v-for="(n, ni) in list('nav_items')"
          :key="ni"
          :class="rowClass('nav_items', ni)"
          @dragover="itemDragOver('nav_items', ni, $event)"
          @drop.prevent="itemDrop('nav_items', ni)"
        >
          <div class="flex items-center gap-1.5">
            <span class="cursor-grab text-slate-400" draggable="true" @dragstart="itemDragStart('nav_items', ni, $event)" @dragend="itemDragEnd">
              <GripVertical class="h-4 w-4" />
            </span>
            <input v-model="n.label" placeholder="Chữ" class="w-2/5 rounded-lg border border-slate-200 px-2 py-1 text-sm" />
            <input v-model="n.link" list="pp-page-links" placeholder="#products, https://..." class="min-w-0 flex-1 rounded-lg border border-slate-200 px-2 py-1 text-sm" />
            <button type="button" class="rounded p-1 text-rose-600 hover:bg-rose-50" @click="removeFrom('nav_items', ni)"><Trash2 class="h-3.5 w-3.5" /></button>
          </div>
        </div>
        <button type="button" class="inline-flex items-center gap-1 text-xs font-bold text-indigo-700" @click="addTo('nav_items', { label: 'Mục mới', link: '' })">
          <Plus class="h-3.5 w-3.5" /> Thêm mục menu
        </button>
      </div>

      <div class="space-y-1">
        <span :class="lbl">Góc phải đầu trang</span>
        <div class="grid grid-cols-2 gap-1">
          <button
            v-for="m in [{ v: 'account', t: 'Giỏ hàng + Tài khoản' }, { v: 'cta', t: 'Nút đặt mua' }]"
            :key="m.v"
            type="button"
            class="rounded-lg border px-1 py-1.5 text-[11px] font-bold"
            :class="(block.data.right_mode || 'cta') === m.v ? 'border-indigo-600 bg-indigo-50 text-indigo-700' : 'border-slate-200 text-slate-600 hover:border-slate-300'"
            @click="block.data.right_mode = m.v"
          >
            {{ m.t }}
          </button>
        </div>
        <template v-if="block.data.right_mode === 'account'">
          <span :class="lbl" class="pt-1">Kiểu nút tài khoản</span>
          <div class="grid grid-cols-2 gap-1">
            <button
              v-for="m in [{ v: 'icon', t: 'Biểu tượng 👤' }, { v: 'buttons', t: 'Chữ Đăng nhập / Đăng ký' }]"
              :key="m.v"
              type="button"
              class="rounded-lg border px-1 py-1.5 text-[11px] font-bold"
              :class="(block.data.account_style || 'buttons') === m.v ? 'border-indigo-600 bg-indigo-50 text-indigo-700' : 'border-slate-200 text-slate-600 hover:border-slate-300'"
              @click="block.data.account_style = m.v"
            >
              {{ m.t }}
            </button>
          </div>
          <p class="text-[10px] leading-snug text-slate-400">
            {{ block.data.account_style === 'icon'
              ? 'Chưa đăng nhập: bấm 👤 mở hộp Đăng nhập / Đăng ký. Đã đăng nhập: 👤 có chấm xanh, bấm mở Đơn hàng của tôi / Đăng xuất.'
              : 'Khách thấy nút Đăng nhập / Đăng ký; đã đăng nhập thì thấy tên mình, “Đơn hàng của tôi” và Đăng xuất.' }}
          </p>
        </template>
      </div>
      <template v-if="block.data.right_mode !== 'account'">
        <label class="flex items-center gap-2 text-xs font-bold text-slate-600">
          <input type="checkbox" :checked="block.data.show_cta !== false" @change="block.data.show_cta = ($event.target as HTMLInputElement).checked" />
          Hiện nút
        </label>
        <label v-if="block.data.show_cta !== false" class="space-y-1">
          <span :class="lbl">Liên kết nút</span>
          <input v-model="block.data.cta_link" list="pp-page-links" placeholder="#products" :class="inp" />
        </label>
      </template>
      <div class="flex flex-wrap items-center gap-4">
        <label class="flex items-center gap-2 text-xs font-bold text-slate-600"><input v-model="block.data.show_search" type="checkbox" /> Ô tìm kiếm</label>
        <label v-if="block.data.right_mode !== 'account'" class="flex items-center gap-2 text-xs font-bold text-slate-600"><input v-model="block.data.show_cart" type="checkbox" /> Giỏ hàng</label>
      </div>
      <div class="flex flex-wrap items-center gap-4">
        <label class="flex items-center gap-2 text-xs font-bold text-slate-600"><input v-model="block.data.sticky" type="checkbox" /> Ghim khi cuộn</label>
        <label class="flex items-center gap-2 text-xs font-bold text-slate-600">
          <input type="checkbox" :checked="block.data.border !== false" @change="block.data.border = ($event.target as HTMLInputElement).checked" />
          Viền dưới
        </label>
        <label class="flex items-center gap-2 text-xs font-bold text-slate-600">
          Màu chữ
          <input type="color" :value="block.data.text_color || '#0f172a'" class="h-7 w-9 rounded border border-slate-200" @input="block.data.text_color = ($event.target as HTMLInputElement).value" />
        </label>
        <button v-if="block.data.text_color" type="button" class="text-xs text-slate-500 underline" @click="block.data.text_color = ''">Mặc định</button>
      </div>
      <!-- Màu nền đầu trang -->
      <div class="space-y-2 rounded-xl border border-slate-200 p-3" data-testid="header-bg-panel">
        <div class="flex flex-wrap items-center gap-3">
          <span :class="lbl">Màu nền đầu trang</span>
          <input
            type="color"
            data-testid="header-bg"
            :value="block.data.style?.bg || '#ffffff'"
            class="h-7 w-9 rounded border border-slate-200"
            @input="setStyle('bg', ($event.target as HTMLInputElement).value)"
          />
          <button v-if="block.data.style?.bg" type="button" class="text-xs text-slate-500 underline" data-testid="header-bg-reset" @click="setStyle('bg', '')">Bỏ màu</button>
        </div>
        <div class="flex flex-wrap gap-1.5">
          <button
            v-for="c in headerBgSwatches"
            :key="c"
            type="button"
            class="h-6 w-6 rounded-full border-2"
            :class="(block.data.style?.bg || '').toLowerCase() === c.toLowerCase() ? 'border-indigo-600' : 'border-slate-200'"
            :style="{ backgroundColor: c }"
            :title="c"
            @click="setStyle('bg', c)"
          />
        </div>
        <p class="text-[11px] leading-snug text-slate-400">Nền tối thì chữ tự chuyển sang màu trắng; muốn màu chữ riêng thì chọn ở "Màu chữ" phía trên.</p>
      </div>
    </template>

    <!-- ===== Banner ===== -->
    <template v-else-if="block.type === 'hero'">
      <!-- Ảnh nền + chiều cao -->
      <div class="space-y-2.5 rounded-xl border border-slate-200 p-3" data-testid="hero-bg-panel">
        <span :class="lbl">Ảnh nền banner</span>
        <ImageField v-model="block.data.bg_image_url" :site-id="siteId" compact />
        <template v-if="block.data.bg_image_url">
          <label class="flex items-center gap-3 text-xs font-bold text-slate-600">
            Độ phủ tối
            <input
              type="range"
              min="0"
              max="85"
              step="5"
              data-testid="hero-overlay"
              :value="block.data.bg_overlay ?? 40"
              class="flex-1 accent-indigo-600"
              @input="block.data.bg_overlay = Number(($event.target as HTMLInputElement).value)"
            />
            <span class="w-9 text-right font-normal text-slate-500">{{ block.data.bg_overlay ?? 40 }}%</span>
          </label>
          <div class="space-y-1">
            <span :class="lbl">Phần ảnh được giữ lại khi bị cắt</span>
            <div class="grid grid-cols-3 gap-1">
              <button
                v-for="o in [{ v: 'top', t: 'Phía trên' }, { v: 'center', t: 'Ở giữa' }, { v: 'bottom', t: 'Phía dưới' }]"
                :key="o.v"
                type="button"
                class="rounded-lg border px-1 py-1.5 text-[11px] font-bold"
                :class="(block.data.bg_focus || 'center') === o.v ? 'border-indigo-600 bg-indigo-50 text-indigo-700' : 'border-slate-200 text-slate-600 hover:border-slate-300'"
                @click="block.data.bg_focus = o.v"
              >
                {{ o.t }}
              </button>
            </div>
          </div>
        </template>
        <div class="space-y-1">
          <div class="flex items-center justify-between">
            <span :class="lbl">Chiều cao banner</span>
            <span class="text-[11px] text-slate-500" data-testid="hero-height-label">{{ block.data.bg_height ? block.data.bg_height + 'px' : 'Tự động' }}</span>
          </div>
          <div class="grid grid-cols-5 gap-1">
            <button
              v-for="o in heroHeights"
              :key="o.v"
              type="button"
              class="rounded-lg border px-0.5 py-1.5 text-[11px] font-bold"
              :class="(Number(block.data.bg_height) || 0) === o.v ? 'border-indigo-600 bg-indigo-50 text-indigo-700' : 'border-slate-200 text-slate-600 hover:border-slate-300'"
              :data-height="o.v"
              @click="block.data.bg_height = o.v"
            >
              {{ o.t }}
            </button>
          </div>
          <label v-if="Number(block.data.bg_height) > 0" class="flex items-center gap-3 text-xs font-bold text-slate-600">
            Tinh chỉnh
            <input
              type="range"
              min="240"
              max="900"
              step="10"
              data-testid="hero-height"
              :value="block.data.bg_height"
              class="flex-1 accent-indigo-600"
              @input="block.data.bg_height = Number(($event.target as HTMLInputElement).value)"
            />
          </label>
        </div>
        <div class="space-y-1">
          <span :class="lbl">Căn chữ</span>
          <div class="grid grid-cols-3 gap-1">
            <button
              v-for="o in [{ v: 'left', t: 'Trái' }, { v: 'center', t: 'Giữa' }, { v: 'right', t: 'Phải' }]"
              :key="o.v"
              type="button"
              class="rounded-lg border px-1 py-1.5 text-[11px] font-bold"
              :class="(block.data.align || '') === o.v ? 'border-indigo-600 bg-indigo-50 text-indigo-700' : 'border-slate-200 text-slate-600 hover:border-slate-300'"
              @click="block.data.align = o.v"
            >
              {{ o.t }}
            </button>
          </div>
        </div>
        <div class="flex flex-wrap items-center gap-3">
          <span :class="lbl">Màu chữ</span>
          <input type="color" :value="block.data.text_color || '#ffffff'" class="h-7 w-9 rounded border border-slate-200" @input="block.data.text_color = ($event.target as HTMLInputElement).value" />
          <button v-if="block.data.text_color" type="button" class="text-xs text-slate-500 underline" @click="block.data.text_color = ''">Tự động</button>
        </div>
      </div>
      <ImageField v-model="block.data.image_url" :site-id="siteId" label="Ảnh bên cạnh chữ (không bắt buộc)" />
      <label class="space-y-1"><span :class="lbl">Nhãn nhỏ</span><input v-model="block.data.badge" :class="inp" /></label>
      <label class="space-y-1"><span :class="lbl">Tiêu đề</span><input v-model="block.data.title" :class="inp" /></label>
      <label class="space-y-1"><span :class="lbl">Mô tả</span><textarea v-model="block.data.subtitle" rows="3" :class="inp" /></label>
      <div class="grid grid-cols-2 gap-2">
        <label class="space-y-1"><span :class="lbl">Chữ trên nút</span><input v-model="block.data.cta_label" :class="inp" /></label>
        <label class="space-y-1"><span :class="lbl">Liên kết nút</span><input v-model="block.data.cta_link" list="pp-page-links" :class="inp" /></label>
      </div>
    </template>

    <!-- ===== Sản phẩm ===== -->
    <template v-else-if="block.type === 'products'">
      <label class="space-y-1"><span :class="lbl">Tiêu đề mục</span><input v-model="block.data.heading" :class="inp" /></label>
      <!-- Kiểu hiển thị: lưới hoặc các kiểu carousel -->
      <div class="space-y-2" data-testid="product-layout">
        <span :class="lbl">Kiểu hiển thị</span>
        <div class="grid grid-cols-2 gap-1.5">
          <button
            v-for="o in productLayouts"
            :key="o.v"
            type="button"
            class="rounded-xl border p-2 text-left transition"
            :class="productLayout === o.v ? 'border-indigo-600 bg-indigo-50' : 'border-slate-200 hover:border-slate-300'"
            :data-layout="o.v"
            @click="setProductLayout(o.v)"
          >
            <svg viewBox="0 0 60 30" class="mb-1.5 h-8 w-full" aria-hidden="true">
              <rect v-for="(r, ri) in layoutPreview(o.v).rects" :key="ri" :x="r.x" :y="r.y" :width="r.w" :height="r.h" rx="2" :fill="productLayout === o.v ? '#6366f1' : '#94a3b8'" :opacity="r.o" />
              <circle v-for="di in layoutPreview(o.v).dots" :key="'d' + di" :cx="30 + (di - 1 - (layoutPreview(o.v).dots - 1) / 2) * 6" cy="27" r="1.4" :fill="di === 1 ? '#6366f1' : '#cbd5e1'" />
            </svg>
            <span class="block text-[11px] font-bold text-slate-800">{{ o.t }}</span>
            <span class="block text-[10px] leading-tight text-slate-400">{{ o.d }}</span>
          </button>
        </div>
        <template v-if="productLayout !== 'grid'">
          <div class="grid grid-cols-2 gap-2">
            <label class="space-y-1">
              <span :class="lbl">Số sản phẩm mỗi hàng</span>
              <select :value="block.data.per_view || (productLayout === 'center' ? 3 : 4)" :class="inp" data-testid="per-view" @change="block.data.per_view = Number(($event.target as HTMLSelectElement).value)">
                <option v-for="n in [2, 3, 4, 5, 6]" :key="n" :value="n">{{ n }}</option>
              </select>
            </label>
            <label v-if="productLayout !== 'marquee'" class="space-y-1">
              <span :class="lbl">Tự chuyển sau</span>
              <select :value="block.data.autoplay ?? (productLayout === 'dots' ? 5 : 0)" :class="inp" data-testid="autoplay" @change="block.data.autoplay = Number(($event.target as HTMLSelectElement).value)">
                <option :value="0">Không tự chuyển</option>
                <option v-for="n in [3, 4, 5, 7, 10]" :key="n" :value="n">{{ n }} giây</option>
              </select>
            </label>
            <label v-else class="space-y-1">
              <span :class="lbl">Tốc độ chạy</span>
              <select :value="block.data.speed || 'normal'" :class="inp" @change="block.data.speed = ($event.target as HTMLSelectElement).value">
                <option value="slow">Chậm</option>
                <option value="normal">Vừa</option>
                <option value="fast">Nhanh</option>
              </select>
            </label>
          </div>
          <div v-if="productLayout !== 'marquee'" class="flex flex-wrap items-center gap-4">
            <label class="flex items-center gap-2 text-xs font-bold text-slate-600">
              <input type="checkbox" :checked="block.data.arrows !== false" @change="block.data.arrows = ($event.target as HTMLInputElement).checked" />
              Mũi tên trái / phải
            </label>
            <label class="flex items-center gap-2 text-xs font-bold text-slate-600">
              <input type="checkbox" :checked="block.data.dots ?? (productLayout === 'dots' || productLayout === 'center')" @change="block.data.dots = ($event.target as HTMLInputElement).checked" />
              Chấm tròn bên dưới
            </label>
          </div>
          <p class="text-[11px] leading-snug text-slate-400">Trong trình chỉnh sửa, carousel không tự chạy để bạn dễ chỉnh sửa; trên trang đã xuất bản thì chạy bình thường.</p>
        </template>
      </div>
      <label class="space-y-1">
        <span :class="lbl">Nguồn sản phẩm</span>
        <select v-model="block.data.source" :class="inp">
          <option value="catalog">Danh sách sản phẩm của shop</option>
          <option value="manual">Nhập tay trong section này</option>
        </select>
      </label>
      <!-- Lọc theo thẻ (thẻ gắn ở Quản lý sản phẩm) -->
      <div v-if="block.data.source === 'catalog'" class="space-y-1.5 rounded-xl border border-slate-200 p-3" data-testid="tag-filter">
        <span :class="lbl">Chỉ hiện sản phẩm có thẻ</span>
        <div v-if="tagChoices.length" class="flex flex-wrap gap-1.5">
          <button
            v-for="t in tagChoices"
            :key="t.name"
            type="button"
            class="rounded-full border px-2.5 py-1 text-xs font-semibold"
            :class="tagOn(t.name) ? 'border-indigo-600 bg-indigo-50 text-indigo-700' : 'border-slate-200 text-slate-600 hover:bg-slate-50'"
            :data-tag="t.name"
            @click="toggleTag(t.name)"
          >
            {{ t.name }} <span class="text-[10px] opacity-60">{{ t.count }}</span>
          </button>
        </div>
        <p v-else class="text-[11px] leading-relaxed text-slate-400">Chưa có sản phẩm nào gắn thẻ. Thêm thẻ khi sửa sản phẩm ở trang “Quản lý sản phẩm”.</p>
        <p class="text-[11px] leading-relaxed text-slate-500">
          <template v-if="selectedTags.length">Section này chỉ hiện sản phẩm có ít nhất một thẻ đã chọn: <strong>{{ matchingCount }}</strong> sản phẩm.</template>
          <template v-else>Chưa chọn thẻ nào: section hiện tất cả sản phẩm đang bán.</template>
          <button v-if="selectedTags.length" type="button" class="ml-1 font-bold text-indigo-700 hover:underline" @click="block.data.tags = []">Bỏ chọn hết</button>
        </p>
      </div>
      <label class="flex items-center gap-2 text-xs font-bold text-slate-600">
        <input type="checkbox" :checked="block.data.show_buy !== false" @change="block.data.show_buy = ($event.target as HTMLInputElement).checked" />
        Hiện nút đặt mua dưới mỗi sản phẩm
      </label>
      <template v-if="block.data.show_buy !== false">
        <label v-if="block.data.source === 'catalog'" class="space-y-1">
          <span :class="lbl">Khi bấm nút</span>
          <select :value="block.data.buy_mode || 'cart'" :class="inp" @change="block.data.buy_mode = ($event.target as HTMLSelectElement).value">
            <option value="cart">Thêm vào giỏ hàng (khách đặt hàng & thanh toán trên web)</option>
            <option value="link">Mở liên kết (Messenger, Zalo, số điện thoại...)</option>
          </select>
        </label>
        <div class="grid grid-cols-2 gap-2">
          <label class="space-y-1"><span :class="lbl">Chữ trên nút</span><input v-model="block.data.buy_label" :placeholder="block.data.buy_mode === 'link' || block.data.source !== 'catalog' ? 'Đặt mua' : 'Thêm vào giỏ'" :class="inp" /></label>
          <label v-if="block.data.buy_mode === 'link' || block.data.source !== 'catalog'" class="space-y-1">
            <span :class="lbl">Liên kết nút</span><input v-model="block.data.buy_link" list="pp-page-links" placeholder="#contact, https://m.me/..." :class="inp" />
          </label>
        </div>
      </template>
      <p v-if="block.data.source === 'catalog'" class="rounded-lg bg-slate-50 px-3 py-2 text-xs text-slate-500">
        Sản phẩm, giá và tồn kho quản lý ở trang
        <router-link :to="{ name: 'site-products', params: { id: siteId } }" class="font-bold text-indigo-700 hover:underline">Quản lý sản phẩm</router-link>.
      </p>
      <template v-else>
        <div
          v-for="(p, pi) in list('items')"
          :key="p.id || pi"
          :class="rowClass('items', pi)"
          class="space-y-2"
          @dragover="itemDragOver('items', pi, $event)"
          @drop.prevent="itemDrop('items', pi)"
        >
          <div class="flex items-center gap-1.5">
            <span class="cursor-grab text-slate-400" draggable="true" @dragstart="itemDragStart('items', pi, $event)" @dragend="itemDragEnd"><GripVertical class="h-4 w-4" /></span>
            <input v-model="p.name" placeholder="Tên sản phẩm" class="min-w-0 flex-1 rounded-lg border border-slate-200 px-2 py-1 text-sm" />
            <input v-model="p.price" placeholder="Giá" class="w-24 rounded-lg border border-slate-200 px-2 py-1 text-sm" />
            <button type="button" class="rounded p-1 text-rose-600 hover:bg-rose-50" @click="removeFrom('items', pi)"><Trash2 class="h-3.5 w-3.5" /></button>
          </div>
          <ImageField v-model="p.image_url" :site-id="siteId" compact />
          <textarea v-model="p.desc" rows="2" placeholder="Mô tả" class="w-full rounded-lg border border-slate-200 px-2 py-1 text-sm" />
        </div>
        <button type="button" class="inline-flex items-center gap-1 text-xs font-bold text-indigo-700" @click="addTo('items', { id: 'p' + Date.now(), name: 'Sản phẩm mới', price: '0đ', image_url: '', desc: '' })">
          <Plus class="h-3.5 w-3.5" /> Thêm sản phẩm
        </button>
      </template>
    </template>

    <!-- ===== Cam kết ===== -->
    <template v-else-if="block.type === 'features'">
      <label class="space-y-1"><span :class="lbl">Tiêu đề mục</span><input v-model="block.data.heading" :class="inp" /></label>
      <span :class="lbl">Các cam kết</span>
      <div
        v-for="(_, fi) in list('items')"
        :key="fi"
        :class="rowClass('items', fi)"
        @dragover="itemDragOver('items', fi, $event)"
        @drop.prevent="itemDrop('items', fi)"
      >
        <div class="flex items-center gap-1.5">
          <span class="cursor-grab text-slate-400" draggable="true" @dragstart="itemDragStart('items', fi, $event)" @dragend="itemDragEnd"><GripVertical class="h-4 w-4" /></span>
          <input v-model="block.data.items[fi]" class="min-w-0 flex-1 rounded-lg border border-slate-200 px-2 py-1 text-sm" />
          <button type="button" class="rounded p-1 text-rose-600 hover:bg-rose-50" @click="removeFrom('items', fi)"><Trash2 class="h-3.5 w-3.5" /></button>
        </div>
      </div>
      <button type="button" class="inline-flex items-center gap-1 text-xs font-bold text-indigo-700" @click="addTo('items', 'Cam kết mới')">
        <Plus class="h-3.5 w-3.5" /> Thêm cam kết
      </button>
    </template>

    <!-- ===== Đánh giá ===== -->
    <template v-else-if="block.type === 'testimonials'">
      <label class="space-y-1"><span :class="lbl">Tiêu đề mục</span><input v-model="block.data.heading" :class="inp" /></label>
      <div class="flex gap-1">
        <button
          v-for="o in [{ v: 'grid', t: 'Dạng lưới' }, { v: 'carousel', t: 'Dạng trượt' }]"
          :key="o.v"
          type="button"
          class="flex-1 rounded-lg border px-2 py-1.5 text-xs font-bold"
          :class="(block.data.layout || 'grid') === o.v ? 'border-indigo-600 bg-indigo-50 text-indigo-700' : 'border-slate-200 text-slate-600'"
          @click="block.data.layout = o.v"
        >
          {{ o.t }}
        </button>
      </div>
      <div
        v-for="(t, ti) in list('items')"
        :key="ti"
        :class="rowClass('items', ti)"
        class="space-y-2"
        @dragover="itemDragOver('items', ti, $event)"
        @drop.prevent="itemDrop('items', ti)"
      >
        <div class="flex items-center gap-1.5">
          <span class="cursor-grab text-slate-400" draggable="true" @dragstart="itemDragStart('items', ti, $event)" @dragend="itemDragEnd"><GripVertical class="h-4 w-4" /></span>
          <input v-model="t.name" placeholder="Tên khách" class="min-w-0 flex-1 rounded-lg border border-slate-200 px-2 py-1 text-sm" />
          <button type="button" class="rounded p-1 text-rose-600 hover:bg-rose-50" @click="removeFrom('items', ti)"><Trash2 class="h-3.5 w-3.5" /></button>
        </div>
        <textarea v-model="t.quote" rows="2" placeholder="Nội dung đánh giá" class="w-full rounded-lg border border-slate-200 px-2 py-1 text-sm" />
      </div>
      <button type="button" class="inline-flex items-center gap-1 text-xs font-bold text-indigo-700" @click="addTo('items', { name: '', quote: '' })">
        <Plus class="h-3.5 w-3.5" /> Thêm đánh giá
      </button>
    </template>

    <!-- ===== Kêu gọi hành động ===== -->
    <template v-else-if="block.type === 'cta'">
      <label class="space-y-1"><span :class="lbl">Tiêu đề</span><input v-model="block.data.title" :class="inp" /></label>
      <label class="space-y-1"><span :class="lbl">Mô tả</span><textarea v-model="block.data.subtitle" rows="2" :class="inp" /></label>
      <div class="grid grid-cols-2 gap-2">
        <label class="space-y-1"><span :class="lbl">Chữ trên nút</span><input v-model="block.data.button_text" :class="inp" /></label>
        <label class="space-y-1"><span :class="lbl">Liên kết nút</span><input v-model="block.data.button_link" list="pp-page-links" placeholder="tel:, https://..." :class="inp" /></label>
      </div>
      <label class="space-y-1"><span :class="lbl">Ghi chú chân trang</span><input v-model="block.data.footer_note" :class="inp" /></label>
    </template>

    <!-- ===== Văn bản ===== -->
    <template v-else-if="block.type === 'richtext'">
      <label class="space-y-1"><span :class="lbl">Tiêu đề</span><input v-model="block.data.heading" :class="inp" /></label>
      <label class="space-y-1"><span :class="lbl">Nội dung</span><textarea v-model="block.data.body" rows="7" :class="inp" /></label>
      <label class="flex items-center gap-2 text-xs font-bold text-slate-600"><input v-model="block.data.compact" type="checkbox" /> Thu gọn khoảng cách trên / dưới</label>
      <div class="flex gap-1">
        <button
          v-for="a in [{ v: 'left', t: 'Trái' }, { v: 'center', t: 'Giữa' }, { v: 'right', t: 'Phải' }]"
          :key="a.v"
          type="button"
          class="flex-1 rounded-lg border px-2 py-1.5 text-xs font-bold"
          :class="(block.data.align || 'left') === a.v ? 'border-indigo-600 bg-indigo-50 text-indigo-700' : 'border-slate-200 text-slate-600'"
          @click="block.data.align = a.v"
        >
          {{ a.t }}
        </button>
      </div>
    </template>

    <!-- ===== Ảnh lớn ===== -->
    <template v-else-if="block.type === 'image'">
      <ImageField v-model="block.data.image_url" :site-id="siteId" label="Ảnh" />
      <label class="space-y-1"><span :class="lbl">Mô tả ảnh (SEO)</span><input v-model="block.data.alt" :class="inp" /></label>
      <label class="space-y-1"><span :class="lbl">Liên kết khi bấm ảnh</span><input v-model="block.data.link" list="pp-page-links" placeholder="https://..." :class="inp" /></label>
    </template>

    <!-- ===== Thư viện ảnh ===== -->
    <template v-else-if="block.type === 'gallery'">
      <label class="space-y-1"><span :class="lbl">Tiêu đề mục</span><input v-model="block.data.heading" :class="inp" /></label>
      <div
        v-for="(g, gi) in list('items')"
        :key="gi"
        :class="rowClass('items', gi)"
        class="space-y-2"
        @dragover="itemDragOver('items', gi, $event)"
        @drop.prevent="itemDrop('items', gi)"
      >
        <div class="flex items-center gap-1.5">
          <span class="cursor-grab text-slate-400" draggable="true" @dragstart="itemDragStart('items', gi, $event)" @dragend="itemDragEnd"><GripVertical class="h-4 w-4" /></span>
          <span class="flex-1 text-xs font-semibold text-slate-500">Ảnh {{ gi + 1 }}</span>
          <button type="button" class="rounded p-1 text-rose-600 hover:bg-rose-50" @click="removeFrom('items', gi)"><Trash2 class="h-3.5 w-3.5" /></button>
        </div>
        <ImageField v-model="g.image_url" :site-id="siteId" compact />
        <input v-model="g.caption" placeholder="Chú thích" class="w-full rounded-lg border border-slate-200 px-2 py-1 text-sm" />
      </div>
      <button type="button" class="inline-flex items-center gap-1 text-xs font-bold text-indigo-700" @click="addTo('items', { image_url: '', caption: '' })">
        <Plus class="h-3.5 w-3.5" /> Thêm ảnh
      </button>
    </template>

    <!-- ===== FAQ ===== -->
    <template v-else-if="block.type === 'faq'">
      <label class="space-y-1"><span :class="lbl">Tiêu đề mục</span><input v-model="block.data.heading" :class="inp" /></label>
      <div
        v-for="(f, fi) in list('items')"
        :key="fi"
        :class="rowClass('items', fi)"
        class="space-y-2"
        @dragover="itemDragOver('items', fi, $event)"
        @drop.prevent="itemDrop('items', fi)"
      >
        <div class="flex items-center gap-1.5">
          <span class="cursor-grab text-slate-400" draggable="true" @dragstart="itemDragStart('items', fi, $event)" @dragend="itemDragEnd"><GripVertical class="h-4 w-4" /></span>
          <input v-model="f.q" placeholder="Câu hỏi" class="min-w-0 flex-1 rounded-lg border border-slate-200 px-2 py-1 text-sm" />
          <button type="button" class="rounded p-1 text-rose-600 hover:bg-rose-50" @click="removeFrom('items', fi)"><Trash2 class="h-3.5 w-3.5" /></button>
        </div>
        <textarea v-model="f.a" rows="2" placeholder="Câu trả lời" class="w-full rounded-lg border border-slate-200 px-2 py-1 text-sm" />
      </div>
      <button type="button" class="inline-flex items-center gap-1 text-xs font-bold text-indigo-700" @click="addTo('items', { q: '', a: '' })">
        <Plus class="h-3.5 w-3.5" /> Thêm câu hỏi
      </button>
    </template>

    <!-- ===== Liên hệ ===== -->
    <template v-else-if="block.type === 'contact'">
      <label class="space-y-1"><span :class="lbl">Tiêu đề mục</span><input v-model="block.data.heading" :class="inp" /></label>
      <label class="space-y-1"><span :class="lbl">Số điện thoại</span><input v-model="block.data.phone" :class="inp" /></label>
      <label class="space-y-1"><span :class="lbl">Email</span><input v-model="block.data.email" :class="inp" /></label>
      <label class="space-y-1"><span :class="lbl">Địa chỉ</span><input v-model="block.data.address" :class="inp" /></label>
      <label class="space-y-1"><span :class="lbl">Link Zalo</span><input v-model="block.data.zalo_link" placeholder="https://zalo.me/..." :class="inp" /></label>
    </template>

    <!-- ===== HTML ===== -->
    <template v-else-if="block.type === 'html'">
      <textarea v-model="block.data.html" rows="12" spellcheck="false" class="w-full rounded-lg border border-slate-200 px-2.5 py-2 font-mono text-xs" />
      <label class="flex items-center gap-2 text-xs font-bold text-slate-600">
        Chiều cao (px)
        <input v-model.number="block.data.height" type="number" min="80" max="2000" class="w-24 rounded-lg border border-slate-200 px-2 py-1 text-sm" />
      </label>
      <p class="rounded-lg bg-amber-50 px-3 py-2 text-xs text-amber-800">
        Mã chạy trong khung cách ly: không đọc được dữ liệu đăng nhập hay điều hướng trang chính.
      </p>
    </template>

    <!-- ===== Dải thông báo ===== -->
    <template v-else-if="block.type === 'announcement'">
      <label class="space-y-1"><span :class="lbl">Nội dung</span><input v-model="block.data.text" :class="inp" /></label>
      <label class="space-y-1"><span :class="lbl">Liên kết (không bắt buộc)</span><input v-model="block.data.link" list="pp-page-links" placeholder="#products" :class="inp" /></label>
      <label class="flex items-center gap-2 text-xs font-bold text-slate-600">
        Màu chữ
        <input type="color" :value="block.data.text_color || '#ffffff'" class="h-7 w-9 rounded border border-slate-200" @input="block.data.text_color = ($event.target as HTMLInputElement).value" />
      </label>
      <p class="text-[11px] text-slate-400">Màu nền mặc định là màu chủ đạo; đổi ở mục "Giao diện section" bên dưới.</p>
    </template>

    <!-- ===== Banner ảnh trượt ===== -->
    <template v-else-if="block.type === 'banner'">
      <div class="grid grid-cols-2 gap-2">
        <label class="space-y-1">
          <span :class="lbl">Chiều cao</span>
          <select v-model="block.data.height" :class="inp">
            <option value="sm">Thấp</option>
            <option value="md">Vừa</option>
            <option value="lg">Cao</option>
            <option value="full">Toàn màn hình</option>
          </select>
        </label>
        <label class="space-y-1">
          <span :class="lbl">Tự trượt sau</span>
          <select v-model.number="block.data.autoplay" :class="inp">
            <option :value="0">Không tự trượt</option>
            <option :value="4">4 giây</option>
            <option :value="5">5 giây</option>
            <option :value="7">7 giây</option>
            <option :value="10">10 giây</option>
          </select>
        </label>
      </div>
      <label class="flex items-center gap-2 text-xs font-bold text-slate-600">
        <input type="checkbox" :checked="block.data.overlay !== false" @change="block.data.overlay = ($event.target as HTMLInputElement).checked" />
        Phủ tối ảnh để chữ dễ đọc
      </label>
      <span :class="lbl">Các ảnh (kéo để đổi thứ tự)</span>
      <div
        v-for="(s, si) in list('slides')"
        :key="si"
        :class="rowClass('slides', si)"
        class="space-y-2"
        @dragover="itemDragOver('slides', si, $event)"
        @drop.prevent="itemDrop('slides', si)"
      >
        <div class="flex items-center gap-1.5">
          <span class="cursor-grab text-slate-400" draggable="true" @dragstart="itemDragStart('slides', si, $event)" @dragend="itemDragEnd"><GripVertical class="h-4 w-4" /></span>
          <span class="flex-1 text-xs font-bold text-slate-600">Ảnh {{ si + 1 }}</span>
          <button type="button" class="rounded p-1 text-rose-600 hover:bg-rose-50" @click="removeFrom('slides', si)"><Trash2 class="h-3.5 w-3.5" /></button>
        </div>
        <ImageField v-model="s.image_url" :site-id="siteId" label="Ảnh máy tính (ngang, ~1920×800)" compact />
        <ImageField v-model="s.mobile_image_url" :site-id="siteId" label="Ảnh điện thoại (dọc, không bắt buộc)" compact />
        <input v-model="s.heading" placeholder="Tiêu đề" class="w-full rounded-lg border border-slate-200 px-2 py-1 text-sm" />
        <textarea v-model="s.text" rows="2" placeholder="Mô tả" class="w-full rounded-lg border border-slate-200 px-2 py-1 text-sm" />
        <div class="grid grid-cols-2 gap-2">
          <input v-model="s.button_label" placeholder="Chữ trên nút" class="rounded-lg border border-slate-200 px-2 py-1 text-sm" />
          <input v-model="s.link" list="pp-page-links" placeholder="Liên kết" class="rounded-lg border border-slate-200 px-2 py-1 text-sm" />
        </div>
      </div>
      <button type="button" class="inline-flex items-center gap-1 text-xs font-bold text-indigo-700" @click="addTo('slides', { image_url: '', mobile_image_url: '', heading: '', text: '', button_label: 'Mua ngay', link: '#products' })">
        <Plus class="h-3.5 w-3.5" /> Thêm ảnh
      </button>
    </template>

    <!-- ===== Ảnh + chữ ===== -->
    <template v-else-if="block.type === 'image_text'">
      <ImageField v-model="block.data.image_url" :site-id="siteId" label="Ảnh" />
      <div class="flex gap-1">
        <button
          v-for="o in [{ v: 'left', t: 'Ảnh bên trái' }, { v: 'right', t: 'Ảnh bên phải' }]"
          :key="o.v"
          type="button"
          class="flex-1 rounded-lg border px-2 py-1.5 text-xs font-bold"
          :class="(block.data.image_side || 'left') === o.v ? 'border-indigo-600 bg-indigo-50 text-indigo-700' : 'border-slate-200 text-slate-600'"
          @click="block.data.image_side = o.v"
        >
          {{ o.t }}
        </button>
      </div>
      <label class="space-y-1"><span :class="lbl">Dòng chữ nhỏ phía trên</span><input v-model="block.data.eyebrow" :class="inp" /></label>
      <label class="space-y-1"><span :class="lbl">Tiêu đề</span><input v-model="block.data.heading" :class="inp" /></label>
      <label class="space-y-1"><span :class="lbl">Nội dung</span><textarea v-model="block.data.body" rows="4" :class="inp" /></label>
      <span :class="lbl">Điểm nổi bật</span>
      <div
        v-for="(pt, pi) in list('points')"
        :key="pi"
        :class="rowClass('points', pi)"
        class="space-y-1.5"
        @dragover="itemDragOver('points', pi, $event)"
        @drop.prevent="itemDrop('points', pi)"
      >
        <div class="flex items-center gap-1.5">
          <span class="cursor-grab text-slate-400" draggable="true" @dragstart="itemDragStart('points', pi, $event)" @dragend="itemDragEnd"><GripVertical class="h-4 w-4" /></span>
          <input v-model="pt.title" placeholder="Tiêu đề" class="min-w-0 flex-1 rounded-lg border border-slate-200 px-2 py-1 text-sm" />
          <button type="button" class="rounded p-1 text-rose-600 hover:bg-rose-50" @click="removeFrom('points', pi)"><Trash2 class="h-3.5 w-3.5" /></button>
        </div>
        <input v-model="pt.text" placeholder="Mô tả ngắn" class="w-full rounded-lg border border-slate-200 px-2 py-1 text-sm" />
      </div>
      <button type="button" class="inline-flex items-center gap-1 text-xs font-bold text-indigo-700" @click="addTo('points', { title: '', text: '' })">
        <Plus class="h-3.5 w-3.5" /> Thêm điểm nổi bật
      </button>
      <div class="grid grid-cols-2 gap-2">
        <label class="space-y-1"><span :class="lbl">Chữ trên nút</span><input v-model="block.data.button_label" :class="inp" /></label>
        <label class="space-y-1"><span :class="lbl">Liên kết nút</span><input v-model="block.data.button_link" list="pp-page-links" :class="inp" /></label>
      </div>
    </template>

    <!-- ===== Ô danh mục ===== -->
    <template v-else-if="block.type === 'collections'">
      <label class="space-y-1"><span :class="lbl">Tiêu đề mục</span><input v-model="block.data.heading" :class="inp" /></label>
      <div
        v-for="(col, ci) in list('items')"
        :key="ci"
        :class="rowClass('items', ci)"
        class="space-y-2"
        @dragover="itemDragOver('items', ci, $event)"
        @drop.prevent="itemDrop('items', ci)"
      >
        <div class="flex items-center gap-1.5">
          <span class="cursor-grab text-slate-400" draggable="true" @dragstart="itemDragStart('items', ci, $event)" @dragend="itemDragEnd"><GripVertical class="h-4 w-4" /></span>
          <input v-model="col.title" placeholder="Tên danh mục" class="min-w-0 flex-1 rounded-lg border border-slate-200 px-2 py-1 text-sm" />
          <button type="button" class="rounded p-1 text-rose-600 hover:bg-rose-50" @click="removeFrom('items', ci)"><Trash2 class="h-3.5 w-3.5" /></button>
        </div>
        <ImageField v-model="col.image_url" :site-id="siteId" compact />
        <input v-model="col.link" list="pp-page-links" placeholder="Liên kết (#products, https://...)" class="w-full rounded-lg border border-slate-200 px-2 py-1 text-sm" />
      </div>
      <button type="button" class="inline-flex items-center gap-1 text-xs font-bold text-indigo-700" @click="addTo('items', { image_url: '', title: 'Danh mục mới', link: '#products' })">
        <Plus class="h-3.5 w-3.5" /> Thêm ô danh mục
      </button>
    </template>

    <!-- ===== Dải biểu tượng ===== -->
    <template v-else-if="block.type === 'icon_strip'">
      <div
        v-for="(ic, ii) in list('items')"
        :key="ii"
        :class="rowClass('items', ii)"
        class="space-y-1.5"
        @dragover="itemDragOver('items', ii, $event)"
        @drop.prevent="itemDrop('items', ii)"
      >
        <div class="flex items-center gap-1.5">
          <span class="cursor-grab text-slate-400" draggable="true" @dragstart="itemDragStart('items', ii, $event)" @dragend="itemDragEnd"><GripVertical class="h-4 w-4" /></span>
          <select v-model="ic.icon" class="w-28 rounded-lg border border-slate-200 px-1.5 py-1 text-xs">
            <option v-for="o in iconChoices" :key="o.key" :value="o.key">{{ o.label }}</option>
          </select>
          <input v-model="ic.title" placeholder="Tiêu đề" class="min-w-0 flex-1 rounded-lg border border-slate-200 px-2 py-1 text-sm" />
          <button type="button" class="rounded p-1 text-rose-600 hover:bg-rose-50" @click="removeFrom('items', ii)"><Trash2 class="h-3.5 w-3.5" /></button>
        </div>
        <input v-model="ic.text" placeholder="Mô tả ngắn" class="w-full rounded-lg border border-slate-200 px-2 py-1 text-sm" />
      </div>
      <button type="button" class="inline-flex items-center gap-1 text-xs font-bold text-indigo-700" @click="addTo('items', { icon: 'star', title: '', text: '' })">
        <Plus class="h-3.5 w-3.5" /> Thêm biểu tượng
      </button>
    </template>

    <!-- ===== Đăng ký email ===== -->
    <template v-else-if="block.type === 'newsletter'">
      <label class="space-y-1"><span :class="lbl">Tiêu đề</span><input v-model="block.data.heading" :class="inp" /></label>
      <label class="space-y-1"><span :class="lbl">Mô tả</span><textarea v-model="block.data.text" rows="2" :class="inp" /></label>
      <div class="grid grid-cols-2 gap-2">
        <label class="space-y-1"><span :class="lbl">Chữ gợi ý trong ô</span><input v-model="block.data.placeholder" :class="inp" /></label>
        <label class="space-y-1"><span :class="lbl">Chữ trên nút</span><input v-model="block.data.button_label" :class="inp" /></label>
      </div>
      <p class="rounded-lg bg-slate-50 px-3 py-2 text-xs text-slate-500">Email khách đăng ký xem ở trang <b>Đơn hàng → Email đăng ký</b>.</p>
    </template>

    <!-- ===== Chân trang ===== -->
    <template v-else-if="block.type === 'footer'">
      <label class="space-y-1"><span :class="lbl">Tên công ty / cửa hàng</span><input v-model="block.data.company" :class="inp" /></label>
      <label class="space-y-1"><span :class="lbl">Giới thiệu ngắn</span><textarea v-model="block.data.about_text" rows="2" :class="inp" /></label>
      <label class="space-y-1"><span :class="lbl">Địa chỉ</span><input v-model="block.data.address" :class="inp" /></label>
      <div class="grid grid-cols-2 gap-2">
        <label class="space-y-1"><span :class="lbl">Điện thoại</span><input v-model="block.data.phone" :class="inp" /></label>
        <label class="space-y-1"><span :class="lbl">Email</span><input v-model="block.data.email" :class="inp" /></label>
      </div>
      <span :class="lbl">Cột liên kết</span>
      <div v-for="(col, ci) in list('columns')" :key="ci" class="space-y-2 rounded-xl border border-slate-200 bg-slate-50 p-2.5">
        <div class="flex items-center gap-1.5">
          <input v-model="col.title" placeholder="Tiêu đề cột" class="min-w-0 flex-1 rounded-lg border border-slate-200 px-2 py-1 text-sm font-semibold" />
          <button type="button" class="rounded p-1 text-rose-600 hover:bg-rose-50" @click="removeFrom('columns', ci)"><Trash2 class="h-3.5 w-3.5" /></button>
        </div>
        <select :value="col.menu_id || ''" :data-testid="'footer-menu-select-' + ci" class="w-full rounded-lg border border-slate-200 bg-white px-2 py-1 text-xs" @change="col.menu_id = ($event.target as HTMLSelectElement).value">
          <option value="">Tự nhập liên kết</option>
          <option v-for="m in menus || []" :key="m.id" :value="m.id">Menu: {{ m.name }}</option>
        </select>
        <template v-if="menuOf(col.menu_id)">
          <p class="rounded-lg bg-indigo-50 px-2 py-1 text-[11px] text-indigo-800">Dùng menu "{{ menuOf(col.menu_id)?.name }}" — sửa ở tab <b>Menu</b>.</p>
        </template>
        <template v-else>
          <div v-for="(lnk, li) in col.links || []" :key="li" class="flex items-center gap-1">
            <input v-model="lnk.label" placeholder="Chữ" class="w-2/5 rounded-lg border border-slate-200 px-2 py-1 text-xs" />
            <input v-model="lnk.link" list="pp-page-links" placeholder="Liên kết" class="min-w-0 flex-1 rounded-lg border border-slate-200 px-2 py-1 text-xs" />
            <button type="button" class="rounded p-1 text-rose-600 hover:bg-rose-50" @click="col.links.splice(li, 1)"><Trash2 class="h-3 w-3" /></button>
          </div>
          <button type="button" class="text-[11px] font-bold text-indigo-700" @click="(col.links = col.links || []).push({ label: '', link: '' })">+ Thêm liên kết</button>
          <button v-if="(col.links || []).length" type="button" class="block text-[11px] font-bold text-slate-500 hover:text-indigo-700" @click="colToMenu(col)">Chuyển thành menu dùng chung</button>
        </template>
      </div>
      <button type="button" class="inline-flex items-center gap-1 text-xs font-bold text-indigo-700" @click="addTo('columns', { title: 'Cột mới', links: [] })">
        <Plus class="h-3.5 w-3.5" /> Thêm cột
      </button>
      <span :class="lbl">Mạng xã hội</span>
      <div class="space-y-1.5">
        <input
          v-for="s in [
            { k: 'facebook_link', p: 'Link Facebook' },
            { k: 'instagram_link', p: 'Link Instagram' },
            { k: 'tiktok_link', p: 'Link TikTok' },
            { k: 'youtube_link', p: 'Link YouTube' },
            { k: 'zalo_link', p: 'Link Zalo' }
          ]"
          :key="s.k"
          v-model="(block.data.socials = block.data.socials || {})[s.k]"
          :placeholder="s.p"
          class="w-full rounded-lg border border-slate-200 px-2 py-1 text-sm"
        />
      </div>
      <label class="space-y-1"><span :class="lbl">Dòng bản quyền</span><input v-model="block.data.copyright" :class="inp" /></label>
    </template>

    <!-- ===== Nền & khoảng cách (mọi section) ===== -->
    <div class="space-y-3 border-t border-slate-100 pt-4">
      <span class="block text-[11px] font-extrabold uppercase tracking-wide text-slate-400">Giao diện section</span>
      <div v-if="block.type !== 'header'" class="flex items-center gap-3">
        <span class="text-xs font-bold text-slate-600">Màu nền</span>
        <input type="color" :value="block.data.style?.bg || '#ffffff'" class="h-7 w-9 rounded border border-slate-200" @input="setStyle('bg', ($event.target as HTMLInputElement).value)" />
        <button v-if="block.data.style?.bg" type="button" class="text-xs text-slate-500 underline" @click="setStyle('bg', '')">Bỏ màu</button>
      </div>
      <div class="space-y-1">
        <span class="text-xs font-bold text-slate-600">Khoảng cách trên/dưới</span>
        <div class="flex gap-1">
          <button
            v-for="p in [{ v: 'none', t: 'Mặc định' }, { v: 'sm', t: 'Nhỏ' }, { v: 'md', t: 'Vừa' }, { v: 'lg', t: 'Lớn' }]"
            :key="p.v"
            type="button"
            class="flex-1 rounded-lg border px-1 py-1.5 text-[11px] font-bold"
            :class="(block.data.style?.pad || 'none') === p.v ? 'border-indigo-600 bg-indigo-50 text-indigo-700' : 'border-slate-200 text-slate-600'"
            @click="setStyle('pad', p.v)"
          >
            {{ p.t }}
          </button>
        </div>
      </div>
      <label class="block space-y-1">
        <span class="text-xs font-bold text-slate-600">CSS tuỳ chỉnh cho section này</span>
        <textarea
          :value="block.data.style?.css || ''"
          rows="6"
          spellcheck="false"
          :placeholder="cssPlaceholder"
          class="w-full rounded-lg border border-slate-200 bg-slate-900 px-2.5 py-2 font-mono text-[11px] leading-relaxed text-emerald-200 placeholder:text-slate-500 focus:border-indigo-500 focus:outline-none"
          @input="setStyle('css', ($event.target as HTMLTextAreaElement).value)"
        />
        <span class="block text-[10px] leading-snug text-slate-400">
          Chỉ áp dụng trong section này. Viết thuộc tính trực tiếp (áp cho cả section) hoặc kèm bộ chọn như <code>h2 { … }</code>, <code>a:hover { … }</code>, <code>&amp;:hover { … }</code>.
        </span>
      </label>
    </div>
  </div>
</template>
