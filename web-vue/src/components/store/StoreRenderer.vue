<script setup lang="ts">
import { safeHref, safeImg } from '../../lib/safeUrl'
import { formatVnd } from '../../lib/format'
import type { BlockStyle, PublicProduct, StoreBlock } from '../../types/store'

const props = defineProps<{
  siteTitle: string
  primaryColor: string
  blocks: StoreBlock[]
  products?: PublicProduct[]
  logoUrl?: string
}>()

const padMap: Record<string, string> = { none: '0', sm: '16px', md: '32px', lg: '64px' }

// Nền và khoảng cách thêm do người dùng chọn cho từng section
function wrapStyle(blk: StoreBlock): Record<string, string> {
  const st = (blk.data?.style || {}) as BlockStyle
  const s: Record<string, string> = {}
  if (st.bg) s.backgroundColor = st.bg
  if (st.pad && padMap[st.pad] !== undefined) {
    s.paddingTop = padMap[st.pad]!
    s.paddingBottom = padMap[st.pad]!
  }
  // Đầu trang ghim khi cuộn (phải có nền để không bị chữ phía dưới xuyên qua)
  if (blk.type === 'header' && blk.data?.sticky) {
    s.position = 'sticky'
    s.top = '0'
    s.zIndex = '20'
    if (!s.backgroundColor) s.backgroundColor = '#ffffff'
  }
  return s
}

// ----- Đầu trang -----
function navItems(blk: StoreBlock): { label: string; link: string }[] {
  const arr = Array.isArray(blk.data.nav_items) ? blk.data.nav_items : []
  return arr
    .map((x: any) =>
      typeof x === 'string' ? { label: x, link: '' } : { label: String(x?.label || ''), link: String(x?.link || '') }
    )
    .filter((x: { label: string }) => x.label.trim())
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
    case 'split': // logo trái · menu giữa · nút phải
      return {
        wrap: 'flex flex-wrap items-center gap-4 md:grid md:grid-cols-[auto_1fr_auto]',
        brand: '',
        nav: 'justify-center',
        cta: ''
      }
    case 'center': // logo giữa · menu bên dưới
      return {
        wrap: 'flex flex-wrap items-center justify-center gap-x-6 gap-y-3',
        brand: 'basis-full justify-center',
        nav: 'justify-center',
        cta: ''
      }
    case 'center_split': // menu trái · logo giữa · nút phải
      return {
        wrap: 'flex flex-wrap items-center justify-center gap-4 md:grid md:grid-cols-[1fr_auto_1fr]',
        brand: 'md:order-2 justify-center',
        nav: 'md:order-1 justify-start',
        cta: 'md:order-3 md:justify-self-end'
      }
    default: // logo trái · menu và nút bên phải
      return {
        wrap: 'flex flex-wrap items-center gap-x-6 gap-y-3',
        brand: 'mr-auto',
        nav: '',
        cta: ''
      }
  }
}

function alignOf(v?: string) {
  return v === 'center' || v === 'right' ? v : 'left'
}

function clampHeight(h?: number) {
  const n = Number(h) || 300
  return Math.min(2000, Math.max(80, n))
}

// Mã HTML của khách chạy trong iframe sandbox (xem thuộc tính sandbox bên dưới)
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
</script>

