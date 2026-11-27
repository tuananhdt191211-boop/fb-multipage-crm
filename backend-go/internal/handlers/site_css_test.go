package handlers

import "testing"

func TestNormalizeStyleKeepsCleanCSS(t *testing.T) {
	data := map[string]interface{}{"style": map[string]interface{}{
		"bg":  "#ffffff",
		"css": "h2 { color: red; }</style><script>x</script>@import url(evil.css); div{width:expression(1)}",
	}}
	normalizeStyle(data)
	st := data["style"].(map[string]interface{})
	css, _ := st["css"].(string)
	if css == "" || st["bg"] != "#ffffff" {
		t.Fatalf("css/bg dropped: %#v", st)
	}
	for _, bad := range []string{"</style", "<style", "@import", "expression("} {
		if contains(css, bad) {
			t.Fatalf("css still contains %q: %s", bad, css)
		}
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
