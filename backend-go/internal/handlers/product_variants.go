package handlers

// Biến thể sản phẩm (kiểu Shopify): tối đa 3 nhóm phân loại (Màu, Size...) và mỗi tổ hợp là một biến thể có giá, tồn kho, SKU, ảnh riêng.
// Khi sản phẩm có biến thể: đặt hàng lấy giá / tồn kho từ biến thể; Product.Price (thấp nhất) và Product.Stock (tổng) chỉ là
// số tổng hợp để hiển thị ở danh sách. Giỏ hàng / đơn hàng ghi lại variant_id + tên biến thể.

import (
	"fmt"
	"sort"
	"strings"

	"fb-multipage-crm/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const (
	maxVariantsPerProduct = 100
	maxOptionGroups       = 3
	maxValuesPerOption    = 100
)

type variantReq struct {
	ID           uint     `json:"id"`
	OptionValues []string `json:"option_values"`
	SKU          string   `json:"sku"`
	Barcode      string   `json:"barcode"`
	Price        int64    `json:"price"`
	ComparePrice int64    `json:"compare_price"`
	CostPrice    int64    `json:"cost_price"`
	Stock        int      `json:"stock"`
	ImageURL     string   `json:"image_url"`
}

// cleanProductOptions chuẩn hoá các nhóm phân loại: bỏ nhóm rỗng, khử trùng lặp (không phân biệt hoa thường), giới hạn độ dài.
func cleanProductOptions(in []models.ProductOption) ([]models.ProductOption, string) {
	out := []models.ProductOption{}
	names := map[string]bool{}
	for _, o := range in {
		name := truncRunes(strings.TrimSpace(o.Name), 40)
		seen := map[string]bool{}
		vals := []string{}
		for _, v := range o.Values {
			v = truncRunes(strings.TrimSpace(v), 60)
			if v == "" || seen[strings.ToLower(v)] {
				continue
			}
			seen[strings.ToLower(v)] = true
			vals = append(vals, v)
		}
		if name == "" && len(vals) == 0 {
			continue // dòng trống
		}
		if name == "" {
			return nil, "Hãy đặt tên cho nhóm phân loại (vd. Màu, Size)."
		}
		if len(vals) == 0 {
			continue // nhóm chưa có giá trị: bỏ
		}
		if names[strings.ToLower(name)] {
			return nil, fmt.Sprintf("Tên nhóm phân loại bị trùng: %s.", name)
		}
		names[strings.ToLower(name)] = true
		if len(vals) > maxValuesPerOption {
			return nil, fmt.Sprintf("Mỗi nhóm phân loại tối đa %d giá trị.", maxValuesPerOption)
		}
		out = append(out, models.ProductOption{Name: name, Values: vals})
	}
	if len(out) > maxOptionGroups {
		return nil, fmt.Sprintf("Tối đa %d nhóm phân loại.", maxOptionGroups)
	}
	return out, ""
}

func variantKey(vals []string) string {
	l := make([]string, len(vals))
	for i, v := range vals {
		l[i] = strings.ToLower(v)
	}
	return strings.Join(l, "\x1f")
}

