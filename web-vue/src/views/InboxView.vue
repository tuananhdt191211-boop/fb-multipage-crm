<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import {
  ArrowLeft,
  Bot,
  Check,
  CheckCircle2,
  Copy,
  CornerUpLeft,
  Facebook,
  Hash,
  ImagePlus,
  Layers,
  Loader2,
  MessageSquare,
  Plus,
  RefreshCw,
  Save,
  Search,
  Send,
  ShieldCheck,
  Sparkles,
  Tag,
  Trash2,
  X,
  Zap
} from 'lucide-vue-next'
import { api } from '../lib/api'
import type { FacebookPageItem } from '../types/page'

interface ConversationItem {
  id: number
  page_id: string
  page_name: string
  thread_id: string
  customer_psid: string
  customer_name: string
  customer_avatar: string
  last_message: string
  last_message_time: string
  unread_count: number
}

interface MessageItem {
  id: number
  conversation_id: number
  fb_message_id: string
  sender_type: 'customer' | 'page'
  sender_name: string
  content: string
  attachment_url?: string
  attachment_type?: string
  sent_at: string
}

interface QuickReplyItem {
  id: string
  title: string
  category: 'Báo giá' | 'Chốt đơn' | 'Chính sách' | 'Chào hỏi'
  keywords: string
  autoTrigger: boolean
  content: string
  images?: string[]
}

interface PageSettingMeta {
  page_id: string
  auto_reply: boolean
  product_tag: string
  tag_color: string
  quick_replies: string
  tone: string
  system_prompt: string
}

interface NewMessageAlert {
  id: number
  convId: number
  customerName: string
  pageName: string
  message: string
}

const router = useRouter()

const pages = ref<FacebookPageItem[]>([])
const conversations = ref<ConversationItem[]>([])
const messages = ref<MessageItem[]>([])
const selectedConv = ref<ConversationItem | null>(null)
const pageSettingsMap = ref<Record<string, PageSettingMeta>>({})

const selectedPageFilter = ref('all')
const readFilter = ref<'all' | 'unread'>('all')
const searchQuery = ref('')
const draftMessage = ref('')

// Chức năng mới: Chọn tin nhắn cụ thể + Chỉ đạo AI + Khung duyệt trước khi gửi
const selectedTargetMsg = ref<MessageItem | null>(null)
const aiCustomInstruction = ref('')
const aiAskBeforeSend = ref(true)
const aiApprovalDraft = ref('')

// Hình ảnh đính kèm trong khung chat (chọn nhiều file hoặc dán Ctrl+V)
const pendingImages = ref<string[]>([])
const uploadingImages = ref(false)
const chatFileInputRef = ref<HTMLInputElement | null>(null)

// Hình ảnh đính kèm khi tạo / sửa Tin nhắn mẫu
const newReplyImages = ref<string[]>([])
const uploadingTemplateImages = ref(false)
const templateFileInputRef = ref<HTMLInputElement | null>(null)

const loadingList = ref(false)
const loadingMessages = ref(false)
const syncing = ref(false)
const sending = ref(false)
const copiedField = ref('')
const noticeMsg = ref('')
const errorMsg = ref('')
const fbInboxFallbackUrl = ref('')
const chatBoxRef = ref<HTMLElement | null>(null)
const generatingScript = ref(false)

// Trạng thái AI Co-pilot, Nhãn sản phẩm & Thư viện tin nhắn mẫu từng Fanpage
const generatingAI = ref(false)
const loadingAISetting = ref(false)
const savingAISetting = ref(false)
const autoSaveStatus = ref<'idle' | 'saving' | 'saved'>('idle')
const applyingToAllPages = ref(false)
const editingReplyId = ref<string | null>(null)

const aiAutoReply = ref(false)
const pageProductTag = ref('')
const pageTagColor = ref('emerald')
const pageQuickReplies = ref<QuickReplyItem[]>([])
const quickReplyCategoryFilter = ref<string>('all')
const quickReplySearch = ref('')

const newReplyTitle = ref('')
const newReplyCategory = ref<QuickReplyItem['category']>('Báo giá')
const newReplyKeywords = ref('')
const newReplyAutoTrigger = ref(false)
const newReplyContent = ref('')

const aiTone = ref('')
const aiPrompt = ref('')

const newMsgAlert = ref<NewMessageAlert | null>(null)
const hasInitializedInbox = ref(false)
const prevLastMsgMap = ref<Record<number, string>>({})
let alertHideTimer: number | null = null
let autoSaveDebounceTimer: number | null = null
let pollTimer: number | null = null

const brokenAvatars = ref<Record<string, boolean>>({})

const tagColorOptions = [
  { id: 'emerald', label: 'Xanh lá', badge: 'bg-emerald-100 text-emerald-800 border-emerald-200', dot: 'bg-emerald-500' },
  { id: 'blue', label: 'Xanh dương', badge: 'bg-blue-100 text-blue-800 border-blue-200', dot: 'bg-blue-500' },
  { id: 'amber', label: 'Vàng cam', badge: 'bg-amber-100 text-amber-900 border-amber-200', dot: 'bg-amber-500' },
  { id: 'rose', label: 'Đỏ hồng', badge: 'bg-rose-100 text-rose-800 border-rose-200', dot: 'bg-rose-500' },
  { id: 'purple', label: 'Tím', badge: 'bg-purple-100 text-purple-800 border-purple-200', dot: 'bg-purple-500' },
  { id: 'slate', label: 'Xám đậm', badge: 'bg-slate-200 text-slate-800 border-slate-300', dot: 'bg-slate-600' }
]

const quickInstructionPresets = [
  'Báo giá ưu đãi & miễn phí ship',
  'Xin SĐT + địa chỉ lên đơn',
  'Hỏi nhu cầu sử dụng của khách',
  'Tư vấn chính sách bảo hành 12 tháng'
]

// Trạng thái bật/tắt nhanh Auto-Rep bằng Mẫu từ khóa trên Page hiện tại
const isAnyTemplateAutoEnabled = computed(() =>
  pageQuickReplies.value.some((qr) => qr.autoTrigger)
)

async function togglePageAIAutoReply() {
  if (!selectedConv.value) return
  aiAutoReply.value = !aiAutoReply.value
  await persistPageConfigSilently()
  noticeMsg.value = aiAutoReply.value
    ? `Đã BẬT AI tự động trả lời sau 2 phút cho ${selectedConv.value.page_name}`
    : `Đã TẮT AI tự động trả lời cho ${selectedConv.value.page_name}`
  setTimeout(() => {
    noticeMsg.value = ''
  }, 2500)
}

async function toggleAllPageTemplatesAuto() {
  if (!selectedConv.value) return
  if (pageQuickReplies.value.length === 0) {
    errorMsg.value = 'Fanpage này chưa có tin nhắn mẫu nào. Hãy thêm mẫu ở cột bên phải trước!'
    return
  }
  const nextState = !isAnyTemplateAutoEnabled.value
  for (const qr of pageQuickReplies.value) {
    qr.autoTrigger = nextState
  }
  await persistPageConfigSilently()
  noticeMsg.value = nextState
    ? `Đã BẬT tự động trả lời bằng mẫu (${pageQuickReplies.value.length} mẫu) cho ${selectedConv.value.page_name}`
    : `Đã TẮT tự động trả lời bằng mẫu cho ${selectedConv.value.page_name}`
  setTimeout(() => {
    noticeMsg.value = ''
  }, 2500)
}

function getBadgeStyle(colorId?: string) {
  return tagColorOptions.find((c) => c.id === colorId) || tagColorOptions[0]
}

const pageMap = computed(() => {
  const map: Record<string, FacebookPageItem> = {}
  for (const p of pages.value) {
    map[p.page_id] = p
  }
  return map
})

function resolveMediaUrl(raw?: string): string {
  if (!raw) return ''
  if (raw.startsWith('/uploads/')) {
    const base = api.defaults.baseURL || ''
    if (base.startsWith('http://') || base.startsWith('https://')) {
      try {
        const origin = new URL(base).origin
        return `${origin}${raw}`
      } catch {
        return raw
      }
    }
  }
  return raw
}

function personalizeTemplate(raw: string, conv: ConversationItem | null): string {
  const custName = conv?.customer_name || 'anh/chị'
  const pgName = conv?.page_name || 'Shop'
  return raw
    .replace(/\{customer_name\}/gi, custName)
    .replace(/\{page_name\}/gi, pgName)
}

function parseQuickReplies(raw?: string): QuickReplyItem[] {
  if (!raw) return []
  try {
    const arr = JSON.parse(raw)
    if (!Array.isArray(arr)) return []
    return arr.map((item: any, idx: number) => ({
      id: String(item.id || `qr_${idx}_${Date.now()}`),
      title: String(item.title || `Mẫu ${idx + 1}`),
      category: (['Báo giá', 'Chốt đơn', 'Chính sách', 'Chào hỏi'].includes(item.category)
        ? item.category
        : 'Báo giá') as QuickReplyItem['category'],
      keywords: String(item.keywords || ''),
      autoTrigger: Boolean(item.autoTrigger),
      content: String(item.content || ''),
      images: Array.isArray(item.images) ? item.images.map((u: any) => String(u)).filter(Boolean) : []
    }))
  } catch {
    return []
  }
}

const filteredQuickReplies = computed(() => {
  const q = quickReplySearch.value.trim().toLowerCase()
  return pageQuickReplies.value.filter((qr) => {
    const matchCat =
      quickReplyCategoryFilter.value === 'all' || qr.category === quickReplyCategoryFilter.value
    const matchText =
      !q ||
      qr.title.toLowerCase().includes(q) ||
      qr.content.toLowerCase().includes(q) ||
      qr.keywords.toLowerCase().includes(q)
    return matchCat && matchText
  })
})

async function uploadFilesToServer(files: File[]): Promise<string[]> {
  const imageFiles = files.filter((f) => f.type.startsWith('image/'))
  if (imageFiles.length === 0) return []

  const formData = new FormData()
  for (const f of imageFiles) {
    formData.append('images', f)
  }

  const { data } = await api.post<{ urls: string[] }>('/inbox/upload', formData, {
    headers: { 'Content-Type': 'multipart/form-data' }
  })
  return data.urls || []
}

async function handleChatPaste(e: ClipboardEvent) {
  const items = e.clipboardData?.items
  if (!items) return

  const pastedFiles: File[] = []
  for (const item of Array.from(items)) {
    if (item.kind === 'file' && item.type.startsWith('image/')) {
      const f = item.getAsFile()
      if (f) pastedFiles.push(f)
    }
  }
  if (pastedFiles.length === 0) return

  e.preventDefault()
  uploadingImages.value = true
  errorMsg.value = ''
  try {
    const urls = await uploadFilesToServer(pastedFiles)
    pendingImages.value.push(...urls)
  } catch (err: any) {
    errorMsg.value = err.response?.data?.error || 'Không thể tải ảnh vừa dán.'
  } finally {
    uploadingImages.value = false
  }
}

async function handleChatFilesSelected(e: Event) {
  const input = e.target as HTMLInputElement
  if (!input.files?.length) return
  uploadingImages.value = true
  errorMsg.value = ''
  try {
    const urls = await uploadFilesToServer(Array.from(input.files))
    pendingImages.value.push(...urls)
  } catch (err: any) {
    errorMsg.value = err.response?.data?.error || 'Tải ảnh thất bại.'
  } finally {
    uploadingImages.value = false
    input.value = ''
  }
}

async function handleTemplateFilesSelected(e: Event) {
  const input = e.target as HTMLInputElement
  if (!input.files?.length) return
  uploadingTemplateImages.value = true
  errorMsg.value = ''
  try {
    const urls = await uploadFilesToServer(Array.from(input.files))
    newReplyImages.value.push(...urls)
  } catch (err: any) {
    errorMsg.value = err.response?.data?.error || 'Tải ảnh mẫu thất bại.'
  } finally {
    uploadingTemplateImages.value = false
    input.value = ''
  }
}

