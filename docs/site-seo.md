# Website nhiều trang — giai đoạn 2: menu, SEO, sitemap, landing page, AI viết SEO

Tiếp nối `site-pages.md`. Thêm bốn thứ cho website bán hàng của khách: **menu quản lý riêng**, **SEO từng trang**, **sitemap / robots.txt**, **landing page** (ẩn đầu / chân trang), và **AI viết SEO**.

## Menu quản lý riêng

- Tab **Menu** trong trình chỉnh sửa: tạo nhiều menu (tối đa 10), mỗi menu tối đa 40 mục, mỗi mục tối đa 12 mục con (2 cấp). Thêm nhanh một trang vào menu, thêm liên kết tuỳ ý (`#products`, `page:<trang>`, `https://...`), lên / xuống, thụt vào / đưa ra.
- **Đầu trang** chọn menu ở cài đặt (`data.menu_id`), mỗi **cột chân trang** chọn menu riêng (`col.menu_id`). Không chọn thì dùng danh sách tự nhập như cũ (không cần chuyển đổi dữ liệu). Nút "Tạo menu từ danh sách này" / "Chuyển thành menu dùng chung" đổi danh sách tự nhập hiện có thành menu một lần bấm.
- Hiển thị: menu con thả xuống khi rê chuột / bấm Tab (máy tính), thụt vào dưới mục cha (điện thoại, chân trang). Menu bị xoá thì section tự quay về danh sách tự nhập.
- Lưu ở `storefronts.{draft,published}_menus_json`; máy chủ làm sạch (`sanitizeMenusJSON`): id duy nhất, nhãn gọn, link an toàn, tối đa 2 cấp. **Bản dịch**: mỗi menu được coi như một "section giả" `menu-<id>` nên tab Ngôn ngữ và "Xuất bản" tự dịch nhãn menu như mọi chữ khác.

## SEO từng trang

Tab **SEO** (theo trang đang sửa): xem trước kết quả Google, đếm ký tự (tiêu đề ≤ 60, mô tả ≤ 155), ảnh chia sẻ (Facebook / Zalo), `noindex`. Cài đặt lưu trong `storefront_pages.{draft,published}_settings_json`
(`seo_title`, `seo_description`, `seo_image`, `noindex`, `hide_header`, `hide_footer`); SEO mặc định của cả website ở `storefronts.{draft,published}_seo_json` (`description`, `image`, `noindex` = ẩn cả website).

Quy tắc ra tiêu đề / mô tả (máy chủ, `seoFor`):

| | Tiêu đề | Mô tả |
|---|---|---|
| Trang chủ | `seo_title` hoặc tên website | `seo_description` → mô tả mặc định của website → tự rút từ đoạn chữ đầu tiên đáng kể (phụ đề hero, văn bản…, ≤ 155 ký tự) |
| Trang khác | `seo_title` hoặc `<tên trang> – <tên website>` | như trên |
| Sản phẩm `/p/<id>` | `<tên sản phẩm> – <tên website>` | tóm tắt mô tả sản phẩm; kèm dữ liệu **Product** (giá, còn hàng, SKU, ảnh) cho JSON-LD |
| Giỏ hàng, thanh toán, tài khoản, đơn hàng, trang không có | tên website | — luôn `noindex` |

Ảnh chia sẻ: ảnh của trang → ảnh mặc định của website → logo. Canonical dùng tên miền riêng khi đã kết nối xong, không thì nơi đang truy cập. Trang `noindex`, trang ẩn, website `noindex` không có trong sitemap.

**Chấm điểm SEO (không cần AI)** — `lib/seo.ts#auditPage`: độ dài tiêu đề / mô tả, có tiêu đề chính, đủ nội dung (≥ 150 từ), ảnh có mô tả (alt), có ảnh chia sẻ, noindex, trùng tiêu đề với trang khác.

## Vì sao có Cloudflare Function (chèn SEO vào HTML)

Web là ứng dụng JavaScript một trang, nên bot **không chạy JavaScript** (Facebook, Zalo, Messenger xem trước link, nhiều bot khác) chỉ thấy `index.html` trống. `functions/_middleware.js` hỏi `GET /storefront/public/seo` rồi chèn `<title>`, `<meta description>`, Open Graph, Twitter, canonical, robots và JSON-LD (Organization + WebSite ở trang chủ, Product ở trang sản phẩm) thẳng vào HTML trước khi trả về. Dùng chung hàm dựng thẻ `src/lib/seoTags.ts` với trình duyệt (`lib/seoHead.ts` cập nhật khi khách chuyển trang).

