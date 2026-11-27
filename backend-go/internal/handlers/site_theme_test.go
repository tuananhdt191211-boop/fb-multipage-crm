package handlers

import (
	"encoding/json"
	"testing"
)

func TestSanitizeThemeJSON(t *testing.T) {
	in := `{"heading_font":"Playfair Display","body_font":"Comic Sans<script>","evil":1,
	 "tags":{"h1":{"size":500,"size_m":30,"weight":"800","color":"#112233","upper":true,"font":"Lora","x":"y"},
	         "p":{"lh":1.6,"color":"red;}body{","weight":"bold"},"h9":{"size":10}}}`
	out := sanitizeThemeJSON(in)
	var m map[string]any
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatal(err, out)
	}
	if m["heading_font"] != "Playfair Display" || m["body_font"] != nil || m["evil"] != nil {
		t.Fatalf("fonts wrong: %s", out)
	}
	tags := m["tags"].(map[string]any)
	h1 := tags["h1"].(map[string]any)
	if h1["size"] != 160.0 || h1["weight"] != "800" || h1["color"] != "#112233" || h1["x"] != nil || h1["font"] != "Lora" {
		t.Fatalf("h1 wrong: %v", h1)
	}
	p := tags["p"].(map[string]any)
	if p["color"] != nil || p["weight"] != nil || p["lh"] != 1.6 {
		t.Fatalf("p wrong: %v", p)
	}
	if tags["h9"] != nil {
		t.Fatal("unknown tag kept")
	}
	if sanitizeThemeJSON("not json") != "" || sanitizeThemeJSON("{}") != "" {
		t.Fatal("invalid input should give empty")
	}
}
