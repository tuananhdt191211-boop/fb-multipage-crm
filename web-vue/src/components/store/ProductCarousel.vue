<script setup lang="ts">
// Danh sách sản phẩm dạng trượt (carousel). Thẻ sản phẩm do nơi dùng truyền vào qua slot (mỗi phần tử con = một thẻ).
// Kiểu hiển thị:
// - slide:   trượt ngang, vuốt bằng tay / mũi tên, dừng đúng đầu thẻ
// - dots:    chia trang, có chấm tròn bên dưới và tự chuyển trang
// - center:  thẻ ở giữa to nhất, hai bên nhỏ và mờ dần (vuốt / mũi tên / tự chạy)
// - marquee: dải chạy liên tục, rê chuột vào thì dừng
// Số thẻ trên một hàng đổi theo bề rộng khung chứa (container query), nên xem trước điện thoại / máy tính bảng trong trình dựng đúng như thật.
import { computed, nextTick, onBeforeUnmount, onMounted, onUpdated, ref, watch } from 'vue'
import { ChevronLeft, ChevronRight } from 'lucide-vue-next'
import { t } from '../../lib/i18n'
import type { CarouselLayout } from '../../types/store'


const props = defineProps<{
  layout: CarouselLayout
  count: number // số thẻ
  perView?: number // số thẻ trên một hàng (máy tính)
  autoplay?: number // giây, 0 = tắt
  arrows?: boolean
  dots?: boolean
  speed?: 'slow' | 'normal' | 'fast' // chỉ cho marquee
  accent?: string
  editable?: boolean // trong trình chỉnh sửa: không tự chạy
}>()

const pvLg = computed(() => Math.min(6, Math.max(1, Math.round(props.perView || 4))))
const pvMd = computed(() => Math.min(pvLg.value, 3))
// Điện thoại: trượt/marquee hé lộ thẻ kế tiếp; chia trang thì đúng 2 thẻ; tâm điểm thì 1 thẻ giữa + hai bên hé
const pvSm = computed(() => (props.layout === 'center' ? 1.45 : props.layout === 'dots' ? 2 : 1.7))

const vars = computed(() => ({ '--pv-lg': pvLg.value, '--pv-md': pvMd.value, '--pv-sm': pvSm.value, '--accent': props.accent || '#059669' }))

// Chế độ marquee chỉ chạy trên web công khai; trình chỉnh sửa dùng dải cuộn tay để thấy hết thẻ
const isMarquee = computed(() => props.layout === 'marquee' && !props.editable)
const showArrows = computed(() => props.arrows !== false && !isMarquee.value && overflowing.value)
const showDots = computed(() => (props.dots ?? (props.layout === 'dots' || props.layout === 'center')) && !isMarquee.value && overflowing.value && pages.value > 1)

// ---------- marquee ----------
const reps = computed(() => Math.max(1, Math.ceil((pvLg.value + 1) / Math.max(1, props.count))))
const mqSeconds = computed(() => {
  const per = props.speed === 'slow' ? 8 : props.speed === 'fast' ? 3 : 5
  return Math.max(12, props.count * reps.value * per)
})

// ---------- trượt ----------
const track = ref<HTMLElement | null>(null)
const pvNow = ref(pvLg.value)
const overflowing = ref(false)
const page = ref(0)
const hover = ref(false)

const perPage = computed(() => Math.max(1, Math.floor(pvNow.value + 0.01)))
const pages = computed(() => (props.layout === 'center' ? props.count : Math.max(1, Math.ceil(props.count / perPage.value))))

function stride(): number {
  const el = track.value
  const a = el?.children[0] as HTMLElement | undefined
  if (!el || !a) return 0
  const b = el.children[1] as HTMLElement | undefined
  return b ? b.offsetLeft - a.offsetLeft : a.offsetWidth
}

function applyCenter() {
  const el = track.value
  if (!el || props.layout !== 'center') return
  const s = stride() || 1
  const mid = el.scrollLeft + el.clientWidth / 2
  for (const k of Array.from(el.children) as HTMLElement[]) {
    const d = Math.abs(k.offsetLeft + k.offsetWidth / 2 - mid) / s
    k.style.transform = `scale(${(1 - Math.min(d, 1.6) * 0.075).toFixed(3)})`
    k.style.opacity = String(Math.max(0.55, 1 - Math.min(d, 2) * 0.22).toFixed(2))
  }
}

