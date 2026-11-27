<script setup lang="ts">
// Hiển thị website từ danh sách section.
// - Web công khai: chỉ đọc.
// - Trình chỉnh sửa (editable): click chọn section, sửa chữ trực tiếp, kéo thả đổi vị trí.
// Bố cục dùng container query (@container) nên xem trước chế độ điện thoại/máy tính bảng hiển thị đúng.
// Nền mỗi section trải hết chiều ngang, nội dung nằm trong khung max-w-6xl ở giữa.
import { storePath } from '../../lib/storeHost'
import { computed, nextTick, onMounted, ref, watchEffect } from 'vue'
import {
  ArrowDown,
  ArrowUp,
  ChevronDown,
  ChevronLeft,
  ChevronRight,
  Copy,
  Eye,
  EyeOff,
  GripVertical,
  LogOut,
  Mail,
  Package,
  MapPin,
  Menu,
  MessageCircle,
  Phone,
  Plus,
  Search,
  ShoppingBag,
  Trash2,
  User,
  X
} from 'lucide-vue-next'
import { safeHref, safeImg } from '../../lib/safeUrl'
import { formatVnd } from '../../lib/format'
import { blockLabels } from '../../lib/blockFactory'
import { useCart } from '../../lib/cart'
import { customerState, useCustomer } from '../../lib/customer'
import EditableText from './EditableText.vue'
import BannerSlider from './BannerSlider.vue'
import NewsletterForm from './NewsletterForm.vue'
import StoreIcon from './StoreIcon.vue'
import { pathKey, scopedSectionCss } from '../../lib/textStyle'
import { themeCss, usedFonts } from '../../lib/theme'
import { ensureFonts } from '../../lib/fonts'
import type { BlockStyle, PublicProduct, SiteTheme, StoreBlock, TextFx, TextStyle } from '../../types/store'

const props = defineProps<{
  siteTitle: string
  primaryColor: string
  blocks: StoreBlock[]
  products?: PublicProduct[]
  logoUrl?: string
  siteSlug?: string
  editable?: boolean
  selectedId?: string | null
  dragActive?: boolean
  theme?: SiteTheme | null
}>()

// Kiểu chữ toàn trang + tải font Google đang dùng
const themeStyle = computed(() => themeCss(props.theme))
watchEffect(() => ensureFonts(usedFonts(props.theme, props.blocks)))

const emit = defineEmits<{
  (e: 'select', id: string | null): void
  (e: 'edit', id: string, path: (string | number)[], value: string | boolean): void
  (e: 'action', kind: 'up' | 'down' | 'duplicate' | 'hide' | 'delete', index: number): void
  (e: 'drop', index: number): void
  (e: 'section-dragstart', index: number, ev: DragEvent): void
  (e: 'section-dragend'): void
  (e: 'add-at', index: number): void
}>()

const padMap: Record<string, string> = { none: '0', sm: '16px', md: '32px', lg: '64px' }

const accent = computed(() => props.primaryColor || '#059669')
// Màu nhạt của màu chủ đạo (dùng cho nền thẻ, icon)
function tint(pct: number) {
  return `color-mix(in srgb, ${accent.value} ${pct}%, white)`
}

function wrapStyle(blk: StoreBlock): Record<string, string> {
  const st = (blk.data?.style || {}) as BlockStyle
  const s: Record<string, string> = {}
  if (st.bg) s.backgroundColor = st.bg
  if (st.pad && padMap[st.pad] !== undefined) {
    s.paddingTop = padMap[st.pad]!
    s.paddingBottom = padMap[st.pad]!
  }
  if (blk.type === 'header' && blk.data?.sticky && !props.editable) {
    s.position = 'sticky'
    s.top = '0'
    s.zIndex = '20'
    if (!s.backgroundColor) s.backgroundColor = 'rgba(255,255,255,0.92)'
    s.backdropFilter = 'blur(8px)'
  }
  return s
}

function set(blk: StoreBlock, path: (string | number)[], v: string) {
  emit('edit', blk.id, path, v)
}

// Kiểu chữ tuỳ chỉnh của một đoạn chữ (block.data._text["đường.dẫn"])
function fx(blk: StoreBlock, path: (string | number)[]): TextFx {
  const key = pathKey(path)
  return { block: blk.id, key, style: blk.data?._text?.[key] as TextStyle | undefined }
}

// CSS tuỳ chỉnh của section (chỉ áp dụng bên trong section đó)
function sectionCss(blk: StoreBlock): string {
  return scopedSectionCss(blk.id, (blk.data?.style as BlockStyle | undefined)?.css)
}

// ----- Giỏ hàng & sản phẩm (chỉ trên web công khai) -----
const cart = props.siteSlug && !props.editable ? useCart(props.siteSlug) : null
const cartCount = computed(() => cart?.count.value || 0)

function productLink(id: number) {
  return props.siteSlug && !props.editable ? storePath(props.siteSlug, `/p/${id}`) : undefined
}

function addToCart(p: PublicProduct) {
  if (cart && p.in_stock) cart.add(p)
}

function openCart() {
  if (cart) cart.state.open = true
}

// ----- Tài khoản khách (chỉ trên web công khai) -----
const cust = props.siteSlug && !props.editable ? useCustomer(props.siteSlug) : null
const accountOpen = ref(false)
const customerName = computed(() => (cust ? customerState.customer?.name || '' : ''))
// Tên gọi (từ cuối của họ tên Việt Nam), vd. "Nguyễn Thu Trang" -> "Trang"
const firstName = computed(() => customerName.value.trim().split(/\s+/).pop() || '')
const accountPath = computed(() => (props.siteSlug ? storePath(props.siteSlug, '/account') : '/'))
function openAuth(mode: 'login' | 'register') {
  if (cust) customerState.dialog = mode
}
function logoutCustomer() {
  cust?.logout()
  accountOpen.value = false
}
onMounted(() => cust?.refresh())

function discount(p: { price: number; compare_price: number }) {
  return p.compare_price > p.price ? Math.round((1 - p.price / p.compare_price) * 100) : 0
}

// ----- Tìm kiếm sản phẩm ở đầu trang -----
const searchOpen = ref<string | null>(null)
const searchText = ref('')
const searchInput = ref<HTMLInputElement[] | HTMLInputElement | null>(null)
const searchResults = computed(() => {
  const q = searchText.value.trim().toLowerCase()
  if (!q) return []
  return (props.products || []).filter((p) => p.name.toLowerCase().includes(q)).slice(0, 6)
})
function toggleSearch(id: string) {
  searchOpen.value = searchOpen.value === id ? null : id
  searchText.value = ''
  nextTick(() => {
    const el = Array.isArray(searchInput.value) ? searchInput.value[0] : searchInput.value
    el?.focus()
  })
}

// ----- Đánh giá dạng trượt -----
function scrollCarousel(ev: MouseEvent, dir: 1 | -1) {
  const wrap = (ev.currentTarget as HTMLElement).closest('[data-carousel]')?.querySelector('[data-track]') as HTMLElement | null
  if (wrap) wrap.scrollBy({ left: dir * wrap.clientWidth * 0.85, behavior: 'smooth' })
}

// ----- Chân trang -----
const socialDefs = [
  { key: 'facebook_link', label: 'Facebook', short: 'f' },
  { key: 'instagram_link', label: 'Instagram', short: 'IG' },
  { key: 'tiktok_link', label: 'TikTok', short: 'TT' },
  { key: 'youtube_link', label: 'YouTube', short: 'YT' },
  { key: 'zalo_link', label: 'Zalo', short: 'Zalo' }
]
function socials(blk: StoreBlock) {
  const s = blk.data.socials || {}
  return socialDefs.filter((d) => String(s[d.key] || '').trim()).map((d) => ({ ...d, href: safeHref(s[d.key]) }))
}

// ----- Đầu trang -----
function navItems(blk: StoreBlock): { label: string; link: string; idx: number }[] {
  const arr = Array.isArray(blk.data.nav_items) ? blk.data.nav_items : []
  return arr
    .map((x: any, idx: number) =>
      typeof x === 'string' ? { label: x, link: '', idx } : { label: String(x?.label || ''), link: String(x?.link || ''), idx }
    )
    .filter((x: { label: string }) => props.editable || x.label.trim())
}

function logoSrc(blk: StoreBlock): string {
  return safeImg(blk.data.logo_url) || safeImg(props.logoUrl)
}

// Logo có hiện không (có ảnh và không bị tắt)
function showLogo(blk: StoreBlock): boolean {
  return !!logoSrc(blk) && blk.data.show_logo !== false
}

// Tên chữ luôn hiện nếu không có logo, để đầu trang không bị trống
function showBrand(blk: StoreBlock): boolean {
  return blk.data.show_brand !== false || !showLogo(blk)
}

