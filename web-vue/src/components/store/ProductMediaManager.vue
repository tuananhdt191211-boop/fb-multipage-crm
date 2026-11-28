<script setup lang="ts">
// Bộ sưu tập ảnh / video của sản phẩm: tải ảnh lên, tải VIDEO lên (khi đã cấu hình kho R2/S3), dán link ảnh / YouTube / .mp4,
// chọn từ thư viện, kéo thả để đổi thứ tự (ảnh đầu tiên là ảnh bìa), tải ảnh link ngoài về thư viện của shop.
import { computed, ref, watch } from 'vue'
import { Download, Film, ImagePlus, Link2, Loader2, Play, Plus, Trash2, UploadCloud } from 'lucide-vue-next'
import {
  guessMediaKind,
  importImagesToLibrary,
  isOwnMedia,
  makeVideoPoster,
  uploadImageFile,
  uploadVideoFile,
  youtubePoster,
  type MediaConfig,
  type ProductMedia
} from '../../lib/productMedia'
import { safeImg } from '../../lib/safeUrl'
import MediaLibrary from './MediaLibrary.vue'

const MAX_ITEMS = 20

const props = defineProps<{ modelValue: ProductMedia[]; siteId: number | string; cfg: MediaConfig | null }>()
const emit = defineEmits<{ (e: 'update:modelValue', v: ProductMedia[]): void }>()

// Danh sách cục bộ: cập nhật ngay lập tức để nhiều thao tác liên tiếp (dán nhiều link, nhiều lượt tải xong cùng lúc)
// không ghi đè lên nhau trong lúc chờ component cha cập nhật lại prop.
const local = ref<ProductMedia[]>([...(props.modelValue || [])])
let lastEmitted: ProductMedia[] | null = null
watch(
  () => props.modelValue,
  (v) => {
    if (v !== lastEmitted) local.value = [...(v || [])] // thay đổi từ bên ngoài (mở sản phẩm khác, nhập dữ liệu...)
  }
)
const items = computed(() => local.value)
const imgInput = ref<HTMLInputElement | null>(null)
const vidInput = ref<HTMLInputElement | null>(null)
const showLib = ref(false)
const linkText = ref('')
const showLink = ref(false)
const msg = ref('')
const msgKind = ref<'error' | 'info'>('error')
const dragFrom = ref<number | null>(null)
const dragOver = ref<number | null>(null)
const fileDrag = ref(false)
const rehosting = ref(false)

interface Pending {
  id: number
  name: string
  pct: number
  kind: 'image' | 'video'
}
const pending = ref<Pending[]>([])
let pid = 0

const room = computed(() => MAX_ITEMS - items.value.length - pending.value.length)
const externalImages = computed(() => items.value.filter((m) => m.type === 'image' && !isOwnMedia(m.url, props.cfg)))
const canVideo = computed(() => !!props.cfg?.video_upload)

function setItems(list: ProductMedia[]) {
  local.value = list
  lastEmitted = list
  emit('update:modelValue', list)
}
function add(m: ProductMedia) {
  if (items.value.length >= MAX_ITEMS) return
  setItems([...items.value, m])
}
function say(text: string, kind: 'error' | 'info' = 'error') {
  msg.value = text
  msgKind.value = kind
}

async function uploadImages(files: File[]) {
  for (const f of files.slice(0, Math.max(0, room.value))) {
    const p: Pending = { id: ++pid, name: f.name, pct: 0, kind: 'image' }
    pending.value.push(p)
    try {
      const m = await uploadImageFile(props.siteId, f, (pct) => (p.pct = pct))
      add({ type: 'image', url: m.url })
    } catch (e: any) {
      say(`${f.name}: ${e?.response?.data?.error || e?.message || 'tải ảnh thất bại'}`)
    } finally {
      pending.value = pending.value.filter((x) => x.id !== p.id)
    }
  }
  if (files.length > room.value + files.length) say(`Tối đa ${MAX_ITEMS} ảnh / video cho mỗi sản phẩm.`)
}

