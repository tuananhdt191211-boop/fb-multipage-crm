<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import {
  ArrowLeft,
  CheckCircle2,
  ChevronDown,
  ChevronUp,
  MessageSquare,
  Film,
  ImagePlus,
  Pause,
  Play,
  Plus,
  RefreshCw,
  Send,
  ShieldCheck,
  SlidersHorizontal,
  Sparkles,
  Trash2,
  UserPlus,
  Users,
  X
} from 'lucide-vue-next'
import { api } from '../lib/api'

interface PageItem {
  id?: number
  page_id: string
  name?: string
  page_name?: string
  is_connected?: boolean
}

interface MediaAttachment {
  type: 'image' | 'video'
  url: string
  name?: string
  previewUrl?: string
}

interface Campaign {
  id: number | string
  page_id: string
  name: string
  target_mode?: string
  out_of_window_tag?: string
  status: string
  pause_reason?: string
  total_recipients: number
  sent_count: number
  failed_count: number
  created_at: string
  attachments?: MediaAttachment[]
  delivery_mode?: string
}

interface SubscriberStats {
  active: number
  stopped: number
  eligible_now: number
  invites_7d: number
  marketing_ready: boolean
  marketing_error: string
}

const MAX_ATTACHMENTS = 5
const MAX_IMAGE_MB = 8
const MAX_VIDEO_MB = 25

const router = useRouter()

const pages = ref<PageItem[]>([])
const campaigns = ref<Campaign[]>([])
const loading = ref(false)
const submitting = ref(false)
const syncingInbox = ref(false)
const showAdvanced = ref(false)
const message = ref('')
const error = ref('')
let pollTimer: ReturnType<typeof setInterval> | null = null

const attachments = ref<MediaAttachment[]>([])
const uploadingMedia = ref(false)
const mediaInput = ref<HTMLInputElement | null>(null)

const form = ref({
  page_id: '',
  name: '',
  templates: [
    'Chào {{full_name}}, shop cập nhật thông tin chăm sóc khách hàng cho bạn. Nếu bạn cần hỗ trợ thêm hoặc nhận thông tin mới, nhắn lại cho shop nhé!',
    'Hello {{full_name}}, bên mình gửi thông tin hỗ trợ dành cho khách hàng của shop. Bạn cần tư vấn thêm cứ phản hồi tin nhắn này nha!'
  ],
  target_mode: 'all',
  out_of_window_tag: 'HUMAN_AGENT',
  cooldown_days: 0,
  max_recipients: 200,
  min_delay_sec: 4,
  max_delay_sec: 9,
  batch_size: 20,
  batch_pause_sec: 60,
  // Marketing Messages (khách đã đăng ký nhận tin)
  marketing_title: '',
  marketing_image_url: '',
  marketing_button_url: '',
  marketing_button_title: 'Xem ngay',
  marketing_daily_budget: 0
})

const isMarketing = computed(() => form.value.target_mode === 'subscribers')

// ---- Lời mời nhận tin & người đăng ký ----
const subStats = ref<SubscriberStats | null>(null)
const optinForm = ref({ title: 'Thông tin mới và khuyến mãi', image_url: '' })
const sendingOptin = ref(false)
const syncingSubs = ref(false)

async function fetchSubscribers() {
  if (!form.value.page_id) {
    subStats.value = null
    return
  }
  try {
    const { data } = await api.get<SubscriberStats>(`/broadcasts/subscribers?page_id=${form.value.page_id}`)
    subStats.value = data
  } catch {
    subStats.value = null
  }
}

async function sendOptinRequests() {
  error.value = ''
  message.value = ''
  if (!form.value.page_id) return
  sendingOptin.value = true
  try {
    const { data } = await api.post('/broadcasts/optin-requests', {
      page_id: form.value.page_id,
      title: optinForm.value.title,
      image_url: optinForm.value.image_url
    })
    message.value = data?.message || `Đang gửi lời mời cho ${data?.queued ?? 0} khách`
    await fetchSubscribers()
  } catch (e: any) {
    error.value = e?.response?.data?.error || 'Gửi lời mời nhận tin thất bại'
  } finally {
    sendingOptin.value = false
  }
}

async function syncSubscribers() {
  error.value = ''
  message.value = ''
  if (!form.value.page_id) return
  syncingSubs.value = true
  try {
    const { data } = await api.post(`/broadcasts/subscribers/sync?page_id=${form.value.page_id}`)
    message.value = data?.message || 'Đã đồng bộ người đăng ký'
    await fetchSubscribers()
  } catch (e: any) {
    error.value = e?.response?.data?.error || 'Không đồng bộ được người đăng ký'
  } finally {
    syncingSubs.value = false
  }
}

watch(() => form.value.page_id, () => fetchSubscribers())

const runningCount = computed(() => campaigns.value.filter((c) => c.status === 'running').length)

function addTemplate() {
  if (form.value.templates.length < 5) {
    form.value.templates.push('')
  }
}

function removeTemplate(idx: number) {
  if (form.value.templates.length > 1) {
    form.value.templates.splice(idx, 1)
  }
}

function appendVariableToLastTemplate() {
  const idx = form.value.templates.length - 1
  const current = form.value.templates[idx] || ''
  form.value.templates[idx] = current + (current.endsWith(' ') || current === '' ? '' : ' ') + '{{full_name}}'
}

