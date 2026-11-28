<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ArrowLeft, Loader2, Pencil, Plus, Trash2 } from 'lucide-vue-next'
import { api } from '../lib/api'
import { adminMoney, adminMoneyTitle } from '../lib/uiI18n'
import type { AdminProduct, Site } from '../types/store'

const route = useRoute()
const router = useRouter()
const siteId = computed(() => String(route.params.id || ''))

const loading = ref(true)
const saving = ref(false)
const errorMsg = ref('')
const site = ref<Site | null>(null)
const products = ref<AdminProduct[]>([])
const maxProducts = ref(200)

const showForm = ref(false)
const editingId = ref<number | null>(null)
const unlimited = ref(true)
const form = ref({
  name: '',
  description: '',
  price: 0,
  compare_price: 0,
  image_url: '',
  stock: 0,
  is_active: true,
  sort_order: 0
})

function apiError(err: any, fallback: string) {
  return err?.response?.data?.error || fallback
}

async function load() {
  loading.value = true
  errorMsg.value = ''
  try {
    const [siteRes, prodRes] = await Promise.all([
      api.get<{ site: Site }>(`/storefront/sites/${siteId.value}`),
      api.get<{ products: AdminProduct[]; max_products: number }>(`/storefront/sites/${siteId.value}/products`)
    ])
    site.value = siteRes.data.site
    products.value = prodRes.data.products || []
    maxProducts.value = prodRes.data.max_products || 200
  } catch (err) {
    errorMsg.value = apiError(err, 'Không tải được dữ liệu.')
  } finally {
    loading.value = false
  }
}

function openCreate() {
  editingId.value = null
  unlimited.value = true
  form.value = { name: '', description: '', price: 0, compare_price: 0, image_url: '', stock: 0, is_active: true, sort_order: 0 }
  showForm.value = true
}

function openEdit(p: AdminProduct) {
  editingId.value = p.id
  unlimited.value = p.stock === -1
  form.value = {
    name: p.name,
    description: p.description,
    price: p.price,
    compare_price: p.compare_price,
    image_url: p.image_url,
    stock: p.stock === -1 ? 0 : p.stock,
    is_active: p.is_active,
    sort_order: p.sort_order
  }
  showForm.value = true
}

async function save() {
  if (!form.value.name.trim()) {
    errorMsg.value = 'Hãy nhập tên sản phẩm.'
    return
  }
  saving.value = true
  errorMsg.value = ''
  const payload = {
    ...form.value,
    price: Number(form.value.price) || 0,
    compare_price: Number(form.value.compare_price) || 0,
    stock: unlimited.value ? -1 : Math.max(0, Number(form.value.stock) || 0)
  }
  try {
    if (editingId.value) {
      const { data } = await api.put<{ product: AdminProduct }>(
        `/storefront/sites/${siteId.value}/products/${editingId.value}`,
        payload
      )
      const idx = products.value.findIndex((x) => x.id === editingId.value)
      if (idx >= 0) products.value[idx] = data.product
    } else {
      const { data } = await api.post<{ product: AdminProduct }>(`/storefront/sites/${siteId.value}/products`, payload)
      products.value.push(data.product)
    }
    showForm.value = false
  } catch (err) {
    errorMsg.value = apiError(err, 'Lưu sản phẩm thất bại.')
  } finally {
    saving.value = false
  }
}

async function toggleActive(p: AdminProduct) {
  try {
    const { data } = await api.put<{ product: AdminProduct }>(`/storefront/sites/${siteId.value}/products/${p.id}`, {
      ...p,
      is_active: !p.is_active
    })
    Object.assign(p, data.product)
  } catch (err) {
    errorMsg.value = apiError(err, 'Cập nhật thất bại.')
  }
}

async function remove(p: AdminProduct) {
  if (!window.confirm(`Xóa sản phẩm "${p.name}"?`)) return
  try {
    await api.delete(`/storefront/sites/${siteId.value}/products/${p.id}`)
    products.value = products.value.filter((x) => x.id !== p.id)
  } catch (err) {
    errorMsg.value = apiError(err, 'Xóa sản phẩm thất bại.')
  }
}

function stockLabel(p: AdminProduct) {
  if (p.stock === -1) return 'Không giới hạn'
  if (p.stock === 0) return 'Hết hàng'
  return `${p.stock} cái`
}

onMounted(load)
</script>