async function uploadVideos(files: File[]) {
  const maxMb = props.cfg?.max_video_mb || 80
  for (const f of files.slice(0, Math.max(0, room.value))) {
    if (f.size > maxMb * 1024 * 1024) {
      say(`Video "${f.name}" (${(f.size / 1024 / 1024).toFixed(1)}MB) vượt quá giới hạn ${maxMb}MB.`)
      continue
    }
    const p: Pending = { id: ++pid, name: f.name, pct: 0, kind: 'video' }
    pending.value.push(p)
    try {
      const m = await uploadVideoFile(props.siteId, f, (pct) => (p.pct = pct))
      let poster = ''
      const pf = await makeVideoPoster(f)
      if (pf) {
        try {
          poster = (await uploadImageFile(props.siteId, pf)).url
        } catch {
          /* video vẫn dùng được khi không có ảnh bìa */
        }
      }
      add({ type: 'video', url: m.url, poster })
    } catch (e: any) {
      say(`${f.name}: ${e?.response?.data?.error || e?.message || 'tải video thất bại'}`)
    } finally {
      pending.value = pending.value.filter((x) => x.id !== p.id)
    }
  }
}

function onImagePick(e: Event) {
  const input = e.target as HTMLInputElement
  const files = Array.from(input.files || [])
  input.value = ''
  msg.value = ''
  if (files.length) void uploadImages(files)
}
function onVideoPick(e: Event) {
  const input = e.target as HTMLInputElement
  const files = Array.from(input.files || [])
  input.value = ''
  msg.value = ''
  if (files.length) void uploadVideos(files)
}

function onDropFiles(e: DragEvent) {
  fileDrag.value = false
  const files = Array.from(e.dataTransfer?.files || [])
  if (!files.length) return
  msg.value = ''
  const imgs = files.filter((f) => f.type.startsWith('image/'))
  const vids = files.filter((f) => f.type.startsWith('video/'))
  if (imgs.length) void uploadImages(imgs)
  if (vids.length) {
    if (canVideo.value) void uploadVideos(vids)
    else say('Chưa bật tải video lên (cần cấu hình kho lưu trữ R2/S3). Hãy dán link YouTube hoặc link .mp4.')
  }
}

function addLinks() {
  msg.value = ''
  const urls = linkText.value.split(/[\s,]+/).map((s) => s.trim()).filter(Boolean)
  let bad = 0
  for (const u of urls) {
    if (!/^https?:\/\//i.test(u)) {
      bad++
      continue
    }
    const kind = guessMediaKind(u)
    if (kind === 'image') add({ type: 'image', url: u })
    else add({ type: kind, url: u, poster: youtubePoster(u) })
  }
  linkText.value = ''
  if (bad) say(`${bad} link không hợp lệ (phải bắt đầu bằng http:// hoặc https://).`)
  else showLink.value = false
}

function pickFromLibrary(url: string, kind: 'image' | 'video' = 'image') {
  showLib.value = false
  add(kind === 'video' ? { type: 'video', url } : { type: 'image', url })
}

function remove(i: number) {
  setItems(items.value.filter((_, k) => k !== i))
}
function makeCover(i: number) {
  const list = [...items.value]
  const [m] = list.splice(i, 1)
  setItems([m!, ...list])
}

// Kéo thả đổi thứ tự (chỉ khi kéo chính các ô, không phải kéo file từ máy)
function onTileDrop(i: number) {
  const from = dragFrom.value
  dragFrom.value = null
  dragOver.value = null
  if (from === null || from === i) return
  const list = [...items.value]
  const [m] = list.splice(from, 1)
  list.splice(i, 0, m!)
  setItems(list)
}

async function rehost() {
  rehosting.value = true
  msg.value = ''
  try {
    const urls = externalImages.value.map((m) => m.url)
    const { map, failed } = await importImagesToLibrary(props.siteId, urls)
    setItems(items.value.map((m) => (m.type === 'image' && map[m.url] ? { ...m, url: map[m.url]! } : m)))
    const ok = Object.keys(map).length
    if (failed.length) say(`Đã tải ${ok} ảnh về thư viện, ${failed.length} ảnh không tải được (giữ nguyên link gốc).`, ok ? 'info' : 'error')
    else say(`Đã tải ${ok} ảnh về thư viện của bạn.`, 'info')
  } finally {
    rehosting.value = false
  }
}

function thumb(m: ProductMedia): string {
  return safeImg(m.type === 'image' ? m.url : m.poster)
}
</script>

