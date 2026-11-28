<script setup lang="ts">
import { currentSlug, storePath } from '../lib/storeHost'
import { computed, onMounted, ref, watch } from 'vue'
import { track } from '../lib/analytics'
import { useRoute, useRouter } from 'vue-router'
import { ChevronRight, Loader2, Minus, Plus, ShieldCheck, ShoppingBag, Truck } from 'lucide-vue-next'
import PublicShell from '../components/store/PublicShell.vue'
import ProductGallery from '../components/store/ProductGallery.vue'
import { loadPublicProduct, loadPublicSite, localizeSite } from '../lib/publicSite'
import { localizeProducts, money, priceLabel, t } from '../lib/i18n'
import { toRichHtml } from '../lib/richHtml'
import { useCart } from '../lib/cart'
import { applyPublicSeo } from '../lib/seoHead'
import { safeImg } from '../lib/safeUrl'
import { findVariant, pickValue, valueState } from '../lib/variants'
import type { PublicProduct, PublicSiteData } from '../types/store'

const route = useRoute()
const router = useRouter()
const slug = computed(() => currentSlug(route.params.slug))
const rawSite = ref<PublicSiteData | null>(null)
const site = computed(() => (rawSite.value ? localizeSite(rawSite.value) : null))
const loading = ref(true)
const errorMsg = ref('')
const qty = ref(1)

// Chi tiết (mô tả đầy đủ, ảnh / video) lấy riêng; danh sách của trang chủ chỉ có bản tóm tắt
const detail = ref<PublicProduct | null>(null)
const product = computed<PublicProduct | null>(() => {
  const base = site.value?.products.find((p) => String(p.id) === String(route.params.pid)) || null
  if (!detail.value || (base && base.id !== detail.value.id)) return base
  const full = localizeProducts([detail.value])[0]!
  return base ? { ...base, ...full, name: full.name || base.name } : null
})
const descriptionHtml = computed(() => toRichHtml(product.value?.description || ''))

// Biến thể (Màu / Size...): luôn có một biến thể đang chọn; trạng thái lưu theo id nên đổi ngôn ngữ không làm mất lựa chọn
const options = computed(() => product.value?.options || [])
const variants = computed(() => product.value?.variants || [])
const hasVariants = computed(() => options.value.length > 0 && variants.value.length > 0)
const variantId = ref(0)
const variant = computed(() => (hasVariants.value ? variants.value.find((v) => v.id === variantId.value) : undefined))
const selection = computed(() => variant.value?.option_values || [])
watch(
  variants,
  (vs) => {
    if (vs.length && !vs.some((v) => v.id === variantId.value)) variantId.value = (vs.find((v) => v.in_stock) || vs[0])!.id
  },
  { immediate: true }
)
function choose(p: number, val: string) {
  const next = findVariant(variants.value, pickValue(variants.value, selection.value, p, val))
  if (next) variantId.value = next.id
}
const unitPrice = computed(() => variant.value?.price ?? product.value?.price ?? 0)
const unitCompare = computed(() => (variant.value ? variant.value.compare_price : product.value?.compare_price) || 0)
const inStock = computed(() => (variant.value ? variant.value.in_stock : !!product.value?.in_stock))
const skuShown = computed(() => variant.value?.sku || product.value?.sku || '')
const discount = computed(() => (unitCompare.value > unitPrice.value ? Math.round((1 - unitPrice.value / unitCompare.value) * 100) : 0))
const related = computed(() => (site.value?.products || []).filter((p) => p.id !== product.value?.id).slice(0, 4))
const accent = computed(() => site.value?.primary_color || '#059669')

async function load() {
  loading.value = true
  try {
    rawSite.value = await loadPublicSite({ slug: slug.value })
    await loadDetail()
    if (product.value) document.title = `${product.value.name} – ${rawSite.value.site_title}`
    void applyPublicSeo(rawSite.value.slug, '/p/' + String(route.params.pid))
  } catch (e: any) {
    errorMsg.value = e?.response?.data?.error || t('shop_not_found')
  } finally {
    loading.value = false
  }
}

async function loadDetail() {
  detail.value = null
  try {
    detail.value = await loadPublicProduct(slug.value, String(route.params.pid))
  } catch {
    detail.value = null // vẫn xem được bản tóm tắt từ danh sách
  }
  // Thống kê: khách xem sản phẩm (chỉ khi sản phẩm có thật)
  if (product.value) track('product_view', { pid: product.value.id, l: detail.value?.name || product.value.name })
}

