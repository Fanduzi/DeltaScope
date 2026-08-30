// Package main verifies the DeltaScope MCP bootstrap behavior.
// input: CLI args, stub MCP server builders, and captured stdout/stderr buffers
// output: regression coverage for meta-invocation fast paths and connections-path startup wiring
// pos: command-layer tests for the MCP stdio entrypoint
// note: if this file changes, update this header and module README.md.
package main

import (
	"bytes"
	"context"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Fanduzi/DeltaScope/internal/infrastructure/logger"
	"github.com/Fanduzi/DeltaScope/internal/infrastructure/runtimeconfig"
	mcpapi "github.com/Fanduzi/DeltaScope/internal/interfaces/mcp"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

const runAsMCPServer = "_DELTASCOPE_MCP_RUN_AS_SERVER"
const runAsMCPPanicTest = "_DELTASCOPE_MCP_PANIC_TEST"

func TestMain(m *testing.M) {
	if os.Getenv(runAsMCPPanicTest) != "" {
		os.Unsetenv(runAsMCPPanicTest)
		newMCPServer = func(config mcpapi.Config) *sdkmcp.Server {
			panic("test construction panic")
		}
		runMCPServer = func(server *sdkmcp.Server) error { return nil }
		os.Exit(run([]string{}, os.Stdout, os.Stderr))
	}
	if os.Getenv(runAsMCPServer) != "" {
		os.Unsetenv(runAsMCPServer)
		os.Exit(run([]string{}, os.Stdout, os.Stderr))
	}
	os.Exit(m.Run())
}

func TestRunVersionPrintsVersionWithoutBuildingServer(t *testing.T) {
	previousVersion := Version
	previousNewServer := newMCPServer
	previousRunServer := runMCPServer
	Version = "v9.9.9"
	t.Cleanup(func() {
		Version = previousVersion
		newMCPServer = previousNewServer
		runMCPServer = previousRunServer
	})

	newMCPServer = func(config mcpapi.Config) *sdkmcp.Server {
		t.Fatal("expected -version to avoid server construction")
		return nil
	}
	runMCPServer = func(server *sdkmcp.Server) error {
		t.Fatal("expected -version to avoid server startup")
		return nil
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := run([]string{"-version"}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected zero exit code, got %d", exitCode)
	}
	if got := stdout.String(); got != "v9.9.9\n" {
		t.Fatalf("unexpected stdout: %q", got)
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got %q", stderr.String())
	}
}

func TestRunPositionalMetaArgumentsMatchDashedFormsWithoutStartingServer(t *testing.T) {
	previousVersion := Version
	previousNewServer := newMCPServer
	previousRunServer := runMCPServer
	Version = "v9.9.9"
	t.Cleanup(func() {
		Version = previousVersion
		newMCPServer = previousNewServer
		runMCPServer = previousRunServer
	})

	server := sdkmcp.NewServer(&sdkmcp.Implementation{Name: "test", Version: "v0.0.1"}, nil)
	newServerCalled := false
	runServerCalled := false
	newMCPServer = func(config mcpapi.Config) *sdkmcp.Server {
		newServerCalled = true
		return server
	}
	runMCPServer = func(server *sdkmcp.Server) error {
		runServerCalled = true
		return nil
	}

	for _, tc := range []struct {
		name   string
		dashed string
	}{
		{name: "version", dashed: "-version"},
		{name: "help", dashed: "-help"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			newServerCalled = false
			runServerCalled = false
			var dashedStdout, dashedStderr bytes.Buffer
			dashedExitCode := run([]string{tc.dashed}, &dashedStdout, &dashedStderr)

			newServerCalled = false
			runServerCalled = false
			var positionalStdout, positionalStderr bytes.Buffer
			positionalExitCode := run([]string{tc.name}, &positionalStdout, &positionalStderr)

			if positionalExitCode != dashedExitCode {
				t.Fatalf("expected positional %q exit %d, got %d", tc.name, dashedExitCode, positionalExitCode)
			}
			if positionalStdout.String() != dashedStdout.String() {
				t.Fatalf("expected positional %q stdout %q, got %q", tc.name, dashedStdout.String(), positionalStdout.String())
			}
			if positionalStderr.String() != dashedStderr.String() {
				t.Fatalf("expected positional %q stderr %q, got %q", tc.name, dashedStderr.String(), positionalStderr.String())
			}
			if newServerCalled {
				t.Fatalf("expected positional %q to avoid server construction", tc.name)
			}
			if runServerCalled {
				t.Fatalf("expected positional %q to avoid server startup", tc.name)
			}
		})
	}
}

func TestRunPassesConnectionsPathToServerConfig(t *testing.T) {
	previousNewServer := newMCPServer
	previousRunServer := runMCPServer
	t.Cleanup(func() {
		newMCPServer = previousNewServer
		runMCPServer = previousRunServer
	})

	var gotConfig mcpapi.Config
	server := sdkmcp.NewServer(&sdkmcp.Implementation{Name: "test", Version: "v0.0.1"}, nil)
	newMCPServer = func(config mcpapi.Config) *sdkmcp.Server {
		gotConfig = config
		return server
	}
	runMCPServer = func(gotServer *sdkmcp.Server) error {
		if gotServer != server {
			t.Fatalf("unexpected server pointer: %p", gotServer)
		}
		return nil
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := run([]string{"-connections-path", "/tmp/custom-connections.yaml"}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected zero exit code, got %d", exitCode)
	}
	if gotConfig.ConnectionsPath != "/tmp/custom-connections.yaml" {
		t.Fatalf("unexpected connections path: %q", gotConfig.ConnectionsPath)
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got %q", stderr.String())
	}
}

func TestRunServesMCPOverRealStdio(t *testing.T) {
	ctx := context.Background()
	cmd := createMCPServerCommand(t)

	client := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "test-client", Version: "v0.0.1"}, nil)
	session, err := client.Connect(ctx, &sdkmcp.CommandTransport{Command: cmd}, nil)
	if err != nil {
		t.Fatalf("connect stdio mcp server: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })
	if info := session.InitializeResult().ServerInfo; info == nil || info.Name != "deltascope-mcp" {
		t.Fatalf("unexpected server info: %#v", info)
	}

	result, err := session.CallTool(ctx, &sdkmcp.CallToolParams{
		Name:      "audit_sql",
		Arguments: map[string]any{"sql": "delete from users"},
	})
	if err != nil {
		t.Fatalf("call audit_sql over stdio: %v", err)
	}
	if result == nil || result.IsError {
		t.Fatalf("expected successful tool result, got %#v", result)
	}
	if len(result.Content) == 0 {
		t.Fatalf("expected non-empty tool content, got %#v", result)
	}

	body, ok := result.StructuredContent.(map[string]any)
	if !ok {
		t.Fatalf("expected structured result body, got %T", result.StructuredContent)
	}
	if body["verdict"] != "reject" {
		t.Fatalf("unexpected verdict: %#v", body["verdict"])
	}
	contextValue, ok := body["context"].(map[string]any)
	if !ok {
		t.Fatalf("expected audit context object, got %#v", body["context"])
	}
	if contextValue["mode"] != "offline" {
		t.Fatalf("unexpected audit mode: %#v", contextValue["mode"])
	}
}

func createMCPServerCommand(t *testing.T) *exec.Cmd {
	t.Helper()

	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("resolve test executable: %v", err)
	}

	cmd := exec.Command(exe)
	cmd.Env = append(os.Environ(), runAsMCPServer+"=1")
	return cmd
}

func TestRunDefaultLoggingFlagsSucceed(t *testing.T) {
	previousNewServer := newMCPServer
	previousRunServer := runMCPServer
	t.Cleanup(func() {
		newMCPServer = previousNewServer
		runMCPServer = previousRunServer
	})

	server := sdkmcp.NewServer(&sdkmcp.Implementation{Name: "test", Version: "v0.0.1"}, nil)
	newMCPServer = func(config mcpapi.Config) *sdkmcp.Server { return server }
	runMCPServer = func(gotServer *sdkmcp.Server) error { return nil }

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := run([]string{}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit 0, got %d: %s", exitCode, stderr.String())
	}
}

func TestRunLoggingStdoutRejectedForMCP(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := run([]string{"--log-output", "stdout"}, &stdout, &stderr)
	if exitCode != 2 {
		t.Fatalf("expected exit 2, got %d", exitCode)
	}
	if !bytes.Contains(stderr.Bytes(), []byte("stdout is forbidden")) {
		t.Fatalf("expected stdout-forbidden error, got: %s", stderr.String())
	}
}

func TestRunLoggingInvalidFormat(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := run([]string{"--log-format", "xml"}, &stdout, &stderr)
	if exitCode != 2 {
		t.Fatalf("expected exit 2, got %d", exitCode)
	}
	if !bytes.Contains(stderr.Bytes(), []byte("invalid format")) {
		t.Fatalf("expected invalid-format error, got: %s", stderr.String())
	}
}

func TestRunLoggingInvalidLevel(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := run([]string{"--log-level", "trace"}, &stdout, &stderr)
	if exitCode != 2 {
		t.Fatalf("expected exit 2, got %d", exitCode)
	}
	if !bytes.Contains(stderr.Bytes(), []byte("invalid level")) {
		t.Fatalf("expected invalid-level error, got: %s", stderr.String())
	}
}

func TestRunLoggingDebugLevelSucceeds(t *testing.T) {
	previousNewServer := newMCPServer
	previousRunServer := runMCPServer
	t.Cleanup(func() {
		newMCPServer = previousNewServer
		runMCPServer = previousRunServer
	})

	server := sdkmcp.NewServer(&sdkmcp.Implementation{Name: "test", Version: "v0.0.1"}, nil)
	newMCPServer = func(config mcpapi.Config) *sdkmcp.Server { return server }
	runMCPServer = func(gotServer *sdkmcp.Server) error { return nil }

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := run([]string{"--log-level", "debug"}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit 0, got %d: %s", exitCode, stderr.String())
	}
}

func TestRunRotateWithFileSucceeds(t *testing.T) {
	previousNewServer := newMCPServer
	previousRunServer := runMCPServer
	t.Cleanup(func() {
		newMCPServer = previousNewServer
		runMCPServer = previousRunServer
	})

	server := sdkmcp.NewServer(&sdkmcp.Implementation{Name: "test", Version: "v0.0.1"}, nil)
	newMCPServer = func(config mcpapi.Config) *sdkmcp.Server { return server }
	runMCPServer = func(gotServer *sdkmcp.Server) error { return nil }

	tmpFile := filepath.Join(t.TempDir(), "mcp.log")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := run([]string{"--log-output", "file", "--log-file", tmpFile, "--log-rotate"}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit 0, got %d: %s", exitCode, stderr.String())
	}
}

func TestRunRotateWithStderrReturnsExit2(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := run([]string{"--log-output", "stderr", "--log-rotate"}, &stdout, &stderr)
	if exitCode != 2 {
		t.Fatalf("expected exit 2, got %d", exitCode)
	}
	if !bytes.Contains(stderr.Bytes(), []byte("rotation requires output=file")) {
		t.Fatalf("expected rotation error, got: %s", stderr.String())
	}
}

func TestRunRotateInvalidMaxSizeReturnsExit2(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "mcp.log")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := run([]string{"--log-output", "file", "--log-file", tmpFile, "--log-rotate", "--log-max-size-mb", "-1"}, &stdout, &stderr)
	if exitCode != 2 {
		t.Fatalf("expected exit 2, got %d", exitCode)
	}
	if !bytes.Contains(stderr.Bytes(), []byte("log-max-size-mb must be > 0")) {
		t.Fatalf("expected log-max-size-mb error, got: %s", stderr.String())
	}
}

