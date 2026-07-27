package logging

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

func New(baseDir, configuredFile, configuredLevel string) (*slog.Logger, io.Closer, error) {
	logPath := filepath.Join(baseDir, filepath.Clean(configuredFile))
	if err := os.MkdirAll(filepath.Dir(logPath), 0o750); err != nil {
		return nil, nil, fmt.Errorf("create log directory: %w", err)
	}

	file, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o640)
	if err != nil {
		return nil, nil, fmt.Errorf("open log file: %w", err)
	}

	level := slog.LevelInfo
	switch strings.ToLower(configuredLevel) {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}

	handler := slog.NewJSONHandler(io.MultiWriter(os.Stdout, file), &slog.HandlerOptions{
		Level: level,
	})
	return slog.New(handler), file, nil
}
