//go:build integration

package cache

import (
	"context"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"
)

func setupRedisContainer(t *testing.T) (Cache, func()) {
	t.Helper()
	ctx := context.Background()

	container, err := tcredis.Run(ctx, "redis:7-alpine")
	if err != nil {
		t.Fatalf("failed to start redis container: %v", err)
	}

	endpoint, err := container.Endpoint(ctx, "")
	if err != nil {
		t.Fatalf("failed to get redis endpoint: %v", err)
	}

	client := redis.NewClient(&redis.Options{Addr: endpoint})
	if err := client.Ping(ctx).Err(); err != nil {
		t.Fatalf("failed to ping redis: %v", err)
	}

	c := &redisCache{client: client}
	cleanup := func() {
		client.Close()
		container.Terminate(ctx)
	}
	return c, cleanup
}

func TestRedisIntegration_SetGetDelete(t *testing.T) {
	c, cleanup := setupRedisContainer(t)
	defer cleanup()
	ctx := context.Background()

	err := c.Set(ctx, "key1", "value1", time.Minute)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	val, err := c.Get(ctx, "key1")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if val != "value1" {
		t.Fatalf("expected value1, got %s", val)
	}

	exists, err := c.Exists(ctx, "key1")
	if err != nil || !exists {
		t.Fatalf("expected key to exist")
	}

	err = c.Delete(ctx, "key1")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	_, err = c.Get(ctx, "key1")
	if err != ErrKeyNotFound {
		t.Fatalf("expected ErrKeyNotFound after delete, got %v", err)
	}
}

func TestRedisIntegration_TTLSemantics(t *testing.T) {
	c, cleanup := setupRedisContainer(t)
	defer cleanup()
	ctx := context.Background()

	t.Run("nonexistent key returns ErrKeyNotFound (redis -2)", func(t *testing.T) {
		_, err := c.TTL(ctx, "does-not-exist")
		if err != ErrKeyNotFound {
			t.Errorf("expected ErrKeyNotFound, got %v", err)
		}
	})

	t.Run("key without expiry returns 0 nil (redis -1)", func(t *testing.T) {
		err := c.Set(ctx, "persist", "val", 0)
		if err != nil {
			t.Fatalf("Set failed: %v", err)
		}
		ttl, err := c.TTL(ctx, "persist")
		if err != nil {
			t.Errorf("expected nil error, got %v", err)
		}
		if ttl != 0 {
			t.Errorf("expected 0 duration for no-expiry key, got %v", ttl)
		}
	})

	t.Run("key with TTL returns positive duration", func(t *testing.T) {
		err := c.Set(ctx, "expiring", "val", 10*time.Minute)
		if err != nil {
			t.Fatalf("Set failed: %v", err)
		}
		ttl, err := c.TTL(ctx, "expiring")
		if err != nil {
			t.Errorf("expected nil error, got %v", err)
		}
		if ttl <= 0 || ttl > 10*time.Minute {
			t.Errorf("expected positive TTL <= 10m, got %v", ttl)
		}
	})

	t.Run("expired key returns ErrKeyNotFound", func(t *testing.T) {
		err := c.Set(ctx, "short-lived", "val", 1*time.Second)
		if err != nil {
			t.Fatalf("Set failed: %v", err)
		}

		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			_, err = c.TTL(ctx, "short-lived")
			if err == ErrKeyNotFound {
				return
			}
			time.Sleep(200 * time.Millisecond)
		}
		t.Error("key did not expire within expected window")
	})
}

func TestRedisIntegration_MSetMGet(t *testing.T) {
	c, cleanup := setupRedisContainer(t)
	defer cleanup()
	ctx := context.Background()

	pairs := map[string]string{
		"mk1": "mv1",
		"mk2": "mv2",
		"mk3": "mv3",
	}
	err := c.MSet(ctx, pairs, time.Minute)
	if err != nil {
		t.Fatalf("MSet failed: %v", err)
	}

	results, err := c.MGet(ctx, "mk1", "mk2", "mk3", "mk4-nonexistent")
	if err != nil {
		t.Fatalf("MGet failed: %v", err)
	}
	if results["mk1"] != "mv1" || results["mk2"] != "mv2" || results["mk3"] != "mv3" {
		t.Errorf("MGet returned unexpected values: %v", results)
	}
	if _, found := results["mk4-nonexistent"]; found {
		t.Error("expected mk4-nonexistent to be absent from results")
	}
}

func TestRedisIntegration_IncrementDecrement(t *testing.T) {
	c, cleanup := setupRedisContainer(t)
	defer cleanup()
	ctx := context.Background()

	val, err := c.Increment(ctx, "counter")
	if err != nil {
		t.Fatalf("Increment failed: %v", err)
	}
	if val != 1 {
		t.Errorf("expected 1, got %d", val)
	}

	val, err = c.Increment(ctx, "counter")
	if err != nil {
		t.Fatalf("Increment failed: %v", err)
	}
	if val != 2 {
		t.Errorf("expected 2, got %d", val)
	}

	val, err = c.Decrement(ctx, "counter")
	if err != nil {
		t.Fatalf("Decrement failed: %v", err)
	}
	if val != 1 {
		t.Errorf("expected 1, got %d", val)
	}
}
