package ratelimit

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type Limiter interface {
	Allow(ctx context.Context, key string, limit int64, window time.Duration) (bool, error)
	Ping(ctx context.Context) error
	Close() error
}

type RedisLimiter struct {
	client *redis.Client
}

func NewRedisLimiter(ctx context.Context, redisURL string) (*RedisLimiter, error) {
	options, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("parse redis url: %w", err)
	}
	client := redis.NewClient(options)
	if err := client.Ping(ctx).Err(); err != nil {
		client.Close()
		return nil, fmt.Errorf("ping redis: %w", err)
	}
	return &RedisLimiter{client: client}, nil
}

func (l *RedisLimiter) Allow(ctx context.Context, key string, limit int64, window time.Duration) (bool, error) {
	if l == nil || l.client == nil {
		return false, fmt.Errorf("rate limiter unavailable")
	}
	count, err := l.client.Incr(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("increment rate limit key: %w", err)
	}
	if count == 1 {
		if err := l.client.Expire(ctx, key, window).Err(); err != nil {
			return false, fmt.Errorf("expire rate limit key: %w", err)
		}
	}
	return count <= limit, nil
}

func (l *RedisLimiter) Close() error {
	if l == nil || l.client == nil {
		return nil
	}
	return l.client.Close()
}

func (l *RedisLimiter) Ping(ctx context.Context) error {
	if l == nil || l.client == nil {
		return fmt.Errorf("rate limiter unavailable")
	}
	return l.client.Ping(ctx).Err()
}