function toggleFlag(blk: StoreBlock, key: string, v: boolean) {
  emit('edit', blk.id, [key], v)
}

function logoH(blk: StoreBlock): number {
  const n = Number(blk.data.logo_height) || 40
  return Math.min(120, Math.max(20, n))
}

function brandSizeClass(size?: string): string {
  return size === 'sm' ? 'text-base' : size === 'lg' ? 'text-2xl' : 'text-xl'
}

function headerLayout(layout?: string) {
  switch (layout) {
    case 'split':
      return { wrap: '@3xl:grid @3xl:grid-cols-[auto_1fr_auto]', brand: '', nav: '@3xl:justify-center', cta: '' }
    case 'center':
      return { wrap: '@3xl:flex-col @3xl:justify-center', brand: '@3xl:justify-center', nav: '@3xl:justify-center', cta: '' }
    case 'center_split':
      return {
        wrap: '@3xl:grid @3xl:grid-cols-[1fr_auto_1fr]',
        brand: '@3xl:order-2 @3xl:justify-center',
        nav: '@3xl:order-1 @3xl:justify-start',
        cta: '@3xl:order-3 @3xl:justify-self-end'
      }
    default:
      return { wrap: '', brand: '@3xl:mr-auto', nav: '', cta: '' }
  }
}

