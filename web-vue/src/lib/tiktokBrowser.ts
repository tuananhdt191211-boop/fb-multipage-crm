// Gọi TikTok (qua tikwm) ngay từ trình duyệt của người dùng.
// Dùng khi máy chủ Render bị tikwm chặn (IP đám mây): IP điện thoại / máy tính thường không bị chặn.
// Thứ tự thử: gọi thẳng tikwm.com → qua bộ chuyển tiếp /tikwm của chính web (Cloudflare Pages Function).

export interface TikTokTarget {
  kind: 'channel' | 'keyword' | 'channels' | 'single'
  value: string
}

const DIRECT = 'https://www.tikwm.com'
const PROXY = '/tikwm'

let preferred: 'direct' | 'proxy' | null = null
let lastCall = 0

const sleep = (ms: number) => new Promise((r) => setTimeout(r, ms))

function abs(u: unknown): string {
  let s = ''
  if (typeof u === 'string') s = u
  else if (u && typeof u === 'object') {
    const o = u as { url_list?: string[]; url?: string }
    s = o.url_list?.[0] || o.url || ''
  }
  s = s.trim()
  return s.startsWith('/') && !s.startsWith('//') ? DIRECT + s : s
}

const num = (v: unknown) => {
  const n = Number(v)
  return Number.isFinite(n) ? Math.max(0, Math.floor(n)) : 0
}

const cur = (v: unknown) => {
  const s = v == null ? '' : String(v)
  return s === '0' ? '' : s
}

async function callOnce(base: string, path: string, params: Record<string, string>) {
  const wait = 1100 - (Date.now() - lastCall)
  if (wait > 0) await sleep(wait)
  lastCall = Date.now()
  const res = await fetch(`${base}${path}?${new URLSearchParams(params)}`, {
    headers: { Accept: 'application/json' },
    credentials: 'omit'
  })
  const text = await res.text()
  let body: { code?: number; msg?: string; data?: any }
  try {
    body = JSON.parse(text)
  } catch {
    throw new Error(`HTTP ${res.status}`)
  }
  return body
}

async function tikwm(path: string, params: Record<string, string>): Promise<any> {
  const order: ('direct' | 'proxy')[] = preferred === 'proxy' ? ['proxy', 'direct'] : ['direct', 'proxy']
  let lastErr = 'không rõ nguyên nhân'
  for (const way of order) {
    for (let attempt = 0; attempt < 2; attempt++) {
      try {
        const body = await callOnce(way === 'direct' ? DIRECT : PROXY, path, params)
        if (body.code === 0) {
          preferred = way
          return body.data
        }
        lastErr = body.msg || lastErr
        if (/limit/i.test(lastErr) && attempt === 0) {
          await sleep(1500)
          continue
        }
        // tikwm trả lỗi nội dung (vd. không có kênh) → không cần thử đường khác
        throw Object.assign(new Error(lastErr), { final: true })
      } catch (e: any) {
        if (e?.final) throw new Error(`TikTok báo lỗi: ${e.message}`)
        lastErr = e?.message || lastErr
        break // lỗi mạng / CORS / bị chặn → thử đường tiếp theo
      }
    }
  }
  throw new Error(`Không gọi được TikTok từ trình duyệt (${lastErr}). Thử lại sau ít phút.`)
}

function toItem(v: any, fb: { handle?: string; name?: string; avatar?: string } = {}) {
  const id = String(v.video_id || v.id || v.aweme_id || '')
  const handle = v.author?.unique_id || fb.handle || ''
  const name = v.author?.nickname || fb.name || (handle ? '@' + handle : '')
  const photo = Array.isArray(v.images) && v.images.length > 0
  let thumb = abs(v.cover) || abs(v.origin_cover)
  if (!thumb && photo) thumb = abs(v.images[0])
  const seg = photo ? 'photo' : 'video'
  return {
    id,
    platform: 'tiktok' as const,
    kind: (photo ? 'photo' : 'video') as 'photo' | 'video',
    title: String(v.title || '').trim(),
    thumbnail: thumb,
    url: handle && id ? `https://www.tiktok.com/@${handle}/${seg}/${id}` : id ? `https://www.tiktok.com/video/${id}` : '',
    author_name: name,
    author_handle: handle,
    author_avatar: abs(v.author?.avatar) || fb.avatar || '',
    views: num(v.play_count),
    likes: num(v.digg_count),
    comments: num(v.comment_count),
    shares: num(v.share_count),
    duration: num(v.duration),
    published_at: num(v.create_time)
  }
}

