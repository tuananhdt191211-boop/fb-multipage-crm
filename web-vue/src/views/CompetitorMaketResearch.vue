<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import {
  BookmarkPlus,
  Copy,
  Check,
  Filter,
  Layers,
  Search,
  Sparkles,
  Trash2,
  TrendingUp
} from 'lucide-vue-next'
import { api } from '../lib/api'

interface CompetitorSample {
  id: string
  productGroup: string
  competitorName: string
  channel: 'Facebook' | 'TikTok' | 'Shopee / Web'
  hook: string
  offerOrPrice: string
  contentSummary: string
  insightNote: string
  createdAt: string
}

type OutputFormat = 'post' | 'ugc_video' | 'dm_script'

const STORAGE_KEY = 'pagepilot_competitor_research_v1'

const defaultGroups = [
  'Thiết bị âm thanh & phụ kiện',
  'Chăm sóc sức khỏe & làm đẹp',
  'Quà tặng & sản phẩm mùa vụ',
  'Đồ gia dụng & tiện ích'
]

const productGroups = ref<string[]>([...defaultGroups])
const selectedGroup = ref<string>('all')
const searchText = ref('')
const copiedId = ref<string | null>(null)
const generating = ref(false)
const statusBanner = ref('')

// Form thêm mẫu đối thủ / tín hiệu thị trường mới
const newItem = ref({
  productGroup: defaultGroups[0],
  competitorName: '',
  channel: 'Facebook' as CompetitorSample['channel'],
  hook: '',
  offerOrPrice: '',
  contentSummary: '',
  insightNote: ''
})

// Form sinh kịch bản / mẫu bài nhanh
const generatorConfig = ref({
  targetGroup: defaultGroups[0],
  format: 'post' as OutputFormat,
  yourProductName: '',
  yourOffer: 'Miễn phí vận chuyển, kiểm tra hàng trước khi thanh toán',
  tone: 'Thân thiện, rõ ràng, tập trung giải quyết nỗi đau khách hàng'
})

const generatedDraft = ref('')

const samples = ref<CompetitorSample[]>([
  {
    id: 'sample_1',
    productGroup: 'Thiết bị âm thanh & phụ kiện',
    competitorName: 'Shop Audio Pro',
    channel: 'Facebook',
    hook: 'Quay video ngoài trời ồn gió làm mất tiếng? Xem ngay cách lọc ồn 1 chạm',
    offerOrPrice: 'Giảm 25%, tặng kèm đầu lọc gió + bảo hành 12 tháng',
    contentSummary: 'So sánh trực tiếp âm thanh có bật lọc ồn và tắt lọc ồn ở ngã tư đông xe, kêu gọi bình luận để nhận báo giá.',
    insightNote: 'Video test thực tế 15 giây đầu giữ chân người xem tốt, chốt đơn mạnh nhờ quà tặng phụ kiện.',
    createdAt: new Date().toISOString()
  },
  {
    id: 'sample_2',
    productGroup: 'Chăm sóc sức khỏe & làm đẹp',
    competitorName: 'Care Lab VN',
    channel: 'TikTok',
    hook: '3 sai lầm khiến da đầu nhanh bết và nhiều gàu dù gội mỗi ngày',
    offerOrPrice: 'Combo 2 chai freeship + tư vấn tình trạng da đầu miễn phí',
    contentSummary: 'Kể chuyện góc nhìn người dùng, chỉ ra nguyên nhân phổ biến rồi giới thiệu giải pháp dịu nhẹ, an toàn.',
    insightNote: 'Khách quan tâm thành phần lành tính và cam kết không kích ứng trước khi hỏi giá.',
    createdAt: new Date().toISOString()
  }
])

onMounted(() => {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (raw) {
      const parsed = JSON.parse(raw)
      if (Array.isArray(parsed.samples)) samples.value = parsed.samples
      if (Array.isArray(parsed.productGroups) && parsed.productGroups.length > 0) {
        productGroups.value = parsed.productGroups
      }
    }
  } catch {
    // Bỏ qua nếu dữ liệu localStorage không hợp lệ
  }
})

watch(
  [samples, productGroups],
  () => {
    try {
      localStorage.setItem(
        STORAGE_KEY,
        JSON.stringify({
          samples: samples.value,
          productGroups: productGroups.value
        })
      )
    } catch {
      // ignore
    }
  },
  { deep: true }
)

