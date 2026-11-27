// Giỏ hàng của khách trên web công khai — lưu trong trình duyệt theo từng website (slug)
import { computed, reactive, watch } from 'vue'
import type { PublicProduct } from '../types/store'

export interface CartLine {
  product_id: number
  name: string
  price: number
  image_url: string
  qty: number
}

const state = reactive({
  slug: '',
  lines: [] as CartLine[],
  open: false
})

function storageKey(slug: string) {
  return `pp_cart_${slug}`
}

function load(slug: string) {
  state.slug = slug
  state.lines = []
  try {
    const raw = localStorage.getItem(storageKey(slug))
    const parsed = raw ? JSON.parse(raw) : []
    if (Array.isArray(parsed)) {
      state.lines = parsed
        .filter((l) => l && Number(l.product_id) > 0 && Number(l.qty) > 0)
        .map((l) => ({ ...l, qty: Math.min(99, Math.floor(Number(l.qty))) }))
    }
  } catch {
    /* trình duyệt chặn lưu trữ: giỏ chỉ sống trong phiên này */
  }
}

watch(
  () => state.lines,
  (lines) => {
    if (!state.slug) return
    try {
      localStorage.setItem(storageKey(state.slug), JSON.stringify(lines))
    } catch {
      /* bỏ qua */
    }
  },
  { deep: true }
)

const count = computed(() => state.lines.reduce((s, l) => s + l.qty, 0))
const subtotal = computed(() => state.lines.reduce((s, l) => s + l.price * l.qty, 0))

function add(p: PublicProduct, qty = 1) {
  const line = state.lines.find((l) => l.product_id === p.id)
  if (line) line.qty = Math.min(99, line.qty + qty)
  else state.lines.push({ product_id: p.id, name: p.name, price: p.price, image_url: p.image_url, qty: Math.min(99, qty) })
  state.open = true
}

function setQty(productId: number, qty: number) {
  const line = state.lines.find((l) => l.product_id === productId)
  if (!line) return
  if (qty <= 0) remove(productId)
  else line.qty = Math.min(99, Math.floor(qty))
}

function remove(productId: number) {
  state.lines = state.lines.filter((l) => l.product_id !== productId)
}

function clear() {
  state.lines = []
}

// Cập nhật giá/tên theo dữ liệu mới nhất của shop, bỏ sản phẩm đã ngừng bán
function syncWith(products: PublicProduct[]) {
  const byId = new Map(products.map((p) => [p.id, p]))
  state.lines = state.lines
    .filter((l) => byId.get(l.product_id)?.in_stock)
    .map((l) => {
      const p = byId.get(l.product_id)!
      return { ...l, name: p.name, price: p.price, image_url: p.image_url }
    })
}

export function useCart(slug?: string) {
  if (slug && state.slug !== slug) load(slug)
  return { state, count, subtotal, add, setQty, remove, clear, syncWith }
}
