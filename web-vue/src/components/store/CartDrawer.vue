<script setup lang="ts">
// Giỏ hàng trượt ra từ bên phải
import { useRouter } from 'vue-router'
import { Minus, Plus, ShoppingBag, Trash2, X } from 'lucide-vue-next'
import { useCart } from '../../lib/cart'
import { formatVnd } from '../../lib/format'
import { safeImg } from '../../lib/safeUrl'

const props = defineProps<{ slug: string; accent: string }>()
const router = useRouter()
const cart = useCart(props.slug)

function checkout() {
  cart.state.open = false
  router.push({ name: 'public-checkout', params: { slug: props.slug } })
}
</script>

<template>
  <Teleport to="body">
    <Transition enter-from-class="opacity-0" leave-to-class="opacity-0" enter-active-class="transition-opacity" leave-active-class="transition-opacity">
      <div v-if="cart.state.open" class="fixed inset-0 z-[90] bg-black/40" @click="cart.state.open = false" />
    </Transition>
    <Transition enter-from-class="translate-x-full" leave-to-class="translate-x-full" enter-active-class="transition-transform duration-300" leave-active-class="transition-transform duration-200">
      <aside v-if="cart.state.open" class="fixed inset-y-0 right-0 z-[91] flex w-full max-w-md flex-col bg-white text-slate-800 shadow-2xl">
        <div class="flex items-center justify-between border-b border-slate-100 px-5 py-4">
          <h2 class="text-lg font-extrabold">Giỏ hàng ({{ cart.count.value }})</h2>
          <button class="rounded-lg p-1.5 hover:bg-slate-100" aria-label="Đóng" @click="cart.state.open = false"><X class="h-5 w-5" /></button>
        </div>

        <div v-if="!cart.state.lines.length" class="flex flex-1 flex-col items-center justify-center gap-3 p-8 text-center text-slate-500">
          <ShoppingBag class="h-12 w-12 text-slate-300" />
          <p class="font-semibold">Giỏ hàng đang trống</p>
          <button class="rounded-full px-5 py-2.5 text-sm font-bold text-white" :style="{ backgroundColor: accent }" @click="cart.state.open = false">Tiếp tục mua sắm</button>
        </div>

        <template v-else>
          <ul class="flex-1 divide-y divide-slate-100 overflow-y-auto px-5">
            <li v-for="l in cart.state.lines" :key="l.product_id" class="flex gap-3 py-4">
              <div class="h-20 w-20 shrink-0 overflow-hidden rounded-xl bg-slate-100">
                <img v-if="safeImg(l.image_url)" :src="safeImg(l.image_url)" :alt="l.name" class="h-full w-full object-cover" />
              </div>
              <div class="flex min-w-0 flex-1 flex-col">
                <div class="line-clamp-2 text-sm font-semibold">{{ l.name }}</div>
                <div class="mt-0.5 text-sm font-bold" :style="{ color: accent }">{{ formatVnd(l.price) }}</div>
                <div class="mt-auto flex items-center justify-between pt-2">
                  <div class="flex items-center rounded-full border border-slate-200">
                    <button class="p-1.5 hover:text-slate-900" aria-label="Giảm" @click="cart.setQty(l.product_id, l.qty - 1)"><Minus class="h-3.5 w-3.5" /></button>
                    <span class="w-8 text-center text-sm font-semibold">{{ l.qty }}</span>
                    <button class="p-1.5 hover:text-slate-900" aria-label="Tăng" @click="cart.setQty(l.product_id, l.qty + 1)"><Plus class="h-3.5 w-3.5" /></button>
                  </div>
                  <button class="rounded-lg p-1.5 text-slate-400 hover:bg-rose-50 hover:text-rose-600" aria-label="Xoá" @click="cart.remove(l.product_id)"><Trash2 class="h-4 w-4" /></button>
                </div>
              </div>
            </li>
          </ul>
          <div class="space-y-3 border-t border-slate-100 p-5">
            <div class="flex items-center justify-between text-base">
              <span class="text-slate-600">Tạm tính</span>
              <span class="text-lg font-extrabold">{{ formatVnd(cart.subtotal.value) }}</span>
            </div>
            <p class="text-xs text-slate-400">Phí vận chuyển tính ở bước thanh toán.</p>
            <button class="w-full rounded-full py-3.5 text-sm font-bold text-white shadow-lg transition hover:brightness-110" :style="{ backgroundColor: accent }" @click="checkout">
              Thanh toán
            </button>
          </div>
        </template>
      </aside>
    </Transition>
  </Teleport>
</template>
