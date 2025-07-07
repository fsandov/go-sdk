package cache

import (
	"context"
	"errors"
	"time"
)

var (
	ErrKeyNotFound    = errors.New("key not found")
	ErrInvalidType    = errors.New("invalid type")
	ErrInvalidKey     = errors.New("invalid key")
	ErrInvalidContext = errors.New("invalid context")
)

type Cache interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
	Exists(ctx context.Context, key string) (bool, error)
	Expire(ctx context.Context, key string, ttl time.Duration) (bool, error)
	TTL(ctx context.Context, key string) (time.Duration, error)
	Flush(ctx context.Context) error
	Close() error

	Increment(ctx context.Context, key string, value int64) (int64, error)
	Decrement(ctx context.Context, key string, value int64) (int64, error)

	MGet(ctx context.Context, keys ...string) ([]interface{}, error)
	MSet(ctx context.Context, values map[string]interface{}, ttl time.Duration) error

	ZAdd(ctx context.Context, key string, score float64, member string) error
	ZRem(ctx context.Context, key string, member string) error
	ZRange(ctx context.Context, key string, start, stop int64) ([]string, error)
}
