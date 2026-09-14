package kvcache

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var ErrCacheMiss = errors.New("cache miss")

type entry struct {
	Value     json.RawMessage `json:"value"`
	ExpiresAt time.Time       `json:"expiresAt"` // per-key expiry timestamp
}

type Cache struct {
	mu      sync.Mutex
	path    string
	ttl     time.Duration
	loaded  bool
	entries map[string]entry
	dirty   bool
}

func configFilePath(file string) string {
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = "."
	}
	return filepath.Join(dir, "eves", file)
}

// New creates a disk-backed cache stored as a JSON file at path.
// ttl <= 0 means "never expire" (still stored per key as zero time).
func New(path string, ttl time.Duration) *Cache {
	return &Cache{
		path:    configFilePath(path),
		ttl:     ttl,
		entries: make(map[string]entry),
	}
}

// Get returns the cached value for key if present and not expired; otherwise ErrCacheMiss.
func Get[V any](c *Cache, key string) (V, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	var zero V
	if err := c.loadLocked(); err != nil {
		return zero, err
	}

	e, ok := c.entries[key]
	if !ok || isExpired(e) {
		if ok {
			delete(c.entries, key)
			c.dirty = true
		}
		return zero, ErrCacheMiss
	}

	if err := json.Unmarshal(e.Value, &zero); err != nil {
		// Corrupt value for this key -> treat as miss and delete it.
		delete(c.entries, key)
		c.dirty = true
		return zero, ErrCacheMiss
	}

	return zero, nil
}

// Entries returns a snapshot of valid cached values, omitting expired or corrupt entries.
func Entries[V any](c *Cache) (map[string]V, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.loadLocked(); err != nil {
		return nil, err
	}
	values := make(map[string]V)
	for key, e := range c.entries {
		if isExpired(e) {
			continue
		}
		var value V
		if err := json.Unmarshal(e.Value, &value); err == nil {
			values[key] = value
		}
	}
	return values, nil
}

// Put stores v under key with an expiry timestamp based on the cache TTL.
func Put[V any](c *Cache, key string, v V) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.loadLocked(); err != nil {
		return err
	}

	b, err := json.Marshal(v)
	if err != nil {
		return err
	}

	var exp time.Time
	if c.ttl > 0 {
		exp = time.Now().Add(c.ttl)
	}

	c.entries[key] = entry{Value: b, ExpiresAt: exp}
	c.dirty = true
	return c.saveLocked()
}

// GetOrLoad returns cached value if present+valid; otherwise runs loader(), stores, returns.
func GetOrLoad[V any](c *Cache, key string, loader func() (V, error)) (V, error) {
	// Try cache first
	if v, err := Get[V](c, key); err == nil {
		return v, nil
	}

	// Miss/expired -> compute
	v, err := loader()
	if err != nil {
		var zero V
		return zero, err
	}

	// Persist result (if this fails, still return v)
	_ = Put(c, key, v)
	return v, nil
}

// Optional: call at end of program if you switch Put() to not save immediately.
// With current implementation Put() already saves, but Flush is handy if you refactor.
func (c *Cache) Flush() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.loadLocked(); err != nil {
		return err
	}
	if !c.dirty {
		return nil
	}
	return c.saveLocked()
}

func isExpired(e entry) bool {
	return !e.ExpiresAt.IsZero() && time.Now().After(e.ExpiresAt)
}

func (c *Cache) loadLocked() error {
	if c.loaded {
		return nil
	}
	c.loaded = true

	b, err := os.ReadFile(c.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var m map[string]entry
	if err := json.Unmarshal(b, &m); err != nil {
		// If file is corrupt, start empty (you can change this to "return err" if desired)
		c.entries = make(map[string]entry)
		c.dirty = true
		return nil
	}

	c.entries = m

	// Drop expired entries on load (per-key expiry)
	now := time.Now()
	changed := false
	for k, e := range c.entries {
		if !e.ExpiresAt.IsZero() && now.After(e.ExpiresAt) {
			delete(c.entries, k)
			changed = true
		}
	}
	if changed {
		c.dirty = true
		// Save immediately so future runs don't re-load expired stuff
		return c.saveLocked()
	}

	return nil
}

func (c *Cache) saveLocked() error {
	if !c.dirty {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(c.path), 0o755); err != nil {
		return err
	}

	b, err := json.MarshalIndent(c.entries, "", "  ")
	if err != nil {
		return err
	}

	tmp := c.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, c.path); err != nil {
		return err
	}

	c.dirty = false
	return nil
}
