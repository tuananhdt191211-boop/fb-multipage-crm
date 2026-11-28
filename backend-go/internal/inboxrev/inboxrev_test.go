package inboxrev

import "testing"

func TestBumpIncreasesRevision(t *testing.T) {
	a := Current()
	Bump()
	Bump()
	if Current() != a+2 {
		t.Fatalf("rev %d -> %d, want +2", a, Current())
	}
}

func TestWatchedTables(t *testing.T) {
	for _, tb := range []string{"conversations", "messages"} {
		if !watched(tb) {
			t.Errorf("%s should be watched", tb)
		}
	}
	for _, tb := range []string{"users", "facebook_pages", "broadcast_messages", ""} {
		if watched(tb) {
			t.Errorf("%q should not be watched", tb)
		}
	}
}
