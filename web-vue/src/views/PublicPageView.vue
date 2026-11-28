<script setup lang="ts">
// Một trang của website (giới thiệu, liên hệ, chính sách...) — /s/:slug/pages/:page, hoặc /pages/:page trên tên miền riêng.
// Đầu / chân trang dùng chung lấy từ trang chủ (PublicShell); ở giữa là các section riêng của trang.
import { computed, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { Loader2 } from 'lucide-vue-next'
import PublicShell from '../components/store/PublicShell.vue'
import StoreRenderer from '../components/store/StoreRenderer.vue'
import { currentSlug, storePath } from '../lib/storeHost'
import { loadPublicPage, loadPublicSite, localizeSite, type PublicPageData } from '../lib/publicSite'
import { localizeBlocks, t } from '../lib/i18n'
import { useCart } from '../lib/cart'
import { applyPublicSeo } from '../lib/seoHead'
import type { PublicSiteData } from '../types/store'

const route = useRoute()
const slug = computed(() => currentSlug(route.params.slug))
const rawSite = ref<PublicSiteData | null>(null)
const site = computed(() => (rawSite.value ? localizeSite(rawSite.value) : null))
const page = ref<PublicPageData | null>(null)
// Nội dung trang theo ngôn ngữ khách chọn (tự cập nhật khi đổi ngôn ngữ)
const blocks = computed(() => (page.value ? localizeBlocks(page.value.blocks.filter((b) => b.enabled !== false)) : []))
const loading = ref(true)
const errorMsg = ref('')

async function load() {
  loading.value = true
  errorMsg.value = ''
  page.value = null
  try {
    rawSite.value = await loadPublicSite({ slug: slug.value })
    useCart(rawSite.value.slug).syncWith(rawSite.value.products)
    page.value = await loadPublicPage(rawSite.value.slug, String(route.params.page || ''))
    document.title = `${page.value.title} – ${rawSite.value.site_title}`
    void applyPublicSeo(rawSite.value.slug, '/pages/' + page.value.slug)
    window.scrollTo({ top: 0 })
  } catch (e: any) {
    errorMsg.value = e?.response?.status === 404 ? t('page_not_found') : e?.response?.data?.error || t('shop_not_found')
    if (rawSite.value) void applyPublicSeo(rawSite.value.slug, '/pages/' + String(route.params.page || '')) // trang không có: noindex
  } finally {
    loading.value = false
  }
}

watch(() => [route.params.slug, route.params.page], load, { immediate: true })
</script>

<template>
  <div v-if="loading" class="flex min-h-screen items-center justify-center gap-2 text-sm text-slate-500"><Loader2 class="h-5 w-5 animate-spin" /> {{ t('loading') }}</div>
  <div v-else-if="!site" class="py-24 text-center text-sm font-semibold text-rose-600">{{ errorMsg }}</div>
  <PublicShell v-else :site="site" :hide-top="page?.settings.hide_header" :hide-bottom="page?.settings.hide_footer">
    <div v-if="!page" class="mx-auto max-w-2xl px-6 py-24 text-center" data-testid="page-missing">
      <p class="font-semibold text-slate-700">{{ errorMsg || t('page_not_found') }}</p>
      <router-link :to="storePath(site.slug)" class="mt-4 inline-block rounded-full px-5 py-2.5 text-sm font-bold text-white" :style="{ backgroundColor: site.primary_color }">{{ t('back_home') }}</router-link>
    </div>
    <StoreRenderer
      v-else
      data-testid="page-content"
      :site-title="site.site_title"
      :primary-color="site.primary_color"
      :theme="site.theme"
      :logo-url="site.logo_url"
      :blocks="blocks"
      :products="site.products"
      :site-slug="site.slug"
      :anchor-base="storePath(site.slug, '')"
    />
  </PublicShell>
</template>
