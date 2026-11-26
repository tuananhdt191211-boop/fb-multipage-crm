<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ArrowDown, ArrowLeft, ArrowUp, ExternalLink, Loader2, Plus, Trash2 } from 'lucide-vue-next'
import { api } from '../lib/api'
import StoreRenderer from '../components/store/StoreRenderer.vue'
import type { AdminProduct, PublicProduct, Site, StoreBlock } from '../types/store'

const route = useRoute()
const router = useRouter()
const siteId = computed(() => String(route.params.id || ''))

const loading = ref(true)
const saving = ref(false)
const publishing = ref(false)
const dirty = ref(false)
const errorMsg = ref('')
const okMsg = ref('')

const site = ref<Site | null>(null)
const blocks = ref<StoreBlock[]>([])
const products = ref<PublicProduct[]>([])

async function loadProducts() {
  try {
    const { data } = await api.get<{ products: AdminProduct[] }>(`/storefront/sites/${siteId.value}/products`)
    products.value = (data.products || [])
      .filter((p) => p.is_active)
      .map((p) => ({
        id: p.id,
        name: p.name,
        description: p.description,
        price: p.price,
        compare_price: p.compare_price,
        image_url: p.image_url,
        in_stock: p.stock !== 0
      }))
  } catch {
    products.value = []
  }
}

const blockLabels: Record<string, string> = {
  header: 'Đầu trang',
  hero: 'Banner chính',
  products: 'Sản phẩm',
  features: 'Cam kết dịch vụ',
  testimonials: 'Đánh giá khách hàng',
  cta: 'Kêu gọi hành động & chân trang'
}

function apiError(err: any, fallback: string) {
  return err?.response?.data?.error || fallback
}

function applySite(s: Site) {
  site.value = s
  try {
    const parsed = JSON.parse(s.draft_blocks_json || '[]')
    blocks.value = Array.isArray(parsed) ? parsed : []
  } catch {
    blocks.value = []
  }
  dirty.value = false
}

async function load() {
  loading.value = true
  errorMsg.value = ''
  try {
    const { data } = await api.get<{ site: Site }>(`/storefront/sites/${siteId.value}`)
    applySite(data.site)
        await loadProducts()
  } catch (err) {
    errorMsg.value = apiError(err, 'Không tải được website.')
  } finally {
    loading.value = false
  }
}

function cleanBlocks(): StoreBlock[] {
  const copy: StoreBlock[] = JSON.parse(JSON.stringify(blocks.value))
  for (const b of copy) {
    if (Array.isArray(b.data.nav_items)) b.data.nav_items = b.data.nav_items.map((x: string) => x.trim()).filter(Boolean)
    if (b.type === 'features' && Array.isArray(b.data.items)) {
      b.data.items = b.data.items.map((x: string) => x.trim()).filter(Boolean)
    }
  }
  return copy
}

async function saveDraft(): Promise<boolean> {
  if (!site.value) return false
  saving.value = true
  errorMsg.value = ''
  okMsg.value = ''
  try {
    const { data } = await api.put<{ site: Site }>(`/storefront/sites/${site.value.id}/draft`, {
      site_title: site.value.site_title,
      logo_url: site.value.logo_url,
      primary_color: site.value.primary_color,
      draft_blocks_json: JSON.stringify(cleanBlocks())
    })
    applySite(data.site)
    okMsg.value = 'Đã lưu bản nháp.'
    return true
  } catch (err) {
    errorMsg.value = apiError(err, 'Lưu bản nháp thất bại.')
    return false
  } finally {
    saving.value = false
  }
}

async function publish() {
  if (!site.value) return
  publishing.value = true
  try {
    if (!(await saveDraft())) return
    const { data } = await api.post<{ site: Site }>(`/storefront/sites/${site.value.id}/publish`)
    applySite(data.site)
    okMsg.value = 'Đã xuất bản. Khách hàng sẽ thấy bản mới ngay.'
  } catch (err) {
    errorMsg.value = apiError(err, 'Xuất bản thất bại.')
  } finally {
    publishing.value = false
  }
}

function move(i: number, dir: -1 | 1) {
  const j = i + dir
  if (j < 0 || j >= blocks.value.length) return
  const arr = blocks.value
  ;[arr[i], arr[j]] = [arr[j]!, arr[i]!]
  dirty.value = true
}

