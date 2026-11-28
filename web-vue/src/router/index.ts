import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'
import { useAuthStore } from '@/stores/auth.stores'
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
    {
      path: '/',
      name: 'dashboard',
      component: () => import('@/views/DashboardView.vue'),
      meta: { requiresAuth: true }
    },
    {
      path: '/roles',
      name: 'roles',
      component: () => import('@/views/RoleManagementView.vue'),
      meta: { requiresAuth: true, requiresRoleManagerOrAdmin: true }
    },
    {
      path: '/pages',
      name: 'pages',
      component: () => import('@/views/PageView.vue'),
      meta: { requiresAuth: true }
    },
    {
      path: '/inbox',
      name: 'inbox',
      component: () => import('@/views/InboxView.vue'),
      meta: { requiresAuth: true }
    },
  
    {
  path: '/posts',
  name: 'posts',
  component: () => import('@/views/PostSchedulerView.vue'),
  meta: { requiresAuth: true }
},
{
  path: '/competitors',
  name: 'competitors',
  component: () => import('@/views/CompetitorView.vue'),
  meta: { requiresAuth: true }
},
{
  path: '/broadcasts',
  name: 'broadcasts',
  component: () => import('../views/BroadcastView.vue'),
  meta: { requiresAuth: true }
},
{
  path: '/sites',
  name: 'sites',
  component: () => import('../views/SitesView.vue'),
  meta: { requiresAuth: true }
},
{
  path: '/sites/:id/edit',
  name: 'site-edit',
  component: () => import('../views/StoreBuilderView.vue'),
  meta: { requiresAuth: true, noShell: true }
},
{
  path: '/sites/:id/products',
  name: 'site-products',
  component: () => import('../views/ProductsView.vue'),
  meta: { requiresAuth: true }
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
  meta: { requiresAuth: true }
},
{
  path: '/sites/:id/orders',
  name: 'site-orders',
  component: () => import('../views/SiteOrdersView.vue'),
  meta: { requiresAuth: true }
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
  { path: '/checkout', name: 'public-checkout', component: () => import('../views/PublicCheckoutView.vue') },
  { path: '/order/:code', name: 'public-order', component: () => import('../views/PublicOrderView.vue') },
  { path: '/account', name: 'public-account', component: () => import('../views/PublicAccountView.vue') },
  // link cũ dạng /s/<slug>/... vẫn mở được
  { path: '/s/:slug/account', redirect: '/account' },
  { path: '/s/:slug', redirect: '/' },
  { path: '/s/:slug/p/:pid', redirect: (to) => `/p/${to.params.pid}` },
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

router.beforeEach(async (to) => {
  if (onCustomDomain) return true
  const auth = useAuthStore()
  const storedToken = localStorage.getItem('access_token')

  // Đồng bộ lại state Pinia nếu localStorage đã bị xóa do lỗi 401
  if (!storedToken && auth.token) {
    auth.clearSession()
  }

  if (storedToken && !auth.user && to.name !== 'auth-callback') {
    const ok = await auth.fetchMe()
    if (!ok) {
      return { name: 'login' }
    }
  }

  const loggedIn = Boolean(localStorage.getItem('access_token'))

  if (to.meta.requiresAuth && !loggedIn) {
    return { name: 'login', query: { redirect: to.fullPath } }
  }

  if (to.meta.requiresRoleManagerOrAdmin && !auth.canManageRoles) {
    return { name: 'dashboard' }
  }

  if (to.meta.guestOnly && loggedIn) {
    return { name: 'dashboard' }
  }
})

export default router