package auth

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/lzyu/QuickEval/apps/api/internal/id"
	"github.com/redis/go-redis/v9"
)

func integrationRedis(t *testing.T) *redis.Client {
	t.Helper()
	address := os.Getenv("QUICKEVAL_REDIS_TEST_ADDR")
	if address == "" {
		t.Skip("QUICKEVAL_REDIS_TEST_ADDR is not set")
	}
	client := redis.NewClient(&redis.Options{
		Addr:     address,
		Password: os.Getenv("QUICKEVAL_REDIS_TEST_PASSWORD"),
		DB:       15,
	})
	t.Cleanup(func() {
		_ = client.FlushDB(context.Background()).Err()
		_ = client.Close()
	})
	if err := client.Ping(context.Background()).Err(); err != nil {
		t.Fatalf("ping redis: %v", err)
	}
	return client
}

func TestSessionStoreLifecycle(t *testing.T) {
	client := integrationRedis(t)
	store := NewSessionStore(client, "test-secret-with-at-least-32-characters", time.Minute)
	userID := id.MustNew()

	token, created, err := store.Create(context.Background(), userID)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	if token == "" || token == created.CSRFToken {
		t.Fatal("session and CSRF tokens must be non-empty and independent")
	}
	loaded, err := store.Load(context.Background(), token)
	if err != nil || loaded.UserID != userID.String() {
		t.Fatalf("load session: %#v, %v", loaded, err)
	}
	if err := store.RevokeUser(context.Background(), userID); err != nil {
		t.Fatalf("revoke user: %v", err)
	}
	if _, err := store.Load(context.Background(), token); err != redis.Nil {
		t.Fatalf("expected revoked session, got %v", err)
	}
}

func TestLoginRateLimiterBlocksAndResets(t *testing.T) {
	client := integrationRedis(t)
	limiter := NewLoginRateLimiter(client, 2, time.Minute)
	ctx := context.Background()

	for range 2 {
		if err := limiter.RecordFailure(ctx, "127.0.0.1", "member"); err != nil {
			t.Fatal(err)
		}
	}
	blocked, retryAfter, err := limiter.Blocked(ctx, "127.0.0.1", "member")
	if err != nil || !blocked || retryAfter <= 0 {
		t.Fatalf("expected blocked login, blocked=%v retry=%v err=%v", blocked, retryAfter, err)
	}
	if err := limiter.Reset(ctx, "127.0.0.1", "member"); err != nil {
		t.Fatal(err)
	}
	blocked, _, err = limiter.Blocked(ctx, "127.0.0.1", "member")
	if err != nil || blocked {
		t.Fatalf("expected reset limiter, blocked=%v err=%v", blocked, err)
	}
}
