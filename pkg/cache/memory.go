package cache

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"
)

type memoryEntry struct {
	value      interface{}
	expiration time.Time
}

type memoryCache struct {
	mu     sync.RWMutex
	items  map[string]memoryEntry
	stopGC chan struct{}
	closed bool
}

func NewMemoryCache() Cache {
	c := &memoryCache{
		items:  make(map[string]memoryEntry),
		stopGC: make(chan struct{}),
	}
	go c.startGC()
	return c
}

func (c *memoryCache) Get(_ context.Context, key string) (string, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	item, exists := c.items[key]
	if !exists {
		return "", ErrKeyNotFound
	}
	if !item.expiration.IsZero() && item.expiration.Before(time.Now()) {
		return "", ErrKeyNotFound
	}

	str, ok := item.value.(string)
	if !ok {
		return "", ErrInvalidType
	}
	return str, nil
}

func (c *memoryCache) Set(_ context.Context, key string, value interface{}, ttl time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	var exp time.Time
	if ttl > 0 {
		exp = time.Now().Add(ttl)
	}
	c.items[key] = memoryEntry{value: value, expiration: exp}
	return nil
}

func (c *memoryCache) Delete(_ context.Context, key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.items[key]; !exists {
		return ErrKeyNotFound
	}

	delete(c.items, key)
	return nil
}

func (c *memoryCache) Exists(_ context.Context, key string) (bool, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	item, exists := c.items[key]
	if !exists || (!item.expiration.IsZero() && item.expiration.Before(time.Now())) {
		return false, nil
	}

	return true, nil
}

func (c *memoryCache) Expire(_ context.Context, key string, ttl time.Duration) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	item, exists := c.items[key]
	if !exists {
		return false, nil
	}

	if ttl <= 0 {
		delete(c.items, key)
		return true, nil
	}

	item.expiration = time.Now().Add(ttl)
	c.items[key] = item
	return true, nil
}

func (c *memoryCache) TTL(_ context.Context, key string) (time.Duration, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	item, exists := c.items[key]
	if !exists {
		return 0, ErrKeyNotFound
	}

	if item.expiration.IsZero() {
		return 0, nil
	}

	ttl := time.Until(item.expiration)
	if ttl <= 0 {
		return 0, ErrKeyNotFound
	}

	return ttl, nil
}

func (c *memoryCache) Flush(_ context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items = make(map[string]memoryEntry)
	return nil
}

func (c *memoryCache) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}

	close(c.stopGC)
	c.items = nil
	c.closed = true
	return nil
}

func (c *memoryCache) Increment(_ context.Context, key string, value int64) (int64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	item, ok := c.items[key]
	if !ok {
		return 0, fmt.Errorf("key %s not found", key)
	}

	var result int64
	switch v := item.value.(type) {
	case int:
		result = int64(v) + value
		if int64(int(result)) != result {
			return 0, fmt.Errorf("integer overflow")
		}
		item.value = int(result)
	case int64:
		result = v + value
		item.value = result
	case float64:
		result = int64(v + float64(value))
		item.value = result
	case string:
		if num, err := strconv.ParseInt(v, 10, 64); err == nil {
			result = num + value
			item.value = strconv.FormatInt(result, 10)
		} else {
			if f, err := strconv.ParseFloat(v, 64); err == nil {
				result = int64(f + float64(value))
				item.value = strconv.FormatInt(result, 10)
			} else {
				return 0, fmt.Errorf("value is not a number")
			}
		}
	default:
		return 0, fmt.Errorf("value is not a number")
	}

	c.items[key] = item
	return result, nil
}

func (c *memoryCache) Decrement(ctx context.Context, key string, value int64) (int64, error) {
	return c.Increment(ctx, key, -value)
}

func (c *memoryCache) MGet(_ context.Context, keys ...string) ([]interface{}, error) {
	if len(keys) == 0 {
		return []interface{}{}, nil
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	now := time.Now()
	result := make([]interface{}, len(keys))

	for i, key := range keys {
		item, exists := c.items[key]
		if !exists || (!item.expiration.IsZero() && item.expiration.Before(now)) {
			result[i] = ""
			continue
		}

		switch v := item.value.(type) {
		case string:
			result[i] = v
		default:
			result[i] = fmt.Sprintf("%v", v)
		}
	}

	return result, nil
}

func (c *memoryCache) MSet(_ context.Context, values map[string]interface{}, ttl time.Duration) error {
	if len(values) == 0 {
		return nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	var exp time.Time
	if ttl > 0 {
		exp = time.Now().Add(ttl)
	}

	for k, v := range values {
		c.items[k] = memoryEntry{value: v, expiration: exp}
	}

	return nil
}

func (c *memoryCache) startGC() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.cleanup()
		case <-c.stopGC:
			return
		}
	}
}

func (c *memoryCache) cleanup() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return fmt.Errorf("cache is closed")
	}

	if c.items == nil {
		return fmt.Errorf("cache items map is nil")
	}

	var expiredCount int
	now := time.Now()
	for k, v := range c.items {
		if !v.expiration.IsZero() && v.expiration.Before(now) {
			delete(c.items, k)
			expiredCount++
		}
	}

	return nil
}
