<script setup lang="ts">
import { safeHref, safeImg } from '../../lib/safeUrl'
import { formatVnd } from '../../lib/format'
import type { PublicProduct, StoreBlock } from '../../types/store'

defineProps<{
  siteTitle: string
  primaryColor: string
  blocks: StoreBlock[]
  products?: PublicProduct[]
}>()
</script>

<template>
  <main class="mx-auto max-w-6xl bg-white text-slate-900">
    <template v-for="blk in blocks" :key="blk.id">
      <template v-if="blk.enabled">
        <header
          v-if="blk.type === 'header'"
          class="flex flex-wrap items-center justify-between gap-4 border-b border-slate-100 px-6 py-4"
        >
          <div class="text-base font-extrabold text-slate-900">
            {{ blk.data.brand_name || siteTitle }}
          </div>
          <div class="flex items-center gap-5 text-sm text-slate-600">
            <span v-for="(navItem, idx) in blk.data.nav_items || []" :key="idx">{{ navItem }}</span>
            <a
              :href="safeHref(blk.data.cta_link, '#products')"
              class="rounded-xl px-4 py-2 text-xs font-bold text-white"
              :style="{ backgroundColor: primaryColor }"
            >
              {{ blk.data.cta_label || 'Đặt mua ngay' }}
            </a>
          </div>
        </header>

        <section
          v-else-if="blk.type === 'hero'"
          class="grid grid-cols-1 items-center gap-8 bg-slate-50/70 px-6 py-12 md:grid-cols-2"
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
            <p class="text-sm leading-relaxed text-slate-600">{{ blk.data.subtitle }}</p>
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

             <section v-else-if="blk.type === 'products'" id="products" class="space-y-5 px-6 py-12">
          <h2 class="text-xl font-extrabold text-slate-900">{{ blk.data.heading || 'Sản phẩm nổi bật' }}</h2>

          <!-- Lấy từ danh sách sản phẩm của shop -->
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

          <!-- Nhập tay (kiểu cũ) -->
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

        <section v-else-if="blk.type === 'features'" class="bg-slate-50/70 px-6 py-10 space-y-4">
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
      </template>
    </template>
  </main>
</template>