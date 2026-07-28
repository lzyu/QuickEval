package badcase

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/lzyu/QuickEval/apps/api/internal/id"
	"github.com/redis/go-redis/v9"
)

var ErrRequestInProgress = errors.New("badcase request is in progress")
var ErrIdempotencyKeyReused = errors.New("badcase idempotency key reused")

type IdempotencyStore struct {
	client *redis.Client
	ttl    time.Duration
}

type operationReceipt struct {
	State  string `json:"state"`
	Digest string `json:"digest"`
	ItemID string `json:"item_id,omitempty"`
}

func (store IdempotencyStore) ReserveOperation(
	ctx context.Context,
	actorID id.UUID,
	scope, key, digest string,
) (*id.UUID, error) {
	cacheKey := "quickeval:badcase:operation:" + actorID.String() + ":" + scope + ":" + key
	pending, err := json.Marshal(operationReceipt{State: "pending", Digest: digest})
	if err != nil {
		return nil, err
	}
	ok, err := store.client.SetNX(ctx, cacheKey, pending, store.ttl).Result()
	if err != nil {
		return nil, err
	}
	if ok {
		return nil, nil
	}
	value, err := store.client.Get(ctx, cacheKey).Bytes()
	if err != nil {
		return nil, err
	}
	var receipt operationReceipt
	if err := json.Unmarshal(value, &receipt); err != nil {
		return nil, err
	}
	if receipt.Digest != digest {
		return nil, ErrIdempotencyKeyReused
	}
	if receipt.State == "pending" {
		return nil, ErrRequestInProgress
	}
	itemID, err := id.Parse(receipt.ItemID)
	if err != nil {
		return nil, err
	}
	return &itemID, nil
}

func (store IdempotencyStore) CommitOperation(
	ctx context.Context,
	actorID id.UUID,
	scope, key, digest string,
	itemID id.UUID,
) error {
	cacheKey := "quickeval:badcase:operation:" + actorID.String() + ":" + scope + ":" + key
	value, err := json.Marshal(operationReceipt{
		State: "committed", Digest: digest, ItemID: itemID.String(),
	})
	if err != nil {
		return err
	}
	return store.client.Set(ctx, cacheKey, value, store.ttl).Err()
}

func (store IdempotencyStore) ReleaseOperation(
	ctx context.Context,
	actorID id.UUID,
	scope, key string,
) {
	cacheKey := "quickeval:badcase:operation:" + actorID.String() + ":" + scope + ":" + key
	store.client.Del(ctx, cacheKey)
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
