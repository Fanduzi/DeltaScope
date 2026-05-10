package logger

import (
	"bufio"
	"bytes"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewLoggerDefaultConfig(t *testing.T) {
	t.Parallel()
	l, err := NewLogger(Config{}, "server")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if l == nil {
		t.Fatal("expected non-nil logger")
	}
}

func TestNewLoggerLevels(t *testing.T) {
	t.Parallel()
	for _, level := range []string{"debug", "info", "warn", "error", ""} {
		l, err := NewLogger(Config{Level: level}, "server")
		if err != nil {
			t.Errorf("level %q: unexpected error: %v", level, err)
		}
		if l == nil {
			t.Errorf("level %q: expected non-nil logger", level)
		}
	}
}

func TestNewLoggerInvalidLevel(t *testing.T) {
	t.Parallel()
	_, err := NewLogger(Config{Level: "trace"}, "server")
	if err == nil {
		t.Fatal("expected error for invalid level")
	}
	if !strings.Contains(err.Error(), "invalid level") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNewLoggerFormats(t *testing.T) {
	t.Parallel()
	for _, format := range []string{"json", "text", ""} {
		l, err := NewLogger(Config{Format: format}, "server")
		if err != nil {
			t.Errorf("format %q: unexpected error: %v", format, err)
		}
		if l == nil {
			t.Errorf("format %q: expected non-nil logger", format)
		}
	}
}

func TestNewLoggerInvalidFormat(t *testing.T) {
	t.Parallel()
	_, err := NewLogger(Config{Format: "xml"}, "server")
	if err == nil {
		t.Fatal("expected error for invalid format")
	}
	if !strings.Contains(err.Error(), "invalid format") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNewLoggerOutputs(t *testing.T) {
	t.Parallel()
	for _, output := range []string{"stderr", "stdout", ""} {
		l, err := NewLogger(Config{Output: output}, "server")
		if err != nil {
			t.Errorf("output %q: unexpected error: %v", output, err)
		}
		if l == nil {
			t.Errorf("output %q: expected non-nil logger", output)
		}
	}
}

func TestNewLoggerInvalidOutput(t *testing.T) {
	t.Parallel()
	_, err := NewLogger(Config{Output: "syslog"}, "server")
	if err == nil {
		t.Fatal("expected error for invalid output")
	}
	if !strings.Contains(err.Error(), "invalid output") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNewLoggerMCPStdoutRejected(t *testing.T) {
	t.Parallel()
	_, err := NewLogger(Config{Output: "stdout"}, "mcp")
	if err == nil {
		t.Fatal("expected error for mcp + stdout")
	}
	if !strings.Contains(err.Error(), "stdout is forbidden") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNewLoggerMCPStderrAllowed(t *testing.T) {
	t.Parallel()
	l, err := NewLogger(Config{Output: "stderr"}, "mcp")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if l == nil {
		t.Fatal("expected non-nil logger")
	}
}

func TestNewLoggerFileOutputEmptyPathRejected(t *testing.T) {
	t.Parallel()
	_, err := NewLogger(Config{Output: "file", FilePath: ""}, "server")
	if err == nil {
		t.Fatal("expected error for file output without path")
	}
	if !strings.Contains(err.Error(), "file_path is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNewLoggerFileOutputCreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "sub", "dir", "test.log")

	l, err := NewLogger(Config{Output: "file", FilePath: logPath}, "server")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if l == nil {
		t.Fatal("expected non-nil logger")
	}

	l.Info("test message")

	// Verify the file was created in the nested directory.
	fi, err := os.Stat(logPath)
	if err != nil {
		t.Fatalf("log file not created: %v", err)
	}
	if fi.Size() == 0 {
		t.Fatal("log file is empty")
	}
}

func TestNewLoggerFileOutputWritesJSON(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	l, err := NewLogger(Config{Output: "file", FilePath: logPath, Format: "json"}, "server")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	l.Info("hello", "key", "value")

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read log file: %v", err)
	}

	var entry map[string]any
	if err := json.Unmarshal(data, &entry); err != nil {
		t.Fatalf("log entry is not valid JSON: %v\nraw: %s", err, string(data))
	}
	if entry["msg"] != "hello" {
		t.Fatalf("expected msg=hello, got %v", entry["msg"])
	}
}

func TestNewLoggerInvalidSurface(t *testing.T) {
	t.Parallel()
	_, err := NewLogger(Config{}, "cli")
	if err == nil {
		t.Fatal("expected error for unsupported surface")
	}
	if !strings.Contains(err.Error(), "unsupported surface") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNewStdLoggerWithNil(t *testing.T) {
	stdLog := NewStdLogger(nil)
	if stdLog == nil {
		t.Fatal("expected non-nil *log.Logger")
	}
}

func TestNewStdLoggerBridge(t *testing.T) {
	var buf bytes.Buffer
	sl := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}))
	stdLog := NewStdLogger(sl)

	stdLog.Println("bridge test")

	output := buf.String()
	if !strings.Contains(output, "bridge test") {
		t.Fatalf("expected bridge test in output, got: %s", output)
	}

	// Verify it's valid JSON.
	var entry map[string]any
	scanner := bufio.NewScanner(&buf)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "bridge test") {
			if err := json.Unmarshal([]byte(line), &entry); err != nil {
				t.Fatalf("bridge output is not valid JSON: %v\nraw: %s", err, line)
			}
			return
		}
	}
	t.Fatal("bridge test message not found in output")
}

func TestNewLoggerTextFormat(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	l, err := NewLogger(Config{Output: "file", FilePath: logPath, Format: "text"}, "server")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	l.Info("text test")

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read log file: %v", err)
	}

	// Text format should NOT start with '{'.
	line := string(bytes.TrimSpace(data))
	if strings.HasPrefix(line, "{") {
		t.Fatal("text format produced JSON output")
	}
	if !strings.Contains(line, "text test") {
		t.Fatalf("expected 'text test' in output, got: %s", line)
	}
}

