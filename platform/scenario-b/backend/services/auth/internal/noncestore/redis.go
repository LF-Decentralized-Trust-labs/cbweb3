// SPDX-License-Identifier: Apache-2.0

package noncestore

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

const nonceKeyPrefix = "pki:nonce:"

// RedisStore is a Redis-backed nonce store suitable for multi-instance production deployments.
// Nonces are stored as plain strings with a Redis TTL; GetAndDelete uses a Lua script for atomicity.
type RedisStore struct {
	client *redis.Client
}

// NewRedisStore creates a RedisStore connected to the given address.
// password may be empty; db is typically 0.
func NewRedisStore(addr, password string, db int) *RedisStore {
	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})
	return &RedisStore{client: rdb}
}

// Set stores a nonce for userID in Redis with the specified TTL.
func (s *RedisStore) Set(ctx context.Context, userID, nonce string, ttl time.Duration) error {
	return s.client.Set(ctx, nonceKeyPrefix+userID, nonce, ttl).Err()
}

// GetAndDelete atomically retrieves and removes the nonce for userID via a Lua script.
func (s *RedisStore) GetAndDelete(ctx context.Context, userID string) (string, bool, error) {
	// Atomic GET + DEL using a Lua script to prevent TOCTOU races.
	script := redis.NewScript(`
		local val = redis.call("GET", KEYS[1])
		if val == false then
			return nil
		end
		redis.call("DEL", KEYS[1])
		return val
	`)
	result, err := script.Run(ctx, s.client, []string{nonceKeyPrefix + userID}).Text()
	if errors.Is(err, redis.Nil) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return result, true, nil
}
