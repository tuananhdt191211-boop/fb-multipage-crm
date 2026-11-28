<script setup lang="ts">
// Bộ sưu tập ảnh / video của sản phẩm trên web công khai: khung xem lớn + dải ảnh nhỏ, phát video tải lên, YouTube / Vimeo nhúng.
import { computed, ref, watch } from 'vue'
import { ChevronLeft, ChevronRight, Play } from 'lucide-vue-next'
import { safeImg } from '../../lib/safeUrl'
import { t } from '../../lib/i18n'
import type { ProductMedia } from '../../lib/productMedia'

const props = defineProps<{ media?: ProductMedia[]; fallback?: string; name: string; accent?: string }>()

const index = ref(0)
const items = computed<ProductMedia[]>(() => {
  if (props.media?.length) return props.media
  return props.fallback ? [{ type: 'image', url: props.fallback }] : []
})
watch(items, () => (index.value = 0))
const current = computed(() => items.value[index.value])

// Chỉ nhúng đúng 2 nguồn đã chuẩn hoá ở máy chủ (phòng thủ lớp hai)
const EMBED_OK = /^https:\/\/(www\.youtube-nocookie\.com\/embed\/[\w-]+|player\.vimeo\.com\/video\/\d+)(\?.*)?$/
const embedSrc = (u: string) => (EMBED_OK.test(u) ? u : '')
const thumb = (m: ProductMedia) => safeImg(m.type === 'image' ? m.url : m.poster)

function go(d: number) {
  const n = items.value.length
  if (n > 1) index.value = (index.value + d + n) % n
}

let touchX = 0
function onTouchStart(e: TouchEvent) {
  touchX = e.changedTouches[0]?.clientX ?? 0
}
function onTouchEnd(e: TouchEvent) {
  const dx = (e.changedTouches[0]?.clientX ?? 0) - touchX
  if (Math.abs(dx) > 50) go(dx < 0 ? 1 : -1)
}
</script>

<template>
  <div class="space-y-3">
    <div class="relative aspect-square overflow-hidden rounded-3xl bg-slate-100" @touchstart.passive="onTouchStart" @touchend.passive="onTouchEnd">
      <img v-if="current?.type === 'image' && safeImg(current.url)" :key="current.url" :src="safeImg(current.url)" :alt="current.alt || name" referrerpolicy="no-referrer" class="h-full w-full object-cover" />
      <video
        v-else-if="current?.type === 'video' && safeImg(current.url)"
        :key="current.url"
        :src="safeImg(current.url)"
        :poster="safeImg(current.poster) || undefined"
        controls
        playsinline
        preload="metadata"
        class="h-full w-full bg-black object-contain"
      />
      <iframe
        v-else-if="current?.type === 'embed' && embedSrc(current.url)"
        :key="current.url"
        :src="embedSrc(current.url)"
        :title="name"
        class="h-full w-full"
        loading="lazy"
        allowfullscreen
        referrerpolicy="strict-origin-when-cross-origin"
        allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture"
      />
      <slot />
      <template v-if="items.length > 1">
        <button type="button" class="absolute left-3 top-1/2 -translate-y-1/2 rounded-full bg-white/85 p-2 shadow hover:bg-white" :aria-label="t('slide_prev')" @click="go(-1)">
          <ChevronLeft class="h-5 w-5" />
        </button>
        <button type="button" class="absolute right-3 top-1/2 -translate-y-1/2 rounded-full bg-white/85 p-2 shadow hover:bg-white" :aria-label="t('slide_next')" @click="go(1)">
          <ChevronRight class="h-5 w-5" />
        </button>
      </template>
    </div>

    <div v-if="items.length > 1" class="flex gap-2 overflow-x-auto pb-1">
      <button
        v-for="(m, i) in items"
        :key="m.url + i"
        type="button"
        class="relative h-16 w-16 shrink-0 overflow-hidden rounded-xl border-2 bg-slate-100 sm:h-[72px] sm:w-[72px]"
        :style="i === index ? { borderColor: accent || '#059669' } : undefined"
        :class="i === index ? '' : 'border-transparent opacity-70 hover:opacity-100'"
        :aria-label="t('slide_n', { n: i + 1 })"
        @click="index = i"
      >
        <img v-if="thumb(m)" :src="thumb(m)" alt="" loading="lazy" referrerpolicy="no-referrer" class="h-full w-full object-cover" />
        <span v-if="m.type !== 'image'" class="absolute inset-0 flex items-center justify-center bg-black/30 text-white"><Play class="h-5 w-5" /></span>
      </button>
    </div>
  </div>
</template>