func TestNewLoggerDefaultFormatIsJSON(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	l, err := NewLogger(Config{Output: "file", FilePath: logPath}, "server")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	l.Info("default format test")

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read log file: %v", err)
	}

	var entry map[string]any
	if err := json.Unmarshal(data, &entry); err != nil {
		t.Fatalf("default format is not JSON: %v\nraw: %s", err, string(data))
	}
	if entry["msg"] != "default format test" {
		t.Fatalf("expected msg='default format test', got %v", entry["msg"])
	}
}

func TestNewLoggerRotateNilUsesPlainAppend(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	l, err := NewLogger(Config{Output: "file", FilePath: logPath, Rotate: nil}, "server")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if l == nil {
		t.Fatal("expected non-nil logger")
	}

	l.Info("plain append test")

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read log file: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("log file is empty")
	}
}

func TestNewLoggerRotateDisabledUsesPlainAppend(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	l, err := NewLogger(Config{
		Output:   "file",
		FilePath: logPath,
		Rotate:   &RotateConfig{Enabled: false},
	}, "server")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if l == nil {
		t.Fatal("expected non-nil logger")
	}

	l.Info("disabled rotate test")

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read log file: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("log file is empty")
	}
}

func TestNewLoggerRotateEnabledWithFile(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	compress := true
	l, err := NewLogger(Config{
		Output:   "file",
		FilePath: logPath,
		Rotate:   &RotateConfig{Enabled: true, Compress: &compress},
	}, "server")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if l == nil {
		t.Fatal("expected non-nil logger")
	}

	l.Info("rotated log test")

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read log file: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("log file is empty")
	}
}

func TestNewLoggerRotateEnabledCreatesParentDir(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "deep", "nested", "test.log")

	l, err := NewLogger(Config{
		Output:   "file",
		FilePath: logPath,
		Rotate:   &RotateConfig{Enabled: true},
	}, "server")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if l == nil {
		t.Fatal("expected non-nil logger")
	}

	l.Info("nested dir test")

	fi, err := os.Stat(logPath)
	if err != nil {
		t.Fatalf("log file not created: %v", err)
	}
	if fi.Size() == 0 {
		t.Fatal("log file is empty")
	}
}

