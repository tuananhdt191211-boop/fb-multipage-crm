export type BlockType =
  | 'header'
  | 'hero'
  | 'products'
  | 'features'
  | 'testimonials'
  | 'cta'
  | 'richtext'
  | 'image'
  | 'gallery'
  | 'faq'
  | 'contact'
  | 'html'

export interface BlockStyle {
  bg?: string
  pad?: 'none' | 'sm' | 'md' | 'lg'
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