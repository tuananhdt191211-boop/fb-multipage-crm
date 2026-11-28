package handlers

// Bộ lọc HTML cho mô tả sản phẩm (soạn bằng trình soạn thảo định dạng, hiển thị công khai bằng v-html).
//
// Cách làm: phân tích HTML thành cây DOM đúng chuẩn (giống trình duyệt) rồi tự dựng lại chuỗi chỉ từ các thẻ và
// thuộc tính nằm trong danh sách cho phép. Mọi thứ khác (script, style, iframe, form, on*=, style=, class=...) bị bỏ;
// thẻ lạ thì bỏ thẻ nhưng giữ chữ bên trong. Đầu ra luôn cân thẻ và escape đầy đủ nên không thể nhúng mã chạy được.

import (
	"net/url"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

const maxRichHTMLBytes = 400 << 10 // 400KB đầu vào

// thẻ được giữ nguyên (đã chuẩn hoá về h2-h4)
var richAllowedTags = map[string]bool{
	"p": true, "br": true, "hr": true, "strong": true, "b": true, "em": true, "i": true, "u": true, "s": true, "strike": true,
	"h2": true, "h3": true, "h4": true, "ul": true, "ol": true, "li": true, "blockquote": true, "pre": true, "code": true,
	"a": true, "img": true, "table": true, "thead": true, "tbody": true, "tr": true, "th": true, "td": true,
	"sub": true, "sup": true, "figure": true, "figcaption": true,
}

// thẻ bỏ luôn cả nội dung bên trong
var richDropWithContent = map[string]bool{
	"script": true, "style": true, "noscript": true, "iframe": true, "object": true, "embed": true, "svg": true, "math": true,
	"template": true, "textarea": true, "select": true, "option": true, "form": true, "input": true, "button": true,
	"link": true, "meta": true, "head": true, "title": true, "base": true, "frame": true, "frameset": true, "canvas": true,
	"audio": true, "video": true, "source": true, "applet": true,
}

var richVoidTags = map[string]bool{"br": true, "hr": true, "img": true}

// h1 -> h2, h5/h6 -> h4 (tiêu đề trang đã là tên sản phẩm)
func richTagAlias(tag string) string {
	switch tag {
	case "h1":
		return "h2"
	case "h5", "h6":
		return "h4"
	}
	return tag
}

func richSafeLink(raw string) string {
	v := strings.TrimSpace(raw)
	if v == "" || len(v) > 2000 {
		return ""
	}
	if strings.HasPrefix(v, "#") {
		return v
	}
	if strings.HasPrefix(v, "/") && !strings.HasPrefix(v, "//") {
		return v
	}
	u, err := url.Parse(v)
	if err != nil {
		return ""
	}
	switch strings.ToLower(u.Scheme) {
	case "http", "https", "mailto", "tel":
		return v
	}
	return ""
}

func attrOf(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if strings.EqualFold(a.Key, key) {
			return a.Val
		}
	}
	return ""
}

func richWriteChildren(sb *strings.Builder, n *html.Node) {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		richWriteNode(sb, c)
	}
}

