package id

import (
	"database/sql/driver"
	"fmt"

	"github.com/google/uuid"
)

type UUID [16]byte

func New() (UUID, error) {
	value, err := uuid.NewV7()
	if err != nil {
		return UUID{}, fmt.Errorf("generate UUIDv7: %w", err)
	}
	return UUID(value), nil
}

func MustNew() UUID {
	value, err := New()
	if err != nil {
		panic(err)
	}
	return value
}

func Parse(value string) (UUID, error) {
	parsed, err := uuid.Parse(value)
	if err != nil {
		return UUID{}, fmt.Errorf("parse UUID: %w", err)
	}
	return UUID(parsed), nil
}

func (value UUID) String() string {
	return uuid.UUID(value).String()
}

func (value UUID) Value() (driver.Value, error) {
	bytes := make([]byte, 16)
	copy(bytes, value[:])
	return bytes, nil
}

func (value *UUID) Scan(src any) error {
	if src == nil {
		return fmt.Errorf("scan UUID: nil value")
	}

	var bytes []byte
	switch typed := src.(type) {
	case []byte:
		bytes = typed
	case string:
		bytes = []byte(typed)
	default:
		return fmt.Errorf("scan UUID: unsupported type %T", src)
	}

	if len(bytes) != 16 {
		return fmt.Errorf("scan UUID: got %d bytes, want 16", len(bytes))
	}
	copy(value[:], bytes)
	return nil
}
