<script setup lang="ts">
// Chọn ngôn ngữ giao diện quản trị + tiền tệ hiển thị. Tên ngôn ngữ luôn viết bằng chính ngôn ngữ đó (không bị bộ dịch đụng tới).
// variant="sidebar": một hàng trong thanh menu / ngăn kéo; variant="compact": nút nhỏ ở góc trang đăng nhập.
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import { Check, Globe } from 'lucide-vue-next'
import { UI_CURRENCIES, UI_LANGS, currencyName, ensureRates, setUiCurrency, setUiLang, ui } from '../lib/uiI18n'

const props = withDefaults(defineProps<{ variant?: 'sidebar' | 'compact'; collapsed?: boolean }>(), { variant: 'sidebar', collapsed: false })

const open = ref(false)
const btn = ref<HTMLElement | null>(null)
const pop = ref<HTMLElement | null>(null)
const pos = ref<Record<string, string>>({})

const current = computed(() => UI_LANGS.find((l) => l.code === ui.lang) ?? UI_LANGS[0]!)
// Chỉ hiện tiền tệ đã có tỉ giá (và loại đang chọn); VND luôn có
const currencies = computed(() => UI_CURRENCIES.filter((c) => c === 'VND' || c === ui.currency || ui.rates[c]))
const pct = computed(() => (ui.total ? Math.min(100, Math.round((ui.count / ui.total) * 100)) : 0))

// Đặt khung cạnh nút rồi kẹp lại cho nằm trọn trong màn hình (màn hình thấp / điện thoại vẫn thấy đủ, cuộn được bên trong)
const placed = ref(false)
async function place() {
  const r = btn.value?.getBoundingClientRect()
  if (!r) return
  await nextTick() // đợi khung hiện ra để đo chiều cao thật
  const margin = 8
  const w = Math.min(320, window.innerWidth - margin * 2)
  const h = Math.min(pop.value?.offsetHeight ?? 520, window.innerHeight - margin * 2)
  let left: number
  let top: number
  if (props.variant === 'sidebar') {
    // mở sang phải của thanh menu, đáy khung ngang đáy nút; không đủ chỗ bên phải (ngăn kéo điện thoại) thì đè lên trên
    left = r.right + 8
    if (left + w > window.innerWidth - margin) left = r.left
    top = r.bottom - h
  } else {
    left = r.right - w
    top = r.bottom + 8
  }
  left = Math.max(margin, Math.min(left, window.innerWidth - w - margin))
  top = Math.max(margin, Math.min(top, window.innerHeight - h - margin))
  pos.value = { left: left + 'px', top: top + 'px', width: w + 'px', maxHeight: window.innerHeight - margin * 2 + 'px' }
  placed.value = true
}

async function toggle() {
  open.value = !open.value
  placed.value = false
  if (open.value) {
    void ensureRates() // để danh sách tiền tệ đầy đủ
    await place()
  }
}

function pick(code: string) {
  void setUiLang(code)
}

function onCurrency(e: Event) {
  setUiCurrency((e.target as HTMLSelectElement).value)
}

function onDown(e: MouseEvent) {
  if (!open.value) return
  const t = e.target as Node
  if (btn.value?.contains(t) || pop.value?.contains(t)) return
  open.value = false
}
function onKey(e: KeyboardEvent) {
  if (e.key === 'Escape') open.value = false
}
function onResize() {
  if (open.value) void place()
}

onMounted(() => {
  document.addEventListener('mousedown', onDown)
  document.addEventListener('keydown', onKey)
  window.addEventListener('resize', onResize)
})
onBeforeUnmount(() => {
  document.removeEventListener('mousedown', onDown)
  document.removeEventListener('keydown', onKey)
  window.removeEventListener('resize', onResize)
})
</script>

