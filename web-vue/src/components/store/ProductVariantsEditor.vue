<script setup lang="ts">
// Phân loại sản phẩm (kiểu Shopify): tối đa 3 nhóm (Màu, Size...), mỗi nhóm nhiều giá trị; bảng biến thể tự sinh ra
// với giá, giá gốc, giá nhập, tồn kho, SKU và ảnh riêng. Sửa nhóm / giá trị thì dữ liệu của biến thể còn tổ hợp được giữ nguyên.
import { computed, ref } from 'vue'
import { ImagePlus, Plus, Trash2, X } from 'lucide-vue-next'
import { adminMoney } from '../../lib/uiI18n'
import { safeImg } from '../../lib/safeUrl'
import type { ProductMedia } from '../../lib/productMedia'
import type { AdminVariant, ProductOption } from '../../types/store'
import {
  MAX_OPTION_GROUPS,
  MAX_VALUES_PER_OPTION,
  MAX_VARIANTS,
  comboCount,
  dropGroup,
  rebuildVariants,
  renameValue,
  summarize,
  variantTitle,
  type VariantDefaults
} from '../../lib/variants'

const props = defineProps<{ media: ProductMedia[]; defaults: VariantDefaults }>()
const options = defineModel<ProductOption[]>('options', { required: true })
const variants = defineModel<AdminVariant[]>('variants', { required: true })

const error = ref('')
const drafts = ref<string[]>(['', '', ''])
const pickerFor = ref<number | null>(null)
const sameFirst = ref(false)

const imageMedia = computed(() => props.media.filter((m) => m.type === 'image'))
const total = computed(() => summarize(variants.value))
const nextCount = computed(() => comboCount(options.value))

// Gán một lần cho cả hai v-model (cha chỉ cập nhật sau khi render lại nên không đọc lại giữa chừng)
function commit(nextOpts: ProductOption[], nextVars?: AdminVariant[]): boolean {
  if (comboCount(nextOpts) > MAX_VARIANTS) {
    error.value = `Tối đa ${MAX_VARIANTS} biến thể cho mỗi sản phẩm. Hãy bớt giá trị phân loại.`
    return false
  }
  error.value = ''
  variants.value = rebuildVariants(nextOpts, nextVars ?? variants.value, props.defaults)
  options.value = nextOpts
  return true
}

function addGroup() {
  if (options.value.length >= MAX_OPTION_GROUPS) return
  options.value = [...options.value, { name: '', values: [] }]
}

function removeGroup(p: number) {
  const nextOpts = options.value.filter((_, i) => i !== p)
  error.value = ''
  variants.value = rebuildVariants(nextOpts, dropGroup(variants.value, p), props.defaults)
  options.value = nextOpts
  drafts.value = ['', '', '']
  pickerFor.value = null
}

function setName(p: number, name: string) {
  options.value = options.value.map((o, i) => (i === p ? { ...o, name } : o))
}

// "Đen, Trắng, Xanh" → thêm từng giá trị (bỏ trùng, không phân biệt hoa thường)
function addValues(p: number, raw: string) {
  const cur = options.value[p]
  if (!cur) return
  const vals = [...cur.values]
  for (const piece of raw.split(/[,\n;]/)) {
    const v = piece.trim().slice(0, 60)
    if (v && !vals.some((x) => x.toLowerCase() === v.toLowerCase()) && vals.length < MAX_VALUES_PER_OPTION) vals.push(v)
  }
  if (vals.length === cur.values.length) return
  commit(options.value.map((o, i) => (i === p ? { ...o, values: vals } : o)))
}

function commitDraft(p: number) {
  const raw = drafts.value[p] || ''
  drafts.value[p] = ''
  if (raw.trim()) addValues(p, raw)
}

function onDraftKey(p: number, e: KeyboardEvent) {
  if (e.key === 'Enter' || e.key === ',') {
    e.preventDefault()
    commitDraft(p)
  }
}

