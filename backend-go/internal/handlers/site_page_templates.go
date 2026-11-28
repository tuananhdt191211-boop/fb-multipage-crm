package handlers

// Mẫu trang có sẵn khi bấm "Thêm trang": trang trắng, giới thiệu, liên hệ và các trang chính sách thường gặp.
// Nội dung là chữ MẪU chung chung — chủ shop phải đọc và sửa lại cho đúng với cửa hàng của mình
// (đặc biệt các trang chính sách có ràng buộc pháp lý), hệ thống không thay họ cam kết điều gì.

import (
	"sort"

	"fb-multipage-crm/internal/models"
)

type pageTemplate struct {
	Key    string
	Title  string // tên mặc định của trang
	Slug   string // đường dẫn mặc định
	Kind   string // models.PageKind*
	Desc   string // mô tả ngắn hiện ở hộp chọn mẫu
	Order  int
	Blocks func(title string) []StorefrontBlock
}

func textBlock(heading, body string) StorefrontBlock {
	return blk(newBlockID("richtext"), "richtext", map[string]interface{}{"heading": heading, "body": body, "align": "left"})
}

// clauseBlock: một mục của trang chính sách — khoảng cách trên / dưới thu gọn để cả trang đọc liền mạch
func clauseBlock(heading, body string) StorefrontBlock {
	return blk(newBlockID("richtext"), "richtext", map[string]interface{}{"heading": heading, "body": body, "align": "left", "compact": true})
}

// policyBlocks: tiêu đề lớn + các mục chính sách (mỗi mục một khối Văn bản để chỉnh riêng từng đoạn)
func policyBlocks(title, intro string, sections [][2]string) func(string) []StorefrontBlock {
	return func(t string) []StorefrontBlock {
		if t == "" {
			t = title
		}
		out := []StorefrontBlock{textBlock(t, intro)}
		for _, s := range sections {
			out = append(out, clauseBlock(s[0], s[1]))
		}
		return out
	}
}

