<script setup lang="ts">
import { currentSlug, storePath } from '../lib/storeHost'
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ChevronRight, Loader2, Minus, Plus, ShieldCheck, ShoppingBag, Truck } from 'lucide-vue-next'
import PublicShell from '../components/store/PublicShell.vue'
import { loadPublicSite } from '../lib/publicSite'
import { useCart } from '../lib/cart'
import { formatVnd } from '../lib/format'
import { safeImg } from '../lib/safeUrl'
import type { PublicSiteData } from '../types/store'

const route = useRoute()
const router = useRouter()
const slug = computed(() => currentSlug(route.params.slug))
const site = ref<PublicSiteData | null>(null)
const loading = ref(true)
const errorMsg = ref('')
const qty = ref(1)

const product = computed(() => site.value?.products.find((p) => String(p.id) === String(route.params.pid)) || null)
const discount = computed(() => {
  const p = product.value
  return p && p.compare_price > p.price ? Math.round((1 - p.price / p.compare_price) * 100) : 0
})
const related = computed(() => (site.value?.products || []).filter((p) => p.id !== product.value?.id).slice(0, 4))
const accent = computed(() => site.value?.primary_color || '#059669')

async function load() {
  loading.value = true
  try {
    site.value = await loadPublicSite({ slug: slug.value })
    if (product.value) document.title = `${product.value.name} – ${site.value.site_title}`
  } catch (e: any) {
    errorMsg.value = e?.response?.data?.error || 'Không tìm thấy cửa hàng.'
  } finally {
    loading.value = false
  }
}

function addToCart(go = false) {
  if (!product.value || !product.value.in_stock) return
  const cart = useCart(slug.value)
  cart.add(product.value, qty.value)
  if (go) {
    cart.state.open = false
    router.push({ name: 'public-checkout', params: { slug: slug.value } })
  }
}

watch(() => route.params.pid, () => {
  qty.value = 1
  window.scrollTo({ top: 0 })
})
onMounted(load)
</script>

