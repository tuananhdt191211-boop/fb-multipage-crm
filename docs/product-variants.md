# Biến thể sản phẩm (Màu, Size...)

Mỗi sản phẩm có tối đa **3 nhóm phân loại** (vd. Màu sắc, Kích cỡ), mỗi nhóm tối đa 100 giá trị, tổng tối đa **100 biến thể**.
Mỗi biến thể (một tổ hợp như “Đen / L”) có **giá bán, giá gốc, giá nhập, tồn kho, SKU và ảnh riêng**.

## Cách dùng (chủ shop)

1. **Sản phẩm → Sửa / Thêm sản phẩm → mục Phân loại → Thêm phân loại.** Đặt tên nhóm, gõ các giá trị rồi nhấn Enter (hoặc dán “Đen, Trắng, Xanh”).
2. Bảng biến thể tự sinh ra, mỗi dòng nhận giá / tồn kho đang nhập ở sản phẩm. Dùng **Đặt cùng lúc cho tất cả biến thể** để đổi hàng loạt,
   hoặc sửa từng dòng. Ô **∞** = không giới hạn tồn kho. Bấm ô ảnh để chọn một ảnh trong bộ sưu tập sản phẩm cho biến thể đó.
3. Sửa chữ một giá trị ngay trên nhãn (vd. “Trắng” → “Trắng ngà”): biến thể đổi theo và **giữ nguyên** giá, kho, SKU, ảnh. Thêm / bớt giá trị
   chỉ thêm / bớt các dòng tương ứng, dòng còn lại giữ dữ liệu.
4. Khi đã có biến thể, giá và tồn kho của sản phẩm chỉ là **số tổng hợp** (giá thấp nhất; tổng tồn kho, hoặc “không giới hạn” nếu có biến thể không giới hạn).

Nhập từ AliExpress / 1688 / link: các nhóm tìm thấy tự thành biến thể (xem [media-storage.md](media-storage.md)).

## Khách mua hàng

- Thẻ sản phẩm ở trang chủ hiện **“Từ <giá thấp nhất>”** và nút **Chọn phân loại** (mở trang sản phẩm).
- Trang sản phẩm: chọn từng nhóm, giá / ảnh / SKU / tình trạng còn hàng đổi theo biến thể. Giá trị hết hàng bị gạch ngang.
  Chọn một giá trị khiến tổ hợp không tồn tại thì tự chuyển sang biến thể gần nhất (ưu tiên còn hàng).
- Giỏ hàng, thanh toán, đơn hàng đều hiện tên biến thể; cùng sản phẩm nhưng khác biến thể là **hai dòng riêng**.
- Tên nhóm / giá trị được dịch ở tab **Ngôn ngữ** của trình dựng web (mục “Sản phẩm” trong bảng dịch, khoá `p|<id>|o|<chữ gốc>`);
  đơn hàng và chủ shop luôn thấy chữ gốc.

## Dữ liệu & quy tắc (kỹ thuật)

- Bảng `product_variants` (`product_id`, `storefront_id`, `position`, `title`, `option1..3`, `sku`, `barcode`, `price`, `compare_price`, `cost_price`,
  `stock`, `image_url`); nhóm phân loại nằm ở `products.options_json`. Bảng tự tạo khi khởi động (AutoMigrate).
- **API quản trị**: `POST/PUT /storefront/sites/:id/products` nhận thêm `options: [{name, values}]` và `variants: [{id?, option_values, price, compare_price,
  cost_price, stock, sku, barcode, image_url}]`. Không gửi hai trường này (client cũ) = **giữ nguyên** biến thể hiện có; gửi `options: []` = bỏ hết biến thể.
  Khi lưu, biến thể cũ được khớp theo `id`, rồi theo tổ hợp giá trị (không phân biệt hoa thường); không còn trong danh sách thì bị xoá.
- **API công khai**: `GET /storefront/public` mỗi sản phẩm có `variant_count`, `price_max`; `GET /storefront/public/products/:pid` có thêm `options`,
  `variants` (id, title, option_values, price, compare_price, in_stock, image_url, sku) — **không** có giá nhập, số tồn kho cụ thể, mã vạch.
- **Đặt hàng** (`POST /storefront/public/orders`): mỗi dòng `{product_id, variant_id, qty}`. Sản phẩm có biến thể **bắt buộc** `variant_id` hợp lệ của chính sản phẩm đó;
  giá và tồn kho lấy từ biến thể (không tin giá từ trình duyệt). Đơn lưu `variant_id`, `variant_title`, `sku` tại thời điểm đặt (đổi giá / tên sau đó không đổi đơn cũ).
- **Tồn kho**: trừ / hoàn theo biến thể bằng câu lệnh có điều kiện (`stock >= qty`) nên không bán quá tồn kho dù nhiều đơn cùng lúc; các đơn đụng cùng một sản phẩm
  xếp hàng (khoá dòng sản phẩm theo thứ tự id) để số tổng hợp ở sản phẩm luôn đúng. Biến thể `stock = -1` không bị trừ. Huỷ đơn hoàn kho đúng biến thể, mở lại đơn trừ lại.
  Đơn đặt **trước** khi sản phẩm chuyển sang biến thể (dòng không có `variant_id`) không còn cộng / trừ vào số tổng hợp.
- Xoá sản phẩm / website xoá luôn các biến thể.

## Kiểm thử

```bash
cd backend-go
TEST_DATABASE_URL=postgres://...fbcrm_test?sslmode=disable go test ./internal/handlers -run Variant -v
```

Gồm: tạo / sửa / số tổng hợp, kiểm tra dữ liệu (nhóm, trùng tổ hợp, giới hạn 100), client cũ không làm mất biến thể, đặt hàng (thiếu / sai biến thể, hết hàng, vượt tồn kho),
trừ / hoàn kho, 8 đơn đồng thời không bán quá tồn kho, biến thể không giới hạn, đơn cũ sau khi chuyển sang biến thể, sản phẩm cũ (cột NULL).
