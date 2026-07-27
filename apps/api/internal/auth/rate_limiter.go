package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

type LoginRateLimiter struct {
	client      *redis.Client
	maxAttempts int64
	window      time.Duration
}

func NewLoginRateLimiter(
	client *redis.Client,
	maxAttempts int,
	window time.Duration,
) LoginRateLimiter {
	return LoginRateLimiter{
		client:      client,
		maxAttempts: int64(maxAttempts),
		window:      window,
	}
}

func (limiter LoginRateLimiter) Blocked(
	ctx context.Context,
	clientIP, username string,
) (bool, time.Duration, error) {
	key := limiter.key(clientIP, username)
	count, err := limiter.client.Get(ctx, key).Int64()
	if err == redis.Nil {
		return false, 0, nil
	}
	if err != nil {
		return false, 0, err
	}
	if count < limiter.maxAttempts {
		return false, 0, nil
	}
	ttl, err := limiter.client.TTL(ctx, key).Result()
	return true, ttl, err
}

func (limiter LoginRateLimiter) RecordFailure(
	ctx context.Context,
	clientIP, username string,
) error {
	key := limiter.key(clientIP, username)
	count, err := limiter.client.Incr(ctx, key).Result()
	if err != nil {
		return fmt.Errorf("increment login failures: %w", err)
	}
	if count == 1 {
		if err := limiter.client.Expire(ctx, key, limiter.window).Err(); err != nil {
			return fmt.Errorf("expire login failures: %w", err)
		}
	}
	return nil
}

func (limiter LoginRateLimiter) Reset(
	ctx context.Context,
	clientIP, username string,
) error {
	return limiter.client.Del(ctx, limiter.key(clientIP, username)).Err()
}

func (limiter LoginRateLimiter) key(clientIP, username string) string {
	value := strings.ToLower(strings.TrimSpace(clientIP + "|" + username))
	hash := sha256.Sum256([]byte(value))
	return "quickeval:login-failures:" + hex.EncodeToString(hash[:])
}
