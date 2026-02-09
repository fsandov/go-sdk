package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisConfig struct {
	Enabled     bool
	Addr        string
	Password    string
	DB          int
	PoolSize    int
	DialTimeout time.Duration
}

func (c *RedisConfig) applyDefaults() {
	if c.PoolSize == 0 {
		c.PoolSize = 10
	}
	if c.DialTimeout == 0 {
		c.DialTimeout = 5 * time.Second
	}
}

func (c *RedisConfig) Validate() error {
	if !c.Enabled {
		return nil
	}
	if c.Addr == "" {
		return errors.New("redis: missing Addr")
	}
	return nil
}

type redisCache struct {
	client *redis.Client
}

type RedisOption func(*redis.Options)

func WithPoolSize(size int) RedisOption {
	return func(o *redis.Options) {
		o.PoolSize = size
	}
}

func WithReadTimeout(timeout time.Duration) RedisOption {
	return func(o *redis.Options) {
		o.ReadTimeout = timeout
	}
}

func WithWriteTimeout(timeout time.Duration) RedisOption {
	return func(o *redis.Options) {
		o.WriteTimeout = timeout
	}
}

func NewRedisCacheFromConfig(cfg RedisConfig, opts ...RedisOption) (Cache, error) {
	cfg.applyDefaults()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if !cfg.Enabled {
		return nil, errors.New("redis: Redis cache is not enabled")
	}

	var client *redis.Client

	if strings.HasPrefix(cfg.Addr, "redis://") || strings.HasPrefix(cfg.Addr, "rediss://") {
		o, err := redis.ParseURL(cfg.Addr)
		if err != nil {
			return nil, fmt.Errorf("redis: invalid URL in Addr: %w", err)
		}
		if cfg.PoolSize > 0 {
			o.PoolSize = cfg.PoolSize
		}
		if cfg.DialTimeout > 0 {
			o.DialTimeout = cfg.DialTimeout
		}
		client = redis.NewClient(o)
	} else {
		options := &redis.Options{
			Addr:        cfg.Addr,
			Password:    cfg.Password,
			DB:          cfg.DB,
			PoolSize:    cfg.PoolSize,
			DialTimeout: cfg.DialTimeout,
		}
		for _, opt := range opts {
			opt(options)
		}
		client = redis.NewClient(options)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("error al conectar a Redis: %w", err)
	}

	return &redisCache{client: client}, nil
}

func (r *redisCache) Get(ctx context.Context, key string) (string, error) {
	if ctx == nil {
		return "", ErrInvalidContext
	}

	if key == "" {
		return "", ErrInvalidKey
	}

	val, err := r.client.Get(ctx, key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return "", ErrKeyNotFound
		}
		return "", fmt.Errorf("redis get error: %w", err)
	}
	return val, nil
}

func (r *redisCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	return r.client.Set(ctx, key, value, ttl).Err()
}

func (r *redisCache) Delete(ctx context.Context, key string) error {
	result := r.client.Del(ctx, key)
	if err := result.Err(); err != nil {
		return err
	}
	if result.Val() == 0 {
		return ErrKeyNotFound
	}
	return nil
}

func (r *redisCache) Exists(ctx context.Context, key string) (bool, error) {
	exists, err := r.client.Exists(ctx, key).Result()
	if err != nil {
		return false, err
	}
	return exists > 0, nil
}

func (r *redisCache) Expire(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	result := r.client.Expire(ctx, key, ttl)
	if err := result.Err(); err != nil {
		return false, err
	}
	return result.Val(), nil
}

func (r *redisCache) TTL(ctx context.Context, key string) (time.Duration, error) {
	ttl, err := r.client.TTL(ctx, key).Result()
	if err != nil {
		return 0, err
	}
	switch {
	case ttl == -2:
		return 0, ErrKeyNotFound
	case ttl == -1:
		return 0, nil
	default:
		return ttl, nil
	}
}

func (r *redisCache) Flush(ctx context.Context) error {
	return r.client.FlushDB(ctx).Err()
}

func (r *redisCache) Close() error {
	return r.client.Close()
}

func (r *redisCache) Increment(ctx context.Context, key string, value int64) (int64, error) {
	if ctx == nil {
		return 0, ErrInvalidContext
	}
	if key == "" {
		return 0, ErrInvalidKey
	}

	result, err := r.client.IncrBy(ctx, key, value).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to increment key %s: %w", key, err)
	}
	return result, nil
}

func (r *redisCache) Decrement(ctx context.Context, key string, value int64) (int64, error) {
	if ctx == nil {
		return 0, ErrInvalidContext
	}
	if key == "" {
		return 0, ErrInvalidKey
	}

	result, err := r.client.DecrBy(ctx, key, value).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to decrement key %s: %w", key, err)
	}
	return result, nil
}

func (r *redisCache) MGet(ctx context.Context, keys ...string) ([]interface{}, error) {
	if ctx == nil {
		return nil, ErrInvalidContext
	}

	if len(keys) == 0 {
		return make([]interface{}, 0), nil
	}

	result, err := r.client.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, err
	}

	for i := range result {
		if result[i] == nil {
			result[i] = ""
		}
	}

	return result, nil
}

func (r *redisCache) MSet(ctx context.Context, values map[string]interface{}, ttl time.Duration) error {
	if len(values) == 0 {
		return nil
	}

	pipe := r.client.Pipeline()
	for k, v := range values {
		jsonData, err := json.Marshal(v)
		if err != nil {
			return fmt.Errorf("failed to marshal value for key %s: %w", k, err)
		}
		pipe.Set(ctx, k, jsonData, ttl)
	}

	_, err := pipe.Exec(ctx)
	return err
}

func (r *redisCache) ZAdd(ctx context.Context, key string, score float64, member string) error {
	if key == "" {
		return errors.New("key cannot be empty")
	}

	z := redis.Z{
		Score:  score,
		Member: member,
	}

	_, err := r.client.ZAdd(ctx, key, z).Result()
	return err
}

func (r *redisCache) ZRem(ctx context.Context, key, member string) error {
	if key == "" {
		return errors.New("key cannot be empty")
	}

	_, err := r.client.ZRem(ctx, key, member).Result()
	return err
}

func (r *redisCache) ZRange(ctx context.Context, key string, start, stop int64) ([]string, error) {
	if key == "" {
		return nil, errors.New("key cannot be empty")
	}

	result, err := r.client.ZRange(ctx, key, start, stop).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get range from sorted set: %w", err)
	}

	return result, nil
}
