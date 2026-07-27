package auth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/lzyu/QuickEval/apps/api/internal/id"
	"github.com/redis/go-redis/v9"
)

type Session struct {
	UserID    string    `json:"user_id"`
	CSRFToken string    `json:"csrf_token"`
	CreatedAt time.Time `json:"created_at"`
}

type SessionStore struct {
	client *redis.Client
	secret []byte
	ttl    time.Duration
}

func NewSessionStore(client *redis.Client, secret string, ttl time.Duration) SessionStore {
	return SessionStore{
		client: client,
		secret: []byte(secret),
		ttl:    ttl,
	}
}

func (store SessionStore) Create(ctx context.Context, userID id.UUID) (string, Session, error) {
	token, err := randomToken(32)
	if err != nil {
		return "", Session{}, err
	}
	csrfToken, err := randomToken(32)
	if err != nil {
		return "", Session{}, err
	}

	session := Session{
		UserID:    userID.String(),
		CSRFToken: csrfToken,
		CreatedAt: time.Now().UTC(),
	}
	payload, err := json.Marshal(session)
	if err != nil {
		return "", Session{}, fmt.Errorf("encode session: %w", err)
	}

	key := store.sessionKey(token)
	userSessionsKey := store.userSessionsKey(userID)
	pipeline := store.client.TxPipeline()
	pipeline.Set(ctx, key, payload, store.ttl)
	pipeline.SAdd(ctx, userSessionsKey, key)
	pipeline.Expire(ctx, userSessionsKey, store.ttl)
	if _, err := pipeline.Exec(ctx); err != nil {
		return "", Session{}, fmt.Errorf("store session: %w", err)
	}

	return token, session, nil
}

func (store SessionStore) Load(ctx context.Context, token string) (Session, error) {
	payload, err := store.client.Get(ctx, store.sessionKey(token)).Bytes()
	if err != nil {
		return Session{}, err
	}

	var session Session
	if err := json.Unmarshal(payload, &session); err != nil {
		return Session{}, fmt.Errorf("decode session: %w", err)
	}
	return session, nil
}

func (store SessionStore) Revoke(ctx context.Context, token string) error {
	session, err := store.Load(ctx, token)
	if err != nil && err != redis.Nil {
		return err
	}

	sessionKey := store.sessionKey(token)
	if err == redis.Nil {
		return store.client.Del(ctx, sessionKey).Err()
	}

	userID, parseErr := id.Parse(session.UserID)
	if parseErr != nil {
		return store.client.Del(ctx, sessionKey).Err()
	}
	pipeline := store.client.TxPipeline()
	pipeline.Del(ctx, sessionKey)
	pipeline.SRem(ctx, store.userSessionsKey(userID), sessionKey)
	_, execErr := pipeline.Exec(ctx)
	return execErr
}

func (store SessionStore) RevokeUser(ctx context.Context, userID id.UUID) error {
	userSessionsKey := store.userSessionsKey(userID)
	keys, err := store.client.SMembers(ctx, userSessionsKey).Result()
	if err != nil {
		return err
	}
	pipeline := store.client.TxPipeline()
	if len(keys) > 0 {
		pipeline.Del(ctx, keys...)
	}
	pipeline.Del(ctx, userSessionsKey)
	_, err = pipeline.Exec(ctx)
	return err
}

func (store SessionStore) sessionKey(token string) string {
	mac := hmac.New(sha256.New, store.secret)
	_, _ = mac.Write([]byte(token))
	return "quickeval:session:" + hex.EncodeToString(mac.Sum(nil))
}

func (store SessionStore) userSessionsKey(userID id.UUID) string {
	return "quickeval:user-sessions:" + userID.String()
}

func randomToken(byteCount int) (string, error) {
	buffer := make([]byte, byteCount)
	if _, err := rand.Read(buffer); err != nil {
		return "", fmt.Errorf("generate random token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}
