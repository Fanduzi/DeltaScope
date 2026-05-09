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
