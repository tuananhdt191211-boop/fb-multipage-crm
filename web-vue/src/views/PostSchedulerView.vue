<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import {
  ArrowLeft,
  Calendar,
  CheckCircle2,
  Clock,
  Facebook,
  Image as ImageIcon,
  Layers,
  Loader2,
  Play,
  Send,
  Trash2,
  Video
} from 'lucide-vue-next'
import { api } from '../lib/api'
import type { FacebookPageItem } from '../types/page'

interface ScheduledPostItem {
  id: number
  content: string
  media_type: 'none' | 'image' | 'video'
  media_url: string
  page_ids_json: string
  page_names_json: string
  scheduled_at: string
  status: 'pending' | 'processing' | 'published' | 'partial' | 'failed'
  success_count: number
  failed_count: number
  publish_results: string
}

const router = useRouter()

const pages = ref<FacebookPageItem[]>([])
const posts = ref<ScheduledPostItem[]>([])
const selectedPageIDs = ref<string[]>([])
const pageSearch = ref('')

const content = ref('')
const mediaType = ref<'none' | 'image' | 'video'>('none')
const mediaURL = ref('')
const scheduledDateTime = ref('')

const loading = ref(false)
const submitting = ref(false)
const publishingID = ref<number | null>(null)
const noticeMsg = ref('')
const errorMsg = ref('')

