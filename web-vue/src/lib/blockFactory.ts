import type { BlockType, StoreBlock } from '../types/store'

export interface BlockMeta {
  type: BlockType
  label: string
  desc: string
}

export const blockLabels: Record<string, string> = {
  header: 'Đầu trang',
  hero: 'Banner chính',
  products: 'Sản phẩm',
  features: 'Cam kết dịch vụ',
  testimonials: 'Đánh giá khách hàng',
  cta: 'Kêu gọi hành động & chân trang',
  richtext: 'Văn bản',
  image: 'Ảnh lớn',
  gallery: 'Thư viện ảnh',
  faq: 'Câu hỏi thường gặp',
  contact: 'Liên hệ',
  html: 'HTML tự viết'
}

// Danh sách hiện trong cửa sổ "Thêm section"
export const blockCatalog: BlockMeta[] = [
  { type: 'hero', label: 'Banner chính', desc: 'Tiêu đề lớn, mô tả, nút và ảnh' },
  { type: 'richtext', label: 'Văn bản', desc: 'Tiêu đề và đoạn chữ tự do' },
  { type: 'image', label: 'Ảnh lớn', desc: 'Một ảnh banner, có thể gắn link' },
  { type: 'gallery', label: 'Thư viện ảnh', desc: 'Lưới nhiều ảnh kèm chú thích' },
  { type: 'products', label: 'Sản phẩm', desc: 'Lưới sản phẩm của shop' },
  { type: 'features', label: 'Cam kết dịch vụ', desc: 'Các điểm mạnh ngắn gọn' },
  { type: 'testimonials', label: 'Đánh giá khách hàng', desc: 'Lời khen từ khách đã mua' },
  { type: 'faq', label: 'Câu hỏi thường gặp', desc: 'Hỏi đáp, bấm để mở ra' },
  { type: 'contact', label: 'Liên hệ', desc: 'Số điện thoại, email, địa chỉ, Zalo' },
  { type: 'cta', label: 'Kêu gọi hành động', desc: 'Lời kêu gọi và chân trang' },
  { type: 'html', label: 'HTML tự viết', desc: 'Nhúng mã HTML/JS (chạy trong khung cách ly an toàn)' },
  { type: 'header', label: 'Đầu trang', desc: 'Tên thương hiệu, menu, nút' }
]

const defaults: Record<BlockType, () => Record<string, any>> = {
header: () => ({
    brand_name: 'Tên thương hiệu',
    show_brand: true,
    logo_url: '',
    logo_height: 40,
    brand_size: 'md',
    layout: 'left',
    nav_items: [
      { label: 'Trang chủ', link: '#' },
      { label: 'Sản phẩm', link: '#products' },
      { label: 'Liên hệ', link: '#contact' }
    ],
    show_cta: true,
    cta_label: 'Đặt mua ngay',
    cta_link: '#products',
    sticky: false,
    border: true,
    text_color: ''
  }),
  hero: () => ({
    badge: '',
    title: 'Tiêu đề chính',
    subtitle: 'Mô tả ngắn về cửa hàng của bạn.',
    image_url: '',
    cta_label: 'Khám phá ngay',
    cta_link: '#products'
  }),
  products: () => ({ heading: 'Sản phẩm nổi bật', source: 'catalog', items: [] }),
  features: () => ({ heading: 'Cam kết dịch vụ', items: ['Chính hãng', 'Giao nhanh', 'Đổi trả dễ dàng'] }),
  testimonials: () => ({
    heading: 'Khách hàng nói gì',
    items: [{ name: 'Khách hàng', quote: 'Sản phẩm rất tốt, giao hàng nhanh!' }]
  }),
  cta: () => ({
    title: 'Sẵn sàng đặt hàng?',
    subtitle: 'Liên hệ ngay để được tư vấn.',
    button_text: 'Liên hệ ngay',
    button_link: '',
    footer_note: ''
  }),
  richtext: () => ({ heading: 'Tiêu đề mục', body: 'Nhập nội dung của bạn tại đây.', align: 'left' }),
  image: () => ({ image_url: '', alt: '', link: '' }),
  gallery: () => ({ heading: 'Thư viện ảnh', items: [{ image_url: '', caption: '' }] }),
  faq: () => ({
    heading: 'Câu hỏi thường gặp',
    items: [{ q: 'Giao hàng mất bao lâu?', a: 'Từ 2 đến 4 ngày tùy khu vực.' }]
  }),
  contact: () => ({ heading: 'Liên hệ', phone: '', email: '', address: '', zalo_link: '' }),
  html: () => ({ html: '<p style="font-family:sans-serif">Nhập mã HTML tại đây</p>', height: 300 })
}

export function newBlock(type: BlockType): StoreBlock {
  const rand = Math.random().toString(36).slice(2, 7)
  return { id: `${type}-${Date.now().toString(36)}${rand}`, type, enabled: true, data: defaults[type]() }
}