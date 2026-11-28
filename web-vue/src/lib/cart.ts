// Giỏ hàng của khách trên web công khai — lưu trong trình duyệt theo từng website (slug)
// Mỗi dòng là một (sản phẩm, biến thể): cùng sản phẩm nhưng khác phân loại là hai dòng riêng.
import { computed, reactive, watch } from 'vue'
import type { PublicProduct, PublicVariant } from '../types/store'

export interface CartLine {
  product_id: number
  variant_id?: number // 0 / không có = sản phẩm không có biến thể
  variant_title?: string // "Đen / L"
  sku?: string
  name: string
  price: number
  image_url: string
  qty: number
}

type LineRef = { product_id: number; variant_id?: number }

export const lineKey = (l: LineRef) => `${l.product_id}:${l.variant_id || 0}`

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
        .map((l) => ({ ...l, variant_id: Number(l.variant_id) > 0 ? Number(l.variant_id) : 0, qty: Math.min(99, Math.floor(Number(l.qty))) }))
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

// Thêm vào giỏ. Sản phẩm có biến thể phải truyền biến thể đã chọn (giá / ảnh lấy theo biến thể).
function add(p: PublicProduct, qty = 1, v?: PublicVariant) {
  const ref: LineRef = { product_id: p.id, variant_id: v?.id || 0 }
  const line = state.lines.find((l) => lineKey(l) === lineKey(ref))
  if (line) line.qty = Math.min(99, line.qty + qty)
  else {
    state.lines.push({
      product_id: p.id,
      variant_id: v?.id || 0,
      variant_title: v?.title || '',
      sku: v?.sku || '',
      name: p.name,
      price: v ? v.price : p.price,
      image_url: v?.image_url || p.image_url,
      qty: Math.min(99, qty)
    })
  }
  state.open = true
}

function setQty(ref: LineRef, qty: number) {
  const line = state.lines.find((l) => lineKey(l) === lineKey(ref))
  if (!line) return
  if (qty <= 0) remove(ref)
  else line.qty = Math.min(99, Math.floor(qty))
}

function remove(ref: LineRef) {
  const k = lineKey(ref)
  state.lines = state.lines.filter((l) => lineKey(l) !== k)
}

function clear() {
  state.lines = []
}

// Đối chiếu với danh sách sản phẩm của shop (trang chủ): cập nhật tên, giá, ảnh của sản phẩm thường; bỏ dòng của sản phẩm đã
// ngừng bán / hết hàng, và dòng không còn khớp (sản phẩm vừa chuyển sang có biến thể, hoặc biến thể đã bị gỡ).
// Giá của dòng có biến thể chỉ cập nhật được từ chi tiết sản phẩm (xem syncVariants). Trả về số dòng đã bị bỏ.
function syncWith(products: PublicProduct[]): number {
  const byId = new Map(products.map((p) => [p.id, p]))
  const before = state.lines.length
  state.lines = state.lines
    .filter((l) => {
      const p = byId.get(l.product_id)
      if (!p || !p.in_stock) return false
      const hasVariants = (p.variant_count || 0) > 0
      return hasVariants === !!l.variant_id
    })
    .map((l) => {
      const p = byId.get(l.product_id)!
      if (l.variant_id) return { ...l, name: p.name }
      return { ...l, name: p.name, price: p.price, image_url: p.image_url }
    })
  return before - state.lines.length
}

// Cập nhật các dòng có biến thể theo chi tiết sản phẩm (giá, ảnh, tên phân loại); bỏ dòng có biến thể đã hết hàng / bị gỡ.
// loader trả null khi sản phẩm không còn (404) và ném lỗi khi lỗi mạng — lỗi mạng thì giữ nguyên giỏ. Trả về số dòng đã bị bỏ.
async function syncVariants(loader: (productId: number) => Promise<PublicProduct | null>): Promise<number> {
  const pids = [...new Set(state.lines.filter((l) => l.variant_id).map((l) => l.product_id))]
  if (!pids.length) return 0
  const details = new Map<number, PublicProduct | null>()
  await Promise.all(
    pids.map(async (pid) => {
      try {
        details.set(pid, await loader(pid))
      } catch {
        /* lỗi mạng: bỏ qua sản phẩm này, giữ dòng như cũ */
      }
    })
  )
  const before = state.lines.length
  const next: CartLine[] = []
  for (const l of state.lines) {
    if (!l.variant_id || !details.has(l.product_id)) {
      next.push(l)
      continue
    }
    const d = details.get(l.product_id)
    const v = d?.variants?.find((x) => x.id === l.variant_id)
    if (!d || !v || !v.in_stock) continue
    next.push({ ...l, name: d.name, price: v.price, image_url: v.image_url || d.image_url, variant_title: v.title, sku: v.sku || '' })
  }
  state.lines = next
  return before - next.length
}

export function useCart(slug?: string) {
  if (slug && state.slug !== slug) load(slug)
  return { state, count, subtotal, add, setQty, remove, clear, syncWith, syncVariants }
}
