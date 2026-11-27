<script setup lang="ts">
import { ref } from 'vue'
import { CheckCircle2, Loader2 } from 'lucide-vue-next'
import { api } from '../../lib/api'
import EditableText from './EditableText.vue'
import type { StoreBlock } from '../../types/store'

const props = defineProps<{ blk: StoreBlock; editable?: boolean; accent: string; siteSlug?: string; compact?: boolean }>()
const emit = defineEmits<{ (e: 'edit', path: (string | number)[], v: string): void }>()

const email = ref('')
const busy = ref(false)
const done = ref(false)
const err = ref('')

async function submit() {
  if (props.editable || !props.siteSlug) return
  err.value = ''
  if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email.value.trim())) {
    err.value = 'Email chưa đúng định dạng'
    return
  }
  busy.value = true
  try {
    await api.post('/storefront/public/subscribe', { slug: props.siteSlug, email: email.value.trim() })
    done.value = true
  } catch (e: any) {
    err.value = e?.response?.data?.error || 'Đăng ký chưa thành công, vui lòng thử lại'
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <div class="mx-auto max-w-2xl space-y-4 text-center">
    <EditableText tag="h2" :model-value="blk.data.heading" :editable="editable" placeholder="Tiêu đề" class="block text-2xl font-extrabold tracking-tight text-slate-900 @3xl:text-3xl" @update:model-value="emit('edit', ['heading'], $event)" />
    <EditableText tag="p" multiline :model-value="blk.data.text" :editable="editable" placeholder="Mô tả" class="block text-base text-slate-600" @update:model-value="emit('edit', ['text'], $event)" />
    <div v-if="done" class="inline-flex items-center gap-2 rounded-full bg-emerald-50 px-4 py-2.5 text-sm font-semibold text-emerald-700">
      <CheckCircle2 class="h-4 w-4" /> Cảm ơn bạn đã đăng ký!
    </div>
    <form v-else class="mx-auto flex max-w-md flex-col gap-2 pt-2 @xl:flex-row" @submit.prevent="submit">
      <input
        v-model="email"
        type="email"
        :placeholder="blk.data.placeholder || 'Email của bạn'"
        :readonly="editable"
        class="min-w-0 flex-1 rounded-full border border-slate-300 bg-white px-5 py-3 text-sm outline-none focus:border-slate-500"
      />
      <button type="submit" class="rounded-full px-6 py-3 text-sm font-bold text-white transition hover:brightness-110 disabled:opacity-60" :style="{ backgroundColor: accent }" :disabled="busy">
        <Loader2 v-if="busy" class="inline h-4 w-4 animate-spin" />
        <EditableText v-else :model-value="blk.data.button_label || (editable ? '' : 'Đăng ký')" :editable="editable" placeholder="Chữ trên nút" @update:model-value="emit('edit', ['button_label'], $event)" />
      </button>
    </form>
    <p v-if="err" class="text-xs font-semibold text-rose-600">{{ err }}</p>
  </div>
</template>
