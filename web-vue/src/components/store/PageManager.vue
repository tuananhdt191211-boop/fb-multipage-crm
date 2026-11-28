<script setup lang="ts">
// Ô chọn trang của trình chỉnh sửa: danh sách trang (chọn để sửa), thêm trang từ mẫu (trang thường / chính sách),
// đổi tên - đường dẫn - ẩn / hiện, nhân bản, xoá, sắp xếp. Mọi thao tác trang lưu nháp trước rồi gọi máy chủ,
// nên danh sách luôn khớp với máy chủ; nội dung trang chỉ đi công khai khi bấm "Xuất bản".
import { computed, ref } from 'vue'
import { ChevronDown, ChevronUp, Copy, EyeOff, FileText, Home, Loader2, Plus, Settings2, ShieldCheck, Trash2, X } from 'lucide-vue-next'
import { api } from '../../lib/api'
import { pageSubPath, templateLabel, type PageTemplate, type ServerPage, type SitePage } from '../../lib/pages'

const props = defineProps<{
  siteId: number | string
  siteSlug: string
  pages: SitePage[]
  currentId: number
  templates: PageTemplate[]
  maxPages: number
  ensureSaved: () => Promise<boolean>
}>()
const emit = defineEmits<{
  (e: 'select', id: number): void
  (e: 'changed', pages: ServerPage[], selectId?: number): void
}>()

const open = ref(false)
const busy = ref(false)
const error = ref('')

const current = computed(() => props.pages.find((p) => p.id === props.currentId))
const full = computed(() => props.pages.length >= props.maxPages)
const base = computed(() => `/storefront/sites/${props.siteId}/pages`)

function errText(e: any, fallback: string) {
  return e?.response?.data?.error || fallback
}

// Lưu nháp trước (để danh sách khớp máy chủ), rồi chạy thao tác
async function run(fn: () => Promise<void>): Promise<boolean> {
  if (busy.value) return false
  busy.value = true
  error.value = ''
  try {
    if (!(await props.ensureSaved())) {
      error.value = 'Chưa lưu được bản nháp nên chưa thể thao tác với trang.'
      return false
    }
    await fn()
    return true
  } catch (e) {
    error.value = errText(e, 'Thao tác thất bại.')
    return false
  } finally {
    busy.value = false
  }
}

function choose(id: number) {
  open.value = false
  emit('select', id)
}

// ------------------------------------------------------------ Thêm trang
const addOpen = ref(false)
const tplKey = ref('blank')
const newTitle = ref('')
const newSlug = ref('')
let titleTouched = false

const groups = computed(() => [
  { name: 'Trang thường', items: props.templates.filter((t) => t.kind !== 'policy') },
  { name: 'Trang chính sách (mẫu có sẵn nội dung)', items: props.templates.filter((t) => t.kind === 'policy') }
])

function openAdd() {
  open.value = false
  error.value = ''
  tplKey.value = 'blank'
  newTitle.value = ''
  newSlug.value = ''
  titleTouched = false
  addOpen.value = true
}

function pickTemplate(t: PageTemplate) {
  tplKey.value = t.key
  if (!titleTouched) newTitle.value = t.key === 'blank' ? '' : templateLabel(t).title
}

async function create() {
  const ok = await run(async () => {
    const { data } = await api.post<{ page: ServerPage; pages: ServerPage[] }>(base.value, {
      template: tplKey.value,
      title: newTitle.value.trim(),
      slug: newSlug.value.trim()
    })
    emit('changed', data.pages, data.page.id)
    addOpen.value = false
  })
  return ok
}

// ------------------------------------------------------------ Cài đặt một trang
const editing = ref<SitePage | null>(null)
const eTitle = ref('')
const eSlug = ref('')
const eVisible = ref(true)
const copied = ref(false)

function openSettings(p: SitePage) {
  open.value = false
  error.value = ''
  copied.value = false
  editing.value = p
  eTitle.value = p.title
  eSlug.value = p.slug
  eVisible.value = p.is_visible
}

const publicPath = computed(() => (editing.value ? `/s/${props.siteSlug}${pageSubPath(editing.value)}` : ''))
const linkCode = computed(() => (editing.value ? (editing.value.kind === 'home' ? 'page:home' : `page:${editing.value.slug}`) : ''))

async function copyLink() {
  try {
    await navigator.clipboard.writeText(linkCode.value)
    copied.value = true
  } catch {
    /* trình duyệt chặn clipboard: người dùng tự chép từ ô hiển thị */
  }
}

