<script setup lang="ts">
// Hiển thị website từ danh sách section.
// - Web công khai: chỉ đọc.
// - Trình chỉnh sửa (editable): click chọn section, sửa chữ trực tiếp, kéo thả đổi vị trí.
// Bố cục dùng container query (@container) nên xem trước chế độ điện thoại/máy tính bảng hiển thị đúng.
import { ref } from 'vue'
import { ArrowDown, ArrowUp, Copy, EyeOff, GripVertical, Plus, Trash2 } from 'lucide-vue-next'
import { safeHref, safeImg } from '../../lib/safeUrl'
import { formatVnd } from '../../lib/format'
import { blockLabels } from '../../lib/blockFactory'
import EditableText from './EditableText.vue'
import type { BlockStyle, PublicProduct, StoreBlock } from '../../types/store'

const props = defineProps<{
  siteTitle: string
  primaryColor: string
  blocks: StoreBlock[]
  products?: PublicProduct[]
  logoUrl?: string
  editable?: boolean
  selectedId?: string | null
  dragActive?: boolean
}>()

const emit = defineEmits<{
  (e: 'select', id: string | null): void
  (e: 'edit', id: string, path: (string | number)[], value: string): void
  (e: 'action', kind: 'up' | 'down' | 'duplicate' | 'hide' | 'delete', index: number): void
  (e: 'drop', index: number): void
  (e: 'section-dragstart', index: number, ev: DragEvent): void
  (e: 'section-dragend'): void
  (e: 'add-at', index: number): void
}>()

const padMap: Record<string, string> = { none: '0', sm: '16px', md: '32px', lg: '64px' }

function wrapStyle(blk: StoreBlock): Record<string, string> {
  const st = (blk.data?.style || {}) as BlockStyle
  const s: Record<string, string> = {}
  if (st.bg) s.backgroundColor = st.bg
  if (st.pad && padMap[st.pad] !== undefined) {
    s.paddingTop = padMap[st.pad]!
    s.paddingBottom = padMap[st.pad]!
  }
  // Ghim đầu trang khi cuộn (trong trình chỉnh sửa thì tắt để khung chọn không bị lệch)
  if (blk.type === 'header' && blk.data?.sticky && !props.editable) {
    s.position = 'sticky'
    s.top = '0'
    s.zIndex = '20'
    if (!s.backgroundColor) s.backgroundColor = '#ffffff'
  }
  return s
}

// ----- Sửa chữ trực tiếp -----
function set(blk: StoreBlock, path: (string | number)[], v: string) {
  emit('edit', blk.id, path, v)
}

// ----- Đầu trang -----
function navItems(blk: StoreBlock): { label: string; link: string; idx: number }[] {
  const arr = Array.isArray(blk.data.nav_items) ? blk.data.nav_items : []
  return arr
    .map((x: any, idx: number) =>
      typeof x === 'string'
        ? { label: x, link: '', idx }
        : { label: String(x?.label || ''), link: String(x?.link || ''), idx }
    )
    .filter((x: { label: string }) => props.editable || x.label.trim())
}

function logoSrc(blk: StoreBlock): string {
  return safeImg(blk.data.logo_url) || safeImg(props.logoUrl)
}

function logoH(blk: StoreBlock): number {
  const n = Number(blk.data.logo_height) || 40
  return Math.min(120, Math.max(20, n))
}

function brandSizeClass(size?: string): string {
  return size === 'sm' ? 'text-sm' : size === 'lg' ? 'text-xl' : 'text-base'
}

function headerLayout(layout?: string) {
  switch (layout) {
    case 'split':
      return { wrap: 'flex flex-wrap items-center gap-4 @3xl:grid @3xl:grid-cols-[auto_1fr_auto]', brand: '', nav: 'justify-center', cta: '' }
    case 'center':
      return { wrap: 'flex flex-wrap items-center justify-center gap-x-6 gap-y-3', brand: 'basis-full justify-center', nav: 'justify-center', cta: '' }
    case 'center_split':
      return {
        wrap: 'flex flex-wrap items-center justify-center gap-4 @3xl:grid @3xl:grid-cols-[1fr_auto_1fr]',
        brand: '@3xl:order-2 justify-center',
        nav: '@3xl:order-1 justify-start',
        cta: '@3xl:order-3 @3xl:justify-self-end'
      }
    default:
      return { wrap: 'flex flex-wrap items-center gap-x-6 gap-y-3', brand: 'mr-auto', nav: '', cta: '' }
  }
}

