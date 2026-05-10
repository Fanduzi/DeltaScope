// Package logger provides a unified structured logging foundation for DeltaScope surfaces.
// input: logging configuration (level, format, output path) and surface identifier
// output: configured *slog.Logger instances and bridges to *log.Logger for legacy consumers
// pos: infrastructure adapter for structured logging across server and MCP surfaces
// note: if this file changes, update this header and module README.md.
package logger

import (
	"context"
	"fmt"
	"io"
	"log"
	"log/slog"
	"os"
	"path/filepath"

	"gopkg.in/natefinch/lumberjack.v2"
)

// Config configures structured logger creation.
type Config struct {
	Level    string        // debug, info, warn, error — default "info"
	Format   string        // json, text — default "json"
	Output   string        // stderr, stdout, file — default "stderr"
	FilePath string        // required when Output is "file"
	Rotate   *RotateConfig // optional rotation; nil means plain append
}

// RotateConfig configures log file rotation via lumberjack.
// All fields use sensible defaults when zero.
type RotateConfig struct {
	Enabled    bool
	MaxSizeMB  int   // default 100
	MaxBackups int   // default 3
	MaxAgeDays int   // default 30
	Compress   *bool // nil defaults to true; set explicitly to false to disable
}

// NewLogger creates a configured *slog.Logger for the given surface.
// Surface must be "server" or "mcp". When surface is "mcp" and Output is "stdout",
// NewLogger returns an error to prevent stdout log pollution that would break MCP stdio.
func NewLogger(cfg Config, surface string) (*slog.Logger, error) {
	if surface != "server" && surface != "mcp" {
		return nil, fmt.Errorf("logger: unsupported surface %q", surface)
	}

	level, err := parseLevel(cfg.Level)
	if err != nil {
		return nil, err
	}

	handlerOpts := &slog.HandlerOptions{Level: level}

	writer, err := writerForOutput(cfg, surface)
	if err != nil {
		return nil, err
	}

	handler, err := handlerForFormat(cfg.Format, writer, handlerOpts)
	if err != nil {
		return nil, err
	}

	return slog.New(handler), nil
}

// NewStdLogger bridges a *slog.Logger to a *log.Logger for legacy consumers.
// If sl is nil, it returns a logger that writes to os.Stderr at info level using a JSON handler.
func NewStdLogger(sl *slog.Logger) *log.Logger {
	if sl == nil {
		h := slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})
		return slog.NewLogLogger(h, slog.LevelInfo)
	}
	// Extract the handler from the slog.Logger and create a log.Logger bridge.
	// slog doesn't expose the handler directly, so we use slog.Default as fallback
	// and write through the slog.Logger itself via a writer adapter.
	return slog.NewLogLogger(newHandlerFromLogger(sl), slog.LevelInfo)
}

// handlerFromLogger wraps a *slog.Logger to implement slog.Handler by forwarding
// calls through the logger. This is needed because slog.NewLogLogger requires
// an slog.Handler, and we want to preserve any configured handler.
type handlerFromLogger struct {
	logger *slog.Logger
}

func newHandlerFromLogger(sl *slog.Logger) slog.Handler {
	return &handlerFromLogger{logger: sl}
}

func (h *handlerFromLogger) Enabled(ctx context.Context, r slog.Level) bool {
	return h.logger.Enabled(ctx, r)
}
func (h *handlerFromLogger) Handle(ctx context.Context, r slog.Record) error {
	return h.logger.Handler().Handle(ctx, r)
}
func (h *handlerFromLogger) WithAttrs(attrs []slog.Attr) slog.Handler {
	return h.logger.Handler().WithAttrs(attrs)
}
func (h *handlerFromLogger) WithGroup(name string) slog.Handler {
	return h.logger.Handler().WithGroup(name)
}

func parseLevel(s string) (slog.Level, error) {
	switch s {
	case "", "info":
		return slog.LevelInfo, nil
	case "debug":
		return slog.LevelDebug, nil
	case "warn":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("logger: invalid level %q (valid: debug, info, warn, error)", s)
	}
}

func handlerForFormat(format string, w io.Writer, opts *slog.HandlerOptions) (slog.Handler, error) {
	switch format {
	case "", "json":
		return slog.NewJSONHandler(w, opts), nil
	case "text":
		return slog.NewTextHandler(w, opts), nil
	default:
		return nil, fmt.Errorf("logger: invalid format %q (valid: json, text)", format)
	}
}

func writerForOutput(cfg Config, surface string) (io.Writer, error) {
	switch cfg.Output {
	case "", "stderr":
		if cfg.Rotate != nil && cfg.Rotate.Enabled {
			return nil, fmt.Errorf("logger: rotation requires output=file, got output=%q", cfg.Output)
		}
		return os.Stderr, nil
	case "stdout":
		if surface == "mcp" {
			return nil, fmt.Errorf("logger: output=stdout is forbidden for surface %q to protect MCP stdio protocol", surface)
		}
		if cfg.Rotate != nil && cfg.Rotate.Enabled {
			return nil, fmt.Errorf("logger: rotation requires output=file, got output=%q", cfg.Output)
		}
		return os.Stdout, nil
	case "file":
		if cfg.FilePath == "" {
			return nil, fmt.Errorf("logger: file_path is required when output=file")
		}
		dir := filepath.Dir(cfg.FilePath)
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return nil, fmt.Errorf("logger: create log directory %q: %w", dir, err)
		}
		if cfg.Rotate != nil && cfg.Rotate.Enabled {
			if err := validateRotateConfig(cfg.Rotate); err != nil {
				return nil, err
			}
			return &lumberjack.Logger{
				Filename:   cfg.FilePath,
				MaxSize:    rotateField(cfg.Rotate.MaxSizeMB, 100),
				MaxBackups: rotateField(cfg.Rotate.MaxBackups, 3),
				MaxAge:     rotateField(cfg.Rotate.MaxAgeDays, 30),
				Compress:   rotateBoolField(cfg.Rotate.Compress, true),
			}, nil
		}
		f, err := os.OpenFile(cfg.FilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
		if err != nil {
			return nil, fmt.Errorf("logger: open log file %q: %w", cfg.FilePath, err)
		}
		return f, nil
	default:
		return nil, fmt.Errorf("logger: invalid output %q (valid: stderr, stdout, file)", cfg.Output)
	}
}

func validateRotateConfig(rc *RotateConfig) error {
	if rc.MaxSizeMB < 0 {
		return fmt.Errorf("logger: max_size_mb must be positive, got %d", rc.MaxSizeMB)
	}
	if rc.MaxBackups < 0 {
		return fmt.Errorf("logger: max_backups must be non-negative, got %d", rc.MaxBackups)
	}
	if rc.MaxAgeDays < 0 {
		return fmt.Errorf("logger: max_age_days must be non-negative, got %d", rc.MaxAgeDays)
	}
	return nil
}

func rotateField(val, def int) int {
	if val <= 0 {
		return def
	}
	return val
}

func rotateBoolField(val *bool, def bool) bool {
	if val == nil {
		return def
	}
	return *val
}
