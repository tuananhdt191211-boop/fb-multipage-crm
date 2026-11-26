<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import {
  ArrowLeft,
  Bot,
  Calendar,
  Check,
  CheckCircle2,
  ClipboardPaste,
  Clock,
  Copy,
  Facebook,
  FileText,
  Image as ImageIcon,
  Layers,
  Lightbulb,
  Link2,
  Loader2,
  Play,
  RotateCcw,
  Send,
  Sparkles,
  Trash2,
  Upload,
  Video
} from 'lucide-vue-next'
import { api } from '../lib/api'
import type { FacebookPageItem } from '../types/page'

interface ScheduledPostItem {
  id: number
  content: string
  media_type: 'none' | 'image' | 'video'
  media_url: string
  image_urls?: string[]
  page_ids_json: string
  page_names_json: string
  scheduled_at: string
  status: 'pending' | 'processing' | 'published' | 'partial' | 'failed'
  success_count: number
  failed_count: number
  publish_results: string
}

interface AIChatTurn {
  id: string
  role: 'user' | 'assistant'
  text: string
  draftContent?: string
}

interface AIContentSuggestion {
  angle: string
  content: string
}

const router = useRouter()

const pages = ref<FacebookPageItem[]>([])
const posts = ref<ScheduledPostItem[]>([])
const selectedPageIDs = ref<string[]>([])
const pageSearch = ref('')

const content = ref('')
const mediaType = ref<'none' | 'image' | 'video'>('none')
const mediaURL = ref('')
const mediaPreviewURL = ref('')
const mediaGallery = ref<string[]>([])
const scheduledDateTime = ref('')
const isDraggingMedia = ref(false)
const uploadingMedia = ref(false)
const copiedIndex = ref<number | null>(null)
const copiedSuggestionIndex = ref<number | null>(null)

const loading = ref(false)
const submitting = ref(false)
const publishingID = ref<number | null>(null)
const noticeMsg = ref('')
const errorMsg = ref('')

// AI Content Studio — 100% gọi AI thật, không fix cứng mẫu
const showAIEditor = ref(true)
const showFormBox = ref(true)
const aiProvider = ref<'gemini' | 'openai'>('gemini')
const suggestionCount = ref<10 | 15 | 20>(10)
const userInsight = ref('')
const referenceForm = ref('')
const aiPromptInput = ref('')
const aiLoading = ref(false)
const aiError = ref('')
const activeConditions = ref<string[]>([])
const aiSuggestions = ref<AIContentSuggestion[]>([])
const aiChatTurns = ref<AIChatTurn[]>([
  {
    id: 'welcome',
    role: 'assistant',
    text: 'Dán bất kỳ content mẫu, sản phẩm, insight hoặc câu hỏi nào rồi bấm Sinh bài — AI sẽ tự suy nghĩ văn và đề xuất 10–20 bài mới hoàn toàn cho bạn.'
  }
])

const aiConditionOptions = [
  { id: 'follow_form', label: 'Đúng form mẫu' },
  { id: 'deep_insight', label: 'Khai thác sâu Insight' },
  { id: 'strong_cta', label: 'Kêu gọi chốt đơn' },
  { id: 'add_hashtags', label: 'Đủ Hashtag' }
]

const goldenHours = [
  { label: '08:00 Sáng', hour: 8, minute: 0 },
  { label: '11:30 Trưa', hour: 11, minute: 30 },
  { label: '19:30 Tối', hour: 19, minute: 30 },
  { label: '21:00 Tối', hour: 21, minute: 0 }
]

const filteredPages = computed(() => {
  const q = pageSearch.value.trim().toLowerCase()
  if (!q) return pages.value
  return pages.value.filter((p) => p.name.toLowerCase().includes(q))
})

const isAllSelected = computed(
  () => pages.value.length > 0 && selectedPageIDs.value.length === pages.value.length
)

function setDefaultScheduleTime() {
  const d = new Date(Date.now() + 30 * 60 * 1000)
  scheduledDateTime.value = toLocalDateTimeInput(d)
}