async function handleTemplatePaste(e: ClipboardEvent) {
  const items = e.clipboardData?.items
  if (!items) return

  const pastedFiles: File[] = []
  for (const item of Array.from(items)) {
    if (item.kind === 'file' && item.type.startsWith('image/')) {
      const f = item.getAsFile()
      if (f) pastedFiles.push(f)
    }
  }
  if (pastedFiles.length === 0) return

  e.preventDefault()
  uploadingTemplateImages.value = true
  errorMsg.value = ''
  try {
    const urls = await uploadFilesToServer(pastedFiles)
    newReplyImages.value.push(...urls)
  } catch (err: any) {
    errorMsg.value = err.response?.data?.error || 'Không thể tải ảnh vừa dán cho mẫu.'
  } finally {
    uploadingTemplateImages.value = false
  }
}

async function handleEditTemplatePaste(e: ClipboardEvent, qr: QuickReplyItem) {
  const items = e.clipboardData?.items
  if (!items) return

  const pastedFiles: File[] = []
  for (const item of Array.from(items)) {
    if (item.kind === 'file' && item.type.startsWith('image/')) {
      const f = item.getAsFile()
      if (f) pastedFiles.push(f)
    }
  }
  if (pastedFiles.length === 0) return

  e.preventDefault()
  try {
    const urls = await uploadFilesToServer(pastedFiles)
    if (!qr.images) qr.images = []
    qr.images.push(...urls)
    scheduleSilentSave()
  } catch {
    errorMsg.value = 'Không thể dán thêm ảnh vào mẫu.'
  }
}

async function handleEditTemplateFilesSelected(e: Event, qr: QuickReplyItem) {
  const input = e.target as HTMLInputElement
  if (!input.files?.length) return
  try {
    const urls = await uploadFilesToServer(Array.from(input.files))
    if (!qr.images) qr.images = []
    qr.images.push(...urls)
    scheduleSilentSave()
  } catch {
    errorMsg.value = 'Không thể thêm ảnh vào mẫu.'
  } finally {
    input.value = ''
  }
}

async function pollInboxUpdates() {
  if (!localStorage.getItem('access_token')) return
  if (syncing.value || sending.value || loadingList.value) return

  try {
    const { data } = await api.get<{
      conversations: ConversationItem[]
      pages: FacebookPageItem[]
    }>('/inbox/conversations')
    const incoming = data.conversations || []

    if (!hasInitializedInbox.value) {
      const snapshot: Record<number, string> = {}
      for (const c of incoming) {
        snapshot[c.id] = `${c.last_message_time}|${c.last_message}`
      }
      prevLastMsgMap.value = snapshot
      hasInitializedInbox.value = true
    } else {
      let alertedConv: ConversationItem | null = null
      for (const c of incoming) {
        const sig = `${c.last_message_time}|${c.last_message}`
        const prevSig = prevLastMsgMap.value[c.id]
        if (((prevSig && prevSig !== sig) || !prevSig) && c.unread_count > 0) {
          alertedConv = c
          break
        }
      }

      const nextSnapshot: Record<number, string> = {}
      for (const c of incoming) {
        nextSnapshot[c.id] = `${c.last_message_time}|${c.last_message}`
      }
      prevLastMsgMap.value = nextSnapshot

      if (alertedConv) {
        triggerRepNotify(alertedConv)
      }
    }

    conversations.value = incoming
    const unreadSum = incoming.reduce((sum, item) => sum + (item.unread_count || 0), 0)
    document.title = unreadSum > 0 ? `(${unreadSum}) Tin nhắn mới - PagePilot CRM` : 'Hộp thư gộp đa Fanpage - PagePilot CRM'

    if (data.pages?.length) {
      pages.value = data.pages
    }

    if (selectedConv.value) {
      const matched = incoming.find((c) => c.id === selectedConv.value?.id)
      if (matched) {
        const changed =
          matched.last_message !== selectedConv.value.last_message ||
          matched.last_message_time !== selectedConv.value.last_message_time
        selectedConv.value = matched
        if (changed && !loadingMessages.value) {
          const msgRes = await api.get<{
            conversation: ConversationItem
            messages: MessageItem[]
          }>(`/inbox/conversations/${matched.id}/messages`)
          messages.value = msgRes.data.messages || []
          await scrollToBottom()
        }
      }
    }
  } catch {
    // Bỏ qua lỗi nền tạm thời
  }
}

function playNewMessageSound() {
  try {
    const AudioCtx = window.AudioContext || (window as unknown as { webkitAudioContext: typeof AudioContext }).webkitAudioContext
    if (!AudioCtx) return
    const ctx = new AudioCtx()
    const osc = ctx.createOscillator()
    const gain = ctx.createGain()
    osc.type = 'sine'
    osc.frequency.setValueAtTime(880, ctx.currentTime)
    gain.gain.setValueAtTime(0.08, ctx.currentTime)
    gain.gain.exponentialRampToValueAtTime(0.001, ctx.currentTime + 0.25)
    osc.connect(gain)
    gain.connect(ctx.destination)
    osc.start()
    osc.stop(ctx.currentTime + 0.25)
  } catch {
    // ignore
  }
}

function triggerRepNotify(conv: ConversationItem) {
  playNewMessageSound()

  if (alertHideTimer !== null) {
    clearTimeout(alertHideTimer)
  }

  newMsgAlert.value = {
    id: Date.now(),
    convId: conv.id,
    customerName: conv.customer_name || 'Khách hàng',
    pageName: conv.page_name || 'Fanpage',
    message: conv.last_message || 'Có tin nhắn mới cần phản hồi'
  }

  alertHideTimer = window.setTimeout(() => {
    newMsgAlert.value = null
  }, 8000)

  if ('Notification' in window && Notification.permission === 'granted' && document.visibilityState !== 'visible') {
    try {
      const n = new Notification(`Tin nhắn mới từ ${conv.customer_name} (${conv.page_name})`, {
        body: conv.last_message || 'Bấm để mở hội thoại và rep khách ngay',
        tag: `conv_${conv.id}`
      })
      n.onclick = () => {
        window.focus()
        selectConversation(conv)
        n.close()
      }
    } catch {
      // ignore
    }
  }
}

function isOver24Hours(iso?: string) {
  if (!iso) return false
  const diffMs = Date.now() - new Date(iso).getTime()
  return diffMs > 24 * 60 * 60 * 1000
}

function getMetaBusinessInboxUrl(conv: ConversationItem) {
  return `https://business.facebook.com/latest/inbox/all?asset_id=${conv.page_id}&selected_item_id=${conv.customer_psid}`
}

const totalUnread = computed(() =>
  conversations.value.reduce((sum, c) => sum + (c.unread_count || 0), 0)
)

const filteredConversations = computed(() => {
  const q = searchQuery.value.trim().toLowerCase()
  return conversations.value.filter((c) => {
    const matchPage = selectedPageFilter.value === 'all' || c.page_id === selectedPageFilter.value
    const matchUnread = readFilter.value === 'all' || c.unread_count > 0
    const tag = (pageSettingsMap.value[c.page_id]?.product_tag || '').toLowerCase()
    const matchText =
      !q ||
      c.customer_name.toLowerCase().includes(q) ||
      c.last_message.toLowerCase().includes(q) ||
      c.page_name.toLowerCase().includes(q) ||
      tag.includes(q)
    return matchPage && matchUnread && matchText
  })
})

function handleAvatarError(key: string) {
  brokenAvatars.value[key] = true
}

function getCustomerAvatarUrl(conv: ConversationItem) {
  const baseURL = (api.defaults.baseURL || '/api/v1').replace(/\/$/, '')
  return `${baseURL}/inbox/avatar/${encodeURIComponent(conv.page_id)}/${encodeURIComponent(conv.customer_psid)}`
}

function getInitials(name: string) {
  if (!name) return 'K'
  const parts = name.trim().split(/\s+/)
  if (parts.length === 1) return parts[0].charAt(0).toUpperCase()
  return (parts[0].charAt(0) + parts[parts.length - 1].charAt(0)).toUpperCase()
}

function formatTime(iso: string) {
  if (!iso) return ''
  const d = new Date(iso)
  const now = new Date()
  const isToday = d.toDateString() === now.toDateString()
  if (isToday) {
    return d.toLocaleTimeString('vi-VN', { hour: '2-digit', minute: '2-digit' })
  }
  return d.toLocaleDateString('vi-VN', { day: '2-digit', month: '2-digit' })
}

function formatFullTime(iso: string) {
  if (!iso) return ''
  const d = new Date(iso)
  return d.toLocaleTimeString('vi-VN', { hour: '2-digit', minute: '2-digit' })
}

async function copyText(text: string, label: string) {
  try {
    await navigator.clipboard.writeText(text)
    copiedField.value = label
    setTimeout(() => {
      if (copiedField.value === label) copiedField.value = ''
    }, 1800)
  } catch {
    // ignore
  }
}

async function scrollToBottom() {
  await nextTick()
  if (chatBoxRef.value) {
    chatBoxRef.value.scrollTop = chatBoxRef.value.scrollHeight
  }
}

async function loadAllPageSettings() {
  if (!localStorage.getItem('access_token')) return
  try {
    const { data } = await api.get<{ settings: PageSettingMeta[] }>('/ai/settings')
    const map: Record<string, PageSettingMeta> = {}
    for (const s of data.settings || []) {
      map[s.page_id] = s
    }
    pageSettingsMap.value = map
  } catch {
    // ignore
  }
}

async function loadPageAISetting(pageId: string) {
  if (!localStorage.getItem('access_token')) return

  editingReplyId.value = null
  const cached = pageSettingsMap.value[pageId]
  if (cached) {
    aiAutoReply.value = Boolean(cached.auto_reply)
    pageProductTag.value = cached.product_tag || ''
    pageTagColor.value = cached.tag_color || 'emerald'
    pageQuickReplies.value = parseQuickReplies(cached.quick_replies)
    aiTone.value = cached.tone || ''
    aiPrompt.value = cached.system_prompt || ''
  }

  loadingAISetting.value = true
  try {
    const { data } = await api.get<{ setting: PageSettingMeta }>(`/ai/settings/${pageId}`)
    const st = data.setting
    if (st) {
      pageSettingsMap.value[pageId] = st
    }
    aiAutoReply.value = Boolean(st?.auto_reply)
    pageProductTag.value = st?.product_tag || ''
    pageTagColor.value = st?.tag_color || 'emerald'
    pageQuickReplies.value = parseQuickReplies(st?.quick_replies)
    quickReplyCategoryFilter.value = 'all'
    quickReplySearch.value = ''
    aiTone.value = st?.tone || ''
    aiPrompt.value = st?.system_prompt || ''
  } catch {
    if (!cached) {
      aiAutoReply.value = false
      pageProductTag.value = ''
      pageTagColor.value = 'emerald'
      pageQuickReplies.value = []
      aiTone.value = ''
      aiPrompt.value = ''
    }
  } finally {
    loadingAISetting.value = false
  }
}

async function persistPageConfigSilently(targetPageId?: string) {
  const pageId = targetPageId || selectedConv.value?.page_id
  if (!pageId) return

  autoSaveStatus.value = 'saving'
  try {
    const quickRepliesJSON = JSON.stringify(pageQuickReplies.value)
    const { data } = await api.put<{ message: string; setting: PageSettingMeta }>(
      `/ai/settings/${pageId}`,
      {
        enabled: true,
        auto_reply: aiAutoReply.value,
        product_tag: pageProductTag.value,
        tag_color: pageTagColor.value,
        quick_replies: quickRepliesJSON,
        tone: aiTone.value,
        system_prompt: aiPrompt.value
      }
    )
    if (data.setting) {
      pageSettingsMap.value[pageId] = data.setting
    }
    autoSaveStatus.value = 'saved'
    window.setTimeout(() => {
      if (autoSaveStatus.value === 'saved') autoSaveStatus.value = 'idle'
    }, 2200)
  } catch (err: any) {
    autoSaveStatus.value = 'idle'
    if (err.response?.status !== 401) {
      errorMsg.value = err.response?.data?.error || 'Không thể tự động lưu cấu hình.'
    }
  }
}