function measure() {
  const el = track.value
  if (!el) return
  const v = parseFloat(getComputedStyle(el).getPropertyValue('--pv'))
  if (v > 0) pvNow.value = v
  overflowing.value = el.scrollWidth > el.clientWidth + 2
  const s = stride() || 1
  const atEnd = el.scrollLeft + el.clientWidth >= el.scrollWidth - 2
  page.value = props.layout === 'center' ? Math.round(el.scrollLeft / s) : atEnd ? pages.value - 1 : Math.round(el.scrollLeft / (perPage.value * s))
  applyCenter()
}

let raf = 0
function onScroll() {
  if (raf) return
  raf = requestAnimationFrame(() => {
    raf = 0
    measure()
  })
}

function maxScroll(): number {
  const el = track.value
  return el ? el.scrollWidth - el.clientWidth : 0
}

// Đến cuối thì quay lại đầu (và ngược lại) để mũi tên / tự chạy không bị kẹt
function go(dir: 1 | -1) {
  const el = track.value
  const s = stride()
  if (!el || !s) return
  const step = props.layout === 'center' ? s : perPage.value * s
  const next = el.scrollLeft + dir * step
  if (dir === 1 && el.scrollLeft >= maxScroll() - 2) el.scrollTo({ left: 0, behavior: 'smooth' })
  else if (dir === -1 && el.scrollLeft <= 2) el.scrollTo({ left: maxScroll(), behavior: 'smooth' })
  else el.scrollTo({ left: Math.round(next / s) * s, behavior: 'smooth' })
}

function goPage(i: number) {
  const s = stride()
  track.value?.scrollTo({ left: i * (props.layout === 'center' ? 1 : perPage.value) * s, behavior: 'smooth' })
}

// ---------- tự chạy ----------
let timer: ReturnType<typeof setInterval> | null = null
function stopTimer() {
  if (timer) clearInterval(timer)
  timer = null
}
function startTimer() {
  stopTimer()
  const sec = props.autoplay ?? (props.layout === 'dots' ? 5 : 0)
  if (props.editable || isMarquee.value || !sec || sec < 1) return
  if (typeof matchMedia !== 'undefined' && matchMedia('(prefers-reduced-motion: reduce)').matches) return
  timer = setInterval(() => {
    if (hover.value || document.hidden || !overflowing.value) return
    go(1)
  }, sec * 1000)
}

let ro: ResizeObserver | null = null
onMounted(() => {
  nextTick(measure)
  if (typeof ResizeObserver !== 'undefined' && track.value) {
    ro = new ResizeObserver(() => measure())
    ro.observe(track.value)
  }
  startTimer()
})
onUpdated(measure)
watch(() => [props.autoplay, props.layout, props.editable], () => {
  startTimer()
  nextTick(measure)
})
watch(() => [props.layout, props.perView], () => {
  // đổi kiểu: bỏ hiệu ứng thu nhỏ của kiểu "tâm điểm" còn sót trên thẻ
  for (const k of Array.from(track.value?.children || []) as HTMLElement[]) {
    k.style.transform = ''
    k.style.opacity = ''
  }
  track.value?.scrollTo({ left: 0 })
  nextTick(measure)
})
onBeforeUnmount(() => {
  stopTimer()
  ro?.disconnect()
  if (raf) cancelAnimationFrame(raf)
})
</script>

<template>
  <div
    class="pc"
    :class="`pc-l-${isMarquee || layout !== 'marquee' ? layout : 'slide'}`"
    :style="vars"
    role="region"
    aria-roledescription="carousel"
    @mouseenter="hover = true"
    @mouseleave="hover = false"
    @focusin="hover = true"
    @focusout="hover = false"
  >
    <!-- Dải chạy liên tục: danh sách được lặp lại 2 lần rồi dịch đúng nửa độ dài nên vòng lặp liền mạch -->
    <div v-if="isMarquee" class="pc-mq" data-testid="carousel-marquee">
      <div class="pc-mq-track" :style="{ animationDuration: mqSeconds + 's' }">
        <div v-for="c in 2" :key="c" class="pc-mq-copy" :aria-hidden="c > 1 ? 'true' : undefined" :inert="c > 1 ? true : undefined">
          <template v-for="r in reps" :key="r"><slot /></template>
        </div>
      </div>
    </div>

    <div v-else class="pc-viewport">
      <div ref="track" class="pc-track" data-testid="carousel-track" @scroll.passive="onScroll">
        <slot />
      </div>
      <template v-if="showArrows">
        <button type="button" class="pc-arrow pc-prev" data-testid="carousel-prev" :aria-label="t('prev')" @click.stop="go(-1)"><ChevronLeft class="h-5 w-5" /></button>
        <button type="button" class="pc-arrow pc-next" data-testid="carousel-next" :aria-label="t('next')" @click.stop="go(1)"><ChevronRight class="h-5 w-5" /></button>
      </template>
    </div>

    <div v-if="showDots" class="pc-dots" data-testid="carousel-dots">
      <button
        v-for="i in pages"
        :key="i"
        type="button"
        class="pc-dot"
        :class="{ 'pc-dot-on': i - 1 === page }"
        :aria-label="t('page_n', { n: i })"
        :aria-current="i - 1 === page ? 'true' : undefined"
        @click.stop="goPage(i - 1)"
      />
    </div>
  </div>
