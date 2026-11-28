package handlers

import "testing"

func TestCleanAIBaseURL(t *testing.T) {
	want := "https://generativelanguage.googleapis.com/v1beta/openai"
	cases := []string{
		"[https://generativelanguage.googleapis.com/v1beta/openai](https://generativelanguage.googleapis.com/v1beta/openai)",
		"  https://generativelanguage.googleapis.com/v1beta/openai/  ",
		"https://generativelanguage.googleapis.com/v1beta/openai/chat/completions",
		"<https://generativelanguage.googleapis.com/v1beta/openai>",
		want,
	}
	for _, in := range cases {
		if got := cleanAIBaseURL(in); got != want {
			t.Errorf("cleanAIBaseURL(%q) = %q, muốn %q", in, got, want)
		}
	}
	if got := cleanAIBaseURL("https://openrouter.ai/api/v1"); got != "https://openrouter.ai/api/v1" {
		t.Errorf("OpenRouter bị đổi sai: %q", got)
	}
	if got := cleanAIBaseURL(""); got != "" {
		t.Errorf("chuỗi rỗng phải giữ rỗng: %q", got)
	}
}
