<script setup lang="ts">
// Quét kênh / tìm từ khoá trên TikTok, YouTube, Facebook và xem kết quả ngay.
// Chỉ lưu vào kho đối thủ khi người dùng bấm Lưu.
import { computed, ref } from 'vue'
import {
  AlertCircle,
  BookmarkPlus,
  Check,
  Clock,
  ExternalLink,
  Eye,
  Hash,
  Heart,
  Image,
  Link2,
  Loader2,
  MessageCircle,
  Play,
  Search,
  Share2,
  UserPlus,
  Users,
  X
} from 'lucide-vue-next'
import { api } from '../../lib/api'
import { safeHref, safeImg } from '../../lib/safeUrl'

type Platform = 'tiktok' | 'youtube' | 'facebook'
type Mode = 'keyword' | 'channel'

interface ExploreItem {
  id: string
  platform: Platform
  kind: 'video' | 'photo' | 'post'
  title: string
  thumbnail: string
  url: string
  author_name: string
  author_handle: string
  author_avatar: string
  views: number
  likes: number
  comments: number
  shares: number
  duration: number
  published_at: number
}

interface ExploreChannel {
  platform: Platform
  id: string
  handle: string
  name: string
  avatar: string
  url: string
  followers: number
  videos: number
}

interface ExploreResult {
  mode: 'channel' | 'keyword' | 'channels' | 'single'
  platform: Platform
  query: string
  channel?: ExploreChannel
  channels: ExploreChannel[]
  items: ExploreItem[]
  next_cursor: string
  has_more: boolean
  notice: string
}

const emit = defineEmits<{ (e: 'saved'): void }>()

const RECENT_KEY = 'pp_explore_recent_v1'

const query = ref('')
const platform = ref<Platform>('tiktok')
const mode = ref<Mode>('keyword')

const loading = ref(false)
const loadingMore = ref(false)
const errorMsg = ref('')
const toast = ref('')

const lastRequest = ref<{ q: string; platform: Platform; mode: Mode } | null>(null)
const result = ref<ExploreResult | null>(null)
const items = ref<ExploreItem[]>([])
const channels = ref<ExploreChannel[]>([])
const nextCursor = ref('')
const hasMore = ref(false)

const sortBy = ref<'default' | 'newest' | 'views' | 'likes'>('default')
const selected = ref<Record<string, boolean>>({})
const saved = ref<Record<string, boolean>>({})
const saving = ref(false)
const tracking = ref(false)
const brokenImg = ref<Record<string, boolean>>({})

const recent = ref<{ q: string; platform: Platform; mode: Mode }[]>(readRecent())

const platforms: { id: Platform; label: string; active: string }[] = [
  { id: 'tiktok', label: 'TikTok', active: 'bg-slate-900 text-white' },
  { id: 'youtube', label: 'YouTube', active: 'bg-red-600 text-white' },
  { id: 'facebook', label: 'Facebook', active: 'bg-blue-600 text-white' }
]

const isLink = computed(() => {
  const q = query.value.trim().toLowerCase()
  return /^[a-z][a-z0-9+.-]*:\/\//.test(q) || /(tiktok\.com|youtube\.com|youtu\.be|facebook\.com|fb\.com|fb\.watch)\//.test(q) || q.startsWith('www.')
})
const isHandle = computed(() => /^@\S+$/.test(query.value.trim()))
const autoDetected = computed(() => isLink.value || isHandle.value)

const placeholder = computed(() => {
  if (platform.value === 'facebook') return 'Dán link Page Facebook hoặc gõ từ khoá...'
  if (mode.value === 'channel') return `Gõ tên kênh ${platform.value === 'tiktok' ? 'TikTok' : 'YouTube'} cần tìm...`
  return 'Dán link kênh / video, @tên kênh hoặc gõ từ khoá...'
})

const sortedItems = computed(() => {
  const list = [...items.value]
  if (sortBy.value === 'newest') list.sort((a, b) => b.published_at - a.published_at)
  if (sortBy.value === 'views') list.sort((a, b) => b.views - a.views)
  if (sortBy.value === 'likes') list.sort((a, b) => b.likes - a.likes)
  return list
})

