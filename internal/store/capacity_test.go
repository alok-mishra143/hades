package store

import (
	"testing"
	"time"
)

func TestStore_ExpiredKeyFreesCapacity(t *testing.T) {
	s, advance := newTestStore(1)
	if err := s.SetWithTTL("a", "1", time.Second); err != nil {
		t.Fatalf("SetWithTTL: %v", err)
	}

	advance(2 * time.Second)

	if err := s.Set("b", "2"); err != nil {
		t.Fatalf("Set after expiry = %v, want nil", err)
	}
	if val, ok := s.Get("b"); !ok || val != "2" {
		t.Errorf("Get(b) = (%q, %v), want (\"2\", true)", val, ok)
	}
	if got := s.Len(); got != 1 {
		t.Errorf("Len() = %d, want 1", got)
	}
}
