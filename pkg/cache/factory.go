package cache

import (
	"fmt"
	"os"

	"github.com/fsandov/go-sdk/pkg/env"
)

// NewFromEnvironment creates a cache instance based on the current environment.
// Returns MemoryCache in local, RedisCache in remote environments.
func NewFromEnvironment() (Cache, error) {
	if env.IsLocal() {
		return NewMemoryCache(), nil
	}
	addr := os.Getenv("REDIS_HOST")
	if addr == "" {
		return nil, fmt.Errorf("REDIS_HOST is required in non-local environments")
	}
	return NewRedisCacheFromConfig(RedisConfig{
		Enabled: true,
		Addr:    addr,
	})
}
