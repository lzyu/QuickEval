package badcase

import (
	"context"
	"errors"
	"time"

	"github.com/lzyu/QuickEval/apps/api/internal/id"
	"github.com/redis/go-redis/v9"
)

var ErrRequestInProgress = errors.New("badcase request is in progress")

type IdempotencyStore struct {
	client *redis.Client
	ttl    time.Duration
}

func NewIdempotencyStore(client *redis.Client) IdempotencyStore {
	return IdempotencyStore{client: client, ttl: 24 * time.Hour}
}

func (store IdempotencyStore) Reserve(
	ctx context.Context,
	actorID id.UUID,
	key string,
) (*id.UUID, error) {
	cacheKey := "quickeval:badcase:mark:" + actorID.String() + ":" + key
	ok, err := store.client.SetNX(ctx, cacheKey, "pending", store.ttl).Result()
	if err != nil {
		return nil, err
	}
	if ok {
		return nil, nil
	}
	value, err := store.client.Get(ctx, cacheKey).Result()
	if err != nil {
		return nil, err
	}
	if value == "pending" {
		return nil, ErrRequestInProgress
	}
	itemID, err := id.Parse(value)
	if err != nil {
		return nil, err
	}
	return &itemID, nil
}

func (store IdempotencyStore) Commit(
	ctx context.Context,
	actorID id.UUID,
	key string,
	itemID id.UUID,
) error {
	cacheKey := "quickeval:badcase:mark:" + actorID.String() + ":" + key
	return store.client.Set(ctx, cacheKey, itemID.String(), store.ttl).Err()
}

func (store IdempotencyStore) Release(ctx context.Context, actorID id.UUID, key string) {
	cacheKey := "quickeval:badcase:mark:" + actorID.String() + ":" + key
	store.client.Del(ctx, cacheKey)
}