// buildVariants kiểm tra danh sách biến thể gửi lên so với các nhóm phân loại và trả về bản đã chuẩn hoá (ID giữ lại để so khớp khi lưu).
func buildVariants(opts []models.ProductOption, in []variantReq) ([]models.ProductVariant, string) {
	if len(opts) == 0 {
		return nil, ""
	}
	if len(in) == 0 {
		return nil, "Hãy thêm ít nhất một biến thể cho các nhóm phân loại."
	}
	if len(in) > maxVariantsPerProduct {
		return nil, fmt.Sprintf("Tối đa %d biến thể cho mỗi sản phẩm (hiện có %d). Hãy bớt giá trị phân loại.", maxVariantsPerProduct, len(in))
	}
	canon := make([]map[string]string, len(opts)) // chữ thường -> giá trị chuẩn
	for i, o := range opts {
		canon[i] = map[string]string{}
		for _, v := range o.Values {
			canon[i][strings.ToLower(v)] = v
		}
	}
	seen := map[string]bool{}
	out := make([]models.ProductVariant, 0, len(in))
	for i, r := range in {
		if len(r.OptionValues) != len(opts) {
			return nil, "Có biến thể thiếu giá trị phân loại."
		}
		vals := make([]string, len(opts))
		for k, raw := range r.OptionValues {
			v, ok := canon[k][strings.ToLower(truncRunes(strings.TrimSpace(raw), 60))]
			if !ok {
				return nil, fmt.Sprintf("Giá trị phân loại \"%s\" không thuộc nhóm %s.", raw, opts[k].Name)
			}
			vals[k] = v
		}
		key := variantKey(vals)
		if seen[key] {
			return nil, fmt.Sprintf("Biến thể bị trùng: %s.", strings.Join(vals, " / "))
		}
		seen[key] = true
		switch {
		case r.Price < 0 || r.Price > 10000000000 || r.ComparePrice < 0 || r.ComparePrice > 10000000000 || r.CostPrice < 0 || r.CostPrice > 10000000000:
			return nil, fmt.Sprintf("Giá của biến thể %s không hợp lệ.", strings.Join(vals, " / "))
		case r.Stock < -1 || r.Stock > 1000000:
			return nil, fmt.Sprintf("Tồn kho của biến thể %s không hợp lệ (dùng -1 nếu không giới hạn).", strings.Join(vals, " / "))
		}
		v := models.ProductVariant{
			ID: r.ID, Position: i, Title: strings.Join(vals, " / "),
			SKU: truncRunes(strings.TrimSpace(r.SKU), 100), Barcode: truncRunes(strings.TrimSpace(r.Barcode), 100),
			Price: r.Price, ComparePrice: r.ComparePrice, CostPrice: r.CostPrice, Stock: r.Stock, ImageURL: safeMediaURL(r.ImageURL),
		}
		v.Option1 = vals[0]
		if len(vals) > 1 {
			v.Option2 = vals[1]
		}
		if len(vals) > 2 {
			v.Option3 = vals[2]
		}
		out = append(out, v)
	}
	return out, ""
}

// aggregateVariants tính số tổng hợp lưu ở sản phẩm: giá thấp nhất (kèm giá gốc / giá nhập của biến thể đó) và tồn kho (-1 nếu có biến thể không giới hạn).
func aggregateVariants(vs []models.ProductVariant) (price, compare, cost int64, stock int) {
	if len(vs) == 0 {
		return
	}
	minIdx := 0
	unlimited := false
	total := 0
	for i, v := range vs {
		if v.Price < vs[minIdx].Price {
			minIdx = i
		}
		if v.Stock < 0 {
			unlimited = true
		} else {
			total += v.Stock
		}
	}
	price, cost = vs[minIdx].Price, vs[minIdx].CostPrice
	if vs[minIdx].ComparePrice > price {
		compare = vs[minIdx].ComparePrice
	}
	stock = total
	if unlimited {
		stock = -1
	}
	return
}

func loadVariants(db *gorm.DB, productID uint) []models.ProductVariant {
	var vs []models.ProductVariant
	db.Where("product_id = ?", productID).Order("position ASC, id ASC").Find(&vs)
	if vs == nil {
		vs = []models.ProductVariant{}
	}
	return vs
}

