// Tải dữ liệu website công khai (dùng chung cho trang chủ, trang sản phẩm, thanh toán, đơn hàng)
import { api } from './api'
import { parseTheme } from './theme'
import { initLocale, localizeBlocks, localizeProducts, parseI18n } from './i18n'
import { startTracking } from './analytics'
import { parseBlockList, type PageKind, type PublicPageRef } from './pages'
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
      i18n_json?: string
      products?: PublicProduct[]
      pages?: PublicPageRef[]
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
        theme: parseTheme(data.theme_json),
        i18n: parseI18n(data.i18n_json),
        pages: Array.isArray(data.pages) ? data.pages : []
      }
    })
    .then((site) => {
      // Chọn thị trường (ngôn ngữ + tiền tệ) cho khách
      initLocale(site.slug, site.i18n)
      startTracking(site.slug) // thống kê: bắt đầu gửi sự kiện đã xếp hàng
      return site
    })
  p.catch(() => cache.delete(key))
  cache.set(key, p)
  return p
}

// Chi tiết một sản phẩm: mô tả đầy đủ + bộ sưu tập ảnh / video (danh sách ở trang chủ chỉ có bản tóm tắt)
export async function loadPublicProduct(slug: string, pid: string | number): Promise<PublicProduct> {
  const { data } = await api.get<{ product: PublicProduct }>(`/storefront/public/products/${pid}`, { params: { slug } })
  return data.product
}

// Một trang của website (giới thiệu, liên hệ, chính sách...): chỉ có section riêng của trang,
// đầu / chân trang dùng chung lấy từ dữ liệu website (PublicShell)
export interface PublicPageData {
  slug: string
  title: string
  kind: PageKind
  blocks: StoreBlock[]
}

export async function loadPublicPage(siteSlug: string, pageSlug: string): Promise<PublicPageData> {
  const { data } = await api.get<{ slug: string; title: string; kind: PageKind; blocks_json?: string }>(
    `/storefront/public/pages/${encodeURIComponent(pageSlug)}`,
    { params: { slug: siteSlug } }
  )
  return { slug: data.slug, title: data.title, kind: data.kind, blocks: parseBlockList(data.blocks_json) }
}

// Các section dùng làm "khung" cho trang con (sản phẩm, giỏ hàng, đơn hàng)
export function shellBlocks(blocks: StoreBlock[]) {
  const enabled = blocks.filter((b) => b.enabled)
  return {
    top: enabled.filter((b) => b.type === 'announcement' || b.type === 'header'),
    bottom: enabled.filter((b) => b.type === 'footer')
  }
}

// Bản đã dịch theo ngôn ngữ đang chọn (nội dung section + tên/mô tả sản phẩm)
export function localizeSite(site: PublicSiteData): PublicSiteData {
  return { ...site, blocks: localizeBlocks(site.blocks), products: localizeProducts(site.products) }
}
