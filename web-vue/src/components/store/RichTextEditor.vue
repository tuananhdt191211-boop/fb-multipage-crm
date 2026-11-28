<script setup lang="ts">
// Trình soạn mô tả sản phẩm kiểu Shopify / ShopBase: in đậm, nghiêng, tiêu đề, danh sách, link, chèn ảnh (tải lên / thư viện),
// dán từ web khác (được lọc sạch), chuyển sang xem / sửa mã HTML. Nội dung là HTML đã lọc (máy chủ lọc lại khi lưu).
import { nextTick, onMounted, ref, watch } from 'vue'
import { Bold, Code, Eraser, Heading2, Heading3, ImagePlus, Italic, Link2, List, ListOrdered, Minus, Pilcrow, Quote, Redo2, Underline, Undo2, Unlink } from 'lucide-vue-next'
import { sanitizeHtml, toRichHtml } from '../../lib/richHtml'
import { uploadImageFile } from '../../lib/productMedia'
import MediaLibrary from './MediaLibrary.vue'

const props = defineProps<{ modelValue: string; siteId: number | string; placeholder?: string }>()
const emit = defineEmits<{ (e: 'update:modelValue', v: string): void }>()

const el = ref<HTMLElement | null>(null)
const htmlMode = ref(false)
const rawHtml = ref('')
const showLib = ref(false)
const linkOpen = ref(false)
const linkUrl = ref('https://')
const busy = ref(0)
const errorMsg = ref('')
const empty = ref(true)
let lastEmitted = ''
let savedRange: Range | null = null

function setHtml(v: string) {
  if (!el.value) return
  el.value.innerHTML = toRichHtml(v || '')
  empty.value = !el.value.textContent?.trim() && !el.value.querySelector('img,hr')
}

onMounted(() => {
  try {
    document.execCommand('defaultParagraphSeparator', false, 'p') // Enter tạo đoạn <p>, không phải <div>
  } catch {
    /* trình duyệt cũ */
  }
  setHtml(props.modelValue)
  lastEmitted = props.modelValue
})

watch(
  () => props.modelValue,
  (v) => {
    if (v !== lastEmitted && !htmlMode.value) setHtml(v)
    if (v !== lastEmitted && htmlMode.value) rawHtml.value = v
  }
)

function commit() {
  if (!el.value) return
  const clean = sanitizeHtml(el.value.innerHTML)
  empty.value = !el.value.textContent?.trim() && !el.value.querySelector('img,hr')
  lastEmitted = empty.value ? '' : clean
  emit('update:modelValue', lastEmitted)
}

function focusEditor() {
  el.value?.focus()
}

function cmd(name: string, value?: string) {
  focusEditor()
  restoreSel()
  document.execCommand(name, false, value)
  commit()
}

function block(tag: 'p' | 'h2' | 'h3' | 'blockquote') {
  cmd('formatBlock', tag)
}

function saveSel() {
  const s = window.getSelection()
  if (s && s.rangeCount && el.value?.contains(s.anchorNode)) savedRange = s.getRangeAt(0).cloneRange()
}

function restoreSel() {
  if (!savedRange) return
  const s = window.getSelection()
  s?.removeAllRanges()
  s?.addRange(savedRange)
}

function openLink() {
  saveSel()
  linkUrl.value = 'https://'
  linkOpen.value = true
  nextTick(() => (document.getElementById('rte-link-input') as HTMLInputElement | null)?.select())
}