// Khung giờ vàng phổ biến
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
  // Nếu khung giờ hôm nay đã qua thì tự động chuyển sang ngày mai
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
  if (!content.value.trim() || selectedPageIDs.value.length === 0 || submitting.value) return
  submitting.value = true
  errorMsg.value = ''
  noticeMsg.value = ''

  try {
    const isoSchedule = scheduledDateTime.value
      ? new Date(scheduledDateTime.value).toISOString()
      : new Date().toISOString()

    const { data } = await api.post<{ message: string }>('/posts', {
      content: content.value.trim(),
      media_type: mediaType.value,
      media_url: mediaType.value === 'none' ? '' : mediaURL.value.trim(),
      page_ids: selectedPageIDs.value,
      scheduled_at: isoSchedule,
      publish_now: publishNow
    })

    noticeMsg.value = data.message
    content.value = ''
    mediaURL.value = ''
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
  <div class="min-h-screen bg-slate-50 text-slate-900">
    <!-- Top Header -->
    <header class="sticky top-0 z-20 flex h-14 items-center justify-between border-b border-slate-200 bg-white px-5">
      <div class="flex items-center gap-3">
        <button
          @click="router.push('/')"
          class="inline-flex items-center gap-1.5 rounded-xl border border-slate-200 bg-white px-3 py-1.5 text-xs font-semibold text-slate-700 hover:border-emerald-200 hover:bg-emerald-50 hover:text-emerald-800 cursor-pointer"
        >
          <ArrowLeft class="h-3.5 w-3.5" />
          <span>Trang chủ</span>
        </button>

        <div class="h-5 w-px bg-slate-200" />

        <div class="flex items-center gap-2.5">
          <div class="flex h-8 w-8 items-center justify-center rounded-xl bg-emerald-600 text-white">
            <Calendar class="h-4 w-4" />
          </div>
          <div>
            <h1 class="text-sm font-bold !text-slate-900">Lên lịch đăng bài đa Fanpage</h1>
          </div>
        </div>
      </div>

      <button
        @click="router.push('/inbox')"
        class="inline-flex items-center gap-1.5 rounded-xl border border-slate-200 bg-white px-3 py-1.5 text-xs font-semibold text-slate-700 hover:bg-slate-50 cursor-pointer"
      >
        <Layers class="h-3.5 w-3.5 text-emerald-600" />
        <span>Mở Hộp thư gộp</span>
      </button>
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
      class="border-b border-red-200 bg-red-50 px-4 py-2 text-center text-xs font-semibold text-red-700"
    >
      {{ errorMsg }}
    </div>

    <!-- Main Content -->
    <main class="mx-auto grid max-w-7xl grid-cols-12 gap-6 p-5">
      <!-- CỘT TRÁI: Trình soạn thảo & Chọn Fanpage -->
      <section class="col-span-12 space-y-5 lg:col-span-7">
        <div class="rounded-2xl border border-slate-200 bg-white p-5 shadow-2xs space-y-4">
          <h2 class="text-sm font-bold !text-slate-900">1. Soạn nội dung bài viết</h2>

          <textarea
            v-model="content"
            rows="6"
            placeholder="Nhập nội dung bài viết, chương trình khuyến mãi, hashtag... để đăng hàng loạt lên các Fanpage đã chọn"
            class="w-full rounded-xl border border-slate-200 bg-slate-50/50 p-3.5 text-sm leading-relaxed text-slate-900 outline-none transition focus:border-emerald-600 focus:bg-white focus:ring-2 focus:ring-emerald-100"
          />

          <!-- Chọn đính kèm Ảnh / Video -->
          <div class="space-y-2.5">
            <div class="flex flex-wrap items-center gap-2">
              <button
                type="button"
                @click="mediaType = 'none'"
                class="rounded-xl border px-3 py-1.5 text-xs font-semibold transition cursor-pointer"
                :class="mediaType === 'none' ? 'border-emerald-600 bg-emerald-50 text-emerald-800' : 'border-slate-200 text-slate-600 hover:bg-slate-50'"
              >
                Chỉ văn bản
              </button>
              <button
                type="button"
                @click="mediaType = 'image'"
                class="inline-flex items-center gap-1.5 rounded-xl border px-3 py-1.5 text-xs font-semibold transition cursor-pointer"
                :class="mediaType === 'image' ? 'border-emerald-600 bg-emerald-50 text-emerald-800' : 'border-slate-200 text-slate-600 hover:bg-slate-50'"
              >
                <ImageIcon class="h-3.5 w-3.5" />
                <span>Đính kèm Ảnh (URL)</span>
              </button>
              <button
                type="button"
                @click="mediaType = 'video'"
                class="inline-flex items-center gap-1.5 rounded-xl border px-3 py-1.5 text-xs font-semibold transition cursor-pointer"
                :class="mediaType === 'video' ? 'border-emerald-600 bg-emerald-50 text-emerald-800' : 'border-slate-200 text-slate-600 hover:bg-slate-50'"
              >
                <Video class="h-3.5 w-3.5" />
                <span>Đính kèm Video (URL)</span>
              </button>
            </div>

            <input
              v-if="mediaType !== 'none'"
              v-model="mediaURL"
              type="url"
              :placeholder="mediaType === 'image' ? 'Dán đường dẫn ảnh công khai (https://...jpg/png)' : 'Dán đường dẫn video công khai (https://...mp4)'"
              class="w-full rounded-xl border border-slate-200 bg-white px-3.5 py-2 text-xs text-slate-800 outline-none focus:border-emerald-600"
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
              class="w-full rounded-xl border border-slate-200 bg-slate-50/60 px-3.5 py-2 text-xs font-semibold text-slate-800 outline-none focus:border-emerald-600 focus:bg-white"
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
              class="w-full rounded-xl border border-slate-200 bg-white px-3 py-1.5 text-xs text-slate-800 outline-none focus:border-emerald-600"
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
                <Facebook class="h-3.5 w-3.5 shrink-0 text-blue-600" />
                <span class="truncate font-semibold text-slate-800">{{ p.name }}</span>
              </label>
            </div>
          </div>

          <!-- Nút hành động -->
          <div class="flex flex-wrap items-center justify-end gap-2.5 border-t border-slate-100 pt-4">
            <button
              type="button"
              @click="handleSubmit(true)"
              :disabled="submitting || !content.trim() || selectedPageIDs.length === 0"
              class="inline-flex items-center gap-1.5 rounded-xl border border-emerald-200 bg-emerald-50 px-4 py-2.5 text-xs font-bold text-emerald-800 transition hover:bg-emerald-100 disabled:opacity-50 cursor-pointer"
            >
              <Send class="h-3.5 w-3.5" />
              <span>Đăng ngay lập tức</span>
            </button>

            <button
              type="button"
              @click="handleSubmit(false)"
              :disabled="submitting || !content.trim() || selectedPageIDs.length === 0"
              class="inline-flex items-center gap-1.5 rounded-xl bg-emerald-600 px-5 py-2.5 text-xs font-bold text-white shadow-xs transition hover:bg-emerald-700 disabled:opacity-50 cursor-pointer"
            >
              <Loader2 v-if="submitting" class="h-3.5 w-3.5 animate-spin" />
              <Calendar v-else class="h-3.5 w-3.5" />
              <span>Lên lịch tự động đăng</span>
            </button>
          </div>
        </div>
      </section>

      <!-- CỘT PHẢI: Danh sách bài đã lên lịch & Lịch sử đăng -->
      <section class="col-span-12 space-y-4 lg:col-span-5">
        <div class="rounded-2xl border border-slate-200 bg-white p-5 shadow-2xs">
          <div class="mb-4 flex items-center justify-between">
            <h2 class="text-sm font-bold !text-slate-900">Hàng đợi & Lịch sử đăng bài ({{ posts.length }})</h2>
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

              <p class="whitespace-pre-wrap text-xs text-slate-800 line-clamp-3">
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

              <!-- Log lỗi nếu có -->
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