<template>
  <div class="min-h-screen bg-slate-50 text-slate-900">
    <div class="mx-auto max-w-5xl px-4 py-8 space-y-6">
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
        <button
          class="inline-flex items-center gap-2 rounded-xl bg-emerald-600 px-4 py-2 text-sm font-bold text-white hover:bg-emerald-700"
          @click="openCreate"
        >
          <Plus class="h-4 w-4" /> Thêm sản phẩm
        </button>
      </div>

      <p v-if="errorMsg" class="rounded-xl bg-rose-50 px-4 py-2 text-sm font-semibold text-rose-600">{{ errorMsg }}</p>

      <div v-if="loading" class="flex items-center justify-center gap-2 py-20 text-sm text-slate-500">
        <Loader2 class="h-5 w-5 animate-spin text-emerald-600" /> Đang tải...
      </div>

      <div v-else-if="!products.length" class="rounded-2xl border border-dashed border-slate-300 bg-white py-16 text-center text-sm text-slate-500">
        Chưa có sản phẩm nào. Bấm “Thêm sản phẩm” để bắt đầu.
      </div>

      <ul v-else class="space-y-3">
        <li
          v-for="p in products"
          :key="p.id"
          class="flex flex-wrap items-center justify-between gap-3 rounded-2xl border border-slate-200 bg-white p-3"
          :class="{ 'opacity-60': !p.is_active }"
        >
          <div class="flex min-w-0 items-center gap-3">
            <div class="h-14 w-14 shrink-0 overflow-hidden rounded-xl bg-slate-100">
              <img v-if="p.image_url" :src="p.image_url" :alt="p.name" class="h-full w-full object-cover" />
            </div>
            <div class="min-w-0">
              <div class="truncate text-sm font-extrabold">{{ p.name }}</div>
              <div class="text-xs text-slate-500">
                <span :title="adminMoneyTitle(p.price)">{{ adminMoney(p.price) }}</span>
                <span v-if="p.compare_price > p.price" class="ml-1 line-through text-slate-400" :title="adminMoneyTitle(p.compare_price)">{{ adminMoney(p.compare_price) }}</span>
                · Tồn kho: {{ stockLabel(p) }}
              </div>
            </div>
          </div>
          <div class="flex items-center gap-2">
            <button class="rounded-xl border border-slate-200 px-3 py-1.5 text-xs font-bold hover:bg-slate-50" @click="toggleActive(p)">
              {{ p.is_active ? 'Đang bán' : 'Đã ẩn' }}
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

    <div v-if="showForm" class="fixed inset-0 z-50 flex items-center justify-center overflow-auto bg-black/40 p-4">
      <div class="w-full max-w-lg space-y-3 rounded-2xl bg-white p-6">
        <h2 class="text-base font-extrabold">{{ editingId ? 'Sửa sản phẩm' : 'Thêm sản phẩm' }}</h2>

        <label class="block space-y-1">
          <span class="text-xs font-bold text-slate-600">Tên sản phẩm</span>
          <input v-model="form.name" class="w-full rounded-xl border border-slate-200 px-3 py-2 text-sm" />
        </label>

        <div class="grid grid-cols-2 gap-3">
          <label class="block space-y-1">
            <span class="text-xs font-bold text-slate-600">Giá bán (đ)</span>
            <input v-model.number="form.price" type="number" min="0" class="w-full rounded-xl border border-slate-200 px-3 py-2 text-sm" />
          </label>
          <label class="block space-y-1">
            <span class="text-xs font-bold text-slate-600">Giá gốc (đ, tùy chọn)</span>
            <input v-model.number="form.compare_price" type="number" min="0" class="w-full rounded-xl border border-slate-200 px-3 py-2 text-sm" />
          </label>
        </div>

        <label class="block space-y-1">
          <span class="text-xs font-bold text-slate-600">URL ảnh</span>
          <input v-model="form.image_url" placeholder="https://..." class="w-full rounded-xl border border-slate-200 px-3 py-2 text-sm" />
        </label>

        <label class="block space-y-1">
          <span class="text-xs font-bold text-slate-600">Mô tả</span>
          <textarea v-model="form.description" rows="3" class="w-full rounded-xl border border-slate-200 px-3 py-2 text-sm" />
        </label>

        <div class="flex items-center gap-4">
          <label class="flex items-center gap-2 text-xs font-bold text-slate-600">
            <input v-model="unlimited" type="checkbox" /> Không giới hạn tồn kho
          </label>
          <input
            v-if="!unlimited"
            v-model.number="form.stock"
            type="number"
            min="0"
            class="w-28 rounded-xl border border-slate-200 px-3 py-1.5 text-sm"
          />
        </div>

        <label class="flex items-center gap-2 text-xs font-bold text-slate-600">
          <input v-model="form.is_active" type="checkbox" /> Hiển thị trên website
        </label>

        <div class="flex justify-end gap-2 pt-2">
          <button class="rounded-xl border border-slate-200 px-4 py-2 text-sm font-bold" @click="showForm = false">Hủy</button>
          <button
            class="inline-flex items-center gap-2 rounded-xl bg-emerald-600 px-4 py-2 text-sm font-bold text-white disabled:opacity-50"
            :disabled="saving"
            @click="save"
          >
            <Loader2 v-if="saving" class="h-4 w-4 animate-spin" /> Lưu
          </button>
        </div>
      </div>
    </div>
  </div>
</template>