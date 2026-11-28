// Màu: chọn chữ sáng / tối dễ đọc trên một màu nền (dùng cho đầu trang, banner nền ảnh...)

// "#abc" / "#aabbcc" -> [r, g, b]; định dạng khác (rgb(), tên màu...) -> null
export function parseHex(color: string | undefined | null): [number, number, number] | null {
  const m = /^#?([0-9a-f]{3}|[0-9a-f]{6})$/i.exec(String(color || '').trim())
  if (!m) return null
  let h = m[1]!
  if (h.length === 3) h = [...h].map((c) => c + c).join('')
  const n = parseInt(h, 16)
  return [(n >> 16) & 255, (n >> 8) & 255, n & 255]
}

// Độ sáng tương đối theo WCAG (0 = đen, 1 = trắng)
export function luminance(rgb: [number, number, number]): number {
  const [r, g, b] = rgb.map((v) => {
    const c = v / 255
    return c <= 0.03928 ? c / 12.92 : Math.pow((c + 0.055) / 1.055, 2.4)
  }) as [number, number, number]
  return 0.2126 * r + 0.7152 * g + 0.0722 * b
}

// Chữ trắng trên nền tối, chữ đen xanh trên nền sáng; không đọc được màu thì trả '' (giữ màu mặc định)
export function readableOn(bg: string | undefined | null, light = '#ffffff', dark = '#0f172a'): string {
  const rgb = parseHex(bg)
  if (!rgb) return ''
  return luminance(rgb) < 0.4 ? light : dark
}
