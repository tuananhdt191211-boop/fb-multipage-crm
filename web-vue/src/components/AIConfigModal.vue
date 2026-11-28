<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { KeyRound, Save, CheckCircle2, AlertCircle, Sparkles, X, Loader2 } from 'lucide-vue-next'
import { api } from '@/lib/api'

const emit = defineEmits<{ (e: 'close'): void }>()

const baseURL = ref('https://generativelanguage.googleapis.com/v1beta/openai')
const apiKeys = ref('')
const model = ref('gemini-3.5-flash-lite')
const keyCount = ref(0)

const loading = ref(false)
const testing = ref(false)
const statusMsg = ref('')
const errorMsg = ref('')

const presets = [
  {
    name: 'Google Gemini (Miễn phí - Khuyên dùng)',
    baseURL: 'https://generativelanguage.googleapis.com/v1beta/openai',
    model: 'gemini-3.5-flash-lite',
    placeholder: 'Dán mã AIzaSy... từ aistudio.google.com/apikey (mỗi dòng 1 key để xoay vòng)'
  },
  {
    name: 'Groq Cloud (Miễn phí - Phản hồi < 1s)',
    baseURL: 'https://api.groq.com/openai/v1',
    model: 'llama-3.3-70b-versatile',
    placeholder: 'Dán mã gsk_... từ console.groq.com/keys'
  },
  {
    name: 'OpenAI (GPT-4o mini)',
    baseURL: 'https://api.openai.com/v1',
    model: 'gpt-4o-mini',
    placeholder: 'Dán mã sk-... từ platform.openai.com/api-keys'
  }
]

function applyPreset(p: typeof presets[0]) {
  baseURL.value = p.baseURL
  model.value = p.model
}

async function fetchCurrentConfig() {
  try {
    const res = await api.get('/ai/config')
    if (res.data.base_url) baseURL.value = res.data.base_url
    if (res.data.api_keys) apiKeys.value = res.data.api_keys
    if (res.data.model) model.value = res.data.model
    keyCount.value = res.data.key_count || 0
  } catch {
    // Bỏ qua nếu chưa có dữ liệu
  }
}

async function handleSave() {
  errorMsg.value = ''
  statusMsg.value = ''
  loading.value = true
  try {
    const res = await api.post('/ai/config', {
      base_url: baseURL.value,
      api_keys: apiKeys.value,
      model: model.value
    })
    statusMsg.value = res.data.message || 'Đã lưu API Key thành công!'
    keyCount.value = res.data.key_count || 0
  } catch (err: any) {
    errorMsg.value = err?.response?.data?.error || 'Lỗi khi lưu cấu hình API Key'
  } finally {
    loading.value = false
  }
}

async function handleTest() {
  errorMsg.value = ''
  statusMsg.value = ''
  testing.value = true
  try {
    // Lưu trước rồi test ngay
    await api.post('/ai/config', {
      base_url: baseURL.value,
      api_keys: apiKeys.value,
      model: model.value
    })
    const res = await api.post('/ai/config/test')
    statusMsg.value = `${res.data.message} ("${res.data.reply}")`
  } catch (err: any) {
    errorMsg.value = err?.response?.data?.error || 'Kết nối AI thất bại, vui lòng kiểm tra lại mã Key'
  } finally {
    testing.value = false
  }
}

onMounted(fetchCurrentConfig)
</script>

