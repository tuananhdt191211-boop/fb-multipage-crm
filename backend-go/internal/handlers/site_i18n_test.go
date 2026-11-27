package handlers

import (
	"encoding/json"
	"testing"
)

func TestSanitizeI18nJSON(t *testing.T) {
	in := `{"markets":["us","de","ca-fr","BAD!","us"],"default":"vn",
	 "tr":{"de":{"b":{"blk1":{"heading":"Hallo","x":5}},"p":{"3":{"name":"Creme"}},"ui":{"cart":"Warenkorb"},"evil":{"a":"b"}},
	       "xx":{"ui":{"a":"b"}},"vi":{"ui":{"a":"b"}}}}`
	out := sanitizeI18nJSON(in)
	var m struct {
		Markets []string                             `json:"markets"`
		Default string                               `json:"default"`
		Tr      map[string]map[string]map[string]any `json:"tr"`
	}
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatal(err, out)
	}
	if len(m.Markets) != 3 || m.Default != "vn" {
		t.Fatalf("markets %v default %q", m.Markets, m.Default)
	}
	if _, ok := m.Tr["xx"]; ok {
		t.Fatal("unknown language kept")
	}
	if _, ok := m.Tr["vi"]; ok {
		t.Fatal("source language kept")
	}
	de := m.Tr["de"]
	if de["evil"] != nil || de["ui"]["cart"] != "Warenkorb" {
		t.Fatalf("de wrong: %v", de)
	}
	blk := de["b"]["blk1"].(map[string]any)
	if blk["heading"] != "Hallo" || blk["x"] != nil {
		t.Fatalf("block tr wrong: %v", blk)
	}
}