<template>
  <main class="mx-auto max-w-6xl bg-white text-slate-900">
    <template v-for="blk in blocks" :key="blk.id">
      <template v-if="blk.enabled">
        <div :style="wrapStyle(blk)">
          <!-- ===== Đầu trang ===== -->
                 <header
            v-if="blk.type === 'header'"
            class="px-6 py-4"
            :class="blk.data.border === false ? '' : 'border-b border-slate-100'"
            :style="blk.data.text_color ? { color: blk.data.text_color } : undefined"
          >
            <div :class="headerLayout(blk.data.layout).wrap">
              <!-- Logo + tên -->
              <div class="flex items-center gap-2" :class="headerLayout(blk.data.layout).brand">
                <img
                  v-if="logoSrc(blk)"
                  :src="logoSrc(blk)"
                  :alt="blk.data.brand_name || siteTitle"
                  class="w-auto max-w-[200px] object-contain"
                  :style="{ height: logoH(blk) + 'px' }"
                />
                <span
                  v-if="blk.data.show_brand !== false || !logoSrc(blk)"
                  class="font-extrabold"
                  :class="[brandSizeClass(blk.data.brand_size), blk.data.text_color ? '' : 'text-slate-900']"
                >
                  {{ blk.data.brand_name || siteTitle }}
                </span>
              </div>

              <!-- Menu -->
              <nav
                v-if="navItems(blk).length"
                class="flex flex-wrap items-center gap-x-5 gap-y-1 text-sm"
                :class="[headerLayout(blk.data.layout).nav, blk.data.text_color ? '' : 'text-slate-600']"
              >
                <template v-for="(item, idx) in navItems(blk)" :key="idx">
                  <a v-if="item.link" :href="safeHref(item.link)" class="hover:underline">{{ item.label }}</a>
                  <span v-else>{{ item.label }}</span>
                </template>
              </nav>

              <!-- Nút -->
              <a
                v-if="blk.data.show_cta !== false"
                :href="safeHref(blk.data.cta_link, '#products')"
                class="rounded-xl px-4 py-2 text-xs font-bold text-white"
                :class="headerLayout(blk.data.layout).cta"
                :style="{ backgroundColor: primaryColor }"
              >
                {{ blk.data.cta_label || 'Đặt mua ngay' }}
              </a>
            </div>
          </header>

          <!-- ===== Banner chính ===== -->
          <section
            v-else-if="blk.type === 'hero'"
            class="grid grid-cols-1 items-center gap-8 px-6 py-12 md:grid-cols-2"
          >
            <div class="space-y-4">
              <span
                v-if="blk.data.badge"
                class="inline-block rounded-full px-3 py-1 text-xs font-bold text-white"
                :style="{ backgroundColor: primaryColor }"
              >
                {{ blk.data.badge }}
              </span>
              <h1 class="text-2xl sm:text-3xl font-extrabold leading-tight text-slate-900">{{ blk.data.title }}</h1>
              <p class="whitespace-pre-line text-sm leading-relaxed text-slate-600">{{ blk.data.subtitle }}</p>
              <div class="pt-1">
                <a
                  :href="safeHref(blk.data.cta_link, '#products')"
                  class="inline-block rounded-xl px-5 py-2.5 text-sm font-bold text-white"
                  :style="{ backgroundColor: primaryColor }"
                >
                  {{ blk.data.cta_label || 'Khám phá ngay' }}
                </a>
              </div>
            </div>
            <div v-if="safeImg(blk.data.image_url)" class="overflow-hidden rounded-2xl border border-slate-200 bg-white">
              <img :src="safeImg(blk.data.image_url)" :alt="blk.data.title" class="h-full w-full object-cover" />
            </div>
          </section>

          <!-- ===== Sản phẩm ===== -->
          <section v-else-if="blk.type === 'products'" id="products" class="space-y-5 px-6 py-12">
            <h2 class="text-xl font-extrabold text-slate-900">{{ blk.data.heading || 'Sản phẩm nổi bật' }}</h2>

            <template v-if="blk.data.source === 'catalog'">
              <p v-if="!products?.length" class="text-sm text-slate-500">Sản phẩm sắp ra mắt.</p>
              <div class="grid grid-cols-1 gap-5 sm:grid-cols-2 lg:grid-cols-3">
                <div
                  v-for="prod in products || []"
                  :key="prod.id"
                  class="rounded-2xl border border-slate-200 bg-white p-4 space-y-2.5 shadow-2xs"
                >
                  <div v-if="safeImg(prod.image_url)" class="relative aspect-square overflow-hidden rounded-xl bg-slate-100">
                    <img :src="safeImg(prod.image_url)" :alt="prod.name" class="h-full w-full object-cover" />
                    <span
                      v-if="!prod.in_stock"
                      class="absolute left-2 top-2 rounded-full bg-slate-900/80 px-2 py-0.5 text-[10px] font-bold text-white"
                    >
                      Hết hàng
                    </span>
                  </div>
                  <div class="text-sm font-bold text-slate-900">{{ prod.name }}</div>
                  <div class="flex items-baseline gap-2">
                    <span class="text-sm font-extrabold" :style="{ color: primaryColor }">{{ formatVnd(prod.price) }}</span>
                    <span v-if="prod.compare_price > prod.price" class="text-xs text-slate-400 line-through">
                      {{ formatVnd(prod.compare_price) }}
                    </span>
                  </div>
                  <p class="text-xs leading-relaxed text-slate-600">{{ prod.description }}</p>
                </div>
              </div>
            </template>

            <div v-else class="grid grid-cols-1 gap-5 sm:grid-cols-2 lg:grid-cols-3">
              <div
                v-for="(prod, pIdx) in blk.data.items || []"
                :key="prod.id || pIdx"
                class="rounded-2xl border border-slate-200 bg-white p-4 space-y-2.5 shadow-2xs"
              >
                <div v-if="safeImg(prod.image_url)" class="aspect-square overflow-hidden rounded-xl bg-slate-100">
                  <img :src="safeImg(prod.image_url)" :alt="prod.name" class="h-full w-full object-cover" />
                </div>
                <div class="text-sm font-bold text-slate-900">{{ prod.name }}</div>
                <div class="text-sm font-extrabold" :style="{ color: primaryColor }">{{ prod.price }}</div>
                <p class="text-xs leading-relaxed text-slate-600">{{ prod.desc }}</p>
              </div>
            </div>
          </section>

          <!-- ===== Cam kết ===== -->
          <section v-else-if="blk.type === 'features'" class="px-6 py-10 space-y-4">
            <h2 class="text-lg font-extrabold text-slate-900">{{ blk.data.heading || 'Cam kết dịch vụ' }}</h2>
            <div class="grid grid-cols-1 gap-4 sm:grid-cols-3">
              <div
                v-for="(feat, fIdx) in blk.data.items || []"
                :key="fIdx"
                class="rounded-xl border border-slate-200 bg-white p-4 text-sm font-semibold text-slate-700"
              >
                ✓ {{ feat }}
              </div>
            </div>
          </section>

          <!-- ===== Đánh giá ===== -->
          <section v-else-if="blk.type === 'testimonials'" class="px-6 py-10 space-y-4">
            <h2 class="text-lg font-extrabold text-slate-900">{{ blk.data.heading || 'Khách hàng nói gì' }}</h2>
            <div class="grid grid-cols-1 gap-4 sm:grid-cols-3">
              <figure
                v-for="(t, tIdx) in blk.data.items || []"
                :key="tIdx"
                class="rounded-xl border border-slate-200 bg-white p-4 text-sm text-slate-700"
              >
                <blockquote>“{{ t.quote }}”</blockquote>
                <figcaption class="mt-2 text-xs font-bold text-slate-500">{{ t.name }}</figcaption>
              </figure>
            </div>
          </section>

          <!-- ===== Văn bản ===== -->
          <section
            v-else-if="blk.type === 'richtext'"
            class="px-6 py-10 space-y-3"
            :style="{ textAlign: alignOf(blk.data.align) }"
          >
            <h2 v-if="blk.data.heading" class="text-xl font-extrabold text-slate-900">{{ blk.data.heading }}</h2>
            <p class="whitespace-pre-line text-sm leading-relaxed text-slate-600">{{ blk.data.body }}</p>
          </section>

          <!-- ===== Ảnh lớn ===== -->
          <section v-else-if="blk.type === 'image'" class="px-6 py-6">
            <component
              :is="blk.data.link ? 'a' : 'div'"
              :href="blk.data.link ? safeHref(blk.data.link) : undefined"
              class="block overflow-hidden rounded-2xl"
            >
              <img
                v-if="safeImg(blk.data.image_url)"
                :src="safeImg(blk.data.image_url)"
                :alt="blk.data.alt || ''"
                class="w-full object-cover"
              />
              <div v-else class="flex h-40 items-center justify-center bg-slate-100 text-xs text-slate-400">
                Chưa chọn ảnh
              </div>
            </component>
          </section>

          <!-- ===== Thư viện ảnh ===== -->
          <section v-else-if="blk.type === 'gallery'" class="px-6 py-10 space-y-4">
            <h2 v-if="blk.data.heading" class="text-lg font-extrabold text-slate-900">{{ blk.data.heading }}</h2>
            <div class="grid grid-cols-2 gap-3 sm:grid-cols-3">
              <figure
                v-for="(g, gIdx) in blk.data.items || []"
                :key="gIdx"
                class="overflow-hidden rounded-xl border border-slate-200 bg-white"
              >
                <img
                  v-if="safeImg(g.image_url)"
                  :src="safeImg(g.image_url)"
                  :alt="g.caption || ''"
                  class="aspect-square w-full object-cover"
                />
                <div v-else class="flex aspect-square items-center justify-center bg-slate-100 text-xs text-slate-400">
                  Chưa chọn ảnh
                </div>
                <figcaption v-if="g.caption" class="p-2 text-xs text-slate-600">{{ g.caption }}</figcaption>
              </figure>
            </div>
          </section>

          <!-- ===== Câu hỏi thường gặp ===== -->
          <section v-else-if="blk.type === 'faq'" class="px-6 py-10 space-y-3">
            <h2 v-if="blk.data.heading" class="text-lg font-extrabold text-slate-900">{{ blk.data.heading }}</h2>
            <details
              v-for="(f, fqIdx) in blk.data.items || []"
              :key="fqIdx"
              class="rounded-xl border border-slate-200 bg-white p-4"
            >
              <summary class="cursor-pointer text-sm font-bold text-slate-800">{{ f.q }}</summary>
              <p class="mt-2 whitespace-pre-line text-sm leading-relaxed text-slate-600">{{ f.a }}</p>
            </details>
          </section>

          <!-- ===== Liên hệ ===== -->
          <section v-else-if="blk.type === 'contact'" class="px-6 py-10 space-y-3">
            <h2 v-if="blk.data.heading" class="text-lg font-extrabold text-slate-900">{{ blk.data.heading }}</h2>
            <ul class="space-y-1.5 text-sm text-slate-700">
              <li v-if="blk.data.phone">
                📞
                <a :href="safeHref('tel:' + blk.data.phone)" class="font-semibold hover:underline">{{ blk.data.phone }}</a>
              </li>
              <li v-if="blk.data.email">
                ✉️
                <a :href="safeHref('mailto:' + blk.data.email)" class="font-semibold hover:underline">{{ blk.data.email }}</a>
              </li>
              <li v-if="blk.data.address">📍 {{ blk.data.address }}</li>
              <li v-if="blk.data.zalo_link">
                💬
                <a :href="safeHref(blk.data.zalo_link)" target="_blank" rel="noopener" class="font-semibold hover:underline">
                  Nhắn Zalo
                </a>
              </li>
            </ul>
          </section>

          <!-- ===== HTML tự viết: chạy trong iframe cách ly ===== -->
          <section v-else-if="blk.type === 'html'" class="px-6 py-4">
            <iframe
              :srcdoc="htmlDoc(blk.data.html)"
              sandbox="allow-scripts allow-popups"
              referrerpolicy="no-referrer"
              loading="lazy"
              title="Nội dung nhúng"
              class="w-full rounded-xl border-0"
              :style="{ height: clampHeight(blk.data.height) + 'px' }"
            ></iframe>
          </section>

          <!-- ===== Kêu gọi hành động & chân trang ===== -->
          <footer
            v-else-if="blk.type === 'cta'"
            id="contact"
            class="border-t border-slate-100 px-6 py-12 text-center space-y-3"
          >
            <h2 class="text-xl font-extrabold text-slate-900">{{ blk.data.title }}</h2>
            <p class="mx-auto max-w-lg text-sm text-slate-600">{{ blk.data.subtitle }}</p>
            <div>
              <a
                :href="safeHref(blk.data.button_link)"
                class="inline-block rounded-xl px-5 py-2.5 text-sm font-bold text-white"
                :style="{ backgroundColor: primaryColor }"
              >
                {{ blk.data.button_text || 'Liên hệ ngay' }}
              </a>
            </div>
            <p class="pt-4 text-xs text-slate-400">{{ blk.data.footer_note }}</p>
          </footer>
        </div>
      </template>
    </template>
  </main>
</template>