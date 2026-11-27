// Thu nhỏ & nén ảnh ngay trên trình duyệt trước khi tải lên (giữ nguyên GIF để không mất chuyển động)
export interface PreparedImage {
  file: File
  width: number
  height: number
}

const MAX_SIDE = 1920

function loadImage(file: File): Promise<HTMLImageElement> {
  return new Promise((resolve, reject) => {
    const url = URL.createObjectURL(file)
    const img = new Image()
    img.onload = () => {
      URL.revokeObjectURL(url)
      resolve(img)
    }
    img.onerror = () => {
      URL.revokeObjectURL(url)
      reject(new Error('Không đọc được ảnh'))
    }
    img.src = url
  })
}

export async function prepareImage(file: File): Promise<PreparedImage> {
  if (!file.type.startsWith('image/') || file.type === 'image/svg+xml') {
    throw new Error('Chỉ nhận ảnh JPG, PNG, GIF hoặc WEBP')
  }
  const img = await loadImage(file)
  let { naturalWidth: w, naturalHeight: h } = img

  if (file.type === 'image/gif') return { file, width: w, height: h }

  const scale = Math.min(1, MAX_SIDE / Math.max(w, h))
  // Ảnh nhỏ và nhẹ thì giữ nguyên
  if (scale === 1 && file.size < 400 * 1024) return { file, width: w, height: h }

  w = Math.round(w * scale)
  h = Math.round(h * scale)
  const canvas = document.createElement('canvas')
  canvas.width = w
  canvas.height = h
  const ctx = canvas.getContext('2d')
  if (!ctx) return { file, width: img.naturalWidth, height: img.naturalHeight }
  ctx.drawImage(img, 0, 0, w, h)

  const blob: Blob | null = await new Promise((r) => canvas.toBlob(r, 'image/webp', 0.85))
  if (!blob || blob.size >= file.size) return { file, width: img.naturalWidth, height: img.naturalHeight }

  const name = file.name.replace(/\.[^.]+$/, '') + '.webp'
  return { file: new File([blob], name, { type: 'image/webp' }), width: w, height: h }
}