function applyLink() {
  const u = linkUrl.value.trim()
  linkOpen.value = false
  if (!u || u === 'https://') return
  cmd('createLink', /^(https?:|mailto:|tel:|\/|#)/i.test(u) ? u : 'https://' + u)
}

function openLibrary() {
  saveSel()
  showLib.value = true
}

function insertImage(url: string) {
  showLib.value = false
  cmd('insertImage', url)
}

async function insertUploaded(files: File[]) {
  errorMsg.value = ''
  saveSel()
  for (const f of files) {
    busy.value++
    try {
      const m = await uploadImageFile(props.siteId, f)
      insertImage(m.url)
    } catch (e: any) {
      errorMsg.value = `${f.name}: ${e?.response?.data?.error || e?.message || 'tải ảnh thất bại'}`
    } finally {
      busy.value--
    }
  }
}

function onPaste(e: ClipboardEvent) {
  const cd = e.clipboardData
  if (!cd) return
  const imgs = Array.from(cd.files || []).filter((f) => f.type.startsWith('image/'))
  e.preventDefault()
  if (imgs.length) {
    void insertUploaded(imgs)
    return
  }
  const html = cd.getData('text/html')
  if (html) document.execCommand('insertHTML', false, sanitizeHtml(html))
  else document.execCommand('insertText', false, cd.getData('text/plain'))
  commit()
}

function onDrop(e: DragEvent) {
  const imgs = Array.from(e.dataTransfer?.files || []).filter((f) => f.type.startsWith('image/'))
  if (!imgs.length) return
  e.preventDefault()
  void insertUploaded(imgs)
}

function toggleHtml() {
  if (!htmlMode.value) {
    commit()
    rawHtml.value = lastEmitted
    htmlMode.value = true
  } else {
    htmlMode.value = false
    const clean = sanitizeHtml(rawHtml.value)
    lastEmitted = clean
    emit('update:modelValue', clean)
    nextTick(() => setHtml(clean))
  }
}

function onRawInput() {
  lastEmitted = sanitizeHtml(rawHtml.value)
  emit('update:modelValue', lastEmitted)
}

const tools = [
  { key: 'bold', title: 'In đậm (Ctrl+B)', icon: Bold, run: () => cmd('bold') },
  { key: 'italic', title: 'In nghiêng (Ctrl+I)', icon: Italic, run: () => cmd('italic') },
  { key: 'underline', title: 'Gạch chân (Ctrl+U)', icon: Underline, run: () => cmd('underline') },
  { key: 'p', title: 'Đoạn văn thường', icon: Pilcrow, run: () => block('p') },
  { key: 'h2', title: 'Tiêu đề lớn', icon: Heading2, run: () => block('h2') },
  { key: 'h3', title: 'Tiêu đề nhỏ', icon: Heading3, run: () => block('h3') },
  { key: 'ul', title: 'Danh sách gạch đầu dòng', icon: List, run: () => cmd('insertUnorderedList') },
  { key: 'ol', title: 'Danh sách đánh số', icon: ListOrdered, run: () => cmd('insertOrderedList') },
  { key: 'quote', title: 'Trích dẫn', icon: Quote, run: () => block('blockquote') },
  { key: 'hr', title: 'Đường kẻ ngang', icon: Minus, run: () => cmd('insertHorizontalRule') },
  { key: 'unlink', title: 'Gỡ link', icon: Unlink, run: () => cmd('unlink') },
  { key: 'clear', title: 'Xoá định dạng', icon: Eraser, run: () => cmd('removeFormat') },
  { key: 'undo', title: 'Hoàn tác (Ctrl+Z)', icon: Undo2, run: () => cmd('undo') },
  { key: 'redo', title: 'Làm lại (Ctrl+Y)', icon: Redo2, run: () => cmd('redo') }
]
</script>

<template>
  <div class="overflow-hidden rounded-xl border border-slate-200 bg-white focus-within:border-emerald-400">
    <div class="flex flex-wrap items-center gap-0.5 border-b border-slate-100 bg-slate-50 px-1.5 py-1">
      <template v-if="!htmlMode">
        <button
          v-for="t in tools.slice(0, 3)"
          :key="t.key"
          type="button"
          class="rounded-lg p-1.5 text-slate-600 hover:bg-white hover:text-emerald-700"
          :title="t.title"
          @mousedown.prevent
          @click="t.run()"
        >
          <component :is="t.icon" class="h-4 w-4" />
        </button>
        <span class="mx-1 h-4 w-px bg-slate-200" />
        <button
          v-for="t in tools.slice(3, 9)"
          :key="t.key"
          type="button"
          class="rounded-lg p-1.5 text-slate-600 hover:bg-white hover:text-emerald-700"
          :title="t.title"
          @mousedown.prevent
          @click="t.run()"
        >
          <component :is="t.icon" class="h-4 w-4" />
        </button>
        <span class="mx-1 h-4 w-px bg-slate-200" />
        <button type="button" class="rounded-lg p-1.5 text-slate-600 hover:bg-white hover:text-emerald-700" title="Chèn link" @mousedown.prevent @click="openLink">
          <Link2 class="h-4 w-4" />
        </button>
        <button type="button" class="rounded-lg p-1.5 text-slate-600 hover:bg-white hover:text-emerald-700" title="Chèn ảnh" @mousedown.prevent @click="openLibrary">
          <ImagePlus class="h-4 w-4" />
        </button>
        <button
          v-for="t in tools.slice(9)"
          :key="t.key"
          type="button"
          class="rounded-lg p-1.5 text-slate-600 hover:bg-white hover:text-emerald-700"
          :title="t.title"
          @mousedown.prevent
          @click="t.run()"
        >
          <component :is="t.icon" class="h-4 w-4" />
        </button>
      </template>
      <span v-else class="px-2 text-[11px] font-semibold text-slate-500">Chế độ mã HTML — chỉ giữ lại các thẻ an toàn khi lưu</span>
      <button
        type="button"
        class="ml-auto inline-flex items-center gap-1 rounded-lg px-2 py-1 text-[11px] font-bold"
        :class="htmlMode ? 'bg-emerald-600 text-white' : 'text-slate-600 hover:bg-white hover:text-emerald-700'"
        title="Xem / sửa mã HTML"
        @mousedown.prevent
        @click="toggleHtml"
      >
        <Code class="h-3.5 w-3.5" /> HTML
      </button>
    </div>

    <div v-if="linkOpen" class="flex items-center gap-2 border-b border-slate-100 bg-amber-50/60 px-3 py-2">
      <input
        id="rte-link-input"
        v-model="linkUrl"
        class="min-w-0 flex-1 rounded-lg border border-slate-200 bg-white px-2.5 py-1.5 text-xs"
        placeholder="https://..."
        @keydown.enter.prevent="applyLink"
        @keydown.esc.prevent="linkOpen = false"
      />
      <button type="button" class="rounded-lg bg-emerald-600 px-3 py-1.5 text-xs font-bold text-white" @click="applyLink">Chèn link</button>
      <button type="button" class="rounded-lg px-2 py-1.5 text-xs font-semibold text-slate-500 hover:bg-white" @click="linkOpen = false">Huỷ</button>
    </div>

    <p v-if="errorMsg" class="border-b border-rose-100 bg-rose-50 px-3 py-1.5 text-xs font-semibold text-rose-600">{{ errorMsg }}</p>

    <div class="relative">
      <div
        v-show="!htmlMode"
        ref="el"
        contenteditable="true"
        spellcheck="false"
        class="pp-prose rte-body min-h-[220px] max-h-[520px] overflow-y-auto px-4 py-3 text-sm outline-none"
        @input="commit"
        @blur="commit"
        @paste="onPaste"
        @drop="onDrop"
        @mouseup="saveSel"
        @keyup="saveSel"
      />
      <p v-if="!htmlMode && empty" class="pointer-events-none absolute left-4 top-3 text-sm text-slate-400">{{ placeholder || 'Mô tả sản phẩm...' }}</p>
      <textarea
        v-if="htmlMode"
        v-model="rawHtml"
        spellcheck="false"
        class="block min-h-[220px] w-full resize-y border-0 bg-slate-50 px-4 py-3 font-mono text-xs outline-none"
        @input="onRawInput"
      />
      <div v-if="busy" class="absolute inset-x-0 bottom-0 bg-emerald-600/90 px-3 py-1 text-center text-[11px] font-bold text-white">Đang tải ảnh lên...</div>
    </div>

    <MediaLibrary v-if="showLib" :site-id="siteId" @close="showLib = false" @select="insertImage" />
  </div>
</template>
