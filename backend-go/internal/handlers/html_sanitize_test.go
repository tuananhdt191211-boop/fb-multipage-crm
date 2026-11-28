package handlers

import (
	"strings"
	"testing"
)

func TestSanitizeRichHTML(t *testing.T) {
	cases := []struct {
		name, in string
		want     []string // phải có trong kết quả
		deny     []string // không được có
	}{
		{"giữ định dạng cơ bản", `<p>Xin <strong>chào</strong> <em>bạn</em></p><ul><li>A</li><li>B</li></ul>`,
			[]string{"<p>Xin <strong>chào</strong> <em>bạn</em></p>", "<ul><li>A</li><li>B</li></ul>"}, nil},
		{"bỏ script và nội dung của nó", `<p>ok</p><script>alert(1)</script>`, []string{"<p>ok</p>"}, []string{"script", "alert"}},
		{"bỏ thuộc tính sự kiện và style", `<p onclick="x()" style="color:red" class="a" id="b">hi</p>`, []string{"<p>hi</p>"}, []string{"onclick", "style", "class", "id="}},
		{"img: chặn javascript: và data:", `<img src="javascript:alert(1)"><img src="data:text/html;base64,AAAA"><img src="https://cdn.x/a.jpg" onerror="x()">`,
			[]string{`<img src="https://cdn.x/a.jpg"`}, []string{"javascript", "data:", "onerror"}},
		{"a: chỉ http(s)/mailto/tel, thêm rel", `<a href="javascript:alert(1)">bad</a><a href="https://shop.vn" onclick="x()">good</a><a href="mailto:a@b.c">m</a>`,
			[]string{`<a href="https://shop.vn" target="_blank" rel="noopener noreferrer nofollow">good</a>`, `<a href="mailto:a@b.c"`, "bad"}, []string{"javascript", "onclick"}},
		{"iframe/object/form/svg bị bỏ cả nội dung", `<iframe src="https://evil"></iframe><object data="x"></object><form><input value="a"></form><svg onload="x()"><text>t</text></svg><p>ok</p>`,
			[]string{"<p>ok</p>"}, []string{"iframe", "object", "form", "input", "svg", "evil"}},
		{"thẻ lạ: bỏ thẻ giữ chữ", `<div><span style="x">giữ <font color="red">chữ</font></span></div>`, []string{"giữ chữ"}, []string{"<div", "<span", "<font"}},
		{"h1 → h2, h5 → h4", `<h1>T</h1><h5>S</h5>`, []string{"<h2>T</h2>", "<h4>S</h4>"}, []string{"<h1", "<h5"}},
		{"escape ký tự đặc biệt trong chữ", `<p>1 < 2 && "a" > 'b'</p>`, []string{"&lt;", "&amp;&amp;"}, []string{"< 2"}},
		{"thẻ chưa đóng vẫn cân", `<p><strong>đậm <em>nghiêng`, []string{"<p><strong>đậm <em>nghiêng</em></strong></p>"}, nil},
		{"mXSS: thẻ lồng nhau kiểu noscript/style", `<noscript><p title="</noscript><img src=x onerror=alert(1)>"></noscript>`, nil, []string{"onerror", "alert"}},
		{"comment bị bỏ", `<p>a<!-- <script>x</script> -->b</p>`, []string{"<p>ab</p>"}, []string{"script", "--"}},
		{"bảng giữ colspan hợp lệ", `<table><tr><td colspan="2" style="x">a</td><td colspan="javascript:1">b</td></tr></table>`,
			[]string{`<td colspan="2">a</td>`, "<td>b</td>"}, []string{"style", "javascript"}},
		{"rel/target tự gài không tin đầu vào", `<a href="https://a.b" target="_self" rel="opener">x</a>`, []string{`target="_blank" rel="noopener noreferrer nofollow"`}, []string{`target="_self"`, `rel="opener"`}},
	}
	for _, c := range cases {
		got := sanitizeRichHTML(c.in)
		for _, w := range c.want {
			if !strings.Contains(got, w) {
				t.Errorf("[%s] thiếu %q trong %q", c.name, w, got)
			}
		}
		for _, d := range c.deny {
			if strings.Contains(strings.ToLower(got), strings.ToLower(d)) {
				t.Errorf("[%s] còn %q trong %q", c.name, d, got)
			}
		}
	}
}

func TestSanitizeRichHTMLEmptyAndIdempotent(t *testing.T) {
	for _, in := range []string{"", "   ", "<p></p>", "<div> </div>", "<script>x</script>"} {
		if got := sanitizeRichHTML(in); got != "" {
			t.Errorf("%q phải rỗng, được %q", in, got)
		}
	}
	// chạy lại trên kết quả đã lọc thì không đổi
	in := `<h2>Tiêu đề</h2><p>Mô tả <a href="https://a.b/c?x=1&y=2">link</a></p><img src="https://cdn.x/a.jpg" alt="ảnh"><table><tr><td>1</td></tr></table>`
	once := sanitizeRichHTML(in)
	twice := sanitizeRichHTML(once)
	if once != twice {
		t.Errorf("không idempotent:\n1: %s\n2: %s", once, twice)
	}
	// ảnh một mình vẫn là nội dung hợp lệ
	if got := sanitizeRichHTML(`<img src="https://cdn.x/a.jpg">`); !strings.Contains(got, "<img") {
		t.Errorf("ảnh đơn bị mất: %q", got)
	}
}

func TestPlainToRichHTMLAndLooksLikeHTML(t *testing.T) {
	got := plainToRichHTML("Dòng 1\nDòng 2\n\nĐoạn <2> & \"x\"")
	if got != "<p>Dòng 1<br>Dòng 2</p><p>Đoạn &lt;2&gt; &amp; &#34;x&#34;</p>" {
		t.Errorf("plainToRichHTML = %q", got)
	}
	if looksLikeHTML("chỉ là chữ thường\nxuống dòng") || !looksLikeHTML("<p>a</p>") || !looksLikeHTML("a<br>b") {
		t.Error("looksLikeHTML sai")
	}
	if txt := strings.TrimSpace(htmlToText("<p>A</p><ul><li>B</li></ul><script>x</script>")); !strings.Contains(txt, "A") || !strings.Contains(txt, "B") || strings.Contains(txt, "x") {
		t.Errorf("htmlToText = %q", txt)
	}
}
