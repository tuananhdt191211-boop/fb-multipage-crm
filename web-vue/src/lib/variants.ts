// Biến thể sản phẩm (kiểu Shopify): các hàm thuần dùng chung cho trình sửa, trình nhập và trang sản phẩm công khai.
// - Trình sửa giữ danh sách nhóm phân loại (tối đa 3) và danh sách biến thể; mỗi biến thể có option_values THẲNG HÀNG với nhóm
//   (nhóm chưa có giá trị thì ô tương ứng là ''), nhờ vậy thêm / bớt giá trị không làm lệch hàng.
// - Trang công khai chọn biến thể theo các giá trị khách bấm.
import type { AdminVariant, AdminVariantRow, ProductOption, PublicVariant } from '../types/store'

export const MAX_OPTION_GROUPS = 3
export const MAX_VARIANTS = 100
export const MAX_VALUES_PER_OPTION = 100

const SEP = '\u001f'
const norm = (s: string) => s.trim().toLowerCase()
export const comboKey = (vals: string[]) => vals.map(norm).join(SEP)

export interface VariantDefaults {
  price: number
  compare_price: number
  cost_price: number
  stock: number
}

// ----------------------------------------------------------------- Trình sửa

// Số tổ hợp (= số biến thể) tạo ra từ các nhóm; nhóm chưa có giá trị bị bỏ qua, không có nhóm nào → 0
export function comboCount(options: ProductOption[]): number {
  const groups = options.filter((o) => o.values.length > 0)
  if (!groups.length) return 0
  return groups.reduce((n, o) => n * o.values.length, 1)
}

// Mọi tổ hợp theo thứ tự (nhóm đầu thay đổi chậm nhất); nhóm chưa có giá trị góp một ô ''
export function combos(options: ProductOption[]): string[][] {
  if (!options.some((o) => o.values.length > 0)) return []
  let out: string[][] = [[]]
  for (const o of options) {
    const vals = o.values.length ? o.values : ['']
    out = out.flatMap((prefix) => vals.map((v) => [...prefix, v]))
  }
  return out
}

export function newVariant(vals: string[], d: VariantDefaults): AdminVariant {
  return { option_values: vals, sku: '', barcode: '', price: d.price, compare_price: d.compare_price, cost_price: d.cost_price, stock: d.stock, image_url: '' }
}

// Tạo lại danh sách biến thể sau khi nhóm / giá trị thay đổi: biến thể cũ còn tổ hợp thì GIỮ NGUYÊN dữ liệu (giá, kho, SKU, ảnh),
// tổ hợp mới nhận giá trị mặc định, tổ hợp không còn bị bỏ. Nhóm vừa có giá trị đầu tiên: biến thể cũ nhận giá trị đó.
export function rebuildVariants(options: ProductOption[], old: AdminVariant[], d: VariantDefaults): AdminVariant[] {
  const n = options.length
  const byKey = new Map<string, AdminVariant>()
  for (const v of old) {
    const vals = Array.from({ length: n }, (_, p) => {
      const first = options[p]?.values[0]
      if (first === undefined) return '' // nhóm không còn giá trị
      return v.option_values[p] || first
    })
    const k = comboKey(vals)
    if (!byKey.has(k)) byKey.set(k, v)
  }
  return combos(options).map((vals) => {
    const hit = byKey.get(comboKey(vals))
    return hit ? { ...hit, option_values: vals } : newVariant(vals, d)
  })
}

// Đổi tên một giá trị: các biến thể mang giá trị đó đổi theo (giữ nguyên dữ liệu và id)
export function renameValue(variants: AdminVariant[], p: number, from: string, to: string): AdminVariant[] {
  return variants.map((v) => (norm(v.option_values[p] ?? '') === norm(from) ? { ...v, option_values: v.option_values.map((x, i) => (i === p ? to : x)) } : v))
}

// Bỏ cả một nhóm: bỏ cột tương ứng, các biến thể trùng tổ hợp thì giữ biến thể đầu
export function dropGroup(variants: AdminVariant[], p: number): AdminVariant[] {
  const seen = new Set<string>()
  const out: AdminVariant[] = []
  for (const v of variants) {
    const vals = v.option_values.filter((_, i) => i !== p)
    const k = comboKey(vals)
    if (seen.has(k)) continue
    seen.add(k)
    out.push({ ...v, option_values: vals })
  }
  return out
}

export function variantTitle(v: { option_values: string[] }): string {
  return v.option_values.filter((x) => x.trim()).join(' / ')
}

// Nhóm có giá trị (đúng tập nhóm máy chủ nhận)
export function activeOptions(options: ProductOption[]): ProductOption[] {
  return options.filter((o) => o.values.length > 0)
}

// Biến thể từ máy chủ → dòng trong trình sửa (nhóm máy chủ trả chỉ gồm nhóm có giá trị nên thẳng hàng sẵn)
export function fromRows(options: ProductOption[], rows: AdminVariantRow[]): AdminVariant[] {
  return rows.map((r) => ({
    id: r.id,
    option_values: [r.option1, r.option2, r.option3].slice(0, options.length),
    sku: r.sku || '',
    barcode: r.barcode || '',
    price: r.price,
    compare_price: r.compare_price,
    cost_price: r.cost_price,
    stock: r.stock,
    image_url: r.image_url || ''
  }))
}

