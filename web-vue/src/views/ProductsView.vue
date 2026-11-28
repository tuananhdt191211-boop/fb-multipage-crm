<script setup lang="ts">
// Quản lý sản phẩm của website: danh sách, trình sửa kiểu Shopify / ShopBase (mô tả định dạng, ảnh + video, giá nhập & lợi nhuận,
// SKU, thẻ, nguồn hàng), nhân bản, nhập từ AliExpress / 1688 / link bất kỳ.
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ArrowLeft, Copy, ExternalLink, Film, Loader2, PackageSearch, Pencil, Plus, Search, Trash2, X } from 'lucide-vue-next'
import { api } from '../lib/api'
import { adminMoney, adminMoneyTitle } from '../lib/uiI18n'
import { formatVnd } from '../lib/format'
import { summaryFromHtml } from '../lib/richHtml'
import { fetchMediaConfig, type MediaConfig, type ProductMedia } from '../lib/productMedia'
import { storePath } from '../lib/storeHost'
import { safeImg } from '../lib/safeUrl'
import RichTextEditor from '../components/store/RichTextEditor.vue'
import ProductMediaManager from '../components/store/ProductMediaManager.vue'
import ProductImportDialog, { type ImportResult } from '../components/store/ProductImportDialog.vue'
import ProductVariantsEditor from '../components/store/ProductVariantsEditor.vue'
import { fromRows, rebuildVariants, toPayload, validateVariants, type VariantDefaults } from '../lib/variants'
import type { AdminProduct, AdminProductLite, AdminVariant, ProductOption, Site } from '../types/store'

const route = useRoute()
const router = useRouter()
const siteId = computed(() => String(route.params.id || ''))

const loading = ref(true)
const saving = ref(false)
const opening = ref(false)
const errorMsg = ref('')
const site = ref<Site | null>(null)
const products = ref<AdminProductLite[]>([])
const maxProducts = ref(200)
const mediaCfg = ref<MediaConfig | null>(null)

const search = ref('')
const statusFilter = ref<'all' | 'active' | 'hidden'>('all')

const showForm = ref(false)
const showImport = ref(false)
const editingId = ref<number | null>(null)
const importedFrom = ref('')

interface ProductForm {
  name: string
  description: string
  price: number
  compare_price: number
  cost_price: number
  media: ProductMedia[]
  stock: number
  unlimited: boolean
  is_active: boolean
  sort_order: number
  sku: string
  barcode: string
  source_url: string
  tags: string[]
  options: ProductOption[] // nhóm phân loại (Màu, Size...)
  variants: AdminVariant[] // thẳng hàng với options
}
const emptyForm = (): ProductForm => ({
  name: '', description: '', price: 0, compare_price: 0, cost_price: 0, media: [], stock: 0, unlimited: true, is_active: true,
  sort_order: 0, sku: '', barcode: '', source_url: '', tags: [], options: [], variants: []
})
const form = ref<ProductForm>(emptyForm())
const tagInput = ref('')
let snapshot = ''

function apiError(err: any, fallback: string) {
  return err?.response?.data?.error || fallback
}

function toLite(p: AdminProduct): AdminProductLite {
  const media = p.media || []
  const vs = p.variants || []
  return {
    id: p.id, name: p.name, summary: summaryFromHtml(p.description || ''), price: p.price, compare_price: p.compare_price,
    image_url: p.image_url, stock: p.stock, is_active: p.is_active, sort_order: p.sort_order, sku: p.sku || '',
    cost_price: p.cost_price || 0, source_url: p.source_url || '', tags: p.tags || '', media_count: media.length,
    has_video: media.some((m) => m.type !== 'image'),
    variant_count: vs.length, price_max: Math.max(p.price, ...vs.map((v) => v.price))
  }
}

async function load() {
  loading.value = true
  errorMsg.value = ''
  try {
    const [siteRes, prodRes] = await Promise.all([
      api.get<{ site: Site }>(`/storefront/sites/${siteId.value}`),
      api.get<{ products: AdminProductLite[]; max_products: number }>(`/storefront/sites/${siteId.value}/products`, { params: { lite: 1 } })
    ])
    site.value = siteRes.data.site
    products.value = prodRes.data.products || []
    maxProducts.value = prodRes.data.max_products || 200
  } catch (err) {
    errorMsg.value = apiError(err, 'Không tải được dữ liệu.')
  } finally {
    loading.value = false
  }
  fetchMediaConfig().then((c) => (mediaCfg.value = c)).catch(() => (mediaCfg.value = null))
}