func TestRunRotateZeroMaxSizeReturnsExit2(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "mcp.log")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := run([]string{"--log-output", "file", "--log-file", tmpFile, "--log-rotate", "--log-max-size-mb", "0"}, &stdout, &stderr)
	if exitCode != 2 {
		t.Fatalf("expected exit 2, got %d", exitCode)
	}
	if !bytes.Contains(stderr.Bytes(), []byte("log-max-size-mb must be > 0")) {
		t.Fatalf("expected log-max-size-mb error, got: %s", stderr.String())
	}
}

func TestRunMCPRotateWithStdoutStillForbidden(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := run([]string{"--log-output", "stdout", "--log-rotate"}, &stdout, &stderr)
	if exitCode != 2 {
		t.Fatalf("expected exit 2, got %d", exitCode)
	}
	if !bytes.Contains(stderr.Bytes(), []byte("stdout is forbidden")) {
		t.Fatalf("expected stdout forbidden error, got: %s", stderr.String())
	}
}

func TestMCPLoggerConfigFromRuntimeAndFlags(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "runtime.yaml")
	if err := os.WriteFile(path, []byte(`
logging:
  level: debug
  format: text
  output: file
  file: `+filepath.Join(dir, "test.log")+`
`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	runtimeCfg, err := runtimeconfig.Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	var level, format, output, file string
	var rotate bool
	var maxMB, maxBackups, maxAge int
	var compress bool
	fs.StringVar(&level, "log-level", "info", "")
	fs.StringVar(&format, "log-format", "json", "")
	fs.StringVar(&output, "log-output", "stderr", "")
	fs.StringVar(&file, "log-file", "", "")
	fs.BoolVar(&rotate, "log-rotate", false, "")
	fs.IntVar(&maxMB, "log-max-size-mb", 100, "")
	fs.IntVar(&maxBackups, "log-max-backups", 3, "")
	fs.IntVar(&maxAge, "log-max-age-days", 30, "")
	fs.BoolVar(&compress, "log-compress", true, "")
	_ = fs.Parse([]string{})

	cfg := mcpLoggerConfigFromRuntimeAndFlags(runtimeCfg, loggingFlagSet{
		level: level, format: format, output: output, file: file,
		rotate: rotate, maxMB: maxMB, maxBackups: maxBackups,
		maxAge: maxAge, compress: compress,
	}, fs)

	if cfg.Level != "debug" {
		t.Fatalf("expected debug level from runtime, got %q", cfg.Level)
	}
	if cfg.Format != "text" {
		t.Fatalf("expected text format from runtime, got %q", cfg.Format)
	}
	if cfg.Output != "file" {
		t.Fatalf("expected file output from runtime, got %q", cfg.Output)
	}

	l, err := logger.NewLogger(cfg, "mcp")
	if err != nil {
		t.Fatalf("expected valid logger: %v", err)
	}
	if l == nil {
		t.Fatal("expected non-nil logger")
	}
}

func TestMCPLoggerFlagsOverrideRuntimeConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "runtime.yaml")
	if err := os.WriteFile(path, []byte(`
logging:
  level: debug
  format: text
`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	runtimeCfg, err := runtimeconfig.Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	var level, format, output, file string
	var rotate bool
	var maxMB, maxBackups, maxAge int
	var compress bool
	fs.StringVar(&level, "log-level", "info", "")
	fs.StringVar(&format, "log-format", "json", "")
	fs.StringVar(&output, "log-output", "stderr", "")
	fs.StringVar(&file, "log-file", "", "")
	fs.BoolVar(&rotate, "log-rotate", false, "")
	fs.IntVar(&maxMB, "log-max-size-mb", 100, "")
	fs.IntVar(&maxBackups, "log-max-backups", 3, "")
	fs.IntVar(&maxAge, "log-max-age-days", 30, "")
	fs.BoolVar(&compress, "log-compress", true, "")
	_ = fs.Parse([]string{"--log-level", "warn"})

	cfg := mcpLoggerConfigFromRuntimeAndFlags(runtimeCfg, loggingFlagSet{
		level: level, format: format, output: output, file: file,
		rotate: rotate, maxMB: maxMB, maxBackups: maxBackups,
		maxAge: maxAge, compress: compress,
	}, fs)

	if cfg.Level != "warn" {
		t.Fatalf("expected warn level from explicit flag overriding runtime debug, got %q", cfg.Level)
	}
	if cfg.Format != "text" {
		t.Fatalf("expected text format from runtime (no explicit flag), got %q", cfg.Format)
	}
}

func TestMCPRuntimeConfigStdoutRejectedByLogger(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "runtime.yaml")
	if err := os.WriteFile(path, []byte(`
logging:
  output: stdout
`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	runtimeCfg, err := runtimeconfig.Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	var level, format, output, file string
	var rotate bool
	var maxMB, maxBackups, maxAge int
	var compress bool
	fs.StringVar(&level, "log-level", "info", "")
	fs.StringVar(&format, "log-format", "json", "")
	fs.StringVar(&output, "log-output", "stderr", "")
	fs.StringVar(&file, "log-file", "", "")
	fs.BoolVar(&rotate, "log-rotate", false, "")
	fs.IntVar(&maxMB, "log-max-size-mb", 100, "")
	fs.IntVar(&maxBackups, "log-max-backups", 3, "")
	fs.IntVar(&maxAge, "log-max-age-days", 30, "")
	fs.BoolVar(&compress, "log-compress", true, "")
	_ = fs.Parse([]string{})

	cfg := mcpLoggerConfigFromRuntimeAndFlags(runtimeCfg, loggingFlagSet{
		level: level, format: format, output: output, file: file,
		rotate: rotate, maxMB: maxMB, maxBackups: maxBackups,
		maxAge: maxAge, compress: compress,
	}, fs)

	_, err = logger.NewLogger(cfg, "mcp")
	if err == nil {
		t.Fatal("expected error for stdout in MCP logger")
	}
	if !strings.Contains(err.Error(), "stdout is forbidden") {
		t.Fatalf("expected stdout forbidden error, got: %v", err)
	}
}

func TestMCPRuntimeConfigStdoutCanBeOverriddenByFlag(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "runtime.yaml")
	if err := os.WriteFile(path, []byte(`
logging:
  output: stdout
`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	runtimeCfg, err := runtimeconfig.Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	var level, format, output, file string
	var rotate bool
	var maxMB, maxBackups, maxAge int
	var compress bool
	fs.StringVar(&level, "log-level", "info", "")
	fs.StringVar(&format, "log-format", "json", "")
	fs.StringVar(&output, "log-output", "stderr", "")
	fs.StringVar(&file, "log-file", "", "")
	fs.BoolVar(&rotate, "log-rotate", false, "")
	fs.IntVar(&maxMB, "log-max-size-mb", 100, "")
	fs.IntVar(&maxBackups, "log-max-backups", 3, "")
	fs.IntVar(&maxAge, "log-max-age-days", 30, "")
	fs.BoolVar(&compress, "log-compress", true, "")
	_ = fs.Parse([]string{"--log-output", "stderr"})

	cfg := mcpLoggerConfigFromRuntimeAndFlags(runtimeCfg, loggingFlagSet{
		level: level, format: format, output: output, file: file,
		rotate: rotate, maxMB: maxMB, maxBackups: maxBackups,
		maxAge: maxAge, compress: compress,
	}, fs)

	if cfg.Output != "stderr" {
		t.Fatalf("expected stderr from explicit flag overriding runtime stdout, got %q", cfg.Output)
	}

	l, err := logger.NewLogger(cfg, "mcp")
	if err != nil {
		t.Fatalf("expected valid logger: %v", err)
	}
	if l == nil {
		t.Fatal("expected non-nil logger")
	}
}

func TestMCPRuntimeConfigRotateEnabledMapsToLoggerConfig(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "mcp.log")
	path := filepath.Join(dir, "runtime.yaml")
	if err := os.WriteFile(path, []byte(`
logging:
  output: file
  file: `+logFile+`
  rotate:
    enabled: true
    max_size_mb: 10
    max_backups: 2
    max_age_days: 7
    compress: false
`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	runtimeCfg, err := runtimeconfig.Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	var level, format, output, file string
	var rotate bool
	var maxMB, maxBackups, maxAge int
	var compress bool
	fs.StringVar(&level, "log-level", "info", "")
	fs.StringVar(&format, "log-format", "json", "")
	fs.StringVar(&output, "log-output", "stderr", "")
	fs.StringVar(&file, "log-file", "", "")
	fs.BoolVar(&rotate, "log-rotate", false, "")
	fs.IntVar(&maxMB, "log-max-size-mb", 100, "")
	fs.IntVar(&maxBackups, "log-max-backups", 3, "")
	fs.IntVar(&maxAge, "log-max-age-days", 30, "")
	fs.BoolVar(&compress, "log-compress", true, "")
	_ = fs.Parse([]string{})

	cfg := mcpLoggerConfigFromRuntimeAndFlags(runtimeCfg, loggingFlagSet{
		level: level, format: format, output: output, file: file,
		rotate: rotate, maxMB: maxMB, maxBackups: maxBackups,
		maxAge: maxAge, compress: compress,
	}, fs)

	if cfg.Rotate == nil || !cfg.Rotate.Enabled {
		t.Fatal("expected rotate enabled from runtime config")
	}
	if cfg.Rotate.MaxSizeMB != 10 {
		t.Fatalf("expected MaxSizeMB=10, got %d", cfg.Rotate.MaxSizeMB)
	}
	if cfg.Rotate.MaxBackups != 2 {
		t.Fatalf("expected MaxBackups=2, got %d", cfg.Rotate.MaxBackups)
	}
	if cfg.Rotate.MaxAgeDays != 7 {
		t.Fatalf("expected MaxAgeDays=7, got %d", cfg.Rotate.MaxAgeDays)
	}
	if cfg.Rotate.Compress == nil || *cfg.Rotate.Compress != false {
		t.Fatalf("expected Compress=false, got %v", cfg.Rotate.Compress)
	}
}

func TestMCPRuntimeConfigRejectsInvalidYAML(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "bad.yaml")
	if err := os.WriteFile(tmpFile, []byte("logging: ["), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	_, err := runtimeconfig.Load(tmpFile)
	if err == nil {
		t.Fatal("expected error for malformed YAML")
	}
	if !strings.Contains(err.Error(), "parse runtime config") {
		t.Fatalf("expected parse error, got: %v", err)
	}
}

func TestMCPRunWithRuntimeConfigFileSucceeds(t *testing.T) {
	previousNewServer := newMCPServer
	previousRunServer := runMCPServer
	t.Cleanup(func() {
		newMCPServer = previousNewServer
		runMCPServer = previousRunServer
	})

	server := sdkmcp.NewServer(&sdkmcp.Implementation{Name: "test", Version: "v0.0.1"}, nil)
	newMCPServer = func(config mcpapi.Config) *sdkmcp.Server { return server }
	runMCPServer = func(gotServer *sdkmcp.Server) error { return nil }

	dir := t.TempDir()
	path := filepath.Join(dir, "runtime.yaml")
	if err := os.WriteFile(path, []byte("logging:\n  level: debug\n"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := run([]string{"--runtime-config", path}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit 0, got %d: %s", exitCode, stderr.String())
	}
}

func TestParseMCPMetadataConnectTimeoutFromRuntime(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "runtime.yaml")
	if err := os.WriteFile(path, []byte("metadata:\n  connect_timeout: 7s\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	runtimeCfg, err := runtimeconfig.Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	d, err := parseMCPMetadataConnectTimeout(runtimeCfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d != 7*time.Second {
		t.Fatalf("expected 7s, got %v", d)
	}
}

func TestParseMCPMetadataConnectTimeoutRejectsInvalid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "runtime.yaml")
	if err := os.WriteFile(path, []byte("metadata:\n  connect_timeout: not-a-duration\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	runtimeCfg, err := runtimeconfig.Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	_, err = parseMCPMetadataConnectTimeout(runtimeCfg)
	if err == nil {
		t.Fatal("expected error for invalid metadata.connect_timeout")
	}
	if !strings.Contains(err.Error(), "metadata.connect_timeout") {
		t.Fatalf("expected metadata.connect_timeout error, got: %v", err)
	}
}