<template>
  <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4">
    <div class="w-full max-w-lg rounded-2xl bg-white p-6 shadow-xl">
      <div class="mb-4 flex items-center justify-between border-b pb-3">
        <div class="flex items-center gap-2">
          <div class="flex h-9 w-9 items-center justify-center rounded-lg bg-emerald-100 text-emerald-700">
            <KeyRound class="h-5 w-5"/>
          </div>
          <div>
            <h3 class="text-base font-bold text-slate-800">Cài đặt API Key AI Trực Chat</h3>
            <p class="text-xs text-slate-500">Đang hoạt động: {{ keyCount }} API Key (tự động xoay vòng khi đầy)</p>
          </div>
        </div>
        <button @click="emit('close')" class="rounded-lg p-1 text-slate-400 hover:bg-slate-100 hover:text-slate-600">
          <X class="h-5 w-5"/>
        </button>
      </div>

      <!-- Chọn nhanh nhà cung cấp -->
      <div class="mb-4">
        <label class="mb-1.5 block text-xs font-semibold text-slate-600">Chọn nhanh nhà cung cấp AI:</label>
        <div class="grid grid-cols-1 gap-1.5 sm:grid-cols-3">
          <button
            v-for="p in presets"
            :key="p.name"
            type="button"
            @click="applyPreset(p)"
            class="rounded-lg border px-2.5 py-2 text-left text-xs font-medium transition"
            :class="baseURL === p.baseURL ? 'border-emerald-500 bg-emerald-50 text-emerald-800' : 'border-slate-200 hover:bg-slate-50 text-slate-700'"
          >
            {{ p.name.split(' (')[0] }}
          </button>
        </div>
      </div>

      <div class="space-y-3">
        <div>
          <label class="mb-1 block text-xs font-semibold text-slate-700">
            Danh sách API Key (nhập nhiều key cách nhau bằng dấu phẩy hoặc xuống dòng)
          </label>
          <textarea
            v-model="apiKeys"
            rows="3"
            placeholder="AIzaSyKey1..., AIzaSyKey2..."
            class="w-full rounded-xl border border-slate-300 p-2.5 font-mono text-xs focus:border-emerald-500 focus:outline-none"
          />
        </div>

        <div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
          <div>
            <label class="mb-1 block text-xs font-semibold text-slate-700">Model AI</label>
            <input
              v-model="model"
              type="text"
              class="w-full rounded-xl border border-slate-300 px-3 py-2 text-xs focus:border-emerald-500 focus:outline-none"
            />
          </div>
          <div>
            <label class="mb-1 block text-xs font-semibold text-slate-700">API Base URL</label>
            <input
              v-model="baseURL"
              type="text"
              class="w-full rounded-xl border border-slate-300 px-3 py-2 text-xs focus:border-emerald-500 focus:outline-none"
            />
          </div>
        </div>
      </div>

      <div v-if="statusMsg" class="mt-3 flex items-start gap-2 rounded-xl bg-emerald-50 p-3 text-xs text-emerald-800">
        <CheckCircle2 class="mt-0.5 h-4 w-4 shrink-0 text-emerald-600"/>
        <span>{{ statusMsg }}</span>
      </div>

      <div v-if="errorMsg" class="mt-3 flex items-start gap-2 rounded-xl bg-red-50 p-3 text-xs text-red-700">
        <AlertCircle class="mt-0.5 h-4 w-4 shrink-0 text-red-600"/>
        <span>{{ errorMsg }}</span>
      </div>

      <div class="mt-5 flex items-center justify-end gap-2 border-t pt-4">
        <button
          type="button"
          :disabled="testing || loading"
          @click="handleTest"
          class="inline-flex items-center gap-1.5 rounded-xl border border-emerald-600 px-4 py-2 text-xs font-semibold text-emerald-700 hover:bg-emerald-50 disabled:opacity-50"
        >
          <Loader2 class="h-3.5 w-3.5 animate-spin" v-if="testing"/>
          <Sparkles class="h-3.5 w-3.5" v-else/>
          Lưu & Chạy thử AI
        </button>
        <button
          type="button"
          :disabled="loading || testing"
          @click="handleSave"
          class="inline-flex items-center gap-1.5 rounded-xl bg-emerald-600 px-4 py-2 text-xs font-semibold text-white hover:bg-emerald-700 disabled:opacity-50"
        >
          <Loader2 class="h-3.5 w-3.5 animate-spin" v-if="loading"/>
          <Save class="h-3.5 w-3.5" v-else/>
          Lưu cấu hình
        </button>
      </div>
    </div>
  </div>
</template>