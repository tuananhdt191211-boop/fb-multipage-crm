<script setup lang="ts">
// Gắn tên miền riêng (khách tự mua) cho một website bán hàng
import { computed, onMounted, ref } from 'vue'
import { CheckCircle2, Copy, Globe, Loader2, RefreshCw, Trash2, X } from 'lucide-vue-next'
import { api } from '../../lib/api'
import type { Site } from '../../types/store'

interface DomainInfo {
  domain: string
  status: '' | 'manual' | 'pending' | 'active' | 'error'
  error: string
  cname_target: string
  cf_configured: boolean
  is_apex?: boolean
  host_label?: string
  txt_name?: string
  txt_value?: string
}

const props = defineProps<{ site: Site }>()
const emit = defineEmits<{ close: []; updated: [info: DomainInfo] }>()

const info = ref<DomainInfo | null>(null)
const input = ref('')
const loading = ref(true)
const saving = ref(false)
const checking = ref(false)
const errorMsg = ref('')
const copied = ref('')

const statusView = computed(() => {
  switch (info.value?.status) {
    case 'active':
      return { text: 'Đang hoạt động', cls: 'bg-emerald-100 text-emerald-700' }
    case 'pending':
      return { text: 'Đang chờ trỏ DNS & cấp SSL', cls: 'bg-amber-100 text-amber-700' }
    case 'manual':
      return { text: 'Chờ quản trị viên kích hoạt', cls: 'bg-sky-100 text-sky-700' }
    case 'error':
      return { text: 'Lỗi', cls: 'bg-rose-100 text-rose-700' }
    default:
      return null
  }
})

function apiError(err: any, fallback: string) {
  return err?.response?.data?.error || fallback
}

function apply(d: DomainInfo) {
  info.value = d
  if (d.domain) input.value = d.domain
  emit('updated', d)
}

async function refresh(showSpinner = false) {
  if (showSpinner) checking.value = true
  errorMsg.value = ''
  try {
    const { data } = await api.get<DomainInfo>(`/storefront/sites/${props.site.id}/domain`)
    apply(data)
  } catch (err) {
    errorMsg.value = apiError(err, 'Không tải được thông tin tên miền.')
  } finally {
    loading.value = false
    checking.value = false
  }
}

async function save() {
  if (!input.value.trim()) {
    errorMsg.value = 'Hãy nhập tên miền, ví dụ www.cuahangcuaban.vn'
    return
  }
  saving.value = true
  errorMsg.value = ''
  try {
    const { data } = await api.put<DomainInfo>(`/storefront/sites/${props.site.id}/domain`, { domain: input.value.trim() })
    apply(data)
  } catch (err) {
    errorMsg.value = apiError(err, 'Không gắn được tên miền.')
  } finally {
    saving.value = false
  }
}

async function remove() {
  if (!info.value?.domain || !window.confirm(`Gỡ tên miền ${info.value.domain} khỏi website này?`)) return
  saving.value = true
  errorMsg.value = ''
  try {
    const { data } = await api.delete<DomainInfo>(`/storefront/sites/${props.site.id}/domain`)
    input.value = ''
    apply(data)
  } catch (err) {
    errorMsg.value = apiError(err, 'Không gỡ được tên miền.')
  } finally {
    saving.value = false
  }
}

async function copy(text: string) {
  try {
    await navigator.clipboard.writeText(text)
    copied.value = text
    setTimeout(() => (copied.value = ''), 1500)
  } catch {
    /* trình duyệt chặn clipboard */
  }
}

onMounted(() => refresh())
</script>