function removeValue(p: number, val: string) {
  commit(options.value.map((o, i) => (i === p ? { ...o, values: o.values.filter((x) => x !== val) } : o)))
}

// Sửa chữ của một giá trị tại chỗ: các biến thể mang giá trị đó đổi theo (giữ nguyên giá, kho, SKU, ảnh)
function renameVal(p: number, old: string, e: Event) {
  const input = e.target as HTMLInputElement
  const to = input.value.trim().slice(0, 60)
  if (!to) return removeValue(p, old)
  if (to === old) return
  if (options.value[p]!.values.some((x) => x !== old && x.toLowerCase() === to.toLowerCase())) {
    error.value = `Giá trị "${to}" đã có trong nhóm này.`
    input.value = old
    return
  }
  commit(
    options.value.map((o, i) => (i === p ? { ...o, values: o.values.map((x) => (x === old ? to : x)) } : o)),
    renameValue(variants.value, p, old, to)
  )
}

// ----- Đặt hàng loạt -----
const bulkField = ref<'price' | 'compare_price' | 'cost_price' | 'stock'>('price')
const bulkValue = ref<number | ''>('')
function applyBulk(unlimited = false) {
  if (!unlimited && (bulkValue.value === '' || Number(bulkValue.value) < 0)) return
  const val = unlimited ? -1 : Math.round(Number(bulkValue.value))
  for (const v of variants.value) v[bulkField.value] = val
}

function toggleUnlimited(v: AdminVariant) {
  v.stock = v.stock < 0 ? 0 : -1
}

// ----- Ảnh của biến thể (chọn trong ảnh của sản phẩm) -----
function setImage(i: number, url: string) {
  const v = variants.value[i]
  if (!v) return
  if (sameFirst.value && options.value.length > 1) {
    const key = v.option_values[0]
    for (const x of variants.value) if (x.option_values[0] === key) x.image_url = url
  } else v.image_url = url
  pickerFor.value = null
}
</script>

