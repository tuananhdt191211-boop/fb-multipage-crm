<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { Loader2 } from 'lucide-vue-next'
import { api } from '../lib/api'
import StoreRenderer from '../components/store/StoreRenderer.vue'
import type { PublicProduct, StoreBlock } from '../types/store'

interface PublicStorefrontResponse {
  site_title?: string
  primary_color?: string
  blocks_json?: string
  products?: PublicProduct[]
}

const route = useRoute()

const loading = ref(true)
const errorMsg = ref('')
const siteTitle = ref('Cửa hàng')
const primaryColor = ref('#059669')
const blocks = ref<StoreBlock[]>([])
const products = ref<PublicProduct[]>([])

function extractProductsFromBlocks(blockList: StoreBlock[]): PublicProduct[] {
  const result: PublicProduct[] = []
  for (const blk of blockList) {
    if (blk.type === 'products' && Array.isArray(blk.data?.items)) {
      for (let i = 0; i < blk.data.items.length; i++) {
        const raw = blk.data.items[i] as Record<string, any>
        const prod: Record<string, any> = {
          id: raw.id ?? `blk_prod_${i}`,
          name: String(raw.name ?? ''),
          price: raw.price ?? 0,
          image_url: String(raw.image_url ?? raw.image ?? ''),
          image: String(raw.image ?? raw.image_url ?? ''),
          desc: String(raw.desc ?? raw.description ?? ''),
          description: String(raw.description ?? raw.desc ?? '')
        }
        result.push(prod as PublicProduct)
      }
    }
  }
  return result
}

async function loadPublicStore() {
  loading.value = true
  errorMsg.value = ''
  try {
    // Ưu tiên slug từ /s/:slug, sau đó ?slug=, cuối cùng mới theo tên miền
    const slug =
      (typeof route.params.slug === 'string' && route.params.slug) ||
      (typeof route.query.slug === 'string' && route.query.slug.trim()) ||
      ''
    const params: Record<string, string> = {}
    if (slug) params.slug = slug
    else
      params.domain =
        typeof route.query.domain === 'string' && route.query.domain.trim()
          ? route.query.domain.trim()
          : window.location.hostname

    const { data } = await api.get<PublicStorefrontResponse>(
      '/storefront/public',
      { params }
    )

    if (data?.site_title) {
      siteTitle.value = data.site_title
      document.title = data.site_title
    }
    if (data?.primary_color) primaryColor.value = data.primary_color
    const parsed: StoreBlock[] = JSON.parse(data?.blocks_json || '[]')
    const validBlocks = Array.isArray(parsed) ? parsed : []
    blocks.value = validBlocks
    products.value =
      Array.isArray(data?.products) && data.products.length > 0
        ? data.products
        : extractProductsFromBlocks(validBlocks)
  } catch (err: any) {
    errorMsg.value = err?.response?.data?.error || 'Không tìm thấy trang bán hàng đã xuất bản.'
  } finally {
    loading.value = false
  }
}

onMounted(loadPublicStore)
</script>

<template>
  <div class="min-h-screen bg-white text-slate-900">
    <div v-if="loading" class="flex items-center justify-center gap-2 py-24 text-sm text-slate-500">
      <Loader2 class="h-5 w-5 animate-spin text-emerald-600" />
      <span>Đang tải cửa hàng...</span>
    </div>
    <div v-else-if="errorMsg" class="mx-auto max-w-lg px-6 py-24 text-center">
      <p class="text-sm font-semibold text-rose-600">{{ errorMsg }}</p>
    </div>
    <StoreRenderer
      v-else
      :site-title="siteTitle"
      :primary-color="primaryColor"
      :blocks="blocks"
      :products="products"
    />
  </div>
</template>