<template>
  <div v-if="loading" class="flex min-h-screen items-center justify-center gap-2 text-sm text-slate-500"><Loader2 class="h-5 w-5 animate-spin" /> Đang tải...</div>
  <div v-else-if="!site" class="py-24 text-center text-sm font-semibold text-rose-600">{{ errorMsg }}</div>
  <PublicShell v-else :site="site">
    <div class="mx-auto max-w-6xl px-5 py-8 sm:px-8 sm:py-12">
      <nav class="mb-6 flex items-center gap-1 text-sm text-slate-500">
        <router-link :to="storePath(site.slug)" class="hover:underline">Trang chủ</router-link>
        <ChevronRight class="h-4 w-4" />
        <span class="truncate text-slate-800">{{ product?.name || 'Sản phẩm' }}</span>
      </nav>

      <div v-if="!product" class="py-20 text-center">
        <p class="font-semibold text-slate-700">Sản phẩm không tồn tại hoặc đã ngừng bán.</p>
        <router-link :to="storePath(site.slug)" class="mt-4 inline-block rounded-full px-5 py-2.5 text-sm font-bold text-white" :style="{ backgroundColor: accent }">Về trang chủ</router-link>
      </div>

      <div v-else class="grid grid-cols-1 gap-10 md:grid-cols-2 md:gap-14">
        <div class="relative aspect-square overflow-hidden rounded-3xl bg-slate-100">
          <img v-if="safeImg(product.image_url)" :src="safeImg(product.image_url)" :alt="product.name" class="h-full w-full object-cover" />
          <span v-if="discount" class="absolute left-4 top-4 rounded-full bg-rose-500 px-3 py-1 text-sm font-bold text-white">-{{ discount }}%</span>
        </div>

        <div class="space-y-5">
          <h1 class="text-2xl font-extrabold leading-tight tracking-tight text-slate-900 sm:text-3xl">{{ product.name }}</h1>
          <div class="flex flex-wrap items-baseline gap-3">
            <span class="text-3xl font-extrabold" :style="{ color: accent }">{{ formatVnd(product.price) }}</span>
            <span v-if="discount" class="text-lg text-slate-400 line-through">{{ formatVnd(product.compare_price) }}</span>
            <span v-if="discount" class="rounded-full bg-rose-50 px-2.5 py-0.5 text-sm font-bold text-rose-600">Tiết kiệm {{ formatVnd(product.compare_price - product.price) }}</span>
          </div>
          <p class="text-sm font-semibold" :class="product.in_stock ? 'text-emerald-600' : 'text-rose-600'">{{ product.in_stock ? '● Còn hàng' : '● Tạm hết hàng' }}</p>

          <div v-if="product.in_stock" class="space-y-3 pt-1">
            <div class="flex items-center gap-3">
              <span class="text-sm font-semibold text-slate-600">Số lượng</span>
              <div class="flex items-center rounded-full border border-slate-300">
                <button class="p-2.5" aria-label="Giảm" @click="qty = Math.max(1, qty - 1)"><Minus class="h-4 w-4" /></button>
                <input v-model.number="qty" type="number" min="1" max="99" class="w-12 bg-transparent text-center font-semibold outline-none [appearance:textfield] [&::-webkit-inner-spin-button]:appearance-none" />
                <button class="p-2.5" aria-label="Tăng" @click="qty = Math.min(99, qty + 1)"><Plus class="h-4 w-4" /></button>
              </div>
            </div>
            <div class="flex flex-col gap-3 sm:flex-row">
              <button class="flex flex-1 items-center justify-center gap-2 rounded-full border-2 py-3.5 text-sm font-bold transition hover:bg-slate-50" :style="{ borderColor: accent, color: accent }" @click="addToCart(false)">
                <ShoppingBag class="h-4 w-4" /> Thêm vào giỏ
              </button>
              <button class="flex-1 rounded-full py-3.5 text-sm font-bold text-white shadow-lg transition hover:brightness-110" :style="{ backgroundColor: accent }" @click="addToCart(true)">Mua ngay</button>
            </div>
          </div>

          <div class="grid grid-cols-1 gap-2 rounded-2xl bg-slate-50 p-4 text-sm text-slate-600 sm:grid-cols-2">
            <span class="flex items-center gap-2"><Truck class="h-4 w-4" :style="{ color: accent }" /> Giao hàng toàn quốc</span>
            <span class="flex items-center gap-2"><ShieldCheck class="h-4 w-4" :style="{ color: accent }" /> Kiểm tra hàng khi nhận</span>
          </div>

          <div v-if="product.description" class="border-t border-slate-100 pt-5">
            <h2 class="mb-2 text-base font-bold text-slate-900">Mô tả sản phẩm</h2>
            <p class="whitespace-pre-line text-[15px] leading-relaxed text-slate-600">{{ product.description }}</p>
          </div>
        </div>
      </div>

      <section v-if="product && related.length" class="mt-16 space-y-6">
        <h2 class="text-xl font-extrabold text-slate-900">Có thể bạn cũng thích</h2>
        <div class="grid grid-cols-2 gap-3 md:grid-cols-4 md:gap-5">
          <router-link v-for="p in related" :key="p.id" :to="storePath(site.slug, `/p/${p.id}`)" class="group overflow-hidden rounded-2xl border border-slate-200/80 bg-white transition hover:shadow-lg">
            <div class="aspect-square overflow-hidden bg-slate-100">
              <img v-if="safeImg(p.image_url)" :src="safeImg(p.image_url)" :alt="p.name" loading="lazy" class="h-full w-full object-cover transition group-hover:scale-105" />
            </div>
            <div class="p-3">
              <div class="line-clamp-2 text-sm font-semibold text-slate-800">{{ p.name }}</div>
              <div class="mt-1 font-bold" :style="{ color: accent }">{{ formatVnd(p.price) }}</div>
            </div>
          </router-link>
        </div>
      </section>
    </div>
  </PublicShell>
</template>