</template>

<style>
.pc {
  --gap: 14px;
  --pv: var(--pv-sm);
  --item-w: calc((100cqw - (var(--pv) - 1) * var(--gap)) / var(--pv));
  position: relative;
  container-type: inline-size;
}
/* Số thẻ theo bề rộng của khung trang: điện thoại / máy tính bảng / máy tính */
@container (min-width: 36rem) {
  .pc { --pv: var(--pv-md); --gap: 18px; }
}
@container (min-width: 64rem) {
  .pc { --pv: var(--pv-lg); --gap: 24px; }
}

.pc-viewport { position: relative; }

/* ----- Trượt có chốt vị trí ----- */
.pc-track {
  position: relative;
  display: flex;
  gap: var(--gap);
  overflow-x: auto;
  scroll-snap-type: x mandatory;
  overscroll-behavior-x: contain;
  scrollbar-width: none;
  /* chừa chỗ cho bóng đổ / hiệu ứng nhấc thẻ khi rê chuột (overflow-x làm cắt luôn theo chiều dọc) */
  padding: 10px 0 22px;
  margin: -10px 0 -22px;
}
.pc-track::-webkit-scrollbar { display: none; }
.pc-track > * {
  flex: 0 0 var(--item-w);
  min-width: 0;
  scroll-snap-align: start;
}
.pc-l-center .pc-track > * {
  scroll-snap-align: center;
  transition: transform 0.3s ease, opacity 0.3s ease;
  will-change: transform;
}
/* Hai khoảng đệm để thẻ đầu / cuối cũng nằm được chính giữa */
.pc-l-center .pc-track::before,
.pc-l-center .pc-track::after {
  content: '';
  flex: 0 0 calc((100cqw - var(--item-w)) / 2 - var(--gap));
}

/* ----- Mũi tên ----- */
.pc-arrow {
  position: absolute;
  top: 50%;
  z-index: 5;
  display: flex;
  height: 2.5rem;
  width: 2.5rem;
  align-items: center;
  justify-content: center;
  border-radius: 9999px;
  background: rgba(255, 255, 255, 0.95);
  color: #0f172a;
  box-shadow: 0 4px 14px rgba(15, 23, 42, 0.18), 0 0 0 1px rgba(15, 23, 42, 0.06);
  transform: translateY(-50%);
  transition: background 0.15s, transform 0.15s;
}
.pc-arrow:hover { background: #fff; transform: translateY(-50%) scale(1.08); }
.pc-prev { left: 4px; }
.pc-next { right: 4px; }
@container (min-width: 64rem) {
  .pc-prev { left: -14px; }
  .pc-next { right: -14px; }
}

/* ----- Chấm tròn ----- */
.pc-dots { display: flex; justify-content: center; gap: 8px; padding-top: 18px; }
.pc-dot {
  height: 8px;
  width: 8px;
  border-radius: 9999px;
  background: #cbd5e1;
  transition: width 0.25s ease, background 0.25s ease;
}
.pc-dot:hover { background: #94a3b8; }
.pc-dot-on { width: 24px; background: var(--accent); }

/* ----- Chạy liên tục ----- */
.pc-mq {
  overflow: hidden;
  padding: 10px 0 22px;
  margin: -10px 0 -22px;
  -webkit-mask-image: linear-gradient(90deg, transparent, #000 5%, #000 95%, transparent);
  mask-image: linear-gradient(90deg, transparent, #000 5%, #000 95%, transparent);
}
.pc-mq-track {
  display: flex;
  width: max-content;
  animation: pc-marquee linear infinite;
}
.pc-mq:hover .pc-mq-track,
.pc-mq:focus-within .pc-mq-track { animation-play-state: paused; }
.pc-mq-copy { display: flex; flex: none; }
.pc-mq-copy > * {
  flex: none;
  width: var(--item-w);
  margin-right: var(--gap);
}
@keyframes pc-marquee {
  to { transform: translateX(-50%); }
}

@media (prefers-reduced-motion: reduce) {
  .pc-mq { overflow-x: auto; }
  .pc-mq-track { animation: none; }
  .pc-l-center .pc-track > * { transition: none; }
}
</style>
