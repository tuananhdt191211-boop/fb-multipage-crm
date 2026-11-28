package handlers

// Test tích hợp: biến thể sản phẩm (Màu / Size...) — tạo, sửa, số tổng hợp, đặt hàng, trừ / hoàn kho từng biến thể.
//   TEST_DATABASE_URL=postgres://fbcrm:fbcrm_secret_password@localhost:5432/fbcrm_test?sslmode=disable \
//   go test ./internal/handlers -run 'Variant' -v

import (
	"fmt"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"fb-multipage-crm/internal/models"
)

func vtMap(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
}

func vtList(v any) []any {
	l, _ := v.([]any)
	return l
}

func vtNum(v any) int64 {
	f, _ := v.(float64)
	return int64(f)
}

func resetOrderLimiter() {
	orderLimiter.Lock()
	orderLimiter.hits = map[string][]time.Time{}
	orderLimiter.Unlock()
}

func shirtOptions() []map[string]any {
	return []map[string]any{
		{"name": "Màu", "values": []string{"Đen", "Trắng"}},
		{"name": "Size", "values": []string{"M", "L"}},
	}
}

// 4 biến thể: Đen/M 200k (còn 5), Đen/L 210k (hết), Trắng/M 190k (còn 3, có ảnh riêng), Trắng/L 190k (còn 2)
func shirtVariants() []map[string]any {
	return []map[string]any{
		{"option_values": []string{"Đen", "M"}, "price": 200000, "compare_price": 250000, "cost_price": 90000, "stock": 5, "sku": "AO-D-M"},
		{"option_values": []string{"Đen", "L"}, "price": 210000, "stock": 0, "sku": "AO-D-L"},
		{"option_values": []string{"Trắng", "M"}, "price": 190000, "compare_price": 220000, "cost_price": 80000, "stock": 3, "sku": "AO-T-M", "image_url": "https://cdn.example.com/white.jpg"},
		{"option_values": []string{"Trắng", "L"}, "price": 190000, "stock": 2, "sku": "AO-T-L"},
	}
}

func shirtBody(name string, opts, vars []map[string]any) map[string]any {
	return map[string]any{
		"name": name, "price": 1, "stock": 1, "is_active": true,
		"media":    []map[string]any{{"type": "image", "url": "https://cdn.example.com/cover.jpg"}},
		"options":  opts,
		"variants": vars,
	}
}

// createShirt tạo sản phẩm "Áo thun" có biến thể, trả id sản phẩm và map tiêu đề -> id biến thể
func (e *productEnv) createShirt(t *testing.T, vars []map[string]any) (uint, map[string]uint) {
	t.Helper()
	w, out := e.do("POST", e.base()+"/products", shirtBody("Áo thun", shirtOptions(), vars))
	if w.Code != 201 {
		t.Fatalf("tạo sản phẩm có biến thể: %d %s", w.Code, w.Body.String())
	}
	p := vtMap(out["product"])
	ids := map[string]uint{}
	for _, v := range vtList(p["variants"]) {
		vm := vtMap(v)
		ids[vm["title"].(string)] = uint(vtNum(vm["id"]))
	}
	return uint(vtNum(p["id"])), ids
}

func (e *productEnv) order(items []map[string]any) (*httptest.ResponseRecorder, map[string]any) {
	return e.do("POST", "/api/v1/storefront/public/orders", map[string]any{
		"slug": "test-shop", "items": items, "customer_name": "Khách Thử", "phone": "0912345678",
		"address": "1 Lê Lợi, Quận 1", "payment_method": "cod",
	})
}

func (e *productEnv) variantStock(id uint) int {
	var s int
	e.db.Raw("SELECT stock FROM product_variants WHERE id = ?", id).Scan(&s)
	return s
}

func (e *productEnv) productStock(id uint) int {
	var s int
	e.db.Raw("SELECT stock FROM products WHERE id = ?", id).Scan(&s)
	return s
}

func (e *productEnv) countVariants(pid uint) int64 {
	var n int64
	e.db.Raw("SELECT count(*) FROM product_variants WHERE product_id = ?", pid).Scan(&n)
	return n
}

