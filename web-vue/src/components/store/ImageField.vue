<script setup lang="ts">
// Ô chọn ảnh: xem trước + mở thư viện / tải lên + dán link
import { ref } from 'vue'
import { ImagePlus, Link2, X } from 'lucide-vue-next'
import { safeImg } from '../../lib/safeUrl'
import MediaLibrary from './MediaLibrary.vue'

const props = defineProps<{ modelValue?: string; siteId: number | string; label?: string; compact?: boolean }>()
const emit = defineEmits<{ (e: 'update:modelValue', v: string): void }>()

const showLib = ref(false)
const showUrl = ref(false)

function pick(url: string) {
  emit('update:modelValue', url)
  showLib.value = false
}
</script>

<template>
  <div class="space-y-1.5">
    <span v-if="label" class="block text-xs font-bold text-slate-600">{{ label }}</span>
    <div class="flex items-stretch gap-2">
      <button
        type="button"
        class="group relative flex shrink-0 items-center justify-center overflow-hidden rounded-xl border border-slate-200 bg-slate-50 hover:border-indigo-400"
        :class="compact ? 'h-14 w-14' : 'h-20 w-28'"
        title="Chọn ảnh"
        @click="showLib = true"
      >
        <img v-if="safeImg(props.modelValue)" :src="safeImg(props.modelValue)" alt="" class="h-full w-full object-cover" />
        <ImagePlus v-else class="h-5 w-5 text-slate-400 group-hover:text-indigo-500" />
      </button>
      <div class="flex min-w-0 flex-1 flex-col justify-center gap-1.5">
        <div class="flex flex-wrap gap-1.5">
          <button type="button" class="rounded-lg border border-slate-200 px-2.5 py-1 text-xs font-bold text-slate-700 hover:border-indigo-400 hover:text-indigo-700" @click="showLib = true">
            {{ safeImg(props.modelValue) ? 'Đổi ảnh' : 'Chọn ảnh' }}
          </button>
          <button type="button" class="inline-flex items-center gap-1 rounded-lg border border-slate-200 px-2 py-1 text-xs text-slate-500 hover:text-slate-800" @click="showUrl = !showUrl">
            <Link2 class="h-3 w-3" /> Dán link
          </button>
          <button
            v-if="props.modelValue"
            type="button"
            class="inline-flex items-center gap-1 rounded-lg px-2 py-1 text-xs text-rose-600 hover:bg-rose-50"
            @click="emit('update:modelValue', '')"
          >
            <X class="h-3 w-3" /> Bỏ
          </button>
        </div>
        <input
          v-if="showUrl"
          :value="props.modelValue"
          placeholder="https://..."
          class="w-full rounded-lg border border-slate-200 px-2 py-1.5 text-xs"
          @input="emit('update:modelValue', ($event.target as HTMLInputElement).value)"
        />
      </div>
    </div>
    <MediaLibrary v-if="showLib" :site-id="siteId" @close="showLib = false" @select="pick" />
  </div>
</template>
