package dataset

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type ImportPreviewStore struct {
	client *redis.Client
	ttl    time.Duration
}

func NewImportPreviewStore(client *redis.Client, ttl time.Duration) ImportPreviewStore {
	return ImportPreviewStore{client: client, ttl: ttl}
}

func (store ImportPreviewStore) Save(
	ctx context.Context,
	preview ImportPreview,
) (string, error) {
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", fmt.Errorf("generate import token: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(tokenBytes)
	payload, err := json.Marshal(preview)
	if err != nil {
		return "", fmt.Errorf("encode import preview: %w", err)
	}
	if err := store.client.Set(ctx, store.key(token), payload, store.ttl).Err(); err != nil {
		return "", fmt.Errorf("store import preview: %w", err)
	}
	return token, nil
}

func (store ImportPreviewStore) Consume(ctx context.Context, token string) (ImportPreview, error) {
	payload, err := store.client.GetDel(ctx, store.key(token)).Bytes()
	if err != nil {
		return ImportPreview{}, err
	}
	var preview ImportPreview
	if err := json.Unmarshal(payload, &preview); err != nil {
		return ImportPreview{}, fmt.Errorf("decode import preview: %w", err)
	}
	return preview, nil
}

func (store ImportPreviewStore) key(token string) string {
	hash := sha256.Sum256([]byte(token))
	return "quickeval:case-import:" + hex.EncodeToString(hash[:])
}