<template>
  <div class="relative">
    <button
      ref="btn"
      type="button"
      :aria-expanded="open"
      aria-haspopup="dialog"
      title="Ngôn ngữ & tiền tệ"
      class="flex items-center gap-3 rounded-lg text-[13px] font-semibold transition-colors"
      :class="
        variant === 'compact'
          ? 'border border-slate-200 bg-white px-2.5 py-1.5 text-xs text-slate-600 shadow-xs hover:border-emerald-300 hover:text-emerald-700'
          : ['w-full px-3 py-2 text-slate-500 hover:bg-slate-50 hover:text-slate-800', collapsed ? 'justify-center px-0' : '']
      "
      @click="toggle"
    >
      <Globe :class="variant === 'compact' ? 'h-3.5 w-3.5' : 'h-[18px] w-[18px] shrink-0'" />
      <template v-if="variant === 'compact'">
        <span translate="no" data-i18n="skip">{{ current.name }}</span>
      </template>
      <template v-else-if="!collapsed">
        <span class="min-w-0 flex-1 truncate text-left" translate="no" data-i18n="skip">{{ current.name }}</span>
        <span class="shrink-0 rounded bg-slate-100 px-1.5 py-0.5 text-[10px] font-bold text-slate-500" translate="no" data-i18n="skip">{{ ui.currency }}</span>
      </template>
    </button>

    <Teleport to="body">
      <div
        v-if="open"
        ref="pop"
        role="dialog"
        class="fixed z-[90] w-[320px] max-w-[calc(100vw-16px)] overflow-y-auto rounded-2xl border border-slate-200 bg-white p-3 text-slate-800 shadow-xl"
        :style="[pos, { visibility: placed ? 'visible' : 'hidden' }]"
      >
        <p class="px-1 pb-2 text-[11px] font-bold uppercase tracking-wider text-slate-400">Ngôn ngữ giao diện</p>
        <div class="grid max-h-[44vh] grid-cols-2 gap-1 overflow-y-auto pr-0.5">
          <button
            v-for="l in UI_LANGS"
            :key="l.code"
            type="button"
            class="flex items-center justify-between gap-1 rounded-lg px-2.5 py-1.5 text-left text-xs font-semibold transition-colors"
            :class="l.code === ui.lang ? 'bg-emerald-50 text-emerald-800 ring-1 ring-emerald-200' : 'text-slate-600 hover:bg-slate-50'"
            @click="pick(l.code)"
          >
            <span class="truncate" translate="no" data-i18n="skip">{{ l.name }}</span>
            <Check v-if="l.code === ui.lang" class="h-3.5 w-3.5 shrink-0 text-emerald-600" />
          </button>
        </div>

        <div v-if="ui.status === 'filling'" class="mt-2 rounded-lg bg-emerald-50 px-2.5 py-2">
          <p class="text-[11px] font-semibold text-emerald-800">Đang dịch giao diện... {{ ui.count }}/{{ ui.total }}</p>
          <div class="mt-1.5 h-1.5 overflow-hidden rounded-full bg-emerald-100">
            <div class="h-full rounded-full bg-emerald-500 transition-[width] duration-300" :style="{ width: pct + '%' }" />
          </div>
        </div>
        <p v-else-if="ui.status === 'partial'" class="mt-2 rounded-lg bg-amber-50 px-2.5 py-2 text-[11px] leading-snug text-amber-800">
          Một số câu chưa được dịch nên tạm hiện tiếng Anh. Hệ thống sẽ dịch tiếp trong lần đăng nhập sau.
        </p>

        <div class="mt-3 border-t border-slate-100 pt-3">
          <label class="block px-1 text-[11px] font-bold uppercase tracking-wider text-slate-400" for="ui-currency">Tiền tệ hiển thị</label>
          <select
            id="ui-currency"
            class="mt-1.5 w-full rounded-xl border border-slate-200 bg-white px-2.5 py-2 text-xs font-semibold text-slate-700 outline-none focus:border-emerald-400"
            :value="ui.currencyAuto ? 'auto' : ui.currency"
            @change="onCurrency"
          >
            <option value="auto">Tự động theo ngôn ngữ</option>
            <option v-for="c in currencies" :key="c" :value="c" translate="no" data-i18n="skip">{{ c }} — {{ currencyName(c) }}</option>
          </select>
          <p class="mt-1.5 px-1 text-[11px] leading-snug text-slate-400">
            Giá trong hệ thống được nhập bằng VND; các tiền tệ khác chỉ là quy đổi tham khảo theo tỉ giá cập nhật hằng ngày.
          </p>
        </div>
      </div>
    </Teleport>
  </div>
</template>
