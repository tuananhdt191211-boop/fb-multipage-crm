// Cloudflare Pages Function: chuyển tiếp các API TikTok (tikwm) cần cho mục "Quét kênh & từ khoá".
// Dùng khi trình duyệt không gọi thẳng được tikwm.com (CORS) và máy chủ Render bị chặn.
// Chỉ cho phép đúng các đường dẫn dưới đây để không thành proxy mở.
const ALLOWED = new Set(['api', 'api/feed/search', 'api/user/info', 'api/user/posts', 'api/user/search'])

export async function onRequestGet({ request, params }) {
  const raw = params.path
  const path = (Array.isArray(raw) ? raw : [raw || '']).filter(Boolean).join('/')
  if (!ALLOWED.has(path)) {
    return Response.json({ code: -1, msg: 'Đường dẫn không được phép' }, { status: 404 })
  }
  const target = `https://www.tikwm.com/${path}${path === 'api' ? '/' : ''}${new URL(request.url).search}`
  try {
    const upstream = await fetch(target, {
      headers: {
        'User-Agent': request.headers.get('User-Agent') || 'Mozilla/5.0',
        Accept: 'application/json',
        'Accept-Language': 'vi-VN,vi;q=0.9,en;q=0.8',
        Referer: 'https://www.tikwm.com/'
      },
      cf: { cacheTtl: 60, cacheEverything: true }
    })
    return new Response(await upstream.text(), {
      status: upstream.status,
      headers: { 'Content-Type': 'application/json; charset=utf-8', 'Cache-Control': 'public, max-age=60' }
    })
  } catch (err) {
    return Response.json({ code: -1, msg: 'Không kết nối được tikwm: ' + (err && err.message) }, { status: 502 })
  }
}
