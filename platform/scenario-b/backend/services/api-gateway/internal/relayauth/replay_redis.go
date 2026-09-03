// SPDX-License-Identifier: Apache-2.0

package relayauth

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// seenKeyPrefix namespaces the guard's keys in a Redis instance the entity already uses for other
// things (the auth service keeps its PKI login nonces there under "pki:nonce:").
const seenKeyPrefix = "relayauth:seen:"

// RedisSeenStore is a SeenStore backed by the entity's Redis, so every replica of a gateway admits a
// signature at most once between them.
//
// Redis is not a new dependency of the deployment: the compose template already runs one per entity
// and passes REDIS_ADDR, and the auth service already stores its login nonces there. What is new is
// this service using it — the alternative was to declare the central bank gateway single-replica and
// leave the guard's promise resting on that.
//
// SET NX is the whole mechanism: the first caller creates the key and is admitted, everyone else
// finds it present. It is atomic in Redis, which is the property the interface asks for — two
// replicas asking at the same instant cannot both be told yes.
type RedisSeenStore struct {
	client *redis.Client
}

// NewRedisSeenStore connects to addr (password may be empty; db is typically 0).
func NewRedisSeenStore(addr, password string, db int) *RedisSeenStore {
	return &RedisSeenStore{client: redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})}
}

// Admit reports whether key is being used for the first time within ttl.
//
// An error is returned rather than swallowed: the caller decides what an unreachable cache means,
// and here it deliberately means "keep serving" (see ReplayGuard.Admit), which is a policy that
// belongs at the call site and not in the store.
func (s *RedisSeenStore) Admit(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	return s.client.SetNX(ctx, seenKeyPrefix+key, 1, ttl).Result()
}

// Close releases the connection pool.
func (s *RedisSeenStore) Close() error {
	if s == nil || s.client == nil {
		return nil
	}
	return s.client.Close()
}
