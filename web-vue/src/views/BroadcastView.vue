<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { Pause, Play, Plus, Send, ShieldCheck, Trash2 } from 'lucide-vue-next'
import { api } from '../lib/api'

interface PageItem {
  page_id: string
  page_name: string
}

interface Campaign {
  id: string
  page_id: string
  name: string
  status: string
  pause_reason?: string
  total_recipients: number
  sent_count: number
  failed_count: number
  created_at: string
}

const pages = ref<PageItem[]>([])
const campaigns = ref<Campaign[]>([])
const loading = ref(false)
const submitting = ref(false)
const message = ref('')
const error = ref('')

const form = ref({
  page_id: '',
  name: '',
  templates: [
    'Chào {{full_name}}, shop vừa về thêm mẫu mới phù hợp với bạn. Bạn cần mình gửi ảnh tham khảo không ạ?',
    'Hello {{full_name}}, bên mình đang có ưu đãi dành riêng cho khách cũ hôm nay. Bạn quan tâm nhắn lại cho shop nhé!'
  ],
  only_within_24h: true,
  cooldown_days: 7,
  max_recipients: 100,
  min_delay_sec: 4,
  max_delay_sec: 9,
  batch_size: 20,
  batch_pause_sec: 60
})

function addTemplate() {
  if (form.value.templates.length < 5) {
    form.value.templates.push('')
  }
}

function removeTemplate(idx: number) {
  if (form.value.templates.length > 1) {
    form.value.templates.splice(idx, 1)
  }
}

async function fetchData() {
  loading.value = true
  try {
    const [pagesRes, campRes] = await Promise.all([
      api.get('/pages'),
      api.get('/broadcasts')
    ])
    pages.value = pagesRes.data?.data || pagesRes.data || []
    if (!form.value.page_id && pages.value.length > 0) {
      form.value.page_id = pages.value[0].page_id
    }
    campaigns.value = campRes.data?.data || []
  } catch (e: any) {
    error.value = e?.response?.data?.error || 'Không tải được dữ liệu chiến dịch'
  } finally {
    loading.value = false
  }
}

async function createCampaign() {
  error.value = ''
  message.value = ''
  submitting.value = true
  try {
    const res = await api.post('/broadcasts', form.value)
    message.value = `Đã tạo chiến dịch cho ${res.data.total_recipients} khách hàng (đang gửi giãn cách an toàn).`
    form.value.name = ''
    await fetchData()
  } catch (e: any) {
    error.value = e?.response?.data?.error || 'Tạo chiến dịch thất bại'
  } finally {
    submitting.value = false
  }
}

async function toggleStatus(camp: Campaign) {
  const nextStatus = camp.status === 'running' ? 'paused' : 'running'
  try {
    await api.patch(`/broadcasts/${camp.id}/status`, { status: nextStatus })
    await fetchData()
  } catch (e: any) {
    error.value = e?.response?.data?.error || 'Không đổi được trạng thái'
  }
}

onMounted(fetchData)
</script>

<template>
  <div class="mx-auto max-w-5xl space-y-6 p-6">
    <div class="flex items-center justify-between">
      <div>
        <h1 class="text-xl font-bold text-slate-900">Chăm sóc khách cũ tự động</h1>
        <p class="text-xs text-slate-500">
          Gửi tin theo hàng đợi giãn cách ngẫu nhiên, xoay vòng mẫu câu và tự ngắt khi có cảnh báo từ Meta
        </p>
      </div>
      <span class="inline-flex items-center gap-1.5 rounded-full bg-emerald-50 px-3 py-1 text-xs font-semibold text-emerald-700">
        <ShieldCheck class="h-4 w-4" />
        Anti-Spam Active (08:30 – 21:00)
      </span>
    </div>

    <div v-if="error" class="rounded-xl border border-red-200 bg-red-50 px-4 py-3 text-xs text-red-700">
      {{ error }}
    </div>
    <div v-if="message" class="rounded-xl border border-emerald-200 bg-emerald-50 px-4 py-3 text-xs text-emerald-700">
      {{ message }}
    </div>

    <!-- Form tạo chiến dịch -->
    <form @submit.prevent="createCampaign" class="space-y-4 rounded-2xl border border-slate-200 bg-white p-6 shadow-xs">
      <div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
        <div>
          <label class="mb-1 block text-xs font-semibold text-slate-700">Fanpage gửi tin</label>
          <select v-model="form.page_id" required class="w-full rounded-xl border border-slate-200 px-3 py-2 text-sm">
            <option v-for="p in pages" :key="p.page_id" :value="p.page_id">
              {{ p.page_name || p.page_id }}
            </option>
          </select>
        </div>
        <div>
          <label class="mb-1 block text-xs font-semibold text-slate-700">Tên chiến dịch</label>
          <input
            v-model="form.name"
            required
            placeholder="Ví dụ: Hỏi thăm khách cũ cuối tuần"
            class="w-full rounded-xl border border-slate-200 px-3 py-2 text-sm"
          />
        </div>
      </div>

      <!-- Các biến thể mẫu câu -->
      <div class="space-y-2">
        <div class="flex items-center justify-between">
         <label class="text-xs font-semibold text-slate-700">
  Biến thể nội dung (dùng <code v-pre>{{full_name}}</code> để chèn tên khách)
