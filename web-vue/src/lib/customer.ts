// Tài khoản khách hàng trên web công khai (mỗi website một phiên riêng, lưu trong trình duyệt).
// Dùng axios RIÊNG: lỗi 401 của khách không được làm đăng xuất tài khoản quản trị.
import axios from 'axios'
import { reactive } from 'vue'
import { api } from './api'

export interface ShopCustomer {
  id: number
  name: string
  phone: string
  email: string
  address: string
}

export interface CustomerOrder {
  code: string
  token: string
  total: number
  status: string
  payment_method: string
  payment_status: string
  items: number
  qty: number
  created_at: string
}

const shopApi = axios.create({ baseURL: api.defaults.baseURL, timeout: 30000, headers: { 'Content-Type': 'application/json' } })

const key = (slug: string) => `pp_cust_${slug}`

// Một trạng thái dùng chung cho cả trang (đầu trang, thanh toán, trang tài khoản)
export const customerState = reactive({
  slug: '',
  token: '',
  customer: null as ShopCustomer | null,
  dialog: null as null | 'login' | 'register'
})

function load(slug: string) {
  customerState.slug = slug
  customerState.token = ''
  customerState.customer = null
  try {
    const raw = localStorage.getItem(key(slug))
    if (raw) {
      const v = JSON.parse(raw)
      customerState.token = String(v.token || '')
      customerState.customer = v.customer || null
    }
  } catch {
    /* dữ liệu hỏng -> coi như chưa đăng nhập */
  }
}

function persist() {
  if (!customerState.slug) return
  if (customerState.token) localStorage.setItem(key(customerState.slug), JSON.stringify({ token: customerState.token, customer: customerState.customer }))
  else localStorage.removeItem(key(customerState.slug))
}

function headers(): Record<string, string> {
  const h: Record<string, string> = {}
  if (customerState.token) h['X-Customer-Token'] = customerState.token
  return h
}

export function customerHeaders(slug: string): Record<string, string> {
  useCustomer(slug)
  return headers()
}

export function apiError(err: any, fallback: string): string {
  return err?.response?.data?.error || fallback
}

export function useCustomer(slug: string) {
  if (slug && customerState.slug !== slug) load(slug)

  async function login(login: string, password: string) {
    const { data } = await shopApi.post('/storefront/public/customers/login', { slug, login, password })
    customerState.token = data.token
    customerState.customer = data.customer
    persist()
  }

  async function register(p: { name: string; phone: string; email: string; password: string }) {
    const { data } = await shopApi.post('/storefront/public/customers/register', { slug, ...p })
    customerState.token = data.token
    customerState.customer = data.customer
    persist()
  }

  function logout() {
    customerState.token = ''
    customerState.customer = null
    persist()
  }

  // Kiểm tra lại phiên với máy chủ (token hết hạn -> đăng xuất)
  async function refresh() {
    if (!customerState.token) return
    try {
      const { data } = await shopApi.get('/storefront/public/customers/me', { params: { slug }, headers: headers() })
      customerState.customer = data.customer
      persist()
    } catch (err: any) {
      if (err?.response?.status === 401) logout()
    }
  }

  async function update(p: { name: string; email: string; address: string }) {
    const { data } = await shopApi.put('/storefront/public/customers/me', p, { params: { slug }, headers: headers() })
    customerState.customer = data.customer
    persist()
  }

  async function orders(): Promise<CustomerOrder[]> {
    try {
      const { data } = await shopApi.get('/storefront/public/customers/orders', { params: { slug }, headers: headers() })
      return data.orders || []
    } catch (err: any) {
      if (err?.response?.status === 401) logout()
      throw err
    }
  }

  return { state: customerState, login, register, logout, refresh, update, orders }
}