<template>
  <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4" @click.self="emit('close')">
    <div class="max-h-[90dvh] w-full max-w-lg overflow-y-auto rounded-2xl bg-white p-6 shadow-xl">
      <div class="flex items-start justify-between gap-3">
        <div>
          <h2 class="flex items-center gap-2 text-base font-extrabold"><Globe class="h-5 w-5 text-emerald-600" /> Tên miền riêng</h2>
          <p class="mt-0.5 text-xs text-slate-500">Cho website “{{ site.site_title }}” chạy trên tên miền khách đã mua.</p>
        </div>
        <button class="rounded-lg p-1.5 hover:bg-slate-100" aria-label="Đóng" @click="emit('close')"><X class="h-5 w-5" /></button>
      </div>

      <div v-if="loading" class="flex items-center justify-center gap-2 py-12 text-sm text-slate-500">
        <Loader2 class="h-5 w-5 animate-spin text-emerald-600" /> Đang tải...
      </div>

      <div v-else class="mt-5 space-y-5">
        <label class="block space-y-1.5">
          <span class="text-xs font-bold text-slate-600">Tên miền</span>
          <div class="flex gap-2">
            <input
              v-model="input"
              class="min-w-0 flex-1 rounded-xl border border-slate-200 px-3 py-2 text-sm outline-none focus:border-emerald-500"
              placeholder="www.cuahangcuaban.vn"
              autocomplete="off"
              spellcheck="false"
              @keyup.enter="save"
            />
            <button
              class="inline-flex items-center gap-1.5 rounded-xl bg-emerald-600 px-4 py-2 text-sm font-bold text-white hover:bg-emerald-700 disabled:opacity-50"
              :disabled="saving || (input.trim() === info?.domain && info?.status === 'active')"
              @click="save"
            >
              <Loader2 v-if="saving" class="h-4 w-4 animate-spin" /> {{ info?.domain ? 'Cập nhật' : 'Gắn' }}
            </button>
          </div>
          <span class="block text-[11px] text-slate-500">Nên dùng dạng <b>www.</b>tenmien.vn — trỏ DNS đơn giản nhất.</span>
        </label>

        <p v-if="errorMsg" class="rounded-xl bg-rose-50 px-3 py-2 text-xs font-semibold text-rose-600">{{ errorMsg }}</p>

        <template v-if="info?.domain">
          <div class="flex flex-wrap items-center justify-between gap-2 rounded-xl border border-slate-200 p-3">
            <div class="min-w-0">
              <div class="truncate text-sm font-extrabold">{{ info.domain }}</div>
              <span v-if="statusView" class="mt-1 inline-block rounded-full px-2 py-0.5 text-[10px] font-bold" :class="statusView.cls">{{ statusView.text }}</span>
            </div>
            <div class="flex items-center gap-2">
              <a
                v-if="info.status === 'active'"
                :href="`https://${info.domain}`"
                target="_blank"
                rel="noopener"
                class="rounded-xl border border-slate-200 px-3 py-1.5 text-xs font-bold text-emerald-700 hover:bg-slate-50"
              >Mở web</a>
              <button
                v-else
                class="inline-flex items-center gap-1 rounded-xl border border-slate-200 px-3 py-1.5 text-xs font-bold hover:bg-slate-50 disabled:opacity-50"
                :disabled="checking"
                @click="refresh(true)"
              >
                <RefreshCw class="h-3.5 w-3.5" :class="checking && 'animate-spin'" /> Kiểm tra lại
              </button>
              <button class="rounded-xl border border-rose-200 p-1.5 text-rose-600 hover:bg-rose-50 disabled:opacity-50" :disabled="saving" aria-label="Gỡ tên miền" @click="remove">
                <Trash2 class="h-4 w-4" />
              </button>
            </div>
          </div>

          <p v-if="info.error && info.status !== 'active'" class="rounded-xl bg-amber-50 px-3 py-2 text-xs text-amber-800">{{ info.error }}</p>

          <div v-if="info.status === 'active'" class="flex items-start gap-2 rounded-xl bg-emerald-50 p-3 text-xs text-emerald-800">
            <CheckCircle2 class="mt-0.5 h-4 w-4 shrink-0" />
            <span>Tên miền đã chạy, có HTTPS. Khách vào <b>https://{{ info.domain }}</b> sẽ thấy website (nhớ bấm “Xuất bản”).</span>
          </div>

          <div v-else class="space-y-3">
            <h3 class="text-sm font-extrabold">Cách trỏ tên miền</h3>
            <p class="text-xs text-slate-600">
              Vào trang quản lý DNS ở nơi mua tên miền (Mắt Bão, PA Việt Nam, Tenten, iNET, GoDaddy…) và thêm bản ghi:
            </p>

            <div v-if="!info.is_apex" class="overflow-hidden rounded-xl border border-slate-200 text-xs">
              <div class="grid grid-cols-[70px_1fr_1.6fr] bg-slate-50 px-3 py-2 font-bold text-slate-500">
                <span>Loại</span><span>Tên / Host</span><span>Giá trị / Trỏ tới</span>
              </div>
              <div class="grid grid-cols-[70px_1fr_1.6fr] items-center gap-1 px-3 py-2.5">
                <span class="font-bold">CNAME</span>
                <button class="truncate text-left font-mono hover:text-emerald-700" @click="copy(info.host_label || '')">{{ info.host_label }}</button>
                <button class="inline-flex min-w-0 items-center gap-1 text-left font-mono hover:text-emerald-700" @click="copy(info.cname_target)">
                  <span class="truncate">{{ info.cname_target }}</span>
                  <CheckCircle2 v-if="copied === info.cname_target" class="h-3.5 w-3.5 shrink-0 text-emerald-600" /><Copy v-else class="h-3.5 w-3.5 shrink-0 text-slate-400" />
                </button>
              </div>
              <div v-if="info.txt_name" class="grid grid-cols-[70px_1fr_1.6fr] items-center gap-1 border-t border-slate-100 px-3 py-2.5">
                <span class="font-bold">TXT</span>
                <button class="truncate text-left font-mono hover:text-emerald-700" @click="copy(info.txt_name)">{{ info.txt_name }}</button>
                <button class="truncate text-left font-mono hover:text-emerald-700" @click="copy(info.txt_value || '')">{{ info.txt_value }}</button>
              </div>
            </div>

            <div v-else class="space-y-2 rounded-xl border border-amber-200 bg-amber-50 p-3 text-xs text-amber-900">
              <p><b>{{ info.domain }}</b> là tên miền gốc (không có www). Tên miền gốc chỉ chạy được khi chuyển <b>nameserver</b> của tên miền sang Cloudflare.</p>
              <p>Cách dễ hơn: gắn <b>www.{{ info.domain }}</b>, rồi ở nơi mua tên miền bật “chuyển hướng” {{ info.domain }} → www.{{ info.domain }}.</p>
              <button class="rounded-lg bg-white px-3 py-1.5 font-bold text-amber-900 shadow-sm hover:bg-amber-100" @click="input = `www.${info.domain}`; save()">
                Đổi sang www.{{ info.domain }}
              </button>
            </div>

            <ul class="list-disc space-y-1 pl-5 text-[11px] text-slate-500">
              <li>Bấm vào giá trị để sao chép. Xoá bản ghi A / CNAME cũ trùng tên (nếu có).</li>
              <li>DNS thường cập nhật sau 5–30 phút (có khi tới 24 giờ). HTTPS được cấp tự động.</li>
              <li v-if="info.status === 'manual'">Hệ thống chưa kết nối Cloudflare tự động — quản trị viên cần thêm tên miền này vào Cloudflare Pages.</li>
              <li>Website phải ở trạng thái “Đang chạy” thì tên miền mới hiển thị nội dung.</li>
            </ul>
          </div>
        </template>
      </div>
    </div>
  </div>
</template>
