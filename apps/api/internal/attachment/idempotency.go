package attachment

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/lzyu/QuickEval/apps/api/internal/id"
	"github.com/redis/go-redis/v9"
)

var ErrRequestInProgress = errors.New("attachment request is in progress")

type UploadReceipt struct {
	Items            []Public `json:"items"`
	OwnerLockVersion uint32   `json:"owner_lock_version"`
}

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
	kind string,
	ownerID id.UUID,
	key string,
) (*UploadReceipt, error) {
	cacheKey := store.key(actorID, kind, ownerID, key)
	ok, err := store.client.SetNX(ctx, cacheKey, "pending", store.ttl).Result()
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
	if string(value) == "pending" {
		return nil, ErrRequestInProgress
	}
	var receipt UploadReceipt
	if err := json.Unmarshal(value, &receipt); err != nil {
		return nil, err
	}
	return &receipt, nil
}

func (store IdempotencyStore) Commit(
	ctx context.Context,
	actorID id.UUID,
	kind string,
	ownerID id.UUID,
	key string,
	receipt UploadReceipt,
) error {
	value, err := json.Marshal(receipt)
	if err != nil {
		return err
	}
	return store.client.Set(ctx, store.key(actorID, kind, ownerID, key), value, store.ttl).Err()
}

func (store IdempotencyStore) Release(
	ctx context.Context,
	actorID id.UUID,
	kind string,
	ownerID id.UUID,
	key string,
) {
	store.client.Del(ctx, store.key(actorID, kind, ownerID, key))
}

func (store IdempotencyStore) key(
	actorID id.UUID,
	kind string,
	ownerID id.UUID,
	key string,
) string {
	return "quickeval:attachment:upload:" + actorID.String() + ":" +
		kind + ":" + ownerID.String() + ":" + key
}