const selectedItems = computed(() => items.value.filter((it) => selected.value[keyOf(it)]))
const allSelected = computed(() => items.value.length > 0 && selectedItems.value.length === items.value.length)

function keyOf(it: ExploreItem) {
  return `${it.platform}:${it.id}`
}

function readRecent() {
  try {
    const raw = localStorage.getItem(RECENT_KEY)
    const parsed = raw ? JSON.parse(raw) : []
    return Array.isArray(parsed) ? parsed.slice(0, 8) : []
  } catch {
    return []
  }
}

function rememberSearch(entry: { q: string; platform: Platform; mode: Mode }) {
  const list = [entry, ...recent.value.filter((r) => !(r.q === entry.q && r.platform === entry.platform && r.mode === entry.mode))].slice(0, 8)
  recent.value = list
  try {
    localStorage.setItem(RECENT_KEY, JSON.stringify(list))
  } catch {
    /* bỏ qua */
  }
}

function clearRecent() {
  recent.value = []
  try {
    localStorage.removeItem(RECENT_KEY)
  } catch {
    /* bỏ qua */
  }
}

function showToast(msg: string) {
  toast.value = msg
  setTimeout(() => {
    if (toast.value === msg) toast.value = ''
  }, 3500)
}

async function runSearch(override?: { q: string; platform?: Platform; mode?: Mode }) {
  const q = (override?.q ?? query.value).trim()
  if (!q || loading.value) return
  if (override) {
    query.value = q
    if (override.platform) platform.value = override.platform
    if (override.mode) mode.value = override.mode
  }
  const req = { q, platform: platform.value, mode: mode.value }
  loading.value = true
  errorMsg.value = ''
  try {
    const { data } = await api.get<ExploreResult>('/competitors/explore', { params: req, timeout: 90000 })
    lastRequest.value = req
    result.value = data
    items.value = data.items || []
    channels.value = data.channels || []
    nextCursor.value = data.next_cursor || ''
    hasMore.value = Boolean(data.has_more)
    selected.value = {}
    sortBy.value = 'default'
    if (data.platform && data.platform !== platform.value) platform.value = data.platform
    rememberSearch(req)
  } catch (err: any) {
    errorMsg.value = err.response?.data?.error || (err.code === 'ECONNABORTED' ? 'Máy chủ phản hồi quá lâu, thử lại sau ít phút.' : 'Quét thất bại, thử lại sau.')
  } finally {
    loading.value = false
  }
}

async function loadMore() {
  if (!lastRequest.value || !nextCursor.value || loadingMore.value) return
  loadingMore.value = true
  errorMsg.value = ''
  try {
    const { data } = await api.get<ExploreResult>('/competitors/explore', {
      params: { ...lastRequest.value, cursor: nextCursor.value },
      timeout: 90000
    })
    const seen = new Set(items.value.map(keyOf))
    items.value.push(...(data.items || []).filter((it) => !seen.has(keyOf(it))))
    const seenCh = new Set(channels.value.map((c) => c.url))
    channels.value.push(...(data.channels || []).filter((c) => !seenCh.has(c.url)))
    nextCursor.value = data.next_cursor || ''
    hasMore.value = Boolean(data.has_more) && nextCursor.value !== ''
  } catch (err: any) {
    errorMsg.value = err.response?.data?.error || 'Không tải thêm được.'
  } finally {
    loadingMore.value = false
  }
}

function openChannel(ch: ExploreChannel) {
  runSearch({ q: ch.url, platform: ch.platform })
}

function toggleSelect(it: ExploreItem) {
  const k = keyOf(it)
  selected.value[k] = !selected.value[k]
}

function toggleSelectAll() {
  const next = !allSelected.value
  const map: Record<string, boolean> = {}
  if (next) for (const it of items.value) map[keyOf(it)] = true
  selected.value = map
}

async function saveItems(list: ExploreItem[]) {
  const pending = list.filter((it) => !saved.value[keyOf(it)])
  if (pending.length === 0 || saving.value) return
  saving.value = true
  errorMsg.value = ''
  try {
    const { data } = await api.post<{ message: string }>('/competitors/explore/save', {
      items: pending,
      keyword: result.value?.mode === 'keyword' ? result.value.query : ''
    })
    for (const it of pending) {
      saved.value[keyOf(it)] = true
      selected.value[keyOf(it)] = false
    }
    showToast(data.message)
    emit('saved')
  } catch (err: any) {
    errorMsg.value = err.response?.data?.error || 'Lưu thất bại.'
  } finally {
    saving.value = false
  }
}

