package database

import (
	"errors"
	"strings"
	"testing"
)

func TestExplainQuotaError(t *testing.T) {
	err := errors.New("failed to connect to `user=neondb_owner database=neondb`:\n\t[2406:da18::1]:5432 (ep-x-pooler.neon.tech): server error: ERROR: Your account or project has exceeded the quota. Upgrade your plan to increase limits. (SQLSTATE 53000)")
	if !IsQuotaError(err) {
		t.Fatal("quota error not detected")
	}
	msg := ExplainError(err)
	for _, want := range []string{"Neon", "53000", "console.neon.tech", "docker compose up -d postgres", ".env.local"} {
		if !strings.Contains(msg, want) {
			t.Errorf("explanation missing %q:\n%s", want, msg)
		}
	}
	// chỉ giữ dòng đầu của lỗi nhiều dòng
	if strings.Contains(msg, "2406:da18") {
		t.Errorf("raw multi-line noise leaked:\n%s", msg)
	}
}

func TestExplainSupabaseErrors(t *testing.T) {
	m := ExplainError(errors.New("failed to connect to `user=postgres database=postgres`: server error: FATAL: Tenant or user not found (SQLSTATE XX000)"))
	for _, want := range []string{"postgres.<mã-dự-án>", "Session pooler", "TẠM DỪNG", "Restore"} {
		if !strings.Contains(m, want) {
			t.Errorf("Supabase tenant hint missing %q:\n%s", want, m)
		}
	}
	m = ExplainError(errors.New("dial tcp [2406:da18:1fd8::1]:5432: connect: network is unreachable"))
	for _, want := range []string{"IPv6", "Session pooler", "pooler.supabase.com"} {
		if !strings.Contains(m, want) {
			t.Errorf("IPv6 hint missing %q:\n%s", want, m)
		}
	}
	// lỗi hết hạn mức Neon vẫn được nhận ra trước các nhánh khác
	if m := ExplainError(errors.New("ERROR: Your account or project has exceeded the quota (SQLSTATE 53000)")); !strings.Contains(m, "Neon") {
		t.Errorf("quota error lost priority:\n%s", m)
	}
}

func TestExplainOtherErrors(t *testing.T) {
	if IsQuotaError(errors.New("connection refused")) {
		t.Fatal("refused is not a quota error")
	}
	if m := ExplainError(errors.New("dial tcp 127.0.0.1:5432: connectex: No connection could be made because the target machine actively refused it.")); !strings.Contains(m, "docker compose") {
		t.Errorf("refused hint missing: %s", m)
	}
	if m := ExplainError(errors.New("password authentication failed (SQLSTATE 28P01)")); !strings.Contains(m, "mật khẩu") {
		t.Errorf("auth hint missing: %s", m)
	}
	if ExplainError(nil) != "" {
		t.Error("nil error should give empty text")
	}
}
