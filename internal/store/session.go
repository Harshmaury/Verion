package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// ErrSessionNotFound is returned when a session does not exist or has expired.
var ErrSessionNotFound = errors.New("store: session not found or expired")

// SessionData holds the server-side session state stored in Redis.
type SessionData struct {
	SessionID  string    `json:"session_id"`
	IdentityID string    `json:"identity_id"`
	TenantID   string    `json:"tenant_id"`
	Handle     string    `json:"handle"`
	CreatedAt  time.Time `json:"created_at"`
	LastSeenAt time.Time `json:"last_seen_at"`
	DeviceInfo string    `json:"device_info"`
	IPAddress  string    `json:"ip_address"`
}

// SessionStore manages authenticated sessions in Redis.
type SessionStore struct {
	client *redis.Client
	ttl    time.Duration
}

// NewSessionStore creates a SessionStore using the provided Redis client.
// ttl: how long sessions stay alive without activity (default: 24 hours).
func NewSessionStore(client *redis.Client, ttl time.Duration) *SessionStore {
	return &SessionStore{client: client, ttl: ttl}
}

// key returns the Redis key for a session: "session:{session_id}"
func (s *SessionStore) key(sessionID string) string {
	return fmt.Sprintf("session:%s", sessionID)
}

// indexKey returns the Redis key for the identity's session index.
// "sessions:identity:{identity_id}" — a Redis Set of session IDs.
func (s *SessionStore) indexKey(identityID string) string {
	return fmt.Sprintf("sessions:identity:%s", identityID)
}

// Create stores a new session and adds it to the identity's session index.
// Uses a Redis pipeline for atomicity.
func (s *SessionStore) Create(ctx context.Context, data *SessionData) error {
	b, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("session store: marshal: %w", err)
	}

	pipe := s.client.Pipeline()
	pipe.Set(ctx, s.key(data.SessionID), b, s.ttl)
	pipe.SAdd(ctx, s.indexKey(data.IdentityID), data.SessionID)
	pipe.Expire(ctx, s.indexKey(data.IdentityID), s.ttl)

	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("session store: create: %w", err)
	}
	return nil
}

// Get retrieves a session by ID. Updates LastSeenAt and refreshes TTL (sliding expiry).
// Returns ErrSessionNotFound if the session does not exist or has expired.
func (s *SessionStore) Get(ctx context.Context, sessionID string) (*SessionData, error) {
	b, err := s.client.Get(ctx, s.key(sessionID)).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, ErrSessionNotFound
		}
		return nil, fmt.Errorf("session store: get: %w", err)
	}

	var data SessionData
	if err := json.Unmarshal(b, &data); err != nil {
		return nil, fmt.Errorf("session store: unmarshal: %w", err)
	}

	// Sliding expiry: update LastSeenAt and refresh TTL.
	data.LastSeenAt = time.Now()
	updated, err := json.Marshal(&data)
	if err == nil {
		// Best-effort — don't fail the read if the update fails.
		_ = s.client.Set(ctx, s.key(sessionID), updated, s.ttl).Err()
	}

	return &data, nil
}

// Delete removes a session and removes it from the identity's index.
func (s *SessionStore) Delete(ctx context.Context, sessionID string) error {
	// Get session first to find the identity for index cleanup.
	b, err := s.client.Get(ctx, s.key(sessionID)).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return ErrSessionNotFound
		}
		return fmt.Errorf("session store: delete get: %w", err)
	}

	var data SessionData
	if err := json.Unmarshal(b, &data); err != nil {
		return fmt.Errorf("session store: delete unmarshal: %w", err)
	}

	pipe := s.client.Pipeline()
	pipe.Del(ctx, s.key(sessionID))
	pipe.SRem(ctx, s.indexKey(data.IdentityID), sessionID)

	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("session store: delete: %w", err)
	}
	return nil
}

// DeleteAll removes all sessions for an identity (multi-device logout).
func (s *SessionStore) DeleteAll(ctx context.Context, identityID string) error {
	sessionIDs, err := s.client.SMembers(ctx, s.indexKey(identityID)).Result()
	if err != nil {
		return fmt.Errorf("session store: delete all: list sessions: %w", err)
	}

	if len(sessionIDs) == 0 {
		return nil
	}

	pipe := s.client.Pipeline()
	for _, sid := range sessionIDs {
		pipe.Del(ctx, s.key(sid))
	}
	pipe.Del(ctx, s.indexKey(identityID))

	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("session store: delete all: %w", err)
	}
	return nil
}

// ListByIdentity returns all active sessions for an identity.
func (s *SessionStore) ListByIdentity(ctx context.Context, identityID string) ([]*SessionData, error) {
	sessionIDs, err := s.client.SMembers(ctx, s.indexKey(identityID)).Result()
	if err != nil {
		return nil, fmt.Errorf("session store: list: %w", err)
	}

	var sessions []*SessionData
	for _, sid := range sessionIDs {
		b, err := s.client.Get(ctx, s.key(sid)).Bytes()
		if err != nil {
			if errors.Is(err, redis.Nil) {
				// Session expired — clean up stale index entry.
				_ = s.client.SRem(ctx, s.indexKey(identityID), sid)
				continue
			}
			return nil, fmt.Errorf("session store: list get %s: %w", sid, err)
		}
		var data SessionData
		if err := json.Unmarshal(b, &data); err != nil {
			continue
		}
		sessions = append(sessions, &data)
	}
	return sessions, nil
}