const filtered = computed(() => {
  const q = search.value.trim().toLowerCase()
  return products.value.filter((p) => {
    if (statusFilter.value === 'active' && !p.is_active) return false
    if (statusFilter.value === 'hidden' && p.is_active) return false
    return !q || p.name.toLowerCase().includes(q) || p.sku.toLowerCase().includes(q) || p.tags.toLowerCase().includes(q)
  })
})

function formFrom(p: AdminProduct): ProductForm {
  const options = (p.options || []).map((o) => ({ name: o.name, values: [...o.values] }))
  return {
    options,
    variants: fromRows(options, p.variants || []),
    name: p.name,
    description: p.description || '',
    price: p.price,
    compare_price: p.compare_price,
    cost_price: p.cost_price || 0,
    media: (p.media || []).map((m) => ({ ...m })),
    stock: p.stock === -1 ? 0 : p.stock,
    unlimited: p.stock === -1,
    is_active: p.is_active,
    sort_order: p.sort_order,
    sku: p.sku || '',
    barcode: p.barcode || '',
    source_url: p.source_url || '',
    tags: (p.tags || '').split(',').map((t) => t.trim()).filter(Boolean)
  }
}

function openForm(f: ProductForm, id: number | null, note = '') {
  editingId.value = id
  form.value = f
  importedFrom.value = note
  tagInput.value = ''
  errorMsg.value = ''
  snapshot = JSON.stringify(f)
  showForm.value = true
}

function openCreate() {
  openForm(emptyForm(), null)
}

async function fetchFull(id: number): Promise<AdminProduct | null> {
  try {
    const { data } = await api.get<{ product: AdminProduct }>(`/storefront/sites/${siteId.value}/products/${id}`)
    return data.product
  } catch (err) {
    errorMsg.value = apiError(err, 'Không tải được sản phẩm.')
    return null
  }
}

async function openEdit(p: AdminProductLite) {
  opening.value = true
  const full = await fetchFull(p.id)
  opening.value = false
  if (full) openForm(formFrom(full), p.id)
}

async function duplicate(p: AdminProductLite) {
  opening.value = true
  const full = await fetchFull(p.id)
  opening.value = false
  if (!full) return
  const f = formFrom(full)
  f.name = `${f.name} (bản sao)`
  f.sku = ''
  f.variants = f.variants.map((v) => ({ ...v, id: undefined, sku: '' })) // bản sao là sản phẩm mới: không dùng lại id / SKU của biến thể cũ
  f.is_active = false
  openForm(f, null, '')
}

function onImported(r: ImportResult) {
  showImport.value = false
  const f = emptyForm()
  f.name = r.name
  f.description = r.description
  f.media = r.media
  f.price = r.price
  f.compare_price = r.compare_price
  f.cost_price = r.cost_price
  f.sku = r.sku
  f.source_url = r.source_url
  f.is_active = false // nhập xong để ẩn, xem lại rồi mới bật bán
  // Phân loại tìm thấy → biến thể (giá = giá bán đã tính, tồn kho không giới hạn); chủ shop chỉnh giá từng biến thể sau
  f.options = r.options.map((o) => ({ name: o.name, values: [...o.values] }))
  f.variants = rebuildVariants(f.options, [], { price: f.price, compare_price: f.compare_price, cost_price: f.cost_price, stock: -1 })
  openForm(
    f,
    null,
    f.variants.length
      ? `Đã nhập dữ liệu và tạo ${f.variants.length} biến thể — hãy xem lại tên, ảnh, mô tả, giá và tồn kho từng biến thể rồi bấm Lưu. Sản phẩm đang ở trạng thái ẨN cho tới khi bạn bật bán.`
      : 'Đã nhập dữ liệu — hãy xem lại tên, giá, ảnh, mô tả rồi bấm Lưu. Sản phẩm đang ở trạng thái ẨN cho tới khi bạn bật bán.'
  )
}

const dirty = computed(() => showForm.value && JSON.stringify(form.value) !== snapshot)