func TestVariantCreateAndAggregates(t *testing.T) {
	e := newProductEnv(t)
	pid, ids := e.createShirt(t, shirtVariants())
	if len(ids) != 4 || ids["Đen / M"] == 0 || ids["Trắng / L"] == 0 {
		t.Fatalf("tiêu đề biến thể sai: %v", ids)
	}

	// Số tổng hợp ở sản phẩm: giá thấp nhất (kèm giá gốc / giá nhập của biến thể đó), tồn kho = tổng
	_, got := e.do("GET", fmt.Sprintf("%s/products/%d", e.base(), pid), nil)
	p := vtMap(got["product"])
	if vtNum(p["price"]) != 190000 || vtNum(p["compare_price"]) != 220000 || vtNum(p["cost_price"]) != 80000 || vtNum(p["stock"]) != 10 {
		t.Errorf("tổng hợp sai: price=%v compare=%v cost=%v stock=%v", p["price"], p["compare_price"], p["cost_price"], p["stock"])
	}
	if len(vtList(p["options"])) != 2 || len(vtList(p["variants"])) != 4 {
		t.Errorf("options/variants: %v / %v", p["options"], p["variants"])
	}
	first := vtMap(vtList(p["variants"])[0])
	if first["title"] != "Đen / M" || first["sku"] != "AO-D-M" || vtNum(first["cost_price"]) != 90000 {
		t.Errorf("biến thể đầu: %v", first)
	}

	// Danh sách quản lý (lite)
	_, lite := e.do("GET", e.base()+"/products?lite=1", nil)
	row := vtMap(vtList(lite["products"])[0])
	if vtNum(row["variant_count"]) != 4 || vtNum(row["price"]) != 190000 || vtNum(row["price_max"]) != 210000 {
		t.Errorf("lite: variant_count=%v price=%v price_max=%v", row["variant_count"], row["price"], row["price_max"])
	}

	// Trang công khai: có biến thể nhưng KHÔNG lộ giá nhập, số tồn kho, mã vạch
	w, pub := e.do("GET", fmt.Sprintf("/api/v1/storefront/public/products/%d?slug=test-shop", pid), nil)
	if w.Code != 200 {
		t.Fatalf("public product: %d %s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	for _, secret := range []string{"cost_price", "barcode", "\"stock\""} {
		if strings.Contains(body, secret) {
			t.Errorf("trang công khai lộ %q: %s", secret, body)
		}
	}
	pp := vtMap(pub["product"])
	vars := vtList(pp["variants"])
	if len(vtList(pp["options"])) != 2 || len(vars) != 4 {
		t.Fatalf("public options/variants: %s", body)
	}
	inStock := map[string]bool{}
	for _, v := range vars {
		vm := vtMap(v)
		inStock[vm["title"].(string)], _ = vm["in_stock"].(bool)
	}
	if inStock["Đen / L"] || !inStock["Đen / M"] || !inStock["Trắng / M"] || !inStock["Trắng / L"] {
		t.Errorf("in_stock theo biến thể: %v", inStock)
	}
	if vm := vtMap(vars[2]); vm["image_url"] != "https://cdn.example.com/white.jpg" || vtNum(vm["price"]) != 190000 {
		t.Errorf("biến thể Trắng / M: %v", vm)
	}

	// Thẻ sản phẩm ở trang chủ: "Từ X" + khoảng giá, không kèm danh sách biến thể
	_, home := e.do("GET", "/api/v1/storefront/public?slug=test-shop", nil)
	card := vtMap(vtList(home["products"])[0])
	if vtNum(card["variant_count"]) != 4 || vtNum(card["price"]) != 190000 || vtNum(card["price_max"]) != 210000 || card["in_stock"] != true {
		t.Errorf("thẻ sản phẩm: %v", card)
	}
	if _, has := card["variants"]; has {
		t.Errorf("thẻ sản phẩm không được kèm danh sách biến thể")
	}
}

func TestVariantValidation(t *testing.T) {
	e := newProductEnv(t)
	vals := func(n int, prefix string) []string {
		out := make([]string, n)
		for i := range out {
			out[i] = fmt.Sprintf("%s%d", prefix, i)
		}
		return out
	}
	one := func(price int, stock int, v ...string) map[string]any {
		return map[string]any{"option_values": v, "price": price, "stock": stock}
	}
	colorOnly := []map[string]any{{"name": "Màu", "values": []string{"Đen", "Trắng"}}}
	cases := []struct {
		name string
		opts []map[string]any
		vars []map[string]any
		want string
	}{
		{"quá 3 nhóm", []map[string]any{{"name": "A", "values": []string{"1"}}, {"name": "B", "values": []string{"1"}}, {"name": "C", "values": []string{"1"}}, {"name": "D", "values": []string{"1"}}},
			[]map[string]any{one(1, 1, "1", "1", "1", "1")}, "Tối đa 3 nhóm"},
		{"tên nhóm trùng", []map[string]any{{"name": "Màu", "values": []string{"Đen"}}, {"name": "màu", "values": []string{"Đỏ"}}},
			[]map[string]any{one(1, 1, "Đen", "Đỏ")}, "bị trùng"},
		{"nhóm không tên", []map[string]any{{"name": "", "values": []string{"Đen"}}}, []map[string]any{one(1, 1, "Đen")}, "đặt tên"},
		{"có nhóm mà không có biến thể", colorOnly, nil, "ít nhất một biến thể"},
		{"thiếu giá trị", shirtOptions(), []map[string]any{one(1, 1, "Đen")}, "thiếu giá trị"},
		{"giá trị lạ", colorOnly, []map[string]any{one(1, 1, "Xanh")}, "không thuộc nhóm Màu"},
		{"trùng tổ hợp (không phân biệt hoa thường)", colorOnly, []map[string]any{one(1, 1, "Đen"), one(2, 1, "đen")}, "bị trùng"},
		{"giá âm", colorOnly, []map[string]any{one(-1, 1, "Đen")}, "Giá của biến thể"},
		{"tồn kho < -1", colorOnly, []map[string]any{one(1, -5, "Đen")}, "Tồn kho"},
		{"101 giá trị", []map[string]any{{"name": "Mã", "values": vals(101, "m")}}, []map[string]any{one(1, 1, "m0")}, "tối đa 100 giá trị"},
		{"121 biến thể", []map[string]any{{"name": "A", "values": vals(11, "a")}, {"name": "B", "values": vals(11, "b")}}, func() []map[string]any {
			var out []map[string]any
			for i := 0; i < 11; i++ {
				for j := 0; j < 11; j++ {
					out = append(out, one(1, 1, fmt.Sprintf("a%d", i), fmt.Sprintf("b%d", j)))
				}
			}
			return out
		}(), "Tối đa 100 biến thể"},
	}
	for _, tc := range cases {
		w, out := e.do("POST", e.base()+"/products", shirtBody("SP lỗi", tc.opts, tc.vars))
		if w.Code != 400 || !strings.Contains(fmt.Sprint(out["error"]), tc.want) {
			t.Errorf("%s: mong 400 chứa %q, nhận %d %s", tc.name, tc.want, w.Code, w.Body.String())
		}
	}
	var n int64
	e.db.Raw("SELECT count(*) FROM products").Scan(&n)
	if n != 0 {
		t.Errorf("yêu cầu lỗi vẫn tạo %d sản phẩm", n)
	}

	// đúng 100 biến thể (10 x 10) vẫn hợp lệ
	var ok []map[string]any
	for i := 0; i < 10; i++ {
		for j := 0; j < 10; j++ {
			ok = append(ok, one(1000+i, 1, fmt.Sprintf("a%d", i), fmt.Sprintf("b%d", j)))
		}
	}
	w, _ := e.do("POST", e.base()+"/products", shirtBody("SP 100 biến thể", []map[string]any{{"name": "A", "values": vals(10, "a")}, {"name": "B", "values": vals(10, "b")}}, ok))
	if w.Code != 201 {
		t.Errorf("100 biến thể phải hợp lệ: %d %s", w.Code, w.Body.String())
	}
}

func TestVariantUpdateKeepsIDsAndSyncs(t *testing.T) {
	e := newProductEnv(t)
	pid, ids := e.createShirt(t, shirtVariants())
	url := fmt.Sprintf("%s/products/%d", e.base(), pid)

	// Sửa: giữ Đen/M (gửi kèm id), đổi giá Trắng/M (KHÔNG gửi id → khớp theo tổ hợp), bỏ Đen/L và Trắng/L, thêm nhóm giá trị "Xám"
	opts := []map[string]any{
		{"name": "Màu", "values": []string{"Đen", "Trắng", "Xám"}},
		{"name": "Size", "values": []string{"M", "L"}},
	}
	vars := []map[string]any{
		{"id": ids["Đen / M"], "option_values": []string{"Đen", "M"}, "price": 205000, "stock": 4, "sku": "AO-D-M"},
		{"option_values": []string{"trắng", "m"}, "price": 199000, "stock": 9, "sku": "AO-T-M"}, // sai hoa thường vẫn khớp
		{"option_values": []string{"Xám", "M"}, "price": 150000, "stock": 7, "sku": "AO-X-M"},
	}
	w, out := e.do("PUT", url, shirtBody("Áo thun v2", opts, vars))
	if w.Code != 200 {
		t.Fatalf("sửa: %d %s", w.Code, w.Body.String())
	}
	p := vtMap(out["product"])
	if vtNum(p["price"]) != 150000 || vtNum(p["stock"]) != 20 {
		t.Errorf("tổng hợp sau khi sửa: price=%v stock=%v", p["price"], p["stock"])
	}
	if e.countVariants(pid) != 3 {
		t.Errorf("số biến thể trong DB = %d, mong 3", e.countVariants(pid))
	}
	_, got := e.do("GET", url, nil)
	byTitle := map[string]map[string]any{}
	for _, v := range vtList(vtMap(got["product"])["variants"]) {
		vm := vtMap(v)
		byTitle[vm["title"].(string)] = vm
	}
	if uint(vtNum(byTitle["Đen / M"]["id"])) != ids["Đen / M"] || vtNum(byTitle["Đen / M"]["price"]) != 205000 {
		t.Errorf("Đen / M phải giữ id %d và có giá mới: %v", ids["Đen / M"], byTitle["Đen / M"])
	}
	if uint(vtNum(byTitle["Trắng / M"]["id"])) != ids["Trắng / M"] || vtNum(byTitle["Trắng / M"]["price"]) != 199000 {
		t.Errorf("Trắng / M phải giữ id khi khớp theo tổ hợp: %v", byTitle["Trắng / M"])
	}
	if byTitle["Xám / M"] == nil || byTitle["Đen / L"] != nil || byTitle["Trắng / L"] != nil {
		t.Errorf("thêm / xoá biến thể sai: %v", byTitle)
	}
	var orphans int64
	e.db.Raw("SELECT count(*) FROM product_variants WHERE id IN (?, ?)", ids["Đen / L"], ids["Trắng / L"]).Scan(&orphans)
	if orphans != 0 {
		t.Errorf("biến thể bị bỏ vẫn còn trong DB")
	}
}

func TestVariantStaleClientAndRemoval(t *testing.T) {
	e := newProductEnv(t)
	pid, _ := e.createShirt(t, shirtVariants())
	url := fmt.Sprintf("%s/products/%d", e.base(), pid)

	// Client cũ chỉ gửi trường đơn giản (không có options / variants): biến thể phải còn nguyên, giá / kho vẫn là số tổng hợp
	w, out := e.do("PUT", url, map[string]any{"name": "Áo thun (đổi tên)", "price": 1, "stock": 77, "description": "Mô tả mới"})
	if w.Code != 200 {
		t.Fatalf("PUT client cũ: %d %s", w.Code, w.Body.String())
	}
	p := vtMap(out["product"])
	if p["name"] != "Áo thun (đổi tên)" || vtNum(p["price"]) != 190000 || vtNum(p["compare_price"]) != 220000 || vtNum(p["stock"]) != 10 ||
		len(vtList(p["variants"])) != 4 || len(vtList(p["options"])) != 2 {
		t.Errorf("client cũ làm hỏng biến thể: %v", p)
	}
	if e.countVariants(pid) != 4 {
		t.Errorf("client cũ xoá nhầm biến thể: còn %d", e.countVariants(pid))
	}

	// Đổi hiển thị (PATCH) không đụng tới biến thể
	if w, _ := e.do("PATCH", url, map[string]any{"is_active": false}); w.Code != 200 || e.countVariants(pid) != 4 {
		t.Errorf("PATCH ảnh hưởng biến thể: %d, còn %d", w.Code, e.countVariants(pid))
	}

	// Chủ shop chủ động bỏ biến thể: options = [] → xoá hết, giá / kho lấy từ form
	w, out = e.do("PUT", url, map[string]any{"name": "Áo thun đơn", "price": 123000, "stock": 9, "options": []map[string]any{}, "variants": []map[string]any{}})
	if w.Code != 200 {
		t.Fatalf("bỏ biến thể: %d %s", w.Code, w.Body.String())
	}
	p = vtMap(out["product"])
	if vtNum(p["price"]) != 123000 || vtNum(p["stock"]) != 9 || len(vtList(p["options"])) != 0 || len(vtList(p["variants"])) != 0 || e.countVariants(pid) != 0 {
		t.Errorf("sau khi bỏ biến thể: %v (còn %d dòng)", p, e.countVariants(pid))
	}
	if p["options"] == nil || p["variants"] == nil {
		t.Errorf("options / variants phải là [] chứ không phải null: %v", p)
	}
}

func TestVariantDeleteRemovesRows(t *testing.T) {
	e := newProductEnv(t)
	pid, _ := e.createShirt(t, shirtVariants())
	other, _ := e.createShirt(t, shirtVariants())
	if w, _ := e.do("DELETE", fmt.Sprintf("%s/products/%d", e.base(), pid), nil); w.Code != 200 {
		t.Fatalf("xoá: %d", w.Code)
	}
	if e.countVariants(pid) != 0 || e.countVariants(other) != 4 {
		t.Errorf("sau khi xoá: sản phẩm bị xoá còn %d dòng, sản phẩm khác còn %d (mong 0 và 4)", e.countVariants(pid), e.countVariants(other))
	}
	if w, _ := e.do("DELETE", fmt.Sprintf("%s/products/%d", e.base(), pid), nil); w.Code != 404 {
		t.Errorf("xoá lần hai phải 404: %d", w.Code)
	}
}

func TestVariantOrderValidation(t *testing.T) {
	e := newProductEnv(t)
	pid, ids := e.createShirt(t, shirtVariants())
	_, plain := e.do("POST", e.base()+"/products", map[string]any{"name": "Cốc sứ", "price": 50000, "stock": 10, "is_active": true})
	plainID := vtNum(vtMap(plain["product"])["id"])
	_, otherIDs := e.createShirt(t, shirtVariants())

	cases := []struct {
		name  string
		items []map[string]any
		code  int
		want  string
	}{
		{"thiếu variant_id", []map[string]any{{"product_id": pid, "qty": 1}}, 400, "Vui lòng chọn phân loại"},
		{"biến thể của sản phẩm khác", []map[string]any{{"product_id": pid, "variant_id": otherIDs["Đen / M"], "qty": 1}}, 400, "không còn bán"},
		{"biến thể không tồn tại", []map[string]any{{"product_id": pid, "variant_id": 999999, "qty": 1}}, 400, "không còn bán"},
		{"biến thể hết hàng", []map[string]any{{"product_id": pid, "variant_id": ids["Đen / L"], "qty": 1}}, 409, "không đủ hàng"},
		{"vượt tồn kho biến thể", []map[string]any{{"product_id": pid, "variant_id": ids["Đen / M"], "qty": 6}}, 409, "còn 5"},
		{"gộp dòng trùng vượt tồn kho", []map[string]any{{"product_id": pid, "variant_id": ids["Đen / M"], "qty": 3}, {"product_id": pid, "variant_id": ids["Đen / M"], "qty": 3}}, 409, "còn 5"},
		{"sản phẩm thường nhưng gửi variant_id", []map[string]any{{"product_id": plainID, "variant_id": ids["Đen / M"], "qty": 1}}, 400, "không còn bán"},
	}
	for _, tc := range cases {
		resetOrderLimiter()
		w, out := e.order(tc.items)
		if w.Code != tc.code || !strings.Contains(fmt.Sprint(out["error"]), tc.want) {
			t.Errorf("%s: mong %d chứa %q, nhận %d %s", tc.name, tc.code, tc.want, w.Code, w.Body.String())
		}
	}
	if e.variantStock(ids["Đen / M"]) != 5 || e.productStock(pid) != 10 {
		t.Errorf("đơn bị từ chối không được đổi tồn kho: biến thể=%d, sản phẩm=%d", e.variantStock(ids["Đen / M"]), e.productStock(pid))
	}
	var n int64
	e.db.Raw("SELECT count(*) FROM shop_orders").Scan(&n)
	if n != 0 {
		t.Errorf("đơn bị từ chối vẫn tạo %d đơn", n)
	}
}

func TestVariantOrderStockAndRestock(t *testing.T) {
	e := newProductEnv(t)
	pid, ids := e.createShirt(t, shirtVariants())
	resetOrderLimiter()

	// 2 x Đen/M (+ dòng trùng gộp lại thành 3) và 1 x Trắng/M có ảnh riêng
	w, out := e.order([]map[string]any{
		{"product_id": pid, "variant_id": ids["Đen / M"], "qty": 2},
		{"product_id": pid, "variant_id": ids["Trắng / M"], "qty": 1},
		{"product_id": pid, "variant_id": ids["Đen / M"], "qty": 1},
	})
	if w.Code != 201 {
		t.Fatalf("đặt hàng: %d %s", w.Code, w.Body.String())
	}
	code, token := out["code"].(string), out["token"].(string)
	if vtNum(out["total"]) != 3*200000+190000 {
		t.Errorf("tổng tiền = %v, mong %d", out["total"], 3*200000+190000)
	}
	if e.variantStock(ids["Đen / M"]) != 2 || e.variantStock(ids["Trắng / M"]) != 2 || e.productStock(pid) != 2+0+2+2 {
		t.Errorf("tồn kho sau khi đặt: Đen/M=%d Trắng/M=%d sản phẩm=%d", e.variantStock(ids["Đen / M"]), e.variantStock(ids["Trắng / M"]), e.productStock(pid))
	}

	// Đơn lưu lại id, tên biến thể, SKU, ảnh (biến thể không có ảnh dùng ảnh bìa sản phẩm)
	_, got := e.do("GET", "/api/v1/storefront/public/orders/"+code+"?t="+token, nil)
	items := vtList(vtMap(got["order"])["items"])
	if len(items) != 2 {
		t.Fatalf("đơn phải có 2 dòng (đã gộp): %v", items)
	}
	l0, l1 := vtMap(items[0]), vtMap(items[1])
	if l0["variant_title"] != "Đen / M" || vtNum(l0["qty"]) != 3 || vtNum(l0["price"]) != 200000 || vtNum(l0["line_total"]) != 600000 ||
		l0["sku"] != "AO-D-M" || uint(vtNum(l0["variant_id"])) != ids["Đen / M"] || l0["image_url"] != "https://cdn.example.com/cover.jpg" {
		t.Errorf("dòng 1: %v", l0)
	}
	if l1["variant_title"] != "Trắng / M" || vtNum(l1["price"]) != 190000 || l1["image_url"] != "https://cdn.example.com/white.jpg" {
		t.Errorf("dòng 2: %v", l1)
	}

	// Đổi giá / tên biến thể sau đó không làm thay đổi đơn đã đặt
	e.do("PUT", fmt.Sprintf("%s/products/%d", e.base(), pid), shirtBody("Áo thun", shirtOptions(), []map[string]any{
		{"option_values": []string{"Đen", "M"}, "price": 999000, "stock": 2},
		{"option_values": []string{"Trắng", "M"}, "price": 190000, "stock": 2},
	}))
	_, got = e.do("GET", "/api/v1/storefront/public/orders/"+code+"?t="+token, nil)
	if l := vtMap(vtList(vtMap(got["order"])["items"])[0]); vtNum(l["price"]) != 200000 || l["variant_title"] != "Đen / M" {
		t.Errorf("đơn cũ bị đổi theo biến thể: %v", l)
	}
}

// Huỷ đơn hoàn kho đúng biến thể; mở lại đơn trừ kho lại; số tổng hợp luôn khớp tổng các biến thể
func TestVariantCancelAndReopenOrder(t *testing.T) {
	e := newProductEnv(t)
	pid, ids := e.createShirt(t, shirtVariants())
	resetOrderLimiter()
	w, out := e.order([]map[string]any{
		{"product_id": pid, "variant_id": ids["Đen / M"], "qty": 2},
		{"product_id": pid, "variant_id": ids["Trắng / L"], "qty": 2},
	})
	if w.Code != 201 {
		t.Fatalf("đặt hàng: %d %s", w.Code, w.Body.String())
	}
	var o ShopOrder
	e.db.Where("code = ?", out["code"]).First(&o)
	if e.variantStock(ids["Đen / M"]) != 3 || e.variantStock(ids["Trắng / L"]) != 0 || e.productStock(pid) != 3+0+3+0 {
		t.Fatalf("sau khi đặt: %d %d %d", e.variantStock(ids["Đen / M"]), e.variantStock(ids["Trắng / L"]), e.productStock(pid))
	}
	patch := func(status string) {
		t.Helper()
		if w, _ := e.do("PATCH", fmt.Sprintf("%s/orders/%d", e.base(), o.ID), map[string]any{"status": status}); w.Code != 200 {
			t.Fatalf("PATCH %s: %d %s", status, w.Code, w.Body.String())
		}
	}
	patch("cancelled")
	if e.variantStock(ids["Đen / M"]) != 5 || e.variantStock(ids["Trắng / L"]) != 2 || e.productStock(pid) != 5+0+3+2 {
		t.Errorf("sau khi huỷ: Đen/M=%d Trắng/L=%d sản phẩm=%d (mong 5, 2, 10)", e.variantStock(ids["Đen / M"]), e.variantStock(ids["Trắng / L"]), e.productStock(pid))
	}
	patch("confirmed")
	if e.variantStock(ids["Đen / M"]) != 3 || e.variantStock(ids["Trắng / L"]) != 0 || e.productStock(pid) != 6 {
		t.Errorf("sau khi mở lại: Đen/M=%d Trắng/L=%d sản phẩm=%d (mong 3, 0, 6)", e.variantStock(ids["Đen / M"]), e.variantStock(ids["Trắng / L"]), e.productStock(pid))
	}

	// Mở lại khi biến thể đã hết hàng (có người khác mua mất) thì báo lỗi và không đổi gì
	patch("cancelled")
	resetOrderLimiter()
	if w, _ := e.order([]map[string]any{{"product_id": pid, "variant_id": ids["Trắng / L"], "qty": 2}}); w.Code != 201 {
		t.Fatalf("đơn thứ hai: %d %s", w.Code, w.Body.String())
	}
	w, _ = e.do("PATCH", fmt.Sprintf("%s/orders/%d", e.base(), o.ID), map[string]any{"status": "confirmed"})
	if w.Code != 409 || !strings.Contains(w.Body.String(), "vừa hết hàng") {
		t.Errorf("mở lại đơn khi hết hàng: %d %s", w.Code, w.Body.String())
	}
	if e.variantStock(ids["Đen / M"]) != 5 || e.variantStock(ids["Trắng / L"]) != 0 {
		t.Errorf("mở lại thất bại không được trừ dở: Đen/M=%d Trắng/L=%d", e.variantStock(ids["Đen / M"]), e.variantStock(ids["Trắng / L"]))
	}
}

// Nhiều đơn cùng lúc cho cùng một biến thể: không bán quá tồn kho, số tổng hợp của sản phẩm luôn đúng
func TestVariantConcurrentOrdersNeverOversell(t *testing.T) {
	e := newProductEnv(t)
	vars := []map[string]any{
		{"option_values": []string{"Đen", "M"}, "price": 100000, "stock": 3},
		{"option_values": []string{"Đen", "L"}, "price": 100000, "stock": 3},
		{"option_values": []string{"Trắng", "M"}, "price": 100000, "stock": 0},
		{"option_values": []string{"Trắng", "L"}, "price": 100000, "stock": 0},
	}
	pid, ids := e.createShirt(t, vars)
	resetOrderLimiter()

	var wg sync.WaitGroup
	var mu sync.Mutex
	okByVariant := map[string]int{}
	failByVariant := map[string]int{}
	for i := 0; i < 8; i++ {
		title := "Đen / M"
		if i%2 == 1 {
			title = "Đen / L"
		}
		wg.Add(1)
		go func(title string) {
			defer wg.Done()
			w, _ := e.order([]map[string]any{{"product_id": pid, "variant_id": ids[title], "qty": 1}})
			mu.Lock()
			defer mu.Unlock()
			switch w.Code {
			case 201:
				okByVariant[title]++
			case 409:
				failByVariant[title]++
			default:
				t.Errorf("%s: mã bất ngờ %d %s", title, w.Code, w.Body.String())
			}
		}(title)
	}
	wg.Wait()
	if okByVariant["Đen / M"] != 3 || okByVariant["Đen / L"] != 3 || failByVariant["Đen / M"] != 1 || failByVariant["Đen / L"] != 1 {
		t.Errorf("thành công %v, thất bại %v (mong mỗi biến thể 3 đơn thành công, 1 bị từ chối)", okByVariant, failByVariant)
	}
	if e.variantStock(ids["Đen / M"]) != 0 || e.variantStock(ids["Đen / L"]) != 0 || e.productStock(pid) != 0 {
		t.Errorf("tồn kho cuối: Đen/M=%d Đen/L=%d sản phẩm=%d (mong 0)", e.variantStock(ids["Đen / M"]), e.variantStock(ids["Đen / L"]), e.productStock(pid))
	}
	// Hết sạch hàng: thẻ sản phẩm báo hết
	_, home := e.do("GET", "/api/v1/storefront/public?slug=test-shop", nil)
	if card := vtMap(vtList(home["products"])[0]); card["in_stock"] != false {
		t.Errorf("sản phẩm hết hàng phải in_stock=false: %v", card)
	}
}

// Biến thể không giới hạn (-1) không bị trừ; số tổng hợp là -1 (không giới hạn)
func TestVariantUnlimitedStock(t *testing.T) {
	e := newProductEnv(t)
	vars := []map[string]any{
		{"option_values": []string{"Đen", "M"}, "price": 100000, "stock": -1},
		{"option_values": []string{"Đen", "L"}, "price": 100000, "stock": 2},
		{"option_values": []string{"Trắng", "M"}, "price": 100000, "stock": 0},
		{"option_values": []string{"Trắng", "L"}, "price": 100000, "stock": 0},
	}
	pid, ids := e.createShirt(t, vars)
	if e.productStock(pid) != -1 {
		t.Fatalf("tồn kho tổng hợp = %d, mong -1", e.productStock(pid))
	}
	resetOrderLimiter()
	if w, _ := e.order([]map[string]any{{"product_id": pid, "variant_id": ids["Đen / M"], "qty": 50}}); w.Code != 201 {
		t.Fatalf("đặt biến thể không giới hạn: %d %s", w.Code, w.Body.String())
	}
	if e.variantStock(ids["Đen / M"]) != -1 || e.productStock(pid) != -1 {
		t.Errorf("biến thể không giới hạn bị trừ: %d, sản phẩm %d", e.variantStock(ids["Đen / M"]), e.productStock(pid))
	}
	if w, _ := e.order([]map[string]any{{"product_id": pid, "variant_id": ids["Đen / L"], "qty": 2}}); w.Code != 201 {
		t.Fatalf("đặt biến thể giới hạn: %d", w.Code)
	}
	if e.variantStock(ids["Đen / L"]) != 0 || e.productStock(pid) != -1 {
		t.Errorf("sau khi bán hết biến thể giới hạn: %d, sản phẩm %d", e.variantStock(ids["Đen / L"]), e.productStock(pid))
	}
}

// Đơn đặt TRƯỚC khi sản phẩm chuyển sang biến thể (dòng không có variant_id): huỷ / mở lại không làm lệch số tổng hợp
func TestVariantLegacyOrderAfterConversion(t *testing.T) {
	e := newProductEnv(t)
	_, plain := e.do("POST", e.base()+"/products", map[string]any{"name": "Áo cũ", "price": 100000, "stock": 10, "is_active": true})
	pid := uint(vtNum(vtMap(plain["product"])["id"]))
	resetOrderLimiter()
	w, out := e.order([]map[string]any{{"product_id": pid, "qty": 2}})
	if w.Code != 201 || e.productStock(pid) != 8 {
		t.Fatalf("đơn sản phẩm thường: %d %s, kho=%d", w.Code, w.Body.String(), e.productStock(pid))
	}
	var o ShopOrder
	e.db.Where("code = ?", out["code"]).First(&o)

	// Cửa hàng chuyển sản phẩm sang biến thể
	if w, _ := e.do("PUT", fmt.Sprintf("%s/products/%d", e.base(), pid), shirtBody("Áo cũ", shirtOptions(), shirtVariants())); w.Code != 200 {
		t.Fatalf("chuyển sang biến thể: %d %s", w.Code, w.Body.String())
	}
	if e.productStock(pid) != 10 {
		t.Fatalf("tổng hợp sau khi chuyển: %d", e.productStock(pid))
	}
	if w, _ := e.do("PATCH", fmt.Sprintf("%s/orders/%d", e.base(), o.ID), map[string]any{"status": "cancelled"}); w.Code != 200 {
		t.Fatalf("huỷ đơn cũ: %d", w.Code)
	}
	if e.productStock(pid) != 10 {
		t.Errorf("huỷ đơn cũ làm lệch số tổng hợp: %d (mong 10)", e.productStock(pid))
	}
	var sum int
	e.db.Raw("SELECT COALESCE(SUM(stock), 0) FROM product_variants WHERE product_id = ?", pid).Scan(&sum)
	if sum != 10 {
		t.Errorf("tổng tồn kho biến thể = %d", sum)
	}
}

// Sản phẩm không có biến thể vẫn hoạt động như cũ và public API trả options / variants rỗng
func TestVariantPlainProductUnchanged(t *testing.T) {
	e := newProductEnv(t)
	w, out := e.do("POST", e.base()+"/products", map[string]any{"name": "Cốc sứ", "price": 50000, "compare_price": 70000, "stock": 5, "is_active": true})
	if w.Code != 201 {
		t.Fatalf("tạo: %d %s", w.Code, w.Body.String())
	}
	p := vtMap(out["product"])
	pid := uint(vtNum(p["id"]))
	if len(vtList(p["options"])) != 0 || len(vtList(p["variants"])) != 0 || vtNum(p["price"]) != 50000 || vtNum(p["stock"]) != 5 {
		t.Errorf("sản phẩm thường: %v", p)
	}
	_, pub := e.do("GET", fmt.Sprintf("/api/v1/storefront/public/products/%d?slug=test-shop", pid), nil)
	pp := vtMap(pub["product"])
	if len(vtList(pp["options"])) != 0 || len(vtList(pp["variants"])) != 0 || vtNum(pp["price"]) != 50000 {
		t.Errorf("public sản phẩm thường: %v", pp)
	}
	_, home := e.do("GET", "/api/v1/storefront/public?slug=test-shop", nil)
	card := vtMap(vtList(home["products"])[0])
	if vtNum(card["variant_count"]) != 0 || vtNum(card["price_max"]) != 50000 {
		t.Errorf("thẻ sản phẩm thường: %v", card)
	}
	resetOrderLimiter()
	if w, _ := e.order([]map[string]any{{"product_id": pid, "qty": 2}}); w.Code != 201 || e.productStock(pid) != 3 {
		t.Errorf("đặt sản phẩm thường: %d, kho=%d", w.Code, e.productStock(pid))
	}
}

// Danh sách công khai kèm thẻ của từng sản phẩm để section Sản phẩm lọc theo thẻ
func TestPublicCardsCarryTags(t *testing.T) {
	e := newProductEnv(t)
	e.do("POST", e.base()+"/products", map[string]any{"name": "Có thẻ", "price": 1000, "stock": -1, "is_active": true, "tags": "Bán chạy, khuyến mãi , bán chạy"})
	e.do("POST", e.base()+"/products", map[string]any{"name": "Không thẻ", "price": 1000, "stock": -1, "is_active": true})
	_, home := e.do("GET", "/api/v1/storefront/public?slug=test-shop", nil)
	byName := map[string][]any{}
	for _, p := range vtList(home["products"]) {
		m := vtMap(p)
		byName[m["name"].(string)] = vtList(m["tags"])
	}
	if len(byName["Có thẻ"]) != 2 || byName["Có thẻ"][0] != "Bán chạy" || byName["Có thẻ"][1] != "khuyến mãi" {
		t.Errorf("thẻ (đã bỏ trùng, giữ thứ tự): %v", byName["Có thẻ"])
	}
	if tags, ok := byName["Không thẻ"]; !ok || len(tags) != 0 {
		t.Errorf("sản phẩm không thẻ phải có tags = []: %v", byName)
	}
}

// sản phẩm cũ (cột options_json NULL) và biến thể mồ côi không làm hỏng trang công khai
func TestVariantNullOptionsAndOrphans(t *testing.T) {
	e := newProductEnv(t)
	if err := e.db.Exec(`INSERT INTO products (storefront_id, user_id, name, description, price, compare_price, image_url, stock, is_active, sort_order, created_at, updated_at)
		VALUES (?, 7, 'SP cũ', 'Mô tả', 50000, 0, '', 4, true, 0, now(), now())`, e.sf.ID).Error; err != nil {
		t.Fatal(err)
	}
	var id uint
	e.db.Raw(`SELECT id FROM products WHERE name = 'SP cũ'`).Scan(&id)
	// biến thể mồ côi: có dòng biến thể nhưng sản phẩm không có nhóm phân loại
	e.db.Create(&models.ProductVariant{ProductID: id, StorefrontID: e.sf.ID, Title: "Mồ côi", Option1: "x", Price: 1, Stock: 1})
	w, out := e.do("GET", fmt.Sprintf("/api/v1/storefront/public/products/%d?slug=test-shop", id), nil)
	if w.Code != 200 {
		t.Fatalf("public: %d %s", w.Code, w.Body.String())
	}
	pp := vtMap(out["product"])
	if len(vtList(pp["options"])) != 0 || len(vtList(pp["variants"])) != 0 {
		t.Errorf("sản phẩm không có nhóm phân loại không được hiện biến thể: %v", pp)
	}
	_, home := e.do("GET", "/api/v1/storefront/public?slug=test-shop", nil)
	if card := vtMap(vtList(home["products"])[0]); vtNum(card["variant_count"]) != 0 {
		t.Errorf("thẻ sản phẩm không có nhóm phân loại không được báo có biến thể: %v", card)
	}
	resetOrderLimiter()
	if w, _ := e.order([]map[string]any{{"product_id": id, "qty": 1}}); w.Code != 201 || e.productStock(id) != 3 {
		t.Errorf("đặt sản phẩm không có nhóm phân loại: %d, kho=%d (mong 3)", w.Code, e.productStock(id))
	}
}