function getPageDisplayName(p: PageItem): string {
  const title = (p.name || p.page_name || '').trim()
  return title ? `${title} (${p.page_id})` : p.page_id
}

function getPageNameById(pageId: string): string {
  const found = pages.value.find((p) => p.page_id === pageId)
  return found ? (found.name || found.page_name || pageId) : pageId
}

function formatTargetMode(mode?: string): string {
  if (mode === 'within_24h') return 'Khách trong 24h'
  if (mode === 'over_24h') return 'Khách quá 24h'
  if (mode === 'subscribers') return 'Khách đã đăng ký nhận tin (Marketing)'
  return 'Tất cả khách (trong & quá 24h)'
}

function getProgressPercent(c: Campaign): number {
  if (!c.total_recipients || c.total_recipients <= 0) return 0
  const done = (c.sent_count || 0) + (c.failed_count || 0)
  return Math.min(100, Math.round((done / c.total_recipients) * 100))
}

function openMediaPicker() {
  mediaInput.value?.click()
}

async function handleMediaSelected(e: Event) {
  const input = e.target as HTMLInputElement
  const picked = Array.from(input.files || [])
  input.value = ''
  if (picked.length === 0) return

  error.value = ''
  const room = MAX_ATTACHMENTS - attachments.value.length
  if (room <= 0) {
    error.value = `Tối đa ${MAX_ATTACHMENTS} ảnh/video cho mỗi chiến dịch`
    return
  }

  const valid: File[] = []
  const rejected: string[] = []
  for (const f of picked.slice(0, room)) {
    const isImage = f.type.startsWith('image/')
    const isVideo = f.type.startsWith('video/')
    const limitMB = isVideo ? MAX_VIDEO_MB : MAX_IMAGE_MB
    if (!isImage && !isVideo) {
      rejected.push(`${f.name} (không phải ảnh/video)`)
    } else if (f.size > limitMB * 1024 * 1024) {
      rejected.push(`${f.name} (vượt ${limitMB}MB)`)
    } else {
      valid.push(f)
    }
  }
  if (picked.length > room) {
    rejected.push(`chỉ nhận thêm ${room} file`)
  }
  if (valid.length === 0) {
    error.value = 'Không có file hợp lệ: ' + rejected.join(', ')
    return
  }

  uploadingMedia.value = true
  try {
    const fd = new FormData()
    valid.forEach((f) => fd.append('files', f))
    const { data } = await api.post<{ files: MediaAttachment[]; rejected?: string[] }>(
      '/broadcasts/upload-media',
      fd,
      { headers: { 'Content-Type': 'multipart/form-data' }, timeout: 300000 }
    )
    const uploaded = data?.files || []
    uploaded.forEach((att) => {
      const local = valid.find((f) => f.name === att.name)
      attachments.value.push({ ...att, previewUrl: local ? URL.createObjectURL(local) : undefined })
    })
    const allRejected = [...rejected, ...(data?.rejected || [])]
    if (allRejected.length > 0) {
      error.value = 'Một số file bị bỏ qua: ' + allRejected.join(', ')
    }
  } catch (e: any) {
    error.value = e?.response?.data?.error || 'Tải ảnh/video lên thất bại'
  } finally {
    uploadingMedia.value = false
  }
}

function removeAttachment(idx: number) {
  const [removed] = attachments.value.splice(idx, 1)
  if (removed?.previewUrl) URL.revokeObjectURL(removed.previewUrl)
}

function clearAttachments() {
  attachments.value.forEach((a) => a.previewUrl && URL.revokeObjectURL(a.previewUrl))
  attachments.value = []
}

function countMedia(c: Campaign, type: 'image' | 'video'): number {
  return (c.attachments || []).filter((a) => a.type === type).length
}

async function fetchData(silent = false) {
  if (!silent) loading.value = true
  try {
    const [pagesRes, campRes] = await Promise.all([
      api.get('/pages'),
      api.get('/broadcasts')
    ])

    const rawPages = Array.isArray(pagesRes.data)
      ? pagesRes.data
      : (pagesRes.data?.pages || pagesRes.data?.data || [])

    pages.value = rawPages.filter((p: PageItem) => p.is_connected !== false)
    if (!form.value.page_id && pages.value.length > 0) {
      form.value.page_id = pages.value[0].page_id
    }

    campaigns.value = campRes.data?.data || []
  } catch (e: any) {
    if (!silent) {
      error.value = e?.response?.data?.error || 'Không tải được dữ liệu chiến dịch'
    }
  } finally {
    if (!silent) loading.value = false
  }
}

async function syncCustomerData() {
  syncingInbox.value = true
  error.value = ''
  message.value = ''
  try {
    const query = form.value.page_id ? `?page_id=${form.value.page_id}` : ''
    const { data } = await api.post(`/inbox/sync${query}`)
    message.value = data?.message || 'Đã đồng bộ dữ liệu khách hàng từ Meta Business!'
  } catch (e: any) {
    error.value = e?.response?.data?.error || 'Không đồng bộ được dữ liệu khách hàng'
  } finally {
    syncingInbox.value = false
  }
}