async function saveSettings() {
  const p = editing.value
  if (!p) return
  const patch: Record<string, unknown> = {}
  if (eTitle.value.trim() !== p.title) patch.title = eTitle.value.trim()
  if (p.kind !== 'home') {
    if (eSlug.value.trim() !== p.slug) patch.slug = eSlug.value.trim()
    if (eVisible.value !== p.is_visible) patch.is_visible = eVisible.value
  }
  if (!Object.keys(patch).length) {
    editing.value = null
    return
  }
  await run(async () => {
    const { data } = await api.patch<{ pages: ServerPage[] }>(`${base.value}/${p.id}`, patch)
    emit('changed', data.pages)
    editing.value = null
  })
}

async function duplicate() {
  const p = editing.value
  if (!p) return
  await run(async () => {
    const { data } = await api.post<{ page: ServerPage; pages: ServerPage[] }>(`${base.value}/${p.id}/duplicate`)
    emit('changed', data.pages, data.page.id)
    editing.value = null
  })
}

async function remove() {
  const p = editing.value
  if (!p || p.kind === 'home') return
  if (!window.confirm(`Xoá trang "${p.title}"? Nội dung trang sẽ mất và khách không mở được trang này nữa (sau khi xuất bản).`)) return
  await run(async () => {
    const { data } = await api.delete<{ pages: ServerPage[] }>(`${base.value}/${p.id}`)
    emit('changed', data.pages)
    editing.value = null
  })
}

// ------------------------------------------------------------ Sắp xếp
async function move(p: SitePage, dir: -1 | 1) {
  const ids = props.pages.map((x) => x.id)
  const i = ids.indexOf(p.id)
  const j = i + dir
  if (i < 0 || j < 1 || j >= ids.length) return // trang chủ luôn ở đầu
  ;[ids[i], ids[j]] = [ids[j]!, ids[i]!]
  await run(async () => {
    const { data } = await api.put<{ pages: ServerPage[] }>(`${base.value}/order`, { ids })
    emit('changed', data.pages)
  })
}
</script>

