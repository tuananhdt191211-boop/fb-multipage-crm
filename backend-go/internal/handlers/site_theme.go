package handlers

// Kiểu chữ toàn trang của website (giống mục Typography của Shopify):
// font tiêu đề / nội dung và cỡ, độ đậm... cho từng thẻ H1–H4, P.
// Chỉ giữ các khoá & giá trị hợp lệ; frontend sinh CSS từ dữ liệu này.

import (
	"encoding/json"
	"math"
)

// Phải khớp danh sách font trong web-vue/src/lib/fonts.ts
var allowedThemeFonts = map[string]bool{
	"Be Vietnam Pro": true, "Inter": true, "Roboto": true, "Open Sans": true, "Montserrat": true,
	"Nunito": true, "Quicksand": true, "Lexend": true, "Mulish": true, "Raleway": true,
	"Josefin Sans": true, "Barlow": true, "Oswald": true, "Noto Sans": true,
	"Playfair Display": true, "Lora": true, "Merriweather": true, "Noto Serif": true, "Cormorant Garamond": true,
	"Dancing Script": true, "Lobster": true, "Pacifico": true,
	"Arial": true, "Georgia": true, "Times New Roman": true,
}

var themeTags = []string{"h1", "h2", "h3", "h4", "p"}

var allowedWeights = map[string]bool{"300": true, "400": true, "500": true, "600": true, "700": true, "800": true, "900": true}

func clampF(v interface{}, min, max float64) (float64, bool) {
	f, ok := v.(float64)
	if !ok || math.IsNaN(f) || f <= 0 {
		return 0, false
	}
	return math.Min(max, math.Max(min, f)), true
}

// sanitizeThemeJSON trả về JSON đã làm sạch ("" nếu trống / không hợp lệ)
func sanitizeThemeJSON(raw string) string {
	if raw == "" || len(raw) > 20000 {
		return ""
	}
	var in map[string]interface{}
	if json.Unmarshal([]byte(raw), &in) != nil {
		return ""
	}
	out := map[string]interface{}{}
	for _, k := range []string{"heading_font", "body_font"} {
		if s, ok := in[k].(string); ok && allowedThemeFonts[s] {
			out[k] = s
		}
	}
	if tags, ok := in["tags"].(map[string]interface{}); ok {
		cleanTags := map[string]interface{}{}
		for _, tag := range themeTags {
			t, ok := tags[tag].(map[string]interface{})
			if !ok {
				continue
			}
			c := map[string]interface{}{}
			if s, ok := t["font"].(string); ok && allowedThemeFonts[s] {
				c["font"] = s
			}
			if f, ok := clampF(t["size"], 8, 160); ok {
				c["size"] = f
			}
			if f, ok := clampF(t["size_m"], 8, 120); ok {
				c["size_m"] = f
			}
			if s, ok := t["weight"].(string); ok && allowedWeights[s] {
				c["weight"] = s
			}
			if f, ok := clampF(t["lh"], 0.8, 3); ok {
				c["lh"] = f
			}
			if f, ok := t["ls"].(float64); ok && !math.IsNaN(f) {
				c["ls"] = math.Min(20, math.Max(-5, f))
			}
			if b, ok := t["upper"].(bool); ok && b {
				c["upper"] = true
			}
			if s, ok := t["color"].(string); ok && colorRe.MatchString(s) {
				c["color"] = s
			}
			if len(c) > 0 {
				cleanTags[tag] = c
			}
		}
		if len(cleanTags) > 0 {
			out["tags"] = cleanTags
		}
	}
	if len(out) == 0 {
		return ""
	}
	b, _ := json.Marshal(out)
	return string(b)
}
