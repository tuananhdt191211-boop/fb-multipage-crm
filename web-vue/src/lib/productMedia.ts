// Tải ảnh / video sản phẩm lên máy chủ và các tiện ích liên quan
import { api } from './api'
import { prepareImage } from './imageCompress'

export interface ProductMedia {
  type: 'image' | 'video' | 'embed'
  url: string
  poster?: string
  alt?: string
}

export interface MediaConfig {
  storage: 'db' | 's3'
  video_upload: boolean
  max_image_mb: number
  max_video_mb: number
  limit: number
  public_base?: string
}

export interface UploadedMedia {
  id: number
  kind: 'image' | 'video'
  url: string
  file_name: string
  size: number
  width: number
  height: number
}

let configCache: Promise<MediaConfig> | null = null
export function fetchMediaConfig(force = false): Promise<MediaConfig> {
  if (!configCache || force) {
    configCache = api
      .get<MediaConfig>('/storefront/media-config')
      .then((r) => r.data)
      .catch((e) => {
        configCache = null
        throw e
      })
  }
  return configCache
}

export async function uploadImageFile(siteId: number | string, file: File, onProgress?: (pct: number) => void): Promise<UploadedMedia> {
  const prepared = await prepareImage(file)
  const fd = new FormData()
  fd.append('file', prepared.file)
  fd.append('width', String(prepared.width))
  fd.append('height', String(prepared.height))
  const { data } = await api.post<{ media: UploadedMedia }>(`/storefront/sites/${siteId}/media`, fd, {
    headers: { 'Content-Type': 'multipart/form-data' },
    timeout: 180000,
    onUploadProgress: (e) => onProgress?.(e.total ? Math.round((e.loaded / e.total) * 100) : 0)
  })
  return data.media
}

export async function uploadVideoFile(siteId: number | string, file: File, onProgress?: (pct: number) => void): Promise<UploadedMedia> {
  const fd = new FormData()
  fd.append('file', file)
  const { data } = await api.post<{ media: UploadedMedia }>(`/storefront/sites/${siteId}/media/video`, fd, {
    headers: { 'Content-Type': 'multipart/form-data' },
    timeout: 15 * 60 * 1000,
    onUploadProgress: (e) => onProgress?.(e.total ? Math.round((e.loaded / e.total) * 100) : 0)
  })
  return data.media
}

// Nhờ máy chủ tải ảnh từ link về thư viện của shop (tối đa 12 ảnh / lần). Trả về map link gốc → link mới (ảnh lỗi bị bỏ).
export async function importImagesToLibrary(siteId: number | string, urls: string[]): Promise<{ map: Record<string, string>; failed: string[] }> {
  const map: Record<string, string> = {}
  const failed: string[] = []
  for (let i = 0; i < urls.length; i += 12) {
    const chunk = urls.slice(i, i + 12)
    try {
      const { data } = await api.post<{ results: { source: string; media?: UploadedMedia; error?: string }[] }>(
        `/storefront/sites/${siteId}/media/import`,
        { urls: chunk },
        { timeout: 120000 }
      )
      for (const r of data.results || []) {
        if (r.media?.url) map[r.source] = r.media.url
        else failed.push(r.source)
      }
    } catch {
      failed.push(...chunk)
    }
  }
  return { map, failed }
}

// Chụp một khung hình làm ảnh bìa cho video vừa chọn (nếu trình duyệt đọc được codec)
export function makeVideoPoster(file: File): Promise<File | null> {
  return new Promise((resolve) => {
    const url = URL.createObjectURL(file)
    const video = document.createElement('video')
    video.muted = true
    video.playsInline = true
    video.preload = 'metadata'
    const done = (f: File | null) => {
      URL.revokeObjectURL(url)
      video.removeAttribute('src')
      video.load()
      resolve(f)
    }
    const timer = window.setTimeout(() => done(null), 8000)
    video.onerror = () => {
      clearTimeout(timer)
      done(null)
    }
    video.onloadeddata = () => {
      try {
        video.currentTime = Math.min(1, (video.duration || 1) / 3)
      } catch {
        clearTimeout(timer)
        done(null)
      }
    }
    video.onseeked = () => {
      clearTimeout(timer)
      try {
        const scale = Math.min(1, 900 / Math.max(video.videoWidth, video.videoHeight, 1))
        const canvas = document.createElement('canvas')
        canvas.width = Math.max(1, Math.round(video.videoWidth * scale))
        canvas.height = Math.max(1, Math.round(video.videoHeight * scale))
        canvas.getContext('2d')!.drawImage(video, 0, 0, canvas.width, canvas.height)
        canvas.toBlob((b) => done(b ? new File([b], file.name.replace(/\.[^.]+$/, '') + '-poster.jpg', { type: 'image/jpeg' }) : null), 'image/jpeg', 0.82)
      } catch {
        done(null)
      }
    }
    video.src = url
  })
}

// Link YouTube / Vimeo / file video? (máy chủ chuẩn hoá lại khi lưu; đây chỉ để giao diện biết loại gì)
export function guessMediaKind(raw: string): 'embed' | 'video' | 'image' {
  const v = raw.trim().toLowerCase()
  if (/(youtube\.com|youtu\.be|youtube-nocookie\.com|vimeo\.com)/.test(v)) return 'embed'
  if (/\.(mp4|webm|mov|m4v)(\?|#|$)/.test(v)) return 'video'
  return 'image'
}

export function youtubePoster(url: string): string {
  const m = /(?:embed\/|v=|youtu\.be\/|shorts\/)([A-Za-z0-9_-]{6,20})/.exec(url)
  return m ? `https://i.ytimg.com/vi/${m[1]}/hqdefault.jpg` : ''
}

// Ảnh đã nằm trong kho của shop (thư viện / R2) hay còn là link ngoài?
export function isOwnMedia(url: string, cfg?: MediaConfig | null): boolean {
  if (url.includes('/api/v1/storefront/media/')) return true
  return !!cfg?.public_base && url.startsWith(cfg.public_base)
}
