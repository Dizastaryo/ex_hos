package redis

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

type SessionStore struct {
	cache *Cache

	mu             sync.Mutex
	refreshTokens  map[string]sessionRecord
	blacklisted    map[string]sessionRecord
	rateLimits     map[string]rateLimitRecord
}

type sessionRecord struct {
	userID    string
	expiresAt time.Time
}

func (r sessionRecord) expired(now time.Time) bool {
	if r.expiresAt.IsZero() {
		return false
	}
	return now.After(r.expiresAt)
}

type rateLimitRecord struct {
	count     int64
	expiresAt time.Time
}

func NewSessionStore(cache *Cache) *SessionStore {
	return &SessionStore{
		cache:         cache,
		refreshTokens: make(map[string]sessionRecord),
		blacklisted:   make(map[string]sessionRecord),
		rateLimits:    make(map[string]rateLimitRecord),
	}
}

func (s *SessionStore) StoreRefreshToken(_ context.Context, userID, token string, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec := sessionRecord{userID: userID}
	if ttl > 0 {
		rec.expiresAt = time.Now().Add(ttl)
	}
	s.refreshTokens[refreshTokenKey(token)] = rec
	return nil
}

func (s *SessionStore) GetUserIDByRefreshToken(_ context.Context, token string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.refreshTokens[refreshTokenKey(token)]
	if !ok || rec.expired(time.Now()) {
		if ok {
			delete(s.refreshTokens, refreshTokenKey(token))
		}
		return "", fmt.Errorf("refresh token not found")
	}
	return rec.userID, nil
}

func (s *SessionStore) RevokeRefreshToken(_ context.Context, token string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.refreshTokens, refreshTokenKey(token))
	return nil
}

func (s *SessionStore) RevokeAllUserTokens(_ context.Context, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for k, rec := range s.refreshTokens {
		if rec.userID == userID {
			delete(s.refreshTokens, k)
		}
	}
	return nil
}

func (s *SessionStore) BlacklistToken(_ context.Context, token, userID string, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec := sessionRecord{userID: userID}
	if ttl > 0 {
		rec.expiresAt = time.Now().Add(ttl)
	}
	s.blacklisted[blacklistKey(token)] = rec
	return nil
}

func (s *SessionStore) IsTokenBlacklisted(_ context.Context, token string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.blacklisted[blacklistKey(token)]
	if !ok {
		return false, nil
	}
	if rec.expired(time.Now()) {
		delete(s.blacklisted, blacklistKey(token))
		return false, nil
	}
	return true, nil
}

func (s *SessionStore) SetRateLimit(_ context.Context, key string, _ int, window time.Duration) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	rec, ok := s.rateLimits[key]
	if !ok || (!rec.expiresAt.IsZero() && now.After(rec.expiresAt)) {
		rec = rateLimitRecord{}
		if window > 0 {
			rec.expiresAt = now.Add(window)
		}
	}
	rec.count++
	s.rateLimits[key] = rec
	return rec.count, nil
}

func (s *SessionStore) GetRateLimit(_ context.Context, key string) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.rateLimits[key]
	if !ok {
		return 0, nil
	}
	if !rec.expiresAt.IsZero() && time.Now().After(rec.expiresAt) {
		delete(s.rateLimits, key)
		return 0, nil
	}
	return rec.count, nil
}

func refreshTokenKey(token string) string {
	return "refresh_token:" + token
}

func blacklistKey(token string) string {
	return "blacklist:" + token
}

// Used by services that previously called sessionStore.RevokeAllUserTokens via prefix scan.
func hasPrefix(s, prefix string) bool { return strings.HasPrefix(s, prefix) }