async function createCampaign() {
  error.value = ''
  message.value = ''
  const hasText = form.value.templates.some((t) => t.trim() !== '')
  if (!hasText && attachments.value.length === 0) {
    error.value = 'Hãy nhập ít nhất 1 mẫu nội dung hoặc đính kèm ảnh/video'
    return
  }
  if (uploadingMedia.value) {
    error.value = 'Đang tải ảnh/video lên, vui lòng đợi xong rồi bấm gửi'
    return
  }
  submitting.value = true
  try {
    const res = await api.post('/broadcasts', {
      ...form.value,
      templates: form.value.templates.filter((t) => t.trim() !== ''),
      attachments: attachments.value.map(({ type, url, name }) => ({ type, url, name }))
    }, { timeout: 180000 })
    const total = res.data?.total_recipients ?? 0
    message.value = `Đã khởi chạy chiến dịch cho ${total} khách hàng!`
    form.value.name = ''
    clearAttachments()
    await fetchData()
    if (isMarketing.value) await fetchSubscribers()
  } catch (e: any) {
    error.value = e?.response?.data?.error || 'Tạo chiến dịch thất bại'
  } finally {
    submitting.value = false
  }
}

async function toggleStatus(camp: Campaign) {
  error.value = ''
  const nextStatus = camp.status === 'running' ? 'paused' : 'running'
  try {
    await api.patch(`/broadcasts/${camp.id}/status`, { status: nextStatus })
    await fetchData()
  } catch (e: any) {
    error.value = e?.response?.data?.error || 'Không đổi được trạng thái'
  }
}

onMounted(() => {
  fetchData()
  pollTimer = setInterval(() => {
    if (campaigns.value.some((c) => c.status === 'running')) {
      fetchData(true)
    }
  }, 5000)
})

onUnmounted(() => {
  if (pollTimer) clearInterval(pollTimer)
  clearAttachments()
})
</script>