function scheduleSilentSave() {
  if (autoSaveDebounceTimer !== null) {
    clearTimeout(autoSaveDebounceTimer)
  }
  autoSaveDebounceTimer = window.setTimeout(() => {
    persistPageConfigSilently()
  }, 350)
}

async function addQuickReply() {
  const title = newReplyTitle.value.trim()
  const content = newReplyContent.value.trim()
  if (!title || (!content && newReplyImages.value.length === 0)) return

  pageQuickReplies.value.push({
    id: `qr_${Date.now()}_${Math.random().toString(36).slice(2, 6)}`,
    title,
    category: newReplyCategory.value,
    keywords: newReplyKeywords.value.trim(),
    autoTrigger: newReplyAutoTrigger.value,
    content,
    images: [...newReplyImages.value]
  })

  newReplyTitle.value = ''
  newReplyKeywords.value = ''
  newReplyAutoTrigger.value = false
  newReplyContent.value = ''
  newReplyImages.value = []

  await persistPageConfigSilently()
}

async function removeQuickReply(index: number) {
  pageQuickReplies.value.splice(index, 1)
  await persistPageConfigSilently()
}

async function toggleQuickReplyAutoTrigger() {
  await persistPageConfigSilently()
}

async function applyTemplatesToOtherPages(onlySameTag: boolean) {
  if (!selectedConv.value || pageQuickReplies.value.length === 0 || applyingToAllPages.value) return
  const currentPageId = selectedConv.value.page_id
  const currentTag = pageProductTag.value.trim().toLowerCase()

  const targets = pages.value.filter((p) => {
    if (p.page_id === currentPageId) return false
    if (onlySameTag) {
      const pTag = (pageSettingsMap.value[p.page_id]?.product_tag || '').trim().toLowerCase()
      return currentTag !== '' && pTag === currentTag
    }
    return true
  })

  if (targets.length === 0) {
    noticeMsg.value = onlySameTag
      ? 'Không có Fanpage nào khác đang gắn cùng nhãn sản phẩm này.'
      : 'Không có Fanpage nào khác để đồng bộ.'
    setTimeout(() => (noticeMsg.value = ''), 3000)
    return
  }

  applyingToAllPages.value = true
  errorMsg.value = ''
  try {
    const quickRepliesJSON = JSON.stringify(pageQuickReplies.value)
    await Promise.all(
      targets.map(async (p) => {
        const existing = pageSettingsMap.value[p.page_id]
        const { data } = await api.put<{ setting: PageSettingMeta }>(`/ai/settings/${p.page_id}`, {
          enabled: true,
          auto_reply: existing?.auto_reply ?? aiAutoReply.value,
          product_tag: existing?.product_tag || pageProductTag.value,
          tag_color: existing?.tag_color || pageTagColor.value,
          quick_replies: quickRepliesJSON,
          tone: existing?.tone || aiTone.value,
          system_prompt: existing?.system_prompt || aiPrompt.value
        })
        if (data.setting) {
          pageSettingsMap.value[p.page_id] = data.setting
        }
      })
    )
    noticeMsg.value = `Đã đồng bộ ${pageQuickReplies.value.length} mẫu & Auto-Rep sang ${targets.length} Fanpage!`
    setTimeout(() => (noticeMsg.value = ''), 4000)
  } catch {
    errorMsg.value = 'Có lỗi khi đồng bộ mẫu sang các Fanpage khác.'
  } finally {
    applyingToAllPages.value = false
  }
}

function applyQuickReply(qr: QuickReplyItem) {
  if (qr.content) {
    draftMessage.value = personalizeTemplate(qr.content, selectedConv.value)
  }
  if (qr.images && qr.images.length > 0) {
    pendingImages.value = [...qr.images]
  }
}

async function savePageAISetting() {
  if (!selectedConv.value || savingAISetting.value) return
  savingAISetting.value = true
  errorMsg.value = ''
  try {
    const quickRepliesJSON = JSON.stringify(pageQuickReplies.value)
    const { data } = await api.put<{ message: string; setting: PageSettingMeta }>(
      `/ai/settings/${selectedConv.value.page_id}`,
      {
        enabled: true,
        auto_reply: aiAutoReply.value,
        product_tag: pageProductTag.value,
        tag_color: pageTagColor.value,
        quick_replies: quickRepliesJSON,
        tone: aiTone.value,
        system_prompt: aiPrompt.value
      }
    )
    if (data.setting) {
      pageSettingsMap.value[selectedConv.value.page_id] = data.setting
    }
    noticeMsg.value = data.message || 'Đã lưu kịch bản AI & mẫu tin nhắn cho Fanpage!'
    setTimeout(() => {
      noticeMsg.value = ''
    }, 3000)
  } catch (err: any) {
    if (err.response?.status === 401) return
    errorMsg.value = err.response?.data?.error || 'Lưu kịch bản AI thất bại.'
  } finally {
    savingAISetting.value = false
  }
}

function toggleSelectMessageForAI(msg: MessageItem) {
  if (selectedTargetMsg.value?.id === msg.id) {
    selectedTargetMsg.value = null
  } else {
    selectedTargetMsg.value = msg
  }
}

async function handleAISuggest() {
  if (!selectedConv.value || generatingAI.value) return
  generatingAI.value = true
  errorMsg.value = ''
  try {
    const { data } = await api.post<{ reply: string }>(
      `/ai/suggest/${selectedConv.value.id}`,
      {
        selected_message: selectedTargetMsg.value?.content || '',
        instruction: aiCustomInstruction.value.trim()
      }
    )

    if (aiAskBeforeSend.value) {
      // Hỏi duyệt trước: hiển thị hộp duyệt để nhân viên xem trước khi gửi
      aiApprovalDraft.value = data.reply
    } else {
      draftMessage.value = data.reply
    }
  } catch (err: any) {
    if (err.response?.status === 401) return
    errorMsg.value = err.response?.data?.error || 'AI không tạo được câu trả lời gợi ý.'
  } finally {
    generatingAI.value = false
  }
}

async function approveAndSendAIReply() {
  if (!aiApprovalDraft.value.trim()) return
  draftMessage.value = aiApprovalDraft.value.trim()
  aiApprovalDraft.value = ''
  selectedTargetMsg.value = null
  aiCustomInstruction.value = ''
  await handleSendMessage()
}

function moveAIDraftToEditor() {
  if (!aiApprovalDraft.value.trim()) return
  draftMessage.value = aiApprovalDraft.value.trim()
  aiApprovalDraft.value = ''
}

async function handleGenerateSalesScript() {
  if (!selectedConv.value || generatingScript.value) return
  const product = pageProductTag.value.trim()
  if (!product) {
    errorMsg.value = 'Vui lòng nhập tên sản phẩm vào ô "Nhãn nhận diện sản phẩm" ở trên trước!'
    return
  }

  generatingScript.value = true
  errorMsg.value = ''
  try {
    const { data } = await api.post<{
      message: string
      tone: string
      system_prompt: string
      quick_replies: QuickReplyItem[]
    }>('/ai/generate-script', {
      page_name: selectedConv.value.page_name,
      product_name: product,
      extra_notes: aiPrompt.value
    })

    if (data.tone) aiTone.value = data.tone
    if (data.system_prompt) aiPrompt.value = data.system_prompt
    if (Array.isArray(data.quick_replies) && data.quick_replies.length > 0) {
      pageQuickReplies.value = [...pageQuickReplies.value, ...data.quick_replies]
    }
    aiAutoReply.value = true
    await persistPageConfigSilently()
    noticeMsg.value = data.message || 'Đã tạo & tự động lưu kịch bản Sale cùng 4 mẫu tin nhắn!'
    setTimeout(() => {
      noticeMsg.value = ''
    }, 4500)
  } catch (err: any) {
    if (err.response?.status === 401) return
    errorMsg.value = err.response?.data?.error || 'AI không tạo được kịch bản Sale.'
  } finally {
    generatingScript.value = false
  }
}

async function loadConversations() {
  if (!localStorage.getItem('access_token')) {
    router.replace('/login')
    return
  }
  loadingList.value = true
  errorMsg.value = ''
  try {
    const [convRes] = await Promise.all([
      api.get<{ conversations: ConversationItem[]; pages: FacebookPageItem[] }>('/inbox/conversations'),
      loadAllPageSettings()
    ])
    conversations.value = convRes.data.conversations || []
    pages.value = convRes.data.pages || []
    if (selectedConv.value) {
      const updated = conversations.value.find((c) => c.id === selectedConv.value?.id)
      if (updated) selectedConv.value = updated
    }
  } catch (err: any) {
    if (err.response?.status === 401) return
    errorMsg.value = err.response?.data?.error || 'Không thể tải danh sách hội thoại.'
  } finally {
    loadingList.value = false
  }
}

async function handleSyncMessages() {
  syncing.value = true
  noticeMsg.value = ''
  errorMsg.value = ''
  try {
    const syncEndpoint =
      selectedPageFilter.value && selectedPageFilter.value !== 'all'
        ? `/inbox/sync?page_id=${encodeURIComponent(selectedPageFilter.value)}`
        : '/inbox/sync'
    const { data } = await api.post<{ message: string }>(syncEndpoint)
    noticeMsg.value = data.message
    brokenAvatars.value = {}
    await loadConversations()
    if (selectedConv.value) {
      await selectConversation(selectedConv.value)
    }
    setTimeout(() => {
      noticeMsg.value = ''
    }, 4000)
  } catch (err: any) {
    if (err.response?.status === 401) return
    errorMsg.value = err.response?.data?.error || 'Đồng bộ tin nhắn thất bại.'
  } finally {
    syncing.value = false
  }
}

async function selectConversation(conv: ConversationItem) {
  const isDifferentPage = selectedConv.value?.page_id !== conv.page_id
  selectedConv.value = conv
  pendingImages.value = []
  selectedTargetMsg.value = null
  aiApprovalDraft.value = ''
  loadingMessages.value = true
  errorMsg.value = ''
  fbInboxFallbackUrl.value = ''

  if (isDifferentPage) {
    loadPageAISetting(conv.page_id)
  }

  try {
    const { data } = await api.get<{ conversation: ConversationItem; messages: MessageItem[] }>(
      `/inbox/conversations/${conv.id}/messages`
    )
    messages.value = data.messages || []
    const idx = conversations.value.findIndex((item) => item.id === conv.id)
    if (idx !== -1) {
      conversations.value[idx].unread_count = 0
    }
    await scrollToBottom()
  } catch (err: any) {
    if (err.response?.status === 401) return
    errorMsg.value = err.response?.data?.error || 'Không thể tải nội dung tin nhắn.'
  } finally {
    loadingMessages.value = false
  }
}

async function handleSendMessage() {
  if (!selectedConv.value || sending.value) return
  const text = draftMessage.value.trim()
  const imgs = [...pendingImages.value]
  if (!text && imgs.length === 0) return

  sending.value = true
  errorMsg.value = ''
  fbInboxFallbackUrl.value = ''
  try {
    const { data } = await api.post<{ message?: MessageItem; messages?: MessageItem[] }>(
      `/inbox/conversations/${selectedConv.value.id}/messages`,
      { content: text, image_urls: imgs }
    )
    const sentList = data.messages || (data.message ? [data.message] : [])
    messages.value.push(...sentList)
    draftMessage.value = ''
    pendingImages.value = []
    selectedTargetMsg.value = null

    const idx = conversations.value.findIndex((c) => c.id === selectedConv.value?.id)
    if (idx !== -1 && sentList.length > 0) {
      const last = sentList[sentList.length - 1]
      conversations.value[idx].last_message = text || '[Đã gửi hình ảnh]'
      conversations.value[idx].last_message_time = last.sent_at
    }
    await scrollToBottom()
  } catch (err: any) {
    if (err.response?.status === 401) return
    errorMsg.value = err.response?.data?.error || 'Gửi tin nhắn thất bại.'
    fbInboxFallbackUrl.value = err.response?.data?.fb_inbox_url || ''
  } finally {
    sending.value = false
  }
}