async function trackChannel() {
  const ch = result.value?.channel
  if (!ch || tracking.value) return
  tracking.value = true
  errorMsg.value = ''
  try {
    const { data } = await api.post<{ message: string }>('/competitors', {
      platform: ch.platform,
      page_input: ch.platform === 'youtube' && ch.id ? `https://www.youtube.com/channel/${ch.id}` : ch.url,
      name: ch.name
    })
    showToast(data.message || 'Đã thêm kênh vào danh sách theo dõi.')
    emit('saved')
  } catch (err: any) {
    errorMsg.value = err.response?.data?.error || 'Không thêm được kênh vào danh sách theo dõi.'
  } finally {
    tracking.value = false
  }
}

function resetResults() {
  result.value = null
  items.value = []
  channels.value = []
  errorMsg.value = ''
  query.value = ''
}

const compact = new Intl.NumberFormat('vi-VN', { notation: 'compact', maximumFractionDigits: 1 })
function fmtNum(n: number) {
  return compact.format(Math.max(0, n || 0))
}

function fmtDuration(s: number) {
  if (!s) return ''
  const h = Math.floor(s / 3600)
  const m = Math.floor((s % 3600) / 60)
  const sec = String(s % 60).padStart(2, '0')
  return h ? `${h}:${String(m).padStart(2, '0')}:${sec}` : `${m}:${sec}`
}

function fmtDate(ts: number) {
  if (!ts) return ''
  const diff = Date.now() / 1000 - ts
  if (diff < 3600) return `${Math.max(1, Math.round(diff / 60))} phút trước`
  if (diff < 86400) return `${Math.round(diff / 3600)} giờ trước`
  if (diff < 86400 * 30) return `${Math.round(diff / 86400)} ngày trước`
  return new Date(ts * 1000).toLocaleDateString('vi-VN')
}

function thumbRatio(it: ExploreItem) {
  if (it.platform === 'tiktok') return 'aspect-[3/4]'
  if (it.platform === 'youtube') return 'aspect-video'
  return 'aspect-[4/3]'
}

function platformLabel(p: Platform) {
  return p === 'tiktok' ? 'TikTok' : p === 'youtube' ? 'YouTube' : 'Facebook'
}

const modeLabel = computed(() => {
  const r = result.value
  if (!r) return ''
  if (r.mode === 'keyword') return `Từ khoá “${r.query}”`
  if (r.mode === 'channels') return `Kênh khớp “${r.query}”`
  if (r.mode === 'single') return 'Video / bài từ link'
  return 'Nội dung của kênh'
})
</script>