const openMenus = ref<Record<string, boolean>>({})
function toggleMenu(id: string) {
  openMenus.value[id] = !openMenus.value[id]
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

function initial(name?: string) {
  const n = String(name || '').trim()
  return n ? n.charAt(0).toUpperCase() : '★'
}

// Nút gọi / Zalo nổi ở góc (lấy từ section Liên hệ đầu tiên đang hiện)
const floatContact = computed(() => {
  const c = props.blocks.find((b) => b.enabled && b.type === 'contact' && (b.data.phone || b.data.zalo_link))
  return c ? { phone: String(c.data.phone || '').trim(), zalo: String(c.data.zalo_link || '').trim() } : null
})

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
  const rect = (ev.currentTarget as HTMLElement).getBoundingClientRect()
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

function onCanvasClickCapture(ev: MouseEvent) {
  if (!props.editable) return
  const a = (ev.target as HTMLElement).closest('a')
  if (a) ev.preventDefault()
}

function onBackgroundClick(ev: MouseEvent) {
  if (props.editable && ev.target === ev.currentTarget) emit('select', null)
}

const inner = 'mx-auto w-full max-w-6xl px-5 @3xl:px-8'
</script>

<template>
  <main
    class="pp-store @container w-full bg-white text-slate-800 antialiased"
    :class="editable ? 'min-h-full pb-24 pt-4' : ''"
    @click.capture="onCanvasClickCapture"
    @click="onBackgroundClick"
    @dragover="onCanvasDragOver"
    @drop.prevent="onCanvasDrop"
    @dragleave="onCanvasDragLeave"
  >
    <component :is="'style'" v-if="themeStyle">{{ themeStyle }}</component>
    <!-- Trang trống trong trình chỉnh sửa -->
    <div
      v-if="editable && !blocks.length"
      class="m-6 flex flex-col items-center justify-center gap-3 rounded-2xl border-2 border-dashed py-24 text-center text-sm"
      :class="dragActive && dropIndex === 0 ? 'border-indigo-500 bg-indigo-50 text-indigo-700' : 'border-slate-300 text-slate-500'"
    >
      <p class="font-semibold">Trang đang trống</p>
      <p class="text-xs">Kéo một section từ thư viện bên trái thả vào đây, hoặc</p>
      <button type="button" class="rounded-xl bg-indigo-600 px-4 py-2 text-xs font-bold text-white" @click.stop="emit('add-at', 0)">+ Thêm section</button>
    </div>

    <template v-for="{ blk, index } in visibleBlocks()" :key="blk.id">
      <div
        :data-block-id="blk.id"
        :data-pp-sec="blk.id"
        :style="wrapStyle(blk)"
        :class="editable ? ['relative transition-opacity', !blk.enabled ? 'opacity-40' : ''] : ''"
        @click="onSectionClick(blk)"
        @mouseenter="editable && (hoverId = blk.id)"
        @mouseleave="editable && hoverId === blk.id && (hoverId = null)"
        @dragover="onSectionDragOver(index, $event)"
      >
        <!-- CSS tuỳ chỉnh của section (ghi dạng chữ thuần vào thẻ style, không chạy được mã) -->
        <component :is="'style'" v-if="sectionCss(blk)">{{ sectionCss(blk) }}</component>
        <!-- ===== Khung chọn & thanh công cụ (chỉ trong trình chỉnh sửa) ===== -->
        <template v-if="editable">
          <div v-if="dragActive && dropIndex === index" class="pointer-events-none absolute inset-x-0 -top-[3px] z-40 h-[6px] rounded-full bg-indigo-500 shadow-[0_0_0_3px_rgba(99,102,241,0.25)]" />
          <div v-if="dragActive && dropIndex === index + 1" class="pointer-events-none absolute inset-x-0 -bottom-[3px] z-40 h-[6px] rounded-full bg-indigo-500 shadow-[0_0_0_3px_rgba(99,102,241,0.25)]" />
          <div
            class="pointer-events-none absolute inset-0 z-30 transition-shadow"
            :class="selectedId === blk.id ? 'shadow-[inset_0_0_0_2px_rgb(79,70,229)]' : hoverId === blk.id && !dragActive ? 'shadow-[inset_0_0_0_1px_rgb(129,140,248)]' : ''"
          />
          <div
            v-if="(selectedId === blk.id || hoverId === blk.id) && !dragActive"
            class="absolute left-3 top-0 z-40 flex -translate-y-1/2 items-center gap-0.5 rounded-lg bg-indigo-600 px-1 py-0.5 text-white shadow-lg"
            @click.stop
          >
            <span class="cursor-grab rounded p-1 hover:bg-white/20 active:cursor-grabbing" draggable="true" title="Kéo để đổi vị trí" @dragstart="emit('section-dragstart', index, $event)" @dragend="emit('section-dragend')">
              <GripVertical class="h-3.5 w-3.5" />
            </span>
            <span class="px-1 text-[11px] font-bold">{{ blockLabels[blk.type] || blk.type }}</span>
            <span v-if="!blk.enabled" class="rounded bg-white/20 px-1 text-[10px] font-semibold">Đang ẩn</span>
            <button type="button" class="rounded p-1 hover:bg-white/20 disabled:opacity-30" :disabled="index === 0" title="Lên" @click="emit('action', 'up', index)"><ArrowUp class="h-3.5 w-3.5" /></button>
            <button type="button" class="rounded p-1 hover:bg-white/20 disabled:opacity-30" :disabled="index === blocks.length - 1" title="Xuống" @click="emit('action', 'down', index)"><ArrowDown class="h-3.5 w-3.5" /></button>
            <button type="button" class="rounded p-1 hover:bg-white/20" title="Nhân đôi" @click="emit('action', 'duplicate', index)"><Copy class="h-3.5 w-3.5" /></button>
            <button type="button" class="rounded p-1 hover:bg-white/20" :title="blk.enabled ? 'Ẩn section' : 'Hiện section'" @click="emit('action', 'hide', index)"><EyeOff class="h-3.5 w-3.5" /></button>
            <button type="button" class="rounded p-1 hover:bg-rose-500" title="Xoá" @click="emit('action', 'delete', index)"><Trash2 class="h-3.5 w-3.5" /></button>
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

        <!-- ===================== ĐẦU TRANG ===================== -->
        <header
          v-if="blk.type === 'header'"
          :class="blk.data.border === false ? '' : 'border-b border-slate-200/70'"
          :style="blk.data.text_color ? { color: blk.data.text_color } : undefined"
        >
          <div :class="inner" class="py-3.5">
            <div class="flex flex-wrap items-center gap-x-8 gap-y-3" :class="headerLayout(blk.data.layout).wrap">
              <!-- Logo + tên -->
              <div class="group/brand relative flex min-w-0 flex-1 items-center gap-2.5 @3xl:flex-none" :class="headerLayout(blk.data.layout).brand">
                <span v-if="showLogo(blk)" class="relative inline-flex shrink-0">
                  <img :src="logoSrc(blk)" :alt="blk.data.brand_name || siteTitle" class="w-auto max-w-[180px] object-contain" :style="{ height: logoH(blk) + 'px' }" />
                  <!-- Trình chỉnh sửa: ẩn / hiện tên chữ ngay trên trang (chỉ khi có logo).
                       Nút nổi (absolute) nên không chiếm chỗ — logo nằm đúng vị trí như trang thật. -->
                  <template v-if="editable && showLogo(blk)">
                    <button
                      v-if="blk.data.show_brand !== false"
                      type="button"
                      class="absolute -right-3 -top-2 z-30 rounded-full bg-slate-900/80 p-1 text-white opacity-0 shadow transition hover:bg-rose-600 group-hover/brand:opacity-100"
                      title="Ẩn tên thương hiệu (chỉ hiện logo)"
                      @click.stop="toggleFlag(blk, 'show_brand', false)"
                    >
                      <EyeOff class="h-3 w-3" />
                    </button>
                    <button
                      v-else
                      type="button"
                      class="absolute left-full top-1/2 z-30 ml-2 inline-flex -translate-y-1/2 items-center gap-1 whitespace-nowrap rounded-full border border-dashed border-indigo-400 bg-white/95 px-2 py-0.5 text-[11px] font-bold text-indigo-600 opacity-0 shadow-sm transition hover:bg-indigo-50 group-hover/brand:opacity-100"
                      title="Hiện lại tên thương hiệu cạnh logo"
                      @click.stop="toggleFlag(blk, 'show_brand', true)"
                    >
                      <Eye class="h-3 w-3" /> Hiện tên
                    </button>
                  </template>
                </span>
                <EditableText :fx="fx(blk, ['brand_name'])"
                  v-if="showBrand(blk)"
                  :model-value="blk.data.brand_name || (editable ? '' : siteTitle)"
                  :editable="editable"
                  :placeholder="siteTitle || 'Tên thương hiệu'"
                  class="pp-hfont truncate font-extrabold tracking-tight"
                  :class="[brandSizeClass(blk.data.brand_size), blk.data.text_color ? '' : 'text-slate-900']"
                  @update:model-value="set(blk, ['brand_name'], $event)"
                />
              </div>

              <!-- Menu máy tính -->
              <nav
                v-if="navItems(blk).length"
                class="hidden flex-wrap items-center gap-x-7 gap-y-1 text-[15px] font-medium @3xl:flex"
                :class="[headerLayout(blk.data.layout).nav, blk.data.text_color ? '' : 'text-slate-600']"
              >
                <template v-for="item in navItems(blk)" :key="item.idx">
                  <a :href="safeHref(item.link)" class="transition-colors hover:opacity-70">
                    <EditableText :fx="fx(blk, ['nav_items', item.idx, 'label'])" :model-value="item.label" :editable="editable" placeholder="Mục menu" @update:model-value="set(blk, ['nav_items', item.idx, 'label'], $event)" />
                  </a>
                </template>
              </nav>

              <!-- Nút -->
              <a
                v-if="blk.data.show_cta !== false && blk.data.right_mode !== 'account'"
                :href="safeHref(blk.data.cta_link, '#products')"
                class="hidden rounded-full px-5 py-2.5 text-sm font-bold text-white shadow-sm transition hover:brightness-110 @xl:inline-block"
                :class="headerLayout(blk.data.layout).cta"
                :style="{ backgroundColor: accent }"
              >
                <EditableText :fx="fx(blk, ['cta_label'])" :model-value="blk.data.cta_label || (editable ? '' : 'Đặt mua ngay')" :editable="editable" placeholder="Chữ trên nút" @update:model-value="set(blk, ['cta_label'], $event)" />
              </a>

              <!-- Tìm kiếm, giỏ hàng, menu điện thoại -->
              <div class="flex items-center gap-0.5" :class="[blk.data.text_color ? '' : 'text-slate-700', headerLayout(blk.data.layout).cta ? '@3xl:order-4' : '']">
                <button v-if="blk.data.show_search" type="button" class="rounded-full p-2 hover:bg-black/5" aria-label="Tìm kiếm" @click.stop="toggleSearch(blk.id)">
                  <Search class="h-5 w-5" />
                </button>
                <!-- Điện thoại: biểu tượng tài khoản -->
                <template v-if="blk.data.right_mode === 'account'">
                  <router-link v-if="customerName" :to="accountPath" class="rounded-full p-1.5 @xl:hidden" aria-label="Tài khoản">
                    <span class="flex h-7 w-7 items-center justify-center rounded-full text-xs font-bold text-white" :style="{ backgroundColor: accent }">{{ initial(customerName) }}</span>
                  </router-link>
                  <button v-else type="button" class="rounded-full p-2 hover:bg-black/5 @xl:hidden" aria-label="Đăng nhập" @click.stop="openAuth('login')">
                    <User class="h-5 w-5" />
                  </button>
                </template>
                <button v-if="blk.data.show_cart || blk.data.right_mode === 'account'" type="button" class="relative rounded-full p-2 hover:bg-black/5" aria-label="Giỏ hàng" @click.stop="openCart">
                  <ShoppingBag class="h-5 w-5" />
                  <span
                    v-if="cartCount > 0"
                    class="absolute -right-0.5 -top-0.5 flex h-[18px] min-w-[18px] items-center justify-center rounded-full px-1 text-[10px] font-bold text-white"
                    :style="{ backgroundColor: accent }"
                  >
                    {{ cartCount > 99 ? '99+' : cartCount }}
                  </span>
                </button>
                <!-- Tài khoản khách: Đăng nhập / Đăng ký, hoặc tên + menu khi đã đăng nhập -->
                <div v-if="blk.data.right_mode === 'account'" class="ml-1.5 hidden items-center gap-1.5 @xl:flex">
                  <div v-if="customerName" class="relative">
                    <button
                      type="button"
                      class="flex items-center gap-2 rounded-full border border-slate-200/80 py-1 pl-1 pr-3 text-sm font-semibold transition hover:bg-black/5"
                      @click.stop="accountOpen = !accountOpen"
                    >
                      <span class="flex h-7 w-7 items-center justify-center rounded-full text-xs font-bold text-white" :style="{ backgroundColor: accent }">{{ initial(customerName) }}</span>
                      <span class="max-w-[120px] truncate">{{ firstName }}</span>
                      <ChevronDown class="h-3.5 w-3.5 opacity-60" />
                    </button>
                    <template v-if="accountOpen">
                      <div class="fixed inset-0 z-30" @click.stop="accountOpen = false" />
                      <div class="absolute right-0 top-full z-40 mt-2 w-56 overflow-hidden rounded-2xl border border-slate-200 bg-white py-1.5 text-sm text-slate-700 shadow-xl">
                        <div class="border-b border-slate-100 px-4 pb-2 pt-1">
                          <div class="truncate font-bold text-slate-900">{{ customerName }}</div>
                          <div class="truncate text-xs text-slate-500">{{ customerState.customer?.phone }}</div>
                        </div>
                        <router-link :to="accountPath" class="flex items-center gap-2.5 px-4 py-2.5 hover:bg-slate-50" @click="accountOpen = false"><Package class="h-4 w-4 text-slate-400" /> Đơn hàng của tôi</router-link>
                        <router-link :to="accountPath" class="flex items-center gap-2.5 px-4 py-2.5 hover:bg-slate-50" @click="accountOpen = false"><User class="h-4 w-4 text-slate-400" /> Thông tin tài khoản</router-link>
                        <button type="button" class="flex w-full items-center gap-2.5 border-t border-slate-100 px-4 py-2.5 text-left text-rose-600 hover:bg-rose-50" @click="logoutCustomer"><LogOut class="h-4 w-4" /> Đăng xuất</button>
                      </div>
                    </template>
                  </div>
                  <template v-else>
                    <button type="button" class="rounded-full px-3.5 py-2 text-sm font-semibold transition hover:bg-black/5" @click.stop="openAuth('login')">Đăng nhập</button>
                    <button type="button" class="rounded-full px-5 py-2.5 text-sm font-bold text-white shadow-sm transition hover:brightness-110" :style="{ backgroundColor: accent }" @click.stop="openAuth('register')">Đăng ký</button>
                  </template>
                </div>
                <button
                  v-if="navItems(blk).length"
                  type="button"
                  class="rounded-lg p-2 @3xl:hidden"
                  aria-label="Mở menu"
                  @click.stop="toggleMenu(blk.id)"
                >
                  <component :is="openMenus[blk.id] ? X : Menu" class="h-6 w-6" />
                </button>
              </div>
            </div>

            <!-- Ô tìm kiếm sản phẩm -->
            <div v-if="searchOpen === blk.id" class="relative mt-3" @click.stop>
              <div class="flex items-center gap-2 rounded-full border border-slate-300 bg-white px-4 py-2.5 text-slate-800">
                <Search class="h-4 w-4 text-slate-400" />
                <input ref="searchInput" v-model="searchText" type="search" placeholder="Tìm sản phẩm..." class="min-w-0 flex-1 bg-transparent text-sm outline-none" @keydown.esc="searchOpen = null" />
                <button type="button" class="text-slate-400 hover:text-slate-700" aria-label="Đóng tìm kiếm" @click="searchOpen = null"><X class="h-4 w-4" /></button>
              </div>
              <div v-if="searchText.trim()" class="absolute inset-x-0 top-full z-30 mt-2 overflow-hidden rounded-2xl border border-slate-200 bg-white text-slate-800 shadow-xl">
                <p v-if="!searchResults.length" class="px-4 py-3 text-sm text-slate-500">Không tìm thấy sản phẩm phù hợp</p>
                <a
                  v-for="p in searchResults"
                  :key="p.id"
                  :href="productLink(p.id) || '#products'"
                  class="flex items-center gap-3 px-4 py-2.5 hover:bg-slate-50"
                >
                  <span class="h-11 w-11 shrink-0 overflow-hidden rounded-lg bg-slate-100">
                    <img v-if="safeImg(p.image_url)" :src="safeImg(p.image_url)" :alt="p.name" class="h-full w-full object-cover" />
                  </span>
                  <span class="min-w-0 flex-1 truncate text-sm font-semibold">{{ p.name }}</span>
                  <span class="text-sm font-bold" :style="{ color: accent }">{{ formatVnd(p.price) }}</span>
                </a>
              </div>
            </div>

            <!-- Menu điện thoại (thả xuống) -->
            <div v-if="openMenus[blk.id] && navItems(blk).length" class="mt-3 space-y-1 border-t border-slate-100 pt-3 @3xl:hidden">
              <a
                v-for="item in navItems(blk)"
                :key="'m' + item.idx"
                :href="safeHref(item.link)"
                class="block rounded-lg px-3 py-2.5 text-base font-semibold hover:bg-slate-50"
                @click="openMenus[blk.id] = false"
              >
                {{ item.label }}
              </a>
              <a
                v-if="blk.data.show_cta !== false && blk.data.right_mode !== 'account'"
                :href="safeHref(blk.data.cta_link, '#products')"
                class="mt-2 block rounded-xl px-4 py-3 text-center text-sm font-bold text-white"
                :style="{ backgroundColor: accent }"
                @click="openMenus[blk.id] = false"
              >
                {{ blk.data.cta_label || 'Đặt mua ngay' }}
              </a>
              <!-- Tài khoản trong menu điện thoại -->
              <div v-if="blk.data.right_mode === 'account'" class="mt-2 border-t border-slate-100 pt-3">
                <template v-if="customerName">
                  <router-link :to="accountPath" class="flex items-center gap-2.5 rounded-lg px-3 py-2.5 text-base font-semibold hover:bg-slate-50" @click="openMenus[blk.id] = false">
                    <Package class="h-4 w-4 opacity-60" /> Đơn hàng của tôi
                  </router-link>
                  <button type="button" class="flex w-full items-center gap-2.5 rounded-lg px-3 py-2.5 text-left text-base font-semibold text-rose-600 hover:bg-rose-50" @click="logoutCustomer(); openMenus[blk.id] = false">
                    <LogOut class="h-4 w-4" /> Đăng xuất
                  </button>
                </template>
                <div v-else class="grid grid-cols-2 gap-2">
                  <button type="button" class="rounded-xl border border-slate-300 py-3 text-sm font-bold" @click="openAuth('login'); openMenus[blk.id] = false">Đăng nhập</button>
                  <button type="button" class="rounded-xl py-3 text-sm font-bold text-white" :style="{ backgroundColor: accent }" @click="openAuth('register'); openMenus[blk.id] = false">Đăng ký</button>
                </div>
              </div>
            </div>
          </div>
        </header>

        <!-- ===================== BANNER CHÍNH ===================== -->
        <section v-else-if="blk.type === 'hero'" class="overflow-hidden" :style="blk.data.style?.bg ? undefined : { background: `linear-gradient(180deg, ${tint(7)} 0%, #ffffff 100%)` }">
          <div :class="inner" class="grid grid-cols-1 items-center gap-10 py-14 @3xl:grid-cols-2 @3xl:gap-14 @3xl:py-24" :style="safeImg(blk.data.image_url) || editable ? undefined : { textAlign: 'center' }">
            <div class="space-y-5" :class="safeImg(blk.data.image_url) || editable ? '' : 'mx-auto max-w-3xl @3xl:col-span-2'">
              <span
                v-if="blk.data.badge || editable"
                class="inline-flex items-center rounded-full px-3.5 py-1 text-xs font-bold tracking-wide"
                :style="{ backgroundColor: tint(15), color: accent }"
              >
                <EditableText :fx="fx(blk, ['badge'])" :model-value="blk.data.badge" :editable="editable" placeholder="Nhãn nhỏ" @update:model-value="set(blk, ['badge'], $event)" />
              </span>
              <EditableText :fx="fx(blk, ['title'])"
                tag="h1"
                :model-value="blk.data.title"
                :editable="editable"
                placeholder="Tiêu đề chính"
                class="block text-[2rem] font-extrabold leading-[1.15] tracking-tight text-slate-900 @3xl:text-5xl"
                @update:model-value="set(blk, ['title'], $event)"
              />
              <EditableText :fx="fx(blk, ['subtitle'])"
                tag="p"
                multiline
                :model-value="blk.data.subtitle"
                :editable="editable"
                placeholder="Mô tả ngắn"
                class="block whitespace-pre-line text-base leading-relaxed text-slate-600 @3xl:text-lg"
                @update:model-value="set(blk, ['subtitle'], $event)"
              />
              <div class="pt-2">
                <a
                  :href="safeHref(blk.data.cta_link, '#products')"
                  class="inline-flex items-center justify-center rounded-full px-7 py-3.5 text-base font-bold text-white shadow-lg transition hover:-translate-y-0.5 hover:brightness-110"
                  :style="{ backgroundColor: accent, boxShadow: `0 10px 25px -8px ${accent}` }"
                >
                  <EditableText :fx="fx(blk, ['cta_label'])" :model-value="blk.data.cta_label || (editable ? '' : 'Khám phá ngay')" :editable="editable" placeholder="Chữ trên nút" @update:model-value="set(blk, ['cta_label'], $event)" />
                </a>
              </div>
            </div>
            <div v-if="safeImg(blk.data.image_url)" class="overflow-hidden rounded-3xl shadow-2xl shadow-slate-900/10 ring-1 ring-slate-900/5">
              <img :src="safeImg(blk.data.image_url)" :alt="blk.data.title" class="aspect-[4/3] h-full w-full object-cover" />
            </div>
            <div v-else-if="editable" class="flex aspect-[4/3] items-center justify-center rounded-3xl border-2 border-dashed border-slate-200 bg-white/60 text-xs text-slate-400">
              Chọn ảnh banner ở bảng bên phải
            </div>
          </div>
        </section>

        <!-- ===================== SẢN PHẨM ===================== -->
        <section v-else-if="blk.type === 'products'" id="products">
          <div :class="inner" class="space-y-10 py-16 @3xl:py-20">
            <div class="mx-auto max-w-2xl text-center">
              <EditableText :fx="fx(blk, ['heading'])"
                tag="h2"
                :model-value="blk.data.heading || (editable ? '' : 'Sản phẩm nổi bật')"
                :editable="editable"
                placeholder="Tiêu đề mục"
                class="block text-2xl font-extrabold tracking-tight text-slate-900 @3xl:text-4xl"
                @update:model-value="set(blk, ['heading'], $event)"
              />
              <div class="mx-auto mt-4 h-1 w-14 rounded-full" :style="{ backgroundColor: accent }" />
            </div>

            <!-- Sản phẩm từ danh sách của shop -->
            <template v-if="blk.data.source === 'catalog'">
              <p v-if="!products?.length" class="text-center text-sm text-slate-500">
                {{ editable ? 'Chưa có sản phẩm. Thêm sản phẩm ở trang "Quản lý sản phẩm".' : 'Sản phẩm sắp ra mắt.' }}
              </p>
              <div class="grid grid-cols-2 gap-3 @3xl:grid-cols-3 @3xl:gap-6 @5xl:grid-cols-4">
                <article v-for="prod in products || []" :key="prod.id" class="group flex flex-col overflow-hidden rounded-2xl border border-slate-200/80 bg-white transition hover:-translate-y-1 hover:shadow-xl hover:shadow-slate-900/5">
                  <component :is="productLink(prod.id) ? 'a' : 'div'" :href="productLink(prod.id)" class="relative block aspect-square overflow-hidden bg-slate-100">
                    <img v-if="safeImg(prod.image_url)" :src="safeImg(prod.image_url)" :alt="prod.name" loading="lazy" class="h-full w-full object-cover transition duration-500 group-hover:scale-105" />
                    <span v-if="discount(prod)" class="absolute left-2.5 top-2.5 rounded-full bg-rose-500 px-2 py-0.5 text-[11px] font-bold text-white">Sale -{{ discount(prod) }}%</span>
                    <span v-if="!prod.in_stock" class="absolute inset-x-0 bottom-0 bg-slate-900/75 py-1.5 text-center text-xs font-bold text-white">Hết hàng</span>
                  </component>
                  <div class="flex flex-1 flex-col gap-1.5 p-3.5 @3xl:p-4">
                    <component :is="productLink(prod.id) ? 'a' : 'h3'" :href="productLink(prod.id)" class="line-clamp-2 text-sm font-bold leading-snug text-slate-900 hover:underline @3xl:text-[15px]">{{ prod.name }}</component>
                    <p v-if="prod.description" class="line-clamp-2 text-xs leading-relaxed text-slate-500">{{ prod.description }}</p>
                    <div class="mt-auto flex flex-wrap items-baseline gap-x-2 pt-1.5">
                      <span class="text-base font-extrabold @3xl:text-lg" :style="{ color: accent }">{{ formatVnd(prod.price) }}</span>
                      <span v-if="prod.compare_price > prod.price" class="text-xs text-slate-400 line-through">{{ formatVnd(prod.compare_price) }}</span>
                    </div>
                    <template v-if="blk.data.show_buy !== false && prod.in_stock">
                      <a
                        v-if="blk.data.buy_mode === 'link'"
                        :href="safeHref(blk.data.buy_link, '#contact')"
                        class="mt-2 block rounded-xl py-2.5 text-center text-sm font-bold text-white transition hover:brightness-110"
                        :style="{ backgroundColor: accent }"
                      >
                        {{ blk.data.buy_label || 'Đặt mua' }}
                      </a>
                      <button
                        v-else
                        type="button"
                        class="mt-2 flex w-full items-center justify-center gap-1.5 rounded-xl py-2.5 text-sm font-bold text-white transition hover:brightness-110"
                        :style="{ backgroundColor: accent }"
                        @click.stop="addToCart(prod)"
                      >
                        <ShoppingBag class="h-4 w-4" /> {{ blk.data.buy_label || 'Thêm vào giỏ' }}
                      </button>
                    </template>
                  </div>
                </article>
              </div>
            </template>

            <!-- Sản phẩm nhập tay -->
            <div v-else class="grid grid-cols-2 gap-3 @3xl:grid-cols-3 @3xl:gap-6 @5xl:grid-cols-4">
              <article v-for="(prod, pIdx) in blk.data.items || []" :key="prod.id || pIdx" class="group flex flex-col overflow-hidden rounded-2xl border border-slate-200/80 bg-white transition hover:-translate-y-1 hover:shadow-xl hover:shadow-slate-900/5">
                <div class="aspect-square overflow-hidden bg-slate-100">
                  <img v-if="safeImg(prod.image_url)" :src="safeImg(prod.image_url)" :alt="prod.name" loading="lazy" class="h-full w-full object-cover transition duration-500 group-hover:scale-105" />
                  <div v-else-if="editable" class="flex h-full items-center justify-center text-xs text-slate-400">Chưa có ảnh</div>
                </div>
                <div class="flex flex-1 flex-col gap-1.5 p-3.5 @3xl:p-4">
                  <EditableText :fx="fx(blk, ['items', pIdx, 'name'])" tag="h3" :model-value="prod.name" :editable="editable" placeholder="Tên sản phẩm" class="line-clamp-2 text-sm font-bold leading-snug text-slate-900 @3xl:text-[15px]" @update:model-value="set(blk, ['items', pIdx, 'name'], $event)" />
                  <EditableText :fx="fx(blk, ['items', pIdx, 'desc'])" tag="p" multiline :model-value="prod.desc" :editable="editable" placeholder="Mô tả" class="text-xs leading-relaxed text-slate-500" @update:model-value="set(blk, ['items', pIdx, 'desc'], $event)" />
                  <EditableText :fx="fx(blk, ['items', pIdx, 'price'])" tag="div" :model-value="prod.price" :editable="editable" placeholder="Giá" class="mt-auto pt-1.5 text-base font-extrabold @3xl:text-lg" :style="{ color: accent }" @update:model-value="set(blk, ['items', pIdx, 'price'], $event)" />
                  <a
                    v-if="blk.data.show_buy !== false"
                    :href="safeHref(blk.data.buy_link, '#contact')"
                    class="mt-2 block rounded-xl py-2.5 text-center text-sm font-bold text-white transition hover:brightness-110"
                    :style="{ backgroundColor: accent }"
                  >
                    {{ blk.data.buy_label || 'Đặt mua' }}
                  </a>
                </div>
              </article>
            </div>
          </div>
        </section>

        <!-- ===================== CAM KẾT ===================== -->
        <section v-else-if="blk.type === 'features'">
          <div :class="inner" class="space-y-10 py-14 @3xl:py-16">
            <EditableText :fx="fx(blk, ['heading'])"
              tag="h2"
              :model-value="blk.data.heading || (editable ? '' : 'Cam kết dịch vụ')"
              :editable="editable"
              placeholder="Tiêu đề mục"
              class="block text-center text-2xl font-extrabold tracking-tight text-slate-900 @3xl:text-3xl"
              @update:model-value="set(blk, ['heading'], $event)"
            />
            <div class="grid grid-cols-1 gap-4 @xl:grid-cols-2 @5xl:grid-cols-3">
              <div v-for="(feat, fIdx) in blk.data.items || []" :key="fIdx" class="flex items-start gap-4 rounded-2xl p-5" :style="{ backgroundColor: tint(6) }">
                <span class="flex h-10 w-10 shrink-0 items-center justify-center rounded-full text-lg font-bold text-white" :style="{ backgroundColor: accent }">✓</span>
                <EditableText :fx="fx(blk, ['items', fIdx])" tag="p" :model-value="feat" :editable="editable" placeholder="Cam kết" class="pt-2 text-[15px] font-semibold leading-snug text-slate-800" @update:model-value="set(blk, ['items', fIdx], $event)" />
              </div>
            </div>
          </div>
        </section>

        <!-- ===================== ĐÁNH GIÁ ===================== -->
        <section v-else-if="blk.type === 'testimonials'" :style="blk.data.style?.bg ? undefined : { backgroundColor: '#f8fafc' }">
          <div :class="inner" class="space-y-10 py-16 @3xl:py-20">
            <EditableText :fx="fx(blk, ['heading'])"
              tag="h2"
              :model-value="blk.data.heading || (editable ? '' : 'Khách hàng nói gì')"
              :editable="editable"
              placeholder="Tiêu đề mục"
              class="block text-center text-2xl font-extrabold tracking-tight text-slate-900 @3xl:text-3xl"
              @update:model-value="set(blk, ['heading'], $event)"
            />
            <div data-carousel class="relative">
            <div v-if="blk.data.layout === 'carousel'" class="-mt-4 mb-4 flex justify-end gap-2">
              <button type="button" class="flex h-10 w-10 items-center justify-center rounded-full border border-slate-300 bg-white text-slate-700 hover:bg-slate-50" aria-label="Trước" @click.stop="scrollCarousel($event, -1)">
                <ChevronLeft class="h-5 w-5" />
              </button>
              <button type="button" class="flex h-10 w-10 items-center justify-center rounded-full border border-slate-300 bg-white text-slate-700 hover:bg-slate-50" aria-label="Sau" @click.stop="scrollCarousel($event, 1)">
                <ChevronRight class="h-5 w-5" />
              </button>
            </div>
            <div
              data-track
              :class="
                blk.data.layout === 'carousel'
                  ? 'flex snap-x snap-mandatory gap-5 overflow-x-auto pb-2 [scrollbar-width:none] [&::-webkit-scrollbar]:hidden'
                  : 'grid grid-cols-1 gap-5 @3xl:grid-cols-2 @5xl:grid-cols-3'
              "
            >
              <figure
                v-for="(t, tIdx) in blk.data.items || []"
                :key="tIdx"
                class="flex flex-col rounded-2xl bg-white p-6 shadow-sm ring-1 ring-slate-900/5"
                :class="blk.data.layout === 'carousel' ? 'w-[85%] shrink-0 snap-start @3xl:w-[calc(50%-10px)] @5xl:w-[calc(33.333%-14px)]' : ''"
              >
                <div class="text-base tracking-widest text-amber-400">★★★★★</div>
                <blockquote class="mt-3 flex-1 text-[15px] leading-relaxed text-slate-700">
                  “<EditableText :fx="fx(blk, ['items', tIdx, 'quote'])" :model-value="t.quote" :editable="editable" multiline placeholder="Nội dung đánh giá" @update:model-value="set(blk, ['items', tIdx, 'quote'], $event)" />”
                </blockquote>
                <figcaption class="mt-5 flex items-center gap-3">
                  <span class="flex h-10 w-10 items-center justify-center rounded-full text-sm font-bold text-white" :style="{ backgroundColor: accent }">{{ initial(t.name) }}</span>
                  <EditableText :fx="fx(blk, ['items', tIdx, 'name'])" :model-value="t.name" :editable="editable" placeholder="Tên khách" class="text-sm font-bold text-slate-900" @update:model-value="set(blk, ['items', tIdx, 'name'], $event)" />
                </figcaption>
              </figure>
            </div>
            </div>
          </div>
        </section>

        <!-- ===================== VĂN BẢN ===================== -->
        <section v-else-if="blk.type === 'richtext'">
          <div :class="inner" class="py-14">
            <div class="max-w-3xl space-y-4" :class="alignOf(blk.data.align) === 'center' ? 'mx-auto' : alignOf(blk.data.align) === 'right' ? 'ml-auto' : ''" :style="{ textAlign: alignOf(blk.data.align) }">
              <EditableText :fx="fx(blk, ['heading'])"
                v-if="blk.data.heading || editable"
                tag="h2"
                :model-value="blk.data.heading"
                :editable="editable"
                placeholder="Tiêu đề (không bắt buộc)"
                class="block text-2xl font-extrabold tracking-tight text-slate-900 @3xl:text-3xl"
                @update:model-value="set(blk, ['heading'], $event)"
              />
              <EditableText :fx="fx(blk, ['body'])"
                tag="p"
                multiline
                :model-value="blk.data.body"
                :editable="editable"
                placeholder="Nhập nội dung..."
                class="block whitespace-pre-line text-base leading-[1.8] text-slate-600 @3xl:text-[17px]"
                @update:model-value="set(blk, ['body'], $event)"
              />
            </div>
          </div>
        </section>

        <!-- ===================== ẢNH LỚN ===================== -->
        <section v-else-if="blk.type === 'image'">
          <div :class="inner" class="py-8">
            <component :is="blk.data.link ? 'a' : 'div'" :href="blk.data.link ? safeHref(blk.data.link) : undefined" class="block overflow-hidden rounded-3xl">
              <img v-if="safeImg(blk.data.image_url)" :src="safeImg(blk.data.image_url)" :alt="blk.data.alt || ''" loading="lazy" class="w-full object-cover" />
              <div v-else class="flex h-48 items-center justify-center bg-slate-100 text-sm text-slate-400">{{ editable ? 'Chọn ảnh ở bảng bên phải' : '' }}</div>
            </component>
          </div>
        </section>

        <!-- ===================== THƯ VIỆN ẢNH ===================== -->
        <section v-else-if="blk.type === 'gallery'">
          <div :class="inner" class="space-y-8 py-14">
            <EditableText :fx="fx(blk, ['heading'])"
              v-if="blk.data.heading || editable"
              tag="h2"
              :model-value="blk.data.heading"
              :editable="editable"
              placeholder="Tiêu đề mục"
              class="block text-center text-2xl font-extrabold tracking-tight text-slate-900 @3xl:text-3xl"
              @update:model-value="set(blk, ['heading'], $event)"
            />
            <div class="grid grid-cols-2 gap-3 @3xl:grid-cols-3 @3xl:gap-4">
              <figure v-for="(g, gIdx) in blk.data.items || []" :key="gIdx" class="group overflow-hidden rounded-2xl bg-slate-100">
                <div class="aspect-square overflow-hidden">
                  <img v-if="safeImg(g.image_url)" :src="safeImg(g.image_url)" :alt="g.caption || ''" loading="lazy" class="h-full w-full object-cover transition duration-500 group-hover:scale-105" />
                  <div v-else class="flex h-full items-center justify-center text-xs text-slate-400">Chưa chọn ảnh</div>
                </div>
                <EditableText :fx="fx(blk, ['items', gIdx, 'caption'])"
                  v-if="g.caption || editable"
                  tag="figcaption"
                  :model-value="g.caption"
                  :editable="editable"
                  placeholder="Chú thích"
                  class="block bg-white px-3 py-2.5 text-sm text-slate-600"
                  @update:model-value="set(blk, ['items', gIdx, 'caption'], $event)"
                />
              </figure>
            </div>
          </div>
        </section>

        <!-- ===================== CÂU HỎI THƯỜNG GẶP ===================== -->
        <section v-else-if="blk.type === 'faq'" id="faq">
          <div :class="inner" class="py-14 @3xl:py-16">
            <div class="mx-auto max-w-3xl space-y-8">
              <EditableText :fx="fx(blk, ['heading'])"
                v-if="blk.data.heading || editable"
                tag="h2"
                :model-value="blk.data.heading"
                :editable="editable"
                placeholder="Tiêu đề mục"
                class="block text-center text-2xl font-extrabold tracking-tight text-slate-900 @3xl:text-3xl"
                @update:model-value="set(blk, ['heading'], $event)"
              />
              <div class="space-y-3">
                <details v-for="(f, fqIdx) in blk.data.items || []" :key="fqIdx" class="group rounded-2xl border border-slate-200 bg-white open:shadow-sm" :open="editable || undefined">
                  <summary class="flex cursor-pointer list-none items-center justify-between gap-4 px-5 py-4 text-[15px] font-bold text-slate-900 [&::-webkit-details-marker]:hidden">
                    <EditableText :fx="fx(blk, ['items', fqIdx, 'q'])" :model-value="f.q" :editable="editable" placeholder="Câu hỏi" @update:model-value="set(blk, ['items', fqIdx, 'q'], $event)" />
                    <ChevronDown class="h-5 w-5 shrink-0 text-slate-400 transition group-open:rotate-180" />
                  </summary>
                  <EditableText :fx="fx(blk, ['items', fqIdx, 'a'])" tag="p" multiline :model-value="f.a" :editable="editable" placeholder="Câu trả lời" class="block whitespace-pre-line px-5 pb-5 text-[15px] leading-relaxed text-slate-600" @update:model-value="set(blk, ['items', fqIdx, 'a'], $event)" />
                </details>
              </div>
            </div>
          </div>
        </section>

        <!-- ===================== LIÊN HỆ ===================== -->
        <section v-else-if="blk.type === 'contact'" id="contact">
          <div :class="inner" class="space-y-8 py-14 @3xl:py-16">
            <EditableText :fx="fx(blk, ['heading'])"
              v-if="blk.data.heading || editable"
              tag="h2"
              :model-value="blk.data.heading"
              :editable="editable"
              placeholder="Tiêu đề mục"
              class="block text-center text-2xl font-extrabold tracking-tight text-slate-900 @3xl:text-3xl"
              @update:model-value="set(blk, ['heading'], $event)"
            />
            <div class="mx-auto grid max-w-4xl grid-cols-1 gap-3 @xl:grid-cols-2">
              <a v-if="blk.data.phone" :href="safeHref('tel:' + blk.data.phone)" class="flex items-center gap-4 rounded-2xl border border-slate-200 bg-white p-4 transition hover:shadow-md">
                <span class="flex h-11 w-11 shrink-0 items-center justify-center rounded-full" :style="{ backgroundColor: tint(12), color: accent }"><Phone class="h-5 w-5" /></span>
                <span><span class="block text-xs text-slate-500">Gọi điện</span><span class="block text-base font-bold text-slate-900">{{ blk.data.phone }}</span></span>
              </a>
              <a v-if="blk.data.zalo_link" :href="safeHref(blk.data.zalo_link)" target="_blank" rel="noopener" class="flex items-center gap-4 rounded-2xl border border-slate-200 bg-white p-4 transition hover:shadow-md">
                <span class="flex h-11 w-11 shrink-0 items-center justify-center rounded-full bg-sky-50 text-sky-600"><MessageCircle class="h-5 w-5" /></span>
                <span><span class="block text-xs text-slate-500">Nhắn tin</span><span class="block text-base font-bold text-slate-900">Chat Zalo</span></span>
              </a>
              <a v-if="blk.data.email" :href="safeHref('mailto:' + blk.data.email)" class="flex items-center gap-4 rounded-2xl border border-slate-200 bg-white p-4 transition hover:shadow-md">
                <span class="flex h-11 w-11 shrink-0 items-center justify-center rounded-full" :style="{ backgroundColor: tint(12), color: accent }"><Mail class="h-5 w-5" /></span>
                <span class="min-w-0"><span class="block text-xs text-slate-500">Email</span><span class="block truncate text-base font-bold text-slate-900">{{ blk.data.email }}</span></span>
              </a>
              <div v-if="blk.data.address" class="flex items-center gap-4 rounded-2xl border border-slate-200 bg-white p-4">
                <span class="flex h-11 w-11 shrink-0 items-center justify-center rounded-full" :style="{ backgroundColor: tint(12), color: accent }"><MapPin class="h-5 w-5" /></span>
                <span><span class="block text-xs text-slate-500">Địa chỉ</span><span class="block text-[15px] font-bold leading-snug text-slate-900">{{ blk.data.address }}</span></span>
              </div>
              <p v-if="editable && !blk.data.phone && !blk.data.email && !blk.data.address && !blk.data.zalo_link" class="text-center text-xs text-slate-400 @xl:col-span-2">
                Nhập số điện thoại, email, địa chỉ, Zalo ở bảng bên phải
              </p>
            </div>
          </div>
        </section>

        <!-- ===================== DẢI THÔNG BÁO ===================== -->
        <div
          v-else-if="blk.type === 'announcement'"
          class="px-4 py-2.5 text-center text-[13px] font-semibold tracking-wide"
          :style="{ backgroundColor: blk.data.style?.bg ? undefined : accent, color: blk.data.text_color || '#ffffff' }"
        >
          <component :is="blk.data.link && !editable ? 'a' : 'span'" :href="blk.data.link ? safeHref(blk.data.link) : undefined" class="hover:underline">
            <EditableText :fx="fx(blk, ['text'])" :model-value="blk.data.text" :editable="editable" placeholder="Nội dung thông báo" @update:model-value="set(blk, ['text'], $event)" />
          </component>
        </div>

        <!-- ===================== BANNER ẢNH TRƯỢT ===================== -->
        <BannerSlider v-else-if="blk.type === 'banner'" :blk="blk" :editable="editable" :accent="accent" @edit="(p, v) => set(blk, p, v)" />

        <!-- ===================== ẢNH + CHỮ ===================== -->
        <section v-else-if="blk.type === 'image_text'">
          <div :class="inner" class="grid grid-cols-1 items-center gap-10 py-16 @3xl:grid-cols-2 @3xl:gap-16 @3xl:py-20">
            <div v-if="safeImg(blk.data.image_url) || editable" class="overflow-hidden rounded-3xl bg-slate-100" :class="blk.data.image_side === 'right' ? '@3xl:order-2' : ''">
              <img v-if="safeImg(blk.data.image_url)" :src="safeImg(blk.data.image_url)" :alt="blk.data.heading || ''" loading="lazy" class="aspect-[4/5] h-full w-full object-cover @3xl:aspect-square" />
              <div v-else class="flex aspect-[4/3] items-center justify-center text-xs text-slate-400" :style="{ background: tint(10) }">{{ editable ? 'Chọn ảnh ở bảng bên phải' : '' }}</div>
            </div>
            <div class="space-y-5" :class="safeImg(blk.data.image_url) || editable ? '' : '@3xl:col-span-2 mx-auto max-w-3xl'">
              <EditableText :fx="fx(blk, ['eyebrow'])" v-if="blk.data.eyebrow || editable" tag="p" :model-value="blk.data.eyebrow" :editable="editable" placeholder="Dòng chữ nhỏ phía trên" class="block text-xs font-bold uppercase tracking-[0.18em]" :style="{ color: accent }" @update:model-value="set(blk, ['eyebrow'], $event)" />
              <EditableText :fx="fx(blk, ['heading'])" tag="h2" :model-value="blk.data.heading" :editable="editable" placeholder="Tiêu đề" class="block text-3xl font-extrabold leading-tight tracking-tight text-slate-900 @3xl:text-4xl" @update:model-value="set(blk, ['heading'], $event)" />
              <EditableText :fx="fx(blk, ['body'])" tag="p" multiline :model-value="blk.data.body" :editable="editable" placeholder="Nội dung" class="block whitespace-pre-line text-base leading-relaxed text-slate-600 @3xl:text-[17px]" @update:model-value="set(blk, ['body'], $event)" />
              <ul v-if="(blk.data.points || []).length" class="space-y-4 pt-1">
                <li v-for="(pt, pIdx) in blk.data.points" :key="pIdx" class="flex gap-3.5">
                  <span class="mt-0.5 flex h-7 w-7 shrink-0 items-center justify-center rounded-full text-sm font-bold text-white" :style="{ backgroundColor: accent }">✓</span>
                  <div>
                    <EditableText :fx="fx(blk, ['points', pIdx, 'title'])" tag="p" :model-value="pt.title" :editable="editable" placeholder="Điểm nổi bật" class="block font-bold text-slate-900" @update:model-value="set(blk, ['points', pIdx, 'title'], $event)" />
                    <EditableText :fx="fx(blk, ['points', pIdx, 'text'])" v-if="pt.text || editable" tag="p" :model-value="pt.text" :editable="editable" placeholder="Mô tả" class="block text-sm text-slate-600" @update:model-value="set(blk, ['points', pIdx, 'text'], $event)" />
                  </div>
                </li>
              </ul>
              <div v-if="blk.data.button_label || editable" class="pt-2">
                <a :href="safeHref(blk.data.button_link, '#products')" class="inline-flex rounded-full px-7 py-3.5 text-sm font-bold text-white shadow-lg transition hover:-translate-y-0.5 hover:brightness-110" :style="{ backgroundColor: accent }">
                  <EditableText :fx="fx(blk, ['button_label'])" :model-value="blk.data.button_label" :editable="editable" placeholder="Chữ trên nút" @update:model-value="set(blk, ['button_label'], $event)" />
                </a>
              </div>
            </div>
          </div>
        </section>

        <!-- ===================== Ô DANH MỤC ===================== -->
        <section v-else-if="blk.type === 'collections'" id="collections">
          <div :class="inner" class="space-y-8 py-14 @3xl:py-16">
            <EditableText :fx="fx(blk, ['heading'])" v-if="blk.data.heading || editable" tag="h2" :model-value="blk.data.heading" :editable="editable" placeholder="Tiêu đề mục" class="block text-center text-2xl font-extrabold tracking-tight text-slate-900 @3xl:text-3xl" @update:model-value="set(blk, ['heading'], $event)" />
            <div class="grid grid-cols-1 gap-4 @xl:grid-cols-2" :class="(blk.data.items || []).length >= 3 ? '@5xl:grid-cols-3' : ''">
              <a
                v-for="(col, cIdx) in blk.data.items || []"
                :key="cIdx"
                :href="safeHref(col.link, '#products')"
                class="group relative block aspect-[4/3] overflow-hidden rounded-3xl"
                :style="safeImg(col.image_url) ? undefined : { background: `linear-gradient(135deg, ${tint(25)}, ${tint(55)})` }"
              >
                <img v-if="safeImg(col.image_url)" :src="safeImg(col.image_url)" :alt="col.title" loading="lazy" class="absolute inset-0 h-full w-full object-cover transition duration-700 group-hover:scale-105" />
                <div class="absolute inset-0 bg-gradient-to-t from-black/60 via-black/10 to-transparent" />
                <div class="absolute inset-x-0 bottom-0 flex items-end justify-between gap-3 p-5 text-white">
                  <EditableText :fx="fx(blk, ['items', cIdx, 'title'])" tag="h3" :model-value="col.title" :editable="editable" placeholder="Tên danh mục" class="text-xl font-extrabold @3xl:text-2xl" @update:model-value="set(blk, ['items', cIdx, 'title'], $event)" />
                  <span class="flex h-10 w-10 shrink-0 items-center justify-center rounded-full bg-white/90 text-slate-900 transition group-hover:translate-x-1"><ChevronRight class="h-5 w-5" /></span>
                </div>
              </a>
            </div>
          </div>
        </section>

        <!-- ===================== DẢI BIỂU TƯỢNG ===================== -->
        <section v-else-if="blk.type === 'icon_strip'" :style="blk.data.style?.bg ? undefined : { backgroundColor: tint(6) }">
          <div class="grid grid-cols-1 gap-6 py-10 @xl:grid-cols-2" :class="[inner, (blk.data.items || []).length >= 4 ? '@5xl:grid-cols-4' : '@5xl:grid-cols-3']">
            <div v-for="(ic, iIdx) in blk.data.items || []" :key="iIdx" class="flex items-center gap-4 @5xl:flex-col @5xl:text-center">
              <span class="flex h-14 w-14 shrink-0 items-center justify-center rounded-full bg-white shadow-sm [&>svg]:h-6 [&>svg]:w-6" :style="{ color: accent }">
                <StoreIcon :name="ic.icon" />
              </span>
              <div>
                <EditableText :fx="fx(blk, ['items', iIdx, 'title'])" tag="p" :model-value="ic.title" :editable="editable" placeholder="Tiêu đề" class="block font-bold text-slate-900" @update:model-value="set(blk, ['items', iIdx, 'title'], $event)" />
                <EditableText :fx="fx(blk, ['items', iIdx, 'text'])" v-if="ic.text || editable" tag="p" :model-value="ic.text" :editable="editable" placeholder="Mô tả" class="block text-sm text-slate-600" @update:model-value="set(blk, ['items', iIdx, 'text'], $event)" />
              </div>
            </div>
          </div>
        </section>

        <!-- ===================== ĐĂNG KÝ EMAIL ===================== -->
        <section v-else-if="blk.type === 'newsletter'" :style="blk.data.style?.bg ? undefined : { backgroundColor: '#f8fafc' }">
          <div :class="inner" class="py-14 @3xl:py-16">
            <NewsletterForm :blk="blk" :editable="editable" :accent="accent" :site-slug="siteSlug" @edit="(p, v) => set(blk, p, v)" />
          </div>
        </section>

        <!-- ===================== CHÂN TRANG ===================== -->
        <footer v-else-if="blk.type === 'footer'" class="border-t border-slate-200" :style="blk.data.style?.bg ? undefined : { backgroundColor: '#0f172a', color: '#cbd5e1' }">
          <div :class="inner" class="grid grid-cols-1 gap-10 py-14 @3xl:grid-cols-[1.4fr_repeat(var(--cols),1fr)]" :style="{ '--cols': Math.max(1, (blk.data.columns || []).length) }">
            <div class="space-y-3">
              <EditableText :fx="fx(blk, ['company'])" tag="p" :model-value="blk.data.company || (editable ? '' : siteTitle)" :editable="editable" placeholder="Tên công ty" class="block text-lg font-extrabold text-white" @update:model-value="set(blk, ['company'], $event)" />
              <EditableText :fx="fx(blk, ['about_text'])" v-if="blk.data.about_text || editable" tag="p" multiline :model-value="blk.data.about_text" :editable="editable" placeholder="Giới thiệu ngắn" class="block text-sm leading-relaxed opacity-80" @update:model-value="set(blk, ['about_text'], $event)" />
              <ul class="space-y-1.5 pt-1 text-sm">
                <li v-if="blk.data.address" class="flex gap-2"><MapPin class="mt-0.5 h-4 w-4 shrink-0 opacity-70" /> {{ blk.data.address }}</li>
                <li v-if="blk.data.phone" class="flex gap-2"><Phone class="mt-0.5 h-4 w-4 shrink-0 opacity-70" /> <a :href="safeHref('tel:' + blk.data.phone)" class="hover:underline">{{ blk.data.phone }}</a></li>
                <li v-if="blk.data.email" class="flex gap-2"><Mail class="mt-0.5 h-4 w-4 shrink-0 opacity-70" /> <a :href="safeHref('mailto:' + blk.data.email)" class="hover:underline">{{ blk.data.email }}</a></li>
              </ul>
              <div v-if="socials(blk).length" class="flex flex-wrap gap-2 pt-2">
                <a
                  v-for="s in socials(blk)"
                  :key="s.key"
                  :href="s.href"
                  target="_blank"
                  rel="noopener"
                  :aria-label="s.label"
                  :title="s.label"
                  class="flex h-9 min-w-9 items-center justify-center rounded-full bg-white/10 px-2.5 text-xs font-bold text-white transition hover:bg-white/20"
                >
                  {{ s.short }}
                </a>
              </div>
            </div>
            <div v-for="(col, cIdx) in blk.data.columns || []" :key="cIdx" class="space-y-3">
              <EditableText :fx="fx(blk, ['columns', cIdx, 'title'])" tag="p" :model-value="col.title" :editable="editable" placeholder="Tiêu đề cột" class="block text-sm font-bold uppercase tracking-wider text-white" @update:model-value="set(blk, ['columns', cIdx, 'title'], $event)" />
              <ul class="space-y-2 text-sm">
                <li v-for="(lnk, lIdx) in col.links || []" :key="lIdx">
                  <a :href="safeHref(lnk.link)" class="opacity-80 transition hover:opacity-100 hover:underline">
                    <EditableText :fx="fx(blk, ['columns', cIdx, 'links', lIdx, 'label'])" :model-value="lnk.label" :editable="editable" placeholder="Liên kết" @update:model-value="set(blk, ['columns', cIdx, 'links', lIdx, 'label'], $event)" />
                  </a>
                </li>
              </ul>
            </div>
          </div>
          <div class="border-t border-white/10">
            <EditableText :fx="fx(blk, ['copyright'])" :class="inner" tag="p" :model-value="blk.data.copyright" :editable="editable" placeholder="Dòng bản quyền" class="block py-5 text-center text-xs opacity-60" @update:model-value="set(blk, ['copyright'], $event)" />
          </div>
        </footer>

        <!-- ===================== HTML TỰ VIẾT ===================== -->
        <section v-else-if="blk.type === 'html'">
          <div :class="inner" class="py-6">
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
          </div>
        </section>

        <!-- ===================== KÊU GỌI HÀNH ĐỘNG & CHÂN TRANG ===================== -->
        <footer v-else-if="blk.type === 'cta'" :id="blocks.some((b) => b.type === 'contact' && b.enabled) ? undefined : 'contact'">
          <div :class="inner" class="py-14 @3xl:py-16">
            <div class="relative overflow-hidden rounded-3xl px-6 py-12 text-center text-white @3xl:px-12 @3xl:py-16" :style="{ background: `linear-gradient(135deg, ${accent} 0%, color-mix(in srgb, ${accent} 70%, black) 100%)` }">
              <div class="pointer-events-none absolute -right-16 -top-16 h-56 w-56 rounded-full bg-white/10" />
              <div class="pointer-events-none absolute -bottom-20 -left-10 h-48 w-48 rounded-full bg-white/10" />
              <div class="relative mx-auto max-w-2xl space-y-4">
                <EditableText :fx="fx(blk, ['title'])" tag="h2" :model-value="blk.data.title" :editable="editable" placeholder="Tiêu đề" class="block text-2xl font-extrabold tracking-tight @3xl:text-4xl" @update:model-value="set(blk, ['title'], $event)" />
                <EditableText :fx="fx(blk, ['subtitle'])" tag="p" multiline :model-value="blk.data.subtitle" :editable="editable" placeholder="Mô tả" class="block text-base leading-relaxed text-white/85 @3xl:text-lg" @update:model-value="set(blk, ['subtitle'], $event)" />
                <div class="pt-3">
                  <a :href="safeHref(blk.data.button_link)" class="inline-flex items-center justify-center rounded-full bg-white px-7 py-3.5 text-base font-bold shadow-lg transition hover:-translate-y-0.5" :style="{ color: accent }">
                    <EditableText :fx="fx(blk, ['button_text'])" :model-value="blk.data.button_text || (editable ? '' : 'Liên hệ ngay')" :editable="editable" placeholder="Chữ trên nút" @update:model-value="set(blk, ['button_text'], $event)" />
                  </a>
                </div>
              </div>
            </div>
            <EditableText :fx="fx(blk, ['footer_note'])"
              v-if="blk.data.footer_note || editable"
              tag="p"
              :model-value="blk.data.footer_note"
              :editable="editable"
              placeholder="Ghi chú chân trang"
              class="mt-10 block border-t border-slate-100 pt-6 text-center text-sm text-slate-400"
              @update:model-value="set(blk, ['footer_note'], $event)"
            />
          </div>
        </footer>
      </div>
    </template>

    <!-- Nút gọi / Zalo nổi (chỉ trên web công khai) -->
    <div v-if="!editable && floatContact" class="fixed bottom-5 right-4 z-50 flex flex-col gap-3">
      <a
        v-if="floatContact.zalo"
        :href="safeHref(floatContact.zalo)"
        target="_blank"
        rel="noopener"
        class="flex h-14 w-14 items-center justify-center rounded-full bg-sky-500 text-xs font-extrabold text-white shadow-xl transition hover:scale-105"
        aria-label="Chat Zalo"
      >
        Zalo
      </a>
      <a
        v-if="floatContact.phone"
        :href="safeHref('tel:' + floatContact.phone)"
        class="pp-ring flex h-14 w-14 items-center justify-center rounded-full text-white shadow-xl transition hover:scale-105"
        :style="{ backgroundColor: accent }"
        aria-label="Gọi điện"
      >
        <Phone class="h-6 w-6" />
      </a>
    </div>
  </main>
</template>

<style>
@keyframes pp-ring {
  0%, 100% { transform: rotate(0); }
  5%, 15% { transform: rotate(-12deg); }
  10%, 20% { transform: rotate(12deg); }
  25% { transform: rotate(0); }
}
.pp-ring svg {
  animation: pp-ring 2.5s ease-in-out infinite;
}
@media (prefers-reduced-motion: reduce) {
  .pp-ring svg { animation: none; }
}
</style>
