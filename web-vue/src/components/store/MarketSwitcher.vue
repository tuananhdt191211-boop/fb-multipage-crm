<script setup lang="ts">
// Chọn quốc gia / ngôn ngữ / tiền tệ trên đầu trang (đổi là đổi cả ngôn ngữ và tiền)
import { computed, ref } from 'vue'
import { Check, ChevronDown } from 'lucide-vue-next'
import { LANG_NAMES, MARKET_BY_CODE } from '../../lib/markets'
import { currentMarket, locale, setMarket, t } from '../../lib/i18n'

defineProps<{ compact?: boolean }>()

const open = ref(false)
const cur = computed(() => currentMarket())
const list = computed(() => locale.markets.map((c) => MARKET_BY_CODE.get(c)!).filter(Boolean))

function pick(code: string) {
  setMarket(code)
  open.value = false
}
</script>

<template>
  <div v-if="list.length > 1" class="relative">
    <button
      type="button"
      class="flex items-center gap-1.5 rounded-full px-2 py-1.5 text-sm font-semibold transition hover:bg-current/10"
      :aria-label="t('choose_market')"
      :title="t('choose_market')"
      @click.stop="open = !open"
    >
      <span class="text-base leading-none">{{ cur.flag }}</span>
      <span :class="compact ? 'hidden @xl:inline' : ''">{{ cur.currency }}</span>
      <ChevronDown class="h-3.5 w-3.5 opacity-70" />
    </button>
    <template v-if="open">
      <div class="fixed inset-0 z-30" @click.stop="open = false" />
      <div class="absolute right-0 top-full z-40 mt-2 max-h-[70vh] w-72 overflow-y-auto rounded-2xl border border-slate-200 bg-white py-1.5 text-left text-sm text-slate-700 shadow-xl">
        <div class="px-4 pb-1.5 pt-1 text-[11px] font-bold uppercase tracking-wide text-slate-400">{{ t('choose_market') }}</div>
        <button
          v-for="m in list"
          :key="m.code"
          type="button"
          class="flex w-full items-center gap-3 px-4 py-2 text-left hover:bg-slate-50"
          @click="pick(m.code)"
        >
          <span class="text-lg leading-none">{{ m.flag }}</span>
          <span class="min-w-0 flex-1">
            <span class="block truncate font-semibold text-slate-900">{{ m.country }}</span>
            <span class="block truncate text-xs text-slate-500">{{ LANG_NAMES[m.lang] }} · {{ m.currency }}</span>
          </span>
          <Check v-if="m.code === locale.market" class="h-4 w-4 shrink-0 text-emerald-600" />
        </button>
        <div class="mt-1 border-t border-slate-100 px-4 pb-1 pt-2 text-[10px] leading-snug text-slate-400">
          {{ t('rates_note') }} ·
          <a href="https://www.exchangerate-api.com" target="_blank" rel="noopener" class="underline">Rates by Exchange Rate API</a>
        </div>
      </div>
    </template>
  </div>
</template>
