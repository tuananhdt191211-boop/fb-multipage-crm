<script setup lang="ts">
// Thư viện ảnh của website: tải lên (kéo thả hoặc chọn file), chọn lại ảnh cũ, xoá ảnh.
import { onMounted, ref } from 'vue'
import { Check, ImagePlus, Loader2, Trash2, UploadCloud, X } from 'lucide-vue-next'
import { api } from '../../lib/api'
import { prepareImage } from '../../lib/imageCompress'

export interface MediaItem {
  id: number
  url: string
  file_name: string
  size: number
  width: number
  height: number
}

const props = defineProps<{ siteId: number | string }>()
const emit = defineEmits<{ (e: 'close'): void; (e: 'select', url: string): void }>()

const items = ref<MediaItem[]>([])
const loading = ref(true)
const uploading = ref(0)
const errorMsg = ref('')
const dragOver = ref(false)
const picked = ref<number | null>(null)
const fileInput = ref<HTMLInputElement | null>(null)

async function load() {
  loading.value = true
  try {
    const { data } = await api.get<{ media: MediaItem[] }>(`/storefront/sites/${props.siteId}/media`)
    items.value = data.media || []
  } catch (e: any) {
    errorMsg.value = e?.response?.data?.error || 'Không tải được thư viện ảnh'
  } finally {
    loading.value = false
  }
}

async function uploadFiles(files: File[]) {
  errorMsg.value = ''
  for (const f of files) {
    uploading.value++
    try {
      const prepared = await prepareImage(f)
      const fd = new FormData()
      fd.append('file', prepared.file)
      fd.append('width', String(prepared.width))
      fd.append('height', String(prepared.height))
      const { data } = await api.post<{ media: MediaItem }>(`/storefront/sites/${props.siteId}/media`, fd, {
        headers: { 'Content-Type': 'multipart/form-data' },
        timeout: 120000
      })
      items.value.unshift(data.media)
      picked.value = data.media.id
    } catch (e: any) {
      errorMsg.value = `${f.name}: ${e?.response?.data?.error || e?.message || 'tải lên thất bại'}`
    } finally {
      uploading.value--
    }
  }
}

function onFileChange(e: Event) {
  const input = e.target as HTMLInputElement
  const files = Array.from(input.files || [])
  input.value = ''
  if (files.length) uploadFiles(files)
}

function onDrop(e: DragEvent) {
  dragOver.value = false
  const files = Array.from(e.dataTransfer?.files || []).filter((f) => f.type.startsWith('image/'))
  if (files.length) uploadFiles(files)
}

const confirmDelete = ref<number | null>(null)
async function remove(m: MediaItem) {
  if (confirmDelete.value !== m.id) {
    confirmDelete.value = m.id
    return
  }
  confirmDelete.value = null
  try {
    await api.delete(`/storefront/sites/${props.siteId}/media/${m.id}`)
    items.value = items.value.filter((x) => x.id !== m.id)
    if (picked.value === m.id) picked.value = null
  } catch (e: any) {
    errorMsg.value = e?.response?.data?.error || 'Xoá ảnh thất bại'
  }
}

function choose() {
  const m = items.value.find((x) => x.id === picked.value)
  if (m) emit('select', m.url)
}

function kb(n: number) {
  return n > 1024 * 1024 ? (n / 1024 / 1024).toFixed(1) + ' MB' : Math.round(n / 1024) + ' KB'
}

onMounted(load)
</script>