function alignOf(v?: string) {
  return v === 'center' || v === 'right' ? v : 'left'
}

function clampHeight(h?: number) {
  const n = Number(h) || 300
  return Math.min(2000, Math.max(80, n))
}

function htmlDoc(html?: string) {
  return (
    '<!doctype html><html><head><meta charset="utf-8">' +
    '<meta name="viewport" content="width=device-width,initial-scale=1">' +
    '<base target="_blank">' +
    '<style>body{margin:0;font-family:system-ui,sans-serif}</style></head><body>' +
    (html || '') +
    '</body></html>'
  )
}

// ----- Chế độ chỉnh sửa: chọn, kéo thả -----
const hoverId = ref<string | null>(null)
const dropIndex = ref<number | null>(null)

const visibleBlocks = () => props.blocks.map((blk, index) => ({ blk, index })).filter((x) => props.editable || x.blk.enabled)

function onSectionClick(blk: StoreBlock) {
  if (props.editable) emit('select', blk.id)
}

function onSectionDragOver(index: number, ev: DragEvent) {
  if (!props.editable || !props.dragActive) return
  ev.preventDefault()
  const el = ev.currentTarget as HTMLElement
  const rect = el.getBoundingClientRect()
  dropIndex.value = ev.clientY < rect.top + rect.height / 2 ? index : index + 1
}

function onCanvasDragOver(ev: DragEvent) {
  if (!props.editable || !props.dragActive) return
  ev.preventDefault()
  if (!props.blocks.length) dropIndex.value = 0
}

function onCanvasDrop() {
  if (dropIndex.value === null) return
  const i = dropIndex.value
  dropIndex.value = null
  emit('drop', i)
}

function onCanvasDragLeave(ev: DragEvent) {
  const root = ev.currentTarget as HTMLElement
  if (!root.contains(ev.relatedTarget as Node | null)) dropIndex.value = null
}

// Trong trình chỉnh sửa: không cho link chuyển trang
function onCanvasClickCapture(ev: MouseEvent) {
  if (!props.editable) return
  const a = (ev.target as HTMLElement).closest('a')
  if (a) ev.preventDefault()
}

function onBackgroundClick(ev: MouseEvent) {
  if (props.editable && ev.target === ev.currentTarget) emit('select', null)
}
</script>

