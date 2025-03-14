package database

import (
	"context"
	"encoding/json"
	"time"

	"github.com/go-redis/redis/v8"
)

// RedisClient wraps the Redis client
type RedisClient struct {
	client *redis.Client
	ctx    context.Context
}

// InitRedis initializes the Redis connection
func InitRedis(redisURI string) (*RedisClient, error) {
	opts, err := redis.ParseURL(redisURI)
	if err != nil {
		return nil, err
	}

	client := redis.NewClient(opts)
	ctx := context.Background()

	// Test connection
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, err
	}

	return &RedisClient{
		client: client,
		ctx:    ctx,
	}, nil
}

// Close closes the Redis connection
func (r *RedisClient) Close() error {
	return r.client.Close()
}

// Set stores a key-value pair in Redis with expiration
func (r *RedisClient) Set(key string, value interface{}, expiration time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return r.client.Set(r.ctx, key, data, expiration).Err()
}

// Get retrieves a value from Redis
func (r *RedisClient) Get(key string, dest interface{}) error {
	data, err := r.client.Get(r.ctx, key).Result()
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(data), dest)
}

// Delete removes a key from Redis
func (r *RedisClient) Delete(key string) error {
	return r.client.Del(r.ctx, key).Err()
}

// GetCached gets a value from cache or calls the provider function to generate it
func (r *RedisClient) GetCached(key string, dest interface{}, ttl time.Duration, provider func() (interface{}, error)) error {
	// Try to get from cache
	err := r.Get(key, dest)
	if err == nil {
		return nil
	}

	// If not in cache or error, call provider
	data, err := provider()
	if err != nil {
		return err
	}

	// Store in cache and return
	if err := r.Set(key, data, ttl); err != nil {
		return err
	}

	// Marshal into destination
	dataBytes, err := json.Marshal(data)
	if err != nil {
		return err
	}

	return json.Unmarshal(dataBytes, dest)
}