// syncVariants ghi danh sách biến thể của sản phẩm: cập nhật cái đã có (khớp theo ID, rồi theo tổ hợp giá trị), thêm cái mới, xoá cái bị bỏ.
// vs = nil → xoá hết biến thể.
func syncVariants(tx *gorm.DB, p *models.Product, vs []models.ProductVariant) error {
	var existing []models.ProductVariant
	if err := tx.Where("product_id = ?", p.ID).Find(&existing).Error; err != nil {
		return err
	}
	byID := map[uint]models.ProductVariant{}
	byCombo := map[string]models.ProductVariant{}
	for _, e := range existing {
		byID[e.ID] = e
		byCombo[variantKey([]string{e.Option1, e.Option2, e.Option3})] = e
	}
	keep := []uint{}
	for i := range vs {
		v := &vs[i]
		old, ok := byID[v.ID]
		if !ok || v.ID == 0 {
			old, ok = byCombo[variantKey([]string{v.Option1, v.Option2, v.Option3})]
		}
		v.ProductID, v.StorefrontID = p.ID, p.StorefrontID
		if ok {
			v.ID, v.CreatedAt = old.ID, old.CreatedAt
			if err := tx.Save(v).Error; err != nil {
				return err
			}
		} else {
			v.ID = 0
			if err := tx.Create(v).Error; err != nil {
				return err
			}
		}
		keep = append(keep, v.ID)
	}
	q := tx.Where("product_id = ?", p.ID)
	if len(keep) > 0 {
		q = q.Where("id NOT IN ?", keep)
	}
	return q.Delete(&models.ProductVariant{}).Error
}

// productHasVariants: sản phẩm đang bán theo biến thể khi có cả nhóm phân loại lẫn dòng biến thể
// (cùng quy tắc với lúc đặt hàng và trang công khai; chịu được cột options_json NULL của sản phẩm cũ).
func productHasVariants(tx *gorm.DB, productID uint) bool {
	var n int64
	tx.Raw(`SELECT COUNT(*) FROM products p WHERE p.id = ? AND COALESCE(p.options_json, '') <> '' AND EXISTS (SELECT 1 FROM product_variants v WHERE v.product_id = p.id)`, productID).Scan(&n)
	return n > 0
}

// recomputeProductStock cập nhật tồn kho tổng hợp của sản phẩm theo các biến thể (gọi sau khi trừ / hoàn kho từng biến thể).
func recomputeProductStock(tx *gorm.DB, productID uint) {
	_ = tx.Exec(`UPDATE products SET stock = COALESCE((SELECT CASE WHEN MIN(stock) < 0 THEN -1 ELSE SUM(stock) END FROM product_variants WHERE product_id = ?), 0) WHERE id = ?`, productID, productID).Error
}

type variantStat struct {
	N   int
	Min int64
	Max int64
}

// variantStats: số biến thể và khoảng giá của mọi sản phẩm trong website (1 truy vấn)
func (h *StorefrontHandler) variantStats(siteID uint) map[uint]variantStat {
	type row struct {
		ProductID uint
		N         int
		Lo        int64
		Hi        int64
	}
	var rows []row
	// Chỉ tính sản phẩm đang có nhóm phân loại (cùng quy tắc với trang chi tiết và lúc đặt hàng)
	h.DB.Table("product_variants AS v").
		Select("v.product_id AS product_id, COUNT(*) AS n, MIN(v.price) AS lo, MAX(v.price) AS hi").
		Joins("JOIN products p ON p.id = v.product_id AND COALESCE(p.options_json, '') <> ''").
		Where("v.storefront_id = ?", siteID).Group("v.product_id").Scan(&rows)
	out := make(map[uint]variantStat, len(rows))
	for _, r := range rows {
		out[r.ProductID] = variantStat{N: r.N, Min: r.Lo, Max: r.Hi}
	}
	return out
}

// publicVariants: phần biến thể được công khai (không lộ giá nhập, số tồn kho cụ thể, mã vạch)
func publicVariants(vs []models.ProductVariant, nOpts int) []gin.H {
	out := make([]gin.H, 0, len(vs))
	sort.SliceStable(vs, func(i, j int) bool { return vs[i].Position < vs[j].Position })
	for _, v := range vs {
		out = append(out, gin.H{
			"id": v.ID, "title": v.Title, "option_values": v.OptionValues(nOpts), "price": v.Price,
			"compare_price": v.ComparePrice, "in_stock": v.Stock != 0, "image_url": v.ImageURL, "sku": v.SKU,
		})
	}
	return out
}
