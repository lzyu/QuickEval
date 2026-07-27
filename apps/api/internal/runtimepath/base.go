package runtimepath

import (
	"fmt"
	"os"
	"path/filepath"
)

const baseDirEnv = "QUICKEVAL_BASE_DIR"

// BaseDir resolves all runtime paths from the executable directory. The
// environment override exists for go run, tests, and local development.
func BaseDir() (string, error) {
	if override := os.Getenv(baseDirEnv); override != "" {
		return filepath.Abs(override)
	}

	executable, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolve executable path: %w", err)
	}

	resolved, err := filepath.EvalSymlinks(executable)
	if err != nil {
		return "", fmt.Errorf("resolve executable symlinks: %w", err)
	}

	return filepath.Dir(resolved), nil
}