func TestNewLoggerRotateEnabledWithStderr(t *testing.T) {
	_, err := NewLogger(Config{
		Output: "stderr",
		Rotate: &RotateConfig{Enabled: true},
	}, "server")
	if err == nil {
		t.Fatal("expected error for rotate with stderr")
	}
	if !strings.Contains(err.Error(), "rotation requires output=file") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNewLoggerRotateEnabledWithStdout(t *testing.T) {
	_, err := NewLogger(Config{
		Output: "stdout",
		Rotate: &RotateConfig{Enabled: true},
	}, "server")
	if err == nil {
		t.Fatal("expected error for rotate with stdout")
	}
	if !strings.Contains(err.Error(), "rotation requires output=file") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNewLoggerRotateEnabledMCPStdout(t *testing.T) {
	_, err := NewLogger(Config{
		Output: "stdout",
		Rotate: &RotateConfig{Enabled: true},
	}, "mcp")
	if err == nil {
		t.Fatal("expected error for mcp + stdout")
	}
}

func TestNewLoggerRotateNegativeMaxSize(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	_, err := NewLogger(Config{
		Output:   "file",
		FilePath: logPath,
		Rotate:   &RotateConfig{Enabled: true, MaxSizeMB: -1},
	}, "server")
	if err == nil {
		t.Fatal("expected error for negative max_size_mb")
	}
	if !strings.Contains(err.Error(), "max_size_mb must be positive") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNewLoggerRotateNegativeMaxBackups(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	_, err := NewLogger(Config{
		Output:   "file",
		FilePath: logPath,
		Rotate:   &RotateConfig{Enabled: true, MaxBackups: -1},
	}, "server")
	if err == nil {
		t.Fatal("expected error for negative max_backups")
	}
	if !strings.Contains(err.Error(), "max_backups must be non-negative") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNewLoggerRotateNegativeMaxAge(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	_, err := NewLogger(Config{
		Output:   "file",
		FilePath: logPath,
		Rotate:   &RotateConfig{Enabled: true, MaxAgeDays: -1},
	}, "server")
	if err == nil {
		t.Fatal("expected error for negative max_age_days")
	}
	if !strings.Contains(err.Error(), "max_age_days must be non-negative") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNewLoggerRotateDefaultValuesApplied(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	l, err := NewLogger(Config{
		Output:   "file",
		FilePath: logPath,
		Rotate:   &RotateConfig{Enabled: true},
	}, "server")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if l == nil {
		t.Fatal("expected non-nil logger")
	}

	l.Info("defaults test")

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read log file: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("log file is empty")
	}
}

func TestNewLoggerRotateCompressFalse(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	compress := false
	l, err := NewLogger(Config{
		Output:   "file",
		FilePath: logPath,
		Rotate:   &RotateConfig{Enabled: true, Compress: &compress},
	}, "server")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if l == nil {
		t.Fatal("expected non-nil logger")
	}

	l.Info("compress false test")

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read log file: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("log file is empty")
	}
}

func TestNewLoggerRotateCompressNilDefaultsTrue(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	l, err := NewLogger(Config{
		Output:   "file",
		FilePath: logPath,
		Rotate:   &RotateConfig{Enabled: true, Compress: nil},
	}, "server")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if l == nil {
		t.Fatal("expected non-nil logger")
	}
}

func TestNewLoggerFileOutputPermissionRestricted(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	l, err := NewLogger(Config{Output: "file", FilePath: logPath}, "server")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	l.Info("permission test")

	fi, err := os.Stat(logPath)
	if err != nil {
		t.Fatalf("stat log file: %v", err)
	}
	perm := fi.Mode().Perm()
	if perm&0o077 != 0 {
		t.Fatalf("log file has group/other bits set: %04o", perm)
	}
}

func TestNewLoggerRotateFileOutputPermissionRestricted(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	l, err := NewLogger(Config{
		Output:   "file",
		FilePath: logPath,
		Rotate:   &RotateConfig{Enabled: true},
	}, "server")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	l.Info("permission test")

	// lumberjack creates the file internally; verify no group/other bits.
	fi, err := os.Stat(logPath)
	if err != nil {
		t.Fatalf("stat log file: %v", err)
	}
	perm := fi.Mode().Perm()
	if perm&0o077 != 0 {
		t.Fatalf("rotated log file has group/other bits set: %04o", perm)
	}
}

func TestNewLoggerRotateZeroMaxSizeUsesDefault(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	l, err := NewLogger(Config{
		Output:   "file",
		FilePath: logPath,
		Rotate:   &RotateConfig{Enabled: true, MaxSizeMB: 0},
	}, "server")
	if err != nil {
		t.Fatalf("internal zero MaxSizeMB should use default, got error: %v", err)
	}
	if l == nil {
		t.Fatal("expected non-nil logger")
	}

	l.Info("zero max size uses default")

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read log file: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("log file is empty")
	}
}
