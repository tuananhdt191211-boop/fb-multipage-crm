package handlers

// Mẫu giao diện có sẵn khi tạo website mới.
// Nội dung là chữ mẫu chung chung; khách tự thay ảnh, chữ và sản phẩm.

import (
	"encoding/json"

	"fb-multipage-crm/internal/models"

	"gorm.io/gorm"
)

type siteTemplate struct {
	Key          string
	Name         string
	PrimaryColor string
	Blocks       func() []StorefrontBlock
	Products     []models.Product
}

func blk(id, typ string, data map[string]interface{}) StorefrontBlock {
	return StorefrontBlock{ID: id, Type: typ, Enabled: true, Data: data}
}

var siteTemplates = map[string]siteTemplate{
	"wellness": {
		Key:          "wellness",
		Name:         "Mỹ phẩm / Chăm sóc sức khoẻ",
		PrimaryColor: "#2f6f5e",
		Blocks: func() []StorefrontBlock {
			return []StorefrontBlock{
				blk("tpl-announce", "announcement", map[string]interface{}{
					"text": "Miễn phí giao hàng cho đơn từ 500.000đ · Đổi trả trong 30 ngày",
					"link": "#products",
				}),
				blk("tpl-header", "header", map[string]interface{}{
					"brand_name":  "Tên thương hiệu",
					"show_brand":  true,
					"logo_height": 36,
					"brand_size":  "md",
					"layout":      "split",
					"nav_items": []map[string]string{
						{"label": "Chăm sóc da", "link": "#products"},
						{"label": "Chăm sóc tóc", "link": "#products"},
						{"label": "Bộ sản phẩm", "link": "#collections"},
						{"label": "Hỏi đáp", "link": "#faq"},
						{"label": "Liên hệ", "link": "#contact"},
					},
					"show_cta":    false,
					"show_search": true,
					"show_cart":   true,
					"sticky":      true,
					"border":      true,
				}),
				blk("tpl-banner", "banner", map[string]interface{}{
					"height":   "lg",
					"autoplay": 6,
					"overlay":  true,
					"slides": []map[string]string{
						{"image_url": "", "mobile_image_url": "", "heading": "Làn da khoẻ mạnh bắt đầu từ thiên nhiên", "text": "Công thức thảo mộc dịu nhẹ, phù hợp cả làn da nhạy cảm.", "button_label": "Mua ngay", "link": "#products"},
						{"image_url": "", "mobile_image_url": "", "heading": "Bộ chăm sóc trọn gói – tiết kiệm đến 20%", "text": "Kết hợp kem, dầu gội và sữa tắm cho hiệu quả tốt nhất.", "button_label": "Xem bộ sản phẩm", "link": "#products"},
					},
				}),
				blk("tpl-story", "image_text", map[string]interface{}{
					"image_url":    "",
					"image_side":   "left",
					"eyebrow":      "Câu chuyện thương hiệu",
					"heading":      "Khoa học kết hợp thảo dược tự nhiên",
					"body":         "Chúng tôi nghiên cứu từng thành phần để mang lại sản phẩm an toàn, hiệu quả và dễ dùng hằng ngày.",
					"button_label": "Mua ngay",
					"button_link":  "#products",
					"points": []map[string]string{
						{"title": "Công thức khoa học", "text": "Thành phần được chọn lọc và kiểm nghiệm."},
						{"title": "Hiệu quả rõ rệt", "text": "Tập trung vào kết quả thực tế cho làn da."},
						{"title": "Chất lượng đảm bảo", "text": "Sản xuất theo quy trình kiểm soát chặt chẽ."},
					},
				}),
				blk("tpl-reviews", "testimonials", map[string]interface{}{
					"heading": "Khách hàng nói gì về chúng tôi",
					"layout":  "carousel",
					"items": []map[string]string{
						{"name": "Chị Lan", "quote": "Dùng 2 tuần thấy da dịu hẳn, không còn ngứa rát. Sẽ mua tiếp!"},
						{"name": "Anh Minh", "quote": "Giao hàng nhanh, đóng gói cẩn thận. Sản phẩm thơm nhẹ dễ chịu."},
						{"name": "Chị Hương", "quote": "Shop tư vấn nhiệt tình, dùng đúng hướng dẫn nên hiệu quả rất tốt."},
						{"name": "Chị Mai", "quote": "Mua bộ combo rất tiết kiệm, cả nhà đều dùng được."},
					},
				}),
				blk("tpl-products", "products", map[string]interface{}{
					"heading":   "Bộ chăm sóc toàn diện",
					"source":    "catalog",
					"show_buy":  true,
					"buy_mode":  "cart",
					"buy_label": "Thêm vào giỏ",
				}),
				blk("tpl-collections", "collections", map[string]interface{}{
					"heading": "Mua theo danh mục",
					"items": []map[string]string{
						{"image_url": "", "title": "Chăm sóc da", "link": "#products"},
						{"image_url": "", "title": "Chăm sóc tóc", "link": "#products"},
					},
				}),
				blk("tpl-icons", "icon_strip", map[string]interface{}{
					"items": []map[string]string{
						{"icon": "truck", "title": "Giao hàng miễn phí", "text": "Cho đơn từ 500.000đ"},
						{"icon": "refresh", "title": "Đổi trả 30 ngày", "text": "Hoàn tiền nếu không hài lòng"},
						{"icon": "headset", "title": "Tư vấn tận tình", "text": "Hỗ trợ 8:00 – 21:00 mỗi ngày"},
					},
				}),
				blk("tpl-faq", "faq", map[string]interface{}{
					"heading": "Câu hỏi thường gặp",
					"items": []map[string]string{
						{"q": "Sản phẩm có dùng được cho da nhạy cảm không?", "a": "Có. Nên thử trước trên một vùng da nhỏ trong 24 giờ."},
						{"q": "Bao lâu thì thấy hiệu quả?", "a": "Thông thường sau 2–4 tuần sử dụng đều đặn."},
						{"q": "Giao hàng mất bao lâu?", "a": "Nội thành 1–2 ngày, tỉnh khác 2–4 ngày."},
					},
				}),
				blk("tpl-newsletter", "newsletter", map[string]interface{}{
					"heading":      "Nhận tư vấn & ưu đãi riêng",
					"text":         "Để lại email để nhận gợi ý chăm sóc phù hợp và mã giảm giá cho lần mua đầu.",
					"button_label": "Đăng ký",
					"placeholder":  "Email của bạn",
				}),
				blk("tpl-footer", "footer", map[string]interface{}{
					"company":    "Tên công ty của bạn",
					"about_text": "Sản phẩm chăm sóc da và tóc từ thảo dược tự nhiên.",
					"address":    "Địa chỉ cửa hàng",
					"phone":      "0900 000 000",
					"email":      "hotro@example.com",
					"columns": []map[string]interface{}{
						{"title": "Về chúng tôi", "links": []map[string]string{
							{"label": "Giới thiệu", "link": "#"},
							{"label": "Hỏi đáp", "link": "#faq"},
							{"label": "Liên hệ", "link": "#contact"},
						}},
						{"title": "Chính sách", "links": []map[string]string{
							{"label": "Giao hàng & đổi trả", "link": "#"},
							{"label": "Bảo mật thông tin", "link": "#"},
							{"label": "Điều khoản sử dụng", "link": "#"},
						}},
					},
					"socials":   map[string]string{"facebook_link": "", "instagram_link": "", "tiktok_link": "", "youtube_link": "", "zalo_link": ""},
					"copyright": "© Bản quyền thuộc về cửa hàng của bạn.",
				}),
			}
		},
		Products: []models.Product{
			{Name: "Kem dưỡng thảo mộc làm dịu da", Description: "Làm dịu nhanh vùng da khô, ngứa, bong tróc.", Price: 399000, ComparePrice: 570000, Stock: -1, IsActive: true, SortOrder: 1},
			{Name: "Dầu gội collagen chăm sóc da đầu", Description: "Làm sạch dịu nhẹ, giảm gàu và ngứa da đầu.", Price: 559000, ComparePrice: 929000, Stock: -1, IsActive: true, SortOrder: 2},
			{Name: "Bộ chăm sóc phục hồi trọn gói", Description: "Combo kem, dầu gội và sữa tắm tiết kiệm.", Price: 1255000, ComparePrice: 1509000, Stock: -1, IsActive: true, SortOrder: 3},
			{Name: "Sữa tắm thực vật dịu nhẹ", Description: "Không xà phòng, giữ ẩm tự nhiên cho da.", Price: 559000, ComparePrice: 922000, Stock: -1, IsActive: true, SortOrder: 4},
		},
	},
}

// templateBlocksJSON trả về JSON section của mẫu (đã qua bộ lọc an toàn)
func templateBlocksJSON(key string) (string, siteTemplate, bool) {
	tpl, ok := siteTemplates[key]
	if !ok {
		return "", siteTemplate{}, false
	}
	b, _ := json.Marshal(tpl.Blocks())
	clean, err := sanitizeBlocksJSON(string(b))
	if err != nil {
		return "", siteTemplate{}, false
	}
	return clean, tpl, true
}

func createTemplateProducts(db *gorm.DB, sf *models.Storefront, tpl siteTemplate) {
	for _, p := range tpl.Products {
		p.StorefrontID = sf.ID
		p.UserID = sf.UserID
		_ = db.Create(&p).Error
	}
}
