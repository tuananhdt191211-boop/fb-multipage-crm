# Ảnh & video sản phẩm: kho lưu trữ (Cloudflare R2 / S3) và nhập sản phẩm

## Vì sao cần kho lưu trữ ngoài

- Ổ đĩa của Render bị xoá mỗi lần deploy hoặc khi máy chủ "ngủ dậy" → không lưu video ở đó được.
- Nhét video vào PostgreSQL (Neon) sẽ phá hạn mức dung lượng và băng thông.
- Kho kiểu S3 lưu bền và phục vụ trực tiếp qua CDN; backend không phải gánh băng thông video.

**Chưa cấu hình** thì hệ thống vẫn chạy: ảnh lưu trong PostgreSQL như cũ (tối đa 600 ảnh / website), nút **Tải video lên** bị tắt
(vẫn dán được link YouTube / Vimeo / `.mp4`). Khi cấu hình xong: ảnh **và** video lưu ở kho (tối đa 5000 file / website).

## Cấu hình Cloudflare R2 (khuyên dùng — miễn phí 10GB, không tính phí băng thông tải ra)

1. Cloudflare → **R2 Object Storage** → **Create bucket** (vd. `pagepilot-media`).
2. Trong bucket → **Settings → Public access**:
   - Dùng thử: bật **R2.dev subdomain** → copy địa chỉ dạng `https://pub-xxxxxxxx.r2.dev`.
   - Cửa hàng thật: **Connect Domain** (vd. `cdn.tenmien.com`, tên miền phải nằm trên Cloudflare) — r2.dev có giới hạn tốc độ.
3. **R2 → Manage R2 API Tokens → Create API token**: quyền **Object Read & Write**, giới hạn vào đúng bucket. Copy
   **Access Key ID** và **Secret Access Key** (chỉ hiện một lần). **Account ID** nằm ở trang tổng quan R2.
4. Render → service backend → **Environment** → thêm:

| Biến | Giá trị |
|---|---|
| `R2_ACCOUNT_ID` | Account ID của Cloudflare |
| `R2_ACCESS_KEY_ID` | Access Key ID ở bước 3 |
| `R2_SECRET_ACCESS_KEY` | Secret Access Key ở bước 3 |
| `R2_BUCKET` | tên bucket, vd. `pagepilot-media` |
| `R2_PUBLIC_URL` | địa chỉ công khai ở bước 2 (không có dấu `/` cuối) |

5. Deploy lại. Log khởi động phải có dòng `[media] dùng kho lưu trữ ngoài (bucket ...)`; trang **Sản phẩm → Sửa** sẽ sáng nút
   **Tải video lên**.

Không cần cấu hình CORS: máy chủ tự tải file lên kho (trình duyệt không gọi thẳng R2).

### AWS S3, Backblaze B2, MinIO... (tương thích S3)

Dùng nhóm biến `S3_*` thay cho `R2_*`: `S3_ENDPOINT` (vd. `https://s3.us-east-1.amazonaws.com`), `S3_REGION` (mặc định `auto`),
`S3_BUCKET`, `S3_ACCESS_KEY_ID`, `S3_SECRET_ACCESS_KEY`, `S3_PUBLIC_BASE_URL`. Bucket phải cho phép đọc công khai (hoặc đứng
sau CDN). Hệ thống dùng địa chỉ dạng path-style `endpoint/bucket/khóa`.

## Giới hạn & lưu ý

- Ảnh tối đa 5MB (giao diện tự nén ảnh lớn xuống ≤1920px), video tối đa 80MB (MP4 H.264 / MOV / WEBM), tối đa 20 ảnh/video cho mỗi sản phẩm.
- Xoá **sản phẩm** không xoá file; xoá file ở **Thư viện** (bấm 2 lần) mới xoá khỏi kho. Tab **Video** của thư viện dùng để dọn video không dùng.
- Khoá truy cập R2 chỉ nằm trong biến môi trường của máy chủ, không bao giờ gửi xuống trình duyệt.

## Nhập sản phẩm từ AliExpress / 1688 / website khác

Màn **Sản phẩm → Nhập từ AliExpress / 1688 / link** tạo **bản nháp** để bạn xem lại rồi mới lưu (sản phẩm nhập về mặc định ở trạng thái ẨN).

| Cách | Làm được gì | Giới hạn thực tế |
|---|---|---|
| Dán link — website thường (Shopify, WooCommerce...) | tên, mô tả, giá, ảnh, video, phân loại | tuỳ website |
| Dán link — AliExpress | tên + ảnh | giá, mô tả, phân loại nạp bằng JavaScript nên máy chủ không thấy |
| Dán link — 1688 / Taobao | không | chặn máy chủ bằng xác thực trượt |
| **Dán nội dung trang** (Ctrl+A, Ctrl+C trên trang đang mở rồi Ctrl+V) | mọi nguồn: tên, ảnh, giá, phân loại, mô tả | cần mở trang bằng trình duyệt của bạn |

- Có khoá AI thì AI đọc chữ của trang, bỏ phần rác, **dịch sang tiếng Việt** và tách giá / phân loại. Không có AI vẫn lấy được ảnh và chữ thô.
- Giá nguồn (CNY, USD...) được quy đổi sang VND theo tỉ giá hằng ngày; ô **Hệ số nhân** (mặc định 2,5 cho ngoại tệ) tính ra giá bán và
  giá nhập được lưu riêng để hiện lợi nhuận.
- Tuỳ chọn **Tải ảnh về thư viện của tôi** lưu bản sao ảnh vào kho của bạn, không phụ thuộc nguồn (nguồn xoá ảnh vẫn không mất).
- Hệ thống chưa quản lý biến thể (màu / size với giá và tồn kho riêng): phân loại tìm thấy được ghi vào cuối mô tả.
- Máy chủ chặn tải địa chỉ nội bộ (chống SSRF), chỉ cổng 80/443, mỗi tài khoản tối đa 15 lần nhập / phút.

## Kiểm thử với kho S3 thật

```bash
docker run -d --name pp_s3 -p 18000:8000 -e S3BACKEND=mem -e SCALITY_ACCESS_KEY_ID=accessKey1 -e SCALITY_SECRET_ACCESS_KEY=verySecretKey1 scality/s3server
cd backend-go
TEST_DATABASE_URL=postgres://...fbcrm_test?sslmode=disable \
TEST_S3_ENDPOINT=http://localhost:18000 TEST_S3_ACCESS=accessKey1 TEST_S3_SECRET=verySecretKey1 TEST_S3_BUCKET=pp-test TEST_S3_REGION=us-east-1 \
go test ./internal/storage ./internal/handlers -run 'RealServer|Product|SiteMedia' -v
```