<template>
  <div class="relative">
    <button
      type="button"
      data-testid="page-picker"
      class="flex max-w-[15rem] items-center gap-1.5 rounded-lg border border-slate-200 bg-white px-2.5 py-1.5 text-xs font-bold text-slate-700 hover:border-indigo-400"
      title="Chọn trang để chỉnh sửa"
      @click="open = !open"
    >
      <Home v-if="current?.kind === 'home'" class="h-3.5 w-3.5 shrink-0 text-indigo-600" />
      <ShieldCheck v-else-if="current?.kind === 'policy'" class="h-3.5 w-3.5 shrink-0 text-indigo-600" />
      <FileText v-else class="h-3.5 w-3.5 shrink-0 text-indigo-600" />
      <span class="truncate">{{ current?.title || 'Trang' }}</span>
      <ChevronDown class="h-3.5 w-3.5 shrink-0 text-slate-400" />
    </button>

    <template v-if="open">
      <div class="fixed inset-0 z-40" @click="open = false" />
      <div data-testid="page-list" class="absolute left-0 top-full z-50 mt-1.5 w-72 rounded-xl border border-slate-200 bg-white p-1.5 shadow-xl">
        <div class="px-2 pb-1 pt-1 text-[11px] font-extrabold uppercase tracking-wide text-slate-400">Các trang ({{ pages.length }}/{{ maxPages }})</div>
        <div class="max-h-[50vh] overflow-y-auto">
          <div
            v-for="(p, i) in pages"
            :key="p.id"
            :data-testid="'page-row-' + p.slug"
            class="group flex items-center gap-1 rounded-lg px-1.5 py-1"
            :class="p.id === currentId ? 'bg-indigo-50 text-indigo-700' : 'hover:bg-slate-50'"
          >
            <button type="button" class="flex min-w-0 flex-1 items-center gap-2 py-1 text-left text-sm" @click="choose(p.id)">
              <Home v-if="p.kind === 'home'" class="h-3.5 w-3.5 shrink-0 text-slate-400" />
              <ShieldCheck v-else-if="p.kind === 'policy'" class="h-3.5 w-3.5 shrink-0 text-slate-400" />
              <FileText v-else class="h-3.5 w-3.5 shrink-0 text-slate-400" />
              <span class="truncate font-semibold">{{ p.title }}</span>
              <span v-if="!p.is_visible" class="inline-flex shrink-0 items-center gap-0.5 rounded bg-slate-200 px-1 text-[10px] font-bold text-slate-600"><EyeOff class="h-2.5 w-2.5" /> Ẩn</span>
              <span v-else-if="!p.published_visible" class="shrink-0 rounded bg-amber-100 px-1 text-[10px] font-bold text-amber-700">Chưa xuất bản</span>
            </button>
            <template v-if="p.kind !== 'home'">
              <button type="button" class="rounded p-0.5 text-slate-400 opacity-0 hover:bg-slate-200 hover:text-slate-700 group-hover:opacity-100 disabled:hidden" :disabled="i <= 1 || busy" title="Đưa lên" @click="move(p, -1)"><ChevronUp class="h-3.5 w-3.5" /></button>
              <button type="button" class="rounded p-0.5 text-slate-400 opacity-0 hover:bg-slate-200 hover:text-slate-700 group-hover:opacity-100 disabled:hidden" :disabled="i === pages.length - 1 || busy" title="Đưa xuống" @click="move(p, 1)"><ChevronDown class="h-3.5 w-3.5" /></button>
            </template>
            <button type="button" :data-testid="'page-settings-' + p.slug" class="rounded p-1 text-slate-400 hover:bg-slate-200 hover:text-slate-700" title="Cài đặt trang" @click="openSettings(p)"><Settings2 class="h-3.5 w-3.5" /></button>
          </div>
        </div>
        <button
          type="button"
          data-testid="page-add"
          class="mt-1 flex w-full items-center justify-center gap-1 rounded-lg border border-dashed border-slate-300 py-2 text-xs font-bold text-slate-600 hover:border-indigo-400 hover:text-indigo-700 disabled:opacity-50"
          :disabled="full"
          :title="full ? `Mỗi website tối đa ${maxPages} trang` : ''"
          @click="openAdd"
        >
          <Plus class="h-3.5 w-3.5" /> Thêm trang
        </button>
        <p v-if="error" class="px-2 pt-1.5 text-[11px] font-semibold text-rose-600">{{ error }}</p>
      </div>
    </template>

    <!-- Thêm trang -->
    <div v-if="addOpen" class="fixed inset-0 z-[60] flex items-center justify-center bg-black/40 p-4" @click.self="addOpen = false">
      <div data-testid="page-add-dialog" class="flex max-h-[90vh] w-full max-w-2xl flex-col gap-4 overflow-hidden rounded-2xl bg-white p-5">
        <div class="flex items-center justify-between">
          <h2 class="text-base font-extrabold">Thêm trang mới</h2>
          <button class="rounded-lg p-1 hover:bg-slate-100" @click="addOpen = false"><X class="h-4 w-4" /></button>
        </div>
        <div class="min-h-0 flex-1 space-y-4 overflow-y-auto">
          <div v-for="g in groups" :key="g.name">
            <div class="mb-1.5 text-[11px] font-extrabold uppercase tracking-wide text-slate-400">{{ g.name }}</div>
            <div class="grid grid-cols-1 gap-2 sm:grid-cols-2">
              <button
                v-for="t in g.items"
                :key="t.key"
                type="button"
                :data-testid="'tpl-' + t.key"
                class="rounded-xl border p-3 text-left"
                :class="tplKey === t.key ? 'border-indigo-500 bg-indigo-50' : 'border-slate-200 hover:border-indigo-300'"
                @click="pickTemplate(t)"
              >
                <div class="text-sm font-extrabold">{{ templateLabel(t).title }}</div>
                <div class="mt-0.5 text-xs text-slate-500">{{ templateLabel(t).desc }}</div>
              </button>
            </div>
          </div>
          <p v-if="tplKey !== 'blank' && templates.find((t) => t.key === tplKey)?.kind === 'policy'" class="rounded-lg bg-amber-50 px-3 py-2 text-xs text-amber-800">
            Đây là nội dung mẫu chung chung. Hãy đọc và sửa lại cho đúng với cửa hàng của bạn trước khi xuất bản — hệ thống không thay bạn cam kết về đổi trả, vận chuyển hay bảo mật.
          </p>
          <div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
            <label class="space-y-1">
              <span class="text-xs font-bold text-slate-600">Tên trang</span>
              <input v-model="newTitle" data-testid="page-new-title" maxlength="160" class="w-full rounded-lg border border-slate-200 px-2.5 py-1.5 text-sm" placeholder="Để trống = tên theo mẫu" @input="titleTouched = true" />
            </label>
            <label class="space-y-1">
              <span class="text-xs font-bold text-slate-600">Đường dẫn (không bắt buộc)</span>
              <input v-model="newSlug" data-testid="page-new-slug" maxlength="60" class="w-full rounded-lg border border-slate-200 px-2.5 py-1.5 text-sm" placeholder="Tự tạo theo tên trang" />
            </label>
          </div>
        </div>
        <p v-if="error" data-testid="page-error" class="text-xs font-semibold text-rose-600">{{ error }}</p>
        <div class="flex justify-end gap-2">
          <button class="rounded-xl border border-slate-200 px-4 py-2 text-xs font-bold hover:bg-slate-50" @click="addOpen = false">Huỷ</button>
          <button data-testid="page-create" class="rounded-xl bg-indigo-600 px-4 py-2 text-xs font-bold text-white hover:bg-indigo-700 disabled:opacity-50" :disabled="busy" @click="create">
            <Loader2 v-if="busy" class="mr-1 inline h-3.5 w-3.5 animate-spin" /> Tạo trang
          </button>
        </div>
      </div>
    </div>

    <!-- Cài đặt trang -->
    <div v-if="editing" class="fixed inset-0 z-[60] flex items-center justify-center bg-black/40 p-4" @click.self="editing = null">
      <div data-testid="page-settings-dialog" class="w-full max-w-md space-y-4 rounded-2xl bg-white p-5">
        <div class="flex items-center justify-between">
          <h2 class="text-base font-extrabold">Cài đặt trang</h2>
          <button class="rounded-lg p-1 hover:bg-slate-100" @click="editing = null"><X class="h-4 w-4" /></button>
        </div>
        <label class="block space-y-1">
          <span class="text-xs font-bold text-slate-600">Tên trang</span>
          <input v-model="eTitle" data-testid="page-edit-title" maxlength="160" class="w-full rounded-lg border border-slate-200 px-2.5 py-1.5 text-sm" />
        </label>
        <template v-if="editing.kind !== 'home'">
          <label class="block space-y-1">
            <span class="text-xs font-bold text-slate-600">Đường dẫn</span>
            <input v-model="eSlug" data-testid="page-edit-slug" maxlength="60" class="w-full rounded-lg border border-slate-200 px-2.5 py-1.5 text-sm" />
          </label>
          <label class="flex items-start gap-2 text-sm">
            <input v-model="eVisible" data-testid="page-edit-visible" type="checkbox" class="mt-0.5 h-4 w-4" />
            <span><b>Hiện với khách</b><br /><span class="text-xs text-slate-500">Bỏ chọn để ẩn trang (khách không mở được). Có hiệu lực sau khi xuất bản.</span></span>
          </label>
        </template>
        <p v-else class="rounded-lg bg-slate-50 px-3 py-2 text-xs text-slate-500">Trang chủ luôn hiện, ở đường dẫn gốc của website, và không xoá được.</p>
        <div class="space-y-1 rounded-xl bg-slate-50 p-3 text-xs">
          <div class="font-bold text-slate-600">Địa chỉ trang</div>
          <div class="break-all text-slate-700">{{ publicPath }}</div>
          <div class="pt-1 font-bold text-slate-600">Để liên kết tới trang này (menu, nút, chân trang) gõ vào ô liên kết:</div>
          <div class="flex items-center gap-2">
            <code class="rounded bg-white px-1.5 py-0.5 text-slate-800">{{ linkCode }}</code>
            <button type="button" class="font-bold text-indigo-700 hover:underline" @click="copyLink">{{ copied ? 'Đã chép' : 'Chép' }}</button>
          </div>
        </div>
        <p v-if="error" data-testid="page-error" class="text-xs font-semibold text-rose-600">{{ error }}</p>
        <div class="flex flex-wrap items-center justify-between gap-2">
          <div class="flex gap-1.5">
            <button type="button" data-testid="page-duplicate" class="inline-flex items-center gap-1 rounded-xl border border-slate-200 px-3 py-2 text-xs font-bold hover:bg-slate-50 disabled:opacity-50" :disabled="busy || full" :title="full ? `Mỗi website tối đa ${maxPages} trang` : ''" @click="duplicate">
              <Copy class="h-3.5 w-3.5" /> Nhân bản
            </button>
            <button v-if="editing.kind !== 'home'" type="button" data-testid="page-delete" class="inline-flex items-center gap-1 rounded-xl border border-rose-200 px-3 py-2 text-xs font-bold text-rose-600 hover:bg-rose-50 disabled:opacity-50" :disabled="busy" @click="remove">
              <Trash2 class="h-3.5 w-3.5" /> Xoá
            </button>
          </div>
          <div class="flex gap-2">
            <button class="rounded-xl border border-slate-200 px-4 py-2 text-xs font-bold hover:bg-slate-50" @click="editing = null">Đóng</button>
            <button data-testid="page-save" class="rounded-xl bg-indigo-600 px-4 py-2 text-xs font-bold text-white hover:bg-indigo-700 disabled:opacity-50" :disabled="busy || !eTitle.trim()" @click="saveSettings">
              <Loader2 v-if="busy" class="mr-1 inline h-3.5 w-3.5 animate-spin" /> Lưu
            </button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
