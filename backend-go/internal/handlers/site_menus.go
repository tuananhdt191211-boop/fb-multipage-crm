package handlers

// Menu điều hướng quản lý riêng (giai đoạn 2): website có nhiều menu (vd. "Menu chính", "Menu chân trang"); mỗi menu là danh sách mục
// {label, link, children[]} (tối đa 2 cấp). Đầu trang chọn menu bằng data.menu_id, cột chân trang bằng col.menu_id; không chọn thì dùng
// danh sách tự nhập như cũ (nav_items / col.links). Liên kết tới trang khác viết dạng page:<đường-dẫn> (xem site_pages.go).

import (
	"encoding/json"
	"regexp"
	"strings"
	"unicode/utf8"
)

const (
	maxMenus         = 10
	maxMenuItems     = 40
	maxMenuChildren  = 12
	maxMenuLabelRune = 60
)

var menuIDRe = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,39}$`)

type menuItem struct {
	Label    string     `json:"label"`
	Link     string     `json:"link"`
	Children []menuItem `json:"children,omitempty"`
}

type siteMenu struct {
	ID    string     `json:"id"`
	Name  string     `json:"name"`
	Items []menuItem `json:"items"`
}

func cutRunes(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n])
}

func cleanMenuItems(in []menuItem, depth int) []menuItem {
	limit := maxMenuItems
	if depth > 0 {
		limit = maxMenuChildren
	}
	out := make([]menuItem, 0, len(in))
	for _, it := range in {
		if len(out) >= limit {
			break
		}
		label := cutRunes(strings.Join(strings.Fields(it.Label), " "), maxMenuLabelRune)
		if label == "" {
			continue
		}
		clean := menuItem{Label: label, Link: safeLink(it.Link)}
		if depth == 0 && len(it.Children) > 0 {
			clean.Children = cleanMenuItems(it.Children, 1)
		}
		out = append(out, clean)
	}
	return out
}

// sanitizeMenusJSON: mảng menu hợp lệ (id duy nhất, nhãn gọn, link an toàn, tối đa 2 cấp). Rỗng / lỗi → "[]" hoặc lỗi cấu trúc.
func sanitizeMenusJSON(raw string) (string, error) {
	if strings.TrimSpace(raw) == "" {
		return "[]", nil
	}
	if len(raw) > 128*1024 {
		return "", errMenusTooLarge
	}
	var in []siteMenu
	if err := json.Unmarshal([]byte(raw), &in); err != nil {
		return "", errMenusBad
	}
	seen := map[string]bool{}
	out := make([]siteMenu, 0, len(in))
	for _, m := range in {
		if len(out) >= maxMenus {
			break
		}
		id := strings.ToLower(strings.TrimSpace(m.ID))
		if !menuIDRe.MatchString(id) || seen[id] {
			id = newBlockID("menu")
		}
		seen[id] = true
		name := cutRunes(strings.Join(strings.Fields(m.Name), " "), maxMenuLabelRune)
		if name == "" {
			name = "Menu"
		}
		out = append(out, siteMenu{ID: id, Name: name, Items: cleanMenuItems(m.Items, 0)})
	}
	b, err := json.Marshal(out)
	return string(b), err
}

type menuErr string

func (e menuErr) Error() string { return string(e) }

const (
	errMenusBad      = menuErr("Cấu trúc menu không hợp lệ.")
	errMenusTooLarge = menuErr("Menu quá lớn.")
)
