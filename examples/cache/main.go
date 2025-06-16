package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/fsandov/go-sdk/pkg/cache"
)

func main() {
	demoMemoryCache()
	// Uncomment to test with Redis
	// demoRedisCache()
}

func demoMemoryCache() {
	c := cache.NewMemoryCache()
	defer c.Close()

	runDemo(c, "Memory Cache")
}

func demoRedisCache() {
	cfg := cache.RedisConfig{
		Enabled:     true,
		Addr:        "localhost:6379",
		Password:    "",
		DB:          0,
		PoolSize:    20,
		DialTimeout: 10 * time.Second,
	}

	c, err := cache.NewRedisCacheFromConfig(cfg, cache.WithReadTimeout(2*time.Second), cache.WithWriteTimeout(2*time.Second))
	if err != nil {
		log.Fatalf("Error connecting to Redis: %v", err)
	}
	defer c.Close()

	runDemo(c, "Redis Cache")
}

func runDemo(c cache.Cache, cacheType string) {
	ctx := context.Background()

	err := c.Set(ctx, "greeting", "Hello, World!", 5*time.Minute)
	if err != nil {
		log.Fatalf("Error setting value: %v", err)
	}

	val, err := c.Get(ctx, "greeting")
	if err != nil {
		log.Fatalf("Error getting value: %v", err)
	}
	fmt.Printf("Retrieved value: %s\n", val)

	exists, err := c.Exists(ctx, "greeting")
	if err != nil {
		log.Fatalf("Error checking existence: %v", err)
	}
	fmt.Printf("Does 'greeting' exist? %v\n", exists)

	ttl, err := c.TTL(ctx, "greeting")
	if err != nil {
		log.Fatalf("Error getting TTL: %v", err)
	}
	fmt.Printf("Remaining TTL for 'greeting': %v\n", ttl.Round(time.Second))

	fmt.Printf("\n--- Working with JSON (%s) ---\n", cacheType)
	fmt.Printf("\n--- Numeric Operations (%s) ---\n", cacheType)

	err = c.Set(ctx, "counter", "10", 10*time.Minute)
	if err != nil {
		log.Fatalf("Error setting counter: %v", err)
	}

	newVal, err := c.Increment(ctx, "counter", 5)
	if err != nil {
		log.Fatalf("Error incrementing counter: %v", err)
	}
	fmt.Printf("Counter incremented to: %d\n", newVal)

	newVal, err = c.Decrement(ctx, "counter", 3)
	if err != nil {
		log.Fatalf("Error decrementing counter: %v", err)
	}
	fmt.Printf("Counter decremented to: %d\n", newVal)

	fmt.Printf("\n--- Batch Operations (%s) ---\n", cacheType)

	values := map[string]interface{}{
		"user:1:name": "John Doe",
		"user:1:age":  30,
		"user:1:city": "New York",
	}
	err = c.MSet(ctx, values, 15*time.Minute)
	if err != nil {
		log.Fatalf("Error setting multiple values: %v", err)
	}

	results, err := c.MGet(ctx, "user:1:name", "user:1:age", "user:1:city", "user:1:notfound")
	if err != nil {
		log.Fatalf("Error getting multiple values: %v", err)
	}
	fmt.Println("Retrieved values:")
	for i, key := range []string{"user:1:name", "user:1:age", "user:1:city", "user:1:notfound"} {
		fmt.Printf("  %s: %v\n", key, results[i])
	}

	fmt.Printf("\n--- Key Search (%s) ---\n", cacheType)

	fmt.Printf("\n--- Deletion (%s) ---\n", cacheType)

	err = c.Delete(ctx, "greeting")
	if err != nil {
		log.Fatalf("Error deleting key: %v", err)
	}

	exists, err = c.Exists(ctx, "greeting")
	if err != nil {
		log.Fatalf("Error checking existence: %v", err)
	}
	fmt.Printf("Does 'greeting' exist after deletion? %v\n", exists)

	fmt.Printf("\n--- Flush Cache (%s) ---\n", cacheType)

	err = c.Flush(ctx)
	if err != nil {
		log.Fatalf("Error flushing cache: %v", err)
	}
}
