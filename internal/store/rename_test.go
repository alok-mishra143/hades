package store

import (
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestStore_Rename(t *testing.T) {
	tests := []struct {
		name    string
		setup   map[string]string
		oldKey  string
		newKey  string
		wantErr error
		// want is the exact store contents afterwards.
		want map[string]string
	}{
		{
			name:   "renames to a free key",
			setup:  map[string]string{"a": "1", "b": "2"},
			oldKey: "a", newKey: "c",
			want: map[string]string{"b": "2", "c": "1"},
		},
		{
			name:   "preserves the value of a single key",
			setup:  map[string]string{"a": "hello world"},
			oldKey: "a", newKey: "b",
			want: map[string]string{"b": "hello world"},
		},
		{
			name:   "missing old key is an error and changes nothing",
			setup:  map[string]string{"a": "1"},
			oldKey: "zzz", newKey: "c",
			wantErr: ErrNoSuchKey,
			want:    map[string]string{"a": "1"},
		},
		{
			name:   "occupied destination is refused and changes nothing",
			setup:  map[string]string{"a": "1", "b": "2"},
			oldKey: "a", newKey: "b",
			wantErr: ErrKeyExists,
			want:    map[string]string{"a": "1", "b": "2"},
		},
		{
			name:   "rename to the same key is a no-op",
			setup:  map[string]string{"a": "1"},
			oldKey: "a", newKey: "a",
			want: map[string]string{"a": "1"},
		},
		{
			name:   "rename to the same key still errors when missing",
			oldKey: "a", newKey: "a",
			wantErr: ErrNoSuchKey,
			want:    map[string]string{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			s, _ := newTestStore()
			for k, v := range tc.setup {
				s.Set(k, v)
			}

			err := s.Rename(tc.oldKey, tc.newKey)

			if tc.wantErr == nil {
				if err != nil {
					t.Fatalf("Rename(%q, %q) = %v, want nil", tc.oldKey, tc.newKey, err)
				}
			} else if !errors.Is(err, tc.wantErr) {
				t.Fatalf("Rename(%q, %q) = %v, want %v", tc.oldKey, tc.newKey, err, tc.wantErr)
			}

			got := make(map[string]string, len(tc.want))
			for _, k := range s.keys() {
				v, _ := s.Get(k)
				got[k] = v
			}

			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("store after Rename(%q, %q) = %v, want %v", tc.oldKey, tc.newKey, got, tc.want)
			}
		})
	}
}

func TestStore_RenameWithExpiry(t *testing.T) {
	t.Run("expired source is treated as missing", func(t *testing.T) {
		s, advance := newTestStore()
		if err := s.SetWithTTL("a", "1", time.Second); err != nil {
			t.Fatalf("SetWithTTL: %v", err)
		}

		advance(2 * time.Second)

		if err := s.Rename("a", "b"); !errors.Is(err, ErrNoSuchKey) {
			t.Errorf("Rename(expired source) = %v, want %v", err, ErrNoSuchKey)
		}
		if _, ok := s.Get("b"); ok {
			t.Error("Rename(expired source) created the destination")
		}
	})

	t.Run("expired destination does not block the rename", func(t *testing.T) {
		s, advance := newTestStore()
		if err := s.SetWithTTL("b", "old", time.Second); err != nil {
			t.Fatalf("SetWithTTL: %v", err)
		}
		s.Set("a", "1")

		advance(2 * time.Second)

		if err := s.Rename("a", "b"); err != nil {
			t.Fatalf("Rename onto expired destination = %v, want nil", err)
		}
		if val, ok := s.Get("b"); !ok || val != "1" {
			t.Errorf("Get(b) = (%q, %v), want (\"1\", true)", val, ok)
		}
		if _, ok := s.Get("a"); ok {
			t.Error("Rename left the source key behind")
		}
	})
}