function addToCart(go = false) {
  if (!product.value || !inStock.value) return
  if (hasVariants.value && !variant.value) return
  const cart = useCart(slug.value)
  // Giỏ lưu chữ GỐC của tên và phân loại (chưa dịch): khi hiển thị mới dịch theo ngôn ngữ khách đang chọn
  const raw = detail.value && detail.value.id === product.value.id ? detail.value : product.value
  const rawVariant = variant.value ? (raw.variants?.find((v) => v.id === variant.value!.id) ?? variant.value) : undefined
  cart.add(raw, qty.value, rawVariant)
  if (go) {
    cart.state.open = false
    router.push({ name: 'public-checkout', params: { slug: slug.value } })
  }
}

watch(() => route.params.pid, async () => {
  qty.value = 1
  variantId.value = 0
  window.scrollTo({ top: 0 })
  await loadDetail()
  if (product.value && rawSite.value) document.title = `${product.value.name} – ${rawSite.value.site_title}`
  if (rawSite.value) void applyPublicSeo(rawSite.value.slug, '/p/' + String(route.params.pid))
})
onMounted(load)
</script>

<template>
  <div v-if="loading" class="flex min-h-screen items-center justify-center gap-2 text-sm text-slate-500"><Loader2 class="h-5 w-5 animate-spin" /> {{ t('loading') }}</div>
  <div v-else-if="!site" class="py-24 text-center text-sm font-semibold text-rose-600">{{ errorMsg }}</div>
  <PublicShell v-else :site="site">
    <div class="mx-auto max-w-6xl px-5 py-8 sm:px-8 sm:py-12">
      <nav class="mb-6 flex items-center gap-1 text-sm text-slate-500">
        <router-link :to="storePath(site.slug)" class="hover:underline">{{ t('home') }}</router-link>
        <ChevronRight class="h-4 w-4" />
        <span class="truncate text-slate-800">{{ product?.name || t('product') }}</span>
      </nav>

      <div v-if="!product" class="py-20 text-center">
        <p class="font-semibold text-slate-700">{{ t('product_missing') }}</p>
        <router-link :to="storePath(site.slug)" class="mt-4 inline-block rounded-full px-5 py-2.5 text-sm font-bold text-white" :style="{ backgroundColor: accent }">{{ t('back_home') }}</router-link>
      </div>

      <div v-else class="grid grid-cols-1 gap-10 md:grid-cols-2 md:gap-14">
        <ProductGallery :media="product.media" :fallback="product.image_url" :name="product.name" :accent="accent" :focus="variant?.image_url">
          <span v-if="discount" class="absolute left-4 top-4 rounded-full bg-rose-500 px-3 py-1 text-sm font-bold text-white">-{{ discount }}%</span>
        </ProductGallery>

        <div class="space-y-5">
          <h1 class="text-2xl font-extrabold leading-tight tracking-tight text-slate-900 sm:text-3xl">{{ product.name }}</h1>
          <div class="flex flex-wrap items-baseline gap-3" data-testid="price">
            <span class="text-3xl font-extrabold" :style="{ color: accent }">{{ money(unitPrice) }}</span>
            <span v-if="discount" class="text-lg text-slate-400 line-through">{{ money(unitCompare) }}</span>
            <span v-if="discount" class="rounded-full bg-rose-50 px-2.5 py-0.5 text-sm font-bold text-rose-600">{{ t('save_amount', { amount: money(unitCompare - unitPrice) }) }}</span>
          </div>

          <!-- Phân loại (Màu, Size...) -->
          <div v-if="hasVariants" class="space-y-4" data-testid="options">
            <div v-for="(o, p) in options" :key="o.name + p" class="space-y-2">
              <div class="text-sm font-semibold text-slate-600">{{ o.name }}: <span class="text-slate-900">{{ selection[p] }}</span></div>
              <div class="flex flex-wrap gap-2">
                <button
                  v-for="val in o.values"
                  :key="val"
                  type="button"
                  class="rounded-xl border-2 px-3.5 py-2 text-sm font-semibold transition"
                  :class="[
                    selection[p] === val ? '' : 'border-slate-200 text-slate-700 hover:border-slate-400',
                    valueState(variants, selection, p, val) === 'soldout' ? 'line-through opacity-50' : '',
                    valueState(variants, selection, p, val) === 'missing' ? 'border-dashed opacity-40' : ''
                  ]"
                  :style="selection[p] === val ? { borderColor: accent, color: accent, backgroundColor: accent + '14' } : undefined"
                  :aria-pressed="selection[p] === val"
                  :title="valueState(variants, selection, p, val) === 'soldout' ? t('out_of_stock') : undefined"
                  :data-option="p"
                  :data-value="val"
                  @click="choose(p, val)"
                >
                  {{ val }}
                </button>
              </div>
            </div>
          </div>

          <p class="text-sm font-semibold" :class="inStock ? 'text-emerald-600' : 'text-rose-600'" data-testid="stock">{{ inStock ? t('in_stock') : t('sold_out') }}</p>

          <div v-if="inStock" class="space-y-3 pt-1">
            <div class="flex items-center gap-3">
              <span class="text-sm font-semibold text-slate-600">{{ t('quantity') }}</span>
              <div class="flex items-center rounded-full border border-slate-300">
                <button class="p-2.5" :aria-label="t('decrease')" @click="qty = Math.max(1, qty - 1)"><Minus class="h-4 w-4" /></button>
                <input v-model.number="qty" type="number" min="1" max="99" class="w-12 bg-transparent text-center font-semibold outline-none [appearance:textfield] [&::-webkit-inner-spin-button]:appearance-none" />
                <button class="p-2.5" :aria-label="t('increase')" @click="qty = Math.min(99, qty + 1)"><Plus class="h-4 w-4" /></button>
              </div>
            </div>
            <div class="flex flex-col gap-3 sm:flex-row">
              <button class="flex flex-1 items-center justify-center gap-2 rounded-full border-2 py-3.5 text-sm font-bold transition hover:bg-slate-50" :style="{ borderColor: accent, color: accent }" data-testid="add-to-cart" @click="addToCart(false)">
                <ShoppingBag class="h-4 w-4" /> {{ t('add_to_cart') }}
              </button>
              <button class="flex-1 rounded-full py-3.5 text-sm font-bold text-white shadow-lg transition hover:brightness-110" :style="{ backgroundColor: accent }" @click="addToCart(true)">{{ t('buy_now') }}</button>
            </div>
          </div>

          <div class="grid grid-cols-1 gap-2 rounded-2xl bg-slate-50 p-4 text-sm text-slate-600 sm:grid-cols-2">
            <span class="flex items-center gap-2"><Truck class="h-4 w-4" :style="{ color: accent }" /> {{ t('ship_nationwide') }}</span>
            <span class="flex items-center gap-2"><ShieldCheck class="h-4 w-4" :style="{ color: accent }" /> {{ t('check_on_delivery') }}</span>
          </div>

          <p v-if="skuShown" class="text-xs text-slate-400">SKU: {{ skuShown }}</p>

          <div v-if="descriptionHtml" class="border-t border-slate-100 pt-5">
            <h2 class="mb-2 text-base font-bold text-slate-900">{{ t('description') }}</h2>
            <div class="pp-prose" v-html="descriptionHtml" />
          </div>
        </div>
      </div>

      <section v-if="product && related.length" class="mt-16 space-y-6">
        <h2 class="text-xl font-extrabold text-slate-900">{{ t('related') }}</h2>
        <div class="grid grid-cols-2 gap-3 md:grid-cols-4 md:gap-5">
          <router-link v-for="p in related" :key="p.id" :to="storePath(site.slug, `/p/${p.id}`)" class="group overflow-hidden rounded-2xl border border-slate-200/80 bg-white transition hover:shadow-lg">
            <div class="aspect-square overflow-hidden bg-slate-100">
              <img v-if="safeImg(p.image_url)" :src="safeImg(p.image_url)" :alt="p.name" loading="lazy" class="h-full w-full object-cover transition group-hover:scale-105" />
            </div>
            <div class="p-3">
              <div class="line-clamp-2 text-sm font-semibold text-slate-800">{{ p.name }}</div>
              <div class="mt-1 font-bold" :style="{ color: accent }">{{ priceLabel(p) }}</div>
            </div>
          </router-link>
        </div>
      </section>
    </div>
  </PublicShell>
</template>
