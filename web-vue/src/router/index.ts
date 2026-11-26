import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '@/stores/auth.stores'
import BroadcastView from '../views/BroadcastView.vue'

const router = createRouter({
  history: createWebHistory(),
  routes: [
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
  component: BroadcastView,
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
  meta: { requiresAuth: true }
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
  path: '/s/:slug',
  name: 'public-site',
  component: () => import('../views/PublicStoreView.vue')
},
{
  path: '/:pathMatch(.*)*',
  redirect: '/'
}
  ]
})

router.beforeEach(async (to) => {
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