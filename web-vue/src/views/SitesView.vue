<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ArrowLeft, ExternalLink, Globe, Loader2, Pencil, Plus, Trash2 } from 'lucide-vue-next'
import { api } from '../lib/api'
import { isCustomDomainHost } from '../lib/storeHost'
import DomainDialog from '../components/store/DomainDialog.vue'
import type { Site } from '../types/store'

const router = useRouter()

const loading = ref(true)
const busyId = ref<number | null>(null)
const errorMsg = ref('')
const sites = ref<Site[]>([])
const maxSites = ref(5)

const showCreate = ref(false)
const creating = ref(false)
const form = ref({ site_title: '', slug: '', template: 'wellness' })
const templates = [
  { key: 'wellness', name: 'Mỹ phẩm / Sức khoẻ', desc: 'Đầy đủ: banner, sản phẩm, giỏ hàng, chân trang', color: '#2f6f5e' },
  { key: '', name: 'Cơ bản', desc: 'Vài section đơn giản để tự xây', color: '#059669' }
]

const canCreate = computed(() => sites.value.length < maxSites.value)

function publicUrl(s: Site) {
  return `${window.location.origin}/s/${s.slug}`
}

// Tên miền riêng (bỏ qua giá trị cũ là domain hệ thống)
function customDomain(s: Site) {
  return s.domain && isCustomDomainHost(s.domain) ? s.domain : ''
}

const domainSite = ref<Site | null>(null)
function onDomainUpdated(d: { domain: string; status: string; error: string }) {
  if (!domainSite.value) return
  domainSite.value.domain = d.domain
  domainSite.value.domain_status = d.status
  domainSite.value.domain_error = d.error
}

function apiError(err: any, fallback: string) {
  return err?.response?.data?.error || fallback
}

async function loadSites() {
  loading.value = true
  errorMsg.value = ''
  try {
    const { data } = await api.get<{ sites: Site[]; max_sites: number }>('/storefront/sites')
    sites.value = data.sites || []
    maxSites.value = data.max_sites || 5
  } catch (err) {
    errorMsg.value = apiError(err, 'Không tải được danh sách website.')
  } finally {
    loading.value = false
  }
}

async function createSite() {
  if (!form.value.site_title.trim()) {
    errorMsg.value = 'Hãy nhập tên website.'
    return
  }
  creating.value = true
  errorMsg.value = ''
  try {
    const { data } = await api.post<{ site: Site }>('/storefront/sites', {
      site_title: form.value.site_title.trim(),
      slug: form.value.slug.trim(),
      template: form.value.template
    })
    showCreate.value = false
    form.value = { site_title: '', slug: '', template: 'wellness' }
    router.push({ name: 'site-edit', params: { id: data.site.id } })
  } catch (err) {
    errorMsg.value = apiError(err, 'Tạo website thất bại.')
  } finally {
    creating.value = false
  }
}

async function togglePublish(s: Site) {
  busyId.value = s.id
  errorMsg.value = ''
  try {
    const action = s.is_published ? 'unpublish' : 'publish'
    const { data } = await api.post<{ site: Site }>(`/storefront/sites/${s.id}/${action}`)
    Object.assign(s, data.site)
  } catch (err) {
    errorMsg.value = apiError(err, 'Thao tác thất bại.')
  } finally {
    busyId.value = null
  }
}

async function removeSite(s: Site) {
  if (!window.confirm(`Xóa website "${s.site_title}"? Hành động này không thể hoàn tác.`)) return
  busyId.value = s.id
  errorMsg.value = ''
  try {
    await api.delete(`/storefront/sites/${s.id}`)
    sites.value = sites.value.filter((x) => x.id !== s.id)
  } catch (err) {
    errorMsg.value = apiError(err, 'Xóa website thất bại.')
  } finally {
    busyId.value = null
  }
}

onMounted(loadSites)
</script>

