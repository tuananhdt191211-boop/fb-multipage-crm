<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { Loader2 } from 'lucide-vue-next'
import StoreRenderer from '../components/store/StoreRenderer.vue'
import CartDrawer from '../components/store/CartDrawer.vue'
import { loadPublicSite } from '../lib/publicSite'
import { useCart } from '../lib/cart'
import type { PublicSiteData } from '../types/store'

const route = useRoute()
const loading = ref(true)
const errorMsg = ref('')
const site = ref<PublicSiteData | null>(null)

async function load() {
  loading.value = true
  errorMsg.value = ''
  try {
    // Ưu tiên slug từ /s/:slug, sau đó ?slug=, cuối cùng mới theo tên miền
    const slug =
      (typeof route.params.slug === 'string' && route.params.slug) ||
      (typeof route.query.slug === 'string' && route.query.slug.trim()) ||
      ''
    const domain = typeof route.query.domain === 'string' && route.query.domain.trim() ? route.query.domain.trim() : window.location.hostname
    site.value = await loadPublicSite(slug ? { slug } : { domain })
    document.title = site.value.site_title
    useCart(site.value.slug).syncWith(site.value.products)
  } catch (err: any) {
    errorMsg.value = err?.response?.data?.error || 'Không tìm thấy trang bán hàng đã xuất bản.'
  } finally {
    loading.value = false
  }
}

onMounted(load)
</script>

<template>
  <div class="min-h-screen bg-white text-slate-900">
    <div v-if="loading" class="flex items-center justify-center gap-2 py-24 text-sm text-slate-500">
      <Loader2 class="h-5 w-5 animate-spin text-emerald-600" />
      <span>Đang tải cửa hàng...</span>
    </div>
    <div v-else-if="errorMsg || !site" class="mx-auto max-w-lg px-6 py-24 text-center">
      <p class="text-sm font-semibold text-rose-600">{{ errorMsg }}</p>
    </div>
    <template v-else>
      <StoreRenderer
        :site-title="site.site_title"
        :primary-color="site.primary_color"
        :blocks="site.blocks"
        :products="site.products"
        :logo-url="site.logo_url"
        :site-slug="site.slug"
      />
      <CartDrawer :slug="site.slug" :accent="site.primary_color" />
    </template>
  </div>
</template>
