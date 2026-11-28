package handlers

import "testing"

func TestNormalizeAIBaseURL(t *testing.T) {
	const gemini = "https://generativelanguage.googleapis.com/v1beta/openai"
	cases := map[string]string{
		"":                                      "",
		"   ":                                   "",
		gemini:                                  gemini,
		gemini + "/":                            gemini,
		"  " + gemini + "  ":                    gemini,
		"[" + gemini + "](" + gemini + ")":      gemini, // lỗi thực tế: dán nguyên liên kết markdown
		"[" + gemini + "](" + gemini + ")/":     gemini,
		"[api](https://api.groq.com/openai/v1)": "https://api.groq.com/openai/v1",
		"<https://api.openai.com/v1>":           "https://api.openai.com/v1",
		"\"https://api.openai.com/v1\"":         "https://api.openai.com/v1",
		"http://localhost:11434/v1":             "http://localhost:11434/v1",
	}
	for in, want := range cases {
		if got := normalizeAIBaseURL(in); got != want {
			t.Errorf("normalizeAIBaseURL(%q) = %q, want %q", in, got, want)
		}
	}
}
