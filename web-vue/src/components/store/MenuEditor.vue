<script setup lang="ts">
// Quản lý menu điều hướng của website: tạo nhiều menu, thêm trang / liên kết, đổi thứ tự, tạo menu con (tối đa 2 cấp).
// Đầu trang và cột chân trang chọn menu ở phần cài đặt của chính section đó.
import { computed, ref, watch } from 'vue'
import { ArrowDown, ArrowUp, CornerDownRight, CornerUpLeft, FileText, Plus, Trash2 } from 'lucide-vue-next'
import { flattenMenu, MAX_MENUS, newMenuId, type SiteMenu } from '../../lib/menus'
import type { SitePage } from '../../lib/pages'

const props = defineProps<{ menus: SiteMenu[]; pages: SitePage[] }>()

const selId = ref('')
watch(
  () => props.menus.map((m) => m.id).join(','),
  () => {
    if (!props.menus.some((m) => m.id === selId.value)) selId.value = props.menus[0]?.id || ''
  },
  { immediate: true }
)
const menu = computed(() => props.menus.find((m) => m.id === selId.value))
const rows = computed(() => (menu.value ? flattenMenu(menu.value.items) : []))
type Row = (typeof rows.value)[number]

function addMenu() {
  if (props.menus.length >= MAX_MENUS) return
  let n = props.menus.length + 1
  while (props.menus.some((m) => m.name === `Menu ${n}`)) n++
  const id = newMenuId()
  props.menus.push({ id, name: `Menu ${n}`, items: [] })
  selId.value = id
}

function removeMenu() {
  const m = menu.value
  if (!m) return
  if (!window.confirm(`Xoá menu "${m.name}"? Các section đang dùng menu này sẽ quay về danh sách tự nhập.`)) return
  props.menus.splice(props.menus.indexOf(m), 1)
}

const siblings = (r: Row) => (r.parent ? r.parent.children! : menu.value!.items)

function move(r: Row, dir: -1 | 1) {
  const arr = siblings(r)
  const j = r.index + dir
  if (j < 0 || j >= arr.length) return
  ;[arr[r.index], arr[j]] = [arr[j]!, arr[r.index]!]
}

// Thụt vào: thành mục con của mục đứng ngay trước (chỉ khi mục này chưa có mục con — tối đa 2 cấp)
function indent(r: Row) {
  if (r.depth !== 0 || r.index === 0 || r.item.children?.length) return
  const items = menu.value!.items
  const prev = items[r.index - 1]!
  if ((prev.children?.length || 0) >= 12) return
  items.splice(r.index, 1)
  ;(prev.children ||= []).push(r.item)
}

// Đưa ra: mục con thành mục cấp 1, đứng ngay sau mục cha
function outdent(r: Row) {
  if (r.depth !== 1 || !r.parent) return
  const items = menu.value!.items
  r.parent.children!.splice(r.index, 1)
  if (!r.parent.children!.length) delete r.parent.children
  items.splice(items.indexOf(r.parent) + 1, 0, r.item)
}

function remove(r: Row) {
  siblings(r).splice(r.index, 1)
  if (r.parent && !r.parent.children!.length) delete r.parent.children
}

function addLink() {
  menu.value?.items.push({ label: 'Mục mới', link: '' })
}

const pick = ref('')
function addPage() {
  const p = props.pages.find((x) => String(x.id) === pick.value)
  pick.value = ''
  if (p && menu.value) menu.value.items.push({ label: p.title, link: p.kind === 'home' ? 'page:home' : `page:${p.slug}` })
}

const inp = 'rounded-lg border border-slate-200 bg-white px-2 py-1 text-sm focus:border-indigo-500 focus:outline-none'
</script>