</label>
          <button
            type="button"
            @click="addTemplate"
            class="inline-flex items-center gap-1 text-xs font-semibold text-emerald-700 hover:underline cursor-pointer"
          >
            <Plus class="h-3.5 w-3.5" /> Thêm biến thể
          </button>
        </div>

        <div v-for="(_, idx) in form.templates" :key="idx" class="flex gap-2">
          <textarea
            v-model="form.templates[idx]"
            rows="2"
            required
            class="w-full rounded-xl border border-slate-200 p-2.5 text-sm"
            :placeholder="`Mẫu câu #${idx + 1}...`"
          />
          <button
            v-if="form.templates.length > 1"
            type="button"
            @click="removeTemplate(idx)"
            class="rounded-xl border border-slate-200 px-2.5 text-slate-400 hover:text-red-600 cursor-pointer"
          >
            <Trash2 class="h-4 w-4" />
          </button>
        </div>
      </div>

      <!-- Cấu hình giãn cách chống spam -->
      <div class="grid grid-cols-2 gap-3 rounded-xl bg-slate-50 p-4 sm:grid-cols-4">
        <div>
          <label class="block text-[11px] font-medium text-slate-600">Trễ tối thiểu (giây)</label>
          <input v-model.number="form.min_delay_sec" type="number" min="3" class="mt-1 w-full rounded-lg border border-slate-200 bg-white px-2.5 py-1.5 text-sm" />
        </div>
        <div>
          <label class="block text-[11px] font-medium text-slate-600">Trễ tối đa (giây)</label>
          <input v-model.number="form.max_delay_sec" type="number" min="4" class="mt-1 w-full rounded-lg border border-slate-200 bg-white px-2.5 py-1.5 text-sm" />
        </div>
        <div>
          <label class="block text-[11px] font-medium text-slate-600">Nghỉ sau mỗi (tin)</label>
          <input v-model.number="form.batch_size" type="number" min="5" class="mt-1 w-full rounded-lg border border-slate-200 bg-white px-2.5 py-1.5 text-sm" />
        </div>
        <div>
          <label class="block text-[11px] font-medium text-slate-600">Thời gian nghỉ lô (giây)</label>
          <input v-model.number="form.batch_pause_sec" type="number" min="30" class="mt-1 w-full rounded-lg border border-slate-200 bg-white px-2.5 py-1.5 text-sm" />
        </div>
      </div>

      <div class="flex flex-wrap items-center justify-between gap-4">
        <label class="inline-flex items-center gap-2 text-xs text-slate-700 cursor-pointer">
          <input v-model="form.only_within_24h" type="checkbox" class="rounded border-slate-300 text-emerald-600" />
          <span>Chỉ gửi khách tương tác trong vòng 24 giờ gần nhất (tuân thủ chính sách Meta)</span>
        </label>

        <button
          type="submit"
          :disabled="submitting"
          class="inline-flex items-center gap-2 rounded-xl bg-emerald-600 px-4 py-2.5 text-sm font-semibold text-white hover:bg-emerald-700 disabled:opacity-60 cursor-pointer"
        >
          <Send class="h-4 w-4" />
          <span>{{ submitting ? 'Đang tạo hàng đợi...' : 'Bắt đầu gửi an toàn' }}</span>
        </button>
      </div>
    </form>

    <!-- Danh sách chiến dịch -->
    <div class="overflow-hidden rounded-2xl border border-slate-200 bg-white">
      <div class="border-b border-slate-200 px-5 py-3.5 text-sm font-semibold text-slate-800">
        Tiến độ các chiến dịch
      </div>
      <div v-if="loading" class="p-6 text-center text-xs text-slate-500">Đang tải...</div>
      <div v-else-if="campaigns.length === 0" class="p-6 text-center text-xs text-slate-500">
        Chưa có chiến dịch nào.
      </div>
      <div v-else class="divide-y divide-slate-100">
        <div v-for="c in campaigns" :key="c.id" class="flex items-center justify-between gap-4 px-5 py-4">
          <div class="space-y-1">
            <div class="flex items-center gap-2">
              <span class="text-sm font-semibold text-slate-900">{{ c.name }}</span>
              <span
                class="rounded-full px-2 py-0.5 text-[11px] font-medium"
                :class="{
                  'bg-emerald-50 text-emerald-700': c.status === 'running',
                  'bg-amber-50 text-amber-700': c.status === 'paused',
                  'bg-slate-100 text-slate-700': c.status === 'completed'
                }"
              >
                {{ c.status }}
              </span>
            </div>
            <p class="text-xs text-slate-500">
              Đã gửi: <b>{{ c.sent_count }}/{{ c.total_recipients }}</b> | Lỗi: <b>{{ c.failed_count }}</b>
            </p>
            <p v-if="c.pause_reason" class="text-xs font-medium text-amber-700">
              {{ c.pause_reason }}
            </p>
          </div>

          <button
            v-if="c.status === 'running' || c.status === 'paused'"
            type="button"
            @click="toggleStatus(c)"
            class="inline-flex items-center gap-1.5 rounded-xl border border-slate-200 px-3 py-1.5 text-xs font-semibold text-slate-700 hover:bg-slate-50 cursor-pointer"
          >
            <Pause v-if="c.status === 'running'" class="h-3.5 w-3.5" />
            <Play v-else class="h-3.5 w-3.5" />
            <span>{{ c.status === 'running' ? 'Tạm dừng' : 'Tiếp tục' }}</span>
          </button>
        </div>
      </div>
    </div>
  </div>
</template>