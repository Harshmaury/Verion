package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// ErrChallengeNotFound is returned when a challenge key does not exist or was already consumed.
var ErrChallengeNotFound = errors.New("store: challenge not found or already consumed")

const challengeKeyPrefix = "webauthn:challenge:"

// RedisStore wraps go-redis for Verion's challenge and session storage.
type RedisStore struct {
	client *redis.Client
}

// New creates a RedisStore from a Redis URL.
func New(ctx context.Context, redisURL string) (*RedisStore, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("store: parse redis url: %w", err)
	}
	client := redis.NewClient(opts)
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("store: redis ping failed: %w", err)
	}
	return &RedisStore{client: client}, nil
}

// Client returns the underlying *redis.Client for use by SessionStore.
func (s *RedisStore) Client() *redis.Client {
	return s.client
}

// SetChallenge stores a WebAuthn challenge with TTL.
func (s *RedisStore) SetChallenge(ctx context.Context, challengeID string, data []byte, ttl time.Duration) error {
	key := challengeKeyPrefix + challengeID
	if err := s.client.Set(ctx, key, data, ttl).Err(); err != nil {
		return fmt.Errorf("store: set challenge: %w", err)
	}
	return nil
}

// GetChallenge retrieves and atomically deletes a challenge (single-use, GETDEL).
func (s *RedisStore) GetChallenge(ctx context.Context, challengeID string) ([]byte, error) {
	key := challengeKeyPrefix + challengeID
	data, err := s.client.GetDel(ctx, key).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, ErrChallengeNotFound
		}
		return nil, fmt.Errorf("store: get challenge: %w", err)
	}
	return data, nil
}

// Close closes the Redis connection.
func (s *RedisStore) Close() error {
	return s.client.Close()
}
