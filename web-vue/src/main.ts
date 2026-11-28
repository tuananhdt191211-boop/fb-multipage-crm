import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import router from './router'
import { resolveDomainSlug } from './lib/storeHost'
import { initUiI18n } from './lib/uiI18n'
import './style.css'

const app = createApp(App)
app.use(createPinia())
app.use(router)
// Trên tên miền riêng của khách: biết trước website nào cần hiển thị rồi mới mount.
// Giao diện quản trị: nạp sẵn bản dịch ngôn ngữ đã chọn để lần vẽ đầu tiên đã đúng ngôn ngữ (không chờ mạng).
Promise.allSettled([resolveDomainSlug(), initUiI18n(router)]).finally(() => app.mount('#app'))