func richWriteNode(sb *strings.Builder, n *html.Node) {
	switch n.Type {
	case html.TextNode:
		sb.WriteString(html.EscapeString(n.Data))
	case html.ElementNode:
		tag := strings.ToLower(n.Data)
		if richDropWithContent[tag] {
			return
		}
		tag = richTagAlias(tag)
		if !richAllowedTags[tag] {
			richWriteChildren(sb, n) // thẻ lạ (div, span, font...): bỏ thẻ, giữ chữ
			return
		}
		switch tag {
		case "img":
			src := safeMediaURL(attrOf(n, "src"))
			if src == "" {
				return
			}
			sb.WriteString(`<img src="`)
			sb.WriteString(html.EscapeString(src))
			sb.WriteString(`" alt="`)
			sb.WriteString(html.EscapeString(truncRunes(attrOf(n, "alt"), 200)))
			sb.WriteString(`" loading="lazy" referrerpolicy="no-referrer">`)
			return
		case "a":
			href := richSafeLink(attrOf(n, "href"))
			if href == "" {
				richWriteChildren(sb, n)
				return
			}
			sb.WriteString(`<a href="`)
			sb.WriteString(html.EscapeString(href))
			if !strings.HasPrefix(href, "#") {
				sb.WriteString(`" target="_blank" rel="noopener noreferrer nofollow`)
			}
			sb.WriteString(`">`)
			richWriteChildren(sb, n)
			sb.WriteString("</a>")
			return
		case "td", "th":
			sb.WriteString("<" + tag)
			for _, k := range []string{"colspan", "rowspan"} {
				if v := attrOf(n, k); v != "" && len(v) <= 3 && isDigits(v) {
					sb.WriteString(" " + k + `="` + v + `"`)
				}
			}
			sb.WriteString(">")
			richWriteChildren(sb, n)
			sb.WriteString("</" + tag + ">")
			return
		}
		if richVoidTags[tag] {
			sb.WriteString("<" + tag + ">")
			return
		}
		sb.WriteString("<" + tag + ">")
		richWriteChildren(sb, n)
		sb.WriteString("</" + tag + ">")
	case html.DocumentNode:
		richWriteChildren(sb, n)
	}
}

func isDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return s != ""
}

func truncRunes(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n])
	}
	return s
}

// sanitizeRichHTML trả về HTML an toàn (rỗng nếu đầu vào không có gì đáng giữ).
func sanitizeRichHTML(in string) string {
	in = strings.TrimSpace(in)
	if in == "" {
		return ""
	}
	if len(in) > maxRichHTMLBytes {
		in = in[:maxRichHTMLBytes]
	}
	ctx := &html.Node{Type: html.ElementNode, Data: "div", DataAtom: atom.Div}
	nodes, err := html.ParseFragment(strings.NewReader(in), ctx)
	if err != nil {
		return html.EscapeString(in)
	}
	var sb strings.Builder
	for _, n := range nodes {
		richWriteNode(&sb, n)
	}
	out := strings.TrimSpace(sb.String())
	// Chỉ còn thẻ rỗng / khoảng trắng → coi như không có mô tả
	if strings.TrimSpace(htmlToText(out)) == "" && !strings.Contains(out, "<img") && !strings.Contains(out, "<hr") {
		return ""
	}
	return out
}

// htmlToText bỏ thẻ, giữ chữ (dùng đếm độ dài / tóm tắt)
func htmlToText(s string) string {
	nodes, err := html.ParseFragment(strings.NewReader(s), &html.Node{Type: html.ElementNode, Data: "div", DataAtom: atom.Div})
	if err != nil {
		return s
	}
	var sb strings.Builder
	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			sb.WriteString(n.Data)
		}
		if n.Type == html.ElementNode {
			switch strings.ToLower(n.Data) {
			case "script", "style":
				return
			case "p", "br", "li", "h2", "h3", "h4", "tr", "blockquote", "div":
				sb.WriteString("\n")
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	for _, n := range nodes {
		walk(n)
	}
	return sb.String()
}

// plainToRichHTML đổi mô tả thuần văn bản cũ (có xuống dòng) sang HTML đoạn văn
func plainToRichHTML(s string) string {
	s = strings.TrimSpace(strings.ReplaceAll(s, "\r\n", "\n"))
	if s == "" {
		return ""
	}
	var sb strings.Builder
	for _, para := range strings.Split(s, "\n\n") {
		para = strings.TrimSpace(para)
		if para == "" {
			continue
		}
		sb.WriteString("<p>")
		sb.WriteString(strings.ReplaceAll(html.EscapeString(para), "\n", "<br>"))
		sb.WriteString("</p>")
	}
	return sb.String()
}

// looksLikeHTML: mô tả đã là HTML hay còn là văn bản thuần của bản cũ
func looksLikeHTML(s string) bool {
	return strings.Contains(s, "</") || strings.Contains(s, "<p>") || strings.Contains(s, "<br") || strings.Contains(s, "<img")
}