<template>
  <div class="space-y-3" data-testid="menu-editor">
    <p class="text-[11px] leading-relaxed text-slate-500">
      Tạo menu một lần, dùng ở nhiều nơi: chọn menu trong cài đặt của <b>Đầu trang</b> hoặc từng cột của <b>Chân trang</b>. Sửa menu thì mọi nơi dùng nó đều đổi theo.
    </p>

    <div class="flex items-center gap-1.5">
      <select v-model="selId" data-testid="menu-select" class="min-w-0 flex-1 rounded-lg border border-slate-200 bg-white px-2 py-1.5 text-sm font-semibold">
        <option v-for="m in menus" :key="m.id" :value="m.id">{{ m.name }}</option>
        <option v-if="!menus.length" value="">Chưa có menu</option>
      </select>
      <button type="button" data-testid="menu-add" class="inline-flex items-center gap-1 rounded-lg border border-slate-200 px-2.5 py-1.5 text-xs font-bold text-slate-700 hover:border-indigo-400 hover:text-indigo-700 disabled:opacity-40" :disabled="menus.length >= MAX_MENUS" @click="addMenu">
        <Plus class="h-3.5 w-3.5" /> Menu mới
      </button>
    </div>

    <template v-if="menu">
      <div class="flex items-center gap-1.5">
        <input v-model="menu.name" data-testid="menu-name" maxlength="60" placeholder="Tên menu" :class="[inp, 'min-w-0 flex-1 font-semibold']" />
        <button type="button" class="rounded p-1.5 text-rose-600 hover:bg-rose-50" title="Xoá menu" @click="removeMenu"><Trash2 class="h-4 w-4" /></button>
      </div>

      <p v-if="!rows.length" class="rounded-xl border border-dashed border-slate-300 p-3 text-center text-xs text-slate-500">Menu trống. Thêm trang hoặc liên kết bên dưới.</p>

      <div class="space-y-1.5">
        <div v-for="(r, i) in rows" :key="i" :data-testid="'menu-row-' + i" :class="r.depth ? 'ml-5 border-l-2 border-indigo-100 pl-2' : ''" class="space-y-1 rounded-lg bg-slate-50 p-1.5">
          <div class="flex items-center gap-1">
            <input v-model="r.item.label" maxlength="60" placeholder="Chữ hiện ra" :class="[inp, 'w-2/5']" />
            <input v-model="r.item.link" list="pp-page-links" placeholder="#products, page:..., https://..." :class="[inp, 'min-w-0 flex-1']" />
          </div>
          <div class="flex items-center gap-0.5 text-slate-500">
            <button type="button" class="rounded p-1 hover:bg-slate-200 disabled:opacity-30" title="Lên" :disabled="r.index === 0" @click="move(r, -1)"><ArrowUp class="h-3.5 w-3.5" /></button>
            <button type="button" class="rounded p-1 hover:bg-slate-200 disabled:opacity-30" title="Xuống" :disabled="r.index >= siblings(r).length - 1" @click="move(r, 1)"><ArrowDown class="h-3.5 w-3.5" /></button>
            <button v-if="r.depth === 0" type="button" class="rounded p-1 hover:bg-slate-200 disabled:opacity-30" title="Thụt vào (thành menu con của mục trên)" :disabled="r.index === 0 || !!r.item.children?.length" @click="indent(r)"><CornerDownRight class="h-3.5 w-3.5" /></button>
            <button v-else type="button" class="rounded p-1 hover:bg-slate-200" title="Đưa ra ngoài (thành mục chính)" @click="outdent(r)"><CornerUpLeft class="h-3.5 w-3.5" /></button>
            <span class="flex-1" />
            <button type="button" class="rounded p-1 text-rose-600 hover:bg-rose-50" title="Xoá mục" @click="remove(r)"><Trash2 class="h-3.5 w-3.5" /></button>
          </div>
        </div>
      </div>

      <div class="space-y-2 border-t border-slate-100 pt-3">
        <label class="flex items-center gap-1.5 text-xs font-bold text-slate-600">
          <FileText class="h-3.5 w-3.5" />
          <select v-model="pick" data-testid="menu-add-page" class="min-w-0 flex-1 rounded-lg border border-slate-200 bg-white px-2 py-1.5 text-xs font-semibold" @change="addPage">
            <option value="">+ Thêm một trang vào menu…</option>
            <option v-for="p in pages" :key="p.id" :value="String(p.id)">{{ p.title }}</option>
          </select>
        </label>
        <button type="button" data-testid="menu-add-link" class="inline-flex items-center gap-1 text-xs font-bold text-indigo-700" @click="addLink"><Plus class="h-3.5 w-3.5" /> Thêm liên kết tuỳ ý</button>
      </div>
    </template>
  </div>
</template>