const filteredSamples = computed(() => {
  const q = searchText.value.trim().toLowerCase()
  return samples.value.filter((item) => {
    const matchGroup = selectedGroup.value === 'all' || item.productGroup === selectedGroup.value
    const matchQuery =
      !q ||
      item.competitorName.toLowerCase().includes(q) ||
      item.hook.toLowerCase().includes(q) ||
      item.contentSummary.toLowerCase().includes(q) ||
      item.insightNote.toLowerCase().includes(q) ||
      item.productGroup.toLowerCase().includes(q)
    return matchGroup && matchQuery
  })
})

const groupStats = computed(() => {
  const counts: Record<string, number> = {}
  for (const g of productGroups.value) counts[g] = 0
  for (const s of samples.value) {
    counts[s.productGroup] = (counts[s.productGroup] || 0) + 1
  }
  return counts
})

function addSample() {
  if (!newItem.value.competitorName.trim() && !newItem.value.hook.trim()) {
    statusBanner.value = 'Hãy nhập tên đối thủ hoặc câu hook mở đầu trước khi lưu.'
    return
  }

  const grp = newItem.value.productGroup.trim() || defaultGroups[0]
  if (!productGroups.value.includes(grp)) {
    productGroups.value.push(grp)
  }

  samples.value.unshift({
    id: `cmp_${Date.now()}`,
    productGroup: grp,
    competitorName: newItem.value.competitorName.trim() || 'Đối thủ ẩn danh',
    channel: newItem.value.channel,
    hook: newItem.value.hook.trim() || 'Chưa ghi nhận hook mở đầu',
    offerOrPrice: newItem.value.offerOrPrice.trim() || 'Không công khai ưu đãi',
    contentSummary: newItem.value.contentSummary.trim(),
    insightNote: newItem.value.insightNote.trim(),
    createdAt: new Date().toISOString()
  })

  newItem.value.competitorName = ''
  newItem.value.hook = ''
  newItem.value.offerOrPrice = ''
  newItem.value.contentSummary = ''
  newItem.value.insightNote = ''
  statusBanner.value = `Đã lưu dữ liệu nghiên cứu vào nhóm "${grp}".`
}

function removeSample(id: string) {
  samples.value = samples.value.filter((s) => s.id !== id)
}

function buildLocalFallbackDraft(groupSamples: CompetitorSample[]): string {
  const prod = generatorConfig.value.yourProductName.trim() || generatorConfig.value.targetGroup
  const offer = generatorConfig.value.yourOffer.trim() || 'Ưu đãi đặc biệt hôm nay'
  const topHooks = groupSamples.slice(0, 3).map((s) => s.hook).filter(Boolean)
  const topInsights = groupSamples.slice(0, 3).map((s) => s.insightNote).filter(Boolean)

  if (generatorConfig.value.format === 'ugc_video') {
    return `[KỊCH BẢN VIDEO NGẮN - ${prod.toUpperCase()}]
• 00-03s (Hook): ${topHooks[0] || `Đang phân vân chọn ${prod} dùng cho ổn định? Xem ngay điều này!`}
• 03-12s (Vấn đề & Thị trường): Khách trong nhóm ${generatorConfig.value.targetGroup} thường gặp các điểm cần lưu ý: ${topInsights[0] || 'chất lượng không đồng đều và khó chọn mẫu phù hợp nhu cầu'}.
• 12-22s (Giải pháp của bạn): Giới thiệu ${prod} — giải quyết trực tiếp nhu cầu sử dụng thực tế, thao tác dễ dàng.
• 22-30s (Ưu đãi & CTA): ${offer}. Để lại bình luận hoặc nhắn tin để shop tư vấn đúng nhu cầu cho bạn!`
  }

  if (generatorConfig.value.format === 'dm_script') {
    return `[KỊCH BẢN TƯ VẤN & CHỐT ĐƠN - ${prod.toUpperCase()}]
1. Chào & mở đầu: Dạ em chào anh/chị, không biết mình đang quan tâm ${prod} để sử dụng cho nhu cầu cụ thể nào ạ?
2. Giá trị & khác biệt: Dòng ${prod} bên em tối ưu cho trải nghiệm thực tế, giải quyết tốt các điểm khách hay băn khoăn (${topInsights[0] || 'dễ dùng, bền bỉ, hỗ trợ nhanh'}).
3. Ưu đãi: Hiện bên em đang áp dụng ${offer}.
4. Chốt đơn: Anh/chị chốt mẫu này thì cho em xin Họ tên + SĐT + Địa chỉ để shop lên đơn và gửi sớm cho mình nhé!`
  }

  return `[MẪU BÀI ĐĂNG PAGE - ${prod.toUpperCase()}]
${topHooks[0] || `Giải pháp ${prod} tối ưu cho nhu cầu thực tế của bạn!`}

Qua theo dõi thị trường nhóm "${generatorConfig.value.targetGroup}", khách hàng thường quan tâm nhất đến hiệu quả sử dụng thực tế và chính sách hỗ trợ rõ ràng.

👉 ${prod} mang lại:
- Trải nghiệm sử dụng ổn định, dễ thao tác ngay từ lần đầu.
- Phù hợp nhiều tình huống thực tế, tiết kiệm thời gian.
- ${offer}.

📩 Nhắn tin ngay cho Page để được tư vấn chi tiết và giữ ưu đãi hôm nay!`
}

