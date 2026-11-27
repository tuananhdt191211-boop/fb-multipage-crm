import type { BlockType, StoreBlock } from '../types/store'

export interface BlockMeta {
  type: BlockType
  label: string
  desc: string
  group: 'Đầu & chân trang' | 'Nội dung' | 'Bán hàng' | 'Khác'
}

export const blockLabels: Record<string, string> = {
  announcement: 'Dải thông báo',
  header: 'Đầu trang',
  banner: 'Banner ảnh trượt',
  hero: 'Banner chữ + ảnh',
  image_text: 'Ảnh + chữ',
  products: 'Sản phẩm',
  collections: 'Ô danh mục',
  features: 'Cam kết dịch vụ',
  icon_strip: 'Dải biểu tượng',
  testimonials: 'Đánh giá khách hàng',
  cta: 'Kêu gọi hành động',
  richtext: 'Văn bản',
  image: 'Ảnh lớn',
  gallery: 'Thư viện ảnh',
  faq: 'Câu hỏi thường gặp',
  contact: 'Liên hệ',
  newsletter: 'Đăng ký email',
  footer: 'Chân trang',
  html: 'HTML tự viết'
}

// Danh sách hiện trong thư viện "Thêm section"
export const blockCatalog: BlockMeta[] = [
  { type: 'announcement', label: 'Dải thông báo', desc: 'Dòng chữ nhỏ trên cùng: miễn phí ship, khuyến mãi', group: 'Đầu & chân trang' },
  { type: 'header', label: 'Đầu trang', desc: 'Logo, menu, tìm kiếm, giỏ hàng', group: 'Đầu & chân trang' },
  { type: 'footer', label: 'Chân trang', desc: 'Thông tin công ty, cột liên kết, mạng xã hội', group: 'Đầu & chân trang' },
  { type: 'banner', label: 'Banner ảnh trượt', desc: 'Ảnh lớn tràn màn hình, nhiều ảnh tự trượt', group: 'Nội dung' },
  { type: 'hero', label: 'Banner chữ + ảnh', desc: 'Tiêu đề lớn, mô tả, nút và ảnh', group: 'Nội dung' },
  { type: 'image_text', label: 'Ảnh + chữ', desc: 'Ảnh một bên, chữ và các điểm nổi bật bên kia', group: 'Nội dung' },
  { type: 'richtext', label: 'Văn bản', desc: 'Tiêu đề và đoạn chữ tự do', group: 'Nội dung' },
  { type: 'image', label: 'Ảnh lớn', desc: 'Một ảnh, có thể gắn link', group: 'Nội dung' },
  { type: 'gallery', label: 'Thư viện ảnh', desc: 'Lưới nhiều ảnh kèm chú thích', group: 'Nội dung' },
  { type: 'products', label: 'Sản phẩm', desc: 'Lưới sản phẩm, thêm vào giỏ hàng', group: 'Bán hàng' },
  { type: 'collections', label: 'Ô danh mục', desc: 'Các ô ảnh dẫn tới nhóm sản phẩm', group: 'Bán hàng' },
  { type: 'icon_strip', label: 'Dải biểu tượng', desc: 'Giao hàng, đổi trả, hỗ trợ… kèm biểu tượng', group: 'Bán hàng' },
  { type: 'features', label: 'Cam kết dịch vụ', desc: 'Các điểm mạnh ngắn gọn', group: 'Bán hàng' },
  { type: 'testimonials', label: 'Đánh giá khách hàng', desc: 'Lời khen từ khách, dạng lưới hoặc trượt', group: 'Bán hàng' },
  { type: 'cta', label: 'Kêu gọi hành động', desc: 'Khung màu nổi bật với nút bấm', group: 'Bán hàng' },
  { type: 'faq', label: 'Câu hỏi thường gặp', desc: 'Hỏi đáp, bấm để mở ra', group: 'Khác' },
  { type: 'contact', label: 'Liên hệ', desc: 'Số điện thoại, email, địa chỉ, Zalo', group: 'Khác' },
  { type: 'newsletter', label: 'Đăng ký email', desc: 'Ô nhập email nhận ưu đãi', group: 'Khác' },
  { type: 'html', label: 'HTML tự viết', desc: 'Nhúng mã HTML/JS (khung cách ly an toàn)', group: 'Khác' }
]

