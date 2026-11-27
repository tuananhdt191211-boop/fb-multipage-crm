<script setup lang="ts">
// Khung chung cho các trang con của web công khai: dải thông báo + đầu trang, nội dung, chân trang, giỏ hàng
import { computed } from 'vue'
import StoreRenderer from './StoreRenderer.vue'
import CartDrawer from './CartDrawer.vue'
import { shellBlocks } from '../../lib/publicSite'
import type { PublicSiteData } from '../../types/store'

const props = defineProps<{ site: PublicSiteData }>()
const parts = computed(() => shellBlocks(props.site.blocks))
</script>

<template>
  <div class="flex min-h-screen flex-col bg-white text-slate-800">
    <StoreRenderer
      v-if="parts.top.length"
      :site-title="site.site_title"
      :primary-color="site.primary_color"
      :logo-url="site.logo_url"
      :blocks="parts.top"
      :products="site.products"
      :site-slug="site.slug"
    />
    <div class="flex-1">
      <slot />
    </div>
    <StoreRenderer
      v-if="parts.bottom.length"
      :site-title="site.site_title"
      :primary-color="site.primary_color"
      :logo-url="site.logo_url"
      :blocks="parts.bottom"
      :products="site.products"
      :site-slug="site.slug"
    />
    <CartDrawer :slug="site.slug" :accent="site.primary_color" />
  </div>
</template>
