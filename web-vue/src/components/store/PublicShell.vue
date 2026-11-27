<script setup lang="ts">
// Khung chung cho các trang con của web công khai: dải thông báo + đầu trang, nội dung, chân trang, giỏ hàng
import { computed, watchEffect } from 'vue'
import StoreRenderer from './StoreRenderer.vue'
import CartDrawer from './CartDrawer.vue'
import { shellBlocks } from '../../lib/publicSite'
import { themeCss, usedFonts } from '../../lib/theme'
import { ensureFonts } from '../../lib/fonts'
import type { PublicSiteData } from '../../types/store'

const props = defineProps<{ site: PublicSiteData }>()
const parts = computed(() => shellBlocks(props.site.blocks))
// Nội dung trang con (sản phẩm, thanh toán...) cũng theo kiểu chữ toàn trang
const themeStyle = computed(() => themeCss(props.site.theme))
watchEffect(() => ensureFonts(usedFonts(props.site.theme, [])))
</script>

<template>
  <div class="flex min-h-screen flex-col bg-white text-slate-800">
    <StoreRenderer
      v-if="parts.top.length"
      :site-title="site.site_title"
      :primary-color="site.primary_color"
      :theme="site.theme"
      :logo-url="site.logo_url"
      :blocks="parts.top"
      :products="site.products"
      :site-slug="site.slug"
    />
    <div class="pp-store @container flex-1">
      <component :is="'style'" v-if="themeStyle">{{ themeStyle }}</component>
      <slot />
    </div>
    <StoreRenderer
      v-if="parts.bottom.length"
      :site-title="site.site_title"
      :primary-color="site.primary_color"
      :theme="site.theme"
      :logo-url="site.logo_url"
      :blocks="parts.bottom"
      :products="site.products"
      :site-slug="site.slug"
    />
    <CartDrawer :slug="site.slug" :accent="site.primary_color" />
  </div>
</template>
