import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'
import { useAuthStore } from '@/stores/auth.stores'
import type { Feature } from '@/types/auth'
import { onCustomDomain } from '@/lib/storeHost'
import { trackPage } from '@/lib/analytics'

const appRoutes: RouteRecordRaw[] = [
    {
      path: '/login',
      name: 'login',
      component: () => import('@/views/LoginView.vue'),
      meta: { guestOnly: true }
    },
    {
      path: '/register',
      name: 'register',
      component: () => import('@/views/RegisterLogin.vue'),
      meta: { guestOnly: true }
    },
    {
      path: '/auth/callback',
      name: 'auth-callback',
      component: () => import('@/views/AuthCallbackView.vue')
    },
    // Trang pháp lý CÔNG KHAI của ứng dụng (Meta bắt buộc có): không cần đăng nhập, không dùng khung quản trị
    {
      path: '/privacy',
      name: 'legal-privacy',
      component: () => import('@/views/LegalView.vue'),
      meta: { legal: 'privacy' }
    },
    {
      path: '/terms',
      name: 'legal-terms',
      component: () => import('@/views/LegalView.vue'),
      meta: { legal: 'terms' }
    },
    {
      path: '/data-deletion',
      name: 'legal-deletion',
      component: () => import('@/views/LegalView.vue'),
      meta: { legal: 'deletion' }
    },
    {
      path: '/',
      name: 'dashboard',
      component: () => import('@/views/DashboardView.vue'),
      meta: { requiresAuth: true }
    },
    {
      // đường dẫn cũ → trang mới theo vai trò
      path: '/roles',
      name: 'roles',
      redirect: () => (localStorage.getItem('pp_is_admin') === '1' ? '/admin/customers' : '/team')
    },
    {
      path: '/admin/customers',
      name: 'admin-customers',
      component: () => import('@/views/AdminCustomersView.vue'),
      meta: { requiresAuth: true, requiresAdmin: true }
    },
    {
      path: '/team',
      name: 'team',
      component: () => import('@/views/TeamView.vue'),
      meta: { requiresAuth: true, ownerOnly: true }
    },
    {
      path: '/pages',
      name: 'pages',
      component: () => import('@/views/PageView.vue'),
      meta: { requiresAuth: true, feature: 'pages' }
    },
    {
      path: '/inbox',
      name: 'inbox',
      component: () => import('@/views/InboxView.vue'),
      meta: { requiresAuth: true, feature: 'inbox' }
    },
  
    {
  path: '/posts',
  name: 'posts',
  component: () => import('@/views/PostSchedulerView.vue'),
  meta: { requiresAuth: true, feature: 'posts' }
},
{
  path: '/competitors',
  name: 'competitors',
  component: () => import('@/views/CompetitorView.vue'),
  meta: { requiresAuth: true, feature: 'competitors' }
},
{
  path: '/broadcasts',
  name: 'broadcasts',
  component: () => import('../views/BroadcastView.vue'),
  meta: { requiresAuth: true, feature: 'broadcasts' }
},
{
  path: '/sites',
  name: 'sites',
  component: () => import('../views/SitesView.vue'),
  meta: { requiresAuth: true, feature: 'sites' }
},
{
  path: '/sites/:id/edit',
  name: 'site-edit',
  component: () => import('../views/StoreBuilderView.vue'),
  meta: { requiresAuth: true, noShell: true, feature: 'sites' }
},
{
  path: '/sites/:id/products',
  name: 'site-products',
  component: () => import('../views/ProductsView.vue'),
  meta: { requiresAuth: true, feature: 'sites' }
},
{
  // đường dẫn cũ → danh sách website
  path: '/store-builder',
  name: 'store-builder',
  redirect: { name: 'sites' }
},
{
  path: '/store',
  name: 'public-store',
  component: () => import('../views/PublicStoreView.vue')
},
{
  path: '/sites/:id/analytics',
  name: 'site-analytics',
  component: () => import('../views/SiteAnalyticsView.vue'),
  meta: { requiresAuth: true, feature: 'sites' }
},
{
  path: '/sites/:id/orders',
  name: 'site-orders',
  component: () => import('../views/SiteOrdersView.vue'),
  meta: { requiresAuth: true, feature: 'sites' }
},
{
  path: '/s/:slug',
  name: 'public-site',
  component: () => import('../views/PublicStoreView.vue')
},
{
  path: '/s/:slug/p/:pid',
  name: 'public-product',
  component: () => import('../views/PublicProductView.vue')
},
{
  path: '/s/:slug/pages/:page',
  name: 'public-page',
  component: () => import('../views/PublicPageView.vue')
},
{
  path: '/s/:slug/checkout',
  name: 'public-checkout',
  component: () => import('../views/PublicCheckoutView.vue')
},
{
  path: '/s/:slug/order/:code',
  name: 'public-order',
  component: () => import('../views/PublicOrderView.vue')
},
{
  path: '/s/:slug/account',
  name: 'public-account',
  component: () => import('../views/PublicAccountView.vue')
},
{
  path: '/:pathMatch(.*)*',
  redirect: '/'
}
]

