# Website nhiều trang (giai đoạn 1)

Mỗi website có **nhiều trang** (Trang chủ, Giới thiệu, Liên hệ, các trang chính sách...). **Dải thông báo, đầu trang (header) và
chân trang (footer) dùng chung** cho mọi trang: sửa một lần, hiện ở tất cả các trang.

## Mô hình dữ liệu

| Nơi lưu | Nội dung |
|---|---|
| `storefronts.draft_shared_json` / `published_shared_json` | Phần dùng chung: dải thông báo, đầu trang, chân trang (đúng thứ tự thông báo → đầu trang → chân trang) |
| bảng `storefront_pages` | Mỗi dòng một trang: `slug`, `title`, `kind` (`home` / `page` / `policy`), `position`, `is_visible`, nội dung **riêng** của trang ở `draft_blocks_json` / `published_blocks_json` |
| `published_title`, `published_slug`, `published_visible` | Tên / đường dẫn / trạng thái đã xuất bản của trang (bản nháp có thể đã đổi, khách chỉ thấy bản đã xuất bản) |
| `storefronts.draft_blocks_json` / `published_blocks_json` | **Trang chủ "ghép sẵn"** (phần chung + trang chủ). Giữ để giao diện / API bản cũ vẫn chạy; luôn được tính lại khi lưu / xuất bản |

Quy tắc:

- Nội dung riêng của một trang **không bao giờ** chứa `announcement` / `header` / `footer` (máy chủ tự loại khi lưu).
- Mỗi website đúng **một trang chủ** (`kind = home`, đường dẫn `home`): luôn hiện, không xoá, không đổi đường dẫn.
- Tối đa **30 trang** mỗi website (`maxPagesPerSite`). Đường dẫn trang: chữ thường không dấu, số, dấu `-`, tối đa 60 ký tự, duy nhất trong website; `home` dành riêng.
- Trang đang ẩn (`is_visible = false`) hoặc chưa xuất bản thì khách không mở được (`404`). Ẩn / hiện / đổi tên / đổi đường dẫn có hiệu lực công khai **sau khi bấm Xuất bản**.
- Bản dịch gắn theo **id section** nên id phải khác nhau trong cả website: mẫu trang và "Nhân bản trang" luôn cấp id mới.

## Địa chỉ trang

- Trang chủ: `/s/<slug>` (tên miền riêng: `/`).
- Trang khác: `/s/<slug>/pages/<đường-dẫn-trang>` (tên miền riêng: `/pages/<đường-dẫn-trang>`), tên route `public-page`.
- Trên trang con, liên kết neo của đầu / chân trang (`#products`...) tự đổi thành `<trang chủ>#products` rồi cuộn tới đúng phần.

## Liên kết tới một trang: `page:<đường-dẫn>`

Trong mọi ô liên kết (menu, nút, chân trang, banner...) gõ `page:chinh-sach-doi-tra` (hoặc `page:home`). Ô liên kết có gợi ý sẵn các trang của website
(datalist). Máy chủ chấp nhận dạng này ở `safeLink`; khi khách xem, `StoreRenderer` đổi thành đường dẫn thật theo tên miền (`storePath`). Trong
trình chỉnh sửa liên kết chỉ hiển thị, không chuyển trang. Hộp **Cài đặt trang** hiện sẵn mã `page:...` và nút Chép.

## Trình chỉnh sửa

- **Ô chọn trang** cạnh tên website (`PageManager.vue`): danh sách trang (ẩn / chưa xuất bản có nhãn), thêm trang, cài đặt trang
  (tên, đường dẫn, hiện với khách, nhân bản, xoá), đưa lên / xuống (trang chủ luôn đầu).
- **Thêm trang** từ mẫu: Trang trắng, Giới thiệu, Liên hệ và 5 **trang chính sách** — Đổi trả, Vận chuyển, Bảo mật, Điều khoản, Thanh toán
  (`site_page_templates.go`). Nội dung chỉ là **chữ mẫu chung chung**: chủ shop phải đọc và sửa cho đúng cửa hàng (hộp thêm trang có cảnh báo); hệ thống không cam kết thay họ.