<template>
  <main
    class="@container mx-auto max-w-6xl bg-white text-slate-900"
    :class="editable ? 'min-h-full pb-24' : ''"
    @click.capture="onCanvasClickCapture"
    @click="onBackgroundClick"
    @dragover="onCanvasDragOver"
    @drop.prevent="onCanvasDrop"
    @dragleave="onCanvasDragLeave"
  >
    <!-- Trang trống trong trình chỉnh sửa -->
    <div
      v-if="editable && !blocks.length"
      class="m-6 flex flex-col items-center justify-center gap-3 rounded-2xl border-2 border-dashed py-24 text-center text-sm"
      :class="dragActive && dropIndex === 0 ? 'border-indigo-500 bg-indigo-50 text-indigo-700' : 'border-slate-300 text-slate-500'"
    >
      <p class="font-semibold">Trang đang trống</p>
      <p class="text-xs">Kéo một section từ thư viện bên trái thả vào đây, hoặc</p>
      <button type="button" class="rounded-xl bg-indigo-600 px-4 py-2 text-xs font-bold text-white" @click.stop="emit('add-at', 0)">
        + Thêm section
      </button>
    </div>

    <template v-for="{ blk, index } in visibleBlocks()" :key="blk.id">
      <div
        :data-block-id="blk.id"
        :style="wrapStyle(blk)"
        :class="
          editable
            ? [
                'relative transition-opacity',
                !blk.enabled ? 'opacity-40' : '',
                selectedId === blk.id ? 'pp-selected' : ''
              ]
            : ''
        "
        @click="onSectionClick(blk)"
        @mouseenter="editable && (hoverId = blk.id)"
        @mouseleave="editable && hoverId === blk.id && (hoverId = null)"
        @dragover="onSectionDragOver(index, $event)"
      >
        <!-- ===== Khung chọn & thanh công cụ (chỉ trong trình chỉnh sửa) ===== -->
        <template v-if="editable">
          <div
            v-if="dragActive && dropIndex === index"
            class="pointer-events-none absolute inset-x-0 -top-[3px] z-40 h-[6px] rounded-full bg-indigo-500 shadow-[0_0_0_3px_rgba(99,102,241,0.25)]"
          />
          <div
            v-if="dragActive && dropIndex === index + 1"
            class="pointer-events-none absolute inset-x-0 -bottom-[3px] z-40 h-[6px] rounded-full bg-indigo-500 shadow-[0_0_0_3px_rgba(99,102,241,0.25)]"
          />
          <div
            class="pointer-events-none absolute inset-0 z-30 transition-shadow"
            :class="
              selectedId === blk.id
                ? 'shadow-[inset_0_0_0_2px_rgb(79,70,229)]'
                : hoverId === blk.id && !dragActive
                  ? 'shadow-[inset_0_0_0_1px_rgb(129,140,248)]'
                  : ''
            "
          />
          <div
            v-if="(selectedId === blk.id || hoverId === blk.id) && !dragActive"
            class="absolute left-2 top-2 z-40 flex items-center gap-0.5 rounded-lg bg-indigo-600 px-1 py-0.5 text-white shadow-lg"
            @click.stop
          >
            <span
              class="cursor-grab rounded p-1 hover:bg-white/20 active:cursor-grabbing"
              draggable="true"
              title="Kéo để đổi vị trí"
              @dragstart="emit('section-dragstart', index, $event)"
              @dragend="emit('section-dragend')"
            >
              <GripVertical class="h-3.5 w-3.5" />
            </span>
            <span class="px-1 text-[11px] font-bold">{{ blockLabels[blk.type] || blk.type }}</span>
            <span v-if="!blk.enabled" class="rounded bg-white/20 px-1 text-[10px] font-semibold">Đang ẩn</span>
            <button type="button" class="rounded p-1 hover:bg-white/20 disabled:opacity-30" :disabled="index === 0" title="Lên" @click="emit('action', 'up', index)">
              <ArrowUp class="h-3.5 w-3.5" />
            </button>
            <button type="button" class="rounded p-1 hover:bg-white/20 disabled:opacity-30" :disabled="index === blocks.length - 1" title="Xuống" @click="emit('action', 'down', index)">
              <ArrowDown class="h-3.5 w-3.5" />
            </button>
            <button type="button" class="rounded p-1 hover:bg-white/20" title="Nhân đôi" @click="emit('action', 'duplicate', index)">
              <Copy class="h-3.5 w-3.5" />
            </button>
            <button type="button" class="rounded p-1 hover:bg-white/20" :title="blk.enabled ? 'Ẩn section' : 'Hiện section'" @click="emit('action', 'hide', index)">
              <EyeOff class="h-3.5 w-3.5" />
            </button>
            <button type="button" class="rounded p-1 hover:bg-rose-500" title="Xoá" @click="emit('action', 'delete', index)">
              <Trash2 class="h-3.5 w-3.5" />
            </button>
          </div>
          <button
            v-if="selectedId === blk.id && !dragActive"
            type="button"
            class="absolute -bottom-3 left-1/2 z-40 flex h-6 -translate-x-1/2 items-center gap-1 rounded-full bg-indigo-600 px-2.5 text-[11px] font-bold text-white shadow-lg hover:bg-indigo-700"
            title="Thêm section bên dưới"
            @click.stop="emit('add-at', index + 1)"
          >
            <Plus class="h-3 w-3" /> Thêm
          </button>
        </template>

        <!-- ===== Đầu trang ===== -->
        <header
          v-if="blk.type === 'header'"
          class="px-6 py-4"
          :class="blk.data.border === false ? '' : 'border-b border-slate-100'"
          :style="blk.data.text_color ? { color: blk.data.text_color } : undefined"
        >
          <div :class="headerLayout(blk.data.layout).wrap">
            <div class="flex items-center gap-2" :class="headerLayout(blk.data.layout).brand">
              <img
                v-if="logoSrc(blk)"
                :src="logoSrc(blk)"
                :alt="blk.data.brand_name || siteTitle"
                class="w-auto max-w-[200px] object-contain"
                :style="{ height: logoH(blk) + 'px' }"
              />
              <EditableText
                v-if="blk.data.show_brand !== false || !logoSrc(blk)"
                :model-value="blk.data.brand_name || (editable ? '' : siteTitle)"
                :editable="editable"
                placeholder="Tên thương hiệu"
                class="font-extrabold"
                :class="[brandSizeClass(blk.data.brand_size), blk.data.text_color ? '' : 'text-slate-900']"
                @update:model-value="set(blk, ['brand_name'], $event)"
              />
            </div>

            <nav
              v-if="navItems(blk).length"
              class="flex flex-wrap items-center gap-x-5 gap-y-1 text-sm"
              :class="[headerLayout(blk.data.layout).nav, blk.data.text_color ? '' : 'text-slate-600']"
            >
              <template v-for="item in navItems(blk)" :key="item.idx">
                <a v-if="item.link || editable" :href="safeHref(item.link)" class="hover:underline">
                  <EditableText
                    :model-value="item.label"
                    :editable="editable"
                    placeholder="Mục menu"
                    @update:model-value="set(blk, ['nav_items', item.idx, 'label'], $event)"
                  />
                </a>
                <span v-else>{{ item.label }}</span>
              </template>
            </nav>

            <a
              v-if="blk.data.show_cta !== false"
              :href="safeHref(blk.data.cta_link, '#products')"
              class="rounded-xl px-4 py-2 text-xs font-bold text-white"
              :class="headerLayout(blk.data.layout).cta"
              :style="{ backgroundColor: primaryColor }"
            >
              <EditableText
                :model-value="blk.data.cta_label || (editable ? '' : 'Đặt mua ngay')"
                :editable="editable"
                placeholder="Chữ trên nút"
                @update:model-value="set(blk, ['cta_label'], $event)"
              />
            </a>
          </div>
        </header>

        <!-- ===== Banner chính ===== -->
        <section v-else-if="blk.type === 'hero'" class="grid grid-cols-1 items-center gap-8 px-6 py-12 @3xl:grid-cols-2">
          <div class="space-y-4">
            <span
              v-if="blk.data.badge || editable"
              class="inline-block rounded-full px-3 py-1 text-xs font-bold text-white"
              :style="{ backgroundColor: primaryColor }"
            >
              <EditableText :model-value="blk.data.badge" :editable="editable" placeholder="Nhãn nhỏ" @update:model-value="set(blk, ['badge'], $event)" />
            </span>
            <EditableText
              tag="h1"
              :model-value="blk.data.title"
              :editable="editable"
              placeholder="Tiêu đề chính"
              class="block text-2xl font-extrabold leading-tight text-slate-900 @xl:text-3xl"
              @update:model-value="set(blk, ['title'], $event)"
            />
            <EditableText
              tag="p"
              multiline
              :model-value="blk.data.subtitle"
              :editable="editable"
              placeholder="Mô tả ngắn"
              class="block whitespace-pre-line text-sm leading-relaxed text-slate-600"
              @update:model-value="set(blk, ['subtitle'], $event)"
            />
            <div class="pt-1">
              <a
                :href="safeHref(blk.data.cta_link, '#products')"
                class="inline-block rounded-xl px-5 py-2.5 text-sm font-bold text-white"
                :style="{ backgroundColor: primaryColor }"
              >
                <EditableText
                  :model-value="blk.data.cta_label || (editable ? '' : 'Khám phá ngay')"
                  :editable="editable"
                  placeholder="Chữ trên nút"
                  @update:model-value="set(blk, ['cta_label'], $event)"
                />
              </a>
            </div>
          </div>
          <div v-if="safeImg(blk.data.image_url)" class="overflow-hidden rounded-2xl border border-slate-200 bg-white">
            <img :src="safeImg(blk.data.image_url)" :alt="blk.data.title" class="h-full w-full object-cover" />
          </div>
          <div
            v-else-if="editable"
            class="flex aspect-[4/3] items-center justify-center rounded-2xl border-2 border-dashed border-slate-200 bg-slate-50 text-xs text-slate-400"
          >
            Chọn ảnh banner ở bảng bên phải
          </div>
        </section>

        <!-- ===== Sản phẩm ===== -->
        <section v-else-if="blk.type === 'products'" id="products" class="space-y-5 px-6 py-12">
          <EditableText
            tag="h2"
            :model-value="blk.data.heading || (editable ? '' : 'Sản phẩm nổi bật')"
            :editable="editable"
            placeholder="Tiêu đề mục"
            class="block text-xl font-extrabold text-slate-900"
            @update:model-value="set(blk, ['heading'], $event)"
          />

          <template v-if="blk.data.source === 'catalog'">
            <p v-if="!products?.length" class="text-sm text-slate-500">
              {{ editable ? 'Chưa có sản phẩm. Thêm sản phẩm ở trang "Quản lý sản phẩm".' : 'Sản phẩm sắp ra mắt.' }}
            </p>
            <div class="grid grid-cols-1 gap-5 @xl:grid-cols-2 @5xl:grid-cols-3">
              <div v-for="prod in products || []" :key="prod.id" class="space-y-2.5 rounded-2xl border border-slate-200 bg-white p-4 shadow-2xs">
                <div v-if="safeImg(prod.image_url)" class="relative aspect-square overflow-hidden rounded-xl bg-slate-100">
                  <img :src="safeImg(prod.image_url)" :alt="prod.name" class="h-full w-full object-cover" />
                  <span v-if="!prod.in_stock" class="absolute left-2 top-2 rounded-full bg-slate-900/80 px-2 py-0.5 text-[10px] font-bold text-white">
                    Hết hàng
                  </span>
                </div>
                <div class="text-sm font-bold text-slate-900">{{ prod.name }}</div>
                <div class="flex items-baseline gap-2">
                  <span class="text-sm font-extrabold" :style="{ color: primaryColor }">{{ formatVnd(prod.price) }}</span>
                  <span v-if="prod.compare_price > prod.price" class="text-xs text-slate-400 line-through">{{ formatVnd(prod.compare_price) }}</span>
                </div>
                <p class="text-xs leading-relaxed text-slate-600">{{ prod.description }}</p>
              </div>
            </div>
          </template>

          <div v-else class="grid grid-cols-1 gap-5 @xl:grid-cols-2 @5xl:grid-cols-3">
            <div
              v-for="(prod, pIdx) in blk.data.items || []"
              :key="prod.id || pIdx"
              class="space-y-2.5 rounded-2xl border border-slate-200 bg-white p-4 shadow-2xs"
            >
              <div v-if="safeImg(prod.image_url)" class="aspect-square overflow-hidden rounded-xl bg-slate-100">
                <img :src="safeImg(prod.image_url)" :alt="prod.name" class="h-full w-full object-cover" />
              </div>
              <EditableText tag="div" :model-value="prod.name" :editable="editable" placeholder="Tên sản phẩm" class="text-sm font-bold text-slate-900" @update:model-value="set(blk, ['items', pIdx, 'name'], $event)" />
              <EditableText tag="div" :model-value="prod.price" :editable="editable" placeholder="Giá" class="text-sm font-extrabold" :style="{ color: primaryColor }" @update:model-value="set(blk, ['items', pIdx, 'price'], $event)" />
              <EditableText tag="p" multiline :model-value="prod.desc" :editable="editable" placeholder="Mô tả" class="text-xs leading-relaxed text-slate-600" @update:model-value="set(blk, ['items', pIdx, 'desc'], $event)" />
            </div>
          </div>
        </section>

        <!-- ===== Cam kết ===== -->
        <section v-else-if="blk.type === 'features'" class="space-y-4 px-6 py-10">
          <EditableText
            tag="h2"
            :model-value="blk.data.heading || (editable ? '' : 'Cam kết dịch vụ')"
            :editable="editable"
            placeholder="Tiêu đề mục"
            class="block text-lg font-extrabold text-slate-900"
            @update:model-value="set(blk, ['heading'], $event)"
          />
          <div class="grid grid-cols-1 gap-4 @xl:grid-cols-3">
            <div v-for="(feat, fIdx) in blk.data.items || []" :key="fIdx" class="rounded-xl border border-slate-200 bg-white p-4 text-sm font-semibold text-slate-700">
              ✓ <EditableText :model-value="feat" :editable="editable" placeholder="Cam kết" @update:model-value="set(blk, ['items', fIdx], $event)" />
            </div>
          </div>
        </section>

        <!-- ===== Đánh giá ===== -->
        <section v-else-if="blk.type === 'testimonials'" class="space-y-4 px-6 py-10">
          <EditableText
            tag="h2"
            :model-value="blk.data.heading || (editable ? '' : 'Khách hàng nói gì')"
            :editable="editable"
            placeholder="Tiêu đề mục"
            class="block text-lg font-extrabold text-slate-900"
            @update:model-value="set(blk, ['heading'], $event)"
          />
          <div class="grid grid-cols-1 gap-4 @xl:grid-cols-3">
            <figure v-for="(t, tIdx) in blk.data.items || []" :key="tIdx" class="rounded-xl border border-slate-200 bg-white p-4 text-sm text-slate-700">
              <blockquote>
                “<EditableText :model-value="t.quote" :editable="editable" multiline placeholder="Nội dung đánh giá" @update:model-value="set(blk, ['items', tIdx, 'quote'], $event)" />”
              </blockquote>
              <EditableText tag="figcaption" :model-value="t.name" :editable="editable" placeholder="Tên khách" class="mt-2 block text-xs font-bold text-slate-500" @update:model-value="set(blk, ['items', tIdx, 'name'], $event)" />
            </figure>
          </div>
        </section>

        <!-- ===== Văn bản ===== -->
        <section v-else-if="blk.type === 'richtext'" class="space-y-3 px-6 py-10" :style="{ textAlign: alignOf(blk.data.align) }">
          <EditableText
            v-if="blk.data.heading || editable"
            tag="h2"
            :model-value="blk.data.heading"
            :editable="editable"
            placeholder="Tiêu đề (không bắt buộc)"
            class="block text-xl font-extrabold text-slate-900"
            @update:model-value="set(blk, ['heading'], $event)"
          />
          <EditableText
            tag="p"
            multiline
            :model-value="blk.data.body"
            :editable="editable"
            placeholder="Nhập nội dung..."
            class="block whitespace-pre-line text-sm leading-relaxed text-slate-600"
            @update:model-value="set(blk, ['body'], $event)"
          />
        </section>

        <!-- ===== Ảnh lớn ===== -->
        <section v-else-if="blk.type === 'image'" class="px-6 py-6">
          <component :is="blk.data.link ? 'a' : 'div'" :href="blk.data.link ? safeHref(blk.data.link) : undefined" class="block overflow-hidden rounded-2xl">
            <img v-if="safeImg(blk.data.image_url)" :src="safeImg(blk.data.image_url)" :alt="blk.data.alt || ''" class="w-full object-cover" />
            <div v-else class="flex h-40 items-center justify-center bg-slate-100 text-xs text-slate-400">
              {{ editable ? 'Chọn ảnh ở bảng bên phải' : 'Chưa chọn ảnh' }}
            </div>
          </component>
        </section>

        <!-- ===== Thư viện ảnh ===== -->
        <section v-else-if="blk.type === 'gallery'" class="space-y-4 px-6 py-10">
          <EditableText
            v-if="blk.data.heading || editable"
            tag="h2"
            :model-value="blk.data.heading"
            :editable="editable"
            placeholder="Tiêu đề mục"
            class="block text-lg font-extrabold text-slate-900"
            @update:model-value="set(blk, ['heading'], $event)"
          />
          <div class="grid grid-cols-2 gap-3 @xl:grid-cols-3">
            <figure v-for="(g, gIdx) in blk.data.items || []" :key="gIdx" class="overflow-hidden rounded-xl border border-slate-200 bg-white">
              <img v-if="safeImg(g.image_url)" :src="safeImg(g.image_url)" :alt="g.caption || ''" class="aspect-square w-full object-cover" />
              <div v-else class="flex aspect-square items-center justify-center bg-slate-100 text-xs text-slate-400">Chưa chọn ảnh</div>
              <EditableText
                v-if="g.caption || editable"
                tag="figcaption"
                :model-value="g.caption"
                :editable="editable"
                placeholder="Chú thích"
                class="block p-2 text-xs text-slate-600"
                @update:model-value="set(blk, ['items', gIdx, 'caption'], $event)"
              />
            </figure>
          </div>
        </section>

        <!-- ===== Câu hỏi thường gặp ===== -->
        <section v-else-if="blk.type === 'faq'" class="space-y-3 px-6 py-10">
          <EditableText
            v-if="blk.data.heading || editable"
            tag="h2"
            :model-value="blk.data.heading"
            :editable="editable"
            placeholder="Tiêu đề mục"
            class="block text-lg font-extrabold text-slate-900"
            @update:model-value="set(blk, ['heading'], $event)"
          />
          <!-- Trong trình chỉnh sửa luôn mở để sửa được câu trả lời -->
          <details v-for="(f, fqIdx) in blk.data.items || []" :key="fqIdx" class="rounded-xl border border-slate-200 bg-white p-4" :open="editable || undefined">
            <summary class="cursor-pointer text-sm font-bold text-slate-800">
              <EditableText :model-value="f.q" :editable="editable" placeholder="Câu hỏi" @update:model-value="set(blk, ['items', fqIdx, 'q'], $event)" />
            </summary>
            <EditableText tag="p" multiline :model-value="f.a" :editable="editable" placeholder="Câu trả lời" class="mt-2 block whitespace-pre-line text-sm leading-relaxed text-slate-600" @update:model-value="set(blk, ['items', fqIdx, 'a'], $event)" />
          </details>
        </section>

        <!-- ===== Liên hệ ===== -->
        <section v-else-if="blk.type === 'contact'" class="space-y-3 px-6 py-10">
          <EditableText
            v-if="blk.data.heading || editable"
            tag="h2"
            :model-value="blk.data.heading"
            :editable="editable"
            placeholder="Tiêu đề mục"
            class="block text-lg font-extrabold text-slate-900"
            @update:model-value="set(blk, ['heading'], $event)"
          />
          <ul class="space-y-1.5 text-sm text-slate-700">
            <li v-if="blk.data.phone">📞 <a :href="safeHref('tel:' + blk.data.phone)" class="font-semibold hover:underline">{{ blk.data.phone }}</a></li>
            <li v-if="blk.data.email">✉️ <a :href="safeHref('mailto:' + blk.data.email)" class="font-semibold hover:underline">{{ blk.data.email }}</a></li>
            <li v-if="blk.data.address">📍 {{ blk.data.address }}</li>
            <li v-if="blk.data.zalo_link">💬 <a :href="safeHref(blk.data.zalo_link)" target="_blank" rel="noopener" class="font-semibold hover:underline">Nhắn Zalo</a></li>
            <li v-if="editable && !blk.data.phone && !blk.data.email && !blk.data.address && !blk.data.zalo_link" class="text-xs text-slate-400">
              Nhập số điện thoại, email, địa chỉ ở bảng bên phải
            </li>
          </ul>
        </section>

        <!-- ===== HTML tự viết: chạy trong iframe cách ly ===== -->
        <section v-else-if="blk.type === 'html'" class="relative px-6 py-4">
          <iframe
            :srcdoc="htmlDoc(blk.data.html)"
            sandbox="allow-scripts allow-popups"
            referrerpolicy="no-referrer"
            loading="lazy"
            title="Nội dung nhúng"
            class="w-full rounded-xl border-0"
            :class="editable ? 'pointer-events-none' : ''"
            :style="{ height: clampHeight(blk.data.height) + 'px' }"
          ></iframe>
        </section>

        <!-- ===== Kêu gọi hành động & chân trang ===== -->
        <footer v-else-if="blk.type === 'cta'" id="contact" class="space-y-3 border-t border-slate-100 px-6 py-12 text-center">
          <EditableText tag="h2" :model-value="blk.data.title" :editable="editable" placeholder="Tiêu đề" class="block text-xl font-extrabold text-slate-900" @update:model-value="set(blk, ['title'], $event)" />
          <EditableText tag="p" multiline :model-value="blk.data.subtitle" :editable="editable" placeholder="Mô tả" class="mx-auto block max-w-lg text-sm text-slate-600" @update:model-value="set(blk, ['subtitle'], $event)" />
          <div>
            <a :href="safeHref(blk.data.button_link)" class="inline-block rounded-xl px-5 py-2.5 text-sm font-bold text-white" :style="{ backgroundColor: primaryColor }">
              <EditableText
                :model-value="blk.data.button_text || (editable ? '' : 'Liên hệ ngay')"
                :editable="editable"
                placeholder="Chữ trên nút"
                @update:model-value="set(blk, ['button_text'], $event)"
              />
            </a>
          </div>
          <EditableText
            v-if="blk.data.footer_note || editable"
            tag="p"
            :model-value="blk.data.footer_note"
            :editable="editable"
            placeholder="Ghi chú chân trang"
            class="block pt-4 text-xs text-slate-400"
            @update:model-value="set(blk, ['footer_note'], $event)"
          />
        </footer>
      </div>
    </template>
  </main>
</template>