async function generateQuickScript() {
  generating.value = true
  statusBanner.value = ''
  const groupItems = samples.value.filter((s) => s.productGroup === generatorConfig.value.targetGroup)

  try {
    const { data } = await api.post<{ draft?: string; content?: string; reply?: string }>(
      '/competitors/analyze-group',
      {
        product_group: generatorConfig.value.targetGroup,
        format: generatorConfig.value.format,
        product_name: generatorConfig.value.yourProductName,
        offer: generatorConfig.value.yourOffer,
        tone: generatorConfig.value.tone,
        samples: groupItems
      }
    )
    generatedDraft.value = data.draft || data.content || data.reply || buildLocalFallbackDraft(groupItems)
    statusBanner.value = 'Đã tạo bản nháp kịch bản dựa trên dữ liệu nhóm sản phẩm.'
  } catch {
    // Nếu backend chưa có endpoint phân tích nhóm, tự sinh bản nháp cục bộ ngay để không gián đoạn thao tác
    generatedDraft.value = buildLocalFallbackDraft(groupItems)
    statusBanner.value = 'Đã tạo bản nháp nhanh từ dữ liệu nghiên cứu của nhóm sản phẩm.'
  } finally {
    generating.value = false
  }
}

async function copyDraft(text: string, id = 'draft') {
  try {
    await navigator.clipboard.writeText(text)
    copiedId.value = id
    setTimeout(() => {
      if (copiedId.value === id) copiedId.value = null
    }, 1800)
  } catch {
    // ignore
  }
}
</script>

