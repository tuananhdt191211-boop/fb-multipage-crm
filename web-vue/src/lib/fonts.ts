// Danh sách font (Google Fonts có hỗ trợ tiếng Việt) dùng cho website bán hàng.
// Phải khớp allowedThemeFonts ở backend (site_theme.go).

export interface FontDef {
  name: string
  group: 'Không chân' | 'Có chân' | 'Viết tay' | 'Hệ thống'
  fallback: 'sans-serif' | 'serif' | 'cursive'
  weights?: string // phần "wght@..." của Google Fonts; bỏ trống = chỉ có 400
  google: boolean
}

export const FONTS: FontDef[] = [
  { name: 'Be Vietnam Pro', group: 'Không chân', fallback: 'sans-serif', weights: '300;400;500;600;700;800', google: true },
  { name: 'Inter', group: 'Không chân', fallback: 'sans-serif', weights: '300;400;500;600;700;800;900', google: true },
  { name: 'Roboto', group: 'Không chân', fallback: 'sans-serif', weights: '300;400;500;700;900', google: true },
  { name: 'Open Sans', group: 'Không chân', fallback: 'sans-serif', weights: '300;400;500;600;700;800', google: true },
  { name: 'Montserrat', group: 'Không chân', fallback: 'sans-serif', weights: '300;400;500;600;700;800;900', google: true },
  { name: 'Nunito', group: 'Không chân', fallback: 'sans-serif', weights: '300;400;600;700;800;900', google: true },
  { name: 'Quicksand', group: 'Không chân', fallback: 'sans-serif', weights: '300;400;500;600;700', google: true },
  { name: 'Lexend', group: 'Không chân', fallback: 'sans-serif', weights: '300;400;500;600;700;800', google: true },
  { name: 'Mulish', group: 'Không chân', fallback: 'sans-serif', weights: '300;400;600;700;800;900', google: true },
  { name: 'Raleway', group: 'Không chân', fallback: 'sans-serif', weights: '300;400;500;600;700;800', google: true },
  { name: 'Josefin Sans', group: 'Không chân', fallback: 'sans-serif', weights: '300;400;600;700', google: true },
  { name: 'Barlow', group: 'Không chân', fallback: 'sans-serif', weights: '300;400;500;600;700;800', google: true },
  { name: 'Oswald', group: 'Không chân', fallback: 'sans-serif', weights: '300;400;500;600;700', google: true },
  { name: 'Noto Sans', group: 'Không chân', fallback: 'sans-serif', weights: '300;400;500;600;700;800', google: true },
  { name: 'Playfair Display', group: 'Có chân', fallback: 'serif', weights: '400;500;600;700;800;900', google: true },
  { name: 'Lora', group: 'Có chân', fallback: 'serif', weights: '400;500;600;700', google: true },
  { name: 'Merriweather', group: 'Có chân', fallback: 'serif', weights: '300;400;700;900', google: true },
  { name: 'Noto Serif', group: 'Có chân', fallback: 'serif', weights: '400;700', google: true },
  { name: 'Cormorant Garamond', group: 'Có chân', fallback: 'serif', weights: '300;400;500;600;700', google: true },
  { name: 'Dancing Script', group: 'Viết tay', fallback: 'cursive', weights: '400;500;600;700', google: true },
  { name: 'Lobster', group: 'Viết tay', fallback: 'cursive', google: true },
  { name: 'Pacifico', group: 'Viết tay', fallback: 'cursive', google: true },
  { name: 'Arial', group: 'Hệ thống', fallback: 'sans-serif', google: false },
  { name: 'Georgia', group: 'Hệ thống', fallback: 'serif', google: false },
  { name: 'Times New Roman', group: 'Hệ thống', fallback: 'serif', google: false }
]

const byName = new Map(FONTS.map((f) => [f.name, f]))

export const fontGroups = (['Không chân', 'Có chân', 'Viết tay', 'Hệ thống'] as const).map((g) => ({
  group: g,
  fonts: FONTS.filter((f) => f.group === g)
}))

export function isKnownFont(name?: string): name is string {
  return !!name && byName.has(name)
}

// Giá trị font-family đầy đủ, vd. "'Playfair Display', serif"
export function fontStack(name?: string): string {
  const f = name ? byName.get(name) : undefined
  return f ? `'${f.name}', ${f.fallback}` : ''
}

// Tải font từ Google Fonts (mỗi font một thẻ <link> để một font lỗi không ảnh hưởng font khác)
export function ensureFonts(names: Iterable<string | undefined>) {
  if (typeof document === 'undefined') return
  for (const n of names) {
    const f = n ? byName.get(n) : undefined
    if (!f || !f.google) continue
    const id = 'pp-font-' + f.name.replace(/\s+/g, '-').toLowerCase()
    if (document.getElementById(id)) continue
    const fam = encodeURIComponent(f.name).replace(/%20/g, '+') + (f.weights ? `:wght@${f.weights}` : '')
    const link = document.createElement('link')
    link.id = id
    link.rel = 'stylesheet'
    link.href = `https://fonts.googleapis.com/css2?family=${fam}&display=swap`
    document.head.appendChild(link)
  }
}
