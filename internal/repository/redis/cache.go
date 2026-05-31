package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"sync"
	"time"
)

type cacheEntry struct {
	value     []byte
	expiresAt time.Time
}

func (e cacheEntry) expired(now time.Time) bool {
	if e.expiresAt.IsZero() {
		return false
	}
	return now.After(e.expiresAt)
}

type Cache struct {
	mu      sync.Mutex
	entries map[string]cacheEntry
	stop    chan struct{}
}

func NewCache(_ string) (*Cache, error) {
	c := &Cache{
		entries: make(map[string]cacheEntry),
		stop:    make(chan struct{}),
	}
	go c.gcLoop()
	return c, nil
}

func (c *Cache) Close() error {
	select {
	case <-c.stop:
	default:
		close(c.stop)
	}
	return nil
}

func (c *Cache) gcLoop() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-c.stop:
			return
		case now := <-ticker.C:
			c.gc(now)
		}
	}
}

func (c *Cache) gc(now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for k, e := range c.entries {
		if e.expired(now) {
			delete(c.entries, k)
		}
	}
}

func (c *Cache) Set(_ context.Context, key string, value interface{}, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshal cache value: %w", err)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	entry := cacheEntry{value: data}
	if ttl > 0 {
		entry.expiresAt = time.Now().Add(ttl)
	}
	c.entries[key] = entry
	return nil
}

func (c *Cache) setRaw(key string, raw []byte, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry := cacheEntry{value: raw}
	if ttl > 0 {
		entry.expiresAt = time.Now().Add(ttl)
	}
	c.entries[key] = entry
}

func (c *Cache) getRaw(key string) ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[key]
	if !ok {
		return nil, false
	}
	if e.expired(time.Now()) {
		delete(c.entries, key)
		return nil, false
	}
	return e.value, true
}

func (c *Cache) Get(_ context.Context, key string, dest interface{}) error {
	raw, ok := c.getRaw(key)
	if !ok {
		return ErrCacheMiss
	}
	return json.Unmarshal(raw, dest)
}

func (c *Cache) Delete(_ context.Context, keys ...string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, k := range keys {
		delete(c.entries, k)
	}
	return nil
}

func (c *Cache) DeletePattern(_ context.Context, pattern string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	for k := range c.entries {
		if matched, err := path.Match(pattern, k); err == nil && matched {
			delete(c.entries, k)
		}
	}
	return nil
}

func (c *Cache) Exists(_ context.Context, key string) (bool, error) {
	_, ok := c.getRaw(key)
	return ok, nil
}

func (c *Cache) Increment(_ context.Context, key string) (int64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	var n int64
	if e, ok := c.entries[key]; ok && !e.expired(time.Now()) {
		_ = json.Unmarshal(e.value, &n)
	}
	n++
	data, _ := json.Marshal(n)
	existing, ok := c.entries[key]
	if ok && !existing.expired(time.Now()) {
		existing.value = data
		c.entries[key] = existing
	} else {
		c.entries[key] = cacheEntry{value: data}
	}
	return n, nil
}

func (c *Cache) IncrementWithExpiry(_ context.Context, key string, ttl time.Duration) (int64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	var n int64
	now := time.Now()
	if e, ok := c.entries[key]; ok && !e.expired(now) {
		_ = json.Unmarshal(e.value, &n)
	}
	n++
	data, _ := json.Marshal(n)
	entry := cacheEntry{value: data}
	if ttl > 0 {
		entry.expiresAt = now.Add(ttl)
	}
	c.entries[key] = entry
	return n, nil
}

func (c *Cache) GetTTL(_ context.Context, key string) (time.Duration, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[key]
	if !ok {
		return -2 * time.Second, nil
	}
	if e.expiresAt.IsZero() {
		return -1 * time.Second, nil
	}
	return time.Until(e.expiresAt), nil
}

var ErrCacheMiss = fmt.Errorf("cache miss")

func IsCacheMiss(err error) bool {
	return err == ErrCacheMiss
}

// ----- Convenience invalidators (audit P1: «cache invalidation непоследовательная»). -----
//
// Each domain mutation should call these instead of building keys[] inline:
// the `cache.Delete` / `cache.DeletePattern` distinction was easy to forget,
// and one bug in follow_service silently leaked a wildcard string into
// cache.Delete (where it was treated as literal "feed:<id>:*", never
// matching any cached page).
//
// Patterns used here are coarse-grained on purpose — a dropped cache page
// is cheap to refill, a stale-feed regression is not.

// InvalidateUser drops every cached entry tied to a single user: their
// profile record (by id and username) and their personal feed pages.
// Username may be empty for callers that don't have it handy.
func (c *Cache) InvalidateUser(ctx context.Context, userID, username string) {
	keys := []string{UserKey(userID)}
	if username != "" {
		keys = append(keys, UserByUsernameKey(username))
	}
	_ = c.Delete(ctx, keys...)
	_ = c.DeletePattern(ctx, fmt.Sprintf("feed:%s:*", userID))
	_ = c.Delete(ctx, StoryFeedKey(userID))
}

// InvalidateAllFeeds nukes every cached feed page across all users. Used
// after a graph change (follow/unfollow, follow request accept) where
// multiple users' feeds may now show different posts. Coarse-grained but
// correct: per-follower targeted invalidation requires a follower lookup
// per call which doubles the DB hit, and feed cache TTL is short anyway.
func (c *Cache) InvalidateAllFeeds(ctx context.Context) {
	_ = c.DeletePattern(ctx, "feed:*")
}

// Cache key helpers
func UserKey(userID string) string {
	return fmt.Sprintf("user:%s", userID)
}

func UserByUsernameKey(username string) string {
	return fmt.Sprintf("user:username:%s", username)
}

func PostKey(postID string) string {
	return fmt.Sprintf("post:%s", postID)
}

func FeedKey(userID string, page int) string {
	return fmt.Sprintf("feed:%s:%d", userID, page)
}

func StoryFeedKey(userID string) string {
	return fmt.Sprintf("story_feed:%s", userID)
}

func NotifCountKey(userID string) string {
	return fmt.Sprintf("notif_count:%s", userID)
}