function handleKeydown(e: KeyboardEvent) {
  if (e.key === 'Enter' && !e.shiftKey) {
    e.preventDefault()
    handleSendMessage()
  }
}

onMounted(() => {
  const token = localStorage.getItem('access_token')
  if (!token) {
    router.replace('/login')
    return
  }
  if ('Notification' in window && Notification.permission === 'default') {
    Notification.requestPermission().catch(() => {})
  }
  loadConversations()
  pollTimer = window.setInterval(pollInboxUpdates, 3000)
})

onUnmounted(() => {
  if (pollTimer !== null) {
    clearInterval(pollTimer)
    pollTimer = null
  }
  if (alertHideTimer !== null) {
    clearTimeout(alertHideTimer)
    alertHideTimer = null
  }
  if (autoSaveDebounceTimer !== null) {
    clearTimeout(autoSaveDebounceTimer)
    autoSaveDebounceTimer = null
  }
})
</script>

<template>
  <div class="fixed inset-0 flex flex-col bg-slate-50 text-slate-900">
    <!-- Top Header -->
    <header class="flex h-14 shrink-0 items-center justify-between border-b border-slate-200 bg-white px-5">
      <div class="flex items-center gap-3 min-w-0">
        <button
          @click="router.push('/')"
          class="inline-flex shrink-0 items-center gap-1.5 whitespace-nowrap rounded-xl border border-slate-200 bg-white px-3 py-1.5 text-xs font-semibold text-slate-700 transition hover:border-emerald-200 hover:bg-emerald-50 hover:text-emerald-800 cursor-pointer"
        >
          <ArrowLeft class="h-3.5 w-3.5" />
          <span>Trang chủ</span>
        </button>

        <div class="h-5 w-px shrink-0 bg-slate-200" />

        <div class="flex items-center gap-2.5 min-w-0">
          <div class="flex h-8 w-8 shrink-0 items-center justify-center rounded-xl bg-emerald-600 text-white shadow-xs">
            <MessageSquare class="h-4 w-4" />
          </div>
          <div class="flex items-center gap-2 min-w-0">
            <h1 class="truncate text-sm font-bold !text-slate-900">Hộp thư gộp đa Fanpage</h1>
            <span class="shrink-0 whitespace-nowrap rounded-full bg-emerald-50 px-2 py-0.5 text-[11px] font-semibold text-emerald-700 border border-emerald-200">
              {{ pages.length }} Page
            </span>
          </div>
        </div>
      </div>

      <div class="flex items-center gap-2.5 shrink-0">
        <button
          @click="router.push('/pages')"
          class="inline-flex items-center gap-1.5 whitespace-nowrap rounded-xl border border-slate-200 bg-white px-3 py-1.5 text-xs font-semibold text-slate-700 transition hover:bg-slate-50 cursor-pointer"
        >
          <Layers class="h-3.5 w-3.5 text-emerald-600" />
          <span>Quản lý Page</span>
        </button>

        <button
          @click="handleSyncMessages"
          :disabled="syncing"
          class="inline-flex items-center gap-1.5 whitespace-nowrap rounded-xl bg-emerald-600 px-3.5 py-1.5 text-xs font-semibold text-white shadow-xs transition hover:bg-emerald-700 disabled:opacity-60 cursor-pointer"
        >
          <RefreshCw class="h-3.5 w-3.5" :class="{ 'animate-spin': syncing }" />
          <span>{{ syncing ? 'Đang kéo tin...' : 'Kéo tin nhắn từ FB' }}</span>
        </button>
      </div>
    </header>

    <!-- Toast Thông báo -->
    <div
      v-if="noticeMsg"
      class="flex shrink-0 items-center justify-center gap-2 border-b border-emerald-200 bg-emerald-50 px-4 py-2 text-xs font-semibold text-emerald-800"
    >
      <CheckCircle2 class="h-3.5 w-3.5 text-emerald-600" />
      <span>{{ noticeMsg }}</span>
    </div>
    <div
      v-if="errorMsg"
      class="shrink-0 border-b border-red-200 bg-red-50 px-4 py-2 text-center text-xs font-semibold text-red-700"
    >
      {{ errorMsg }}
    </div>

    <!-- Main 3-Column Workspace -->
    <div class="grid min-h-0 flex-1 grid-cols-12 overflow-hidden">
      <!-- CỘT 1: Danh sách hội thoại đa Page -->
      <aside class="col-span-12 flex min-h-0 flex-col border-r border-slate-200 bg-white md:col-span-4 lg:col-span-3">
        <!-- Bộ lọc Page + Search + Tabs -->
        <div class="shrink-0 space-y-2.5 border-b border-slate-100 p-3">
          <select
            v-model="selectedPageFilter"
            class="w-full rounded-xl border border-slate-200 bg-slate-50/70 px-3 py-2 text-xs font-semibold text-slate-800 outline-none transition focus:border-emerald-600 focus:bg-white"
          >
            <option value="all">Tất cả Fanpage ({{ pages.length }})</option>
            <option v-for="p in pages" :key="p.page_id" :value="p.page_id">
              {{ pageSettingsMap[p.page_id]?.product_tag ? `[${pageSettingsMap[p.page_id].product_tag}] ` : '' }}{{ p.name }}
            </option>
          </select>

          <div class="relative">
            <Search class="pointer-events-none absolute left-3 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-slate-400" />
            <input
              v-model="searchQuery"
              type="text"
              placeholder="Tìm tên khách, sản phẩm hoặc nội dung..."
              class="w-full rounded-xl border border-slate-200 bg-white py-2 pl-8 pr-3 text-xs text-slate-800 placeholder:text-slate-400 outline-none transition focus:border-emerald-600"
            />
          </div>

          <div class="grid grid-cols-2 gap-1 rounded-xl bg-slate-100 p-1">
            <button
              @click="readFilter = 'all'"
              class="rounded-lg py-1 text-[11px] font-semibold transition cursor-pointer"
              :class="readFilter === 'all' ? 'bg-white text-emerald-700 shadow-2xs' : 'text-slate-600 hover:text-slate-900'"
            >
              Tất cả ({{ conversations.length }})
            </button>
            <button
              @click="readFilter = 'unread'"
              class="rounded-lg py-1 text-[11px] font-semibold transition cursor-pointer"
              :class="readFilter === 'unread' ? 'bg-white text-emerald-700 shadow-2xs' : 'text-slate-600 hover:text-slate-900'"
            >
              Chưa đọc ({{ totalUnread }})
            </button>
          </div>
        </div>

        <!-- Danh sách thẻ hội thoại -->
        <div class="flex-1 divide-y divide-slate-100 overflow-y-auto">
          <div v-if="loadingList" class="flex items-center justify-center gap-2 p-8 text-xs text-slate-500">
            <Loader2 class="h-4 w-4 animate-spin text-emerald-600" />
            <span>Đang tải hội thoại...</span>
          </div>

          <div v-else-if="filteredConversations.length === 0" class="p-8 text-center text-xs text-slate-500">
            Không có hội thoại phù hợp. Bấm <strong>Kéo tin nhắn từ FB</strong> để cập nhật.
          </div>

          <button
            v-for="conv in filteredConversations"
            :key="conv.id"
            @click="selectConversation(conv)"
            class="group flex w-full items-start gap-3 px-3.5 py-3 text-left transition cursor-pointer"
            :class="
              selectedConv?.id === conv.id
                ? 'bg-emerald-50/90 border-l-[3px] border-l-emerald-600'
                : 'hover:bg-slate-50'
            "
          >
            <!-- Avatar Khách + Badge Avatar Page góc dưới -->
            <div class="relative h-11 w-11 shrink-0">
              <img
                v-if="!brokenAvatars['c_' + conv.id]"
                :src="getCustomerAvatarUrl(conv)"
                :alt="conv.customer_name"
                loading="lazy"
                @error="handleAvatarError('c_' + conv.id)"
                class="h-11 w-11 rounded-full object-cover border border-slate-200 bg-slate-100"
              />
              <div
                v-else
                class="flex h-11 w-11 items-center justify-center rounded-full bg-gradient-to-br from-emerald-500 to-teal-600 text-xs font-bold text-white shadow-2xs"
              >
                {{ getInitials(conv.customer_name) }}
              </div>

              <!-- Mini avatar của Fanpage -->
              <img
                v-if="pageMap[conv.page_id]?.avatar_url && !brokenAvatars['p_' + conv.page_id]"
                :src="pageMap[conv.page_id].avatar_url"
                :alt="conv.page_name"
                referrerpolicy="no-referrer"
                @error="handleAvatarError('p_' + conv.page_id)"
                class="absolute -bottom-0.5 -right-0.5 h-4 w-4 rounded-full border-2 border-white object-cover bg-white"
              />
              <span
                v-else
                class="absolute -bottom-0.5 -right-0.5 flex h-4 w-4 items-center justify-center rounded-full border-2 border-white bg-blue-600 text-white"
              >
                <Facebook class="h-2.5 w-2.5" />
              </span>
            </div>

            <!-- Thông tin hội thoại -->
            <div class="min-w-0 flex-1">
              <div class="flex items-center justify-between gap-2">
                <p
                  class="truncate text-[13px] !text-slate-900"
                  :class="conv.unread_count > 0 ? 'font-bold' : 'font-semibold'"
                >
                  {{ conv.customer_name }}
                </p>
                <span class="shrink-0 text-[11px] text-slate-400">
                  {{ formatTime(conv.last_message_time) }}
                </span>
              </div>

              <p
                class="mt-0.5 truncate text-xs"
                :class="conv.unread_count > 0 ? 'font-semibold text-slate-800' : 'text-slate-500'"
              >
                {{ conv.last_message || 'Tin nhắn mới' }}
              </p>

              <!-- Nhãn sản phẩm đặc trưng của Page + Tên Fanpage -->
              <div class="mt-1.5 flex items-center justify-between gap-1.5">
                <div class="flex min-w-0 flex-wrap items-center gap-1">
                  <span
                    v-if="pageSettingsMap[conv.page_id]?.product_tag"
                    class="inline-flex shrink-0 items-center gap-1 rounded-md border px-1.5 py-0.5 text-[10px] font-bold"
                    :class="getBadgeStyle(pageSettingsMap[conv.page_id]?.tag_color).badge"
                  >
                    <span
                      class="h-1.5 w-1.5 rounded-full"
                      :class="getBadgeStyle(pageSettingsMap[conv.page_id]?.tag_color).dot"
                    />
                    <span>{{ pageSettingsMap[conv.page_id].product_tag }}</span>
                  </span>

                  <span class="inline-flex max-w-[140px] items-center gap-1 truncate rounded-md bg-slate-100 px-1.5 py-0.5 text-[10px] font-medium text-slate-600">
                    <Facebook class="h-2.5 w-2.5 shrink-0 text-blue-600" />
                    <span class="truncate">{{ conv.page_name }}</span>
                  </span>
                </div>

                <span
                  v-if="conv.unread_count > 0"
                  class="flex h-4 min-w-4 shrink-0 items-center justify-center rounded-full bg-emerald-600 px-1 text-[10px] font-bold text-white"
                >
                  {{ conv.unread_count }}
                </span>
              </div>
            </div>
          </button>
        </div>
      </aside>

      <!-- CỘT 2: Khung chat trực tiếp -->
      <section class="col-span-12 flex min-h-0 flex-col bg-[#f4f7f6] md:col-span-8 lg:col-span-6">
        <div v-if="!selectedConv" class="flex flex-1 flex-col items-center justify-center p-8 text-center">
          <div class="flex h-16 w-16 items-center justify-center rounded-2xl bg-emerald-50 text-emerald-600 border border-emerald-100">
            <MessageSquare class="h-8 w-8" />
          </div>
          <h2 class="mt-4 text-base font-bold !text-slate-800">Chọn một hội thoại để bắt đầu tư vấn</h2>
          <p class="mt-1 max-w-sm text-xs text-slate-500">
            Tin nhắn của khách hàng từ tất cả Fanpage đều tập trung tại cột bên trái và tự động trả lời đúng Page khách đã nhắn.
          </p>
        </div>

        <template v-else>
          <!-- Header hội thoại + Thanh công tắc Bật/Tắt nhanh Auto Mẫu & AI -->
          <div class="flex flex-wrap items-center justify-between gap-2 border-b border-slate-200 bg-white px-4 py-2.5">
            <div class="flex min-w-0 items-center gap-3">
              <img
                v-if="!brokenAvatars['c_' + selectedConv.id]"
                :src="getCustomerAvatarUrl(selectedConv)"
                :alt="selectedConv.customer_name"
                @error="handleAvatarError('c_' + selectedConv.id)"
                class="h-10 w-10 shrink-0 rounded-full object-cover border border-slate-200"
              />
              <div
                v-else
                class="flex h-10 w-10 shrink-0 items-center justify-center rounded-full bg-gradient-to-br from-emerald-500 to-teal-600 text-xs font-bold text-white"
              >
                {{ getInitials(selectedConv.customer_name) }}
              </div>

              <div class="min-w-0">
                <div class="flex items-center gap-2">
                  <h2 class="truncate text-sm font-bold !text-slate-900">
                    {{ selectedConv.customer_name }}
                  </h2>
                  <span
                    v-if="pageProductTag || pageSettingsMap[selectedConv.page_id]?.product_tag"
                    class="inline-flex shrink-0 items-center gap-1 rounded-full border px-2 py-0.5 text-[10px] font-bold"
                    :class="getBadgeStyle(pageTagColor || pageSettingsMap[selectedConv.page_id]?.tag_color).badge"
                  >
                    <Tag class="h-2.5 w-2.5" />
                    <span>{{ pageProductTag || pageSettingsMap[selectedConv.page_id]?.product_tag }}</span>
                  </span>
                </div>
                <div class="flex items-center gap-1.5 text-xs text-emerald-700 font-medium">
                  <span class="h-2 w-2 rounded-full bg-emerald-500" />
                  <span class="truncate">{{ selectedConv.page_name }}</span>
                </div>
              </div>
            </div>

            <!-- Thanh công tắc Bật/Tắt nhanh Auto Mẫu & AI Tự động -->
            <div class="flex flex-wrap items-center gap-1.5">
              <button
                type="button"
                @click="toggleAllPageTemplatesAuto"
                :title="isAnyTemplateAutoEnabled ? 'Bấm để TẮT tự động trả lời bằng mẫu từ khóa của Page này' : 'Bấm để BẬT tự động trả lời bằng mẫu từ khóa của Page này'"
                class="inline-flex items-center gap-1.5 rounded-lg border px-2.5 py-1 text-[11px] font-bold transition cursor-pointer"
                :class="
                  isAnyTemplateAutoEnabled
                    ? 'border-amber-300 bg-amber-50 text-amber-800'
                    : 'border-slate-200 bg-slate-50 text-slate-500 hover:bg-slate-100'
                "
              >
                <Zap class="h-3 w-3" :class="isAnyTemplateAutoEnabled ? 'text-amber-500' : 'text-slate-400'" />
                <span>Auto Mẫu: {{ isAnyTemplateAutoEnabled ? 'BẬT' : 'TẮT' }}</span>
              </button>

              <button
                type="button"
                @click="togglePageAIAutoReply"
                :title="aiAutoReply ? 'Bấm để TẮT AI tự động trả lời sau 2 phút' : 'Bấm để BẬT AI tự động trả lời sau 2 phút'"
                class="inline-flex items-center gap-1.5 rounded-lg border px-2.5 py-1 text-[11px] font-bold transition cursor-pointer"
                :class="
                  aiAutoReply
                    ? 'border-emerald-300 bg-emerald-50 text-emerald-800'
                    : 'border-slate-200 bg-slate-50 text-slate-500 hover:bg-slate-100'
                "
              >
                <Bot class="h-3 w-3" :class="aiAutoReply ? 'text-emerald-600' : 'text-slate-400'" />
                <span>AI Tự động: {{ aiAutoReply ? 'BẬT' : 'TẮT' }}</span>
              </button>

              <button
                @click="copyText(selectedConv.customer_psid, 'psid')"
                title="Sao chép PSID khách hàng"
                class="hidden xl:inline-flex items-center gap-1 rounded-lg border border-slate-200 bg-slate-50 px-2 py-1 text-[11px] font-mono text-slate-600 transition hover:border-emerald-300 hover:bg-emerald-50 cursor-pointer"
              >
                <span>PSID</span>
                <Check v-if="copiedField === 'psid'" class="h-3 w-3 text-emerald-600" />
                <Copy v-else class="h-3 w-3 text-slate-400" />
              </button>
            </div>
          </div>

          <!-- Vùng nội dung tin nhắn -->
          <div ref="chatBoxRef" class="flex-1 space-y-3.5 overflow-y-auto px-5 py-4">
            <div v-if="loadingMessages" class="flex items-center justify-center gap-2 py-8 text-xs text-slate-500">
              <Loader2 class="h-4 w-4 animate-spin text-emerald-600" />
              <span>Đang tải lịch sử tin nhắn...</span>
            </div>

            <div
              v-for="msg in messages"
              :key="msg.id"
              class="group flex items-end gap-2.5"
              :class="msg.sender_type === 'page' ? 'justify-end' : 'justify-start'"
            >
              <!-- Avatar khách ở tin nhắn đến -->
              <template v-if="msg.sender_type === 'customer'">
                <img
                  v-if="!brokenAvatars['c_' + selectedConv.id]"
                  :src="getCustomerAvatarUrl(selectedConv)"
                  :alt="selectedConv.customer_name"
                  @error="handleAvatarError('c_' + selectedConv.id)"
                  class="mb-4 h-7 w-7 shrink-0 rounded-full object-cover border border-slate-200"
                />
                <div
                  v-else
                  class="mb-4 flex h-7 w-7 shrink-0 items-center justify-center rounded-full bg-emerald-100 text-[10px] font-bold text-emerald-800"
                >
                  {{ getInitials(selectedConv.customer_name) }}
                </div>
              </template>

              <!-- Bong bóng tin nhắn (Bấm vào tin khách để chọn trả lời bằng AI) -->
              <div
                class="flex max-w-[72%] flex-col"
                :class="msg.sender_type === 'page' ? 'items-end' : 'items-start'"
              >
                <div class="flex items-center gap-1.5">
                  <div
                    @click="msg.sender_type === 'customer' && msg.content ? toggleSelectMessageForAI(msg) : undefined"
                    class="overflow-hidden rounded-2xl text-[13.5px] leading-relaxed shadow-2xs transition"
                    :class="[
                      msg.attachment_url && !msg.content ? 'p-1.5' : 'px-4 py-2.5',
                      msg.sender_type === 'page'
                        ? 'bg-emerald-600 text-white rounded-br-xs'
                        : 'bg-white border text-slate-800 rounded-bl-xs cursor-pointer hover:border-emerald-400',
                      selectedTargetMsg?.id === msg.id
                        ? 'border-emerald-600 ring-2 ring-emerald-200 bg-emerald-50/60'
                        : 'border-slate-200/90'
                    ]"
                    :title="msg.sender_type === 'customer' ? 'Bấm để chọn tin nhắn này cho AI trả lời' : undefined"
                  >
                    <a
                      v-if="msg.attachment_url && msg.attachment_type === 'image'"
                      :href="resolveMediaUrl(msg.attachment_url)"
                      target="_blank"
                      rel="noopener noreferrer"
                      class="block"
                      @click.stop
                    >
                      <img
                        :src="resolveMediaUrl(msg.attachment_url)"
                        alt="Hình ảnh đính kèm"
                        referrerpolicy="no-referrer"
                        class="max-h-64 w-auto rounded-xl object-contain"
                      />
                    </a>

                    <a
                      v-else-if="msg.attachment_url"
                      :href="resolveMediaUrl(msg.attachment_url)"
                      target="_blank"
                      rel="noopener noreferrer"
                      class="inline-flex items-center gap-1.5 underline font-medium"
                      @click.stop
                    >
                      <span>Tệp đính kèm từ Facebook</span>
                    </a>

                    <p v-if="msg.content" :class="{ 'mt-2': msg.attachment_url }" class="whitespace-pre-wrap break-words">
                      {{ msg.content }}
                    </p>
                  </div>

                  <!-- Nút chọn nhanh tin nhắn khách để AI trả lời -->
                  <button
                    v-if="msg.sender_type === 'customer' && msg.content"
                    type="button"
                    @click="toggleSelectMessageForAI(msg)"
                    :title="selectedTargetMsg?.id === msg.id ? 'Bỏ chọn tin nhắn này' : 'Chọn tin nhắn này để AI trả lời'"
                    class="shrink-0 rounded-lg border px-2 py-1 text-[10px] font-semibold transition cursor-pointer"
                    :class="
                      selectedTargetMsg?.id === msg.id
                        ? 'border-emerald-600 bg-emerald-600 text-white'
                        : 'border-slate-200 bg-white text-slate-500 opacity-0 group-hover:opacity-100 hover:border-emerald-400 hover:text-emerald-700'
                    "
                  >
                    <span class="inline-flex items-center gap-1">
                      <CornerUpLeft class="h-3 w-3" />
                      <span>{{ selectedTargetMsg?.id === msg.id ? 'Đang chọn' : 'Chọn rep' }}</span>
                    </span>
                  </button>
                </div>

                <span class="mt-1 px-1 text-[10px] text-slate-400">
                  {{ msg.sender_type === 'page' ? selectedConv.page_name : selectedConv.customer_name }} •
                  {{ formatFullTime(msg.sent_at) }}
                </span>
              </div>

              <!-- Avatar Fanpage ở tin nhắn đi -->
              <template v-if="msg.sender_type === 'page'">
                <img
                  v-if="pageMap[selectedConv.page_id]?.avatar_url && !brokenAvatars['p_' + selectedConv.page_id]"
                  :src="pageMap[selectedConv.page_id].avatar_url"
                  :alt="selectedConv.page_name"
                  referrerpolicy="no-referrer"
                  @error="handleAvatarError('p_' + selectedConv.page_id)"
                  class="mb-4 h-7 w-7 shrink-0 rounded-full object-cover border border-emerald-200"
                />
                <div
                  v-else
                  class="mb-4 flex h-7 w-7 shrink-0 items-center justify-center rounded-full bg-emerald-600 text-[10px] font-bold text-white"
                >
                  P
                </div>
              </template>
            </div>
          </div>

          <!-- Khung soạn tin nhắn + Duyệt AI trước khi gửi + Chọn tin & Hỏi/Chỉ đạo AI -->
          <form @submit.prevent="handleSendMessage" class="shrink-0 border-t border-slate-200 bg-white p-3.5 space-y-2">
            <!-- Cảnh báo tự động nếu hội thoại đã quá 24h của Meta -->
            <div
              v-if="isOver24Hours(selectedConv.last_message_time) || fbInboxFallbackUrl"
              class="flex flex-wrap items-center justify-between gap-2 rounded-xl border border-amber-200 bg-amber-50 px-3 py-2 text-xs text-amber-900"
            >
              <span>
                Hội thoại này đã quá <strong>24 giờ</strong> (quy định chống spam của Meta). Nếu API bị chặn, hãy mở thẳng hộp thư gốc:
              </span>
              <a
                :href="fbInboxFallbackUrl || getMetaBusinessInboxUrl(selectedConv)"
                target="_blank"
                rel="noopener noreferrer"
                class="inline-flex shrink-0 items-center gap-1 rounded-lg bg-amber-600 px-2.5 py-1 text-[11px] font-bold text-white hover:bg-amber-700"
              >
                <span>Mở trên Meta Business Suite ↗</span>
              </a>
            </div>

            <!-- Hộp Hỏi duyệt câu trả lời AI trước khi gửi cho khách -->
            <div
              v-if="aiApprovalDraft"
              class="rounded-xl border border-emerald-300 bg-emerald-50/90 p-3 space-y-2 shadow-xs"
            >
              <div class="flex items-center justify-between gap-2">
                <span class="inline-flex items-center gap-1.5 text-xs font-bold text-emerald-900">
                  <ShieldCheck class="h-4 w-4 text-emerald-600" />
                  <span>AI đã soạn xong — Bạn duyệt trước khi gửi cho khách:</span>
                </span>
                <button
                  type="button"
                  @click="aiApprovalDraft = ''"
                  class="text-slate-400 hover:text-slate-700 cursor-pointer"
                  title="Bỏ qua gợi ý này"
                >
                  <X class="h-4 w-4" />
                </button>
              </div>

              <textarea
                v-model="aiApprovalDraft"
                rows="2"
                class="w-full resize-none rounded-lg border border-emerald-200 bg-white px-3 py-2 text-xs leading-relaxed text-slate-900 outline-none focus:border-emerald-600"
              />

              <div class="flex flex-wrap items-center justify-end gap-2">
                <button
                  type="button"
                  @click="aiApprovalDraft = ''"
                  class="rounded-lg border border-slate-200 bg-white px-2.5 py-1 text-[11px] font-semibold text-slate-600 hover:bg-slate-50 cursor-pointer"
                >
                  Hủy
                </button>
                <button
                  type="button"
                  @click="moveAIDraftToEditor"
                  class="rounded-lg border border-emerald-200 bg-white px-2.5 py-1 text-[11px] font-semibold text-emerald-800 hover:bg-emerald-100 cursor-pointer"
                >
                  Đưa xuống ô chat
                </button>
                <button
                  type="button"
                  @click="approveAndSendAIReply"
                  :disabled="sending"
                  class="inline-flex items-center gap-1 rounded-lg bg-emerald-600 px-3 py-1 text-[11px] font-bold text-white shadow-2xs hover:bg-emerald-700 disabled:opacity-50 cursor-pointer"
                >
                  <Send class="h-3 w-3" />
                  <span>Duyệt & Gửi ngay</span>
                </button>
              </div>
            </div>

            <!-- Thanh tin nhắn mẫu chuyên nghiệp của riêng Fanpage này -->
            <div v-if="pageQuickReplies.length > 0" class="rounded-xl border border-slate-200/90 bg-slate-50/70 p-2 space-y-1.5">
              <div class="flex flex-wrap items-center justify-between gap-2">
                <div class="flex items-center gap-1.5">
                  <span class="inline-flex items-center gap-1 text-[11px] font-bold text-slate-700">
                    <Zap class="h-3.5 w-3.5 text-amber-500" />
                    <span>Mẫu nhanh ({{ filteredQuickReplies.length }})</span>
                  </span>
                  <select
                    v-model="quickReplyCategoryFilter"
                    class="rounded-lg border border-slate-200 bg-white px-2 py-0.5 text-[10px] font-semibold text-slate-700 outline-none focus:border-emerald-600"
                  >
                    <option value="all">Tất cả nhóm</option>
                    <option value="Báo giá">Báo giá</option>
                    <option value="Chốt đơn">Chốt đơn</option>
                    <option value="Chính sách">Chính sách</option>
                    <option value="Chào hỏi">Chào hỏi</option>
                  </select>
                </div>

                <input
                  v-model="quickReplySearch"
                  type="text"
                  placeholder="Lọc tên mẫu / từ khóa..."
                  class="w-36 sm:w-44 rounded-lg border border-slate-200 bg-white px-2 py-0.5 text-[10px] text-slate-700 outline-none focus:border-emerald-600"
                />
              </div>

              <div class="flex items-center gap-1.5 overflow-x-auto pb-0.5">
                <button
                  v-for="qr in filteredQuickReplies"
                  :key="qr.id"
                  type="button"
                  @click="applyQuickReply(qr)"
                  :title="`${qr.category} • ${qr.content}${qr.images?.length ? ` (${qr.images.length} ảnh)` : ''}`"
                  class="group inline-flex shrink-0 items-center gap-1.5 rounded-lg border border-slate-200 bg-white px-2.5 py-1 text-[11px] font-semibold text-slate-700 shadow-2xs transition hover:border-emerald-400 hover:bg-emerald-50 hover:text-emerald-800 cursor-pointer"
                >
                  <span class="rounded bg-slate-100 px-1.5 py-0.5 text-[9px] font-bold text-slate-500 group-hover:bg-emerald-100 group-hover:text-emerald-700">
                    {{ qr.category }}
                  </span>
                  <span>{{ qr.title }}</span>
                  <span
                    v-if="qr.images && qr.images.length > 0"
                    class="rounded bg-blue-50 px-1 py-0.5 text-[9px] font-bold text-blue-700"
                  >
                    +{{ qr.images.length }} ảnh
                  </span>
                  <span
                    v-if="qr.autoTrigger"
                    class="h-1.5 w-1.5 rounded-full bg-emerald-500"
                    title="Đang bật tự động trả lời theo mẫu"
                  />
                </button>
              </div>
            </div>

            <!-- Thanh Chọn tin nhắn khách + Hỏi/Chỉ đạo AI + Nút AI Gợi ý -->
            <div class="rounded-xl border border-slate-200/80 bg-slate-50/70 p-2 space-y-1.5">
              <!-- Hiển thị tin nhắn của khách đang được chọn để AI tập trung trả lời -->
              <div
                v-if="selectedTargetMsg"
                class="flex items-center justify-between gap-2 rounded-lg border border-emerald-200 bg-emerald-50 px-2.5 py-1 text-[11px] text-emerald-900"
              >
                <div class="flex min-w-0 items-center gap-1.5">
                  <CornerUpLeft class="h-3 w-3 shrink-0 text-emerald-600" />
                  <span class="font-bold shrink-0">Đang chọn trả lời tin:</span>
                  <span class="truncate italic">"{{ selectedTargetMsg.content }}"</span>
                </div>
                <button
                  type="button"
                  @click="selectedTargetMsg = null"
                  class="shrink-0 text-emerald-700 hover:text-red-600 cursor-pointer"
                  title="Bỏ chọn tin nhắn"
                >
                  <X class="h-3.5 w-3.5" />
                </button>
              </div>

              <div class="flex flex-wrap items-center gap-1.5">
                <input
                  v-model="aiCustomInstruction"
                  type="text"
                  placeholder="Hỏi / chỉ đạo AI trước khi soạn (VD: Báo giá 450k, freeship)..."
                  class="min-w-[180px] flex-1 rounded-lg border border-slate-200 bg-white px-2.5 py-1.5 text-xs text-slate-800 placeholder:text-slate-400 outline-none focus:border-emerald-600"
                />

                <label
                  class="inline-flex shrink-0 items-center gap-1 rounded-lg border border-slate-200 bg-white px-2 py-1.5 text-[11px] font-semibold text-slate-700 cursor-pointer"
                  title="Khi bật, AI sẽ hiện bảng hỏi duyệt trước khi gửi cho khách"
                >
                  <input
                    v-model="aiAskBeforeSend"
                    type="checkbox"
                    class="h-3.5 w-3.5 accent-emerald-600 cursor-pointer"
                  />
                  <span>Hỏi duyệt trước</span>
                </label>

                <button
                  type="button"
                  @click="handleAISuggest"
                  :disabled="generatingAI"
                  class="inline-flex shrink-0 items-center gap-1.5 rounded-lg bg-emerald-600 px-3 py-1.5 text-xs font-semibold text-white shadow-2xs transition hover:bg-emerald-700 disabled:opacity-60 cursor-pointer"
                >
                  <Loader2 v-if="generatingAI" class="h-3.5 w-3.5 animate-spin" />
                  <Sparkles v-else class="h-3.5 w-3.5" />
                  <span>{{ generatingAI ? 'AI đang soạn...' : 'AI Gợi ý trả lời' }}</span>
                </button>
              </div>

              <!-- Các chip chỉ đạo nhanh cho AI -->
              <div class="flex items-center gap-1 overflow-x-auto pb-0.5">
                <span class="shrink-0 text-[10px] font-semibold text-slate-400">Chỉ đạo nhanh:</span>
                <button
                  v-for="preset in quickInstructionPresets"
                  :key="preset"
                  type="button"
                  @click="aiCustomInstruction = preset"
                  class="shrink-0 rounded-md border border-slate-200 bg-white px-2 py-0.5 text-[10px] font-medium text-slate-600 hover:border-emerald-300 hover:bg-emerald-50 hover:text-emerald-800 cursor-pointer"
                >
                  {{ preset }}
                </button>
              </div>
            </div>

            <!-- Xem trước nhiều ảnh chuẩn bị gửi -->
            <div
              v-if="pendingImages.length > 0 || uploadingImages"
              class="flex items-center gap-2 overflow-x-auto rounded-xl border border-slate-200 bg-slate-50 p-2"
            >
              <div
                v-for="(imgUrl, i) in pendingImages"
                :key="imgUrl + i"
                class="relative h-14 w-14 shrink-0"
              >
                <img
                  :src="resolveMediaUrl(imgUrl)"
                  alt="Ảnh chuẩn bị gửi"
                  class="h-14 w-14 rounded-lg object-cover border border-slate-200 bg-white"
                />
                <button
                  type="button"
                  @click="pendingImages.splice(i, 1)"
                  title="Gỡ ảnh này"
                  class="absolute -right-1.5 -top-1.5 flex h-4 w-4 items-center justify-center rounded-full bg-red-600 text-[10px] font-bold text-white shadow cursor-pointer"
                >
                  ×
                </button>
              </div>
              <div v-if="uploadingImages" class="flex items-center gap-1.5 px-2 text-xs text-emerald-700 font-semibold">
                <Loader2 class="h-4 w-4 animate-spin" />
                <span>Đang tải ảnh...</span>
              </div>
            </div>

            <div class="flex items-end gap-2">
              <input
                ref="chatFileInputRef"
                type="file"
                accept="image/*"
                multiple
                class="hidden"
                @change="handleChatFilesSelected"
              />
              <button
                type="button"
                @click="chatFileInputRef?.click()"
                title="Chọn nhiều hình ảnh để gửi"
                class="inline-flex h-[46px] w-[46px] shrink-0 items-center justify-center rounded-xl border border-slate-200 bg-slate-50 text-slate-600 transition hover:border-emerald-400 hover:bg-emerald-50 hover:text-emerald-700 cursor-pointer"
              >
                <ImagePlus class="h-5 w-5" />
              </button>

              <textarea
                v-model="draftMessage"
                rows="2"
                @keydown="handleKeydown"
                @paste="handleChatPaste"
                placeholder="Nhập tin nhắn hoặc dán ảnh (Ctrl + V)... (Enter để gửi, Shift + Enter xuống dòng)"
                class="max-h-32 min-h-[46px] flex-1 resize-none rounded-xl border border-slate-200 bg-slate-50/60 px-3.5 py-2.5 text-sm text-slate-900 placeholder:text-slate-400 outline-none transition focus:border-emerald-600 focus:bg-white focus:ring-2 focus:ring-emerald-100"
              />
              <button
                type="submit"
                :disabled="sending || uploadingImages || (!draftMessage.trim() && pendingImages.length === 0)"
                class="inline-flex h-[46px] shrink-0 items-center gap-1.5 rounded-xl bg-emerald-600 px-4 text-sm font-semibold text-white shadow-xs transition hover:bg-emerald-700 disabled:opacity-50 cursor-pointer"
              >
                <Loader2 v-if="sending" class="h-4 w-4 animate-spin" />
                <Send v-else class="h-4 w-4" />
                <span>Gửi</span>
              </button>
            </div>
          </form>
        </template>
      </section>

      <!-- CỘT 3: Hồ sơ khách hàng & Cấu hình Sản phẩm / Mẫu & Auto-Rep / AI cho Fanpage -->
      <aside class="hidden min-h-0 overflow-y-auto border-l border-slate-200 bg-white p-5 lg:col-span-3 lg:block">
        <div v-if="selectedConv" class="space-y-4">
          <!-- Card Avatar + Tên khách -->
          <div class="flex flex-col items-center rounded-2xl border border-slate-100 bg-slate-50/60 p-4 text-center">
            <img
              v-if="!brokenAvatars['c_' + selectedConv.id]"
              :src="getCustomerAvatarUrl(selectedConv)"
              :alt="selectedConv.customer_name"
              @error="handleAvatarError('c_' + selectedConv.id)"
              class="h-14 w-14 rounded-full object-cover border-2 border-white shadow-sm"
            />
            <div
              v-else
              class="flex h-14 w-14 items-center justify-center rounded-full bg-gradient-to-br from-emerald-500 to-teal-600 text-base font-bold text-white shadow-sm"
            >
              {{ getInitials(selectedConv.customer_name) }}
            </div>

            <h3 class="mt-2.5 text-sm font-bold !text-slate-900">{{ selectedConv.customer_name }}</h3>
            <span class="mt-1 inline-flex items-center gap-1 rounded-full bg-blue-50 px-2.5 py-0.5 text-[11px] font-semibold text-blue-700">
              <Facebook class="h-3 w-3" />
              <span>Khách hàng Messenger</span>
            </span>
          </div>

          <!-- Nguồn Fanpage -->
          <div class="rounded-2xl border border-emerald-200/80 bg-emerald-50/40 p-3.5">
            <p class="text-[11px] font-bold uppercase tracking-wider text-emerald-800">Nguồn Fanpage</p>
            <div class="mt-2 flex items-center gap-2.5">
              <img
                v-if="pageMap[selectedConv.page_id]?.avatar_url && !brokenAvatars['p_' + selectedConv.page_id]"
                :src="pageMap[selectedConv.page_id].avatar_url"
                :alt="selectedConv.page_name"
                referrerpolicy="no-referrer"
                @error="handleAvatarError('p_' + selectedConv.page_id)"
                class="h-8 w-8 shrink-0 rounded-full object-cover border border-emerald-200 bg-white"
              />
              <div
                v-else
                class="flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-emerald-600 text-xs font-bold text-white"
              >
                P
              </div>
              <div class="min-w-0">
                <p class="truncate text-xs font-bold !text-slate-900">{{ selectedConv.page_name }}</p>
                <p class="text-[11px] text-slate-500">ID: {{ selectedConv.page_id }}</p>
              </div>
            </div>
          </div>

          <!-- Nhãn nhận diện sản phẩm của Page -->
          <div class="rounded-2xl border border-slate-200 bg-slate-50/50 p-3.5 space-y-2.5">
            <div class="flex items-center justify-between">
              <span class="inline-flex items-center gap-1.5 text-xs font-bold !text-slate-900">
                <Tag class="h-3.5 w-3.5 text-emerald-600" />
                <span>Nhãn nhận diện sản phẩm</span>
              </span>
              <Loader2 v-if="loadingAISetting" class="h-3.5 w-3.5 animate-spin text-emerald-600" />
            </div>

            <input
              v-model="pageProductTag"
              type="text"
              placeholder="VD: Micro Hamax, Hạt Điều, Phân Bón..."
              class="w-full rounded-xl border border-slate-200 bg-white px-2.5 py-1.5 text-xs font-semibold text-slate-800 outline-none focus:border-emerald-600"
            />

            <div class="flex flex-wrap gap-1.5">
              <button
                v-for="color in tagColorOptions"
                :key="color.id"
                type="button"
                @click="pageTagColor = color.id"
                class="inline-flex items-center gap-1 rounded-lg border px-2 py-1 text-[10px] font-semibold transition cursor-pointer"
                :class="[
                  color.badge,
                  pageTagColor === color.id ? 'ring-2 ring-offset-1 ring-slate-400' : 'opacity-75 hover:opacity-100'
                ]"
              >
                <span class="h-2 w-2 rounded-full" :class="color.dot" />
                <span>{{ color.label }}</span>
              </button>
            </div>
          </div>

          <!-- Tin nhắn mẫu & Tự động trả lời bằng mẫu của Page (Kèm nhiều ảnh + Auto-Save) -->
          <div class="rounded-2xl border border-slate-200 bg-slate-50/50 p-3.5 space-y-2.5">
            <div class="flex items-center justify-between gap-2">
              <span class="inline-flex items-center gap-1.5 text-xs font-bold !text-slate-900">
                <Zap class="h-3.5 w-3.5 text-amber-500" />
                <span>Mẫu & Auto-Rep ({{ pageQuickReplies.length }})</span>
              </span>

              <span
                v-if="autoSaveStatus === 'saving'"
                class="text-[10px] font-semibold text-amber-600"
              >
                Đang lưu...
              </span>
              <span
                v-else-if="autoSaveStatus === 'saved'"
                class="inline-flex items-center gap-0.5 text-[10px] font-bold text-emerald-700"
              >
                <Check class="h-3 w-3" />
                <span>Đã tự lưu</span>
              </span>
              <span v-else class="text-[10px] font-semibold text-slate-400">
                &#123;customer_name&#125;, &#123;page_name&#125;
              </span>
            </div>

            <div v-if="pageQuickReplies.length > 0" class="space-y-2 max-h-60 overflow-y-auto pr-0.5">
              <div
                v-for="(qr, idx) in pageQuickReplies"
                :key="qr.id"
                class="rounded-xl border border-slate-200 bg-white p-2.5 text-xs space-y-1.5 shadow-2xs"
              >
                <div class="flex items-start justify-between gap-2">
                  <div class="min-w-0 flex-1">
                    <div class="flex items-center gap-1.5">
                      <span class="rounded bg-slate-100 px-1.5 py-0.5 text-[9px] font-bold text-slate-600">
                        {{ qr.category }}
                      </span>
                      <p class="font-bold !text-slate-900 truncate">{{ qr.title }}</p>
                    </div>
                    <p v-if="qr.keywords" class="mt-0.5 text-[10px] text-emerald-700 font-medium truncate">
                      Từ khóa: {{ qr.keywords }}
                    </p>
                  </div>

                  <div class="flex items-center gap-1.5 shrink-0">
                    <button
                      type="button"
                      @click="editingReplyId = editingReplyId === qr.id ? null : qr.id"
                      class="text-[10px] font-semibold text-slate-500 hover:text-emerald-700 cursor-pointer"
                    >
                      {{ editingReplyId === qr.id ? 'Xong' : 'Sửa' }}
                    </button>
                    <button
                      type="button"
                      @click="removeQuickReply(idx)"
                      class="text-slate-400 hover:text-red-600 cursor-pointer"
                      title="Xóa mẫu này"
                    >
                      <Trash2 class="h-3.5 w-3.5" />
                    </button>
                  </div>
                </div>

                <div v-if="editingReplyId === qr.id" class="space-y-1.5 pt-1">
                  <input
                    v-model="qr.title"
                    @input="scheduleSilentSave"
                    type="text"
                    class="w-full rounded-lg border border-slate-200 px-2 py-1 text-xs font-semibold outline-none focus:border-emerald-600"
                  />
                  <input
                    v-model="qr.keywords"
                    @input="scheduleSilentSave"
                    type="text"
                    placeholder="Từ khóa (VD: giá, bao nhiêu)..."
                    class="w-full rounded-lg border border-slate-200 px-2 py-1 text-[11px] outline-none focus:border-emerald-600"
                  />
                  <textarea
                    v-model="qr.content"
                    @input="scheduleSilentSave"
                    @paste="handleEditTemplatePaste($event, qr)"
                    rows="2"
                    placeholder="Nội dung mẫu (Ctrl+V để dán thêm ảnh)..."
                    class="w-full resize-none rounded-lg border border-slate-200 p-1.5 text-[11px] outline-none focus:border-emerald-600"
                  />

                  <div class="flex items-center gap-1.5 flex-wrap">
                    <div
                      v-for="(imgUrl, imgIdx) in qr.images || []"
                      :key="imgUrl + imgIdx"
                      class="relative h-10 w-10"
                    >
                      <img :src="resolveMediaUrl(imgUrl)" class="h-10 w-10 rounded-md object-cover border border-slate-200" />
                      <button
                        type="button"
                        @click="qr.images?.splice(imgIdx, 1); scheduleSilentSave()"
                        class="absolute -right-1 -top-1 flex h-3.5 w-3.5 items-center justify-center rounded-full bg-red-600 text-[9px] font-bold text-white cursor-pointer"
                      >
                        ×
                      </button>
                    </div>
                    <label class="inline-flex h-10 w-10 items-center justify-center rounded-md border border-dashed border-slate-300 text-slate-500 hover:border-emerald-500 hover:text-emerald-700 cursor-pointer" title="Thêm ảnh vào mẫu">
                      <ImagePlus class="h-4 w-4" />
                      <input type="file" accept="image/*" multiple class="hidden" @change="handleEditTemplateFilesSelected($event, qr)" />
                    </label>
                  </div>
                </div>

                <template v-else>
                  <p v-if="qr.content" class="text-[11px] text-slate-600 line-clamp-2 whitespace-pre-wrap">{{ qr.content }}</p>
                  <div v-if="qr.images && qr.images.length > 0" class="flex items-center gap-1.5 overflow-x-auto pt-0.5">
                    <img
                      v-for="(imgUrl, imgIdx) in qr.images"
                      :key="imgUrl + imgIdx"
                      :src="resolveMediaUrl(imgUrl)"
                      alt="Ảnh mẫu"
                      class="h-10 w-10 shrink-0 rounded-md object-cover border border-slate-200"
                    />
                  </div>
                </template>

                <label class="flex items-center justify-between border-t border-slate-100 pt-1.5 cursor-pointer">
                  <span class="text-[10px] font-semibold text-slate-700">Tự động rep khi khớp từ khóa</span>
                  <input
                    v-model="qr.autoTrigger"
                    @change="toggleQuickReplyAutoTrigger"
                    type="checkbox"
                    class="h-3.5 w-3.5 accent-emerald-600 cursor-pointer"
                  />
                </label>
              </div>
            </div>

            <div v-if="pageQuickReplies.length > 0 && pages.length > 1" class="grid grid-cols-2 gap-1.5">
              <button
                type="button"
                @click="applyTemplatesToOtherPages(true)"
                :disabled="applyingToAllPages || !pageProductTag.trim()"
                title="Chép bộ mẫu này sang các Page có cùng Nhãn nhận diện sản phẩm"
                class="rounded-xl border border-slate-200 bg-white px-2 py-1.5 text-[10px] font-bold text-slate-700 transition hover:border-emerald-300 hover:bg-emerald-50 hover:text-emerald-800 disabled:opacity-40 cursor-pointer"
              >
                {{ applyingToAllPages ? 'Đang chép...' : 'Chép sang Page cùng SP' }}
              </button>
              <button
                type="button"
                @click="applyTemplatesToOtherPages(false)"
                :disabled="applyingToAllPages"
                title="Áp dụng bộ mẫu này cho toàn bộ Fanpage đang kết nối"
                class="rounded-xl border border-slate-200 bg-white px-2 py-1.5 text-[10px] font-bold text-slate-700 transition hover:border-emerald-300 hover:bg-emerald-50 hover:text-emerald-800 disabled:opacity-40 cursor-pointer"
              >
                {{ applyingToAllPages ? 'Đang chép...' : `Áp dụng cả ${pages.length} Page` }}
              </button>
            </div>

            <div class="space-y-1.5 pt-1.5 border-t border-slate-200/80">
              <div class="grid grid-cols-2 gap-1.5">
                <input
                  v-model="newReplyTitle"
                  type="text"
                  placeholder="Tên mẫu (VD: Báo giá)"
                  class="rounded-xl border border-slate-200 bg-white px-2.5 py-1.5 text-xs text-slate-800 outline-none focus:border-emerald-600"
                />
                <select
                  v-model="newReplyCategory"
                  class="rounded-xl border border-slate-200 bg-white px-2 py-1.5 text-xs font-semibold text-slate-800 outline-none focus:border-emerald-600"
                >
                  <option value="Báo giá">Báo giá</option>
                  <option value="Chốt đơn">Chốt đơn</option>
                  <option value="Chính sách">Chính sách</option>
                  <option value="Chào hỏi">Chào hỏi</option>
                </select>
              </div>

              <input
                v-model="newReplyKeywords"
                type="text"
                placeholder="Từ khóa tự động rep (VD: giá, bao nhiêu, ship)..."
                class="w-full rounded-xl border border-slate-200 bg-white px-2.5 py-1.5 text-xs text-slate-800 outline-none focus:border-emerald-600"
              />

              <textarea
                v-model="newReplyContent"
                @paste="handleTemplatePaste"
                rows="2"
                placeholder="Nội dung mẫu hoặc dán ảnh (Ctrl + V)..."
                class="w-full resize-none rounded-xl border border-slate-200 bg-white p-2 text-xs text-slate-800 outline-none focus:border-emerald-600"
              />

              <!-- Chọn nhiều ảnh hoặc xem trước ảnh đính kèm của mẫu -->
              <div class="space-y-1.5">
                <input
                  ref="templateFileInputRef"
                  type="file"
                  accept="image/*"
                  multiple
                  class="hidden"
                  @change="handleTemplateFilesSelected"
                />
                <button
                  type="button"
                  @click="templateFileInputRef?.click()"
                  class="inline-flex w-full items-center justify-center gap-1.5 rounded-xl border border-dashed border-slate-300 bg-white px-2.5 py-1.5 text-[11px] font-semibold text-slate-700 transition hover:border-emerald-400 hover:bg-emerald-50 hover:text-emerald-800 cursor-pointer"
                >
                  <Loader2 v-if="uploadingTemplateImages" class="h-3.5 w-3.5 animate-spin text-emerald-600" />
                  <ImagePlus v-else class="h-3.5 w-3.5 text-emerald-600" />
                  <span>{{ uploadingTemplateImages ? 'Đang tải ảnh mẫu...' : 'Chọn nhiều ảnh cho mẫu (hoặc Ctrl+V)' }}</span>
                </button>

                <div v-if="newReplyImages.length > 0" class="flex items-center gap-1.5 overflow-x-auto pt-0.5">
                  <div
                    v-for="(imgUrl, i) in newReplyImages"
                    :key="imgUrl + i"
                    class="relative h-12 w-12 shrink-0"
                  >
                    <img :src="resolveMediaUrl(imgUrl)" class="h-12 w-12 rounded-lg object-cover border border-slate-200 bg-white" />
                    <button
                      type="button"
                      @click="newReplyImages.splice(i, 1)"
                      class="absolute -right-1 -top-1 flex h-4 w-4 items-center justify-center rounded-full bg-red-600 text-[10px] font-bold text-white shadow cursor-pointer"
                    >
                      ×
                    </button>
                  </div>
                </div>
              </div>

              <label class="flex items-center justify-between rounded-xl border border-slate-200 bg-white px-2.5 py-1.5 cursor-pointer">
                <span class="text-[11px] font-semibold text-slate-700">Bật tự động gửi mẫu khi khớp từ khóa</span>
                <input
                  v-model="newReplyAutoTrigger"
                  type="checkbox"
                  class="h-3.5 w-3.5 accent-emerald-600 cursor-pointer"
                />
              </label>

              <button
                type="button"
                @click="addQuickReply"
                :disabled="!newReplyTitle.trim() || (!newReplyContent.trim() && newReplyImages.length === 0)"
                class="inline-flex w-full items-center justify-center gap-1 rounded-xl bg-emerald-600 px-2.5 py-1.5 text-xs font-semibold text-white shadow-2xs hover:bg-emerald-700 disabled:opacity-50 cursor-pointer"
              >
                <Plus class="h-3.5 w-3.5" />
                <span>Thêm & Lưu mẫu ngay</span>
              </button>
            </div>
          </div>

          <!-- Huấn luyện AI riêng cho Fanpage này -->
          <div class="rounded-2xl border border-slate-200 bg-slate-50/50 p-3.5 space-y-3">
            <div class="flex items-center justify-between gap-2">
              <span class="inline-flex items-center gap-1.5 text-xs font-bold !text-slate-900">
                <Bot class="h-4 w-4 text-emerald-600" />
                <span>Kịch bản AI của Page</span>
              </span>

              <button
                type="button"
                @click="handleGenerateSalesScript"
                :disabled="generatingScript || !pageProductTag.trim()"
                title="Nhập tên sản phẩm ở mục Nhãn nhận diện sản phẩm phía trên rồi bấm nút này"
                class="inline-flex items-center gap-1 rounded-lg border border-emerald-200 bg-emerald-50 px-2 py-1 text-[10px] font-bold text-emerald-800 transition hover:bg-emerald-100 disabled:opacity-50 cursor-pointer"
              >
                <Loader2 v-if="generatingScript" class="h-3 w-3 animate-spin text-emerald-600" />
                <Sparkles v-else class="h-3 w-3 text-emerald-600" />
                <span>{{ generatingScript ? 'Đang viết...' : 'AI Gợi ý kịch bản' }}</span>
              </button>
            </div>

            <div>
              <label class="block text-[11px] font-semibold text-slate-600 mb-1">
                Giọng điệu xưng hô
              </label>
              <input
                v-model="aiTone"
                type="text"
                placeholder="VD: Xưng em - anh/chị, thân thiện..."
                class="w-full rounded-xl border border-slate-200 bg-white px-2.5 py-1.5 text-xs text-slate-800 outline-none focus:border-emerald-600"
              />
            </div>

            <div>
              <label class="block text-[11px] font-semibold text-slate-600 mb-1">
                Bảng giá, sản phẩm & chính sách
              </label>
              <textarea
                v-model="aiPrompt"
                rows="4"
                placeholder="Nhập giá bán, khuyến mãi, phí ship của riêng Fanpage này để AI tư vấn chính xác..."
                class="w-full resize-none rounded-xl border border-slate-200 bg-white p-2.5 text-xs leading-relaxed text-slate-800 outline-none focus:border-emerald-600"
              />
            </div>

            <label class="flex items-center justify-between rounded-xl border border-emerald-200 bg-white px-3 py-2 cursor-pointer">
              <div>
                <span class="block text-xs font-bold !text-slate-900">AI Tự động sau 2 phút</span>
                <span class="block text-[10px] text-slate-500">Tự động rep nếu nhân viên không nhắn</span>
              </div>
              <input
                v-model="aiAutoReply"
                @change="persistPageConfigSilently()"
                type="checkbox"
                class="h-4 w-4 accent-emerald-600 cursor-pointer"
              />
            </label>

            <button
              type="button"
              @click="savePageAISetting"
              :disabled="savingAISetting"
              class="inline-flex w-full items-center justify-center gap-1.5 rounded-xl bg-emerald-600 px-3 py-2 text-xs font-semibold text-white transition hover:bg-emerald-700 disabled:opacity-60 cursor-pointer"
            >
              <Loader2 v-if="savingAISetting" class="h-3.5 w-3.5 animate-spin" />
              <Save v-else class="h-3.5 w-3.5" />
              <span>{{ savingAISetting ? 'Đang lưu...' : 'Lưu toàn bộ cấu hình Page' }}</span>
            </button>
          </div>

          <!-- Thông tin kỹ thuật -->
          <div class="space-y-2.5">
            <div class="rounded-xl border border-slate-200 p-3">
              <div class="flex items-center justify-between">
                <span class="flex items-center gap-1 text-[11px] font-semibold text-slate-600">
                  <Hash class="h-3 w-3 text-slate-400" />
                  <span>PSID Khách hàng</span>
                </span>
                <button
                  @click="copyText(selectedConv.customer_psid, 'psid_side')"
                  class="text-[11px] font-semibold text-emerald-700 hover:underline cursor-pointer"
                >
                  {{ copiedField === 'psid_side' ? 'Đã chép' : 'Sao chép' }}
                </button>
              </div>
              <p class="mt-1 break-all font-mono text-[11px] text-slate-700">{{ selectedConv.customer_psid }}</p>
            </div>

            <div class="rounded-xl border border-slate-200 p-3">
              <div class="flex items-center justify-between">
                <span class="flex items-center gap-1 text-[11px] font-semibold text-slate-600">
                  <Hash class="h-3 w-3 text-slate-400" />
                  <span>Mã hội thoại (Thread ID)</span>
                </span>
                <button
                  @click="copyText(selectedConv.thread_id, 'thread_side')"
                  class="text-[11px] font-semibold text-emerald-700 hover:underline cursor-pointer"
                >
                  {{ copiedField === 'thread_side' ? 'Đã chép' : 'Sao chép' }}
                </button>
              </div>
              <p class="mt-1 break-all font-mono text-[11px] text-slate-700">{{ selectedConv.thread_id }}</p>
            </div>
          </div>
        </div>

        <div v-else class="flex h-full items-center justify-center text-center text-xs text-slate-400">
          Chọn một hội thoại để xem chi tiết hồ sơ khách hàng và cài đặt kịch bản AI cho Fanpage.
        </div>
      </aside>
    </div>

    <!-- Notify nổi khi có tin nhắn mới cần rep -->
    <transition name="fade">
      <div
        v-if="newMsgAlert"
        @click="() => {
          const target = conversations.find((c) => c.id === newMsgAlert?.convId)
          if (target) selectConversation(target)
          newMsgAlert = null
        }"
        class="fixed bottom-5 right-5 z-50 flex max-w-sm items-start gap-3 rounded-2xl border border-emerald-300 bg-white p-4 shadow-xl ring-4 ring-emerald-500/10 transition hover:border-emerald-500 cursor-pointer"
      >
        <div class="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-emerald-600 text-white">
          <MessageSquare class="h-4 w-4" />
        </div>
        <div class="min-w-0 flex-1">
          <div class="flex items-center justify-between gap-2">
            <p class="truncate text-xs font-extrabold !text-slate-900">
              Tin nhắn mới • {{ newMsgAlert.customerName }}
            </p>
            <span class="shrink-0 rounded-full bg-emerald-100 px-2 py-0.5 text-[10px] font-bold text-emerald-800">
              Rep ngay
            </span>
          </div>
          <p class="mt-0.5 truncate text-[11px] font-semibold text-emerald-700">
            Page: {{ newMsgAlert.pageName }}
          </p>
          <p class="mt-1 line-clamp-2 text-xs text-slate-600">
            {{ newMsgAlert.message }}
          </p>
        </div>
        <button
          type="button"
          @click.stop="newMsgAlert = null"
          class="text-xs font-bold text-slate-400 hover:text-slate-700 cursor-pointer"
          title="Đóng thông báo"
        >
          ✕
        </button>
      </div>
    </transition>
  </div>
</template>