<template>
  <div class="fixed inset-0 z-[70] flex items-center justify-center bg-black/50 p-4" @click.self="emit('close')">
    <div
      class="flex max-h-[90vh] w-full max-w-4xl flex-col overflow-hidden rounded-2xl bg-white shadow-2xl"
      @dragover.prevent="dragOver = true"
      @dragleave.self="dragOver = false"
      @drop.prevent="onDrop"
    >
      <div class="flex items-center justify-between border-b border-slate-100 px-5 py-3">
        <h2 class="text-sm font-extrabold">Thư viện ảnh</h2>
        <div class="flex items-center gap-2">
          <button
            type="button"
            class="inline-flex items-center gap-1.5 rounded-xl bg-indigo-600 px-3 py-1.5 text-xs font-bold text-white hover:bg-indigo-700"
            @click="fileInput?.click()"
          >
            <UploadCloud class="h-3.5 w-3.5" /> Tải ảnh lên
          </button>
          <input ref="fileInput" type="file" accept="image/jpeg,image/png,image/gif,image/webp" multiple class="hidden" @change="onFileChange" />
          <button type="button" class="rounded-lg p-1.5 hover:bg-slate-100" @click="emit('close')"><X class="h-4 w-4" /></button>
        </div>
      </div>

      <p v-if="errorMsg" class="mx-5 mt-3 rounded-lg bg-rose-50 px-3 py-2 text-xs font-semibold text-rose-700">{{ errorMsg }}</p>

      <div class="relative min-h-[300px] flex-1 overflow-auto p-5">
        <div
          v-if="dragOver"
          class="pointer-events-none absolute inset-3 z-10 flex items-center justify-center rounded-2xl border-2 border-dashed border-indigo-500 bg-indigo-50/90 text-sm font-bold text-indigo-700"
        >
          Thả ảnh vào đây để tải lên
        </div>

        <div v-if="loading" class="flex items-center justify-center gap-2 py-20 text-sm text-slate-500">
          <Loader2 class="h-4 w-4 animate-spin" /> Đang tải...
        </div>

        <button
          v-else-if="!items.length && !uploading"
          type="button"
          class="flex w-full flex-col items-center justify-center gap-2 rounded-2xl border-2 border-dashed border-slate-200 py-20 text-sm text-slate-500 hover:border-indigo-300 hover:bg-indigo-50/40"
          @click="fileInput?.click()"
        >
          <ImagePlus class="h-8 w-8 text-slate-400" />
          <span class="font-semibold">Chưa có ảnh nào</span>
          <span class="text-xs">Bấm để chọn ảnh, hoặc kéo thả ảnh từ máy tính vào đây</span>
        </button>

        <div v-else class="grid grid-cols-2 gap-3 sm:grid-cols-4 lg:grid-cols-5">
          <div
            v-for="n in uploading"
            :key="'up' + n"
            class="flex aspect-square items-center justify-center rounded-xl border border-slate-200 bg-slate-50"
          >
            <Loader2 class="h-5 w-5 animate-spin text-indigo-500" />
          </div>
          <div
            v-for="m in items"
            :key="m.id"
            class="group relative cursor-pointer overflow-hidden rounded-xl border-2 bg-slate-50"
            :class="picked === m.id ? 'border-indigo-600' : 'border-transparent hover:border-slate-300'"
            @click="picked = m.id"
            @dblclick="emit('select', m.url)"
          >
            <img :src="m.url" :alt="m.file_name" loading="lazy" class="aspect-square w-full object-cover" />
            <div class="truncate px-2 py-1 text-[10px] text-slate-500">{{ m.width ? `${m.width}×${m.height} · ` : '' }}{{ kb(m.size) }}</div>
            <span v-if="picked === m.id" class="absolute left-1.5 top-1.5 rounded-full bg-indigo-600 p-0.5 text-white"><Check class="h-3.5 w-3.5" /></span>
            <button
              type="button"
              class="absolute right-1.5 top-1.5 rounded-lg px-1.5 py-1 text-[10px] font-bold text-white opacity-0 transition group-hover:opacity-100"
              :class="confirmDelete === m.id ? 'bg-rose-600 opacity-100' : 'bg-black/60 hover:bg-rose-600'"
              :title="confirmDelete === m.id ? 'Bấm lần nữa để xoá' : 'Xoá ảnh'"
              @click.stop="remove(m)"
            >
              <template v-if="confirmDelete === m.id">Xoá?</template>
              <Trash2 v-else class="h-3.5 w-3.5" />
            </button>
          </div>
        </div>
      </div>

      <div class="flex items-center justify-between gap-3 border-t border-slate-100 px-5 py-3">
        <span class="text-[11px] text-slate-400">Ảnh lớn tự được thu nhỏ còn tối đa 1920px. Bấm đúp vào ảnh để chọn nhanh.</span>
        <button
          type="button"
          class="rounded-xl bg-indigo-600 px-4 py-2 text-xs font-bold text-white hover:bg-indigo-700 disabled:opacity-40"
          :disabled="picked === null"
          @click="choose"
        >
          Dùng ảnh này
        </button>
      </div>
    </div>
  </div>
</template>