function closeForm() {
  if (dirty.value && !window.confirm('Bạn có thay đổi chưa lưu. Đóng và bỏ các thay đổi?')) return
  showForm.value = false
}

function addTag() {
  for (const t of tagInput.value.split(',').map((s) => s.trim()).filter(Boolean)) {
    if (!form.value.tags.some((x) => x.toLowerCase() === t.toLowerCase()) && form.value.tags.length < 20) form.value.tags.push(t.slice(0, 40))
  }
  tagInput.value = ''
}

function onTagKey(e: KeyboardEvent) {
  if (e.key === 'Enter' || e.key === ',') {
    e.preventDefault()
    addTag()
  }
}

async function save() {
  if (!form.value.name.trim()) {
    errorMsg.value = 'Hãy nhập tên sản phẩm.'
    return
  }
  if (tagInput.value.trim()) addTag()
  const vErr = validateVariants(form.value.options, form.value.variants)
  if (vErr) {
    errorMsg.value = vErr
    return
  }
  saving.value = true
  errorMsg.value = ''
  const f = form.value
  const payload = {
    ...toPayload(f.options, f.variants), // options + variants (rỗng = sản phẩm không có biến thể)
    name: f.name.trim(),
    description: f.description,
    price: Number(f.price) || 0,
    compare_price: Number(f.compare_price) || 0,
    cost_price: Number(f.cost_price) || 0,
    media: f.media,
    stock: f.unlimited ? -1 : Math.max(0, Number(f.stock) || 0),
    is_active: f.is_active,
    sort_order: f.sort_order,
    sku: f.sku,
    barcode: f.barcode,
    source_url: f.source_url,
    tags: f.tags.join(', ')
  }
  try {
    if (editingId.value) {
      const { data } = await api.put<{ product: AdminProduct }>(`/storefront/sites/${siteId.value}/products/${editingId.value}`, payload)
      const idx = products.value.findIndex((x) => x.id === editingId.value)
      if (idx >= 0) products.value[idx] = toLite(data.product)
    } else {
      const { data } = await api.post<{ product: AdminProduct }>(`/storefront/sites/${siteId.value}/products`, payload)
      products.value.push(toLite(data.product))
    }
    snapshot = JSON.stringify(form.value)
    showForm.value = false
  } catch (err) {
    errorMsg.value = apiError(err, 'Lưu sản phẩm thất bại.')
  } finally {
    saving.value = false
  }
}

async function toggleActive(p: AdminProductLite) {
  try {
    const { data } = await api.patch<{ product: AdminProductLite }>(`/storefront/sites/${siteId.value}/products/${p.id}`, { is_active: !p.is_active })
    Object.assign(p, data.product)
  } catch (err) {
    errorMsg.value = apiError(err, 'Cập nhật thất bại.')
  }
}

async function remove(p: AdminProductLite) {
  if (!window.confirm(`Xóa sản phẩm "${p.name}"?`)) return
  try {
    await api.delete(`/storefront/sites/${siteId.value}/products/${p.id}`)
    products.value = products.value.filter((x) => x.id !== p.id)
  } catch (err) {
    errorMsg.value = apiError(err, 'Xóa sản phẩm thất bại.')
  }
}

function stockLabel(p: { stock: number }) {
  if (p.stock === -1) return 'Không giới hạn'
  if (p.stock === 0) return 'Hết hàng'
  return `${p.stock} cái`
}

// Có biến thể: giá / giá gốc / giá nhập / tồn kho nhập theo từng biến thể (số của sản phẩm chỉ là tổng hợp)
const hasVariants = computed(() => form.value.variants.length > 0)
// Giá trị mặc định cho biến thể mới = giá / kho đang nhập ở sản phẩm
const variantDefaults = computed<VariantDefaults>(() => ({
  price: Number(form.value.price) || 0,
  compare_price: Number(form.value.compare_price) || 0,
  cost_price: Number(form.value.cost_price) || 0,
  stock: form.value.unlimited ? -1 : Math.max(0, Number(form.value.stock) || 0)
}))

const discount = computed(() => (form.value.compare_price > form.value.price && form.value.price > 0 ? Math.round((1 - form.value.price / form.value.compare_price) * 100) : 0))
const profit = computed(() => (Number(form.value.price) || 0) - (Number(form.value.cost_price) || 0))
const margin = computed(() => (form.value.price > 0 && form.value.cost_price > 0 ? Math.round((profit.value / form.value.price) * 100) : null))