function toLocalDateTimeInput(d: Date) {
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`
}

function applyGoldenHour(hour: number, minute: number) {
  const now = new Date()
  const target = new Date(now.getFullYear(), now.getMonth(), now.getDate(), hour, minute, 0)
  if (target.getTime() <= now.getTime()) {
    target.setDate(target.getDate() + 1)
  }
  scheduledDateTime.value = toLocalDateTimeInput(target)
}

function toggleAllPages() {
  if (isAllSelected.value) {
    selectedPageIDs.value = []
  } else {
    selectedPageIDs.value = pages.value.map((p) => p.page_id)
  }
}

function toggleSinglePage(pageID: string) {
  const idx = selectedPageIDs.value.indexOf(pageID)
  if (idx === -1) {
    selectedPageIDs.value.push(pageID)
  } else {
    selectedPageIDs.value.splice(idx, 1)
  }
}

function selectMediaType(nextType: 'none' | 'image' | 'video') {
  mediaType.value = nextType
  if (nextType === 'none') {
    mediaURL.value = ''
    mediaPreviewURL.value = ''
    mediaGallery.value = []
  } else if (nextType === 'video') {
    mediaPreviewURL.value = ''
    mediaGallery.value = []
  }
}

function applyImageLink(urlValue: string) {
  const clean = urlValue.trim()
  if (!clean) return
  mediaType.value = 'image'
  mediaURL.value = clean
  mediaPreviewURL.value = clean
  if (!mediaGallery.value.includes(clean)) {
    mediaGallery.value.push(clean)
  }
}

function removeGalleryImage(idx: number) {
  mediaGallery.value.splice(idx, 1)
  if (mediaGallery.value.length === 0) {
    mediaURL.value = ''
    mediaPreviewURL.value = ''
    mediaType.value = 'none'
  } else {
    const last = mediaGallery.value[mediaGallery.value.length - 1]
    mediaURL.value = last
    mediaPreviewURL.value = last
  }
}

async function copyImageToClipboard(imgSrc: string, idx: number) {
  errorMsg.value = ''
  try {
    if (navigator.clipboard && 'write' in navigator.clipboard && typeof ClipboardItem !== 'undefined') {
      const res = await fetch(imgSrc, { mode: 'cors' })
      const blob = await res.blob()
      const mime = blob.type && blob.type.startsWith('image/') ? blob.type : 'image/png'
      let pngBlob = blob
      if (mime !== 'image/png') {
        const bitmap = await createImageBitmap(blob)
        const canvas = document.createElement('canvas')
        canvas.width = bitmap.width
        canvas.height = bitmap.height
        const ctx = canvas.getContext('2d')
        ctx?.drawImage(bitmap, 0, 0)
        pngBlob = await new Promise<Blob>((resolve, reject) => {
          canvas.toBlob((b) => (b ? resolve(b) : reject(new Error('Canvas error'))), 'image/png')
        })
      }

      await navigator.clipboard.write([new ClipboardItem({ 'image/png': pngBlob })])
      copiedIndex.value = idx
      noticeMsg.value = 'Đã sao chép ảnh vào clipboard!'
      setTimeout(() => {
        if (copiedIndex.value === idx) copiedIndex.value = null
        if (noticeMsg.value.startsWith('Đã sao chép ảnh')) noticeMsg.value = ''
      }, 2500)
      return
    }
  } catch {
    // Fallback sao chép đường dẫn/dữ liệu ảnh
  }

  try {
    await navigator.clipboard?.writeText(imgSrc)
    copiedIndex.value = idx
    noticeMsg.value = 'Đã sao chép đường dẫn/dữ liệu ảnh!'
    setTimeout(() => {
      if (copiedIndex.value === idx) copiedIndex.value = null
      if (noticeMsg.value.startsWith('Đã sao chép')) noticeMsg.value = ''
    }, 2000)
  } catch {
    errorMsg.value = 'Trình duyệt không hỗ trợ sao chép ảnh trực tiếp.'
  }
}

async function processImageFiles(files: File[]) {
  const imgs = files.filter((f) => f.type.startsWith('image/'))
  if (imgs.length === 0) return

  uploadingMedia.value = true
  errorMsg.value = ''
  mediaType.value = 'image'

  try {
    const fd = new FormData()
    for (const file of imgs) {
      fd.append('images', file)
    }

    const { data } = await api.post<{ urls?: string[]; url?: string }>('/posts/upload-images', fd, {
      headers: { 'Content-Type': 'multipart/form-data' }
    })

    const uploaded = data?.urls && data.urls.length > 0 ? data.urls : data?.url ? [data.url] : []
    if (uploaded.length > 0) {
      for (const u of uploaded) {
        if (!mediaGallery.value.includes(u)) mediaGallery.value.push(u)
      }
      mediaURL.value = uploaded[uploaded.length - 1]
      mediaPreviewURL.value = uploaded[uploaded.length - 1]
      return
    }
  } catch {
    // Dự phòng đọc ảnh cục bộ
  } finally {
    uploadingMedia.value = false
  }

  for (const file of imgs) {
    const dataUrl = await new Promise<string>((resolve) => {
      const reader = new FileReader()
      reader.onload = () => resolve(typeof reader.result === 'string' ? reader.result : '')
      reader.readAsDataURL(file)
    })
    if (dataUrl) {
      mediaGallery.value.push(dataUrl)
      mediaURL.value = dataUrl
      mediaPreviewURL.value = dataUrl
    }
  }
}

async function handleFileUpload(e: Event) {
  const input = e.target as HTMLInputElement
  if (input.files && input.files.length > 0) {
    await processImageFiles(Array.from(input.files))
    input.value = ''
  }
}

async function handleDropFiles(e: DragEvent) {
  isDraggingMedia.value = false
  const dropped = Array.from(e.dataTransfer?.files || [])
  if (dropped.length > 0) {
    await processImageFiles(dropped)
    return
  }

  const draggedUri = e.dataTransfer?.getData('text/uri-list') || e.dataTransfer?.getData('text/plain') || ''
  if (/^https?:\/\/.+/i.test(draggedUri.trim())) {
    applyImageLink(draggedUri.trim())
  }
}

async function handlePasteOnComposer(e: ClipboardEvent) {
  const items = Array.from(e.clipboardData?.items || [])
  const pastedFiles: File[] = []

  for (const item of items) {
    if (item.kind === 'file' && item.type.startsWith('image/')) {
      const blob = item.getAsFile()
      if (blob) {
        const ext = item.type.split('/')[1] || 'png'
        pastedFiles.push(new File([blob], `paste_${Date.now()}.${ext}`, { type: item.type }))
      }
    }
  }

  if (pastedFiles.length > 0) {
    e.preventDefault()
    await processImageFiles(pastedFiles)
    return
  }

  const text = e.clipboardData?.getData('text/plain')?.trim() || ''
  if (/^https?:\/\/.+\.(jpg|jpeg|png|gif|webp)(\?.*)?$/i.test(text)) {
    e.preventDefault()
    applyImageLink(text)
  }
}

async function handleQuickClipboardAction() {
  try {
    if (navigator.clipboard && 'read' in navigator.clipboard) {
      const items = await navigator.clipboard.read()
      const files: File[] = []
      for (const item of items) {
        const imgMime = item.types.find((t) => t.startsWith('image/'))
        if (imgMime) {
          const blob = await item.getType(imgMime)
          const ext = imgMime.split('/')[1] || 'png'
          files.push(new File([blob], `clipboard_${Date.now()}.${ext}`, { type: imgMime }))
        }
      }
      if (files.length > 0) {
        await processImageFiles(files)
        return
      }
    }

    const txt = await navigator.clipboard?.readText()
    if (txt?.trim()) {
      const clean = txt.trim()
      if (/^https?:\/\/.+\.(jpg|jpeg|png|gif|webp)(\?.*)?$/i.test(clean)) {
        applyImageLink(clean)
      } else {
        content.value = content.value ? `${content.value}\n${clean}` : clean
      }
    }
  } catch {
    errorMsg.value = 'Hãy nhấp vào khung soạn thảo và bấm Ctrl+V để dán ảnh hoặc nội dung.'
  }
}

function toggleCondition(id: string) {
  const idx = activeConditions.value.indexOf(id)
  if (idx === -1) activeConditions.value.push(id)
  else activeConditions.value.splice(idx, 1)
}

function applyDraftToEditor(draft: string) {
  if (!draft.trim()) return
  content.value = draft.trim()
  noticeMsg.value = 'Đã đưa bài content sang khung soạn thảo đăng bài!'
  setTimeout(() => {
    if (noticeMsg.value.startsWith('Đã đưa bài content')) noticeMsg.value = ''
  }, 2200)
}

async function copySuggestionText(text: string, idx: number) {
  try {
    await navigator.clipboard?.writeText(text)
    copiedSuggestionIndex.value = idx
    setTimeout(() => {
      if (copiedSuggestionIndex.value === idx) copiedSuggestionIndex.value = null
    }, 1800)
  } catch {
    // Bỏ qua nếu trình duyệt chặn clipboard
  }
}

function resetAIChat() {
  aiSuggestions.value = []
  aiError.value = ''
  aiChatTurns.value = [
    {
      id: 'reset_' + Date.now(),
      role: 'assistant',
      text: 'Đã làm mới danh sách đề xuất. Nhập bất kỳ content, insight hoặc yêu cầu nào để AI tự viết.'
    }
  ]
}

async function handleSendAIPrompt() {
  const promptText = aiPromptInput.value.trim()
  const effectiveForm = referenceForm.value.trim() || content.value.trim()

  if (!promptText && !effectiveForm && !userInsight.value.trim()) {
    aiError.value = 'Hãy nhập bất kỳ content, sản phẩm, insight hoặc câu hỏi nào để AI tự viết cho bạn.'
    return
  }

  if (!referenceForm.value.trim() && content.value.trim()) {
    referenceForm.value = content.value.trim()
  }

  aiError.value = ''
  aiLoading.value = true

  const userBubbleText =
    promptText ||
    `Tự suy nghĩ và viết ${suggestionCount.value} content mới theo nội dung/form đã đưa với các góc insight khác nhau`

  const apiInstruction =
    promptText ||
    `Hãy tự phân tích nội dung/sản phẩm người dùng đưa vào và sáng tạo ${suggestionCount.value} bài content hoàn chỉnh với ${suggestionCount.value} góc tiếp cận / insight khách hàng khác nhau. Viết văn tự nhiên, cuốn hút, đa dạng câu từ và cuối mỗi bài có 5–6 hashtag riêng.`

  aiChatTurns.value.push({
    id: 'u_' + Date.now(),
    role: 'user',
    text: userBubbleText
  })

  const historyPayload = aiChatTurns.value.slice(-8).map((m) => ({
    role: m.role,
    text: m.text
  }))

  try {
    const { data } = await api.post<{
      content?: string
      revised_content?: string
      reply?: string
      suggestions?: AIContentSuggestion[]
      model?: string
    }>('/posts/ai-rewrite', {
      provider: aiProvider.value,
      content: content.value.trim(),
      reference_form: effectiveForm,
      insight: userInsight.value.trim(),
      count: suggestionCount.value,
      prompt: apiInstruction,
      conditions: activeConditions.value,
      history: historyPayload
    })

    const list = Array.isArray(data?.suggestions) && data.suggestions.length > 0 ? data.suggestions : []
    const bestDraft = (data?.revised_content || data?.content || list[0]?.content || '').trim()

    if (list.length > 0 || bestDraft) {
      aiSuggestions.value = list.length > 0 ? list : [{ angle: 'Đề xuất AI', content: bestDraft }]
      if (bestDraft) {
        content.value = bestDraft
      }
      const modelTag = data?.model ? ` (${data.model})` : ''
      aiChatTurns.value.push({
        id: 'a_' + Date.now(),
        role: 'assistant',
        text: `${data?.reply || `Đã tự viết ${aiSuggestions.value.length} bài content mới theo yêu cầu.`}${modelTag}`,
        draftContent: bestDraft || undefined
      })
      aiPromptInput.value = ''
    } else {
      aiError.value = 'AI không trả về nội dung, vui lòng thử gửi lại yêu cầu.'
    }
  } catch (err: any) {
    const status = err?.response?.status
    const msg =
      status === 405 || status === 404
        ? `Backend chưa cập nhật route POST /api/v1/posts/ai-rewrite (HTTP ${status}). Hãy deploy bản main.go mới lên server.`
        : err?.response?.data?.error ||
          'Không kết nối được tới API AI trên server. Hãy kiểm tra biến AI_API_KEY.'
    aiError.value = msg
    aiChatTurns.value.push({
      id: 'err_' + Date.now(),
      role: 'assistant',
      text: `Lỗi kết nối AI: ${msg}`
    })
  } finally {
    aiLoading.value = false
  }
}

function parsePageNames(raw?: string): string[] {
  if (!raw) return []
  try {
    const arr = JSON.parse(raw)
    return Array.isArray(arr) ? arr : []
  } catch {
    return []
  }
}

function parseResults(raw?: string): Array<{ page_name: string; success: boolean; error?: string }> {
  if (!raw) return []
  try {
    const arr = JSON.parse(raw)
    return Array.isArray(arr) ? arr : []
  } catch {
    return []
  }
}

function formatDateTime(iso: string) {
  if (!iso) return ''
  return new Date(iso).toLocaleString('vi-VN', {
    day: '2-digit',
    month: '2-digit',
    year: 'numeric',
    hour: '2-digit',
    minute: '2-digit'
  })
}

async function loadData() {
  loading.value = true
  errorMsg.value = ''
  try {
    const { data } = await api.get<{ posts: ScheduledPostItem[]; pages: FacebookPageItem[] }>('/posts')
    posts.value = data.posts || []
    pages.value = data.pages || []
  } catch (err: any) {
    errorMsg.value = err.response?.data?.error || 'Không thể tải danh sách lịch đăng bài.'
  } finally {
    loading.value = false
  }
}

async function handleSubmit(publishNow: boolean) {
  if ((!content.value.trim() && mediaGallery.length === 0 && !mediaURL.value.trim()) || selectedPageIDs.value.length === 0 || submitting.value) {
    return
  }
  submitting.value = true
  errorMsg.value = ''
  noticeMsg.value = ''

  try {
    const isoSchedule = scheduledDateTime.value
      ? new Date(scheduledDateTime.value).toISOString()
      : new Date().toISOString()

    const primaryMediaURL =
      mediaType.value === 'image'
        ? (mediaGallery.value[0] || mediaURL.value.trim())
        : mediaType.value === 'video'
          ? mediaURL.value.trim()
          : ''

    const { data } = await api.post<{ message: string }>('/posts', {
      content: content.value.trim(),
      media_type: mediaType.value,
      media_url: primaryMediaURL,
      image_urls: mediaType.value === 'image' ? mediaGallery.value : [],
      page_ids: selectedPageIDs.value,
      scheduled_at: isoSchedule,
      publish_now: publishNow
    })

    noticeMsg.value = data.message
    content.value = ''
    mediaURL.value = ''
    mediaPreviewURL.value = ''
    mediaGallery.value = []
    mediaType.value = 'none'
    await loadData()
    setTimeout(() => {
      noticeMsg.value = ''
    }, 4000)
  } catch (err: any) {
    errorMsg.value = err.response?.data?.error || 'Tạo bài đăng thất bại.'
  } finally {
    submitting.value = false
  }
}

async function handlePublishNow(postID: number) {
  publishingID.value = postID
  errorMsg.value = ''
  try {
    const { data } = await api.post<{ message: string }>(`/posts/${postID}/publish-now`)
    noticeMsg.value = data.message
    await loadData()
    setTimeout(() => {
      noticeMsg.value = ''
    }, 4000)
  } catch (err: any) {
    errorMsg.value = err.response?.data?.error || 'Đăng bài ngay thất bại.'
  } finally {
    publishingID.value = null
  }
}

async function handleDelete(postID: number) {
  try {
    await api.delete(`/posts/${postID}`)
    posts.value = posts.value.filter((p) => p.id !== postID)
  } catch (err: any) {
    errorMsg.value = err.response?.data?.error || 'Xóa bài viết thất bại.'
  }
}

onMounted(() => {
  setDefaultScheduleTime()
  const drafted = localStorage.getItem('draft_scheduled_post_content')
  if (drafted) {
    content.value = drafted
    localStorage.removeItem('draft_scheduled_post_content')
  }
  loadData()
})
</script>

<template>
  <div translate="no" class="notranslate min-h-screen bg-[#f4f7f6] text-slate-900">
    <!-- Top Header -->
    <header class="sticky top-0 z-20 flex min-h-16 flex-wrap items-center justify-between gap-2 border-b border-slate-200/80 bg-white/95 px-4 py-2 sm:px-6 backdrop-blur-xs">
      <div class="flex min-w-0 items-center gap-3">
        <button
          type="button"
          @click="router.push('/')"
          class="inline-flex shrink-0 items-center gap-1.5 rounded-xl border border-slate-200 bg-white px-3 py-1.5 text-xs font-semibold text-slate-700 transition hover:border-emerald-300 hover:bg-emerald-50 hover:text-emerald-800 cursor-pointer"
        >
          <ArrowLeft class="h-3.5 w-3.5" />
          <span>Trang chủ</span>
        </button>

        <div class="h-5 w-px bg-slate-200" />

        <div class="flex min-w-0 items-center gap-2.5">
          <div class="flex h-8 w-8 shrink-0 items-center justify-center rounded-xl bg-emerald-600 text-white">
            <Calendar class="h-4 w-4" />
          </div>
          <h1 class="truncate text-sm sm:text-base font-extrabold tracking-tight !text-slate-900">
            Lên lịch đăng bài đa Fanpage
          </h1>
        </div>
      </div>

      <div class="flex shrink-0 items-center gap-2">
        <button
          type="button"
          @click="showAIEditor = !showAIEditor"
          class="inline-flex items-center gap-1.5 rounded-xl border border-emerald-200 bg-emerald-50 px-3 py-1.5 text-xs font-bold text-emerald-800 transition hover:bg-emerald-100 cursor-pointer"
        >
          <Sparkles class="h-3.5 w-3.5 text-emerald-600" />
          <span class="hidden sm:inline">{{ showAIEditor ? 'Ẩn AI Content' : 'AI Content' }}</span>
        </button>

        <button
          type="button"
          @click="router.push('/inbox')"
          class="inline-flex items-center gap-1.5 rounded-xl border border-slate-200 bg-white px-3 py-1.5 text-xs font-semibold text-slate-700 transition hover:bg-slate-50 cursor-pointer"
        >
          <Layers class="h-3.5 w-3.5 text-emerald-600" />
          <span>Mở Hộp thư gộp</span>
        </button>
      </div>
    </header>

    <!-- Toast -->
    <div
      v-if="noticeMsg"
      class="flex items-center justify-center gap-2 border-b border-emerald-200 bg-emerald-50 px-4 py-2 text-xs font-semibold text-emerald-800"
    >
      <CheckCircle2 class="h-4 w-4 text-emerald-600" />
      <span>{{ noticeMsg }}</span>
    </div>
    <div
      v-if="errorMsg"
      class="border-b border-rose-200 bg-rose-50 px-4 py-2 text-center text-xs font-semibold text-rose-700"
    >
      {{ errorMsg }}
    </div>

    <!-- Main Content -->
    <main class="mx-auto grid max-w-7xl grid-cols-12 gap-6 px-4 py-6 sm:px-6 lg:px-8 items-start">
      <!-- CỘT TRÁI -->
      <section class="col-span-12 space-y-5 lg:col-span-7">
        <div
          @dragover.prevent="isDraggingMedia = true"
          @dragleave.prevent="isDraggingMedia = false"
          @drop.prevent="handleDropFiles"
          class="rounded-2xl border border-slate-200/80 bg-white p-5 sm:p-6 text-left shadow-2xs space-y-5 transition"
          :class="isDraggingMedia ? 'ring-2 ring-emerald-500 border-emerald-400 bg-emerald-50/20' : ''"
        >
          <div class="flex items-center justify-between gap-2">
            <h2 class="text-sm font-extrabold !text-slate-900">1. Soạn nội dung bài viết</h2>
            <button
              type="button"
              @click="handleQuickClipboardAction"
              class="inline-flex items-center gap-1.5 rounded-xl border border-slate-200 bg-slate-50 px-3 py-1.5 text-[11px] font-bold text-slate-700 transition hover:border-emerald-300 hover:bg-emerald-50 hover:text-emerald-800 cursor-pointer"
            >
              <ClipboardPaste class="h-3.5 w-3.5 text-emerald-600" />
              <span>Dán ảnh / Text (Ctrl+V)</span>
            </button>
          </div>

          <textarea
            v-model="content"
            @paste="handlePasteOnComposer"
            rows="9"
            placeholder="Nhập nội dung bài viết hoặc dán bài mẫu vào đây rồi bấm Sinh bài bên phải... Nhấn Ctrl+V để dán ảnh vừa copy hoặc kéo ảnh từ máy tính vào đây."
            class="w-full rounded-xl border border-slate-200 bg-slate-50/50 p-3.5 text-sm leading-relaxed !text-slate-900 placeholder:text-slate-400 outline-none transition focus:border-emerald-600 focus:bg-white focus:ring-2 focus:ring-emerald-100"
          />

          <!-- Chọn loại đính kèm -->
          <div class="space-y-3">
            <div class="flex flex-wrap items-center gap-2">
              <button
                type="button"
                @click="selectMediaType('none')"
                class="rounded-xl border px-3 py-1.5 text-xs font-semibold transition cursor-pointer"
                :class="
                  mediaType === 'none'
                    ? 'border-emerald-600 bg-emerald-50 text-emerald-800 font-bold'
                    : 'border-slate-200 text-slate-600 hover:bg-slate-50'
                "
              >
                Chỉ văn bản
              </button>

              <button
                type="button"
                @click="selectMediaType('image')"
                class="inline-flex items-center gap-1.5 rounded-xl border px-3 py-1.5 text-xs font-semibold transition cursor-pointer"
                :class="
                  mediaType === 'image'
                    ? 'border-emerald-600 bg-emerald-50 text-emerald-800 font-bold'
                    : 'border-slate-200 text-slate-600 hover:bg-slate-50'
                "
              >
                <ImageIcon class="h-3.5 w-3.5" />
                <span>Ảnh (Dán / Kéo thả / Tải / Link)</span>
                <span
                  v-if="mediaGallery.length > 0"
                  class="rounded-full bg-emerald-600 px-1.5 text-[10px] font-bold text-white"
                >
                  {{ mediaGallery.length }}
                </span>
              </button>

              <button
                type="button"
                @click="selectMediaType('video')"
                class="inline-flex items-center gap-1.5 rounded-xl border px-3 py-1.5 text-xs font-semibold transition cursor-pointer"
                :class="
                  mediaType === 'video'
                    ? 'border-emerald-600 bg-emerald-50 text-emerald-800 font-bold'
                    : 'border-slate-200 text-slate-600 hover:bg-slate-50'
                "
              >
                <Video class="h-3.5 w-3.5" />
                <span>Đính kèm Video (URL)</span>
              </button>
            </div>

            <!-- Khu vực Ảnh -->
            <div
              v-if="mediaType === 'image'"
              class="space-y-3 rounded-2xl border-2 border-dashed bg-emerald-50/30 p-4 transition"
              :class="isDraggingMedia ? 'border-emerald-600 bg-emerald-50/80' : 'border-emerald-200/90'"
            >
              <div class="flex flex-wrap items-center justify-between gap-2">
                <label class="inline-flex items-center gap-1.5 rounded-xl bg-emerald-600 px-3.5 py-2 text-xs font-bold text-white transition hover:bg-emerald-700 cursor-pointer">
                  <Loader2 v-if="uploadingMedia" class="h-3.5 w-3.5 animate-spin" />
                  <Upload v-else class="h-3.5 w-3.5" />
                  <span>{{ uploadingMedia ? 'Đang tải...' : 'Tải ảnh từ máy' }}</span>
                  <input
                    type="file"
                    accept="image/*"
                    multiple
                    class="hidden"
                    @change="handleFileUpload"
                  />
                </label>

                <span class="text-[11px] text-emerald-800">
                  <strong>Kéo ảnh từ máy tính</strong> vào đây, bấm <strong>Ctrl+V</strong> dán ảnh copy hoặc dán link bên dưới.
                </span>
              </div>

              <div class="flex gap-2">
                <div class="relative flex-1">
                  <Link2 class="pointer-events-none absolute left-3 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-slate-400" />
                  <input
                    v-model="mediaURL"
                    @keydown.enter.prevent="applyImageLink(mediaURL)"
                    type="url"
                    placeholder="Dán đường dẫn ảnh (https://...jpg/png/webp)"
                    class="w-full rounded-xl border border-slate-200 bg-white py-2 pl-9 pr-3 text-xs !text-slate-900 placeholder:text-slate-400 outline-none focus:border-emerald-600"
                  />
                </div>
                <button
                  type="button"
                  @click="applyImageLink(mediaURL)"
                  class="rounded-xl border border-emerald-300 bg-white px-3 py-2 text-xs font-bold text-emerald-700 hover:bg-emerald-50 cursor-pointer"
                >
                  Áp dụng link
                </button>
              </div>

              <div v-if="mediaGallery.length > 0" class="grid grid-cols-2 gap-3 sm:grid-cols-3 pt-1">
                <div
                  v-for="(img, idx) in mediaGallery"
                  :key="idx"
                  class="group relative aspect-square overflow-hidden rounded-xl border border-slate-200 bg-white"
                >
                  <img :src="img" alt="Preview" class="h-full w-full object-cover" />
                  <div class="absolute right-1.5 top-1.5 flex items-center gap-1">
                    <button
                      type="button"
                      @click="copyImageToClipboard(img, idx)"
                      title="Copy ảnh"
                      class="flex h-6 w-6 items-center justify-center rounded-lg bg-slate-900/75 text-white transition hover:bg-emerald-600 cursor-pointer"
                    >
                      <Check v-if="copiedIndex === idx" class="h-3.5 w-3.5 text-emerald-300" />
                      <Copy v-else class="h-3.5 w-3.5" />
                    </button>
                    <button
                      type="button"
                      @click="removeGalleryImage(idx)"
                      title="Xóa ảnh"
                      class="flex h-6 w-6 items-center justify-center rounded-lg bg-slate-900/75 text-white transition hover:bg-rose-600 cursor-pointer"
                    >
                      <Trash2 class="h-3.5 w-3.5" />
                    </button>
                  </div>
                </div>
              </div>
              <div v-else-if="mediaPreviewURL" class="relative h-40 w-40 overflow-hidden rounded-xl border border-slate-200 bg-white">
                <img :src="mediaPreviewURL" alt="Preview" class="h-full w-full object-cover" />
                <button
                  type="button"
                  @click="copyImageToClipboard(mediaPreviewURL, 0)"
                  title="Copy ảnh"
                  class="absolute right-1.5 top-1.5 flex h-6 w-6 items-center justify-center rounded-lg bg-slate-900/75 text-white transition hover:bg-emerald-600 cursor-pointer"
                >
                  <Check v-if="copiedIndex === 0" class="h-3.5 w-3.5 text-emerald-300" />
                  <Copy v-else class="h-3.5 w-3.5" />
                </button>
              </div>
            </div>

            <input
              v-if="mediaType === 'video'"
              v-model="mediaURL"
              type="url"
              placeholder="Dán đường dẫn video công khai (https://...mp4)"
              class="w-full rounded-xl border border-slate-200 bg-white px-3.5 py-2 text-xs !text-slate-900 placeholder:text-slate-400 outline-none focus:border-emerald-600"
            />
          </div>

          <!-- Chọn thời gian & Khung giờ vàng -->
          <div class="border-t border-slate-100 pt-4 space-y-2.5">
            <div class="flex flex-wrap items-center justify-between gap-2">
              <label class="inline-flex items-center gap-1.5 text-xs font-bold text-slate-800">
                <Clock class="h-3.5 w-3.5 text-emerald-600" />
                <span>2. Chọn thời gian hẹn đăng & Khung giờ vàng</span>
              </label>

              <div class="flex flex-wrap gap-1.5">
                <button
                  v-for="gh in goldenHours"
                  :key="gh.label"
                  type="button"
                  @click="applyGoldenHour(gh.hour, gh.minute)"
                  class="rounded-lg border border-amber-200 bg-amber-50 px-2 py-1 text-[11px] font-semibold text-amber-800 hover:bg-amber-100 cursor-pointer"
                >
                  {{ gh.label }}
                </button>
              </div>
            </div>

            <input
              v-model="scheduledDateTime"
              type="datetime-local"
              class="w-full rounded-xl border border-slate-200 bg-slate-50/60 px-3.5 py-2 text-xs font-semibold !text-slate-800 outline-none focus:border-emerald-600 focus:bg-white"
            />
          </div>

          <!-- Chọn danh sách Fanpage đích -->
          <div class="border-t border-slate-100 pt-4 space-y-2.5">
            <div class="flex items-center justify-between">
              <span class="text-xs font-bold text-slate-800">
                3. Chọn Fanpage đăng bài ({{ selectedPageIDs.length }}/{{ pages.length }} Page)
              </span>
              <button
                type="button"
                @click="toggleAllPages"
                class="text-xs font-semibold text-emerald-700 hover:underline cursor-pointer"
              >
                {{ isAllSelected ? 'Bỏ chọn tất cả' : 'Chọn tất cả Fanpage' }}
              </button>
            </div>

            <input
              v-model="pageSearch"
              type="text"
              placeholder="Lọc nhanh tên Fanpage..."
              class="w-full rounded-xl border border-slate-200 bg-white px-3 py-1.5 text-xs !text-slate-800 outline-none focus:border-emerald-600"
            />

            <div class="grid max-h-56 grid-cols-1 gap-2 overflow-y-auto rounded-xl border border-slate-200 bg-slate-50/40 p-2.5 sm:grid-cols-2">
              <label
                v-for="p in filteredPages"
                :key="p.page_id"
                class="flex items-center gap-2.5 rounded-xl border bg-white p-2 text-xs transition cursor-pointer"
                :class="selectedPageIDs.includes(p.page_id) ? 'border-emerald-500 bg-emerald-50/40' : 'border-slate-200 hover:bg-slate-50'"
              >
                <input
                  type="checkbox"
                  :checked="selectedPageIDs.includes(p.page_id)"
                  @change="toggleSinglePage(p.page_id)"
                  class="h-4 w-4 accent-emerald-600 cursor-pointer"
                />
                <Facebook class="h-3.5 w-3.5 shrink-0 text-emerald-600" />
                <span class="truncate font-semibold text-slate-800">{{ p.name }}</span>
              </label>
            </div>
          </div>

          <!-- Nút hành động -->
          <div class="flex flex-wrap items-center justify-end gap-2.5 border-t border-slate-100 pt-4">
            <button
              type="button"
              @click="handleSubmit(true)"
              :disabled="submitting || (!content.trim() && mediaGallery.length === 0 && !mediaURL.trim()) || selectedPageIDs.length === 0"
              class="inline-flex items-center gap-1.5 rounded-xl border border-emerald-200 bg-emerald-50 px-4 py-2.5 text-xs font-bold text-emerald-800 transition hover:bg-emerald-100 disabled:opacity-50 cursor-pointer"
            >
              <Send class="h-3.5 w-3.5" />
              <span>Đăng ngay lập tức</span>
            </button>

            <button
              type="button"
              @click="handleSubmit(false)"
              :disabled="submitting || (!content.trim() && mediaGallery.length === 0 && !mediaURL.trim()) || selectedPageIDs.length === 0"
              class="inline-flex items-center gap-1.5 rounded-xl bg-emerald-600 px-5 py-2.5 text-xs font-bold text-white shadow-2xs transition hover:bg-emerald-700 disabled:opacity-50 cursor-pointer"
            >
              <Loader2 v-if="submitting" class="h-3.5 w-3.5 animate-spin" />
              <Calendar v-else class="h-3.5 w-3.5" />
              <span>Lên lịch tự động đăng</span>
            </button>
          </div>
        </div>
      </section>

      <!-- CỘT PHẢI -->
      <section class="col-span-12 space-y-5 lg:col-span-5">
        <div
          v-if="showAIEditor"
          class="rounded-2xl border border-emerald-200/80 bg-white p-4 sm:p-5 text-left shadow-2xs space-y-4"
        >
          <!-- Header AI Studio tối ưu cho cả desktop lẫn mobile -->
          <div class="flex flex-wrap items-center justify-between gap-2.5 border-b border-slate-100 pb-3">
            <div class="flex min-w-0 items-center gap-2.5">
              <div class="flex h-8 w-8 shrink-0 items-center justify-center rounded-xl bg-emerald-600 text-white">
                <Bot class="h-4 w-4" />
              </div>
              <div class="min-w-0">
                <h2 class="text-sm font-extrabold !text-slate-900">AI gợi ý 10–20 Content</h2>
                <p class="text-[11px] text-slate-400">AI tự nghĩ văn theo Form, Insight & Chat</p>
              </div>
            </div>

            <div class="flex flex-wrap items-center gap-1.5">
              <div class="inline-flex rounded-lg border border-slate-200 bg-slate-50 p-0.5 text-[11px] font-bold">
                <button
                  type="button"
                  @click="aiProvider = 'gemini'"
                  class="rounded-md px-2 py-0.5 transition cursor-pointer"
                  :class="aiProvider === 'gemini' ? 'bg-emerald-600 text-white' : 'text-slate-600 hover:text-slate-900'"
                >
                  Gemini
                </button>
                <button
                  type="button"
                  @click="aiProvider = 'openai'"
                  class="rounded-md px-2 py-0.5 transition cursor-pointer"
                  :class="aiProvider === 'openai' ? 'bg-emerald-600 text-white' : 'text-slate-600 hover:text-slate-900'"
                >
                  OpenAI
                </button>
              </div>

              <button
                type="button"
                @click="showFormBox = !showFormBox"
                title="Form mẫu & Insight"
                class="inline-flex items-center gap-1 rounded-lg border border-slate-200 px-2 py-1 text-[11px] font-semibold text-slate-600 hover:bg-slate-50 cursor-pointer"
              >
                <FileText class="h-3 w-3 text-emerald-600" />
                <span>Form/Insight</span>
              </button>

              <button
                type="button"
                @click="resetAIChat"
                title="Làm mới"
                class="rounded-lg border border-slate-200 p-1.5 text-slate-500 hover:bg-slate-50 hover:text-slate-800 cursor-pointer"
              >
                <RotateCcw class="h-3.5 w-3.5" />
              </button>
            </div>
          </div>

          <!-- Khung Insight & Form mẫu tham chiếu -->
          <div v-if="showFormBox" class="space-y-2.5 rounded-xl border border-slate-200/80 bg-slate-50/70 p-3">
            <div class="space-y-1">
              <label class="inline-flex items-center gap-1 text-[11px] font-bold text-slate-700">
                <Lightbulb class="h-3.5 w-3.5 text-amber-500" />
                <span>Insight của bạn (tùy chọn):</span>
              </label>
              <textarea
                v-model="userInsight"
                rows="2"
                placeholder="Tự nhập insight tệp khách, nỗi đau, nhu cầu hoặc điểm chạm..."
                class="w-full rounded-lg border border-slate-200 bg-white p-2 text-[11px] leading-relaxed !text-slate-800 placeholder:text-slate-400 outline-none focus:border-emerald-500"
              />
            </div>

            <div class="space-y-1">
              <span class="block text-[11px] font-bold text-slate-700">Form content mẫu (tự dán vào đây):</span>
              <textarea
                v-model="referenceForm"
                rows="4"
                placeholder="Dán bất kỳ content mẫu nào của bạn vào đây để AI phân tích và tự viết 10–20 bài mới..."
                class="w-full rounded-lg border border-slate-200 bg-white p-2 text-[11px] leading-relaxed !text-slate-800 placeholder:text-slate-400 outline-none focus:border-emerald-500"
              />
            </div>

            <div class="flex items-center justify-between gap-2 pt-0.5">
              <span class="text-[11px] font-bold text-slate-600">Số lượng đề xuất:</span>
              <div class="inline-flex rounded-lg border border-slate-200 bg-white p-0.5 text-[11px] font-bold">
                <button
                  v-for="cnt in [10, 15, 20]"
                  :key="cnt"
                  type="button"
                  @click="suggestionCount = cnt as 10 | 15 | 20"
                  class="rounded-md px-2.5 py-0.5 transition cursor-pointer"
                  :class="suggestionCount === cnt ? 'bg-emerald-600 text-white' : 'text-slate-600 hover:text-slate-900'"
                >
                  {{ cnt }} bài
                </button>
              </div>
            </div>
          </div>

          <!-- Điều kiện nhanh -->
          <div class="flex flex-wrap gap-1.5">
            <button
              v-for="opt in aiConditionOptions"
              :key="opt.id"
              type="button"
              @click="toggleCondition(opt.id)"
              class="rounded-lg px-2.5 py-1 text-[11px] font-semibold transition cursor-pointer"
              :class="
                activeConditions.includes(opt.id)
                  ? 'bg-emerald-600 text-white'
                  : 'border border-slate-200 bg-slate-50 text-slate-700 hover:border-emerald-300'
              "
            >
              {{ opt.label }}
            </button>
          </div>

          <!-- Lịch sử chat đa lượt -->
          <div class="max-h-36 space-y-2 overflow-y-auto rounded-xl border border-slate-100 bg-slate-50/80 p-3">
            <div v-for="turn in aiChatTurns" :key="turn.id" class="space-y-1">
              <div
                class="rounded-xl px-3 py-2 text-xs leading-relaxed whitespace-pre-line"
                :class="
                  turn.role === 'user'
                    ? 'ml-6 bg-emerald-600 text-white font-medium'
                    : 'mr-2 bg-white text-slate-700 border border-slate-200/80'
                "
              >
                {{ turn.text }}
              </div>
            </div>
          </div>

          <p v-if="aiError" class="text-[11px] font-semibold text-rose-600">
            {{ aiError }}
          </p>

          <!-- Ô chat prompt / điều kiện -->
          <div class="flex gap-2">
            <input
              v-model="aiPromptInput"
              @keydown.enter.prevent="handleSendAIPrompt"
              type="text"
              placeholder="Nhập prompt hoặc điều kiện chỉnh sửa cho AI..."
              class="min-w-0 flex-1 rounded-xl border border-slate-200 bg-slate-50/60 px-3 py-2 text-xs !text-slate-900 placeholder:text-slate-400 outline-none focus:border-emerald-600 focus:bg-white"
            />
            <button
              type="button"
              @click="handleSendAIPrompt"
              :disabled="aiLoading"
              class="inline-flex shrink-0 items-center gap-1.5 rounded-xl bg-emerald-600 px-3.5 py-2 text-xs font-bold text-white transition hover:bg-emerald-700 disabled:opacity-50 cursor-pointer"
            >
              <Loader2 v-if="aiLoading" class="h-3.5 w-3.5 animate-spin" />
              <Sparkles v-else class="h-3.5 w-3.5" />
              <span>{{ aiLoading ? 'AI đang viết...' : `Sinh ${suggestionCount} bài` }}</span>
            </button>
          </div>

          <!-- Danh sách 10 - 20 Content AI đề xuất -->
          <div v-if="aiSuggestions.length > 0" class="space-y-2.5 border-t border-slate-100 pt-3.5">
            <div class="flex items-center justify-between gap-2">
              <span class="text-xs font-extrabold !text-slate-900">
                Danh sách đề xuất ({{ aiSuggestions.length }} bài)
              </span>
              <span class="text-[11px] text-slate-400">Bấm "Dùng bài này" để đưa sang cột trái</span>
            </div>

            <div class="max-h-96 space-y-2.5 overflow-y-auto pr-1">
              <div
                v-for="(sug, idx) in aiSuggestions"
                :key="idx"
                class="rounded-xl border border-slate-200 bg-slate-50/60 p-3 space-y-2"
              >
                <div class="flex flex-wrap items-center justify-between gap-2">
                  <span class="rounded-md bg-emerald-100 px-2 py-0.5 text-[10px] font-bold text-emerald-800">
                    #{{ idx + 1 }} • {{ sug.angle }}
                  </span>

                  <div class="flex items-center gap-1.5">
                    <button
                      type="button"
                      @click="copySuggestionText(sug.content, idx)"
                      title="Copy bài"
                      class="inline-flex items-center gap-1 rounded-lg border border-slate-200 bg-white px-2 py-1 text-[10px] font-semibold text-slate-600 hover:bg-slate-50 cursor-pointer"
                    >
                      <Check v-if="copiedSuggestionIndex === idx" class="h-3 w-3 text-emerald-600" />
                      <Copy v-else class="h-3 w-3" />
                      <span>{{ copiedSuggestionIndex === idx ? 'Đã copy' : 'Copy' }}</span>
                    </button>

                    <button
                      type="button"
                      @click="applyDraftToEditor(sug.content)"
                      class="rounded-lg bg-emerald-600 px-2.5 py-1 text-[10px] font-bold text-white hover:bg-emerald-700 cursor-pointer"
                    >
                      Dùng bài này
                    </button>
                  </div>
                </div>

                <p class="whitespace-pre-line text-xs leading-relaxed !text-slate-800">
                  {{ sug.content }}
                </p>
              </div>
            </div>
          </div>
        </div>

        <!-- Danh sách bài đã lên lịch & Lịch sử đăng -->
        <div class="rounded-2xl border border-slate-200/80 bg-white p-5 text-left shadow-2xs">
          <div class="mb-4 flex items-center justify-between">
            <h2 class="text-sm font-extrabold !text-slate-900">Hàng đợi & Lịch sử đăng bài ({{ posts.length }})</h2>
          </div>

          <div v-if="loading" class="flex items-center justify-center gap-2 py-10 text-xs text-slate-500">
            <Loader2 class="h-4 w-4 animate-spin text-emerald-600" />
            <span>Đang tải lịch đăng bài...</span>
          </div>

          <div v-else-if="posts.length === 0" class="py-10 text-center text-xs text-slate-500">
            Chưa có bài viết nào được lên lịch.
          </div>

          <div v-else class="space-y-3 max-h-[75vh] overflow-y-auto pr-1">
            <div
              v-for="post in posts"
              :key="post.id"
              class="rounded-xl border border-slate-200 bg-slate-50/40 p-3.5 space-y-2.5"
            >
              <div class="flex items-center justify-between gap-2">
                <span
                  class="rounded-full px-2.5 py-0.5 text-[11px] font-bold"
                  :class="{
                    'bg-amber-100 text-amber-800': post.status === 'pending' || post.status === 'processing',
                    'bg-emerald-100 text-emerald-800': post.status === 'published',
                    'bg-blue-100 text-blue-800': post.status === 'partial',
                    'bg-red-100 text-red-700': post.status === 'failed'
                  }"
                >
                  {{
                    post.status === 'pending'
                      ? 'Chờ đến giờ đăng'
                      : post.status === 'processing'
                        ? 'Đang đăng...'
                        : post.status === 'published'
                          ? `Đã đăng (${post.success_count} Page)`
                          : post.status === 'partial'
                            ? `Xong ${post.success_count}, lỗi ${post.failed_count}`
                            : 'Đăng thất bại'
                  }}
                </span>

                <span class="text-[11px] font-medium text-slate-500">
                  {{ formatDateTime(post.scheduled_at) }}
                </span>
              </div>

              <p class="whitespace-pre-wrap text-xs text-slate-800 line-clamp-4">
                {{ post.content }}
              </p>

              <div class="flex flex-wrap gap-1">
                <span
                  v-for="(pName, i) in parsePageNames(post.page_names_json)"
                  :key="i"
                  class="rounded-md bg-white border border-slate-200 px-2 py-0.5 text-[10px] font-semibold text-slate-600"
                >
                  {{ pName }}
                </span>
              </div>

              <div
                v-if="post.failed_count > 0"
                class="rounded-lg border border-red-200 bg-red-50 p-2 text-[11px] text-red-700 space-y-1"
              >
                <div
                  v-for="(r, idx) in parseResults(post.publish_results).filter((x) => !x.success)"
                  :key="idx"
                >
                  <strong>{{ r.page_name || 'Page' }}:</strong> {{ r.error }}
                </div>
              </div>

              <div class="flex items-center justify-end gap-2 pt-1 border-t border-slate-200/70">
                <button
                  v-if="post.status === 'pending' || post.status === 'failed'"
                  type="button"
                  @click="handlePublishNow(post.id)"
                  :disabled="publishingID === post.id"
                  class="inline-flex items-center gap-1 rounded-lg bg-emerald-600 px-2.5 py-1 text-[11px] font-semibold text-white hover:bg-emerald-700 disabled:opacity-50 cursor-pointer"
                >
                  <Play class="h-3 w-3" />
                  <span>{{ publishingID === post.id ? 'Đang đăng...' : 'Đăng ngay' }}</span>
                </button>

                <button
                  type="button"
                  @click="handleDelete(post.id)"
                  class="inline-flex items-center gap-1 rounded-lg border border-slate-200 bg-white px-2.5 py-1 text-[11px] font-semibold text-slate-600 hover:border-red-200 hover:text-red-600 cursor-pointer"
                >
                  <Trash2 class="h-3 w-3" />
                  <span>Xóa</span>
                </button>
              </div>
            </div>
          </div>
        </div>
      </section>
    </main>
  </div>
</template>