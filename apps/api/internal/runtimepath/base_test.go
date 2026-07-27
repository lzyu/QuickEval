package runtimepath

import (
	"path/filepath"
	"testing"
)

func TestBaseDirUsesEnvironmentOverride(t *testing.T) {
	t.Setenv(baseDirEnv, ".")

	got, err := BaseDir()
	if err != nil {
		t.Fatalf("BaseDir() error = %v", err)
	}

	want, err := filepath.Abs(".")
	if err != nil {
		t.Fatalf("filepath.Abs() error = %v", err)
	}
	if got != want {
		t.Fatalf("BaseDir() = %q, want %q", got, want)
	}
}
