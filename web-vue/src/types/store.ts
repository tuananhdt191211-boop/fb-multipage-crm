export type BlockType =
  | 'announcement'
  | 'header'
  | 'banner'
  | 'hero'
  | 'image_text'
  | 'products'
  | 'collections'
  | 'features'
  | 'icon_strip'
  | 'testimonials'
  | 'cta'
  | 'richtext'
  | 'image'
  | 'gallery'
  | 'faq'
  | 'contact'
  | 'newsletter'
  | 'footer'
  | 'html'

export interface BlockStyle {
  bg?: string
  pad?: 'none' | 'sm' | 'md' | 'lg'
  css?: string // CSS tuỳ chỉnh, chỉ áp dụng trong section này
}

// Kiểu chữ riêng cho từng đoạn chữ (lưu ở block.data._text["đường.dẫn"])
export interface TextStyle {
  tag?: 'h1' | 'h2' | 'h3' | 'h4' | 'p' | 'div'
  size?: number // px, máy tính
  size_m?: number // px, điện thoại
  color?: string
  weight?: '300' | '400' | '500' | '600' | '700' | '800' | '900'
  italic?: boolean
  underline?: boolean
  upper?: boolean
  align?: 'left' | 'center' | 'right' | 'justify'
  lh?: number // line-height
  ls?: number // letter-spacing (px)
  font?: string
  css?: string // khai báo CSS tự viết thêm
}

export interface TextFx {
  block: string
  key: string
  style?: TextStyle
}

export interface StoreBlock {
  id: string
  type: BlockType
  enabled: boolean
  data: Record<string, any>
}

export interface Site {
  id: number
  slug: string
  domain: string
  domain_status?: string
  domain_error?: string
  site_title: string
  logo_url: string
  primary_color: string
  draft_blocks_json: string
  published_blocks_json: string
  is_published: boolean
  created_at: string
  updated_at: string
}

export interface AdminProduct {
  id: number
  storefront_id: number
  name: string
  description: string
  price: number
  compare_price: number
  image_url: string
  stock: number // -1 = không giới hạn, 0 = hết hàng
  is_active: boolean
  sort_order: number
}

export interface PublicProduct {
  id: number
  name: string
  description: string
  price: number
  compare_price: number
  image_url: string
  in_stock: boolean
}

export interface PublicSiteData {
  id: number
  slug: string
  site_title: string
  primary_color: string
  logo_url: string
  blocks: StoreBlock[]
  products: PublicProduct[]
}

export interface OrderItem {
  product_id: number
  name: string
  image_url: string
  price: number
  qty: number
  line_total: number
}

export interface ShopOrder {
  id: number
  code: string
  customer_name: string
  phone: string
  email: string
  address: string
  note: string
  items: OrderItem[]
  subtotal: number
  shipping_fee: number
  total: number
  payment_method: 'cod' | 'vnpay'
  payment_status: 'unpaid' | 'pending' | 'paid' | 'failed'
  status: 'new' | 'confirmed' | 'shipping' | 'completed' | 'cancelled'
  vnp_transaction_no?: string
  vnp_bank_code?: string
  paid_at?: string | null
  created_at: string
}
