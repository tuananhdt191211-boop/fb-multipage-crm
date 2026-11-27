// Tải dữ liệu website công khai (dùng chung cho trang chủ, trang sản phẩm, thanh toán, đơn hàng)
import { api } from './api'
import { parseTheme } from './theme'
import type { PublicProduct, PublicSiteData, StoreBlock } from '../types/store'

const cache = new Map<string, Promise<PublicSiteData>>()

export function loadPublicSite(params: { slug?: string; domain?: string }): Promise<PublicSiteData> {
  const key = params.slug ? `s:${params.slug}` : `d:${params.domain || ''}`
  const hit = cache.get(key)
  if (hit) return hit
  const p = api
    .get<{
      id: number
      slug: string
      site_title?: string
      primary_color?: string
      logo_url?: string
      blocks_json?: string
      theme_json?: string
      products?: PublicProduct[]
    }>('/storefront/public', { params })
    .then(({ data }) => {
      let blocks: StoreBlock[] = []
      try {
        const parsed = JSON.parse(data?.blocks_json || '[]')
        blocks = Array.isArray(parsed) ? parsed : []
      } catch {
        blocks = []
      }
      return {
        id: data.id,
        slug: data.slug,
        site_title: data.site_title || 'Cửa hàng',
        primary_color: data.primary_color || '#059669',
        logo_url: data.logo_url || '',
        blocks,
        products: data.products || [],
        theme: parseTheme(data.theme_json)
      }
    })
  p.catch(() => cache.delete(key))
  cache.set(key, p)
  return p
}

// Các section dùng làm "khung" cho trang con (sản phẩm, giỏ hàng, đơn hàng)
export function shellBlocks(blocks: StoreBlock[]) {
  const enabled = blocks.filter((b) => b.enabled)
  return {
    top: enabled.filter((b) => b.type === 'announcement' || b.type === 'header'),
    bottom: enabled.filter((b) => b.type === 'footer')
  }
}