// Gửi lên máy chủ: chỉ nhóm có giá trị, và mỗi biến thể chỉ giữ giá trị của các nhóm đó
export function toPayload(options: ProductOption[], variants: AdminVariant[]) {
  const keep = options.map((o) => o.values.length > 0)
  const opts = options.filter((_, i) => keep[i]).map((o) => ({ name: o.name.trim(), values: o.values }))
  const vars = variants.map((v) => ({
    id: v.id || 0,
    option_values: v.option_values.filter((_, i) => keep[i]),
    sku: v.sku,
    barcode: v.barcode || '',
    price: Math.max(0, Math.round(Number(v.price) || 0)),
    compare_price: Math.max(0, Math.round(Number(v.compare_price) || 0)),
    cost_price: Math.max(0, Math.round(Number(v.cost_price) || 0)),
    stock: v.stock < 0 ? -1 : Math.max(0, Math.round(Number(v.stock) || 0)),
    image_url: v.image_url || ''
  }))
  return { options: opts, variants: vars }
}

// Kiểm tra trước khi lưu (máy chủ vẫn kiểm tra lại); trả '' nếu hợp lệ
export function validateVariants(options: ProductOption[], variants: AdminVariant[]): string {
  const act = activeOptions(options)
  if (!act.length) return ''
  const names = new Set<string>()
  for (const o of act) {
    if (!o.name.trim()) return 'Hãy đặt tên cho nhóm phân loại (vd. Màu, Size).'
    if (names.has(norm(o.name))) return `Tên nhóm phân loại bị trùng: ${o.name.trim()}.`
    names.add(norm(o.name))
  }
  if (variants.length > MAX_VARIANTS) return `Tối đa ${MAX_VARIANTS} biến thể cho mỗi sản phẩm (hiện có ${variants.length}). Hãy bớt giá trị phân loại.`
  for (const v of variants) {
    if (v.price < 0 || v.compare_price < 0 || v.cost_price < 0) return `Giá của biến thể ${variantTitle(v)} không hợp lệ.`
  }
  return ''
}

// Số tổng hợp hiển thị cho sản phẩm có biến thể (khớp cách máy chủ tính)
export function summarize(variants: AdminVariant[]) {
  if (!variants.length) return { min: 0, max: 0, stock: 0, unlimited: false }
  let min = variants[0]!.price
  let max = min
  let stock = 0
  let unlimited = false
  for (const v of variants) {
    min = Math.min(min, v.price)
    max = Math.max(max, v.price)
    if (v.stock < 0) unlimited = true
    else stock += v.stock
  }
  return { min, max, stock, unlimited }
}

// Nhập từ AliExpress / 1688 / link: đổi các nhóm của nguồn thành nhóm phân loại; vượt giới hạn thì bỏ bớt nhóm
// (nhóm bị bỏ trả về để người dùng ghi vào mô tả)
export function fitOptions(options: ProductOption[], max = MAX_VARIANTS): { kept: ProductOption[]; dropped: ProductOption[] } {
  const kept: ProductOption[] = []
  const dropped: ProductOption[] = []
  let total = 1
  for (const o of options) {
    const vals = [...new Set(o.values.map((v) => v.trim()).filter(Boolean))].slice(0, MAX_VALUES_PER_OPTION)
    const name = o.name.trim()
    if (!name || !vals.length) continue
    if (kept.length < MAX_OPTION_GROUPS && total * vals.length <= max) {
      kept.push({ name, values: vals })
      total *= vals.length
    } else dropped.push({ name, values: vals })
  }
  return { kept, dropped }
}

// ----------------------------------------------------------------- Trang công khai

export function findVariant(variants: PublicVariant[], selection: string[]): PublicVariant | undefined {
  const k = comboKey(selection)
  return variants.find((v) => comboKey(v.option_values) === k)
}

// Lựa chọn ban đầu: biến thể còn hàng đầu tiên (không có thì biến thể đầu tiên)
export function defaultSelection(variants: PublicVariant[]): string[] {
  const v = variants.find((x) => x.in_stock) || variants[0]
  return v ? [...v.option_values] : []
}

// Trạng thái của một giá trị nếu khách chọn nó (giữ nguyên các lựa chọn khác): ok | soldout | missing
export function valueState(variants: PublicVariant[], selection: string[], p: number, value: string): 'ok' | 'soldout' | 'missing' {
  const v = findVariant(variants, selection.map((s, i) => (i === p ? value : s)))
  if (!v) return 'missing'
  return v.in_stock ? 'ok' : 'soldout'
}

// Khách bấm một giá trị: nếu tổ hợp mới không có thì chuyển sang biến thể gần nhất có giá trị đó (ưu tiên còn hàng, rồi trùng nhiều lựa chọn khác)
export function pickValue(variants: PublicVariant[], selection: string[], p: number, value: string): string[] {
  const next = selection.map((s, i) => (i === p ? value : s))
  if (findVariant(variants, next)) return next
  const cands = variants.filter((v) => norm(v.option_values[p] ?? '') === norm(value))
  if (!cands.length) return selection
  const score = (v: PublicVariant) => (v.in_stock ? 100 : 0) + v.option_values.filter((x, i) => i !== p && norm(x) === norm(selection[i] ?? '')).length
  return [...cands.sort((a, b) => score(b) - score(a))[0]!.option_values]
}