// Trên tên miền riêng của khách chỉ có cửa hàng (không có trang quản trị), đường dẫn ngắn.
// Cùng tên route với bản /s/:slug nên router.push({ name: 'public-checkout' }) chạy ở cả hai nơi.
const storeRoutes: RouteRecordRaw[] = [
  { path: '/', name: 'public-site', component: () => import('../views/PublicStoreView.vue') },
  { path: '/p/:pid', name: 'public-product', component: () => import('../views/PublicProductView.vue') },
  { path: '/pages/:page', name: 'public-page', component: () => import('../views/PublicPageView.vue') },
  { path: '/checkout', name: 'public-checkout', component: () => import('../views/PublicCheckoutView.vue') },
  { path: '/order/:code', name: 'public-order', component: () => import('../views/PublicOrderView.vue') },
  { path: '/account', name: 'public-account', component: () => import('../views/PublicAccountView.vue') },
  // link cũ dạng /s/<slug>/... vẫn mở được
  { path: '/s/:slug/account', redirect: '/account' },
  { path: '/s/:slug', redirect: '/' },
  { path: '/s/:slug/p/:pid', redirect: (to) => `/p/${to.params.pid}` },
  { path: '/s/:slug/pages/:page', redirect: (to) => `/pages/${to.params.page}` },
  { path: '/s/:slug/checkout', redirect: '/checkout' },
  { path: '/s/:slug/order/:code', redirect: (to) => ({ path: `/order/${to.params.code}`, query: to.query }) },
  { path: '/:pathMatch(.*)*', redirect: '/' }
]

const router = createRouter({
  history: createWebHistory(),
  routes: onCustomDomain ? storeRoutes : appRoutes
})

// Thống kê: mỗi lần khách mở một trang của web bán hàng
router.afterEach((to) => {
  if (typeof to.name === 'string' && to.name.startsWith('public-')) trackPage(to.path)
})

let sessionChecked = false

router.beforeEach(async (to) => {
  if (onCustomDomain) return true
  if (to.meta.legal) return true // trang pháp lý công khai: không hỏi phiên đăng nhập (token hết hạn cũng không bị đẩy về /login)
  const auth = useAuthStore()
  const storedToken = localStorage.getItem('access_token')

  // Đồng bộ lại state Pinia nếu localStorage đã bị xóa do lỗi 401
  if (!storedToken && auth.token) {
    auth.clearSession()
  }

  // Mỗi lần mở / tải lại web: hỏi máy chủ tài khoản + trang được dùng hiện tại (admin có thể vừa đổi quyền, khoá tài khoản)
  if (storedToken && (!sessionChecked || !auth.user || !auth.tenant) && to.name !== 'auth-callback') {
    sessionChecked = true
    const ok = await auth.fetchMe()
    if (!ok) {
      return { name: 'login' }
    }
  }
  try {
    localStorage.setItem('pp_is_admin', auth.isAdmin ? '1' : '0') // để chuyển hướng /roles cũ
  } catch {
    /* bỏ qua */
  }

  const loggedIn = Boolean(localStorage.getItem('access_token'))

  if (to.meta.requiresAuth && !loggedIn) {
    return { name: 'login', query: { redirect: to.fullPath } }
  }

  // Phân quyền theo vai trò và theo trang được cấp: không có quyền thì về Tổng quan kèm thông báo
  if (to.meta.requiresAdmin && !auth.isAdmin) {
    return { name: 'dashboard', query: { denied: '1' } }
  }
  if (to.meta.ownerOnly && !auth.isOwner) {
    return { name: 'dashboard', query: { denied: '1' } }
  }
  if (typeof to.meta.feature === 'string' && !auth.can(to.meta.feature as Feature)) {
    return { name: 'dashboard', query: { denied: '1' } }
  }

  if (to.meta.guestOnly && loggedIn) {
    return { name: 'dashboard' }
  }
})

export default router