<template>
  <div class="space-y-4">
    <!-- Ô tìm kiếm -->
    <form @submit.prevent="runSearch()" class="space-y-3 rounded-2xl border border-slate-200 bg-white p-4 shadow-2xs">
      <div class="flex flex-wrap items-center gap-2">
        <div class="inline-flex rounded-xl bg-slate-100 p-1">
          <button
            v-for="p in platforms"
            :key="p.id"
            type="button"
            @click="platform = p.id"
            class="rounded-lg px-3 py-1.5 text-xs font-bold transition"
            :class="platform === p.id ? p.active : 'text-slate-600 hover:text-slate-900'"
          >
            {{ p.label }}
          </button>
        </div>

        <div
          v-if="!autoDetected && platform !== 'facebook'"
          class="inline-flex rounded-xl border border-slate-200 bg-white p-1"
        >
          <button
            type="button"
            @click="mode = 'keyword'"
            class="inline-flex items-center gap-1 rounded-lg px-2.5 py-1.5 text-xs font-semibold transition"
            :class="mode === 'keyword' ? 'bg-emerald-600 text-white' : 'text-slate-600 hover:text-slate-900'"
          >
            <Hash class="h-3.5 w-3.5" /> Từ khoá
          </button>
          <button
            type="button"
            @click="mode = 'channel'"
            class="inline-flex items-center gap-1 rounded-lg px-2.5 py-1.5 text-xs font-semibold transition"
            :class="mode === 'channel' ? 'bg-emerald-600 text-white' : 'text-slate-600 hover:text-slate-900'"
          >
            <Users class="h-3.5 w-3.5" /> Tên kênh
          </button>
        </div>
        <span
          v-else-if="autoDetected"
          class="inline-flex items-center gap-1 rounded-lg bg-emerald-50 px-2.5 py-1.5 text-[11px] font-semibold text-emerald-700"
        >
          <Link2 class="h-3.5 w-3.5" /> {{ isLink ? 'Tự nhận diện link' : 'Quét kênh theo @tên' }}
        </span>
      </div>

      <div class="flex gap-2">
        <div class="relative min-w-0 flex-1">
          <Search class="pointer-events-none absolute left-3.5 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400" />
          <input
            v-model="query"
            type="text"
            inputmode="search"
            enterkeyhint="search"
            :placeholder="placeholder"
            class="w-full rounded-xl border border-slate-200 bg-white py-2.5 pl-10 pr-9 text-sm !text-slate-900 shadow-2xs outline-none focus:border-emerald-500 focus:ring-2 focus:ring-emerald-100"
          />
          <button
            v-if="query"
            type="button"
            @click="query = ''"
            class="absolute right-2 top-1/2 -translate-y-1/2 rounded-md p-1 text-slate-400 hover:bg-slate-100 hover:text-slate-600"
            aria-label="Xoá"
          >
            <X class="h-4 w-4" />
          </button>
        </div>
        <button
          type="submit"
          :disabled="loading || !query.trim()"
          class="inline-flex shrink-0 items-center gap-1.5 rounded-xl bg-emerald-600 px-4 py-2.5 text-sm font-bold text-white transition hover:bg-emerald-700 disabled:opacity-50"
        >
          <Loader2 v-if="loading" class="h-4 w-4 animate-spin" />
          <Search v-else class="h-4 w-4" />
          <span>{{ loading ? 'Đang quét' : 'Quét' }}</span>
        </button>
      </div>

      <div v-if="recent.length" class="flex flex-wrap items-center gap-1.5">
        <span class="text-[11px] font-semibold text-slate-400">Gần đây:</span>
        <button
          v-for="r in recent"
          :key="r.platform + r.mode + r.q"
          type="button"
          @click="runSearch(r)"
          class="max-w-[220px] truncate rounded-full border border-slate-200 bg-slate-50 px-2.5 py-1 text-[11px] font-medium text-slate-600 hover:border-emerald-300 hover:text-emerald-700"
        >
          {{ platformLabel(r.platform) }} · {{ r.q }}
        </button>
        <button type="button" @click="clearRecent" class="text-[11px] font-semibold text-slate-400 hover:text-rose-600">Xoá</button>
      </div>
    </form>

    <!-- Thông báo -->
    <div v-if="toast" class="flex items-center gap-2 rounded-xl border border-emerald-200 bg-emerald-50 px-4 py-2.5 text-xs font-semibold text-emerald-800">
      <Check class="h-4 w-4 shrink-0" /> <span>{{ toast }}</span>
    </div>
    <div v-if="errorMsg" class="flex items-start gap-2 rounded-xl border border-rose-200 bg-rose-50 px-4 py-2.5 text-xs font-medium text-rose-700">
      <AlertCircle class="mt-0.5 h-4 w-4 shrink-0" /> <span>{{ errorMsg }}</span>
    </div>

    <!-- Đang quét lần đầu -->
    <div v-if="loading" class="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5">
      <div v-for="n in 10" :key="n" class="animate-pulse overflow-hidden rounded-2xl border border-slate-200 bg-white">
        <div class="aspect-[3/4] bg-slate-100" />
        <div class="space-y-2 p-3">
          <div class="h-3 rounded bg-slate-100" />
          <div class="h-3 w-2/3 rounded bg-slate-100" />
        </div>
      </div>
    </div>

    <!-- Chưa quét -->
    <div v-else-if="!result" class="rounded-2xl border border-dashed border-slate-300 bg-white p-6 text-center">
      <p class="text-sm font-bold !text-slate-900">Quét kênh hoặc tìm theo từ khoá</p>
      <ul class="mx-auto mt-3 max-w-md space-y-1.5 text-left text-xs text-slate-500">
        <li>• Dán link kênh: <span class="font-medium text-slate-700">tiktok.com/@tenkenh</span>, <span class="font-medium text-slate-700">youtube.com/@tenkenh</span>, link Page Facebook</li>
        <li>• Dán link 1 video TikTok / YouTube để xem chỉ số</li>
        <li>• Gõ <span class="font-medium text-slate-700">@tenkenh</span> để quét kênh, hoặc gõ từ khoá như <span class="font-medium text-slate-700">nước hoa nữ</span></li>
        <li>• Chọn <span class="font-medium text-slate-700">Tên kênh</span> để tìm kênh khi chưa biết @tên</li>
      </ul>
    </div>

    <template v-else>
      <!-- Thẻ kênh -->
      <div v-if="result.channel" class="flex flex-wrap items-center gap-3 rounded-2xl border border-slate-200 bg-white p-4 shadow-2xs">
        <img
          v-if="safeImg(result.channel.avatar) && !brokenImg['ch']"
          :src="safeImg(result.channel.avatar)"
          referrerpolicy="no-referrer"
          class="h-12 w-12 shrink-0 rounded-full border border-slate-100 object-cover"
          @error="brokenImg['ch'] = true"
        />
        <div v-else class="flex h-12 w-12 shrink-0 items-center justify-center rounded-full bg-emerald-50 text-emerald-600">
          <Users class="h-5 w-5" />
        </div>
        <div class="min-w-0 flex-1">
          <p class="truncate text-sm font-bold !text-slate-900">{{ result.channel.name }}</p>
          <p class="truncate text-xs text-slate-500">
            {{ platformLabel(result.channel.platform) }}
            <template v-if="result.channel.handle"> · @{{ result.channel.handle }}</template>
            <template v-if="result.channel.followers"> · {{ fmtNum(result.channel.followers) }} người theo dõi</template>
            <template v-if="result.channel.videos"> · {{ fmtNum(result.channel.videos) }} video</template>
          </p>
        </div>
        <div class="flex w-full gap-2 sm:w-auto">
          <a
            :href="safeHref(result.channel.url)"
            target="_blank"
            rel="noopener noreferrer"
            class="inline-flex flex-1 items-center justify-center gap-1.5 rounded-xl border border-slate-200 px-3 py-2 text-xs font-semibold text-slate-700 hover:bg-slate-50 sm:flex-none"
          >
            <ExternalLink class="h-3.5 w-3.5" /> Mở kênh
          </a>
          <button
            v-if="result.mode !== 'single'"
            type="button"
            @click="trackChannel"
            :disabled="tracking"
            class="inline-flex flex-1 items-center justify-center gap-1.5 rounded-xl bg-emerald-600 px-3 py-2 text-xs font-bold text-white hover:bg-emerald-700 disabled:opacity-50 sm:flex-none"
          >
            <Loader2 v-if="tracking" class="h-3.5 w-3.5 animate-spin" />
            <UserPlus v-else class="h-3.5 w-3.5" />
            Theo dõi kênh
          </button>
        </div>
      </div>

      <div v-if="result.notice" class="flex items-start gap-2 rounded-xl border border-amber-200 bg-amber-50 px-4 py-2.5 text-xs font-medium text-amber-800">
        <AlertCircle class="mt-0.5 h-4 w-4 shrink-0" /> <span>{{ result.notice }}</span>
      </div>

      <!-- Danh sách kênh khi tìm theo tên -->
      <div v-if="result.mode === 'channels'" class="space-y-3">
        <p class="text-xs font-semibold text-slate-500">{{ modeLabel }} · {{ channels.length }} kênh · bấm để xem video</p>
        <div class="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3">
          <button
            v-for="ch in channels"
            :key="ch.url"
            type="button"
            @click="openChannel(ch)"
            class="flex min-w-0 items-center gap-3 rounded-2xl border border-slate-200 bg-white p-3 text-left shadow-2xs transition hover:border-emerald-300"
          >
            <img
              v-if="safeImg(ch.avatar) && !brokenImg[ch.url]"
              :src="safeImg(ch.avatar)"
              referrerpolicy="no-referrer"
              class="h-11 w-11 shrink-0 rounded-full object-cover"
              @error="brokenImg[ch.url] = true"
            />
            <div v-else class="flex h-11 w-11 shrink-0 items-center justify-center rounded-full bg-slate-100 text-slate-400">
              <Users class="h-5 w-5" />
            </div>
            <div class="min-w-0 flex-1">
              <p class="truncate text-sm font-bold !text-slate-900">{{ ch.name }}</p>
              <p class="truncate text-xs text-slate-500">
                <template v-if="ch.handle">@{{ ch.handle }} · </template>{{ fmtNum(ch.followers) }} theo dõi
                <template v-if="ch.videos"> · {{ fmtNum(ch.videos) }} video</template>
              </p>
            </div>
          </button>
        </div>
      </div>

      <!-- Lưới video / bài -->
      <template v-else-if="items.length">
        <div class="flex flex-wrap items-center gap-2">
          <p class="mr-auto text-xs font-semibold text-slate-500">{{ modeLabel }} · {{ items.length }} mục</p>
          <select
            v-model="sortBy"
            class="rounded-lg border border-slate-200 bg-white px-2 py-1.5 text-xs font-semibold text-slate-700 outline-none"
          >
            <option value="default">Sắp xếp mặc định</option>
            <option value="newest">Mới nhất</option>
            <option value="views">Nhiều lượt xem</option>
            <option value="likes">Nhiều tim / thích</option>
          </select>
          <button type="button" @click="toggleSelectAll" class="rounded-lg border border-slate-200 bg-white px-2.5 py-1.5 text-xs font-semibold text-slate-700 hover:bg-slate-50">
            {{ allSelected ? 'Bỏ chọn' : 'Chọn tất cả' }}
          </button>
          <button
            type="button"
            @click="saveItems(selectedItems)"
            :disabled="selectedItems.length === 0 || saving"
            class="inline-flex items-center gap-1.5 rounded-lg bg-emerald-600 px-3 py-1.5 text-xs font-bold text-white hover:bg-emerald-700 disabled:opacity-40"
          >
            <Loader2 v-if="saving" class="h-3.5 w-3.5 animate-spin" />
            <BookmarkPlus v-else class="h-3.5 w-3.5" />
            Lưu {{ selectedItems.length || '' }} mục
          </button>
        </div>

        <div class="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5">
          <article
            v-for="it in sortedItems"
            :key="keyOf(it)"
            class="group flex min-w-0 flex-col overflow-hidden rounded-2xl border bg-white shadow-2xs transition"
            :class="selected[keyOf(it)] ? 'border-emerald-500 ring-2 ring-emerald-100' : 'border-slate-200 hover:border-emerald-300'"
          >
            <div class="relative overflow-hidden bg-slate-100" :class="thumbRatio(it)">
              <a :href="safeHref(it.url)" target="_blank" rel="noopener noreferrer" class="block h-full w-full">
                <img
                  v-if="safeImg(it.thumbnail) && !brokenImg[keyOf(it)]"
                  :src="safeImg(it.thumbnail)"
                  :alt="it.title"
                  loading="lazy"
                  referrerpolicy="no-referrer"
                  class="h-full w-full object-cover transition group-hover:scale-[1.02]"
                  @error="brokenImg[keyOf(it)] = true"
                />
                <div v-else class="flex h-full w-full items-center justify-center p-3 text-center text-[11px] text-slate-400">
                  {{ it.kind === 'post' ? 'Bài viết không có ảnh' : 'Không có ảnh bìa' }}
                </div>
              </a>
              <button
                type="button"
                @click="toggleSelect(it)"
                class="absolute left-2 top-2 flex h-6 w-6 items-center justify-center rounded-md border-2 transition"
                :class="selected[keyOf(it)] ? 'border-emerald-600 bg-emerald-600 text-white' : 'border-white bg-black/30 text-transparent'"
                :aria-label="selected[keyOf(it)] ? 'Bỏ chọn' : 'Chọn'"
              >
                <Check class="h-4 w-4" />
              </button>
              <span class="pointer-events-none absolute right-2 top-2 rounded-md bg-black/60 px-1.5 py-0.5 text-[10px] font-bold text-white">
                <Image v-if="it.kind === 'photo'" class="inline h-3 w-3" />
                <Play v-else-if="it.kind === 'video'" class="inline h-3 w-3" />
                {{ it.duration ? fmtDuration(it.duration) : it.kind === 'photo' ? 'Ảnh' : it.kind === 'post' ? 'Bài' : '' }}
              </span>
              <span
                v-if="it.views"
                class="pointer-events-none absolute bottom-2 left-2 inline-flex items-center gap-1 rounded-md bg-black/60 px-1.5 py-0.5 text-[11px] font-bold text-white"
              >
                <Eye class="h-3 w-3" /> {{ fmtNum(it.views) }}
              </span>
            </div>

            <div class="flex min-w-0 flex-1 flex-col gap-1.5 p-2.5">
              <p class="line-clamp-2 break-words text-xs font-semibold leading-snug !text-slate-900" :title="it.title">
                {{ it.title || '(Không có mô tả)' }}
              </p>
              <p class="truncate text-[11px] text-slate-500">
                {{ it.author_name || (it.author_handle ? '@' + it.author_handle : '') }}
              </p>
              <div class="mt-auto flex flex-wrap items-center gap-x-2.5 gap-y-1 text-[11px] text-slate-500">
                <span v-if="it.likes" class="inline-flex items-center gap-0.5"><Heart class="h-3 w-3" /> {{ fmtNum(it.likes) }}</span>
                <span v-if="it.comments" class="inline-flex items-center gap-0.5"><MessageCircle class="h-3 w-3" /> {{ fmtNum(it.comments) }}</span>
                <span v-if="it.shares" class="inline-flex items-center gap-0.5"><Share2 class="h-3 w-3" /> {{ fmtNum(it.shares) }}</span>
                <span v-if="it.published_at" class="inline-flex items-center gap-0.5"><Clock class="h-3 w-3" /> {{ fmtDate(it.published_at) }}</span>
              </div>
              <div class="flex gap-1.5 pt-1">
                <a
                  :href="safeHref(it.url)"
                  target="_blank"
                  rel="noopener noreferrer"
                  class="inline-flex flex-1 items-center justify-center gap-1 rounded-lg border border-slate-200 py-1.5 text-[11px] font-semibold text-slate-600 hover:bg-slate-50"
                >
                  <ExternalLink class="h-3 w-3" /> Mở
                </a>
                <button
                  type="button"
                  @click="saveItems([it])"
                  :disabled="saved[keyOf(it)] || saving"
                  class="inline-flex flex-1 items-center justify-center gap-1 rounded-lg py-1.5 text-[11px] font-bold transition"
                  :class="saved[keyOf(it)] ? 'bg-emerald-50 text-emerald-700' : 'bg-emerald-600 text-white hover:bg-emerald-700'"
                >
                  <Check v-if="saved[keyOf(it)]" class="h-3 w-3" />
                  <BookmarkPlus v-else class="h-3 w-3" />
                  {{ saved[keyOf(it)] ? 'Đã lưu' : 'Lưu' }}
                </button>
              </div>
            </div>
          </article>
        </div>
      </template>

      <div
        v-else-if="!result.notice"
        class="rounded-2xl border border-dashed border-slate-300 bg-white p-6 text-center text-xs text-slate-500"
      >
        Không có kết quả nào.
      </div>

      <div v-if="hasMore" class="flex justify-center">
        <button
          type="button"
          @click="loadMore"
          :disabled="loadingMore"
          class="inline-flex items-center gap-1.5 rounded-xl border border-slate-200 bg-white px-5 py-2.5 text-xs font-bold text-slate-700 shadow-2xs hover:border-emerald-300 hover:text-emerald-700 disabled:opacity-50"
        >
          <Loader2 v-if="loadingMore" class="h-4 w-4 animate-spin" />
          {{ loadingMore ? 'Đang tải...' : 'Xem thêm' }}
        </button>
      </div>

      <div class="flex justify-center">
        <button type="button" @click="resetResults" class="text-xs font-semibold text-slate-400 hover:text-slate-700">Quét mục khác</button>
      </div>
    </template>
  </div>
</template>
