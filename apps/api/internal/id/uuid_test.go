package id

import (
	"testing"

	"github.com/google/uuid"
)

func TestNewReturnsUUIDv7(t *testing.T) {
	value, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if got := uuid.UUID(value).Version(); got != 7 {
		t.Fatalf("UUID version = %d, want 7", got)
	}
}

func TestUUIDDatabaseRoundTrip(t *testing.T) {
	original := MustNew()
	databaseValue, err := original.Value()
	if err != nil {
		t.Fatalf("Value() error = %v", err)
	}

	var restored UUID
	if err := restored.Scan(databaseValue); err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
	if restored != original {
		t.Fatalf("restored UUID = %s, want %s", restored, original)
	}
	if restored.String() != original.String() {
		t.Fatalf("restored string = %q, want %q", restored.String(), original.String())
	}
}
