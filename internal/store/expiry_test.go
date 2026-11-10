package store

import (
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestStore_GetReclaimsOnlyTouchedKey(t *testing.T) {
	s, advance := newTestStore()
	if err := s.SetWithTTL("touched", "1", time.Second); err != nil {
		t.Fatalf("SetWithTTL: %v", err)
	}
	if err := s.SetWithTTL("untouched", "2", time.Second); err != nil {
		t.Fatalf("SetWithTTL: %v", err)
	}

	advance(2 * time.Second)

	if val, ok := s.Get("touched"); ok || val != "" {
		t.Errorf("Get(expired) = (%q, %v), want (\"\", false)", val, ok)
	}
	if _, ok := s.data["touched"]; ok {
		t.Error("Get left the expired entry it touched in the map")
	}
	if _, ok := s.data["untouched"]; !ok {
		t.Error("Get reclaimed a key it did not touch; that is eager purge, not delete-on-read")
	}
}

func TestStore_GetHidesExpiredKey(t *testing.T) {
	s, advance := newTestStore()
	if err := s.SetWithTTL("k", "v", time.Second); err != nil {
		t.Fatalf("SetWithTTL: %v", err)
	}

	advance(2 * time.Second)

	if val, ok := s.Get("k"); ok || val != "" {
		t.Errorf("Get(expired) = (%q, %v), want (\"\", false)", val, ok)
	}
	if _, ok := s.data["k"]; ok {
		t.Error("Get left the expired entry in the map; it should reclaim it")
	}
}

func TestStore_KeysAndLenHideExpired(t *testing.T) {
	s, advance := newTestStore()
	s.Set("persistent", "1")
	if err := s.SetWithTTL("ephemeral", "2", time.Second); err != nil {
		t.Fatalf("SetWithTTL: %v", err)
	}

	advance(2 * time.Second)

	if got, want := s.keys(), []string{"persistent"}; !reflect.DeepEqual(got, want) {
		t.Errorf("keys() = %q, want %q", got, want)
	}
	if got := s.Len(); got != 1 {
		t.Errorf("Len() = %d, want 1", got)
	}
	if _, ok := s.data["ephemeral"]; !ok {
		t.Error("keys/Len deleted the expired entry; they are read-only filters")
	}
}

func TestStore_SweepReclaimsExpired(t *testing.T) {
	s, advance := newTestStore()
	if err := s.SetWithTTL("stale", "1", time.Second); err != nil {
		t.Fatalf("SetWithTTL: %v", err)
	}
	s.Set("live", "2")

	advance(2 * time.Second)

	if got := s.Sweep(); got != 1 {
		t.Errorf("Sweep() = %d, want 1", got)
	}
	if _, ok := s.data["stale"]; ok {
		t.Error("Sweep left the expired entry")
	}
	if _, ok := s.data["live"]; !ok {
		t.Error("Sweep removed a live entry")
	}
	if got := s.Len(); got != 1 {
		t.Errorf("Len() after Sweep = %d, want 1", got)
	}
}

func TestStore_SetClearsTTL(t *testing.T) {
	s, advance := newTestStore()
	if err := s.SetWithTTL("k", "old", time.Second); err != nil {
		t.Fatalf("SetWithTTL: %v", err)
	}

	s.Set("k", "new")

	advance(2 * time.Second)

	if val, ok := s.Get("k"); !ok || val != "new" {
		t.Errorf("Get(k) = (%q, %v), want (\"new\", true)", val, ok)
	}
}

func TestStore_PopExpiredKey(t *testing.T) {
	s, advance := newTestStore()
	if err := s.SetWithTTL("k", "v", time.Second); err != nil {
		t.Fatalf("SetWithTTL: %v", err)
	}

	advance(2 * time.Second)

	if val, ok := s.Pop("k"); ok || val != "" {
		t.Errorf("Pop(expired) = (%q, %v), want (\"\", false)", val, ok)
	}
	if got := s.Len(); got != 0 {
		t.Errorf("Len() = %d, want 0", got)
	}
}

func TestStore_SetWithTTLRejectsNonPositive(t *testing.T) {
	for _, ttl := range []time.Duration{0, -time.Second} {
		s, _ := newTestStore()

		err := s.SetWithTTL("k", "v", ttl)
		if !errors.Is(err, ErrTimeCorrect) {
			t.Errorf("SetWithTTL(ttl=%v) = %v, want %v", ttl, err, ErrTimeCorrect)
		}
		if _, ok := s.Get("k"); ok {
			t.Errorf("SetWithTTL(ttl=%v) wrote the key, want no write", ttl)
		}
	}
}
