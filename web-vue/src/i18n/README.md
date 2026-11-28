# Đa ngôn ngữ giao diện quản trị

Giao diện quản trị PagePilot được **viết bằng tiếng Việt** và được dịch tại chỗ ở trình duyệt. Web công khai của khách
(`/s/:slug`, tên miền riêng) dùng bộ khác: `src/lib/i18n.ts`.

## Cách hoạt động

1. `scripts/extract-ui-strings.mjs` quét mã nguồn (`.vue`, `.ts`) **và các thông báo máy chủ** (`{"error": "..."}` trong
   `backend-go/internal/handlers/*.go`) để lập **danh mục** các câu tiếng Việt. Câu có biến được ghi dạng mẫu
   `Đã chọn {0} trang` (Go `%s`/`%d` được đổi thành `{0}`, `{1}`...).
2. `src/i18n/en.json` (câu tiếng Việt → tiếng Anh) là **nguồn dịch gốc**, do người viết. Script ghi thêm bản sao
   `catalog.vi.json` + `en.json` vào `backend-go/internal/handlers/uicatalog/` (được `go:embed` vào backend — nhớ commit cả hai).
3. `src/lib/uiI18n.ts` dịch DOM trang quản trị: chỉ dịch khi **nguyên văn** một đoạn chữ / thuộc tính
   (`placeholder`, `title`, `alt`, `aria-label`, `label`, `data-placeholder`) trùng một câu trong danh mục. Nội dung của khách
   (tin nhắn, tên, sản phẩm...) không nằm trong danh mục nên không bao giờ bị dịch.
4. Tiếng Anh nằm sẵn trong gói web (nạp lười). Ngôn ngữ khác: backend `GET /api/v1/ui-i18n/:lang` trả bản đã dịch; nếu còn
   thiếu, người dùng đã đăng nhập tự gọi `POST /api/v1/ui-i18n/:lang/fill` để AI dịch tiếp rồi lưu **dùng chung** trong bảng
   `translation_caches` (tiền tố `ui|`). Người đầu tiên chọn một ngôn ngữ làm AI dịch, những người sau dùng luôn.
5. Tiền tệ: giá trong hệ thống luôn là VND; `adminMoney()` chỉ **quy đổi để hiển thị** theo tỉ giá hằng ngày
   (`/storefront/public/rates`), mặc định theo ngôn ngữ, người dùng có thể chọn riêng.

## Khi sửa / thêm chữ giao diện

```bash
npm run i18n:extract        # cập nhật danh mục (ghi cả file cho backend)
node scripts/extract-ui-strings.mjs --missing   # xem câu nào chưa có tiếng Anh ("số<TAB>câu")
node scripts/extract-ui-strings.mjs --merge f.txt  # nạp bản dịch dạng "số|English" vào en.json
npm run i18n:check          # thoát mã 1 nếu còn câu thiếu tiếng Anh
```

Câu chưa có trong danh mục / chưa dịch sẽ tự hiện tiếng Việt (không hỏng gì). Test `go test ./internal/handlers -run UICatalog`
kiểm tra danh mục và bản tiếng Anh luôn khớp nhau (đủ câu, đúng biến `{n}`).

## Đánh dấu vùng không được dịch

Thêm `data-i18n="skip"` cho phần tử chứa nội dung của người dùng (tên, tin nhắn...); bên trong vùng đó muốn dịch lại
một phần (vd. nút của trình chỉnh sửa web) thì dùng `data-i18n="on"`. Ô đang gõ (`contenteditable`) và `<textarea>` luôn bị bỏ qua
(riêng thuộc tính `placeholder` vẫn được dịch).

Không dùng `translate="no"` của HTML cho việc này: nó còn chặn luôn tính năng "Dịch trang" của trình duyệt.

## Thêm ngôn ngữ

Thêm vào `UI_LANGS` (`src/lib/uiI18n.ts`) và `uiLangNames` (`backend-go/internal/handlers/ui_i18n.go`). Chưa hỗ trợ ngôn ngữ
viết từ phải sang trái (bố cục chưa lật).
