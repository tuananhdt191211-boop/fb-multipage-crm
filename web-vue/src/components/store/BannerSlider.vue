<script setup lang="ts">
// Banner ảnh tràn màn hình, nhiều ảnh tự trượt; ảnh riêng cho điện thoại (không bắt buộc)
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { ChevronLeft, ChevronRight } from 'lucide-vue-next'
import { safeHref, safeImg } from '../../lib/safeUrl'
import EditableText from './EditableText.vue'
import { pathKey } from '../../lib/textStyle'
import { t } from '../../lib/i18n'
import type { StoreBlock, TextFx, TextStyle } from '../../types/store'

const props = defineProps<{ blk: StoreBlock; editable?: boolean; accent: string }>()
const emit = defineEmits<{ (e: 'edit', path: (string | number)[], v: string): void }>()

function fx(path: (string | number)[]): TextFx {
  const key = pathKey(path)
  return { block: props.blk.id, key, style: props.blk.data?._text?.[key] as TextStyle | undefined }
}

const slides = computed<any[]>(() => (Array.isArray(props.blk.data.slides) ? props.blk.data.slides : []))
const idx = ref(0)
const heightClass = computed(() => {
  switch (props.blk.data.height) {
    case 'sm':
      return 'min-h-[260px] @3xl:min-h-[360px]'
    case 'md':
      return 'min-h-[340px] @3xl:min-h-[480px]'
    case 'full':
      return 'min-h-[480px] @3xl:min-h-[86vh]'
    default:
      return 'min-h-[420px] @3xl:min-h-[600px]'
  }
})

function go(i: number) {
  const n = slides.value.length
  if (!n) return
  idx.value = (i + n) % n
}

let timer: ReturnType<typeof setInterval> | null = null
function start() {
  stop()
  const sec = Number(props.blk.data.autoplay) || 0
  if (props.editable || sec <= 0 || slides.value.length < 2) return
  timer = setInterval(() => go(idx.value + 1), Math.max(3, sec) * 1000)
}
function stop() {
  if (timer) clearInterval(timer)
  timer = null
}
onMounted(start)
onBeforeUnmount(stop)
watch(() => [props.blk.data.autoplay, slides.value.length, props.editable], start)
watch(
  () => slides.value.length,
  (n) => {
    if (idx.value >= n) idx.value = Math.max(0, n - 1)
  }
)

// Vuốt trên điện thoại
let touchX = 0
function onTouchStart(e: TouchEvent) {
  touchX = e.touches[0]?.clientX || 0
}
function onTouchEnd(e: TouchEvent) {
  const dx = (e.changedTouches[0]?.clientX || 0) - touchX
  if (Math.abs(dx) > 40) go(idx.value + (dx < 0 ? 1 : -1))
}
</script>

<template>
  <section class="relative overflow-hidden bg-slate-900" @mouseenter="stop" @mouseleave="start" @touchstart.passive="onTouchStart" @touchend="onTouchEnd">
    <div class="flex transition-transform duration-700 ease-out" :style="{ transform: `translateX(-${idx * 100}%)` }">
      <div v-for="(s, i) in slides" :key="i" class="relative flex w-full shrink-0 items-center" :class="heightClass">
        <!-- Ảnh -->
        <template v-if="safeImg(s.image_url)">
          <img :src="safeImg(s.image_url)" :alt="s.heading || ''" class="absolute inset-0 h-full w-full object-cover" :class="safeImg(s.mobile_image_url) ? 'hidden @3xl:block' : ''" :loading="i === 0 ? 'eager' : 'lazy'" />
          <img v-if="safeImg(s.mobile_image_url)" :src="safeImg(s.mobile_image_url)" :alt="s.heading || ''" class="absolute inset-0 h-full w-full object-cover @3xl:hidden" loading="lazy" />
        </template>
        <div v-else class="absolute inset-0" :style="{ background: `linear-gradient(120deg, color-mix(in srgb, ${accent} 85%, black) 0%, ${accent} 55%, color-mix(in srgb, ${accent} 55%, white) 100%)` }">
          <span v-if="editable" data-i18n="on" class="absolute right-4 top-4 rounded-lg bg-black/30 px-2 py-1 text-[11px] text-white">Chọn ảnh ở bảng bên phải</span>
        </div>
        <div v-if="blk.data.overlay !== false && safeImg(s.image_url)" class="absolute inset-0 bg-gradient-to-r from-black/55 via-black/25 to-transparent" />

        <!-- Chữ -->
        <div class="relative mx-auto w-full max-w-6xl px-6 py-16 @3xl:px-10">
          <div class="max-w-xl space-y-4 text-white">
            <EditableText :fx="fx(['slides', i, 'heading'])"
              v-if="s.heading || editable"
              tag="h2"
              :model-value="s.heading"
              :editable="editable"
              placeholder="Tiêu đề banner"
              class="block text-3xl font-extrabold leading-tight tracking-tight drop-shadow-sm @3xl:text-5xl"
              @update:model-value="emit('edit', ['slides', i, 'heading'], $event)"
            />
            <EditableText :fx="fx(['slides', i, 'text'])"
              v-if="s.text || editable"
              tag="p"
              multiline
              :model-value="s.text"
              :editable="editable"
              placeholder="Mô tả ngắn"
              class="block text-base leading-relaxed text-white/90 @3xl:text-lg"
              @update:model-value="emit('edit', ['slides', i, 'text'], $event)"
            />
            <div v-if="s.button_label || editable" class="pt-2">
              <a :href="safeHref(s.link, '#products')" class="inline-flex rounded-full bg-white px-7 py-3.5 text-sm font-bold shadow-lg transition hover:-translate-y-0.5" :style="{ color: accent }">
                <EditableText :fx="fx(['slides', i, 'button_label'])" :model-value="s.button_label" :editable="editable" placeholder="Chữ trên nút" @update:model-value="emit('edit', ['slides', i, 'button_label'], $event)" />
              </a>
            </div>
          </div>
        </div>
      </div>
    </div>

    <template v-if="slides.length > 1">
      <button type="button" class="absolute left-3 top-1/2 hidden h-10 w-10 -translate-y-1/2 items-center justify-center rounded-full bg-white/80 text-slate-800 shadow hover:bg-white @3xl:flex" :aria-label="t('slide_prev')" @click.stop="go(idx - 1)">
        <ChevronLeft class="h-5 w-5" />
      </button>
      <button type="button" class="absolute right-3 top-1/2 hidden h-10 w-10 -translate-y-1/2 items-center justify-center rounded-full bg-white/80 text-slate-800 shadow hover:bg-white @3xl:flex" :aria-label="t('slide_next')" @click.stop="go(idx + 1)">
        <ChevronRight class="h-5 w-5" />
      </button>
      <div class="absolute inset-x-0 bottom-4 flex justify-center gap-2">
        <button
          v-for="(_, i) in slides"
          :key="i"
          type="button"
          class="h-2 rounded-full transition-all"
          :class="i === idx ? 'w-7 bg-white' : 'w-2 bg-white/50 hover:bg-white/80'"
          :aria-label="t('slide_n', { n: i + 1 })"
          @click.stop="go(i)"
        />
      </div>
    </template>
  </section>
</template>