export async function exploreTikTokInBrowser(t: TikTokTarget, cursor = '') {
  const res = {
    mode: t.kind,
    platform: 'tiktok' as const,
    query: t.value,
    channel: undefined as any,
    channels: [] as any[],
    items: [] as ReturnType<typeof toItem>[],
    next_cursor: '',
    has_more: false,
    notice: ''
  }

  if (t.kind === 'single') {
    const data = await tikwm('/api/', { url: t.value, hd: '0' })
    const it = toItem(data || {})
    if (!it.id) throw new Error('Không tìm thấy video TikTok này (có thể đã bị xoá hoặc để riêng tư).')
    res.items.push(it)
    if (it.author_handle) {
      res.channel = {
        platform: 'tiktok', id: '', handle: it.author_handle, name: it.author_name, avatar: it.author_avatar,
        url: `https://www.tiktok.com/@${it.author_handle}`, followers: 0, videos: 0
      }
    }
    return res
  }

  if (t.kind === 'channel') {
    const handle = t.value.replace(/^@/, '').replace(/[^a-zA-Z0-9._]/g, '')
    if (!handle) throw new Error('Tên kênh TikTok không hợp lệ.')
    res.query = handle
    const ch = {
      platform: 'tiktok', id: '', handle, name: '@' + handle, avatar: '',
      url: `https://www.tiktok.com/@${handle}`, followers: 0, videos: 0
    }
    if (!cursor) {
      try {
        const info = await tikwm('/api/user/info', { unique_id: handle })
        ch.id = String(info?.user?.id || '')
        ch.name = info?.user?.nickname || ch.name
        ch.avatar = abs(info?.user?.avatarMedium) || abs(info?.user?.avatarThumb)
        ch.followers = num(info?.stats?.followerCount)
        ch.videos = num(info?.stats?.videoCount)
      } catch (e: any) {
        if (/user/i.test(e?.message || '')) throw new Error(`Không tìm thấy kênh TikTok @${handle}.`)
      }
    }
    res.channel = ch
    const data = await tikwm('/api/user/posts', { unique_id: handle, count: '30', cursor: cursor || '0' })
    for (const v of data?.videos || []) {
      const it = toItem(v, { handle, name: ch.name, avatar: ch.avatar })
      if (it.id) res.items.push(it)
    }
    res.next_cursor = cur(data?.cursor)
    res.has_more = Boolean(data?.hasMore) && !!res.next_cursor && res.next_cursor !== cursor
    if (!res.items.length && !cursor) res.notice = 'Kênh này chưa có video công khai, hoặc đang để riêng tư.'
    return res
  }

  if (t.kind === 'channels') {
    const data = await tikwm('/api/user/search', { keywords: t.value, count: '20', cursor: cursor || '0' })
    for (const u of data?.user_list || []) {
      const ui = u?.user_info || {}
      if (!ui.unique_id) continue
      res.channels.push({
        platform: 'tiktok', id: String(ui.uid || ''), handle: ui.unique_id, name: ui.nickname || '@' + ui.unique_id,
        avatar: abs(ui.avatar_medium) || abs(ui.avatar_thumb), url: `https://www.tiktok.com/@${ui.unique_id}`,
        followers: num(ui.follower_count), videos: num(ui.aweme_count)
      })
    }
    res.next_cursor = cur(data?.cursor)
    res.has_more = Boolean(data?.hasMore) && !!res.next_cursor && res.next_cursor !== cursor
    if (!res.channels.length && !cursor) res.notice = 'Không tìm thấy kênh TikTok nào khớp tên này.'
    return res
  }

  const data = await tikwm('/api/feed/search', { keywords: t.value, count: '30', cursor: cursor || '0', HD: '0' })
  for (const v of data?.videos || []) {
    const it = toItem(v)
    if (it.id) res.items.push(it)
  }
  res.next_cursor = cur(data?.cursor)
  res.has_more = Boolean(data?.hasMore) && !!res.next_cursor && res.next_cursor !== cursor
  if (!res.items.length && !cursor) {
    res.notice = 'TikTok không trả về video nào cho từ khoá này. Thử từ khoá ngắn hơn hoặc không dấu.'
  }
  return res
}