<template>
  <div class="min-h-screen bg-slate-50 text-slate-900">
    <div class="mx-auto max-w-5xl px-4 py-8 space-y-6">
      <div class="flex flex-wrap items-center justify-between gap-3">
        <div class="flex items-center gap-3">
          <button class="rounded-xl border border-slate-200 bg-white p-2 hover:bg-slate-100 lg:hidden" @click="router.push('/')">
            <ArrowLeft class="h-4 w-4" />
          </button>
          <div>
            <h1 class="text-xl font-extrabold">Website bán hàng</h1>
            <p class="text-xs text-slate-500">{{ sites.length }}/{{ maxSites }} website</p>
          </div>
        </div>
        <button
          class="inline-flex items-center gap-2 rounded-xl bg-emerald-600 px-4 py-2 text-sm font-bold text-white hover:bg-emerald-700 disabled:opacity-50"
          :disabled="!canCreate"
          @click="showCreate = true"
        >
          <Plus class="h-4 w-4" /> Tạo website
        </button>
      </div>

      <p v-if="errorMsg" class="rounded-xl bg-rose-50 px-4 py-2 text-sm font-semibold text-rose-600">{{ errorMsg }}</p>

      <div v-if="loading" class="flex items-center justify-center gap-2 py-20 text-sm text-slate-500">
        <Loader2 class="h-5 w-5 animate-spin text-emerald-600" /> Đang tải...
      </div>

      <div v-else-if="!sites.length" class="rounded-2xl border border-dashed border-slate-300 bg-white py-16 text-center">
        <Globe class="mx-auto h-8 w-8 text-slate-400" />
        <p class="mt-2 text-sm text-slate-500">Bạn chưa có website nào. Bấm “Tạo website” để bắt đầu.</p>
      </div>

      <ul v-else class="space-y-3">
        <li
          v-for="s in sites"
          :key="s.id"
          class="flex flex-wrap items-center justify-between gap-3 rounded-2xl border border-slate-200 bg-white p-4"
        >
          <div class="min-w-0">
            <div class="flex items-center gap-2">
              <span class="truncate text-sm font-extrabold">{{ s.site_title }}</span>
              <span
                class="rounded-full px-2 py-0.5 text-[10px] font-bold"
                :class="s.is_published ? 'bg-emerald-100 text-emerald-700' : 'bg-slate-100 text-slate-500'"
              >
                {{ s.is_published ? 'Đang chạy' : 'Chưa xuất bản' }}
              </span>
            </div>
            <a
              v-if="s.is_published"
              :href="publicUrl(s)"
              target="_blank"
              rel="noopener"
              class="mt-0.5 inline-flex items-center gap-1 text-xs text-emerald-700 hover:underline"
            >
              {{ publicUrl(s) }} <ExternalLink class="h-3 w-3" />
            </a>
            <p v-else class="mt-0.5 text-xs text-slate-400">/s/{{ s.slug }}</p>
            <div v-if="customDomain(s)" class="mt-1 flex items-center gap-1.5 text-xs">
              <Globe class="h-3.5 w-3.5 text-slate-400" />
              <a
                v-if="s.domain_status === 'active' && s.is_published"
                :href="`https://${customDomain(s)}`"
                target="_blank"
                rel="noopener"
                class="inline-flex items-center gap-1 font-semibold text-emerald-700 hover:underline"
              >https://{{ customDomain(s) }} <ExternalLink class="h-3 w-3" /></a>
              <span v-else class="font-semibold text-slate-600">{{ customDomain(s) }}</span>
              <span
                v-if="s.domain_status !== 'active'"
                class="rounded-full px-2 py-0.5 text-[10px] font-bold"
                :class="s.domain_status === 'error' ? 'bg-rose-100 text-rose-700' : 'bg-amber-100 text-amber-700'"
              >{{ s.domain_status === 'error' ? 'Lỗi tên miền' : 'Chờ trỏ DNS' }}</span>
            </div>
          </div>

          <div class="flex flex-wrap items-center gap-2 whitespace-nowrap">
                        <button
              class="rounded-xl border border-slate-200 px-3 py-1.5 text-xs font-bold hover:bg-slate-50"
              @click="router.push({ name: 'site-products', params: { id: s.id } })"
            >
              Sản phẩm
            </button>
            <button
              class="rounded-xl border border-slate-200 px-3 py-1.5 text-xs font-bold hover:bg-slate-50"
              @click="router.push({ name: 'site-orders', params: { id: s.id } })"
            >
              Đơn hàng
            </button>
            <button
              class="inline-flex items-center gap-1 rounded-xl border border-slate-200 px-3 py-1.5 text-xs font-bold hover:bg-slate-50"
              @click="router.push({ name: 'site-edit', params: { id: s.id } })"
            >
              <Pencil class="h-3.5 w-3.5" /> Chỉnh sửa
            </button>
            <button
              class="inline-flex items-center gap-1 rounded-xl border border-slate-200 px-3 py-1.5 text-xs font-bold hover:bg-slate-50"
              @click="domainSite = s"
            >
              <Globe class="h-3.5 w-3.5" /> Tên miền
            </button>
            <button
              class="rounded-xl border border-slate-200 px-3 py-1.5 text-xs font-bold hover:bg-slate-50 disabled:opacity-50"
              :disabled="busyId === s.id"
              @click="togglePublish(s)"
            >
              {{ s.is_published ? 'Gỡ xuống' : 'Xuất bản' }}
            </button>
            <button
              class="rounded-xl border border-rose-200 p-1.5 text-rose-600 hover:bg-rose-50 disabled:opacity-50"
              :disabled="busyId === s.id"
              @click="removeSite(s)"
            >
              <Trash2 class="h-4 w-4" />
            </button>
          </div>
        </li>
      </ul>
    </div>

    <DomainDialog v-if="domainSite" :site="domainSite" @close="domainSite = null" @updated="onDomainUpdated" />

    <div v-if="showCreate" class="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4">
      <div class="w-full max-w-md space-y-4 rounded-2xl bg-white p-6">
        <h2 class="text-base font-extrabold">Tạo website mới</h2>
        <label class="block space-y-1">
          <span class="text-xs font-bold text-slate-600">Tên website</span>
          <input v-model="form.site_title" class="w-full rounded-xl border border-slate-200 px-3 py-2 text-sm" placeholder="Ví dụ: Mỹ phẩm Hana" />
        </label>
        <label class="block space-y-1">
          <span class="text-xs font-bold text-slate-600">Đường dẫn (để trống để tự tạo)</span>
          <div class="flex items-center rounded-xl border border-slate-200 text-sm">
            <span class="pl-3 text-slate-400">/s/</span>
            <input v-model="form.slug" class="w-full rounded-xl px-1 py-2 outline-none" placeholder="my-pham-hana" />
          </div>
        </label>
        <div class="space-y-1.5">
          <span class="text-xs font-bold text-slate-600">Mẫu giao diện</span>
          <div class="grid grid-cols-2 gap-2">
            <button
              v-for="t in templates"
              :key="t.key"
              type="button"
              class="rounded-xl border-2 p-3 text-left transition"
              :class="form.template === t.key ? 'border-emerald-600 bg-emerald-50' : 'border-slate-200 hover:border-slate-300'"
              @click="form.template = t.key"
            >
              <div class="mb-2 flex h-14 flex-col gap-1 overflow-hidden rounded-lg bg-slate-100 p-1.5">
                <div class="h-1.5 rounded" :style="{ backgroundColor: t.color }" />
                <div class="flex-1 rounded" :style="{ background: t.key ? `linear-gradient(120deg, ${t.color}, #a7c4bc)` : '#e2e8f0' }" />
                <div class="flex gap-1"><div class="h-2 flex-1 rounded bg-white" /><div class="h-2 flex-1 rounded bg-white" /><div class="h-2 flex-1 rounded bg-white" /></div>
              </div>
              <div class="text-xs font-extrabold">{{ t.name }}</div>
              <div class="text-[11px] text-slate-500">{{ t.desc }}</div>
            </button>
          </div>
        </div>
        <div class="flex justify-end gap-2">
          <button class="rounded-xl border border-slate-200 px-4 py-2 text-sm font-bold" @click="showCreate = false">Hủy</button>
          <button
            class="inline-flex items-center gap-2 rounded-xl bg-emerald-600 px-4 py-2 text-sm font-bold text-white disabled:opacity-50"
            :disabled="creating"
            @click="createSite"
          >
            <Loader2 v-if="creating" class="h-4 w-4 animate-spin" /> Tạo
          </button>
        </div>
      </div>
    </div>
  </div>
</template>