<template>
  <div class="space-y-5 p-4 sm:p-6 bg-slate-50 min-h-full text-slate-900">
    <!-- Tiêu đề & Thống kê theo nhóm sản phẩm -->
    <div class="flex flex-wrap items-center justify-between gap-3 rounded-2xl border border-slate-200 bg-white p-4 shadow-2xs">
      <div class="flex items-center gap-3">
        <div class="flex h-10 w-10 items-center justify-center rounded-xl bg-emerald-600 text-white">
          <TrendingUp class="h-5 w-5" />
        </div>
        <div>
          <h2 class="text-base font-bold text-slate-900">Nghiên cứu đối thủ & thị trường theo nhóm sản phẩm</h2>
          <p class="text-xs text-slate-500">
            Phân loại mẫu đối thủ theo từng ngách sản phẩm và tạo nhanh bài đăng / kịch bản video / tin nhắn chốt đơn.
          </p>
        </div>
      </div>

      <div class="flex flex-wrap items-center gap-1.5">
        <button
          type="button"
          @click="selectedGroup = 'all'"
          class="rounded-xl border px-3 py-1.5 text-xs font-bold transition cursor-pointer"
          :class="selectedGroup === 'all' ? 'border-emerald-600 bg-emerald-50 text-emerald-800' : 'border-slate-200 bg-white text-slate-600'"
        >
          Tất cả ({{ samples.length }})
        </button>
        <button
          v-for="grp in productGroups"
          :key="grp"
          type="button"
          @click="selectedGroup = grp"
          class="rounded-xl border px-3 py-1.5 text-xs font-semibold transition cursor-pointer"
          :class="selectedGroup === grp ? 'border-emerald-600 bg-emerald-50 text-emerald-800 font-bold' : 'border-slate-200 bg-white text-slate-600'"
        >
          {{ grp }} ({{ groupStats[grp] || 0 }})
        </button>
      </div>
    </div>

    <div
      v-if="statusBanner"
      class="rounded-xl border border-emerald-200 bg-emerald-50 px-4 py-2 text-xs font-semibold text-emerald-800"
    >
      {{ statusBanner }}
    </div>

    <!-- Lưới 2 cột chính: Trái (Lưu & Lọc mẫu đối thủ) | Phải (Sinh kịch bản / mẫu bài nhanh) -->
    <div class="grid grid-cols-1 gap-5 xl:grid-cols-12">
      <!-- CỘT TRÁI: Nhập tín hiệu thị trường & Danh sách mẫu theo nhóm -->
      <div class="space-y-4 xl:col-span-7">
        <div class="rounded-2xl border border-slate-200 bg-white p-4 space-y-3 shadow-2xs">
          <div class="flex items-center gap-1.5 text-xs font-bold text-slate-900">
            <BookmarkPlus class="h-4 w-4 text-emerald-600" />
            <span>Ghi nhận đối thủ / mẫu bài thị trường</span>
          </div>

          <div class="grid grid-cols-1 gap-2.5 sm:grid-cols-3">
            <div>
              <label class="mb-1 block text-[11px] font-semibold text-slate-600">Nhóm sản phẩm</label>
              <input
                v-model="newItem.productGroup"
                list="product-group-datalist"
                type="text"
                placeholder="Chọn hoặc nhập nhóm..."
                class="w-full rounded-xl border border-slate-200 bg-slate-50/60 px-3 py-2 text-xs font-semibold outline-none focus:border-emerald-600 focus:bg-white"
              />
              <datalist id="product-group-datalist">
                <option v-for="g in productGroups" :key="g" :value="g" />
              </datalist>
            </div>

            <div>
              <label class="mb-1 block text-[11px] font-semibold text-slate-600">Tên đối thủ / nguồn</label>
              <input
                v-model="newItem.competitorName"
                type="text"
                placeholder="VD: Shop ABC..."
                class="w-full rounded-xl border border-slate-200 bg-slate-50/60 px-3 py-2 text-xs outline-none focus:border-emerald-600 focus:bg-white"
              />
            </div>

            <div>
              <label class="mb-1 block text-[11px] font-semibold text-slate-600">Kênh theo dõi</label>
              <select
                v-model="newItem.channel"
                class="w-full rounded-xl border border-slate-200 bg-slate-50/60 px-3 py-2 text-xs font-semibold outline-none focus:border-emerald-600 focus:bg-white"
              >
                <option value="Facebook">Facebook</option>
                <option value="TikTok">TikTok</option>
                <option value="Shopee / Web">Shopee / Web</option>
              </select>
            </div>
          </div>

          <div class="grid grid-cols-1 gap-2.5 sm:grid-cols-2">
            <div>
              <label class="mb-1 block text-[11px] font-semibold text-slate-600">Hook / Câu mở đầu thu hút</label>
              <input
                v-model="newItem.hook"
                type="text"
                placeholder="Câu tiêu đề hoặc 3 giây đầu video..."
                class="w-full rounded-xl border border-slate-200 bg-slate-50/60 px-3 py-2 text-xs outline-none focus:border-emerald-600 focus:bg-white"
              />
            </div>

            <div>
              <label class="mb-1 block text-[11px] font-semibold text-slate-600">Giá bán / Ưu đãi / Quà tặng</label>
              <input
                v-model="newItem.offerOrPrice"
                type="text"
                placeholder="VD: Tặng phụ kiện, freeship, giảm 20%..."
                class="w-full rounded-xl border border-slate-200 bg-slate-50/60 px-3 py-2 text-xs outline-none focus:border-emerald-600 focus:bg-white"
              />
            </div>
          </div>

          <div>
            <label class="mb-1 block text-[11px] font-semibold text-slate-600">Tóm tắt nội dung & góc tiếp cận</label>
            <textarea
              v-model="newItem.contentSummary"
              rows="2"
              placeholder="Cách đối thủ triển khai nội dung, so sánh tính năng, cách kêu gọi tương tác..."
              class="w-full resize-none rounded-xl border border-slate-200 bg-slate-50/60 p-2.5 text-xs outline-none focus:border-emerald-600 focus:bg-white"
            />
          </div>

          <div>
            <label class="mb-1 block text-[11px] font-semibold text-slate-600">Nhận định thị trường (điểm mạnh / điểm có thể làm tốt hơn)</label>
            <input
              v-model="newItem.insightNote"
              type="text"
              placeholder="VD: Khách hay hỏi bảo hành & cách lắp đặt..."
              class="w-full rounded-xl border border-slate-200 bg-slate-50/60 px-3 py-2 text-xs outline-none focus:border-emerald-600 focus:bg-white"
            />
          </div>

          <div class="flex justify-end">
            <button
              type="button"
              @click="addSample"
              class="inline-flex items-center gap-1.5 rounded-xl bg-emerald-600 px-4 py-2 text-xs font-semibold text-white transition hover:bg-emerald-700 cursor-pointer"
            >
              <BookmarkPlus class="h-3.5 w-3.5" />
              <span>Lưu vào kho nghiên cứu</span>
            </button>
          </div>
        </div>

        <!-- Tìm kiếm & danh sách mẫu đã lưu -->
        <div class="rounded-2xl border border-slate-200 bg-white p-4 space-y-3 shadow-2xs">
          <div class="flex flex-wrap items-center justify-between gap-2">
            <span class="inline-flex items-center gap-1.5 text-xs font-bold text-slate-900">
              <Filter class="h-3.5 w-3.5 text-emerald-600" />
              <span>Mẫu nghiên cứu ({{ filteredSamples.length }})</span>
            </span>

            <div class="relative w-full sm:w-64">
              <Search class="pointer-events-none absolute left-3 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-slate-400" />
              <input
                v-model="searchText"
                type="text"
                placeholder="Tìm đối thủ, hook, nhận định..."
                class="w-full rounded-xl border border-slate-200 bg-slate-50/60 py-1.5 pl-8 pr-3 text-xs outline-none focus:border-emerald-600 focus:bg-white"
              />
            </div>
          </div>

          <div v-if="filteredSamples.length === 0" class="py-8 text-center text-xs text-slate-500">
            Chưa có mẫu nghiên cứu nào khớp bộ lọc hiện tại.
          </div>

          <div v-else class="space-y-2.5 max-h-[520px] overflow-y-auto pr-1">
            <div
              v-for="item in filteredSamples"
              :key="item.id"
              class="rounded-xl border border-slate-200 bg-slate-50/40 p-3.5 space-y-1.5"
            >
              <div class="flex items-start justify-between gap-2">
                <div class="flex flex-wrap items-center gap-1.5">
                  <span class="rounded-md bg-emerald-100 px-2 py-0.5 text-[10px] font-bold text-emerald-800">
                    {{ item.productGroup }}
                  </span>
                  <span class="rounded-md bg-slate-200/80 px-2 py-0.5 text-[10px] font-semibold text-slate-700">
                    {{ item.channel }}
                  </span>
                  <span class="text-xs font-bold text-slate-900">{{ item.competitorName }}</span>
                </div>

                <button
                  type="button"
                  @click="removeSample(item.id)"
                  class="text-slate-400 hover:text-red-600 cursor-pointer"
                  title="Xóa mẫu"
                >
                  <Trash2 class="h-3.5 w-3.5" />
                </button>
              </div>

              <p class="text-xs font-semibold text-slate-800">Hook: "{{ item.hook }}"</p>
              <p v-if="item.offerOrPrice" class="text-[11px] font-medium text-emerald-700">
                Ưu đãi / Giá: {{ item.offerOrPrice }}
              </p>
              <p v-if="item.contentSummary" class="text-xs text-slate-600 whitespace-pre-line">
                {{ item.contentSummary }}
              </p>
              <p v-if="item.insightNote" class="rounded-lg bg-white border border-slate-200/80 px-2.5 py-1.5 text-[11px] text-slate-700">
                <strong>Nhận định:</strong> {{ item.insightNote }}
              </p>
            </div>
          </div>
        </div>
      </div>

      <!-- CỘT PHẢI: Sinh kịch bản / mẫu bài nhanh theo nhóm sản phẩm -->
      <div class="space-y-4 xl:col-span-5">
        <div class="rounded-2xl border border-slate-200 bg-white p-4 space-y-3 shadow-2xs">
          <div class="flex items-center gap-1.5 text-xs font-bold text-slate-900">
            <Layers class="h-4 w-4 text-emerald-600" />
            <span>Sinh kịch bản / mẫu bài nhanh theo nhóm</span>
          </div>

          <div>
            <label class="mb-1 block text-[11px] font-semibold text-slate-600">Chọn nhóm sản phẩm để tổng hợp</label>
            <select
              v-model="generatorConfig.targetGroup"
              class="w-full rounded-xl border border-slate-200 bg-slate-50/60 px-3 py-2 text-xs font-semibold outline-none focus:border-emerald-600 focus:bg-white"
            >
              <option v-for="g in productGroups" :key="g" :value="g">
                {{ g }} ({{ groupStats[g] || 0 }} mẫu)
              </option>
            </select>
          </div>

          <div>
            <label class="mb-1 block text-[11px] font-semibold text-slate-600">Loại đầu ra cần tạo</label>
            <div class="grid grid-cols-3 gap-1.5">
              <button
                type="button"
                @click="generatorConfig.format = 'post'"
                class="rounded-xl border py-1.5 text-[11px] font-bold transition cursor-pointer"
                :class="generatorConfig.format === 'post' ? 'border-emerald-600 bg-emerald-50 text-emerald-800' : 'border-slate-200 bg-white text-slate-600'"
              >
                Bài đăng Page
              </button>
              <button
                type="button"
                @click="generatorConfig.format = 'ugc_video'"
                class="rounded-xl border py-1.5 text-[11px] font-bold transition cursor-pointer"
                :class="generatorConfig.format === 'ugc_video' ? 'border-emerald-600 bg-emerald-50 text-emerald-800' : 'border-slate-200 bg-white text-slate-600'"
              >
                Video ngắn
              </button>
              <button
                type="button"
                @click="generatorConfig.format = 'dm_script'"
                class="rounded-xl border py-1.5 text-[11px] font-bold transition cursor-pointer"
                :class="generatorConfig.format === 'dm_script' ? 'border-emerald-600 bg-emerald-50 text-emerald-800' : 'border-slate-200 bg-white text-slate-600'"
              >
                Kịch bản chat
              </button>
            </div>
          </div>

          <div>
            <label class="mb-1 block text-[11px] font-semibold text-slate-600">Tên sản phẩm của bạn</label>
            <input
              v-model="generatorConfig.yourProductName"
              type="text"
              placeholder="Nhập tên sản phẩm cụ thể của shop..."
              class="w-full rounded-xl border border-slate-200 bg-slate-50/60 px-3 py-2 text-xs outline-none focus:border-emerald-600 focus:bg-white"
            />
          </div>

          <div>
            <label class="mb-1 block text-[11px] font-semibold text-slate-600">Ưu đãi / Chính sách của shop</label>
            <input
              v-model="generatorConfig.yourOffer"
              type="text"
              placeholder="VD: Freeship, đổi trả 30 ngày, quà tặng..."
              class="w-full rounded-xl border border-slate-200 bg-slate-50/60 px-3 py-2 text-xs outline-none focus:border-emerald-600 focus:bg-white"
            />
          </div>

          <button
            type="button"
            @click="generateQuickScript"
            :disabled="generating"
            class="inline-flex w-full items-center justify-center gap-1.5 rounded-xl bg-emerald-600 px-4 py-2.5 text-xs font-bold text-white transition hover:bg-emerald-700 disabled:opacity-60 cursor-pointer"
          >
            <Sparkles class="h-4 w-4" />
            <span>{{ generating ? 'Đang tổng hợp...' : 'Tạo kịch bản / mẫu bài nhanh' }}</span>
          </button>

          <div v-if="generatedDraft" class="space-y-2 pt-2 border-t border-slate-200">
            <div class="flex items-center justify-between">
              <span class="text-xs font-bold text-slate-900">Bản nháp gợi ý</span>
              <button
                type="button"
                @click="copyDraft(generatedDraft, 'draft')"
                class="inline-flex items-center gap-1 rounded-lg border border-slate-200 bg-slate-50 px-2.5 py-1 text-[11px] font-semibold text-slate-700 hover:bg-emerald-50 hover:text-emerald-800 cursor-pointer"
              >
                <Check v-if="copiedId === 'draft'" class="h-3.5 w-3.5 text-emerald-600" />
                <Copy v-else class="h-3.5 w-3.5" />
                <span>{{ copiedId === 'draft' ? 'Đã sao chép' : 'Sao chép' }}</span>
              </button>
            </div>

            <textarea
              v-model="generatedDraft"
              rows="12"
              class="w-full resize-y rounded-xl border border-slate-200 bg-slate-50/60 p-3 text-xs leading-relaxed text-slate-800 outline-none focus:border-emerald-600 focus:bg-white"
            />
          </div>
        </div>
      </div>
    </div>
  </div>
</template>