export const iconChoices: { key: string; label: string }[] = [
  { key: 'truck', label: 'Xe giao hàng' },
  { key: 'refresh', label: 'Đổi trả' },
  { key: 'shield', label: 'Bảo đảm' },
  { key: 'headset', label: 'Hỗ trợ' },
  { key: 'gift', label: 'Quà tặng' },
  { key: 'leaf', label: 'Tự nhiên' },
  { key: 'star', label: 'Ngôi sao' },
  { key: 'clock', label: 'Thời gian' },
  { key: 'card', label: 'Thanh toán' },
  { key: 'heart', label: 'Trái tim' }
]

const defaults: Record<BlockType, () => Record<string, any>> = {
  announcement: () => ({ text: 'Miễn phí giao hàng cho đơn từ 500.000đ', link: '' }),
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
    show_search: false,
    show_cart: false,
    sticky: false,
    border: true,
    text_color: ''
  }),
  banner: () => ({
    height: 'lg',
    autoplay: 5,
    overlay: true,
    slides: [{ image_url: '', mobile_image_url: '', heading: 'Tiêu đề banner', text: 'Mô tả ngắn', button_label: 'Mua ngay', link: '#products' }]
  }),
  hero: () => ({
    badge: '',
    title: 'Tiêu đề chính',
    subtitle: 'Mô tả ngắn về cửa hàng của bạn.',
    image_url: '',
    cta_label: 'Khám phá ngay',
    cta_link: '#products'
  }),
  image_text: () => ({
    image_url: '',
    image_side: 'left',
    eyebrow: '',
    heading: 'Câu chuyện thương hiệu',
    body: 'Giới thiệu ngắn về sản phẩm hoặc thương hiệu của bạn.',
    button_label: 'Mua ngay',
    button_link: '#products',
    points: [
      { title: 'Điểm nổi bật 1', text: 'Mô tả ngắn' },
      { title: 'Điểm nổi bật 2', text: 'Mô tả ngắn' }
    ]
  }),
  products: () => ({ heading: 'Sản phẩm nổi bật', source: 'catalog', items: [], show_buy: true, buy_mode: 'cart', buy_label: 'Thêm vào giỏ' }),
  collections: () => ({
    heading: 'Mua theo danh mục',
    items: [
      { image_url: '', title: 'Danh mục 1', link: '#products' },
      { image_url: '', title: 'Danh mục 2', link: '#products' }
    ]
  }),
  features: () => ({ heading: 'Cam kết dịch vụ', items: ['Chính hãng', 'Giao nhanh', 'Đổi trả dễ dàng'] }),
  icon_strip: () => ({
    items: [
      { icon: 'truck', title: 'Giao hàng nhanh', text: 'Toàn quốc 2–4 ngày' },
      { icon: 'refresh', title: 'Đổi trả dễ dàng', text: 'Trong 30 ngày' },
      { icon: 'headset', title: 'Hỗ trợ tận tình', text: '8:00 – 21:00 mỗi ngày' }
    ]
  }),
  testimonials: () => ({
    heading: 'Khách hàng nói gì',
    layout: 'grid',
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
  faq: () => ({ heading: 'Câu hỏi thường gặp', items: [{ q: 'Giao hàng mất bao lâu?', a: 'Từ 2 đến 4 ngày tùy khu vực.' }] }),
  contact: () => ({ heading: 'Liên hệ', phone: '', email: '', address: '', zalo_link: '' }),
  newsletter: () => ({
    heading: 'Nhận ưu đãi qua email',
    text: 'Đăng ký để nhận mã giảm giá và tin mới nhất.',
    button_label: 'Đăng ký',
    placeholder: 'Email của bạn'
  }),
  footer: () => ({
    company: 'Tên cửa hàng',
    about_text: 'Giới thiệu ngắn về cửa hàng.',
    address: '',
    phone: '',
    email: '',
    columns: [
      { title: 'Về chúng tôi', links: [{ label: 'Giới thiệu', link: '#' }, { label: 'Liên hệ', link: '#contact' }] },
      { title: 'Chính sách', links: [{ label: 'Giao hàng & đổi trả', link: '#' }, { label: 'Bảo mật', link: '#' }] }
    ],
    socials: { facebook_link: '', instagram_link: '', tiktok_link: '', youtube_link: '', zalo_link: '' },
    copyright: '© Bản quyền thuộc về cửa hàng.'
  }),
  html: () => ({ html: '<p style="font-family:sans-serif">Nhập mã HTML tại đây</p>', height: 300 })
}

export function newBlock(type: BlockType): StoreBlock {
  const rand = Math.random().toString(36).slice(2, 7)
  return { id: `${type}-${Date.now().toString(36)}${rand}`, type, enabled: true, data: defaults[type]() }
}