- An toàn: chỉ chạm `/s/<slug>/…` và tên miền riêng của khách (không chạm trang quản trị / tệp tĩnh / POST); API chậm quá 1,8 giây hoặc lỗi thì trả nguyên HTML gốc (Render miễn phí ngủ đông cũng không làm trang chậm); thẻ gắn `data-pp-seo` để trình duyệt thay lại.
- Biến môi trường (Cloudflare Pages, không bắt buộc): `API_BASE_URL` (mặc định backend trên Render), `APP_HOSTS` (nếu bản quản trị chạy ở tên miền riêng của bạn: liệt kê để không bị coi là tên miền của khách).
- `public/_routes.json` loại `/assets/*` khỏi Function để tiết kiệm lượt gọi.

## Sitemap và robots.txt

- `GET /api/v1/storefront/public/sitemap.xml?slug=|domain=&origin=&prefix=` (máy chủ): trang chủ, trang đang hiện không noindex, sản phẩm đang bán (tối đa 2000), có `lastmod`. `origin` / `prefix` được kiểm tra chặt (không phản chiếu chuỗi tuỳ ý vào XML).
- `/s/<slug>/sitemap.xml` (web của hệ thống) và `/sitemap.xml` (tên miền riêng) do Function gọi máy chủ. Gửi địa chỉ này vào **Google Search Console → Sơ đồ trang web**.
- `/robots.txt`: chặn giỏ hàng / thanh toán / tài khoản / đơn hàng; tên miền riêng có thêm dòng `Sitemap:`. (Trước đây `/robots.txt` rơi vào `index.html`.)

## Landing page

Cài đặt trang: **ẩn dải thông báo + đầu trang** và / hoặc **ẩn chân trang** riêng cho trang đó (kể cả trang chủ). Phần dùng chung vẫn còn nguyên ở các trang khác, kể cả trang sản phẩm / thanh toán khi trang chủ là landing.

- Canvas không vẽ phần bị ẩn (chỉ số trên canvas được ánh xạ về chỉ số thật), danh sách section gắn nhãn "Ẩn ở trang này".
- Máy chủ trả thêm `shared_json`, `home_blocks_json`, `home_settings_json` để giao diện tự ghép; cột cũ `published_blocks_json` (cho bản giao diện cũ) đã áp cờ landing, còn cột nháp cũ luôn ghép **đủ** để trình chỉnh sửa bản cũ lưu lại không làm mất đầu / chân trang.

## AI viết SEO

`POST /api/v1/storefront/sites/:id/seo/suggest {title, content, keywords, is_home}` → `{title, description, keywords[]}`. Dùng cùng khoá AI của hệ thống như AI dịch (khách dùng chung khoá của admin).

- Giao diện: nút "Viết tiêu đề + mô tả cho trang này" (có thể nhập từ khoá chính; xem kết quả rồi chọn *Dùng cả hai / chỉ tiêu đề / chỉ mô tả*) và "Viết SEO cho mọi trang còn thiếu" (chỉ điền ô trống, không ghi đè phần đã có, bỏ qua trang chưa có nội dung).
- Nội dung gửi AI là **văn bản của trang kể cả phần chưa lưu** (tối đa 6000 ký tự), không gửi mã / liên kết. Lời nhắn yêu cầu: tiêu đề ≤ 60, mô tả 110–155 ký tự, đúng ngôn ngữ của trang, **không bịa giá / khuyến mãi / chứng nhận**. Kết quả được máy chủ làm sạch (bỏ thẻ HTML, ngoặc kép, cắt đúng độ dài tại ranh giới từ).
- Giới hạn **40 lượt / giờ / khách hàng** (`429` mã `rate_limited`); chưa cấu hình AI trả `503`. Người dùng vẫn phải đọc lại trước khi lưu.

## Chưa làm

SEO riêng cho từng sản phẩm (hiện tự lấy từ tên + mô tả), dịch tiêu đề / mô tả SEO sang ngôn ngữ khác và thẻ `hreflang`, trang danh mục, AI viết mô tả cho ảnh (alt), chuyển hướng 301 khi đổi đường dẫn trang, tự gửi sitemap cho Google.

## Kiểm thử

- `go test ./internal/handlers -run 'Menu|SEO|Sitemap|Landing'` (menu, cài đặt + landing, quy tắc SEO, sitemap, AI với máy chủ AI giả, giới hạn lượt).
- `node scripts/test-seo-functions.mjs` (12 kiểm tra Function: chèn SEO, tên miền riêng, API chậm / lỗi, sitemap, robots).
- E2E trình duyệt `scratchpad/e2e/seo_e2e.mjs` (69 kiểm tra) — cần `fake_ai.mjs` có nhánh "expert SEO copywriter".
