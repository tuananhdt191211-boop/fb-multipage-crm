<script setup lang="ts">
// Khung chung cho các trang con của web công khai: dải thông báo + đầu trang, nội dung, chân trang, giỏ hàng
import { computed, watchEffect } from 'vue'
import StoreRenderer from './StoreRenderer.vue'
import CartDrawer from './CartDrawer.vue'
import CustomerAuthDialog from './CustomerAuthDialog.vue'
import { shellBlocks } from '../../lib/publicSite'
import { storePath } from '../../lib/storeHost'
import { themeCss, usedFonts } from '../../lib/theme'
import { ensureFonts } from '../../lib/fonts'
import type { PublicSiteData } from '../../types/store'

// hideTop / hideBottom: landing page ẩn dải thông báo + đầu trang / chân trang
const props = defineProps<{ site: PublicSiteData; hideTop?: boolean; hideBottom?: boolean }>()
const parts = computed(() => shellBlocks(props.site.shared?.length ? props.site.shared : props.site.blocks))
// Liên kết neo (#products) của đầu / chân trang đưa về trang chủ, vì trang con không có các phần đó
const homeBase = computed(() => storePath(props.site.slug, ''))
// Nội dung trang con (sản phẩm, thanh toán...) cũng theo kiểu chữ toàn trang
const themeStyle = computed(() => themeCss(props.site.theme))
watchEffect(() => ensureFonts(usedFonts(props.site.theme, [])))
</script>

<template>
  <div class="flex min-h-screen flex-col bg-white text-slate-800">
    <StoreRenderer
      v-if="!hideTop && parts.top.length"
      :site-title="site.site_title"
      :primary-color="site.primary_color"
      :theme="site.theme"
      :logo-url="site.logo_url"
      :blocks="parts.top"
      :products="site.products"
      :site-slug="site.slug"
      :anchor-base="homeBase"
      :menus="site.menus"
    />
    <div class="pp-store @container flex-1">
      <component :is="'style'" v-if="themeStyle">{{ themeStyle }}</component>
      <slot />
    </div>
    <StoreRenderer
      v-if="!hideBottom && parts.bottom.length"
      :site-title="site.site_title"
      :primary-color="site.primary_color"
      :theme="site.theme"
      :logo-url="site.logo_url"
      :blocks="parts.bottom"
      :products="site.products"
      :site-slug="site.slug"
      :anchor-base="homeBase"
      :menus="site.menus"
    />
    <CartDrawer :slug="site.slug" :accent="site.primary_color" :products="site.products" />
    <CustomerAuthDialog :slug="site.slug" :accent="site.primary_color" :site-title="site.site_title" />
  </div>
</template>