function addProduct(b: StoreBlock) {
  if (!Array.isArray(b.data.items)) b.data.items = []
  b.data.items.push({ id: `p${Date.now()}`, name: 'Sản phẩm mới', price: '0đ', image_url: '', desc: '' })
  dirty.value = true
}

function removeItem(b: StoreBlock, idx: number | string) {
  b.data.items.splice(Number(idx), 1)
  dirty.value = true
}

function linesOf(arr: unknown): string {
  return Array.isArray(arr) ? arr.join('\n') : ''
}

function setLines(b: StoreBlock, key: string, ev: Event) {
  b.data[key] = (ev.target as HTMLTextAreaElement).value.split('\n')
  dirty.value = true
}

const publicUrl = computed(() => (site.value ? `${window.location.origin}/s/${site.value.slug}` : ''))
const previewBlocks = computed(() => blocks.value)

onMounted(load)
</script>

<template>
  <div class="min-h-screen bg-slate-50 text-slate-900">
    <div v-if="loading" class="flex items-center justify-center gap-2 py-24 text-sm text-slate-500">
      <Loader2 class="h-5 w-5 animate-spin text-emerald-600" /> Đang tải...
    </div>

    <div v-else-if="!site" class="mx-auto max-w-lg py-24 text-center">
      <p class="text-sm font-semibold text-rose-600">{{ errorMsg || 'Không tìm thấy website.' }}</p>
      <button class="mt-4 rounded-xl border px-4 py-2 text-sm font-bold" @click="router.push('/sites')">Về danh sách</button>
    </div>

    <template v-else>
      <div class="sticky top-0 z-20 flex flex-wrap items-center justify-between gap-3 border-b border-slate-200 bg-white px-4 py-3">
        <div class="flex items-center gap-3">
          <button class="rounded-xl border border-slate-200 p-2 hover:bg-slate-100" @click="router.push('/sites')">
            <ArrowLeft class="h-4 w-4" />
          </button>
          <div class="text-sm font-extrabold">{{ site.site_title }}</div>
          <span v-if="dirty" class="text-xs font-semibold text-amber-600">Chưa lưu</span>
        </div>
        <div class="flex items-center gap-2">
          <a
            v-if="site.is_published"
            :href="publicUrl"
            target="_blank"
            rel="noopener"
            class="inline-flex items-center gap-1 text-xs font-bold text-emerald-700 hover:underline"
          >
            Xem trang <ExternalLink class="h-3 w-3" />
          </a>
          <button
            class="rounded-xl border border-slate-200 px-4 py-2 text-xs font-bold hover:bg-slate-50 disabled:opacity-50"
            :disabled="saving || publishing"
            @click="saveDraft"
          >
            <Loader2 v-if="saving" class="mr-1 inline h-3.5 w-3.5 animate-spin" /> Lưu nháp
          </button>
          <button
            class="rounded-xl bg-emerald-600 px-4 py-2 text-xs font-bold text-white hover:bg-emerald-700 disabled:opacity-50"
            :disabled="saving || publishing"
            @click="publish"
          >
            <Loader2 v-if="publishing" class="mr-1 inline h-3.5 w-3.5 animate-spin" /> Xuất bản
          </button>
        </div>
      </div>

      <p v-if="errorMsg" class="mx-4 mt-3 rounded-xl bg-rose-50 px-4 py-2 text-sm font-semibold text-rose-600">{{ errorMsg }}</p>
      <p v-if="okMsg" class="mx-4 mt-3 rounded-xl bg-emerald-50 px-4 py-2 text-sm font-semibold text-emerald-700">{{ okMsg }}</p>

      <div class="grid grid-cols-1 gap-4 p-4 lg:grid-cols-[420px_1fr]">
        <!-- ===== Cột chỉnh sửa ===== -->
        <div class="space-y-4">
          <section class="space-y-3 rounded-2xl border border-slate-200 bg-white p-4">
            <h2 class="text-sm font-extrabold">Thông tin chung</h2>
            <label class="block space-y-1">
              <span class="text-xs font-bold text-slate-600">Tên website</span>
              <input v-model="site.site_title" class="w-full rounded-xl border border-slate-200 px-3 py-2 text-sm" @input="dirty = true" />
            </label>
            <label class="block space-y-1">
              <span class="text-xs font-bold text-slate-600">Logo (URL ảnh)</span>
              <input v-model="site.logo_url" class="w-full rounded-xl border border-slate-200 px-3 py-2 text-sm" @input="dirty = true" />
            </label>
            <label class="flex items-center gap-3">
              <span class="text-xs font-bold text-slate-600">Màu chủ đạo</span>
              <input v-model="site.primary_color" type="color" class="h-8 w-12 rounded border border-slate-200" @input="dirty = true" />
              <span class="text-xs text-slate-400">{{ site.primary_color }}</span>
            </label>
          </section>

          <section
            v-for="(b, i) in blocks"
            :key="b.id"
            class="space-y-3 rounded-2xl border border-slate-200 bg-white p-4"
            :class="{ 'opacity-60': !b.enabled }"
          >
            <div class="flex items-center justify-between">
              <h2 class="text-sm font-extrabold">{{ blockLabels[b.type] || b.type }}</h2>
              <div class="flex items-center gap-1">
                <label class="mr-2 flex items-center gap-1 text-xs font-semibold text-slate-500">
                  <input v-model="b.enabled" type="checkbox" @change="dirty = true" /> Hiện
                </label>
                <button class="rounded-lg p-1 hover:bg-slate-100 disabled:opacity-30" :disabled="i === 0" @click="move(i, -1)">
                  <ArrowUp class="h-4 w-4" />
                </button>
                <button class="rounded-lg p-1 hover:bg-slate-100 disabled:opacity-30" :disabled="i === blocks.length - 1" @click="move(i, 1)">
                  <ArrowDown class="h-4 w-4" />
                </button>
              </div>
            </div>

            <!-- header -->
            <template v-if="b.type === 'header'">
              <input v-model="b.data.brand_name" placeholder="Tên thương hiệu" class="w-full rounded-xl border border-slate-200 px-3 py-2 text-sm" @input="dirty = true" />
              <textarea :value="linesOf(b.data.nav_items)" rows="3" placeholder="Mỗi dòng một mục menu" class="w-full rounded-xl border border-slate-200 px-3 py-2 text-sm" @input="setLines(b, 'nav_items', $event)" />
              <div class="grid grid-cols-2 gap-2">
                <input v-model="b.data.cta_label" placeholder="Chữ trên nút" class="rounded-xl border border-slate-200 px-3 py-2 text-sm" @input="dirty = true" />
                <input v-model="b.data.cta_link" placeholder="Liên kết nút" class="rounded-xl border border-slate-200 px-3 py-2 text-sm" @input="dirty = true" />
              </div>
            </template>

            <!-- hero -->
            <template v-else-if="b.type === 'hero'">
              <input v-model="b.data.badge" placeholder="Nhãn nhỏ" class="w-full rounded-xl border border-slate-200 px-3 py-2 text-sm" @input="dirty = true" />
              <input v-model="b.data.title" placeholder="Tiêu đề" class="w-full rounded-xl border border-slate-200 px-3 py-2 text-sm" @input="dirty = true" />
              <textarea v-model="b.data.subtitle" rows="3" placeholder="Mô tả ngắn" class="w-full rounded-xl border border-slate-200 px-3 py-2 text-sm" @input="dirty = true" />
              <input v-model="b.data.image_url" placeholder="URL ảnh banner" class="w-full rounded-xl border border-slate-200 px-3 py-2 text-sm" @input="dirty = true" />
              <div class="grid grid-cols-2 gap-2">
                <input v-model="b.data.cta_label" placeholder="Chữ trên nút" class="rounded-xl border border-slate-200 px-3 py-2 text-sm" @input="dirty = true" />
                <input v-model="b.data.cta_link" placeholder="Liên kết nút" class="rounded-xl border border-slate-200 px-3 py-2 text-sm" @input="dirty = true" />
              </div>
            </template>

            <!-- products -->
            <template v-else-if="b.type === 'products'">
              <input v-model="b.data.heading" placeholder="Tiêu đề mục" class="w-full rounded-xl border border-slate-200 px-3 py-2 text-sm" @input="dirty = true" />

              <label class="block space-y-1">
                <span class="text-xs font-bold text-slate-600">Nguồn sản phẩm</span>
                <select v-model="b.data.source" class="w-full rounded-xl border border-slate-200 px-3 py-2 text-sm" @change="dirty = true">
                  <option value="catalog">Lấy từ danh sách sản phẩm của shop</option>
                  <option value="manual">Nhập tay trong khối này</option>
                </select>
              </label>

              <template v-if="b.data.source === 'catalog'">
                <p class="text-xs text-slate-500">
                  Sản phẩm được quản lý ở trang riêng.
                  <router-link :to="{ name: 'site-products', params: { id: site.id } }" class="font-bold text-emerald-700 hover:underline">
                    Quản lý sản phẩm
                  </router-link>
                </p>
              </template>

              <template v-else>
                <div v-for="(p, pi) in b.data.items || []" :key="p.id || pi" class="space-y-2 rounded-xl border border-slate-100 bg-slate-50 p-3">
                  <div class="flex gap-2">
                    <input v-model="p.name" placeholder="Tên sản phẩm" class="w-full rounded-lg border border-slate-200 px-2 py-1.5 text-sm" @input="dirty = true" />
                    <input v-model="p.price" placeholder="Giá" class="w-28 rounded-lg border border-slate-200 px-2 py-1.5 text-sm" @input="dirty = true" />
                    <button class="rounded-lg p-1.5 text-rose-600 hover:bg-rose-50" @click="removeItem(b, Number(pi))"><Trash2 class="h-4 w-4" /></button>
                  </div>
                  <input v-model="p.image_url" placeholder="URL ảnh" class="w-full rounded-lg border border-slate-200 px-2 py-1.5 text-sm" @input="dirty = true" />
                  <textarea v-model="p.desc" rows="2" placeholder="Mô tả" class="w-full rounded-lg border border-slate-200 px-2 py-1.5 text-sm" @input="dirty = true" />
                </div>
                <button class="inline-flex items-center gap-1 text-xs font-bold text-emerald-700" @click="addProduct(b)">
                  <Plus class="h-3.5 w-3.5" /> Thêm sản phẩm
                </button>
              </template>
            </template>

            <!-- features -->
            <template v-else-if="b.type === 'features'">
              <input v-model="b.data.heading" placeholder="Tiêu đề mục" class="w-full rounded-xl border border-slate-200 px-3 py-2 text-sm" @input="dirty = true" />
              <textarea :value="linesOf(b.data.items)" rows="4" placeholder="Mỗi dòng một cam kết" class="w-full rounded-xl border border-slate-200 px-3 py-2 text-sm" @input="setLines(b, 'items', $event)" />
            </template>

            <!-- cta -->
            <template v-else-if="b.type === 'cta'">
              <input v-model="b.data.title" placeholder="Tiêu đề" class="w-full rounded-xl border border-slate-200 px-3 py-2 text-sm" @input="dirty = true" />
              <textarea v-model="b.data.subtitle" rows="2" placeholder="Mô tả" class="w-full rounded-xl border border-slate-200 px-3 py-2 text-sm" @input="dirty = true" />
              <div class="grid grid-cols-2 gap-2">
                <input v-model="b.data.button_text" placeholder="Chữ trên nút" class="rounded-xl border border-slate-200 px-3 py-2 text-sm" @input="dirty = true" />
                <input v-model="b.data.button_link" placeholder="Liên kết nút (tel:, mailto:, https://...)" class="rounded-xl border border-slate-200 px-3 py-2 text-sm" @input="dirty = true" />
              </div>
              <input v-model="b.data.footer_note" placeholder="Ghi chú chân trang" class="w-full rounded-xl border border-slate-200 px-3 py-2 text-sm" @input="dirty = true" />
            </template>

            <p v-else class="text-xs text-slate-400">Khối này sẽ có trình chỉnh sửa ở bước sau.</p>
          </section>
        </div>

        <!-- ===== Xem trước ===== -->
        <div class="lg:sticky lg:top-20 lg:h-[calc(100vh-6rem)] overflow-auto rounded-2xl border border-slate-200 bg-white">
          <div class="border-b border-slate-100 px-4 py-2 text-xs font-bold text-slate-400">Xem trước (bản nháp)</div>
                 <StoreRenderer :site-title="site.site_title" :primary-color="site.primary_color" :blocks="previewBlocks" :products="products" />
        </div>
      </div>
    </template>
  </div>
</template>