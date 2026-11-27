<script setup lang="ts">
// Chữ hiển thị bình thường trên web công khai; trong trình chỉnh sửa thì bấm vào là gõ trực tiếp.
// fx: kiểu chữ tuỳ chỉnh (cỡ, màu, thẻ H1/H2/P...) do người dùng chọn trong bảng "Kiểu chữ".
import { computed, inject, nextTick, onMounted, ref, useAttrs, watch } from 'vue'
import { textStyleAttrs } from '../../lib/textStyle'
import type { TextFx } from '../../types/store'

// Tự gộp class/style của cha để kiểu chữ người dùng chọn luôn được ưu tiên sau cùng
defineOptions({ inheritAttrs: false })
const attrs = useAttrs()
// attrs không reactive trong computed -> đọc lại mỗi lần render
function restAttrs() {
  const { class: _c, style: _s, ...rest } = attrs
  return rest
}

const props = defineProps<{
  modelValue?: string | number | null
  tag?: string
  editable?: boolean
  multiline?: boolean
  placeholder?: string
  fx?: TextFx
}>()

const emit = defineEmits<{ (e: 'update:modelValue', v: string): void }>()

// Trình chỉnh sửa cung cấp hàm này để mở bảng "Kiểu chữ" khi bấm vào chữ
const onTextFocus = inject<((fx: TextFx, baseTag: string, text: string) => void) | null>('ppTextFocus', null)

const el = ref<HTMLElement | null>(null)

const baseTag = computed(() => props.tag || 'span')
const renderTag = computed(() => props.fx?.style?.tag || baseTag.value)
const fxAttrs = computed(() => textStyleAttrs(props.fx?.style))

function current(): string {
  return props.modelValue === null || props.modelValue === undefined ? '' : String(props.modelValue)
}

// Chỉ ghi lại DOM khi người dùng không gõ, để con trỏ không bị nhảy
function sync() {
  const node = el.value
  if (!node || document.activeElement === node) return
  const v = current()
  if (node.innerText !== v) node.innerText = v
}

watch(() => props.modelValue, sync)
watch(() => props.editable, () => requestAnimationFrame(sync))
// Đổi thẻ (vd. P -> H2) tạo phần tử mới -> ghi lại nội dung
watch(renderTag, () => nextTick(sync))
onMounted(sync)

// Đọc từ chính phần tử phát sự kiện: khi đổi thẻ (P -> H2), el đã trỏ sang phần tử mới (rỗng)
function read(node: HTMLElement | null | undefined = el.value): string {
  const raw = node?.innerText || ''
  const t = raw.replace(/ /g, ' ')
  return props.multiline ? t.replace(/\n$/, '') : t.replace(/\n/g, ' ')
}

function onInput(e: Event) {
  emit('update:modelValue', read(e.target as HTMLElement))
}

function onKeydown(e: KeyboardEvent) {
  if ((!props.multiline && e.key === 'Enter') || e.key === 'Escape') {
    e.preventDefault()
    el.value?.blur()
  }
}

// Chỉ dán chữ thuần, bỏ định dạng từ Word / web khác
function onPaste(e: ClipboardEvent) {
  e.preventDefault()
  let text = e.clipboardData?.getData('text/plain') || ''
  if (!props.multiline) text = text.replace(/\s*\n\s*/g, ' ')
  document.execCommand('insertText', false, text)
}

function onFocus() {
  if (props.fx && onTextFocus) onTextFocus(props.fx, baseTag.value, current())
}

function onBlur(e: FocusEvent) {
  const node = e.target as HTMLElement
  // Phần tử cũ đã bị gỡ khỏi trang (vừa đổi thẻ) -> không ghi đè nội dung
  if (!node.isConnected) return
  emit('update:modelValue', read(node))
  sync()
}
</script>

<template>
  <component :is="renderTag" v-if="!editable" v-bind="restAttrs()" :class="[attrs.class, fxAttrs.cls]" :style="[attrs.style as any, fxAttrs.style]">{{ current() }}</component>
  <component
    :is="renderTag"
    v-else
    v-bind="restAttrs()"
    ref="el"
    contenteditable="true"
    spellcheck="false"
    class="pp-editable"
    :class="[attrs.class, fxAttrs.cls]"
    :style="[attrs.style as any, fxAttrs.style]"
    :data-placeholder="placeholder || 'Nhập nội dung...'"
    @input="onInput"
    @keydown="onKeydown"
    @paste="onPaste"
    @focus="onFocus"
    @blur="onBlur"
    @dragstart.prevent
  />
</template>

<style>
.pp-editable {
  outline: none;
  border-radius: 4px;
  cursor: text;
  transition: box-shadow 0.12s;
  min-width: 1ch;
}
.pp-editable:hover {
  box-shadow: 0 0 0 1px rgba(99, 102, 241, 0.45);
}
.pp-editable:focus {
  box-shadow: 0 0 0 2px rgba(99, 102, 241, 0.9);
  background: rgba(99, 102, 241, 0.04);
}
.pp-editable:empty::before {
  content: attr(data-placeholder);
  opacity: 0.4;
  pointer-events: none;
}
/* Cỡ chữ tuỳ chỉnh: theo chiều rộng khung trang (container query) */
@container (min-width: 48rem) {
  .pp-fx-fsd {
    font-size: var(--pp-fs) !important;
  }
}
@container (max-width: 47.98rem) {
  .pp-fx-fsm {
    font-size: var(--pp-fs-m) !important;
  }
}
</style>
