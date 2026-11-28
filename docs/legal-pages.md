# Trang pháp lý công khai của ứng dụng (Meta bắt buộc)

Ba trang **công khai, không cần đăng nhập**, nằm ngay trong web của ứng dụng (khác với trang chính sách mẫu trong website bán hàng của khách — xem `site-pages.md`):

| Trang | Đường dẫn | Điền vào đâu trong Meta for Developers |
|---|---|---|
| Chính sách quyền riêng tư | `/privacy` | App settings → Basic → **Privacy Policy URL** |
| Điều khoản dịch vụ | `/terms` | App settings → Basic → **Terms of Service URL** |
| Hướng dẫn xoá dữ liệu | `/data-deletion` | App settings → Basic → **User data deletion** → chọn *Data deletion instructions URL* |

Địa chỉ đầy đủ là tên miền web của ứng dụng + đường dẫn, ví dụ `https://fb-multipage-crm.pages.dev/privacy`.

## Cách hoạt động

- `views/LegalView.vue` + nội dung ở `lib/legalContent.ts`; route `legal-privacy` / `legal-terms` / `legal-deletion` (chỉ có trên web quản trị, không có trên tên miền riêng của khách).
- **Song ngữ Việt / Anh**: `?lang=vi|en`, mặc định theo ngôn ngữ trình duyệt (người duyệt của Meta thường dùng tiếng Anh). Nút chuyển ngôn ngữ ở đầu trang, liên kết giữa 3 trang giữ nguyên ngôn ngữ.
- Router bỏ qua bước hỏi phiên đăng nhập cho các trang này (token hết hạn cũng không bị đẩy về `/login`), không dùng khung quản trị. Nội dung đặt `data-i18n="skip"` và được loại khỏi danh mục dịch giao diện quản trị (`extract-ui-strings.mjs`).
- Trang đăng nhập / đăng ký có dòng liên kết tới cả ba trang (`components/LegalLinks.vue`).

## Phải cấu hình khi triển khai (Cloudflare Pages → Settings → Environment variables, rồi build lại)

| Biến | Ý nghĩa |
|---|---|
| `VITE_SUPPORT_EMAIL` | Email nhận câu hỏi và **yêu cầu xoá dữ liệu** — hiện ở cuối các trang và trong "Cách 3" của trang xoá dữ liệu |
| `VITE_OPERATOR_NAME` | Tên cá nhân / công ty vận hành, hiện trong điều khoản và chân trang |

Chưa đặt thì các trang vẫn hiện nhưng chỉ ghi "liên hệ đơn vị vận hành qua kênh hỗ trợ" (không có email) — **chưa đủ để qua Meta App Review**, hãy đặt trước khi nộp. Không ghi email vào mã nguồn.

## Cần chủ ứng dụng đọc lại

Nội dung được soạn **theo đúng cách ứng dụng đang chạy** (các quyền Facebook thật sự xin: `public_profile`, `email`, `pages_show_list`, `pages_messaging`, `pages_read_engagement`, `pages_manage_metadata`; mật khẩu băm; Page token mã hoá AES-256-GCM; thống kê ẩn danh tự xoá sau 60 / 180 ngày; AI qua Google Gemini / dịch vụ tương thích OpenAI; cổng thanh toán VNPay, PayPal, Stripe), nhưng vẫn là văn bản mẫu: các cam kết như "xoá trong 30 ngày", "báo trước khi thu phí", "không dành cho dưới 16 tuổi", luật áp dụng là Việt Nam là quyết định của người vận hành — sửa trong `legalContent.ts` nếu khác thực tế. Khi sửa ý nghĩa, cập nhật `LEGAL_UPDATED`.

Nếu sau này xin thêm quyền Facebook (ví dụ `pages_manage_posts`) hoặc đổi nhà cung cấp AI / hạ tầng, phải cập nhật mục 3 và 5 của chính sách quyền riêng tư.

## Xoá dữ liệu: thực tế hiện nay

- Gỡ Fanpage (Fanpage → "Gỡ Page") **xoá ngay Page token đã lưu** nhưng **không xoá** hội thoại đã đồng bộ.
- Trang hướng dẫn nêu quy trình "gửi email → xoá trong 30 ngày" cho toàn bộ tài khoản và cho người đã nhắn tin tới Fanpage. Việc xoá toàn bộ tài khoản hiện vẫn **làm thủ công**.
- **Data Deletion Request Callback** (Meta gọi khi người dùng gỡ app và yêu cầu xoá dữ liệu) — `internal/handlers/data_deletion.go`:
  - `POST /api/v1/facebook/data-deletion`: kiểm tra `signed_request` bằng `FACEBOOK_APP_SECRET`, tìm tài khoản đăng nhập Facebook có `provider_id` trùng, xoá Page token + ngắt kết nối các Fanpage của chủ tài khoản đó, gỡ liên kết Facebook và ảnh đại diện; ghi bảng `data_deletion_requests`; trả `{url, confirmation_code}`.
  - `GET /api/v1/facebook/data-deletion/:code`: trang trạng thái song ngữ (HTML, `?format=json` để lấy JSON).
  - Điền trong Meta: App settings → Basic → **User data deletion** → chọn *Data deletion callback URL* = `https://<backend>/api/v1/facebook/data-deletion` (thay cho *instructions URL*; trang `/data-deletion` vẫn giữ để người dùng đọc).
- Có thể làm tiếp: nút "Xoá tài khoản và dữ liệu" trong ứng dụng, xoá khách hàng từ trang Admin.

## Kiểm thử

`scratchpad/e2e/legal_e2e.mjs` (20 kiểm tra): mở không cần đăng nhập, song ngữ, tab, email liên hệ, quyền Facebook nêu đúng, trình duyệt tiếng Việt, bộ dịch quản trị không đụng nội dung, liên kết ở đăng nhập / đăng ký, token hết hạn vẫn xem được.
