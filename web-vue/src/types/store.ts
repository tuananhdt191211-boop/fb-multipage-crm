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

// Kiểu chữ toàn trang (tab "Giao diện")
export type ThemeTag = 'h1' | 'h2' | 'h3' | 'h4' | 'p'
export interface ThemeTypo {
  font?: string
  size?: number
  size_m?: number
  weight?: '300' | '400' | '500' | '600' | '700' | '800' | '900'
  lh?: number
  ls?: number
  upper?: boolean
  color?: string
}
export interface SiteTheme {
  heading_font?: string
  body_font?: string
  tags?: Partial<Record<ThemeTag, ThemeTypo>>
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
  draft_blocks_json: string // trang chủ "ghép sẵn" (giữ cho bản cũ); nguồn thật: draft_shared_json + các trang
  published_blocks_json: string
  draft_shared_json?: string // dải thông báo / đầu trang / chân trang dùng chung cho mọi trang
  published_shared_json?: string
  draft_theme_json?: string
  published_theme_json?: string
  draft_i18n_json?: string
  published_i18n_json?: string
  is_published: boolean
  created_at: string
  updated_at: string
}

// Nhóm phân loại của sản phẩm (vd. "Màu": Đen, Trắng). Tối đa 3 nhóm.
export interface ProductOption {
  name: string
  values: string[]
}

// Một biến thể trong trình sửa: option_values thẳng hàng với danh sách nhóm (nhóm chưa có giá trị = '')
export interface AdminVariant {
  id?: number
  option_values: string[]
  sku: string
  barcode?: string
  price: number
  compare_price: number
  cost_price: number
  stock: number // -1 = không giới hạn, 0 = hết hàng
  image_url: string
}

// Biến thể máy chủ trả cho chủ shop
export interface AdminVariantRow {
  id: number
  title: string
  option1: string
  option2: string
  option3: string
  sku: string
  barcode: string
  price: number
  compare_price: number
  cost_price: number
  stock: number
  image_url: string
}

// Biến thể công khai (không có giá nhập, số tồn kho cụ thể)
export interface PublicVariant {
  id: number
  title: string
  option_values: string[]
  price: number
  compare_price: number
  in_stock: boolean
  image_url: string
  sku?: string
}

export interface AdminProduct {
  id: number
  storefront_id: number
  name: string
  description: string // HTML đã lọc
  price: number // có biến thể: giá thấp nhất
  compare_price: number
  image_url: string // ảnh bìa
  stock: number // -1 = không giới hạn, 0 = hết hàng; có biến thể: tổng tồn kho
  is_active: boolean
  sort_order: number
  media?: import('../lib/productMedia').ProductMedia[]
  sku?: string
  barcode?: string
  cost_price?: number // giá nhập — chỉ chủ shop thấy
  source_url?: string
  tags?: string
  summary?: string // tóm tắt chữ thuần của mô tả (máy chủ tính)
  options?: ProductOption[]
  variants?: AdminVariantRow[]
  variant_count?: number
  price_max?: number
}

// Dòng nhẹ trong danh sách quản lý sản phẩm (không kèm mô tả đầy đủ)
export interface AdminProductLite {
  id: number
  name: string
  summary: string
  price: number
  compare_price: number
  image_url: string
  stock: number
  is_active: boolean
  sort_order: number
  sku: string
  cost_price: number
  source_url: string
  tags: string
  media_count: number
  has_video: boolean
  variant_count: number
  price_max: number
}

export interface PublicProduct {
  id: number
  name: string
  description?: string // chỉ có ở trang chi tiết (danh sách chỉ có summary)
  summary?: string
  price: number // có biến thể: giá thấp nhất
  price_max?: number // có biến thể: giá cao nhất
  variant_count?: number // số biến thể (0 = sản phẩm thường)
  compare_price: number
  image_url: string
  in_stock: boolean
  media?: import('../lib/productMedia').ProductMedia[]
  sku?: string
  options?: ProductOption[] // chỉ có ở trang chi tiết
  variants?: PublicVariant[] // chỉ có ở trang chi tiết
  tags?: string[] // thẻ của sản phẩm: section Sản phẩm có thể chỉ hiện sản phẩm theo thẻ
}

export interface PublicSiteData {
  id: number
  slug: string
  site_title: string
  primary_color: string
  logo_url: string
  blocks: StoreBlock[]
  products: PublicProduct[]
  theme: SiteTheme
  i18n: import('../lib/i18n').SiteI18n
  pages: import('../lib/pages').PublicPageRef[] // các trang khác trang chủ đang hiện
}

export interface OrderItem {
  product_id: number
  variant_id?: number
  variant_title?: string
  sku?: string
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
  payment_method: 'cod' | 'vnpay' | 'paypal' | 'stripe'
  currency?: string
  foreign_minor?: number
  foreign_decimals?: number
  gateway_txn?: string
  payment_status: 'unpaid' | 'pending' | 'paid' | 'failed'
  status: 'new' | 'confirmed' | 'shipping' | 'completed' | 'cancelled'
  vnp_transaction_no?: string
  vnp_bank_code?: string
  paid_at?: string | null
  created_at: string
}

// Kiểu trượt của danh sách sản phẩm (section "Sản phẩm"); không đặt = lưới
export type CarouselLayout = 'slide' | 'dots' | 'center' | 'marquee'
