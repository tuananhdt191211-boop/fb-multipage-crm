# Phân quyền theo khách hàng (multi-tenant)

Mỗi tài khoản là **một khách hàng** với dữ liệu Facebook và website riêng. Admin theo dõi toàn bộ. Tài khoản mới đăng ký bắt đầu
với dữ liệu **trống** và tự nhập Fanpage / website của mình.

## Vai trò

| Vai trò | Là gì | Dữ liệu thấy |
|---|---|---|
| `admin` | Quản trị hệ thống (người đăng ký đầu tiên) | Dữ liệu của chính admin; vào được khu **Khách hàng**; "Xem như khách" (chỉ đọc) |
| `customer` | Một khách hàng — chủ của một không gian dữ liệu | Chỉ dữ liệu của mình |
| `staff` | Nhân viên của một khách hàng (`owner_id`) | Dữ liệu của khách hàng chủ, theo các trang được cấp |

`workspace_id` = id của tài khoản khách hàng. Đăng ký thường, đăng nhập Google / Facebook đều tạo `customer`.

## Trang được dùng và giới hạn

Các trang (features): `pages` Fanpage · `inbox` Hộp thư · `ai` AI trả lời & tin nhắn mẫu · `posts` Đăng bài · `competitors` Đối thủ ·
`broadcasts` CSKH tự động · `sites` Website bán hàng. Danh sách nằm ở `users.features` (jsonb):

- `NULL` (tài khoản cũ chưa đặt) = được dùng tất cả; `[]` = không trang nào.
- Nhân viên chỉ dùng được phần giao của **trang được cấp cho họ** và **trang khách hàng chủ còn được dùng**.
- Khách hàng mới mặc định được dùng đủ. Admin bật/tắt từng trang ở **Khách hàng → Quản lý**.
- Giới hạn do admin đặt cho từng khách: `max_pages`, `max_sites`, `max_staff` (0 = không giới hạn). Kết nối Fanpage / tạo website /
  tạo nhân viên vượt giới hạn trả `403` mã `limit_reached`.

Việc chỉ **chủ tài khoản** làm được (nhân viên không): kết nối / đồng bộ / gỡ Fanpage, tạo / xoá website, tên miền riêng, cài đặt thanh
toán, quản lý nhân viên. Khoá AI dùng chung của hệ thống chỉ admin chỉnh; khách hàng dùng chung khoá đó.

## Cách máy chủ thực thi (không tin giao diện)

`middleware.Tenancy` chạy sau `JWTAuth` trên mọi API dữ liệu: đọc lại tài khoản từ database (đệm 20 giây, xoá đệm khi admin sửa),
từ chối tài khoản bị khoá, tính `WorkspaceID` + trang được dùng và đặt vào context. Sau đó:

- `RequireFeature(...)` chặn trang không được cấp (`403`, mã `feature_disabled`); `OwnerOnly()`; `AdminOnly()`.
- Handler lọc theo `wsID(c)`. **Fanpage là nguồn xác định chủ** (`facebook_pages.workspace_id`); hội thoại (`conversations.workspace_id`),
  bài đăng (`scheduled_posts.workspace_id`), đối thủ (`competitor_*.workspace_id`) có cột riêng; chiến dịch, đăng ký nhận tin, cài đặt AI
  lọc theo `page_id IN (Fanpage của workspace)`. Website / sản phẩm / đơn hàng đã theo `user_id` — `extractUserID` giờ trả workspace.
- Webhook Facebook và đồng bộ nền gán hội thoại mới cho chủ của Fanpage. Đồng bộ nền chỉ chạy cho khách hàng đang mở hộp thư.
- Một Fanpage chỉ thuộc một khách hàng (`409` nếu người khác kết nối lại). Đối thủ khoá duy nhất theo `(workspace_id, page_id)` nên hai
  khách theo dõi cùng một kênh vẫn có danh sách riêng. Token Fanpage của khách này không bao giờ được dùng cho khách khác.

## Admin theo dõi / "Xem như khách"

- `GET /admin/overview`, `GET/POST /admin/customers`, `GET/PATCH /admin/customers/:id`, `POST /admin/customers/:id/password`.
- **Xem như khách**: giao diện gửi header `X-Workspace: <id khách>` (chỉ admin được dùng; khách / nhân viên gửi cũng bị bỏ qua). Mọi API
  dữ liệu trả dữ liệu của khách đó; **mọi thao tác ghi bị chặn** (`403`, mã `view_only`). Banner vàng ở đầu trang có nút "Thoát".
  Header phải nằm trong `Access-Control-Allow-Headers` của CORS (`cmd/server/degraded.go`).

## Chuyển dữ liệu cũ

`handlers.MigrateTenancy` chạy mỗi lần máy chủ khởi động (idempotent, vài câu UPDATE):

1. `manager` / `user` / `staff` không có chủ → `customer`.
2. `features` NULL → đủ các trang.
3. Fanpage, hội thoại, bài đăng, đối thủ chưa có chủ (`workspace_id = 0`, dữ liệu dùng chung trước đây) → giao cho **admin đầu tiên**.
   Hội thoại của Fanpage đã gỡ cũng giao cho admin để không mất.

## Giao diện

- `auth` store giữ `tenant` (trang được dùng, giới hạn, đang xem như khách...); `can(feature)`; `/auth/me` được hỏi lại mỗi lần tải trang.
- Router: `meta.feature` / `requiresAdmin` / `ownerOnly`; không có quyền thì về Tổng quan kèm thông báo.
- Menu (`AppShell`, Tổng quan) chỉ hiện trang được dùng; admin có mục **Khách hàng**, chủ tài khoản có mục **Nhân viên**.
- Tổng quan của khách hàng mới có hướng dẫn "Bắt đầu" (kết nối Fanpage, tạo website) vì dữ liệu trống.

## Chưa làm

Đăng ký cần admin duyệt / chống đăng ký hàng loạt (hiện ai cũng đăng ký được); hạn mức theo gói tính phí; nhật ký thao tác của admin
khi xem như khách; số phiên bản hộp thư (`inboxrev`) vẫn dùng chung toàn hệ thống (khách khác có tin mới thì trình duyệt tải lại danh
sách, nhưng dữ liệu trả về vẫn chỉ của khách đó).
