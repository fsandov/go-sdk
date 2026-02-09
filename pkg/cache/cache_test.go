package cache

import (
	"context"
	"testing"
	"time"
)

func TestMemoryCacheTTL(t *testing.T) {
	c := NewMemoryCache()
	defer c.Close()
	ctx := context.Background()

	t.Run("key not found", func(t *testing.T) {
		_, err := c.TTL(ctx, "nonexistent")
		if err != ErrKeyNotFound {
			t.Errorf("expected ErrKeyNotFound, got %v", err)
		}
	})

	t.Run("key without expiry", func(t *testing.T) {
		_ = c.Set(ctx, "no-ttl", "value", 0)
		ttl, err := c.TTL(ctx, "no-ttl")
		if err != nil {
			t.Errorf("expected nil error for key without expiry, got %v", err)
		}
		if ttl != 0 {
			t.Errorf("expected 0 duration for no-expiry key, got %v", ttl)
		}
	})

	t.Run("key with expiry", func(t *testing.T) {
		_ = c.Set(ctx, "with-ttl", "value", 10*time.Minute)
		ttl, err := c.TTL(ctx, "with-ttl")
		if err != nil {
			t.Errorf("expected nil error, got %v", err)
		}
		if ttl <= 0 || ttl > 10*time.Minute {
			t.Errorf("expected positive TTL <= 10m, got %v", ttl)
		}
	})

	t.Run("expired key", func(t *testing.T) {
		_ = c.Set(ctx, "expired", "value", 1*time.Millisecond)
		time.Sleep(5 * time.Millisecond)
		_, err := c.TTL(ctx, "expired")
		if err != ErrKeyNotFound {
			t.Errorf("expected ErrKeyNotFound for expired key, got %v", err)
		}
	})
}

func TestMemoryCacheBasicOps(t *testing.T) {
	c := NewMemoryCache()
	defer c.Close()
	ctx := context.Background()

	_ = c.Set(ctx, "key1", "value1", time.Minute)

	val, err := c.Get(ctx, "key1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "value1" {
		t.Fatalf("expected value1, got %s", val)
	}

	exists, err := c.Exists(ctx, "key1")
	if err != nil || !exists {
		t.Fatal("expected key to exist")
	}

	err = c.Delete(ctx, "key1")
	if err != nil {
		t.Fatalf("unexpected error on delete: %v", err)
	}

	_, err = c.Get(ctx, "key1")
	if err != ErrKeyNotFound {
		t.Fatalf("expected ErrKeyNotFound after delete, got %v", err)
	}
}
