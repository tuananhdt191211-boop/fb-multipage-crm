<script setup lang="ts">
import { computed, nextTick, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { Loader2 } from 'lucide-vue-next'
import StoreRenderer from '../components/store/StoreRenderer.vue'
import CartDrawer from '../components/store/CartDrawer.vue'
import CustomerAuthDialog from '../components/store/CustomerAuthDialog.vue'
import { loadPublicSite, localizeSite } from '../lib/publicSite'
import { t } from '../lib/i18n'
import { useCart } from '../lib/cart'
import type { PublicSiteData } from '../types/store'

const route = useRoute()
const loading = ref(true)
const errorMsg = ref('')
const rawSite = ref<PublicSiteData | null>(null)
// Bản đã dịch theo ngôn ngữ khách chọn (tự cập nhật khi đổi ngôn ngữ)
const site = computed(() => (rawSite.value ? localizeSite(rawSite.value) : null))

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
    rawSite.value = await loadPublicSite(slug ? { slug } : { domain })
    document.title = rawSite.value.site_title
    useCart(rawSite.value.slug).syncWith(rawSite.value.products)
  } catch (err: any) {
    errorMsg.value = err?.response?.data?.error || t('store_not_found')
  } finally {
    loading.value = false
    scrollToHash()
  }
}

// Từ trang con bấm liên kết neo (vd. /s/shop#products) về trang chủ: nội dung hiện ra sau khi tải xong nên cuộn thủ công tới đúng phần
async function scrollToHash() {
  const h = window.location.hash
  if (h.length < 2) return
  await nextTick()
  let id = h.slice(1)
  try {
    id = decodeURIComponent(id)
  } catch {
    /* giữ nguyên */
  }
  setTimeout(() => document.getElementById(id)?.scrollIntoView(), 60)
}

onMounted(load)
</script>

<template>
  <div class="min-h-screen bg-white text-slate-900">
    <div v-if="loading" class="flex items-center justify-center gap-2 py-24 text-sm text-slate-500">
      <Loader2 class="h-5 w-5 animate-spin text-emerald-600" />
      <span>{{ t('loading_store') }}</span>
    </div>
    <div v-else-if="errorMsg || !site" class="mx-auto max-w-lg px-6 py-24 text-center">
      <p class="text-sm font-semibold text-rose-600">{{ errorMsg }}</p>
    </div>
    <template v-else>
      <StoreRenderer
        :site-title="site.site_title"
        :primary-color="site.primary_color"
        :theme="site.theme"
        :blocks="site.blocks"
        :products="site.products"
        :logo-url="site.logo_url"
        :site-slug="site.slug"
      />
      <CartDrawer :slug="site.slug" :accent="site.primary_color" :products="site.products" />
      <CustomerAuthDialog :slug="site.slug" :accent="site.primary_color" :site-title="site.site_title" />
    </template>
  </div>
</template>
