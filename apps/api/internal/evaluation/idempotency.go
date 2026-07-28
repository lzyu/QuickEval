package evaluation

import (
	"context"
	"errors"
	"time"

	"github.com/lzyu/QuickEval/apps/api/internal/id"
	"github.com/redis/go-redis/v9"
)

var ErrRequestInProgress = errors.New("idempotent request is in progress")

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
	cacheKey := store.key(actorID, key)
	acquired, err := store.client.SetNX(ctx, cacheKey, "pending", store.ttl).Result()
	if err != nil {
		return nil, err
	}
	if acquired {
		return nil, nil
	}
	value, err := store.client.Get(ctx, cacheKey).Result()
	if err != nil {
		return nil, err
	}
	if value == "pending" {
		return nil, ErrRequestInProgress
	}
	runID, err := id.Parse(value)
	if err != nil {
		return nil, err
	}
	return &runID, nil
}

func (store IdempotencyStore) Commit(
	ctx context.Context,
	actorID id.UUID,
	key string,
	runID id.UUID,
) error {
	return store.client.Set(ctx, store.key(actorID, key), runID.String(), store.ttl).Err()
}

func (store IdempotencyStore) Release(ctx context.Context, actorID id.UUID, key string) {
	store.client.Del(ctx, store.key(actorID, key))
}

func (store IdempotencyStore) key(actorID id.UUID, key string) string {
	return "quickeval:evaluation-run:create:" + actorID.String() + ":" + key
}
