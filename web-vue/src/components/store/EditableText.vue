<script setup lang="ts">
// Chữ hiển thị bình thường trên web công khai; trong trình chỉnh sửa thì bấm vào là gõ trực tiếp.
import { onMounted, ref, watch } from 'vue'

const props = defineProps<{
  modelValue?: string | number | null
  tag?: string
  editable?: boolean
  multiline?: boolean
  placeholder?: string
}>()

const emit = defineEmits<{ (e: 'update:modelValue', v: string): void }>()

const el = ref<HTMLElement | null>(null)

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
onMounted(sync)

function read(): string {
  const raw = el.value?.innerText || ''
  const t = raw.replace(/ /g, ' ')
  return props.multiline ? t.replace(/\n$/, '') : t.replace(/\n/g, ' ')
}

function onInput() {
  emit('update:modelValue', read())
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

function onBlur() {
  emit('update:modelValue', read())
  sync()
}
</script>

<template>
  <component :is="tag || 'span'" v-if="!editable">{{ current() }}</component>
  <component
    :is="tag || 'span'"
    v-else
    ref="el"
    contenteditable="true"
    spellcheck="false"
    class="pp-editable"
    :data-placeholder="placeholder || 'Nhập nội dung...'"
    @input="onInput"
    @keydown="onKeydown"
    @paste="onPaste"
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
</style>
