<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import {
  ArrowLeft,
  ArrowUpDown,
  Bot,
  Calendar,
  Check,
  CheckCircle2,
  ChevronDown,
  ChevronUp,
  Copy,
  ExternalLink,
  Eye,
  Heart,
  Loader2,
  MessageCircle,
  Play,
  Plus,
  RefreshCw,
  Search,
  Share2,
  Sparkles,
  Trash2,
  TrendingUp
} from 'lucide-vue-next'
import { api } from '../lib/api'
import CompetitorMaketResearch from './CompetitorMaketResearch.vue'

interface CompetitorPageItem {
  id: number
  platform: 'facebook' | 'youtube' | 'tiktok'
  page_id: string
  name: string
  page_url: string
  avatar_url: string
  category: string
  notes: string
  last_synced: string
}

interface CompetitorPostItem {
  id: number
  platform: 'facebook' | 'youtube' | 'tiktok'
  competitor_page_id: string
  competitor_name: string
  fb_post_id: string
  content: string
  permalink_url: string
  full_picture: string
  views_count: number
  likes_count: number
  comments_count: number
  shares_count: number
  published_at: string
  ai_analysis: string
  ai_remixed_post: string
}

const router = useRouter()

const activeCompetitorTab = ref<'tracking' | 'market'>('tracking')

const competitors = ref<CompetitorPageItem[]>([])
const posts = ref<CompetitorPostItem[]>([])
const selectedCompetitorFilter = ref('all')
const selectedPlatformFilter = ref<'all' | 'youtube' | 'tiktok' | 'facebook'>('all')
const searchQuery = ref('')
const sortBy = ref<'newest' | 'engagement' | 'views'>('newest')

// Trạng thái mở rộng nội dung dài & modal thêm thủ công
const expandedPostIDs = ref<Record<number, boolean>>({})
const showManualForm = ref(false)

// Form thêm kênh đối thủ
const newPlatform = ref<'youtube' | 'tiktok' | 'facebook'>('youtube')
const newPageInput = ref('')
const newPageName = ref('')
const newPageCategory = ref('')
const addingCompetitor = ref(false)
const searchingKeyword = ref(false)

// Form thêm bài/video thủ công
const manualPageID = ref('')
const manualPermalink = ref('')
const manualContent = ref('')
const manualViews = ref(0)
const manualLikes = ref(0)
const manualComments = ref(0)
const savingManual = ref(false)

// Ngữ cảnh sản phẩm shop mình để AI remix chuẩn
const myProductContext = ref('')

const loading = ref(false)
const syncingID = ref<number | null>(null)
const syncingAll = ref(false)
const analyzingID = ref<number | null>(null)
const copiedID = ref<number | null>(null)
const noticeMsg = ref('')
const errorMsg = ref('')

// Thống kê nhanh trên thanh KPI
const totalChannels = computed(() => competitors.value.length)
const totalCollectedPosts = computed(() => posts.value.length)
const totalAIAnalyzed = computed(() => posts.value.filter((p) => Boolean(p.ai_remixed_post)).length)

const filteredPosts = computed(() => {
  const q = searchQuery.value.trim().toLowerCase()
  const list = posts.value.filter((p) => {
    const matchPlat =
      selectedPlatformFilter.value === 'all' || p.platform === selectedPlatformFilter.value
    const matchComp =
      selectedCompetitorFilter.value === 'all' ||
      p.competitor_page_id === selectedCompetitorFilter.value
    const matchText =
      !q ||
      p.content.toLowerCase().includes(q) ||
      p.competitor_name.toLowerCase().includes(q)
    return matchPlat && matchComp && matchText
  })

  if (sortBy.value === 'views') {
    return [...list].sort((a, b) => (b.views_count || 0) - (a.views_count || 0))
  }
  if (sortBy.value === 'engagement') {
    return [...list].sort(
      (a, b) =>
        b.likes_count + b.comments_count * 2 + b.shares_count * 3 -
        (a.likes_count + a.comments_count * 2 + a.shares_count * 3)
    )
  }
  return list
})

function platformBadge(plat: string) {
  if (plat === 'youtube') {
    return {
      label: 'YouTube',
      cls: 'bg-red-50 text-red-700 border-red-200',
      dot: 'bg-red-500'
    }
  }
  if (plat === 'tiktok') {
    return {
      label: 'TikTok',
      cls: 'bg-slate-900 text-white border-slate-800',
      dot: 'bg-cyan-400'
    }
  }
  return {
    label: 'Facebook',
    cls: 'bg-blue-50 text-blue-700 border-blue-200',
    dot: 'bg-blue-600'
  }
}

function formatNumber(n: number) {
  if (!n) return '0'
  if (n >= 1000000) return (n / 1000000).toFixed(1) + 'M'
  if (n >= 1000) return (n / 1000).toFixed(1) + 'K'
  return String(n)
}

function formatDate(iso: string) {
  if (!iso || iso.startsWith('0001-')) return 'Chưa quét'
  return new Date(iso).toLocaleString('vi-VN', {
    day: '2-digit',
    month: '2-digit',
    hour: '2-digit',
    minute: '2-digit'
  })
}

function toggleExpand(id: number) {
  expandedPostIDs.value[id] = !expandedPostIDs.value[id]
}