- `blocks` trong trình chỉnh sửa luôn là **trang đang sửa đã ghép sẵn** (thông báo + đầu trang ở trên, chân trang ở dưới), nên kéo thả, chọn, hoàn tác chạy như cũ.
  Đầu / chân trang / thông báo bị khoá ở vị trí trên / dưới cùng, mỗi loại chỉ có một, gắn nhãn "Chung"; chọn chúng thì cột phải báo "dùng chung cho mọi trang".
- Chuyển trang không cần lưu: nội dung trang cũ được giữ trong bộ nhớ, đầu / chân trang mang sang trang mới. "Chưa lưu" tính trên **toàn bộ** website,
  không phụ thuộc đang ở trang nào. Hoàn tác chỉ trong phạm vi một trang (đổi trang thì làm mới lịch sử).
- Thêm / nhân bản / xoá / đổi tên trang đều **lưu nháp trước** rồi gọi máy chủ, để danh sách trang luôn khớp.
- Lưu nháp chỉ gửi **phần dùng chung + các trang có thay đổi**; máy chủ kiểm tra hết rồi mới ghi (một trang sai thì không trang nào bị ghi).
- Tab **Ngôn ngữ** xét **mọi trang** (bản dịch gắn theo id section); xuất bản tự dịch nốt cả các trang.

## API

Chủ shop (`/api/v1/storefront/sites/:id/...`, đã có phân quyền theo khách hàng): `GET /` (kèm `pages`, `page_templates`, `max_pages`), `PUT /draft`
(`draft_shared_json` + `pages[{id, draft_blocks_json}]`; vẫn nhận `draft_blocks_json` kiểu cũ), `POST /publish`, `GET|POST /pages`,
`PATCH|DELETE /pages/:pid`, `POST /pages/:pid/duplicate`, `PUT /pages/order {ids}`.

Công khai: `GET /api/v1/storefront/public?slug=` (trang chủ ghép sẵn + `pages: [{slug,title,kind}]`), `GET /api/v1/storefront/public/pages/:pslug?slug=`
(`{slug,title,kind,blocks_json}`, chỉ phần riêng của trang; đầu / chân trang lấy từ trang chủ — `PublicPageView.vue` đặt trong `PublicShell`).

## Chuyển dữ liệu cũ

`migrateStorefrontPages` chạy mỗi lần máy chủ khởi động (idempotent: chỉ website chưa có trang nào): tạo trang chủ từ mảng section cũ, tách đầu / chân /
thông báo sang phần dùng chung (cả bản nháp lẫn bản đã xuất bản). **Không đụng** cột cũ `draft_blocks_json` / `published_blocks_json` nên khách đang xem
vẫn thấy y như cũ cho tới lần xuất bản kế tiếp. Website tạo mới (`CreateSite`, `getOrCreateForUser`) có sẵn trang chủ. Đã thử trên bản sao dữ liệu thật
(68 website): 68 trang chủ, không mất section nào, cột cũ không đổi.

Tương thích hai chiều: giao diện bản cũ gửi mảng "ghép sẵn" thì được tách ngược thành phần chung + trang chủ; giao diện mới đọc `draft_shared_json` + `pages`.
Đây là lý do máy chủ và web nên được triển khai gần nhau (giao diện mới + máy chủ cũ sẽ từ chối mở trình chỉnh sửa thay vì ghi đè nhầm).

## Chưa làm (các giai đoạn sau)

Menu điều hướng quản lý riêng (hiện menu vẫn là ô liên kết trong đầu trang), SEO từng trang (tiêu đề / mô tả / ảnh chia sẻ), sitemap, trang sản phẩm / bộ sưu tập
tuỳ biến, sao chép section giữa các trang, tự thêm liên kết chính sách vào chân trang, dịch tên trang.

## Kiểm thử

- `go test ./internal/handlers -run Page` — tách / ghép, `page:` link, chuyển dữ liệu cũ, CRUD, nháp → xuất bản, trang ẩn, tương thích kiểu cũ, quyền sở hữu.
- E2E trình duyệt: `scratchpad/e2e/pages_e2e.mjs` (66 kiểm tra) và `pages_lossless.mjs` (kiểm tra không mất section trên dữ liệu thật).