function viewOnSite(p: AdminProductLite) {
  if (site.value) window.open(window.location.origin + storePath(site.value.slug, `/p/${p.id}`), '_blank', 'noopener')
}

onMounted(load)
</script>

<template>
  <div class="min-h-screen bg-slate-50 text-slate-900">
    <div class="mx-auto max-w-5xl space-y-5 px-4 py-8">
      <div class="flex flex-wrap items-center justify-between gap-3">
        <div class="flex items-center gap-3">
          <button class="rounded-xl border border-slate-200 bg-white p-2 hover:bg-slate-100" @click="router.push('/sites')">
            <ArrowLeft class="h-4 w-4" />
          </button>
          <div>
            <h1 class="text-xl font-extrabold">Sản phẩm</h1>
            <p class="text-xs text-slate-500">{{ site?.site_title }} · {{ products.length }}/{{ maxProducts }} sản phẩm</p>
          </div>
        </div>
        <div class="flex flex-wrap items-center gap-2">
          <button
            class="inline-flex items-center gap-2 rounded-xl border border-emerald-200 bg-white px-4 py-2 text-sm font-bold text-emerald-700 hover:bg-emerald-50"
            @click="showImport = true"
          >
            <PackageSearch class="h-4 w-4" /> Nhập từ AliExpress / 1688 / link
          </button>
          <button class="inline-flex items-center gap-2 rounded-xl bg-emerald-600 px-4 py-2 text-sm font-bold text-white hover:bg-emerald-700" @click="openCreate">
            <Plus class="h-4 w-4" /> Thêm sản phẩm
          </button>
        </div>
      </div>

      <p v-if="errorMsg && !showForm" class="rounded-xl bg-rose-50 px-4 py-2 text-sm font-semibold text-rose-600">{{ errorMsg }}</p>

      <div v-if="products.length > 3" class="flex flex-wrap items-center gap-3">
        <div class="relative min-w-[220px] flex-1">
          <Search class="pointer-events-none absolute left-3 top-2.5 h-4 w-4 text-slate-400" />
          <input v-model="search" class="w-full rounded-xl border border-slate-200 bg-white py-2 pl-9 pr-3 text-sm" placeholder="Tìm theo tên, SKU, thẻ..." />
        </div>
        <div class="inline-flex rounded-xl bg-white p-1 text-xs font-bold ring-1 ring-slate-200">
          <button v-for="o in [['all', 'Tất cả'], ['active', 'Đang bán'], ['hidden', 'Đã ẩn']]" :key="o[0]" class="rounded-lg px-3 py-1.5" :class="statusFilter === o[0] ? 'bg-emerald-600 text-white' : 'text-slate-500'" @click="statusFilter = o[0] as any">{{ o[1] }}</button>
        </div>
      </div>

      <div v-if="loading" class="flex items-center justify-center gap-2 py-20 text-sm text-slate-500">
        <Loader2 class="h-5 w-5 animate-spin text-emerald-600" /> Đang tải...
      </div>

      <div v-else-if="!products.length" class="rounded-2xl border border-dashed border-slate-300 bg-white py-16 text-center text-sm text-slate-500">
        Chưa có sản phẩm nào. Bấm “Thêm sản phẩm” để bắt đầu, hoặc nhập nhanh từ AliExpress / 1688.
      </div>

      <p v-else-if="!filtered.length" class="rounded-2xl border border-dashed border-slate-300 bg-white py-10 text-center text-sm text-slate-500">Không có sản phẩm nào khớp.</p>

      <ul v-else class="space-y-3">
        <li
          v-for="p in filtered"
          :key="p.id"
          class="flex flex-wrap items-center justify-between gap-3 rounded-2xl border border-slate-200 bg-white p-3"
          :class="{ 'opacity-60': !p.is_active }"
        >
          <div class="flex min-w-0 items-center gap-3">
            <div class="relative h-16 w-16 shrink-0 overflow-hidden rounded-xl bg-slate-100">
              <img v-if="safeImg(p.image_url)" :src="safeImg(p.image_url)" :alt="p.name" referrerpolicy="no-referrer" class="h-full w-full object-cover" />
              <span v-if="p.has_video" class="absolute bottom-1 left-1 rounded bg-black/65 p-0.5 text-white"><Film class="h-3 w-3" /></span>
              <span v-if="p.media_count > 1" class="absolute bottom-1 right-1 rounded bg-black/65 px-1 text-[10px] font-bold text-white">{{ p.media_count }}</span>
            </div>
            <div class="min-w-0">
              <div class="truncate text-sm font-extrabold">{{ p.name }}</div>
              <div class="text-xs text-slate-500">
                <span :title="adminMoneyTitle(p.price)">{{ adminMoney(p.price) }}</span>
                <span v-if="p.variant_count > 0 && p.price_max > p.price" :title="adminMoneyTitle(p.price_max)"> – {{ adminMoney(p.price_max) }}</span>
                <span v-else-if="p.compare_price > p.price" class="ml-1 line-through text-slate-400" :title="adminMoneyTitle(p.compare_price)">{{ adminMoney(p.compare_price) }}</span>
                · Tồn kho: {{ stockLabel(p) }}
                <span v-if="p.variant_count > 0" class="ml-1 rounded-full bg-indigo-50 px-2 py-0.5 text-[10px] font-bold text-indigo-700" data-testid="variant-badge">{{ p.variant_count }} biến thể</span>
                <template v-if="p.sku"> · SKU: {{ p.sku }}</template>
              </div>
              <div v-if="p.cost_price > 0" class="text-[11px] text-slate-400">
                Giá nhập {{ formatVnd(p.cost_price) }} · Lợi nhuận
                <span :class="p.price - p.cost_price >= 0 ? 'text-emerald-600' : 'text-rose-500'">{{ formatVnd(p.price - p.cost_price) }}</span>
              </div>
            </div>
          </div>
          <div class="flex items-center gap-2">
            <button class="rounded-xl border border-slate-200 px-3 py-1.5 text-xs font-bold hover:bg-slate-50" @click="toggleActive(p)">
              {{ p.is_active ? 'Đang bán' : 'Đã ẩn' }}
            </button>
            <button v-if="site?.is_published && p.is_active" class="rounded-xl border border-slate-200 p-1.5 text-slate-500 hover:bg-slate-50" title="Xem trên website" @click="viewOnSite(p)">
              <ExternalLink class="h-4 w-4" />
            </button>
            <button class="rounded-xl border border-slate-200 p-1.5 text-slate-500 hover:bg-slate-50" title="Nhân bản" @click="duplicate(p)">
              <Copy class="h-4 w-4" />
            </button>
            <button class="inline-flex items-center gap-1 rounded-xl border border-slate-200 px-3 py-1.5 text-xs font-bold hover:bg-slate-50" @click="openEdit(p)">
              <Pencil class="h-3.5 w-3.5" /> Sửa
            </button>
            <button class="rounded-xl border border-rose-200 p-1.5 text-rose-600 hover:bg-rose-50" @click="remove(p)">
              <Trash2 class="h-4 w-4" />
            </button>
          </div>
        </li>
      </ul>
    </div>

    <div v-if="opening" class="fixed inset-0 z-40 flex items-center justify-center bg-black/20"><Loader2 class="h-7 w-7 animate-spin text-emerald-600" /></div>

    <!-- Trình sửa sản phẩm -->
    <div v-if="showForm" class="fixed inset-0 z-50 overflow-auto bg-black/40 p-4" @click.self="closeForm">
      <div class="mx-auto my-4 w-full max-w-5xl rounded-2xl bg-slate-50 shadow-2xl">
        <div class="flex items-center justify-between rounded-t-2xl border-b border-slate-200 bg-white px-6 py-4">
          <h2 class="text-base font-extrabold">{{ editingId ? 'Sửa sản phẩm' : 'Thêm sản phẩm' }}</h2>
          <button type="button" class="rounded-lg p-1.5 hover:bg-slate-100" @click="closeForm"><X class="h-4 w-4" /></button>
        </div>

        <div class="grid grid-cols-1 gap-5 p-5 lg:grid-cols-3">
          <div class="space-y-5 lg:col-span-2">
            <p v-if="importedFrom" class="rounded-xl border border-emerald-200 bg-emerald-50 px-4 py-3 text-xs font-semibold leading-relaxed text-emerald-800">{{ importedFrom }}</p>

            <section class="space-y-3 rounded-2xl border border-slate-200 bg-white p-5">
              <label class="block space-y-1">
                <span class="text-xs font-bold text-slate-600">Tên sản phẩm</span>
                <input v-model="form.name" class="w-full rounded-xl border border-slate-200 px-3 py-2 text-sm" placeholder="Ví dụ: Micro không dây Hamax 03 Pro" />
              </label>
              <div class="space-y-1">
                <span class="text-xs font-bold text-slate-600">Mô tả</span>
                <RichTextEditor v-model="form.description" :site-id="siteId" placeholder="Giới thiệu sản phẩm, thông số, cách dùng... (có thể dán nội dung từ web khác)" />
              </div>
            </section>

            <section class="space-y-3 rounded-2xl border border-slate-200 bg-white p-5">
              <h3 class="text-sm font-extrabold">Hình ảnh &amp; video</h3>
              <ProductMediaManager v-model="form.media" :site-id="siteId" :cfg="mediaCfg" />
            </section>

            <section class="space-y-3 rounded-2xl border border-slate-200 bg-white p-5">
              <h3 class="text-sm font-extrabold">Giá</h3>
              <p v-if="hasVariants" class="rounded-xl bg-indigo-50 px-3 py-2 text-xs leading-relaxed text-indigo-800" data-testid="price-by-variant">
                Sản phẩm có phân loại: giá bán, giá gốc và giá nhập được đặt riêng cho từng biến thể ở mục “Phân loại” bên dưới.
              </p>
              <template v-else>
                <div class="grid grid-cols-1 gap-3 sm:grid-cols-3">
                  <label class="block space-y-1">
                    <span class="text-xs font-bold text-slate-600">Giá bán (đ)</span>
                    <input v-model.number="form.price" type="number" min="0" data-testid="product-price" class="w-full rounded-xl border border-slate-200 px-3 py-2 text-sm" />
                  </label>
                  <label class="block space-y-1">
                    <span class="text-xs font-bold text-slate-600">Giá gốc (đ, tùy chọn)</span>
                    <input v-model.number="form.compare_price" type="number" min="0" class="w-full rounded-xl border border-slate-200 px-3 py-2 text-sm" />
                  </label>
                  <label class="block space-y-1">
                    <span class="text-xs font-bold text-slate-600">Giá nhập (đ, chỉ bạn thấy)</span>
                    <input v-model.number="form.cost_price" type="number" min="0" class="w-full rounded-xl border border-slate-200 px-3 py-2 text-sm" />
                  </label>
                </div>
                <p class="flex flex-wrap gap-x-4 gap-y-1 text-xs text-slate-500">
                  <span v-if="discount" class="font-bold text-rose-600">Giảm {{ discount }}% so với giá gốc</span>
                  <span v-if="form.cost_price > 0 && form.price > 0">
                    Lợi nhuận mỗi sản phẩm:
                    <strong :class="profit >= 0 ? 'text-emerald-700' : 'text-rose-600'">{{ formatVnd(Math.abs(profit)) }}{{ profit < 0 ? ' (lỗ)' : '' }}</strong>
                    <template v-if="margin !== null"> ({{ margin }}%)</template>
                  </span>
                </p>
              </template>
            </section>

            <section class="space-y-3 rounded-2xl border border-slate-200 bg-white p-5">
              <h3 class="text-sm font-extrabold">Kho hàng</h3>
              <div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
                <label class="block space-y-1">
                  <span class="text-xs font-bold text-slate-600">SKU (mã sản phẩm)</span>
                  <input v-model="form.sku" maxlength="100" class="w-full rounded-xl border border-slate-200 px-3 py-2 text-sm" />
                </label>
                <label class="block space-y-1">
                  <span class="text-xs font-bold text-slate-600">Mã vạch (barcode)</span>
                  <input v-model="form.barcode" maxlength="100" class="w-full rounded-xl border border-slate-200 px-3 py-2 text-sm" />
                </label>
              </div>
              <p v-if="hasVariants" class="rounded-xl bg-indigo-50 px-3 py-2 text-xs leading-relaxed text-indigo-800">Tồn kho được quản lý riêng cho từng biến thể ở mục “Phân loại” bên dưới.</p>
              <div v-else class="flex items-center gap-4">
                <label class="flex items-center gap-2 text-xs font-bold text-slate-600">
                  <input v-model="form.unlimited" type="checkbox" /> Không giới hạn tồn kho
                </label>
                <input v-if="!form.unlimited" v-model.number="form.stock" type="number" min="0" data-testid="product-stock" class="w-28 rounded-xl border border-slate-200 px-3 py-1.5 text-sm" />
              </div>
            </section>
          </div>

          <div class="space-y-5">
            <section class="space-y-3 rounded-2xl border border-slate-200 bg-white p-5">
              <h3 class="text-sm font-extrabold">Trạng thái</h3>
              <label class="flex items-center gap-2 text-xs font-bold text-slate-600">
                <input v-model="form.is_active" type="checkbox" /> Hiển thị trên website
              </label>
              <p class="text-[11px] leading-relaxed text-slate-400">Bỏ chọn để lưu nháp: sản phẩm không hiện cho khách và không đặt hàng được.</p>
            </section>

            <section class="space-y-2 rounded-2xl border border-slate-200 bg-white p-5">
              <h3 class="text-sm font-extrabold">Thẻ (nhóm sản phẩm)</h3>
              <div class="flex flex-wrap gap-1.5">
                <span v-for="t in form.tags" :key="t" class="inline-flex items-center gap-1 rounded-full bg-slate-100 px-2.5 py-1 text-xs font-semibold text-slate-700">
                  {{ t }}
                  <button type="button" class="text-slate-400 hover:text-rose-600" @click="form.tags = form.tags.filter((x) => x !== t)"><X class="h-3 w-3" /></button>
                </span>
              </div>
              <input
                v-model="tagInput"
                class="w-full rounded-xl border border-slate-200 px-3 py-2 text-sm"
                placeholder="Gõ thẻ rồi nhấn Enter"
                @keydown="onTagKey"
              />
              <p class="text-[11px] leading-relaxed text-slate-400">Ở trình chỉnh sửa website, section “Sản phẩm” có thể chọn chỉ hiện các sản phẩm theo thẻ (vd. “bán chạy”, “khuyến mãi”). Thẻ nằm trong dữ liệu công khai của website.</p>
            </section>

            <section class="space-y-2 rounded-2xl border border-slate-200 bg-white p-5">
              <h3 class="text-sm font-extrabold">Nguồn hàng</h3>
              <input v-model="form.source_url" class="w-full rounded-xl border border-slate-200 px-3 py-2 text-xs" placeholder="Link sản phẩm gốc (AliExpress, 1688...)" />
              <a v-if="form.source_url" :href="form.source_url" target="_blank" rel="noopener noreferrer" class="inline-flex items-center gap-1 text-xs font-bold text-emerald-700 hover:underline">
                <ExternalLink class="h-3 w-3" /> Mở trang nguồn để nhập hàng
              </a>
              <p class="text-[11px] leading-relaxed text-slate-400">Chỉ bạn thấy mục này; khách hàng không bao giờ thấy giá nhập hay nguồn hàng.</p>
            </section>
          </div>

          <!-- Phân loại: cả bề ngang để bảng biến thể đủ rộng -->
          <section class="space-y-3 rounded-2xl border border-slate-200 bg-white p-5 lg:col-span-3">
            <h3 class="text-sm font-extrabold">Phân loại</h3>
            <ProductVariantsEditor v-model:options="form.options" v-model:variants="form.variants" :media="form.media" :defaults="variantDefaults" />
          </section>
        </div>

        <div class="sticky bottom-0 flex items-center justify-end gap-3 rounded-b-2xl border-t border-slate-200 bg-white px-6 py-3">
          <p v-if="errorMsg" class="mr-auto text-xs font-semibold text-rose-600">{{ errorMsg }}</p>
          <button class="rounded-xl border border-slate-200 px-4 py-2 text-sm font-bold" @click="closeForm">Hủy</button>
          <button class="inline-flex items-center gap-2 rounded-xl bg-emerald-600 px-5 py-2 text-sm font-bold text-white disabled:opacity-50" :disabled="saving" @click="save">
            <Loader2 v-if="saving" class="h-4 w-4 animate-spin" /> Lưu
          </button>
        </div>
      </div>
    </div>

    <ProductImportDialog v-if="showImport" :site-id="siteId" @close="showImport = false" @apply="onImported" />
  </div>
</template>
