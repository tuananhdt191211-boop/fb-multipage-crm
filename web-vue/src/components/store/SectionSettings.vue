<script setup lang="ts">
// Bảng cài đặt cho section đang chọn (cột phải của trình chỉnh sửa)
import { computed, ref } from 'vue'
import { GripVertical, Plus, Trash2 } from 'lucide-vue-next'
import ImageField from './ImageField.vue'
import { iconChoices } from '../../lib/blockFactory'
import type { StoreBlock } from '../../types/store'

const props = defineProps<{ block: StoreBlock; siteId: number | string; siteLogo?: string }>()

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

const cssPlaceholder = 'border-radius: 24px;\n\nh2 {\n  color: #be123c;\n  font-size: 48px;\n}\n\nbutton:hover {\n  transform: scale(1.03);\n}'

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
          <option value="left">Logo trái · Menu và nút bên phải</option>
          <option value="split">Logo trái · Menu giữa · Nút phải</option>
          <option value="center">Logo giữa · Menu bên dưới</option>
          <option value="center_split">Menu trái · Logo giữa · Nút phải</option>
        </select>
      </label>

      <div class="space-y-2">
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
            <input v-model="n.link" placeholder="#products, https://..." class="min-w-0 flex-1 rounded-lg border border-slate-200 px-2 py-1 text-sm" />
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
        <p v-if="block.data.right_mode === 'account'" class="text-[10px] leading-snug text-slate-400">
          Khách thấy giỏ hàng, nút Đăng nhập / Đăng ký; đã đăng nhập thì thấy tên mình, “Đơn hàng của tôi” và Đăng xuất.
        </p>
      </div>
      <template v-if="block.data.right_mode !== 'account'">
        <label class="flex items-center gap-2 text-xs font-bold text-slate-600">
          <input type="checkbox" :checked="block.data.show_cta !== false" @change="block.data.show_cta = ($event.target as HTMLInputElement).checked" />
          Hiện nút
        </label>
        <label v-if="block.data.show_cta !== false" class="space-y-1">
          <span :class="lbl">Liên kết nút</span>
          <input v-model="block.data.cta_link" placeholder="#products" :class="inp" />
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
    </template>

    <!-- ===== Banner ===== -->
    <template v-else-if="block.type === 'hero'">
      <ImageField v-model="block.data.image_url" :site-id="siteId" label="Ảnh banner" />
      <label class="space-y-1"><span :class="lbl">Nhãn nhỏ</span><input v-model="block.data.badge" :class="inp" /></label>
      <label class="space-y-1"><span :class="lbl">Tiêu đề</span><input v-model="block.data.title" :class="inp" /></label>
      <label class="space-y-1"><span :class="lbl">Mô tả</span><textarea v-model="block.data.subtitle" rows="3" :class="inp" /></label>
      <div class="grid grid-cols-2 gap-2">
        <label class="space-y-1"><span :class="lbl">Chữ trên nút</span><input v-model="block.data.cta_label" :class="inp" /></label>
        <label class="space-y-1"><span :class="lbl">Liên kết nút</span><input v-model="block.data.cta_link" :class="inp" /></label>
      </div>
    </template>

    <!-- ===== Sản phẩm ===== -->
    <template v-else-if="block.type === 'products'">
      <label class="space-y-1"><span :class="lbl">Tiêu đề mục</span><input v-model="block.data.heading" :class="inp" /></label>
      <label class="space-y-1">
        <span :class="lbl">Nguồn sản phẩm</span>
        <select v-model="block.data.source" :class="inp">
          <option value="catalog">Danh sách sản phẩm của shop</option>
          <option value="manual">Nhập tay trong section này</option>
        </select>
      </label>
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
            <span :class="lbl">Liên kết nút</span><input v-model="block.data.buy_link" placeholder="#contact, https://m.me/..." :class="inp" />
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
        <label class="space-y-1"><span :class="lbl">Liên kết nút</span><input v-model="block.data.button_link" placeholder="tel:, https://..." :class="inp" /></label>
      </div>
      <label class="space-y-1"><span :class="lbl">Ghi chú chân trang</span><input v-model="block.data.footer_note" :class="inp" /></label>
    </template>

    <!-- ===== Văn bản ===== -->
    <template v-else-if="block.type === 'richtext'">
      <label class="space-y-1"><span :class="lbl">Tiêu đề</span><input v-model="block.data.heading" :class="inp" /></label>
      <label class="space-y-1"><span :class="lbl">Nội dung</span><textarea v-model="block.data.body" rows="7" :class="inp" /></label>
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
      <label class="space-y-1"><span :class="lbl">Liên kết khi bấm ảnh</span><input v-model="block.data.link" placeholder="https://..." :class="inp" /></label>
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
      <label class="space-y-1"><span :class="lbl">Liên kết (không bắt buộc)</span><input v-model="block.data.link" placeholder="#products" :class="inp" /></label>
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
          <input v-model="s.link" placeholder="Liên kết" class="rounded-lg border border-slate-200 px-2 py-1 text-sm" />
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
        <label class="space-y-1"><span :class="lbl">Liên kết nút</span><input v-model="block.data.button_link" :class="inp" /></label>
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
        <input v-model="col.link" placeholder="Liên kết (#products, https://...)" class="w-full rounded-lg border border-slate-200 px-2 py-1 text-sm" />
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
        <div v-for="(lnk, li) in col.links || []" :key="li" class="flex items-center gap-1">
          <input v-model="lnk.label" placeholder="Chữ" class="w-2/5 rounded-lg border border-slate-200 px-2 py-1 text-xs" />
          <input v-model="lnk.link" placeholder="Liên kết" class="min-w-0 flex-1 rounded-lg border border-slate-200 px-2 py-1 text-xs" />
          <button type="button" class="rounded p-1 text-rose-600 hover:bg-rose-50" @click="col.links.splice(li, 1)"><Trash2 class="h-3 w-3" /></button>
        </div>
        <button type="button" class="text-[11px] font-bold text-indigo-700" @click="(col.links = col.links || []).push({ label: '', link: '' })">+ Thêm liên kết</button>
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
      <div class="flex items-center gap-3">
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