async function loadData() {
  loading.value = true
  errorMsg.value = ''
  try {
    const { data } = await api.get<{
      competitors: CompetitorPageItem[]
      posts: CompetitorPostItem[]
    }>('/competitors')
    competitors.value = data.competitors || []
    posts.value = data.posts || []
    if (!manualPageID.value && competitors.value.length > 0) {
      manualPageID.value = competitors.value[0].page_id
    }
  } catch (err: any) {
    errorMsg.value = err.response?.data?.error || 'Không thể tải dữ liệu đối thủ.'
  } finally {
    loading.value = false
  }
}

async function handleAddCompetitor() {
  if (!newPageInput.value.trim() || addingCompetitor.value) return
  addingCompetitor.value = true
  errorMsg.value = ''
  try {
    const { data } = await api.post<{ message: string }>('/competitors', {
      platform: newPlatform.value,
      page_input: newPageInput.value.trim(),
      name: newPageName.value.trim(),
      category: newPageCategory.value.trim()
    })
    noticeMsg.value = data.message
    newPageInput.value = ''
    newPageName.value = ''
    newPageCategory.value = ''
    await loadData()
    setTimeout(() => (noticeMsg.value = ''), 3500)
  } catch (err: any) {
    errorMsg.value = err.response?.data?.error || 'Thêm kênh đối thủ thất bại.'
  } finally {
    addingCompetitor.value = false
  }
}

async function handleSyncCompetitor(comp: CompetitorPageItem) {
  syncingID.value = comp.id
  errorMsg.value = ''
  try {
    const { data } = await api.post<{ message: string }>(`/competitors/${comp.id}/sync`)
    noticeMsg.value = data.message
    await loadData()
    setTimeout(() => (noticeMsg.value = ''), 4000)
  } catch (err: any) {
    errorMsg.value = err.response?.data?.error || 'Quét nội dung đối thủ thất bại.'
  } finally {
    syncingID.value = null
  }
}

async function handleSyncAllChannels() {
  if (competitors.value.length === 0 || syncingAll.value) return
  syncingAll.value = true
  errorMsg.value = ''
  let okCount = 0
  for (const comp of competitors.value) {
    try {
      await api.post(`/competitors/${comp.id}/sync`)
      okCount++
    } catch {
      // Bỏ qua kênh bị giới hạn API và tiếp tục quét các kênh còn lại
    }
  }
  await loadData()
  syncingAll.value = false
  noticeMsg.value = `Đã hoàn tất quét ${okCount}/${competitors.value.length} kênh đối thủ!`
  setTimeout(() => (noticeMsg.value = ''), 4000)
}

async function handleSearchByKeyword() {
  const kw = searchQuery.value.trim()
  if (!kw || searchingKeyword.value) return
  searchingKeyword.value = true
  errorMsg.value = ''
  try {
    const { data } = await api.get<{ message: string }>('/competitors/search', {
      params: { q: kw, platform: selectedPlatformFilter.value }
    })
    noticeMsg.value = data.message
    await loadData()
    setTimeout(() => (noticeMsg.value = ''), 4000)
  } catch (err: any) {
    errorMsg.value = err.response?.data?.error || 'Tìm kiếm theo từ khóa thất bại.'
  } finally {
    searchingKeyword.value = false
  }
}

async function handleDeleteCompetitor(id: number) {
  try {
    await api.delete(`/competitors/${id}`)
    if (
      selectedCompetitorFilter.value !== 'all' &&
      !competitors.value.some((c) => c.id !== id && c.page_id === selectedCompetitorFilter.value)
    ) {
      selectedCompetitorFilter.value = 'all'
    }
    await loadData()
  } catch (err: any) {
    errorMsg.value = err.response?.data?.error || 'Xóa kênh đối thủ thất bại.'
  }
}

async function handleCreateManualPost() {
  if (
    !manualPageID.value ||
    (!manualContent.value.trim() && !manualPermalink.value.trim()) ||
    savingManual.value
  )
    return
  savingManual.value = true
  errorMsg.value = ''
  try {
    const { data } = await api.post<{ message: string }>('/competitors/posts/manual', {
      competitor_page_id: manualPageID.value,
      permalink_url: manualPermalink.value.trim(),
      content: manualContent.value.trim(),
      views_count: Number(manualViews.value) || 0,
      likes_count: Number(manualLikes.value) || 0,
      comments_count: Number(manualComments.value) || 0
    })
    noticeMsg.value = data.message
    manualPermalink.value = ''
    manualContent.value = ''
    manualViews.value = 0
    manualLikes.value = 0
    manualComments.value = 0
    showManualForm.value = false
    await loadData()
    setTimeout(() => (noticeMsg.value = ''), 3500)
  } catch (err: any) {
    errorMsg.value = err.response?.data?.error || 'Lưu nội dung thủ công thất bại.'
  } finally {
    savingManual.value = false
  }
}

async function handleAnalyzePost(post: CompetitorPostItem) {
  analyzingID.value = post.id
  errorMsg.value = ''
  try {
    const { data } = await api.post<{ message: string; post: CompetitorPostItem }>(
      `/competitors/posts/${post.id}/analyze`,
      { product_context: myProductContext.value }
    )
    const idx = posts.value.findIndex((p) => p.id === post.id)
    if (idx !== -1 && data.post) {
      posts.value[idx] = data.post
    }
    noticeMsg.value = data.message
    setTimeout(() => (noticeMsg.value = ''), 3500)
  } catch (err: any) {
    errorMsg.value = err.response?.data?.error || 'AI phân tích nội dung thất bại.'
  } finally {
    analyzingID.value = null
  }
}

