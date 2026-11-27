import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import router from './router'
import { resolveDomainSlug } from './lib/storeHost'
import './style.css'

const app = createApp(App)
app.use(createPinia())
app.use(router)
// Trên tên miền riêng của khách: biết trước website nào cần hiển thị rồi mới mount
resolveDomainSlug().finally(() => app.mount('#app'))
