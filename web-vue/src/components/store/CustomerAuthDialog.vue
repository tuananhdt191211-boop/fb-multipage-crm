<script setup lang="ts">
// Hộp đăng nhập / đăng ký tài khoản khách hàng của website
import { computed, ref, watch } from 'vue'
import { Eye, EyeOff, Loader2, X } from 'lucide-vue-next'
import { apiError, customerState, useCustomer } from '../../lib/customer'
import { t } from '../../lib/i18n'

const props = defineProps<{ slug: string; accent: string; siteTitle?: string }>()
const cust = useCustomer(props.slug)

const busy = ref(false)
const err = ref('')
const showPw = ref(false)
const f = ref({ login: '', name: '', phone: '', email: '', password: '' })

const mode = computed(() => customerState.dialog)
watch(mode, () => {
  err.value = ''
  showPw.value = false
})

const inp = 'w-full rounded-xl border border-slate-200 px-3.5 py-2.5 text-sm outline-none transition focus:border-slate-400 focus:ring-2 focus:ring-slate-100'

function close() {
  customerState.dialog = null
}

async function submit() {
  err.value = ''
  busy.value = true
  try {
    if (mode.value === 'login') {
      await cust.login(f.value.login.trim(), f.value.password)
    } else {
      await cust.register({ name: f.value.name.trim(), phone: f.value.phone.trim(), email: f.value.email.trim(), password: f.value.password })
    }
    f.value.password = ''
    close()
  } catch (e) {
    err.value = apiError(e, mode.value === 'login' ? t('login_failed') : t('register_failed'))
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <Teleport to="body">
    <div v-if="mode" class="fixed inset-0 z-[95] flex items-center justify-center bg-black/45 p-4" @click.self="close">
      <form class="w-full max-w-sm space-y-4 rounded-2xl bg-white p-6 text-slate-800 shadow-2xl" @submit.prevent="submit">
        <div class="flex items-start justify-between">
          <div>
            <h2 class="text-xl font-extrabold">{{ mode === 'login' ? t('login') : t('create_account') }}</h2>
            <p class="mt-0.5 text-xs text-slate-500">{{ t('auth_sub', { shop: siteTitle || '' }) }}</p>
          </div>
          <button type="button" class="rounded-lg p-1.5 hover:bg-slate-100" :aria-label="t('close')" @click="close"><X class="h-5 w-5" /></button>
        </div>

        <div class="grid grid-cols-2 rounded-xl bg-slate-100 p-1 text-sm font-bold">
          <button type="button" class="rounded-lg py-1.5 transition" :class="mode === 'login' ? 'bg-white shadow-sm' : 'text-slate-500'" @click="customerState.dialog = 'login'">{{ t('login') }}</button>
          <button type="button" class="rounded-lg py-1.5 transition" :class="mode === 'register' ? 'bg-white shadow-sm' : 'text-slate-500'" @click="customerState.dialog = 'register'">{{ t('register') }}</button>
        </div>

        <template v-if="mode === 'login'">
          <input v-model="f.login" :class="inp" :placeholder="t('login_id')" autocomplete="username" required />
        </template>
        <template v-else>
          <input v-model="f.name" :class="inp" :placeholder="t('full_name')" autocomplete="name" required />
          <input v-model="f.phone" :class="inp" :placeholder="t('phone')" inputmode="tel" autocomplete="tel" required />
          <input v-model="f.email" :class="inp" type="email" :placeholder="t('email_optional')" autocomplete="email" />
        </template>
        <div class="relative">
          <input
            v-model="f.password"
            :type="showPw ? 'text' : 'password'"
            :class="inp"
            class="pr-10"
            :placeholder="mode === 'login' ? t('password') : t('password_new')"
            :autocomplete="mode === 'login' ? 'current-password' : 'new-password'"
            minlength="6"
            required
          />
          <button type="button" class="absolute right-2.5 top-1/2 -translate-y-1/2 p-1 text-slate-400 hover:text-slate-600" :aria-label="showPw ? t('hide_password') : t('show_password')" @click="showPw = !showPw">
            <EyeOff v-if="showPw" class="h-4 w-4" /><Eye v-else class="h-4 w-4" />
          </button>
        </div>

        <p v-if="err" class="rounded-xl bg-rose-50 px-3 py-2 text-xs font-semibold text-rose-600">{{ err }}</p>

        <button type="submit" class="flex w-full items-center justify-center gap-2 rounded-full py-3 text-sm font-bold text-white transition hover:opacity-90 disabled:opacity-60" :style="{ backgroundColor: accent }" :disabled="busy">
          <Loader2 v-if="busy" class="h-4 w-4 animate-spin" /> {{ mode === 'login' ? t('login') : t('register') }}
        </button>
        <p class="text-center text-xs text-slate-500">
          <template v-if="mode === 'login'">{{ t('no_account') }} <button type="button" class="font-bold underline" @click="customerState.dialog = 'register'">{{ t('register_now') }}</button></template>
          <template v-else>{{ t('have_account') }} <button type="button" class="font-bold underline" @click="customerState.dialog = 'login'">{{ t('login') }}</button></template>
        </p>
      </form>
    </div>
  </Teleport>
</template>
