<script setup lang="ts">
import { computed } from 'vue'
import { Clock, CreditCard, Gift, Headset, Heart, Leaf, RefreshCcw, ShieldCheck, Star, Truck } from 'lucide-vue-next'

const props = defineProps<{ name?: string }>()
const map: Record<string, any> = {
  truck: Truck,
  refresh: RefreshCcw,
  shield: ShieldCheck,
  headset: Headset,
  gift: Gift,
  leaf: Leaf,
  star: Star,
  clock: Clock,
  card: CreditCard,
  heart: Heart
}
const comp = computed(() => map[props.name || ''] || Star)
</script>

<template>
  <component :is="comp" />
</template>
