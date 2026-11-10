package store

import (
	"slices"
	"sync"
	"time"
)

type Entry struct {
	value    string
	expireAt time.Time
}

type Store struct {
	mu      sync.Mutex
	data    map[string]Entry
	maxSize int
	now     func() time.Time
}

func NewStore(maxSize ...int) *Store {
	limit := -1

	if len(maxSize) > 0 {
		limit = maxSize[0]
	}
	return &Store{
		data:    make(map[string]Entry),
		maxSize: limit,
		now:     time.Now,
	}
}

func isLive(entry Entry, now time.Time) bool {
	return entry.expireAt.IsZero() || now.Before(entry.expireAt)
}

func (s *Store) Get(key string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.data[key]
	if !ok {
		return "", false
	}
	if !isLive(entry, s.now()) {
		delete(s.data, key)
		return "", false
	}
	return entry.value, true
}

func (s *Store) Set(key, value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.setLocked(key, Entry{value: value})
}

func (s *Store) setLocked(key string, entry Entry) error {
	if s.maxSize >= 0 {
		if _, exists := s.data[key]; !exists && len(s.data) >= s.maxSize {
			s.sweepLocked()
			if len(s.data) >= s.maxSize {
				return ErrOOM
			}
		}
	}

	s.data[key] = entry
	return nil
}

func (s *Store) Delete(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.data, key)
}

func (s *Store) keys() []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now()
	keys := make([]string, 0, len(s.data))

	for key, entry := range s.data {
		if isLive(entry, now) {
			keys = append(keys, key)
		}
	}
	slices.Sort(keys)
	return keys
}

func (s *Store) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now()
	live := 0
	for _, entry := range s.data {
		if isLive(entry, now) {
			live++
		}
	}
	return live
}

func (s *Store) Rename(oldKey, newKey string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now()
	entry, ok := s.data[oldKey]

	if !ok || !isLive(entry, now) {
		if ok {
			delete(s.data, oldKey)
		}
		return ErrNoSuchKey
	}
	if oldKey == newKey {
		return nil
	}

	if dest, exists := s.data[newKey]; exists && isLive(dest, now) {
		return ErrKeyExists
	}

	delete(s.data, oldKey)
	s.data[newKey] = entry
	return nil
}

func (s *Store) Pop(key string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.data[key]
	if !ok {
		return "", false
	}

	delete(s.data, key)
	if !isLive(entry, s.now()) {
		return "", false
	}
	return entry.value, true
}