var pageTemplates = map[string]pageTemplate{
	"blank": {
		Key: "blank", Title: "Trang mới", Slug: "trang-moi", Kind: models.PageKindPage, Order: 0,
		Desc: "Trang trắng — tự thêm các phần bạn muốn.",
		Blocks: func(t string) []StorefrontBlock {
			return []StorefrontBlock{textBlock(t, "Nhập nội dung của bạn tại đây.")}
		},
	},
	"about": {
		Key: "about", Title: "Giới thiệu", Slug: "gioi-thieu", Kind: models.PageKindPage, Order: 1,
		Desc: "Câu chuyện thương hiệu, cam kết và lời mời liên hệ.",
		Blocks: func(t string) []StorefrontBlock {
			return []StorefrontBlock{
				textBlock(t, "Chúng tôi là cửa hàng chuyên cung cấp sản phẩm chất lượng với mức giá hợp lý. Hãy kể ngắn gọn câu chuyện của bạn: vì sao bắt đầu, bạn muốn mang lại điều gì cho khách hàng.\n\nThay đoạn này bằng nội dung thật của cửa hàng."),
				blk(newBlockID("icon_strip"), "icon_strip", map[string]interface{}{
					"items": []map[string]string{
						{"icon": "truck", "title": "Giao hàng nhanh", "text": "Toàn quốc 2–4 ngày"},
						{"icon": "refresh", "title": "Đổi trả dễ dàng", "text": "Theo chính sách của cửa hàng"},
						{"icon": "headset", "title": "Hỗ trợ tận tình", "text": "Luôn sẵn sàng giải đáp"},
					},
				}),
				blk(newBlockID("cta"), "cta", map[string]interface{}{
					"title": "Cần tư vấn thêm?", "subtitle": "Nhắn cho chúng tôi, đội ngũ sẽ phản hồi sớm nhất.",
					"button_text": "Liên hệ ngay", "button_link": "page:lien-he", "footer_note": "",
				}),
			}
		},
	},
	"contact": {
		Key: "contact", Title: "Liên hệ", Slug: "lien-he", Kind: models.PageKindPage, Order: 2,
		Desc: "Thông tin liên hệ và câu hỏi thường gặp.",
		Blocks: func(t string) []StorefrontBlock {
			return []StorefrontBlock{
				blk(newBlockID("contact"), "contact", map[string]interface{}{
					"heading": t, "phone": "0900 000 000", "email": "hotro@example.com", "address": "Địa chỉ cửa hàng", "zalo_link": "",
				}),
				blk(newBlockID("faq"), "faq", map[string]interface{}{
					"heading": "Câu hỏi thường gặp",
					"items": []map[string]string{
						{"q": "Tôi có thể liên hệ vào giờ nào?", "a": "Từ 8:00 đến 21:00 mỗi ngày, kể cả cuối tuần."},
						{"q": "Bao lâu thì nhận được phản hồi?", "a": "Thông thường trong vòng vài giờ làm việc."},
					},
				}),
			}
		},
	},
	"return": {
		Key: "return", Title: "Chính sách đổi trả", Slug: "chinh-sach-doi-tra", Kind: models.PageKindPolicy, Order: 10,
		Desc: "Điều kiện, thời hạn và cách đổi trả / hoàn tiền.",
		Blocks: policyBlocks("Chính sách đổi trả", "Chúng tôi muốn bạn hài lòng với đơn hàng. Nếu sản phẩm chưa đúng ý, bạn có thể đổi hoặc trả theo các điều kiện dưới đây. (Nội dung mẫu — hãy chỉnh cho đúng với cửa hàng của bạn.)", [][2]string{
			{"1. Thời hạn đổi trả", "Bạn có thể yêu cầu đổi hoặc trả hàng trong vòng 7 ngày kể từ khi nhận hàng."},
			{"2. Điều kiện áp dụng", "• Sản phẩm còn nguyên tem, nhãn, bao bì và chưa qua sử dụng.\n• Sản phẩm bị lỗi do nhà sản xuất hoặc giao sai mẫu, sai kích cỡ.\n• Có hình ảnh / video mở hộp (nếu sản phẩm bị lỗi hoặc hư hại khi vận chuyển)."},
			{"3. Trường hợp không áp dụng", "• Sản phẩm đã qua sử dụng hoặc bị hư hỏng do người dùng.\n• Sản phẩm thuộc nhóm không đổi trả được nêu rõ trên trang sản phẩm."},
			{"4. Cách thực hiện", "Liên hệ cửa hàng qua số điện thoại hoặc email trong trang Liên hệ, cung cấp mã đơn hàng và lý do. Chúng tôi sẽ hướng dẫn bạn các bước tiếp theo."},
			{"5. Hoàn tiền", "Sau khi nhận và kiểm tra sản phẩm, chúng tôi hoàn tiền theo cách bạn đã thanh toán trong vòng 3–7 ngày làm việc."},
		}),
	},
	"shipping": {
		Key: "shipping", Title: "Chính sách vận chuyển", Slug: "chinh-sach-van-chuyen", Kind: models.PageKindPolicy, Order: 11,
		Desc: "Khu vực giao, thời gian và phí vận chuyển.",
		Blocks: policyBlocks("Chính sách vận chuyển", "Thông tin về giao hàng và phí vận chuyển. (Nội dung mẫu — hãy chỉnh cho đúng với cửa hàng của bạn.)", [][2]string{
			{"1. Khu vực giao hàng", "Chúng tôi giao hàng toàn quốc thông qua các đơn vị vận chuyển uy tín."},
			{"2. Thời gian giao hàng", "• Nội thành: 1–2 ngày làm việc.\n• Các tỉnh khác: 2–5 ngày làm việc.\nThời gian có thể kéo dài hơn vào dịp lễ, Tết hoặc khi có thiên tai."},
			{"3. Phí vận chuyển", "Phí được tính tại bước thanh toán theo địa chỉ nhận hàng. Đơn từ mức giá nhất định có thể được miễn phí vận chuyển (xem thông báo trên cửa hàng)."},
			{"4. Kiểm tra hàng khi nhận", "Vui lòng kiểm tra tình trạng kiện hàng khi nhận. Nếu có dấu hiệu bị móp, rách hoặc ướt, hãy liên hệ ngay với chúng tôi."},
		}),
	},
	"privacy": {
		Key: "privacy", Title: "Chính sách bảo mật", Slug: "chinh-sach-bao-mat", Kind: models.PageKindPolicy, Order: 12,
		Desc: "Thu thập, sử dụng và bảo vệ thông tin khách hàng.",
		Blocks: policyBlocks("Chính sách bảo mật thông tin", "Chúng tôi tôn trọng và bảo vệ thông tin cá nhân của bạn. (Nội dung mẫu — hãy chỉnh cho đúng với cách cửa hàng của bạn thực sự xử lý dữ liệu.)", [][2]string{
			{"1. Thông tin chúng tôi thu thập", "Họ tên, số điện thoại, email, địa chỉ nhận hàng và lịch sử đơn hàng khi bạn đặt mua hoặc đăng ký nhận tin."},
			{"2. Mục đích sử dụng", "Xử lý và giao đơn hàng, chăm sóc khách hàng, thông báo ưu đãi (nếu bạn đồng ý) và cải thiện dịch vụ."},
			{"3. Chia sẻ thông tin", "Chúng tôi chỉ chia sẻ thông tin cần thiết cho đơn vị vận chuyển và cổng thanh toán để hoàn tất đơn hàng, không bán thông tin của bạn cho bên thứ ba."},
			{"4. Bảo mật", "Thông tin được lưu trữ trên hệ thống có biện pháp bảo vệ phù hợp. Thông tin thẻ thanh toán do cổng thanh toán xử lý, cửa hàng không lưu số thẻ."},
			{"5. Quyền của bạn", "Bạn có thể yêu cầu xem, chỉnh sửa hoặc xoá thông tin cá nhân bằng cách liên hệ với chúng tôi qua trang Liên hệ."},
		}),
	},
	"terms": {
		Key: "terms", Title: "Điều khoản sử dụng", Slug: "dieu-khoan-su-dung", Kind: models.PageKindPolicy, Order: 13,
		Desc: "Quy định khi mua hàng và sử dụng website.",
		Blocks: policyBlocks("Điều khoản sử dụng", "Khi truy cập và mua hàng trên website này, bạn đồng ý với các điều khoản dưới đây. (Nội dung mẫu — hãy chỉnh cho đúng với cửa hàng của bạn.)", [][2]string{
			{"1. Thông tin sản phẩm và giá", "Chúng tôi cố gắng hiển thị thông tin, hình ảnh và giá chính xác. Trong trường hợp có sai sót, chúng tôi sẽ liên hệ để xác nhận lại với bạn trước khi xử lý đơn."},
			{"2. Đặt hàng và xác nhận", "Đơn hàng chỉ được xác nhận khi cửa hàng thông báo đã tiếp nhận. Chúng tôi có quyền từ chối hoặc huỷ đơn trong trường hợp hết hàng hoặc thông tin không hợp lệ."},
			{"3. Thanh toán", "Các hình thức thanh toán được hiển thị tại bước thanh toán. Bạn chịu trách nhiệm cung cấp thông tin thanh toán chính xác."},
			{"4. Quyền sở hữu nội dung", "Hình ảnh, nội dung và nhãn hiệu trên website thuộc quyền sở hữu của cửa hàng; vui lòng không sao chép khi chưa được đồng ý."},
			{"5. Thay đổi điều khoản", "Chúng tôi có thể cập nhật điều khoản theo thời gian; phiên bản hiện hành là phiên bản đang hiển thị trên trang này."},
		}),
	},
	"payment": {
		Key: "payment", Title: "Chính sách thanh toán", Slug: "chinh-sach-thanh-toan", Kind: models.PageKindPolicy, Order: 14,
		Desc: "Các hình thức thanh toán được chấp nhận.",
		Blocks: policyBlocks("Chính sách thanh toán", "Các hình thức thanh toán bạn có thể dùng khi mua hàng. (Nội dung mẫu — chỉ giữ những hình thức cửa hàng thực sự hỗ trợ.)", [][2]string{
			{"1. Thanh toán khi nhận hàng (COD)", "Bạn thanh toán bằng tiền mặt cho nhân viên giao hàng sau khi kiểm tra đơn hàng."},
			{"2. Chuyển khoản / thanh toán trực tuyến", "Bạn có thể thanh toán qua cổng thanh toán hoặc chuyển khoản theo hướng dẫn ở bước thanh toán. Đơn hàng được xử lý sau khi chúng tôi nhận được tiền."},
			{"3. Hoá đơn", "Nếu cần hoá đơn, vui lòng thông báo khi đặt hàng hoặc liên hệ với chúng tôi trong vòng 24 giờ sau khi đặt."},
		}),
	},
}

// pageTemplateList: danh sách mẫu để giao diện hiển thị (có thứ tự ổn định)
func pageTemplateList() []map[string]string {
	keys := make([]string, 0, len(pageTemplates))
	for k := range pageTemplates {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return pageTemplates[keys[i]].Order < pageTemplates[keys[j]].Order })
	out := make([]map[string]string, 0, len(keys))
	for _, k := range keys {
		t := pageTemplates[k]
		out = append(out, map[string]string{"key": t.Key, "title": t.Title, "slug": t.Slug, "kind": t.Kind, "desc": t.Desc})
	}
	return out
}
