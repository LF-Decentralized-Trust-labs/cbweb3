package noncestore

import (
	"context"
	"sync"
	"time"
)

type nonceEntry struct {
	nonce     string
	expiresAt time.Time
}

// InMemoryStore is a TTL-based in-memory nonce store protected by a mutex.
// Suitable for single-instance dev deployments only; use RedisStore in production.
type InMemoryStore struct {
	mu     sync.Mutex
	nonces map[string]nonceEntry
}

// NewInMemoryStore returns an initialised InMemoryStore.
func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{nonces: make(map[string]nonceEntry)}
}

// Set stores a nonce for userID with the given TTL.
func (s *InMemoryStore) Set(_ context.Context, userID, nonce string, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nonces[userID] = nonceEntry{nonce: nonce, expiresAt: time.Now().Add(ttl)}
	return nil
}

// GetAndDelete atomically retrieves and removes the nonce for userID.
func (s *InMemoryStore) GetAndDelete(_ context.Context, userID string) (string, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.nonces[userID]
	if ok {
		delete(s.nonces, userID)
	}
	if !ok || time.Now().After(entry.expiresAt) {
		return "", false, nil
	}
	return entry.nonce, true, nil
}
