// Menu điều hướng quản lý riêng: website có nhiều menu (vd. "Menu chính", "Menu chân trang"), mỗi menu là danh sách mục
// {label, link, children[]} (tối đa 2 cấp). Đầu trang chọn menu bằng data.menu_id, cột chân trang bằng col.menu_id;
// không chọn thì dùng danh sách tự nhập như cũ (nav_items / col.links).
import { locale, localizeBlocks, type SiteI18n } from './i18n'
import type { StoreBlock } from '../types/store'

export interface MenuItem {
  label: string
  link: string
  children?: MenuItem[]
}

export interface SiteMenu {
  id: string
  name: string
  items: MenuItem[]
}

export const MAX_MENUS = 10

export function parseMenus(raw: string | undefined | null): SiteMenu[] {
  try {
    const v = JSON.parse(raw || '[]')
    if (!Array.isArray(v)) return []
    return v.map((m: any) => ({
      id: String(m?.id || ''),
      name: String(m?.name || 'Menu'),
      items: Array.isArray(m?.items) ? m.items.map(cleanItem) : []
    }))
  } catch {
    return []
  }
}

function cleanItem(x: any): MenuItem {
  const it: MenuItem = { label: String(x?.label || ''), link: String(x?.link || '') }
  if (Array.isArray(x?.children) && x.children.length) it.children = x.children.map((c: any) => ({ label: String(c?.label || ''), link: String(c?.link || '') }))
  return it
}

export function menuById(menus: SiteMenu[] | undefined, id: unknown): SiteMenu | undefined {
  const k = String(id || '')
  return k ? (menus || []).find((m) => m.id === k) : undefined
}

export function newMenuId(): string {
  return 'menu-' + Math.random().toString(36).slice(2, 8)
}

// Bản dịch gắn theo id section, nên mỗi menu được coi như một "section" giả để dùng chung bộ thu thập / áp bản dịch
export function menuPseudoBlock(m: SiteMenu): StoreBlock {
  return { id: 'menu-' + m.id, type: 'menu' as unknown as StoreBlock['type'], enabled: true, data: { items: JSON.parse(JSON.stringify(m.items)) } }
}

export function menuPseudoBlocks(menus: SiteMenu[]): StoreBlock[] {
  return menus.map(menuPseudoBlock)
}

// Menu đã dịch theo ngôn ngữ đang chọn
export function localizeMenus(menus: SiteMenu[], lang = locale.lang, i18n: SiteI18n = locale.i18n): SiteMenu[] {
  if (!menus.length || lang === 'vi') return menus
  const loc = localizeBlocks(menuPseudoBlocks(menus), lang, i18n)
  return menus.map((m, i) => ({ ...m, items: (loc[i]!.data.items as MenuItem[]) || m.items }))
}

// Danh sách phẳng (kèm độ sâu) để hiển thị / sửa trong trình chỉnh sửa
export function flattenMenu(items: MenuItem[]): { item: MenuItem; depth: 0 | 1; parent: MenuItem | null; index: number }[] {
  const out: { item: MenuItem; depth: 0 | 1; parent: MenuItem | null; index: number }[] = []
  items.forEach((it, i) => {
    out.push({ item: it, depth: 0, parent: null, index: i })
    ;(it.children || []).forEach((c, j) => out.push({ item: c, depth: 1, parent: it, index: j }))
  })
  return out
}

// Số mục (cả mục con)
export function menuItemCount(m: SiteMenu): number {
  return m.items.reduce((n, it) => n + 1 + (it.children?.length || 0), 0)
}