async function copyRemixedText(post: CompetitorPostItem) {
  try {
    await navigator.clipboard.writeText(post.ai_remixed_post)
    copiedID.value = post.id
    setTimeout(() => {
      if (copiedID.value === post.id) copiedID.value = null
    }, 2000)
  } catch {
    // ignore
  }
}

function sendToScheduler(remixedContent: string) {
  localStorage.setItem('draft_scheduled_post_content', remixedContent)
  router.push('/posts')
}

onMounted(loadData)
</script>

<template>
  <div class="min-h-screen bg-slate-50 text-slate-900">
    <!-- Top Header -->
    <header class="sticky top-0 z-30 flex h-14 items-center justify-between border-b border-slate-200 bg-white/95 px-5 backdrop-blur-xs">
      <div class="flex items-center gap-3 min-w-0">
        <button
          @click="router.push('/')"
          class="lg:hidden inline-flex shrink-0 items-center gap-1.5 rounded-xl border border-slate-200 bg-white px-3 py-1.5 text-xs font-semibold text-slate-700 transition hover:border-emerald-200 hover:bg-emerald-50 hover:text-emerald-800 cursor-pointer"
        >
          <ArrowLeft class="h-3.5 w-3.5" />
          <span>Trang chủ</span>
        </button>

        <div class="h-5 w-px shrink-0 bg-slate-200 lg:hidden" />

        <div class="flex items-center gap-2.5 min-w-0">
          <div class="flex h-8 w-8 shrink-0 items-center justify-center rounded-xl bg-emerald-600 text-white shadow-2xs">
            <Eye class="h-4 w-4" />
          </div>
          <div class="min-w-0">
            <h1 class="truncate !text-sm !leading-tight font-bold !text-slate-900">
              Nghiên cứu & Bám kênh Đối thủ
            </h1>
          </div>
        </div>
      </div>

      <div class="flex items-center gap-2 shrink-0">
        <button
          v-if="activeCompetitorTab === 'tracking' && competitors.length > 0"
          type="button"
          @click="handleSyncAllChannels"
          :disabled="syncingAll"
          class="hidden sm:inline-flex items-center gap-1.5 rounded-xl border border-emerald-200 bg-emerald-50 px-3 py-1.5 text-xs font-semibold text-emerald-800 transition hover:bg-emerald-100 disabled:opacity-50 cursor-pointer"
        >
          <RefreshCw class="h-3.5 w-3.5" :class="{ 'animate-spin': syncingAll }" />
          <span>{{ syncingAll ? 'Đang quét tất cả...' : 'Quét tất cả kênh' }}</span>
        </button>

        <button
          @click="router.push('/posts')"
          class="inline-flex items-center gap-1.5 rounded-xl bg-emerald-600 px-3.5 py-1.5 text-xs font-semibold text-white shadow-2xs transition hover:bg-emerald-700 cursor-pointer"
        >
          <Calendar class="h-3.5 w-3.5" />
          <span>Lịch đăng bài</span>
        </button>
      </div>
    </header>

    <!-- Thanh chuyển 2 Tab chức năng -->
    <div class="border-b border-slate-200 bg-white px-5 pt-1">
      <div class="mx-auto flex max-w-[1400px] items-center gap-3 overflow-x-auto">
        <button
          type="button"
          @click="activeCompetitorTab = 'tracking'"
          class="inline-flex shrink-0 items-center gap-1.5 border-b-2 px-3 py-2.5 text-xs font-bold transition cursor-pointer"
          :class="
            activeCompetitorTab === 'tracking'
              ? 'border-emerald-600 text-emerald-700'
              : 'border-transparent text-slate-500 hover:text-slate-800'
          "
        >
          <Eye class="h-3.5 w-3.5" />
          <span>Theo dõi kênh đối thủ</span>
        </button>

        <button
          type="button"
          @click="activeCompetitorTab = 'market'"
          class="inline-flex shrink-0 items-center gap-1.5 border-b-2 px-3 py-2.5 text-xs font-bold transition cursor-pointer"
          :class="
            activeCompetitorTab === 'market'
              ? 'border-emerald-600 text-emerald-700'
              : 'border-transparent text-slate-500 hover:text-slate-800'
          "
        >
          <TrendingUp class="h-3.5 w-3.5" />
          <span>Nghiên cứu thị trường & Sinh kịch bản</span>
        </button>
      </div>
    </div>

    <!-- Toast Thông báo -->
    <div
      v-if="noticeMsg"
      class="flex items-center justify-center gap-2 border-b border-emerald-200 bg-emerald-50 px-4 py-2 text-xs font-semibold text-emerald-800"
    >
      <CheckCircle2 class="h-4 w-4 text-emerald-600" />
      <span>{{ noticeMsg }}</span>
    </div>
    <div
      v-if="errorMsg"
      class="border-b border-red-200 bg-red-50 px-4 py-2 text-center text-xs font-semibold text-red-700"
    >
      {{ errorMsg }}
    </div>

    <!-- TAB 1: Theo dõi kênh & nội dung đối thủ -->
    <main v-if="activeCompetitorTab === 'tracking'" class="mx-auto max-w-[1400px] space-y-5 p-5">
      <!-- Thanh Thống kê nhanh (KPI Bar) -->
      <div class="grid grid-cols-1 gap-3.5 sm:grid-cols-3">
        <div class="flex items-center justify-between rounded-2xl border border-slate-200 bg-white px-4 py-3.5 shadow-2xs">
          <div>
            <p class="text-[11px] font-semibold uppercase tracking-wider text-slate-400">Kênh đang bám</p>
            <p class="mt-0.5 text-xl font-extrabold !text-slate-900">{{ totalChannels }} <span class="text-xs font-medium text-slate-500">kênh</span></p>
          </div>
          <div class="flex h-10 w-10 items-center justify-center rounded-xl bg-blue-50 text-blue-600">
            <Eye class="h-5 w-5" />
          </div>
        </div>

        <div class="flex items-center justify-between rounded-2xl border border-slate-200 bg-white px-4 py-3.5 shadow-2xs">
          <div>
            <p class="text-[11px] font-semibold uppercase tracking-wider text-slate-400">Video & Bài thu thập</p>
            <p class="mt-0.5 text-xl font-extrabold !text-slate-900">{{ totalCollectedPosts }} <span class="text-xs font-medium text-slate-500">mẫu nội dung</span></p>
          </div>
          <div class="flex h-10 w-10 items-center justify-center rounded-xl bg-amber-50 text-amber-600">
            <TrendingUp class="h-5 w-5" />
          </div>
        </div>

        <div class="flex items-center justify-between rounded-2xl border border-slate-200 bg-white px-4 py-3.5 shadow-2xs">
          <div>
            <p class="text-[11px] font-semibold uppercase tracking-wider text-slate-400">Đã chuyển thể bằng AI</p>
            <p class="mt-0.5 text-xl font-extrabold !text-emerald-700">{{ totalAIAnalyzed }} <span class="text-xs font-medium text-slate-500">bài sẵn sàng đăng</span></p>
          </div>
          <div class="flex h-10 w-10 items-center justify-center rounded-xl bg-emerald-50 text-emerald-600">
            <Sparkles class="h-5 w-5" />
          </div>
        </div>
      </div>

      <!-- Bố cục 2 cột chính: Sidebar trái + Feed phải linh hoạt -->
      <div class="grid grid-cols-12 items-start gap-5">
        <!-- CỘT TRÁI: Thêm kênh + Cấu hình Shop + Danh sách kênh -->
        <aside class="col-span-12 space-y-4 lg:col-span-4 xl:col-span-3 lg:sticky lg:top-18">
          <!-- Card 1: Thêm kênh đối thủ -->
          <form
            @submit.prevent="handleAddCompetitor"
            class="rounded-2xl border border-slate-200 bg-white p-4 shadow-2xs space-y-3"
          >
            <div class="flex items-center justify-between">
              <h2 class="text-xs font-bold uppercase tracking-wider !text-slate-900">
                Thêm kênh theo dõi
              </h2>
            </div>

            <!-- Segmented Tabs Nền tảng -->
            <div class="grid grid-cols-3 gap-1 rounded-xl bg-slate-100 p-1">
              <button
                type="button"
                @click="newPlatform = 'youtube'"
                class="rounded-lg py-1.5 text-[11px] font-bold transition cursor-pointer"
                :class="newPlatform === 'youtube' ? 'bg-red-600 text-white shadow-2xs' : 'text-slate-600 hover:text-slate-900'"
              >
                YouTube
              </button>
              <button
                type="button"
                @click="newPlatform = 'tiktok'"
                class="rounded-lg py-1.5 text-[11px] font-bold transition cursor-pointer"
                :class="newPlatform === 'tiktok' ? 'bg-slate-900 text-white shadow-2xs' : 'text-slate-600 hover:text-slate-900'"
              >
                TikTok
              </button>
              <button
                type="button"
                @click="newPlatform = 'facebook'"
                class="rounded-lg py-1.5 text-[11px] font-bold transition cursor-pointer"
                :class="newPlatform === 'facebook' ? 'bg-blue-600 text-white shadow-2xs' : 'text-slate-600 hover:text-slate-900'"
              >
                Facebook
              </button>
            </div>

            <input
              v-model="newPageInput"
              type="text"
              required
              :placeholder="
                newPlatform === 'youtube'
                  ? 'Link kênh YouTube (VD: youtube.com/@kenh)...'
                  : newPlatform === 'tiktok'
                    ? 'Link TikTok hoặc @username đối thủ...'
                    : 'Link Fanpage hoặc Page ID đối thủ...'
              "
              class="w-full rounded-xl border border-slate-200 bg-slate-50/70 px-3 py-2 text-xs text-slate-900 outline-none transition focus:border-emerald-600 focus:bg-white"
            />

            <div class="grid grid-cols-2 gap-2">
              <input
                v-model="newPageName"
                type="text"
                placeholder="Tên gợi nhớ"
                class="w-full rounded-xl border border-slate-200 bg-white px-2.5 py-1.5 text-xs text-slate-800 outline-none focus:border-emerald-600"
              />
              <input
                v-model="newPageCategory"
                type="text"
                placeholder="Ngành hàng"
                class="w-full rounded-xl border border-slate-200 bg-white px-2.5 py-1.5 text-xs text-slate-800 outline-none focus:border-emerald-600"
              />
            </div>

            <button
              type="submit"
              :disabled="addingCompetitor || !newPageInput.trim()"
              class="inline-flex w-full items-center justify-center gap-1.5 rounded-xl bg-emerald-600 px-3.5 py-2 text-xs font-bold text-white shadow-2xs transition hover:bg-emerald-700 disabled:opacity-50 cursor-pointer"
            >
              <Loader2 v-if="addingCompetitor" class="h-3.5 w-3.5 animate-spin" />
              <Plus v-else class="h-3.5 w-3.5" />
              <span>Thêm vào danh sách bám</span>
            </button>
          </form>

          <!-- Card 2: Bối cảnh sản phẩm shop mình -->
          <div class="rounded-2xl border border-emerald-200/90 bg-emerald-50/50 p-4 space-y-2">
            <label class="flex items-center gap-1.5 text-xs font-bold text-emerald-950">
              <Bot class="h-4 w-4 text-emerald-600" />
              <span>Sản phẩm & Ưu đãi của Shop bạn</span>
            </label>
            <textarea
              v-model="myProductContext"
              rows="3"
              placeholder="VD: Micro không dây Hamax 03 Pro giảm 50%, bảo hành 12 tháng, miễn phí ship toàn quốc..."
              class="w-full resize-none rounded-xl border border-emerald-200 bg-white p-2.5 text-xs leading-relaxed text-slate-800 outline-none focus:border-emerald-600"
            />
          </div>

          <!-- Card 3: Danh sách kênh đang theo dõi -->
          <div class="rounded-2xl border border-slate-200 bg-white p-4 shadow-2xs space-y-3">
            <div class="flex items-center justify-between">
              <h3 class="text-xs font-bold uppercase tracking-wider !text-slate-900">
                Kênh đang bám ({{ competitors.length }})
              </h3>
              <button
                v-if="selectedCompetitorFilter !== 'all'"
                @click="selectedCompetitorFilter = 'all'"
                class="text-[11px] font-semibold text-emerald-700 hover:underline cursor-pointer"
              >
                Bỏ lọc kênh
              </button>
            </div>

            <div v-if="competitors.length === 0" class="rounded-xl border border-dashed border-slate-200 py-6 text-center text-xs text-slate-400">
              Chưa có kênh đối thủ nào.
            </div>

            <div v-else class="max-h-[420px] space-y-2 overflow-y-auto pr-0.5">
              <div
                v-for="comp in competitors"
                :key="comp.id"
                @click="
                  selectedCompetitorFilter =
                    selectedCompetitorFilter === comp.page_id ? 'all' : comp.page_id
                "
                class="group rounded-xl border p-2.5 transition cursor-pointer"
                :class="
                  selectedCompetitorFilter === comp.page_id
                    ? 'border-emerald-500 bg-emerald-50/60 ring-1 ring-emerald-500/30'
                    : 'border-slate-200 bg-slate-50/40 hover:border-slate-300 hover:bg-white'
                "
              >
                <div class="flex items-start justify-between gap-2">
                  <div class="min-w-0 flex-1">
                    <div class="flex items-center gap-1.5">
                      <span
                        class="shrink-0 rounded border px-1.5 py-0.5 text-[9px] font-extrabold uppercase"
                        :class="platformBadge(comp.platform).cls"
                      >
                        {{ platformBadge(comp.platform).label }}
                      </span>
                      <p class="truncate text-xs font-bold !text-slate-900">{{ comp.name }}</p>
                    </div>
                    <p class="mt-1 text-[10px] text-slate-500">
                      Quét: {{ formatDate(comp.last_synced) }}
                    </p>
                  </div>

                  <div class="flex items-center gap-1 shrink-0">
                    <a
                      :href="comp.page_url"
                      target="_blank"
                      rel="noopener noreferrer"
                      @click.stop
                      class="rounded-lg p-1 text-slate-400 hover:bg-slate-100 hover:text-blue-600"
                      title="Mở trang gốc"
                    >
                      <ExternalLink class="h-3.5 w-3.5" />
                    </a>
                    <button
                      type="button"
                      @click.stop="handleDeleteCompetitor(comp.id)"
                      class="rounded-lg p-1 text-slate-400 hover:bg-red-50 hover:text-red-600 cursor-pointer"
                      title="Xóa kênh"
                    >
                      <Trash2 class="h-3.5 w-3.5" />
                    </button>
                  </div>
                </div>

                <button
                  type="button"
                  @click.stop="handleSyncCompetitor(comp)"
                  :disabled="syncingID === comp.id"
                  class="mt-2 inline-flex w-full items-center justify-center gap-1.5 rounded-lg border border-slate-200 bg-white px-2.5 py-1.5 text-[11px] font-semibold text-slate-700 transition hover:border-emerald-300 hover:bg-emerald-50 hover:text-emerald-800 disabled:opacity-50 cursor-pointer"
                >
                  <RefreshCw class="h-3 w-3 text-emerald-600" :class="{ 'animate-spin': syncingID === comp.id }" />
                  <span>{{ syncingID === comp.id ? 'Đang quét dữ liệu...' : 'Quét bài / video mới' }}</span>
                </button>
              </div>
            </div>
          </div>
        </aside>

        <!-- CỘT PHẢI: Thanh công cụ lọc + Danh sách bài viết / video đối thủ -->
        <section class="col-span-12 space-y-4 lg:col-span-8 xl:col-span-9">
          <!-- Filter & Search Bar -->
          <div class="flex flex-col gap-3 rounded-2xl border border-slate-200 bg-white p-4 shadow-2xs xl:flex-row xl:items-center xl:justify-between">
            <!-- Tabs nền tảng -->
            <div class="flex flex-wrap items-center gap-1.5">
              <button
                v-for="tab in [
                  { id: 'all', label: 'Tất cả' },
                  { id: 'youtube', label: 'YouTube' },
                  { id: 'tiktok', label: 'TikTok' },
                  { id: 'facebook', label: 'Facebook' }
                ]"
                :key="tab.id"
                @click="selectedPlatformFilter = tab.id as any"
                class="rounded-xl px-3 py-1.5 text-xs font-bold transition cursor-pointer"
                :class="
                  selectedPlatformFilter === tab.id
                    ? 'bg-emerald-600 text-white shadow-2xs'
                    : 'bg-slate-100 text-slate-600 hover:bg-slate-200'
                "
              >
                {{ tab.label }}
              </button>
            </div>

            <!-- Search + Sort + Nút dán thủ công -->
            <div class="flex flex-wrap items-center gap-2">
              <div class="flex items-center gap-1.5 flex-1 sm:w-72">
                <div class="relative flex-1">
                  <Search class="pointer-events-none absolute left-2.5 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-slate-400" />
                  <input
                    v-model="searchQuery"
                    @keydown.enter="handleSearchByKeyword"
                    type="text"
                    placeholder="Từ khóa (VD: nước hoa, micro)..."
                    class="w-full rounded-xl border border-slate-200 bg-slate-50/70 py-1.5 pl-8 pr-2.5 text-xs text-slate-800 outline-none focus:border-emerald-600 focus:bg-white"
                  />
                </div>
                <button
                  type="button"
                  @click="handleSearchByKeyword"
                  :disabled="searchingKeyword || !searchQuery.trim()"
                  class="inline-flex shrink-0 items-center gap-1 rounded-xl bg-slate-900 px-3 py-1.5 text-xs font-bold text-white transition hover:bg-slate-800 disabled:opacity-40 cursor-pointer"
                >
                  <Loader2 v-if="searchingKeyword" class="h-3.5 w-3.5 animate-spin" />
                  <span>{{ searchingKeyword ? 'Đang tìm...' : 'Tìm' }}</span>
                </button>
              </div>

              <div class="inline-flex items-center gap-1 rounded-xl border border-slate-200 bg-white px-2.5 py-1.5 text-xs">
                <ArrowUpDown class="h-3.5 w-3.5 text-slate-400" />
                <select
                  v-model="sortBy"
                  class="bg-transparent text-xs font-semibold text-slate-700 outline-none cursor-pointer"
                >
                  <option value="newest">Mới nhất</option>
                  <option value="engagement">Tương tác cao</option>
                  <option value="views">Lượt xem cao</option>
                </select>
              </div>

              <button
                v-if="competitors.length > 0"
                type="button"
                @click="showManualForm = !showManualForm"
                class="inline-flex items-center gap-1.5 rounded-xl border border-emerald-200 bg-emerald-50 px-3 py-1.5 text-xs font-bold text-emerald-800 transition hover:bg-emerald-100 cursor-pointer"
              >
                <Plus class="h-3.5 w-3.5" />
                <span>Dán bài / link mẫu</span>
              </button>
            </div>
          </div>

          <!-- Form dán nhanh Link Video TikTok/YouTube hoặc nội dung thủ công -->
          <form
            v-if="showManualForm && competitors.length > 0"
            @submit.prevent="handleCreateManualPost"
            class="rounded-2xl border border-emerald-200 bg-white p-4 shadow-sm space-y-3"
          >
            <div class="flex items-center justify-between">
              <span class="text-xs font-bold text-emerald-950">
                Thêm nhanh Link Video TikTok / YouTube (tự bóc caption) hoặc nội dung Facebook đối thủ
              </span>
              <button
                type="button"
                @click="showManualForm = false"
                class="text-xs font-semibold text-slate-400 hover:text-slate-600 cursor-pointer"
              >
                Đóng
              </button>
            </div>

            <div class="grid grid-cols-1 gap-2.5 sm:grid-cols-2">
              <select
                v-model="manualPageID"
                class="rounded-xl border border-slate-200 bg-slate-50 px-3 py-2 text-xs font-semibold text-slate-800 outline-none focus:border-emerald-600"
              >
                <option v-for="c in competitors" :key="c.page_id" :value="c.page_id">
                  [{{ c.platform.toUpperCase() }}] {{ c.name }}
                </option>
              </select>
              <input
                v-model="manualPermalink"
                type="url"
                placeholder="Dán Link video TikTok / YouTube (tùy chọn)..."
                class="rounded-xl border border-slate-200 px-3 py-2 text-xs outline-none focus:border-emerald-600"
              />
            </div>

            <div class="grid grid-cols-3 gap-2.5">
              <input
                v-model.number="manualViews"
                type="number"
                min="0"
                placeholder="Lượt xem"
                class="rounded-xl border border-slate-200 px-3 py-1.5 text-xs outline-none focus:border-emerald-600"
              />
              <input
                v-model.number="manualLikes"
                type="number"
                min="0"
                placeholder="Lượt Tim / Like"
                class="rounded-xl border border-slate-200 px-3 py-1.5 text-xs outline-none focus:border-emerald-600"
              />
              <input
                v-model.number="manualComments"
                type="number"
                min="0"
                placeholder="Bình luận"
                class="rounded-xl border border-slate-200 px-3 py-1.5 text-xs outline-none focus:border-emerald-600"
              />
            </div>

            <textarea
              v-model="manualContent"
              rows="3"
              placeholder="Dán nội dung caption / bài viết (có thể bỏ trống nếu đã dán link TikTok/YouTube ở trên)..."
              class="w-full rounded-xl border border-slate-200 p-3 text-xs text-slate-900 outline-none focus:border-emerald-600"
            />

            <div class="flex justify-end gap-2">
              <button
                type="button"
                @click="showManualForm = false"
                class="rounded-xl border border-slate-200 px-3.5 py-2 text-xs font-semibold text-slate-600 hover:bg-slate-50 cursor-pointer"
              >
                Hủy
              </button>
              <button
                type="submit"
                :disabled="savingManual"
                class="rounded-xl bg-emerald-600 px-4 py-2 text-xs font-bold text-white hover:bg-emerald-700 disabled:opacity-50 cursor-pointer"
              >
                {{ savingManual ? 'Đang lấy dữ liệu...' : 'Lưu mẫu nội dung' }}
              </button>
            </div>
          </form>

          <!-- Trạng thái Loading / Empty -->
          <div v-if="loading" class="flex items-center justify-center gap-2 rounded-2xl border border-slate-200 bg-white py-16 text-xs text-slate-500">
            <Loader2 class="h-4 w-4 animate-spin text-emerald-600" />
            <span>Đang tải nội dung đối thủ...</span>
          </div>

          <div
            v-else-if="filteredPosts.length === 0"
            class="rounded-2xl border border-slate-200 bg-white p-12 text-center"
          >
            <div class="mx-auto flex h-12 w-12 items-center justify-center rounded-2xl bg-emerald-50 text-emerald-600">
              <Eye class="h-6 w-6" />
            </div>
            <h3 class="mt-3 text-sm font-bold !text-slate-900">Chưa có bài viết hoặc video nào</h3>
            <p class="mt-1 text-xs text-slate-500">
              Thêm kênh YouTube/TikTok ở cột bên trái và bấm <strong>Quét bài / video mới</strong> để bắt đầu theo dõi.
            </p>
          </div>

          <!-- Danh sách thẻ nội dung đối thủ -->
          <div v-else class="space-y-4">
            <article
              v-for="post in filteredPosts"
              :key="post.id"
              class="overflow-hidden rounded-2xl border border-slate-200 bg-white shadow-2xs transition hover:border-slate-300"
            >
              <!-- Card Header -->
              <div class="flex flex-wrap items-center justify-between gap-2 border-b border-slate-100 bg-slate-50/50 px-5 py-3">
                <div class="flex items-center gap-2 min-w-0">
                  <span
                    class="inline-flex shrink-0 items-center gap-1.5 rounded-lg border px-2 py-0.5 text-[11px] font-bold"
                    :class="platformBadge(post.platform).cls"
                  >
                    <span class="h-1.5 w-1.5 rounded-full" :class="platformBadge(post.platform).dot" />
                    <span>{{ platformBadge(post.platform).label }}</span>
                  </span>
                  <span class="truncate text-xs font-bold !text-slate-900">
                    {{ post.competitor_name }}
                  </span>
                  <span class="shrink-0 text-[11px] text-slate-400">
                    • {{ formatDate(post.published_at) }}
                  </span>
                </div>

                <!-- Metrics Pills -->
                <div class="flex items-center gap-2 text-xs font-semibold">
                  <span
                    v-if="post.views_count > 0"
                    class="inline-flex items-center gap-1 rounded-lg bg-purple-50 px-2 py-1 text-[11px] text-purple-700"
                  >
                    <Play class="h-3 w-3" />
                    {{ formatNumber(post.views_count) }}
                  </span>
                  <span class="inline-flex items-center gap-1 rounded-lg bg-rose-50 px-2 py-1 text-[11px] text-rose-700">
                    <Heart class="h-3 w-3" />
                    {{ formatNumber(post.likes_count) }}
                  </span>
                  <span class="inline-flex items-center gap-1 rounded-lg bg-blue-50 px-2 py-1 text-[11px] text-blue-700">
                    <MessageCircle class="h-3 w-3" />
                    {{ formatNumber(post.comments_count) }}
                  </span>
                  <span
                    v-if="post.shares_count > 0"
                    class="inline-flex items-center gap-1 rounded-lg bg-emerald-50 px-2 py-1 text-[11px] text-emerald-700"
                  >
                    <Share2 class="h-3 w-3" />
                    {{ formatNumber(post.shares_count) }}
                  </span>
                </div>
              </div>

              <!-- Card Body -->
              <div class="p-5 space-y-4">
                <div class="flex flex-col gap-4 sm:flex-row">
                  <a
                    v-if="post.full_picture"
                    :href="post.permalink_url || '#'"
                    target="_blank"
                    rel="noopener noreferrer"
                    class="group relative h-32 w-full shrink-0 overflow-hidden rounded-xl border border-slate-200 bg-slate-100 sm:w-52"
                  >
                    <img
                      :src="post.full_picture"
                      alt="Thumbnail"
                      referrerpolicy="no-referrer"
                      class="h-full w-full object-cover transition duration-300 group-hover:scale-105"
                    />
                  </a>

                  <div class="min-w-0 flex-1 space-y-2">
                    <p
                      class="whitespace-pre-wrap text-[13.5px] leading-relaxed text-slate-800"
                      :class="{ 'line-clamp-4': !expandedPostIDs[post.id] && post.content.length > 280 }"
                    >
                      {{ post.content }}
                    </p>

                    <button
                      v-if="post.content.length > 280"
                      type="button"
                      @click="toggleExpand(post.id)"
                      class="inline-flex items-center gap-1 text-xs font-semibold text-emerald-700 hover:underline cursor-pointer"
                    >
                      <span>{{ expandedPostIDs[post.id] ? 'Thu gọn' : 'Xem toàn bộ nội dung' }}</span>
                      <ChevronUp v-if="expandedPostIDs[post.id]" class="h-3.5 w-3.5" />
                      <ChevronDown v-else class="h-3.5 w-3.5" />
                    </button>
                  </div>
                </div>

                <!-- Card Action Footer -->
                <div class="flex flex-wrap items-center justify-between gap-2 border-t border-slate-100 pt-3">
                  <a
                    v-if="post.permalink_url"
                    :href="post.permalink_url"
                    target="_blank"
                    rel="noopener noreferrer"
                    class="inline-flex items-center gap-1.5 text-xs font-semibold text-slate-600 transition hover:text-blue-600"
                  >
                    <ExternalLink class="h-3.5 w-3.5" />
                    <span>Mở video / bài gốc</span>
                  </a>
                  <span v-else />

                  <button
                    type="button"
                    @click="handleAnalyzePost(post)"
                    :disabled="analyzingID === post.id"
                    class="inline-flex items-center gap-1.5 rounded-xl bg-emerald-600 px-4 py-2 text-xs font-bold text-white shadow-2xs transition hover:bg-emerald-700 disabled:opacity-50 cursor-pointer"
                  >
                    <Loader2 v-if="analyzingID === post.id" class="h-3.5 w-3.5 animate-spin" />
                    <Sparkles v-else class="h-3.5 w-3.5" />
                    <span>
                      {{
                        analyzingID === post.id
                          ? 'AI đang bóc tách...'
                          : post.ai_remixed_post
                            ? 'AI Phân tích lại'
                            : 'AI Phân tích & Viết thành bài FB'
                      }}
                    </span>
                  </button>
                </div>

                <!-- Kết quả AI Phân tích & Viết lại chia 2 cột trực quan -->
                <div
                  v-if="post.ai_analysis || post.ai_remixed_post"
                  class="grid grid-cols-1 gap-4 rounded-2xl border border-emerald-200 bg-emerald-50/40 p-4 xl:grid-cols-12"
                >
                  <!-- Cột phân tích chiến thuật -->
                  <div v-if="post.ai_analysis" class="space-y-2 xl:col-span-5">
                    <div class="flex items-center gap-1.5 text-xs font-bold text-emerald-950">
                      <Bot class="h-4 w-4 text-emerald-600" />
                      <span>Bóc tách công thức đối thủ</span>
                    </div>
                    <div class="rounded-xl border border-emerald-200/70 bg-white/80 p-3 text-xs leading-relaxed text-slate-700 whitespace-pre-wrap">
                      {{ post.ai_analysis }}
                    </div>
                  </div>

                  <!-- Cột bài viết Facebook mới -->
                  <div
                    v-if="post.ai_remixed_post"
                    class="space-y-2"
                    :class="post.ai_analysis ? 'xl:col-span-7' : 'xl:col-span-12'"
                  >
                    <div class="flex flex-wrap items-center justify-between gap-2">
                      <span class="text-xs font-bold text-emerald-950">
                        Bài viết Facebook mới cho Shop bạn
                      </span>
                      <div class="flex items-center gap-1.5">
                        <button
                          type="button"
                          @click="copyRemixedText(post)"
                          class="inline-flex items-center gap-1 rounded-lg border border-emerald-200 bg-white px-2.5 py-1 text-[11px] font-semibold text-emerald-800 hover:bg-emerald-50 cursor-pointer"
                        >
                          <Check v-if="copiedID === post.id" class="h-3 w-3 text-emerald-600" />
                          <Copy v-else class="h-3 w-3" />
                          <span>{{ copiedID === post.id ? 'Đã chép' : 'Sao chép' }}</span>
                        </button>

                        <button
                          type="button"
                          @click="sendToScheduler(post.ai_remixed_post)"
                          class="inline-flex items-center gap-1 rounded-lg bg-emerald-600 px-3 py-1 text-[11px] font-bold text-white shadow-2xs hover:bg-emerald-700 cursor-pointer"
                        >
                          <Calendar class="h-3 w-3" />
                          <span>Đẩy sang Lịch đăng bài →</span>
                        </button>
                      </div>
                    </div>

                    <div class="rounded-xl border border-emerald-200 bg-white p-3.5 text-xs leading-relaxed text-slate-900 whitespace-pre-wrap shadow-2xs">
                      {{ post.ai_remixed_post }}
                    </div>
                  </div>
                </div>
              </div>
            </article>
          </div>
        </section>
      </div>
    </main>

    <!-- TAB 2: Nghiên cứu thị trường theo nhóm sản phẩm & Sinh kịch bản nhanh -->
    <div v-else class="mx-auto max-w-[1400px]">
      <CompetitorMaketResearch />
    </div>
  </div>
</template>