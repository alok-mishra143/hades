package store

import (
	"reflect"
	"testing"
	"time"
)

// newTestStore returns a store backed by a fixed clock.
// Call advance to move that clock forward.
func newTestStore(maxSize ...int) (*Store, func(time.Duration)) {
	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	s := NewStore(maxSize...)
	s.now = func() time.Time { return now }
	return s, func(d time.Duration) { now = now.Add(d) }
}

func TestGetSet_RoundTrip(t *testing.T) {
	s, _ := newTestStore()

	s.Set("hello", "world")

	// INFO checking if we are getting same value or not from storage
	if val, ok := s.Get("hello"); !ok || val != "world" {
		t.Error("Failed to get the real value")
	}

	// INFO a missing key must report absence and return the empty value
	if val, ok := s.Get("missing"); ok || val != "" {
		t.Errorf("Get(missing) = (%q, %v), want (\"\", false)", val, ok)
	}

}

func TestStore_Delete(t *testing.T) {
	tests := []struct {
		name       string
		setup      map[string]string
		deleteKeys []string
		wantKeys   []string
	}{
		{
			name:       "deletes an existing key",
			setup:      map[string]string{"a": "1", "b": "2"},
			deleteKeys: []string{"a"},
			wantKeys:   []string{"b"},
		},
		{
			name:       "deletes the only key",
			setup:      map[string]string{"a": "1"},
			deleteKeys: []string{"a"},
			wantKeys:   []string{},
		},
		{
			name:       "deleting a missing key is a no-op",
			setup:      map[string]string{"a": "1", "b": "2"},
			deleteKeys: []string{"zzz"},
			wantKeys:   []string{"a", "b"},
		},
		{
			name:       "deleting from an empty store is a no-op",
			setup:      nil,
			deleteKeys: []string{"a"},
			wantKeys:   []string{},
		},
		{
			name:       "deleting the same key twice is a no-op the second time",
			setup:      map[string]string{"a": "1", "b": "2"},
			deleteKeys: []string{"a", "a"},
			wantKeys:   []string{"b"},
		},
		{
			name:       "deletes every key",
			setup:      map[string]string{"a": "1", "b": "2", "c": "3"},
			deleteKeys: []string{"a", "b", "c"},
			wantKeys:   []string{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			s, _ := newTestStore()
			for k, v := range tc.setup {
				s.Set(k, v)
			}

			for _, k := range tc.deleteKeys {
				s.Delete(k)
			}

			// The deleted keys must be gone.
			for _, k := range tc.deleteKeys {
				if _, ok := s.Get(k); ok {
					t.Errorf("Get(%q) still found the key after Delete", k)
				}
			}

			// The store must contain exactly the expected keys.
			if got := s.keys(); !reflect.DeepEqual(got, tc.wantKeys) {
				t.Errorf("keys() after Delete = %q, want %q", got, tc.wantKeys)
			}

			// Deleting must not corrupt the values left behind.
			for _, k := range tc.wantKeys {
				want := tc.setup[k]
				if got, ok := s.Get(k); !ok || got != want {
					t.Errorf("Get(%q) = (%q, %v), want (%q, true)", k, got, ok, want)
				}
			}
		})
	}
}

func TestKeys_ReturnAllKeysSorted(t *testing.T) {
	s, _ := newTestStore()
	s.Set("alok", "1")
	s.Set("vinit", "2")
	s.Set("lalit", "3")

	got := s.keys()
	want := []string{"alok", "lalit", "vinit"}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("keys() = %q, want %q", got, want)
	}

}
