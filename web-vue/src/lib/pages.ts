// Nhiều trang cho một website:
// - Dải thông báo, đầu trang (header) và chân trang (footer) là phần DÙNG CHUNG — sửa một lần hiện ở mọi trang.
// - Mỗi trang chỉ giữ các section riêng của nó. Trang chủ là trang kind = 'home' (đường dẫn "/"), trang khác ở /pages/<slug>.
// - Liên kết tới trang khác viết dạng "page:<đường-dẫn>" (vd. page:chinh-sach-doi-tra, page:home) — đổi thành đường dẫn thật khi hiển thị.
import type { StoreBlock } from '../types/store'

export type PageKind = 'home' | 'page' | 'policy'

// Thông tin một trang trong trình chỉnh sửa (không kèm nội dung)
export interface SitePage {
  id: number
  slug: string
  title: string
  kind: PageKind
  position: number
  is_visible: boolean // chủ shop bật / tắt (có hiệu lực công khai sau khi xuất bản)
  published_visible: boolean // bản đã xuất bản có đang hiện không
}

// Trang như máy chủ trả về: kèm nội dung nháp
export interface ServerPage extends SitePage {
  draft_blocks_json: string
  draft_settings_json?: string // SEO + landing page (JSON)
  published_settings_json?: string
}

export function pageMeta(p: ServerPage): SitePage {
  return { id: p.id, slug: p.slug, title: p.title, kind: p.kind, position: p.position, is_visible: p.is_visible, published_visible: p.published_visible }
}

export interface PageTemplate {
  key: string
  title: string
  slug: string
  kind: PageKind
  desc: string
}

// Trang công khai (danh sách ở dữ liệu website)
export interface PublicPageRef {
  slug: string
  title: string
  kind: PageKind
}

const RANK: Record<string, number> = { announcement: 0, header: 1, footer: 2 }

// Tên + mô tả mẫu trang hiện ở hộp "Thêm trang" (viết ở đây, không lấy từ máy chủ, để giao diện quản trị dịch được sang ngôn ngữ khác)
export const templateText: Record<string, { title: string; desc: string }> = {
  blank: { title: 'Trang trắng', desc: 'Bắt đầu từ trang trống, tự thêm các phần bạn muốn.' },
  about: { title: 'Giới thiệu', desc: 'Câu chuyện thương hiệu, cam kết và lời mời liên hệ.' },
  contact: { title: 'Liên hệ', desc: 'Thông tin liên hệ và câu hỏi thường gặp.' },
  return: { title: 'Chính sách đổi trả', desc: 'Điều kiện, thời hạn và cách đổi trả / hoàn tiền.' },
  shipping: { title: 'Chính sách vận chuyển', desc: 'Khu vực giao, thời gian và phí vận chuyển.' },
  privacy: { title: 'Chính sách bảo mật', desc: 'Thu thập, sử dụng và bảo vệ thông tin khách hàng.' },
  terms: { title: 'Điều khoản sử dụng', desc: 'Quy định khi mua hàng và sử dụng website.' },
  payment: { title: 'Chính sách thanh toán', desc: 'Các hình thức thanh toán được chấp nhận.' }
}

export function templateLabel(t: PageTemplate): { title: string; desc: string } {
  return templateText[t.key] ?? { title: t.title, desc: t.desc }
}

export function isSharedType(type: string): boolean {
  return type in RANK
}

// Tách danh sách section "ghép sẵn" thành phần dùng chung (đúng thứ tự thông báo → đầu trang → chân trang) và phần riêng của trang
export function splitBlocks(blocks: StoreBlock[]): { shared: StoreBlock[]; page: StoreBlock[] } {
  const shared: StoreBlock[] = []
  const page: StoreBlock[] = []
  for (const b of blocks) (isSharedType(b.type) ? shared : page).push(b)
  shared.sort((a, b) => RANK[a.type]! - RANK[b.type]!) // sort ổn định: giữ thứ tự cũ khi cùng loại
  return { shared, page }
}

// Ghép lại: [thông báo, đầu trang] + nội dung trang + [chân trang]
export function composeBlocks(shared: StoreBlock[], page: StoreBlock[]): StoreBlock[] {
  return [...shared.filter((b) => RANK[b.type]! < RANK.footer!), ...page, ...shared.filter((b) => RANK[b.type]! >= RANK.footer!)]
}

export function parseBlockList(raw: string | undefined | null): StoreBlock[] {
  try {
    const v = JSON.parse(raw || '[]')
    return Array.isArray(v) ? v : []
  } catch {
    return []
  }
}

export const PAGE_LINK_RE = /^page:([a-z0-9]+(?:-[a-z0-9]+)*)$/i

// Khoá của một trường liên kết (cùng quy tắc với máy chủ: "link", "href", hậu tố "_link")
function isLinkKey(key: string): boolean {
  const k = key.toLowerCase()
  return k === 'link' || k === 'href' || k.endsWith('_link')
}

// Đổi "page:<slug>" thành đường dẫn thật trong mọi trường liên kết (sửa trực tiếp trên bản đã tải, không sao chép)
export function resolvePageLinks<T>(blocks: T, resolve: (slug: string) => string): T {
  const walk = (v: any, key: string): any => {
    if (typeof v === 'string') {
      if (isLinkKey(key)) {
        const m = PAGE_LINK_RE.exec(v.trim())
        if (m) return resolve(m[1]!.toLowerCase())
      }
      return v
    }
    if (Array.isArray(v)) {
      for (let i = 0; i < v.length; i++) v[i] = walk(v[i], key)
      return v
    }
    if (v && typeof v === 'object') {
      for (const k of Object.keys(v)) v[k] = walk(v[k], k)
    }
    return v
  }
  return walk(blocks, '')
}

// Gợi ý liên kết trang trong ô nhập link của trình chỉnh sửa
export function pageLinkOptions(pages: Pick<SitePage, 'slug' | 'title' | 'kind'>[]): { value: string; label: string }[] {
  return pages.map((p) => ({ value: p.kind === 'home' ? 'page:home' : `page:${p.slug}`, label: p.title }))
}

// Đường dẫn công khai của một trang (không gồm tên miền): '' = trang chủ
export function pageSubPath(page: Pick<SitePage, 'slug' | 'kind'>): string {
  return page.kind === 'home' ? '' : `/pages/${page.slug}`
}

// Ghép phần dùng chung + nội dung trang, bỏ phần bị ẩn theo cài đặt trang (landing page: ẩn đầu trang / chân trang)
export function composeForSettings(shared: StoreBlock[], page: StoreBlock[], s: { hide_header?: boolean; hide_footer?: boolean }): StoreBlock[] {
  const use = shared.filter((b) => !(s.hide_header && (b.type === 'announcement' || b.type === 'header')) && !(s.hide_footer && b.type === 'footer'))
  return composeBlocks(use, page)
}