<template>
  <div
    class="space-y-3"
    @dragover="(e) => { if (e.dataTransfer?.types.includes('Files')) { e.preventDefault(); fileDrag = true } }"
    @dragleave.self="fileDrag = false"
    @drop="(e) => { if (e.dataTransfer?.types.includes('Files')) { e.preventDefault(); onDropFiles(e) } }"
  >
    <div class="relative">
      <div
        v-if="fileDrag"
        class="pointer-events-none absolute inset-0 z-10 flex items-center justify-center rounded-2xl border-2 border-dashed border-emerald-500 bg-emerald-50/90 text-sm font-bold text-emerald-700"
      >
        Thả ảnh / video vào đây để tải lên
      </div>

      <div class="grid grid-cols-3 gap-2.5 sm:grid-cols-4 lg:grid-cols-5">
        <div
          v-for="(m, i) in items"
          :key="m.url + '#' + i"
          class="group relative aspect-square cursor-grab overflow-hidden rounded-xl border-2 bg-slate-100"
          :class="dragOver === i && dragFrom !== null && dragFrom !== i ? 'border-emerald-500' : 'border-slate-200'"
          draggable="true"
          @dragstart="dragFrom = i"
          @dragend="dragFrom = null; dragOver = null"
          @dragover.prevent="dragOver = i"
          @drop.prevent.stop="onTileDrop(i)"
        >
          <img v-if="thumb(m)" :src="thumb(m)" :alt="''" referrerpolicy="no-referrer" loading="lazy" class="h-full w-full object-cover" draggable="false" />
          <div v-else class="flex h-full w-full items-center justify-center text-slate-400"><Film class="h-7 w-7" /></div>

          <span v-if="i === 0" class="absolute left-1.5 top-1.5 rounded-md bg-emerald-600 px-1.5 py-0.5 text-[10px] font-bold text-white">Ảnh bìa</span>
          <span v-if="m.type !== 'image'" class="absolute bottom-1.5 left-1.5 inline-flex items-center gap-1 rounded-md bg-black/65 px-1.5 py-0.5 text-[10px] font-bold text-white">
            <Play class="h-3 w-3" /> {{ m.type === 'embed' ? 'Video nhúng' : 'Video' }}
          </span>

          <div class="absolute inset-x-0 top-0 flex justify-end gap-1 p-1.5 opacity-0 transition group-hover:opacity-100">
            <button
              v-if="i > 0 && m.type === 'image'"
              type="button"
              class="rounded-md bg-black/65 px-1.5 py-1 text-[10px] font-bold text-white hover:bg-emerald-600"
              title="Đặt làm ảnh bìa"
              @click="makeCover(i)"
            >
              Ảnh bìa
            </button>
            <button type="button" class="rounded-md bg-black/65 p-1 text-white hover:bg-rose-600" title="Xoá" @click="remove(i)">
              <Trash2 class="h-3.5 w-3.5" />
            </button>
          </div>
        </div>

        <div v-for="p in pending" :key="'p' + p.id" class="relative flex aspect-square flex-col items-center justify-center gap-1.5 rounded-xl border border-slate-200 bg-slate-50 p-2 text-center">
          <Loader2 class="h-5 w-5 animate-spin text-emerald-600" />
          <span class="text-[10px] font-semibold text-slate-500">{{ p.kind === 'video' ? 'Đang tải video' : 'Đang tải ảnh' }} {{ p.pct }}%</span>
          <div class="h-1 w-full overflow-hidden rounded-full bg-slate-200"><div class="h-full bg-emerald-500 transition-all" :style="{ width: p.pct + '%' }" /></div>
        </div>

        <button
          v-if="room > 0"
          type="button"
          class="flex aspect-square flex-col items-center justify-center gap-1 rounded-xl border-2 border-dashed border-slate-300 text-slate-500 hover:border-emerald-400 hover:bg-emerald-50/50 hover:text-emerald-700"
          @click="imgInput?.click()"
        >
          <Plus class="h-6 w-6" />
          <span class="text-[11px] font-bold">Thêm ảnh</span>
        </button>
      </div>
    </div>

    <input ref="imgInput" type="file" accept="image/jpeg,image/png,image/gif,image/webp" multiple class="hidden" @change="onImagePick" />
    <input ref="vidInput" type="file" accept="video/mp4,video/quicktime,video/webm,video/x-m4v,.mp4,.mov,.webm,.m4v" multiple class="hidden" @change="onVideoPick" />

    <div class="flex flex-wrap items-center gap-2">
      <button type="button" class="inline-flex items-center gap-1.5 rounded-lg border border-slate-200 px-3 py-1.5 text-xs font-bold text-slate-700 hover:border-emerald-400 hover:text-emerald-700" :disabled="room <= 0" @click="imgInput?.click()">
        <UploadCloud class="h-3.5 w-3.5" /> Tải ảnh lên
      </button>
      <button
        type="button"
        class="inline-flex items-center gap-1.5 rounded-lg border px-3 py-1.5 text-xs font-bold"
        :class="canVideo ? 'border-slate-200 text-slate-700 hover:border-emerald-400 hover:text-emerald-700' : 'cursor-not-allowed border-slate-100 text-slate-300'"
        :disabled="!canVideo || room <= 0"
        :title="canVideo ? 'Tải video MP4 / MOV / WEBM từ máy tính' : 'Cần cấu hình kho lưu trữ R2/S3 để tải video lên'"
        @click="vidInput?.click()"
      >
        <Film class="h-3.5 w-3.5" /> Tải video lên
      </button>
      <button type="button" class="inline-flex items-center gap-1.5 rounded-lg border border-slate-200 px-3 py-1.5 text-xs font-bold text-slate-700 hover:border-emerald-400 hover:text-emerald-700" @click="showLib = true">
        <ImagePlus class="h-3.5 w-3.5" /> Chọn từ thư viện
      </button>
      <button type="button" class="inline-flex items-center gap-1.5 rounded-lg border border-slate-200 px-3 py-1.5 text-xs font-bold text-slate-700 hover:border-emerald-400 hover:text-emerald-700" @click="showLink = !showLink">
        <Link2 class="h-3.5 w-3.5" /> Dán link ảnh / video
      </button>
      <span class="ml-auto text-[11px] font-semibold text-slate-400">{{ items.length }}/{{ MAX_ITEMS }}</span>
    </div>

    <div v-if="showLink" class="space-y-1.5 rounded-xl border border-slate-200 bg-slate-50 p-3">
      <textarea
        v-model="linkText"
        rows="2"
        class="w-full rounded-lg border border-slate-200 bg-white px-2.5 py-2 text-xs"
        placeholder="Dán một hoặc nhiều link (mỗi link một dòng): ảnh .jpg/.png/.webp, YouTube, Vimeo hoặc video .mp4"
      />
      <div class="flex items-center gap-2">
        <button type="button" class="rounded-lg bg-emerald-600 px-3 py-1.5 text-xs font-bold text-white" @click="addLinks">Thêm</button>
        <span class="text-[11px] text-slate-400">Link YouTube / Vimeo sẽ được nhúng, link .mp4 phát trực tiếp.</span>
      </div>
    </div>

    <p v-if="!canVideo" class="rounded-lg bg-amber-50 px-3 py-2 text-[11px] leading-relaxed text-amber-800">
      Tải video lên cần kho lưu trữ ngoài (Cloudflare R2 miễn phí 10GB hoặc S3) vì ổ đĩa của máy chủ bị xoá mỗi lần khởi động lại.
      Chưa cấu hình thì bạn vẫn thêm được video bằng link YouTube / Vimeo / .mp4. Cách bật: xem hướng dẫn trong tài liệu cài đặt (biến môi trường R2_*).
    </p>

    <button
      v-if="externalImages.length"
      type="button"
      class="inline-flex items-center gap-1.5 rounded-lg border border-emerald-200 bg-emerald-50 px-3 py-1.5 text-xs font-bold text-emerald-800 hover:bg-emerald-100 disabled:opacity-60"
      :disabled="rehosting"
      @click="rehost"
    >
      <Loader2 v-if="rehosting" class="h-3.5 w-3.5 animate-spin" />
      <Download v-else class="h-3.5 w-3.5" />
      Tải {{ externalImages.length }} ảnh từ link ngoài về thư viện của tôi
    </button>

    <p v-if="msg" class="rounded-lg px-3 py-2 text-xs font-semibold" :class="msgKind === 'error' ? 'bg-rose-50 text-rose-700' : 'bg-emerald-50 text-emerald-700'">{{ msg }}</p>
    <p class="text-[11px] text-slate-400">Ảnh đầu tiên là ảnh bìa. Kéo thả các ô để đổi thứ tự; kéo ảnh / video từ máy tính vào khung này để tải lên nhanh.</p>

    <MediaLibrary v-if="showLib" :site-id="siteId" allow-video @close="showLib = false" @select="pickFromLibrary" />
  </div>
</template>