<template>
  <div class="space-y-4" data-testid="variants-editor">
    <div v-if="!options.length" class="rounded-xl border border-dashed border-slate-300 bg-slate-50 p-4 text-xs leading-relaxed text-slate-500">
      Sản phẩm có nhiều lựa chọn như màu sắc, kích cỡ? Thêm phân loại để mỗi lựa chọn có giá, tồn kho, SKU và ảnh riêng — khách chọn ngay trên trang sản phẩm.
    </div>

    <!-- Nhóm phân loại -->
    <div v-for="(o, p) in options" :key="p" class="space-y-2 rounded-xl border border-slate-200 p-3" data-testid="option-group">
      <div class="flex items-center gap-2">
        <input
          :value="o.name"
          list="pp-option-names"
          maxlength="40"
          data-testid="option-name"
          placeholder="Tên nhóm (vd. Màu sắc, Kích cỡ)"
          class="w-full rounded-lg border border-slate-200 px-3 py-1.5 text-sm font-semibold"
          @input="setName(p, ($event.target as HTMLInputElement).value)"
        />
        <button type="button" class="rounded-lg border border-rose-200 p-1.5 text-rose-600 hover:bg-rose-50" title="Xóa nhóm này" @click="removeGroup(p)">
          <Trash2 class="h-4 w-4" />
        </button>
      </div>
      <div class="flex flex-wrap items-center gap-1.5">
        <span v-for="val in o.values" :key="val" class="inline-flex items-center gap-1 rounded-full bg-slate-100 py-1 pl-2.5 pr-1.5">
          <input
            :value="val"
            :size="Math.max(3, val.length)"
            maxlength="60"
            class="bg-transparent text-xs font-semibold text-slate-700 outline-none"
            data-testid="option-value"
            @change="renameVal(p, val, $event)"
          />
          <button type="button" class="text-slate-400 hover:text-rose-600" title="Xóa giá trị" @click="removeValue(p, val)"><X class="h-3 w-3" /></button>
        </span>
        <input
          v-model="drafts[p]"
          data-testid="option-draft"
          placeholder="Thêm giá trị rồi nhấn Enter (vd. Đen, Trắng)"
          class="min-w-[220px] flex-1 rounded-lg border border-dashed border-slate-300 px-3 py-1.5 text-xs"
          @keydown="onDraftKey(p, $event)"
          @blur="commitDraft(p)"
        />
      </div>
    </div>
    <datalist id="pp-option-names">
      <option value="Màu sắc" />
      <option value="Kích cỡ" />
      <option value="Chất liệu" />
      <option value="Kiểu dáng" />
      <option value="Dung lượng" />
    </datalist>

    <button
      v-if="options.length < MAX_OPTION_GROUPS"
      type="button"
      class="inline-flex items-center gap-1.5 rounded-xl border border-emerald-200 px-3 py-1.5 text-xs font-bold text-emerald-700 hover:bg-emerald-50"
      data-testid="add-option"
      @click="addGroup"
    >
      <Plus class="h-3.5 w-3.5" /> {{ options.length ? 'Thêm nhóm phân loại khác' : 'Thêm phân loại (Màu, Size...)' }}
    </button>

    <p v-if="error" class="rounded-lg bg-rose-50 px-3 py-2 text-xs font-semibold text-rose-600">{{ error }}</p>
    <p v-else-if="nextCount >= MAX_VARIANTS" class="text-[11px] text-amber-600">Đã đạt tối đa {{ MAX_VARIANTS }} biến thể.</p>

    <!-- Bảng biến thể -->
    <div v-if="variants.length" class="space-y-3">
      <div class="flex flex-wrap items-center gap-2 rounded-xl bg-slate-50 p-3">
        <span class="text-xs font-bold text-slate-600">Đặt cùng lúc cho tất cả biến thể:</span>
        <select v-model="bulkField" class="rounded-lg border border-slate-200 bg-white px-2 py-1.5 text-xs">
          <option value="price">Giá bán</option>
          <option value="compare_price">Giá gốc</option>
          <option value="cost_price">Giá nhập</option>
          <option value="stock">Tồn kho</option>
        </select>
        <input v-model.number="bulkValue" type="number" min="0" class="w-28 rounded-lg border border-slate-200 px-2 py-1.5 text-xs" data-testid="bulk-value" />
        <button type="button" class="rounded-lg bg-slate-800 px-3 py-1.5 text-xs font-bold text-white hover:bg-slate-900" data-testid="bulk-apply" @click="applyBulk()">Áp dụng</button>
        <button v-if="bulkField === 'stock'" type="button" class="rounded-lg border border-slate-300 px-3 py-1.5 text-xs font-bold hover:bg-white" @click="applyBulk(true)">Không giới hạn</button>
      </div>

      <div class="overflow-x-auto">
        <table class="w-full min-w-[780px] text-left text-xs">
          <thead>
            <tr class="border-b border-slate-200 text-[11px] font-bold uppercase tracking-wide text-slate-400">
              <th class="py-2 pr-2">Biến thể</th>
              <th class="px-1 py-2">Ảnh</th>
              <th class="px-1 py-2">Giá bán (đ)</th>
              <th class="px-1 py-2">Giá gốc (đ)</th>
              <th class="px-1 py-2">Giá nhập (đ)</th>
              <th class="px-1 py-2">Tồn kho</th>
              <th class="px-1 py-2">SKU</th>
            </tr>
          </thead>
          <tbody>
            <template v-for="(v, i) in variants" :key="variantTitle(v)">
              <tr class="border-b border-slate-100 align-middle" data-testid="variant-row">
                <td class="py-2 pr-2 text-sm font-bold text-slate-800" data-i18n="skip" data-testid="variant-title">{{ variantTitle(v) }}</td>
                <td class="px-1 py-2">
                  <button
                    type="button"
                    class="flex h-10 w-10 items-center justify-center overflow-hidden rounded-lg border border-dashed border-slate-300 bg-slate-50 hover:border-emerald-400"
                    title="Chọn ảnh cho biến thể này"
                    data-testid="variant-image-btn"
                    @click="pickerFor = pickerFor === i ? null : i"
                  >
                    <img v-if="safeImg(v.image_url)" :src="safeImg(v.image_url)" alt="" referrerpolicy="no-referrer" class="h-full w-full object-cover" />
                    <ImagePlus v-else class="h-4 w-4 text-slate-400" />
                  </button>
                </td>
                <td class="px-1 py-2"><input v-model.number="v.price" type="number" min="0" class="w-28 rounded-lg border border-slate-200 px-2 py-1.5" data-testid="variant-price" /></td>
                <td class="px-1 py-2"><input v-model.number="v.compare_price" type="number" min="0" class="w-28 rounded-lg border border-slate-200 px-2 py-1.5" /></td>
                <td class="px-1 py-2"><input v-model.number="v.cost_price" type="number" min="0" class="w-28 rounded-lg border border-slate-200 px-2 py-1.5" /></td>
                <td class="px-1 py-2">
                  <div class="flex items-center gap-1.5">
                    <input v-if="v.stock < 0" value="∞" disabled class="w-20 rounded-lg border border-slate-200 bg-slate-100 px-2 py-1.5 text-center" />
                    <input v-else v-model.number="v.stock" type="number" min="0" class="w-20 rounded-lg border border-slate-200 px-2 py-1.5" data-testid="variant-stock" />
                    <label class="flex items-center gap-1 text-[11px] font-semibold text-slate-500" title="Không giới hạn tồn kho">
                      <input type="checkbox" :checked="v.stock < 0" @change="toggleUnlimited(v)" /> ∞
                    </label>
                  </div>
                </td>
                <td class="px-1 py-2"><input v-model="v.sku" maxlength="100" class="w-32 rounded-lg border border-slate-200 px-2 py-1.5" data-testid="variant-sku" /></td>
              </tr>
              <tr v-if="pickerFor === i" class="border-b border-slate-100 bg-slate-50">
                <td colspan="7" class="p-3">
                  <p v-if="!imageMedia.length" class="text-xs text-slate-500">Hãy thêm ảnh sản phẩm ở mục “Hình ảnh &amp; video” trước, rồi chọn ảnh cho biến thể này.</p>
                  <div v-else class="flex flex-wrap items-center gap-2">
                    <button
                      v-for="m in imageMedia"
                      :key="m.url"
                      type="button"
                      class="h-14 w-14 overflow-hidden rounded-lg border-2 bg-white"
                      :class="v.image_url === m.url ? 'border-emerald-500' : 'border-transparent hover:border-slate-300'"
                      data-testid="variant-image-choice"
                      @click="setImage(i, m.url)"
                    >
                      <img :src="safeImg(m.url)" alt="" referrerpolicy="no-referrer" class="h-full w-full object-cover" />
                    </button>
                    <button v-if="v.image_url" type="button" class="rounded-lg border border-slate-300 px-2.5 py-1.5 text-[11px] font-bold hover:bg-white" @click="setImage(i, '')">Bỏ ảnh</button>
                  </div>
                  <label v-if="options.length > 1 && imageMedia.length" class="mt-2 flex items-center gap-1.5 text-[11px] font-semibold text-slate-500">
                    <input v-model="sameFirst" type="checkbox" /> Dùng ảnh này cho mọi biến thể cùng “{{ v.option_values[0] }}”
                  </label>
                </td>
              </tr>
            </template>
          </tbody>
        </table>
      </div>

      <p class="flex flex-wrap gap-x-4 gap-y-1 text-xs text-slate-500" data-testid="variants-summary">
        <span><strong>{{ variants.length }}</strong> biến thể</span>
        <span>
          Giá:
          <strong>{{ adminMoney(total.min) }}</strong><template v-if="total.max > total.min"> – <strong>{{ adminMoney(total.max) }}</strong></template>
        </span>
        <span>Tồn kho: <strong>{{ total.unlimited ? 'Không giới hạn' : total.stock }}</strong></span>
      </p>
    </div>
  </div>
</template>