<template>
  <div class="min-h-screen bg-slate-50 text-slate-900 antialiased">
    <!-- Top Header Navigation -->
    <header class="sticky top-0 z-20 border-b border-slate-200/80 bg-white/95 backdrop-blur-md">
      <div class="mx-auto flex h-16 max-w-7xl items-center justify-between gap-4 px-4 sm:px-6 lg:px-8">
        <div class="flex items-center gap-3">
          <button
            type="button"
            @click="router.push('/')"
            class="lg:hidden inline-flex items-center gap-1.5 rounded-xl border border-slate-200 bg-white px-3 py-2 text-xs font-semibold text-slate-700 transition hover:border-emerald-300 hover:bg-emerald-50 hover:text-emerald-800 cursor-pointer"
          >
            <ArrowLeft class="h-3.5 w-3.5" />
            <span>Trang chủ</span>
          </button>

          <div class="hidden sm:flex items-center gap-2 border-l border-slate-200 pl-3 lg:border-l-0 lg:pl-0">
            <div class="flex h-8 w-8 items-center justify-center rounded-lg bg-emerald-600 text-white">
              <Send class="h-4 w-4" />
            </div>
            <div class="text-left leading-tight">
              <span class="block text-sm font-extrabold !text-slate-900">Chăm sóc khách cũ tự động</span>
              <span class="block text-[11px] text-slate-400">PagePilot Conversion Broadcast</span>
            </div>
          </div>
        </div>

        <div class="flex items-center gap-2">
          <button
            type="button"
            @click="router.push('/inbox')"
            class="hidden md:inline-flex items-center gap-1.5 rounded-xl border border-slate-200 bg-white px-3 py-2 text-xs font-semibold text-slate-700 transition hover:bg-slate-50 cursor-pointer"
          >
            <MessageSquare class="h-3.5 w-3.5 text-emerald-600" />
            <span>Hộp thư</span>
          </button>

          <button
            type="button"
            @click="syncCustomerData"
            :disabled="syncingInbox"
            class="inline-flex items-center gap-1.5 rounded-xl border border-emerald-200 bg-emerald-50 px-3.5 py-2 text-xs font-semibold text-emerald-800 transition hover:bg-emerald-100 disabled:opacity-60 cursor-pointer"
          >
            <RefreshCw class="h-3.5 w-3.5" :class="{ 'animate-spin': syncingInbox }" />
            <span>{{ syncingInbox ? 'Đang đồng bộ...' : 'Đồng bộ khách Meta' }}</span>
          </button>

          <span class="hidden lg:inline-flex items-center gap-1.5 rounded-full bg-emerald-600/10 px-3 py-1.5 text-[11px] font-bold text-emerald-800">
            <ShieldCheck class="h-3.5 w-3.5 text-emerald-600" />
            <span>08:30 – 21:00</span>
          </span>
        </div>
      </div>
    </header>

    <!-- Main Content -->
    <main class="mx-auto max-w-7xl space-y-6 px-4 py-6 sm:px-6 lg:px-8">
      <!-- Intro Banner gọn gàng -->
      <section class="rounded-2xl border border-emerald-200/70 bg-gradient-to-r from-emerald-800 via-emerald-700 to-teal-700 p-5 sm:p-6 text-white shadow-xs">
        <div class="flex flex-col justify-between gap-4 md:flex-row md:items-center">
          <div class="space-y-1 text-left">
            <div class="inline-flex items-center gap-1.5 rounded-full bg-white/15 px-2.5 py-0.5 text-[11px] font-semibold text-emerald-100">
              <Sparkles class="h-3 w-3" />
              <span>Tự động hóa CSKH Meta Business</span>
            </div>
            <h1 class="text-xl sm:text-2xl font-extrabold tracking-tight !text-white">
              Nhắn tin chăm sóc lại toàn bộ khách hàng trong Page
            </h1>
            <p class="text-xs sm:text-[13px] text-emerald-50/90">
              Tự động quét danh sách khách đã nhắn tin trên Meta Business, xoay vòng mẫu câu cá nhân hóa tên khách và giãn cách nhịp gửi an toàn.
            </p>
          </div>
        </div>
      </section>

      <!-- Thông báo Lỗi / Thành công -->
      <div
        v-if="error"
        class="flex flex-wrap items-center justify-between gap-3 rounded-2xl border border-rose-200 bg-rose-50 px-4 py-3 text-left text-xs font-medium text-rose-800"
      >
        <span>{{ error }}</span>
        <button
          type="button"
          @click="syncCustomerData"
          :disabled="syncingInbox"
          class="shrink-0 rounded-lg bg-rose-600 px-3 py-1.5 text-[11px] font-bold text-white hover:bg-rose-700 cursor-pointer"
        >
          {{ syncingInbox ? 'Đang đồng bộ...' : 'Đồng bộ ngay' }}
        </button>
      </div>

      <div
        v-if="message"
        class="flex items-center gap-2 rounded-2xl border border-emerald-200 bg-emerald-50 px-4 py-3 text-left text-xs font-semibold text-emerald-800"
      >
        <CheckCircle2 class="h-4 w-4 shrink-0 text-emerald-600" />
        <span>{{ message }}</span>
      </div>

      <!-- Mời khách đăng ký nhận tin (Marketing Messages) -->
      <section class="rounded-2xl border border-emerald-200/80 bg-white p-5 text-left shadow-2xs">
        <div class="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
          <div class="space-y-1 lg:max-w-md">
            <div class="flex items-center gap-2">
              <div class="flex h-8 w-8 items-center justify-center rounded-lg bg-emerald-600 text-white">
                <UserPlus class="h-4 w-4" />
              </div>
              <h2 class="text-sm font-extrabold !text-slate-900">Mời khách đăng ký nhận tin</h2>
            </div>
            <p class="text-[11px] leading-relaxed text-slate-500">
              Gửi lời mời cho khách đã nhắn Page trong 24h qua. Khách bấm đồng ý sẽ vào danh sách đăng ký, sau đó bạn gửi được tin khuyến mãi cho họ
              <b>kể cả khi đã quá 24h</b> (chọn đối tượng "Khách đã đăng ký nhận tin"). Mỗi khách chỉ được mời 1 lần/tuần với cùng tiêu đề.
            </p>
            <div class="flex flex-wrap gap-2 pt-1 text-[11px]">
              <span class="rounded-full bg-emerald-50 px-2.5 py-1 font-bold text-emerald-800">Đang đăng ký: {{ subStats?.active ?? 0 }}</span>
              <span class="rounded-full bg-emerald-50 px-2.5 py-1 font-bold text-emerald-800">Nhận được hôm nay: {{ subStats?.eligible_now ?? 0 }}</span>
              <span class="rounded-full bg-slate-100 px-2.5 py-1 font-bold text-slate-600">Đã dừng: {{ subStats?.stopped ?? 0 }}</span>
              <span class="rounded-full bg-slate-100 px-2.5 py-1 font-bold text-slate-600">Lời mời 7 ngày: {{ subStats?.invites_7d ?? 0 }}</span>
            </div>
          </div>

          <div class="grid flex-1 grid-cols-1 gap-3 sm:grid-cols-2 lg:max-w-xl">
            <div>
              <label class="block text-[11px] font-semibold text-slate-600">Tiêu đề lời mời (tối đa 65 ký tự)</label>
              <input v-model="optinForm.title" maxlength="65"
                class="mt-1 w-full rounded-lg border border-slate-200 bg-slate-50/50 px-2.5 py-1.5 text-sm !text-slate-900" />
            </div>
            <div>
              <label class="block text-[11px] font-semibold text-slate-600">Link ảnh (không bắt buộc)</label>
              <input v-model="optinForm.image_url" placeholder="https://..."
                class="mt-1 w-full rounded-lg border border-slate-200 bg-slate-50/50 px-2.5 py-1.5 text-sm !text-slate-900" />
            </div>
            <button type="button" @click="sendOptinRequests" :disabled="sendingOptin || !form.page_id"
              class="inline-flex items-center justify-center gap-1.5 rounded-xl bg-emerald-600 px-3.5 py-2 text-xs font-bold text-white transition hover:bg-emerald-700 disabled:opacity-60 cursor-pointer">
              <Send class="h-3.5 w-3.5" />
              <span>{{ sendingOptin ? 'Đang gửi...' : 'Gửi lời mời cho khách trong 24h' }}</span>
            </button>
            <button type="button" @click="syncSubscribers" :disabled="syncingSubs || !form.page_id"
              class="inline-flex items-center justify-center gap-1.5 rounded-xl border border-emerald-200 bg-emerald-50 px-3.5 py-2 text-xs font-bold text-emerald-800 transition hover:bg-emerald-100 disabled:opacity-60 cursor-pointer">
              <RefreshCw class="h-3.5 w-3.5" :class="{ 'animate-spin': syncingSubs }" />
              <span>{{ syncingSubs ? 'Đang đồng bộ...' : 'Đồng bộ người đăng ký từ Meta' }}</span>
            </button>
          </div>
        </div>
      </section>

      <!-- Layout 2 cột: Form tạo chiến dịch (7 cột) + Tiến độ gửi (5 cột) -->
      <div class="grid grid-cols-1 gap-6 lg:grid-cols-12 items-start">
        <!-- Cột trái: Form cấu hình -->
        <form
          @submit.prevent="createCampaign"
          class="space-y-5 rounded-2xl border border-slate-200/80 bg-white p-6 text-left shadow-2xs lg:col-span-7"
        >
          <div class="border-b border-slate-100 pb-3.5">
            <h2 class="text-base font-extrabold !text-slate-900">Tạo chiến dịch gửi tin mới</h2>
            <p class="mt-0.5 text-xs text-slate-500">Chọn Fanpage nhận khách và soạn các biến thể nội dung chăm sóc</p>
          </div>

          <div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
            <div>
              <label class="mb-1.5 block text-xs font-bold !text-slate-700">Fanpage gửi tin</label>
              <select
                v-model="form.page_id"
                required
                class="w-full rounded-xl border border-slate-200 bg-slate-50/50 px-3.5 py-2.5 text-sm font-medium !text-slate-900 transition focus:border-emerald-500 focus:bg-white focus:outline-none"
              >
                <option value="" disabled>-- Chọn Fanpage gửi tin --</option>
                <option v-for="p in pages" :key="p.page_id" :value="p.page_id">
                  {{ getPageDisplayName(p) }}
                </option>
              </select>
            </div>

            <div>
              <label class="mb-1.5 block text-xs font-bold !text-slate-700">Tên chiến dịch</label>
              <input
                v-model="form.name"
                required
                placeholder="VD: Chăm sóc khách cũ tuần này"
                class="w-full rounded-xl border border-slate-200 bg-slate-50/50 px-3.5 py-2.5 text-sm !text-slate-900 placeholder:text-slate-400 transition focus:border-emerald-500 focus:bg-white focus:outline-none"
              />
            </div>
          </div>

          <div class="grid grid-cols-1 gap-4 sm:grid-cols-3">
            <div>
              <label class="mb-1.5 block text-xs font-bold !text-slate-700">Đối tượng nhận tin</label>
              <select
                v-model="form.target_mode"
                class="w-full rounded-xl border border-slate-200 bg-slate-50/50 px-3 py-2.5 text-xs font-semibold !text-slate-800 transition focus:border-emerald-500 focus:bg-white focus:outline-none"
              >
                <option value="all">Tất cả khách (trong & quá 24h)</option>
                <option value="over_24h">Chỉ khách cũ quá 24h</option>
                <option value="within_24h">Chỉ khách tương tác trong 24h</option>
                <option value="subscribers">Khách đã đăng ký nhận tin (Marketing – kể cả ngoài 24h)</option>
              </select>
            </div>

            <div>
              <label class="mb-1.5 block text-xs font-bold !text-slate-700">Tag ưu tiên quá 24h</label>
              <select
                v-model="form.out_of_window_tag"
                :disabled="form.target_mode === 'within_24h' || isMarketing"
                class="w-full rounded-xl border border-slate-200 bg-slate-50/50 px-3 py-2.5 text-xs font-semibold !text-slate-800 disabled:bg-slate-100 disabled:text-slate-400 transition focus:border-emerald-500 focus:bg-white focus:outline-none"
              >
                <option value="HUMAN_AGENT">HUMAN_AGENT (Hỗ trợ CSKH)</option>
                <option value="POST_PURCHASE_UPDATE">POST_PURCHASE_UPDATE (Sau mua)</option>
                <option value="CONFIRMED_EVENT_UPDATE">CONFIRMED_EVENT_UPDATE (Nhắc lịch)</option>
              </select>
            </div>

            <div>
              <label class="mb-1.5 block text-xs font-bold !text-slate-700">Số khách tối đa</label>
              <input
                v-model.number="form.max_recipients"
                type="number"
                min="1"
                max="2000"
                class="w-full rounded-xl border border-slate-200 bg-slate-50/50 px-3.5 py-2.5 text-sm font-semibold !text-slate-900 transition focus:border-emerald-500 focus:bg-white focus:outline-none"
              />
            </div>
          </div>

          <!-- Cấu hình tin Marketing (chỉ khi chọn khách đã đăng ký) -->
          <div v-if="isMarketing" class="space-y-3 rounded-xl border border-emerald-200 bg-emerald-50/50 p-4">
            <div>
              <p class="text-xs font-bold text-emerald-900">Tin Marketing Messages (tính phí theo tin giao tới khách)</p>
              <p class="mt-0.5 text-[11px] text-emerald-800/80">
                Gửi dạng thẻ: tiêu đề & mô tả tối đa 80 ký tự mỗi phần (phần dài hơn sẽ bị cắt). Mỗi khách nhận tối đa 1 tin/ngày.
                Hiện có <b>{{ subStats?.eligible_now ?? 0 }}</b> khách đủ điều kiện nhận.
              </p>
              <p v-if="subStats && !subStats.marketing_ready" class="mt-1.5 rounded-lg bg-amber-100 px-2.5 py-1.5 text-[11px] font-semibold text-amber-900">
                {{ subStats.marketing_error }}
              </p>
            </div>
            <div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
              <div>
                <label class="block text-[11px] font-semibold text-slate-600">Tiêu đề thẻ (để trống = dòng đầu mẫu câu)</label>
                <input v-model="form.marketing_title" maxlength="80" placeholder="VD: Ưu đãi 20% cuối tuần"
                  class="mt-1 w-full rounded-lg border border-slate-200 bg-white px-2.5 py-1.5 text-sm !text-slate-900" />
              </div>
              <div>
                <label class="block text-[11px] font-semibold text-slate-600">Link ảnh (URL công khai, không bắt buộc)</label>
                <input v-model="form.marketing_image_url" placeholder="https://..."
                  class="mt-1 w-full rounded-lg border border-slate-200 bg-white px-2.5 py-1.5 text-sm !text-slate-900" />
              </div>
              <div>
                <label class="block text-[11px] font-semibold text-slate-600">Link nút bấm (không bắt buộc)</label>
                <input v-model="form.marketing_button_url" placeholder="https://shop.vn/khuyen-mai"
                  class="mt-1 w-full rounded-lg border border-slate-200 bg-white px-2.5 py-1.5 text-sm !text-slate-900" />
              </div>
              <div class="grid grid-cols-2 gap-3">
                <div>
                  <label class="block text-[11px] font-semibold text-slate-600">Chữ trên nút</label>
                  <input v-model="form.marketing_button_title" maxlength="20"
                    class="mt-1 w-full rounded-lg border border-slate-200 bg-white px-2.5 py-1.5 text-sm !text-slate-900" />
                </div>
                <div>
                  <label class="block text-[11px] font-semibold text-slate-600">Ngân sách/ngày (0 = Meta tự đặt)</label>
                  <input v-model.number="form.marketing_daily_budget" type="number" min="0"
                    class="mt-1 w-full rounded-lg border border-slate-200 bg-white px-2.5 py-1.5 text-sm !text-slate-900" />
                </div>
              </div>
            </div>
            <p class="text-[11px] text-emerald-800/70">Ảnh/video đính kèm bên dưới không áp dụng cho tin Marketing – dùng ô "Link ảnh" ở trên.</p>
          </div>

          <!-- Biến thể mẫu câu -->
          <div class="space-y-2.5">
            <div class="flex flex-wrap items-center justify-between gap-2">
              <div class="flex items-center gap-2">
                <label class="text-xs font-bold !text-slate-700">Mẫu nội dung xoay vòng</label>
                <button
                  type="button"
                  @click="appendVariableToLastTemplate"
                  class="rounded-md bg-emerald-50 px-2 py-0.5 text-[11px] font-bold text-emerald-700 ring-1 ring-emerald-200/80 hover:bg-emerald-100 cursor-pointer"
                >
                  + Chèn tên khách
                </button>
              </div>

              <button
                type="button"
                @click="addTemplate"
                class="inline-flex items-center gap-1 text-xs font-bold text-emerald-700 hover:text-emerald-800 cursor-pointer"
              >
                <Plus class="h-3.5 w-3.5" />
                <span>Thêm mẫu câu</span>
              </button>
            </div>

            <div class="space-y-2.5">
              <div v-for="(_, idx) in form.templates" :key="idx" class="relative flex gap-2">
                <div class="flex-1">
                  <textarea
                    v-model="form.templates[idx]"
                    rows="2"
                    class="w-full rounded-xl border border-slate-200 bg-slate-50/40 p-3 text-sm leading-relaxed !text-slate-900 placeholder:text-slate-400 transition focus:border-emerald-500 focus:bg-white focus:outline-none"
                    :placeholder="`Nhập nội dung mẫu #${idx + 1} (dùng {{full_name}} để chèn tên khách)...`"
                  />
                </div>
                <button
                  v-if="form.templates.length > 1"
                  type="button"
                  @click="removeTemplate(idx)"
                  title="Xóa mẫu này"
                  class="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl border border-slate-200 bg-white text-slate-400 transition hover:border-rose-200 hover:bg-rose-50 hover:text-rose-600 cursor-pointer"
                >
                  <Trash2 class="h-4 w-4" />
                </button>
              </div>
            </div>
          </div>

          <!-- Ảnh / Video đính kèm -->
          <div class="space-y-2.5">
            <div class="flex flex-wrap items-center justify-between gap-2">
              <div>
                <label class="text-xs font-bold !text-slate-700">Ảnh / Video đính kèm</label>
                <p class="text-[11px] text-slate-400">
                  Gửi sau nội dung chữ. Tối đa {{ MAX_ATTACHMENTS }} file • Ảnh ≤ {{ MAX_IMAGE_MB }}MB • Video MP4/MOV ≤ {{ MAX_VIDEO_MB }}MB
                </p>
              </div>
              <button
                type="button"
                @click="openMediaPicker"
                :disabled="uploadingMedia || attachments.length >= MAX_ATTACHMENTS"
                class="inline-flex items-center gap-1.5 rounded-lg border border-emerald-200 bg-emerald-50 px-3 py-1.5 text-xs font-bold text-emerald-700 transition hover:bg-emerald-100 disabled:opacity-50 cursor-pointer"
              >
                <RefreshCw v-if="uploadingMedia" class="h-3.5 w-3.5 animate-spin" />
                <ImagePlus v-else class="h-3.5 w-3.5" />
                <span>{{ uploadingMedia ? 'Đang tải lên...' : 'Thêm ảnh / video' }}</span>
              </button>
              <input
                ref="mediaInput"
                type="file"
                multiple
                accept="image/jpeg,image/png,image/gif,image/webp,video/mp4,video/quicktime,video/webm,.m4v"
                class="hidden"
                @change="handleMediaSelected"
              />
            </div>

            <div v-if="attachments.length > 0" class="grid grid-cols-3 gap-2.5 sm:grid-cols-5">
              <div
                v-for="(att, idx) in attachments"
                :key="att.url"
                class="group relative aspect-square overflow-hidden rounded-xl border border-slate-200 bg-slate-100"
              >
                <img
                  v-if="att.type === 'image' && att.previewUrl"
                  :src="att.previewUrl"
                  :alt="att.name"
                  class="h-full w-full object-cover"
                />
                <video
                  v-else-if="att.type === 'video' && att.previewUrl"
                  :src="att.previewUrl"
                  muted
                  preload="metadata"
                  class="h-full w-full object-cover"
                />
                <div v-else class="flex h-full w-full items-center justify-center text-slate-400">
                  <Film v-if="att.type === 'video'" class="h-6 w-6" />
                  <ImagePlus v-else class="h-6 w-6" />
                </div>

                <span
                  v-if="att.type === 'video'"
                  class="absolute bottom-1 left-1 inline-flex items-center gap-1 rounded-md bg-black/60 px-1.5 py-0.5 text-[10px] font-bold text-white"
                >
                  <Film class="h-3 w-3" /> Video
                </span>

                <button
                  type="button"
                  @click="removeAttachment(idx)"
                  title="Bỏ file này"
                  class="absolute right-1 top-1 flex h-6 w-6 items-center justify-center rounded-full bg-black/60 text-white transition hover:bg-rose-600 cursor-pointer"
                >
                  <X class="h-3.5 w-3.5" />
                </button>
              </div>
            </div>

            <button
              v-else
              type="button"
              @click="openMediaPicker"
              :disabled="uploadingMedia"
              class="flex w-full flex-col items-center justify-center gap-1 rounded-xl border-2 border-dashed border-slate-200 bg-slate-50/40 py-5 text-xs text-slate-400 transition hover:border-emerald-300 hover:bg-emerald-50/40 hover:text-emerald-700 cursor-pointer"
            >
              <ImagePlus class="h-5 w-5" />
              <span>Bấm để chọn ảnh hoặc video gửi kèm (không bắt buộc)</span>
            </button>
          </div>

          <!-- Cấu hình nâng cao (Thu gọn mặc định cho thoáng) -->
          <div class="rounded-xl border border-slate-200/80 bg-slate-50/70">
            <button
              type="button"
              @click="showAdvanced = !showAdvanced"
              class="flex w-full items-center justify-between px-4 py-2.5 text-xs font-bold text-slate-700 cursor-pointer"
            >
              <span class="inline-flex items-center gap-2">
                <SlidersHorizontal class="h-3.5 w-3.5 text-emerald-600" />
                <span>Cấu hình giãn cách chống spam ({{ form.min_delay_sec }}–{{ form.max_delay_sec }}s / tin)</span>
              </span>
              <ChevronUp v-if="showAdvanced" class="h-4 w-4 text-slate-400" />
              <ChevronDown v-else class="h-4 w-4 text-slate-400" />
            </button>

            <div v-if="showAdvanced" class="grid grid-cols-2 gap-3 border-t border-slate-200/70 p-4 sm:grid-cols-4">
              <div>
                <label class="block text-[11px] font-semibold text-slate-600">Trễ tối thiểu (s)</label>
                <input
                  v-model.number="form.min_delay_sec"
                  type="number"
                  min="3"
                  class="mt-1 w-full rounded-lg border border-slate-200 bg-white px-2.5 py-1.5 text-sm !text-slate-900"
                />
              </div>
              <div>
                <label class="block text-[11px] font-semibold text-slate-600">Trễ tối đa (s)</label>
                <input
                  v-model.number="form.max_delay_sec"
                  type="number"
                  min="4"
                  class="mt-1 w-full rounded-lg border border-slate-200 bg-white px-2.5 py-1.5 text-sm !text-slate-900"
                />
              </div>
              <div>
                <label class="block text-[11px] font-semibold text-slate-600">Nghỉ sau mỗi (tin)</label>
                <input
                  v-model.number="form.batch_size"
                  type="number"
                  min="5"
                  class="mt-1 w-full rounded-lg border border-slate-200 bg-white px-2.5 py-1.5 text-sm !text-slate-900"
                />
              </div>
              <div>
                <label class="block text-[11px] font-semibold text-slate-600">Nghỉ lô (giây)</label>
                <input
                  v-model.number="form.batch_pause_sec"
                  type="number"
                  min="30"
                  class="mt-1 w-full rounded-lg border border-slate-200 bg-white px-2.5 py-1.5 text-sm !text-slate-900"
                />
              </div>
            </div>
          </div>

          <div class="flex flex-wrap items-center justify-between gap-3 pt-1">
            <span class="text-[11px] text-slate-400">
              Hệ thống tự xoay vòng mẫu câu & tự ngắt nếu chạm giới hạn Meta.
            </span>

            <button
              type="submit"
              :disabled="submitting || uploadingMedia || pages.length === 0"
              class="inline-flex items-center gap-2 rounded-xl bg-emerald-600 px-5 py-2.5 text-sm font-bold text-white shadow-xs transition hover:bg-emerald-700 disabled:opacity-60 cursor-pointer"
            >
              <Send class="h-4 w-4" />
              <span>{{ submitting ? 'Đang quét khách & tạo...' : 'Bắt đầu gửi tự động' }}</span>
            </button>
          </div>
        </form>

        <!-- Cột phải: Tiến độ chiến dịch -->
        <div class="overflow-hidden rounded-2xl border border-slate-200/80 bg-white text-left shadow-2xs lg:col-span-5">
          <div class="flex items-center justify-between border-b border-slate-100 px-5 py-4">
            <div>
              <div class="flex items-center gap-2">
                <h2 class="text-sm font-extrabold !text-slate-900">Tiến độ các chiến dịch</h2>
                <span
                  v-if="runningCount > 0"
                  class="rounded-full bg-emerald-100 px-2 py-0.5 text-[10px] font-bold text-emerald-800"
                >
                  {{ runningCount }} đang chạy
                </span>
              </div>
              <p class="text-[11px] text-slate-400">Tự động cập nhật mỗi 5 giây</p>
            </div>

            <button
              type="button"
              @click="fetchData()"
              class="inline-flex items-center gap-1 rounded-lg border border-slate-200 px-2.5 py-1.5 text-xs font-semibold text-slate-600 transition hover:bg-slate-50 cursor-pointer"
            >
              <RefreshCw class="h-3.5 w-3.5" />
              <span>Làm mới</span>
            </button>
          </div>

          <div v-if="loading" class="p-8 text-center text-xs text-slate-400">
            Đang tải danh sách chiến dịch...
          </div>

          <div v-else-if="campaigns.length === 0" class="p-8 text-center space-y-2">
            <div class="mx-auto flex h-10 w-10 items-center justify-center rounded-full bg-slate-100 text-slate-400">
              <Users class="h-5 w-5" />
            </div>
            <p class="text-xs font-semibold text-slate-600">Chưa có chiến dịch nào</p>
            <p class="text-[11px] text-slate-400">Tạo chiến dịch bên trái để bắt đầu nhắn tin tự động.</p>
          </div>

          <div v-else class="divide-y divide-slate-100 max-h-[640px] overflow-y-auto">
            <div v-for="c in campaigns" :key="c.id" class="p-4 sm:p-5 space-y-3">
              <div class="flex items-start justify-between gap-3">
                <div class="min-w-0 space-y-1">
                  <div class="flex flex-wrap items-center gap-2">
                    <span class="truncate text-sm font-bold !text-slate-900">{{ c.name }}</span>
                    <span
                      class="rounded-full px-2 py-0.5 text-[10px] font-bold uppercase"
                      :class="{
                        'bg-emerald-50 text-emerald-700 ring-1 ring-emerald-200': c.status === 'running',
                        'bg-amber-50 text-amber-700 ring-1 ring-amber-200': c.status === 'paused',
                        'bg-slate-100 text-slate-600': c.status === 'completed'
                      }"
                    >
                      {{ c.status === 'running' ? 'Đang gửi' : c.status === 'paused' ? 'Tạm dừng' : 'Hoàn tất' }}
                    </span>
                  </div>
                  <p class="truncate text-xs font-medium text-emerald-700">
                    {{ getPageNameById(c.page_id) }}
                  </p>
                  <p v-if="c.attachments && c.attachments.length > 0" class="flex items-center gap-2 text-[11px] text-slate-500">
                    <span v-if="countMedia(c, 'image') > 0" class="inline-flex items-center gap-1">
                      <ImagePlus class="h-3 w-3" /> {{ countMedia(c, 'image') }} ảnh
                    </span>
                    <span v-if="countMedia(c, 'video') > 0" class="inline-flex items-center gap-1">
                      <Film class="h-3 w-3" /> {{ countMedia(c, 'video') }} video
                    </span>
                  </p>
                </div>

                <button
                  v-if="c.status === 'running' || c.status === 'paused'"
                  type="button"
                  @click="toggleStatus(c)"
                  class="inline-flex shrink-0 items-center gap-1 rounded-xl border border-slate-200 bg-white px-2.5 py-1.5 text-xs font-semibold text-slate-700 transition hover:bg-slate-50 cursor-pointer"
                >
                  <Pause v-if="c.status === 'running'" class="h-3.5 w-3.5 text-amber-600" />
                  <Play v-else class="h-3.5 w-3.5 text-emerald-600" />
                  <span>{{ c.status === 'running' ? 'Dừng' : 'Chạy tiếp' }}</span>
                </button>
              </div>

              <!-- Thanh tiến độ trực quan -->
              <div class="space-y-1.5">
                <div class="flex items-center justify-between text-[11px] text-slate-500">
                  <span>{{ formatTargetMode(c.target_mode) }}</span>
                  <span>
                    Đã gửi: <b class="text-emerald-700">{{ c.sent_count }}</b>/{{ c.total_recipients }}
                    <template v-if="c.failed_count > 0"> • Lỗi: <b class="text-rose-600">{{ c.failed_count }}</b></template>
                  </span>
                </div>
                <div class="h-1.5 w-full overflow-hidden rounded-full bg-slate-100">
                  <div
                    class="h-full rounded-full bg-emerald-600 transition-all duration-300"
                    :style="{ width: getProgressPercent(c) + '%' }"
                  ></div>
                </div>
              </div>

              <p v-if="c.pause_reason" class="rounded-lg bg-amber-50 px-2.5 py-1.5 text-[11px] font-medium text-amber-800">
                {{ c.pause_reason }}
              </p>
            </div>
          </div>
        </div>
      </div>
    </main>
  </div>
</template>