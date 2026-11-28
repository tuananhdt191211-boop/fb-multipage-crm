# Thống kê website bán hàng

Trang quản trị: **Website bán hàng → Thống kê** (`/sites/:id/analytics`). Gồm xem trực tiếp, chỉ số chính, biểu đồ theo ngày,
phễu thanh toán, giỏ hàng bỏ dở, nguồn truy cập, sản phẩm, lượt click, tìm kiếm, thiết bị, quốc gia, giờ cao điểm.

## Cách hoạt động

```
trình duyệt khách ──(lô sự kiện, text/plain)──▶ POST /api/v1/storefront/public/track ──▶ store_sessions + store_events
chủ shop ──▶ GET /storefront/sites/:id/analytics | /analytics/live | /analytics/abandoned
đơn hàng ──(track_sid)──▶ shop_orders.track_sid  → doanh thu theo nguồn / thiết bị / nước
```

- **Phiên** (`store_sessions`): một lượt truy cập, quá 30 phút im lặng thì là phiên mới. Lưu nguồn (utm / tên miền giới thiệu),
  kênh, thiết bị, trình duyệt, nước (ước lượng), bước xa nhất trong phễu (`stage`), bước hiện tại (`last_event`, `last_path`) và
  giỏ hàng gần nhất (tối đa 10 dòng: sản phẩm, số lượng, giá, tên).
- **Sự kiện** (`store_events`): xem trang, xem sản phẩm, thêm / bỏ giỏ, mở giỏ, từng bước thanh toán, click, tìm kiếm, đổi thị trường.
  Heartbeat 40 giây/lần (chỉ khi tab đang mở và khách còn thao tác) chỉ cập nhật `last_seen`, không tạo dòng sự kiện.
- **Đơn hàng** do máy chủ ghi nhận (`order_placed`): không phụ thuộc trình duyệt.

### Các bước phễu (`stage`)

| stage | Sự kiện | Ý nghĩa |
|---|---|---|
| 1 | `product_view` | Xem sản phẩm |
| 2 | `add_to_cart` | Thêm vào giỏ |
| 3 | `checkout_start` | Mở trang thanh toán |
| 4 | `checkout_info` | Bắt đầu gõ thông tin giao hàng |
| 5 | `checkout_info_done` | Điền đủ họ tên + số điện thoại + địa chỉ |
| 6 | `checkout_payment` | Chọn phương thức thanh toán |
| 7 | `checkout_submit` | Bấm đặt hàng / thanh toán |
| 8 | `order_placed` | Đơn đã tạo (máy chủ) |

Khi khách đặt hàng mà không gõ gì (thông tin điền sẵn, phương thức mặc định) thì trang thanh toán tự bù các bước 4–6 trước bước 7
để phễu không bị hụt. **Bỏ dở** = đã tới bước 2–7, chưa đặt hàng và im lặng quá 30 phút.

## Riêng tư

- Không lưu IP, họ tên, số điện thoại, email, địa chỉ. Đường dẫn bị bỏ tham số (`?t=<token>`), trang đơn hàng gộp thành `/order/*`.
- Mã ẩn danh (`pp_vid`, `pp_sess_<slug>`) nằm ở localStorage của khách. Tôn trọng **Do Not Track**; không theo dõi chủ shop đang đăng
  nhập (có `access_token`) hay trình duyệt đặt `localStorage.pp_notrack = '1'`.
- Nước chỉ là ước lượng từ vùng ngôn ngữ trình duyệt (hoặc header `CF-IPCountry` nếu web chạy sau Cloudflare).
- Bot / công cụ kiểm tra (Googlebot, Lighthouse, curl...) bị bỏ qua theo User-Agent.

## Giới hạn & dọn dữ liệu

- Mỗi IP tối đa 90 lô/phút, mỗi website 6000 sự kiện/phút, mỗi lô tối đa 25 sự kiện / 32 KB.
- Mỗi ngày xoá sự kiện > 60 ngày và phiên > 180 ngày (`RunAnalyticsJanitor`) để không làm đầy database (Neon free 0,5–1 GB).
- Tra website theo slug có đệm 60 giây nên mỗi lô chỉ tốn 1 lần ghi phiên (upsert) + 1 lần ghi sự kiện, không có truy vấn đọc.

## Thêm sự kiện mới

1. Thêm vào `trackTypes` (`backend-go/internal/handlers/analytics.go`) và vào `TrackType` (`web-vue/src/lib/analytics.ts`).
2. Gọi `track('tên', {...})` ở nơi xảy ra.
3. Muốn click của một nút hiện tên ổn định (không đổi theo ngôn ngữ) thì gắn `data-track="tên_ổn_định"` và thêm nhãn ở `clickLabels`
   trong `SiteAnalyticsView.vue`.

## Chưa làm

Lưu thông tin liên hệ của khách bỏ dở (để gọi chốt đơn) — cần cân nhắc quyền riêng tư / đồng ý của khách; theo dõi chiến dịch quảng cáo
sâu hơn (chi phí, ROAS); cohort / khách hàng quay lại theo tài khoản.
