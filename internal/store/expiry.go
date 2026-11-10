package store

import "time"

func (s *Store) SetWithTTL(key, value string, ttl time.Duration) error {
	if ttl <= 0 {
		return ErrTimeCorrect
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	return s.setLocked(key, Entry{value: value, expireAt: s.now().Add(ttl)})
}

func (s *Store) Sweep() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.sweepLocked()
}

func (s *Store) sweepLocked() int {
	now := s.now()
	removed := 0

	for key, entry := range s.data {
		if !isLive(entry, now) {
			delete(s.data, key)
			removed++
		}
	}
	return removed
}
