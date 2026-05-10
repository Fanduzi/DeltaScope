// Package main starts the DeltaScope MCP server.
// input: process flags for version printing plus stdio MCP transport startup
// output: a long-running MCP stdio server process over DeltaScope audit and rule capabilities
// pos: MCP service entrypoint above the internal MCP adapter
// note: if this file changes, update this header and module README.md.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"runtime"
	"time"

	"github.com/Fanduzi/DeltaScope/internal/infrastructure/logger"
	"github.com/Fanduzi/DeltaScope/internal/infrastructure/runtimeconfig"
	mcpapi "github.com/Fanduzi/DeltaScope/internal/interfaces/mcp"
	publicapi "github.com/Fanduzi/DeltaScope/pkg/deltascope"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// Version is the build version printed by the MCP service entrypoint.
var Version = publicapi.DefaultVersion

var newMCPServer = mcpapi.NewServer
var runMCPServer = func(server *sdkmcp.Server) error {
	return server.Run(context.Background(), &sdkmcp.StdioTransport{})
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout io.Writer, stderr io.Writer) int {
	defer func() {
		if r := recover(); r != nil {
			buf := make([]byte, 4096)
			n := runtime.Stack(buf, false)
			_, _ = fmt.Fprintf(stderr, "FATAL: MCP server panic: %v\nStack trace:\n%s", r, string(buf[:n]))
			os.Exit(4)
		}
	}()

	flags := flag.NewFlagSet("deltascope-mcp", flag.ContinueOnError)
	flags.SetOutput(stderr)

	showVersion := flags.Bool("version", false, "print the DeltaScope MCP build version")
	connectionsPath := flags.String("connections-path", "", "override the connection_ref config file path")
	logLevel := flags.String("log-level", "info", "log verbosity: debug, info, warn, error")
	logFormat := flags.String("log-format", "json", "log format: json, text")
	logOutput := flags.String("log-output", "stderr", "log destination: stderr, stdout, file")
	logFile := flags.String("log-file", "", "log file path (required when --log-output=file)")
	logRotate := flags.Bool("log-rotate", false, "enable log file rotation (requires --log-output=file)")
	logMaxSizeMB := flags.Int("log-max-size-mb", 100, "max log file size in MB before rotation")
	logMaxBackups := flags.Int("log-max-backups", 3, "max number of rotated log files to retain")
	logMaxAgeDays := flags.Int("log-max-age-days", 30, "max number of days to retain rotated log files")
	logCompress := flags.Bool("log-compress", true, "compress rotated log files")
	runtimeConfigPath := flags.String("runtime-config", "", "path to DeltaScope runtime config YAML")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *showVersion {
		_, _ = fmt.Fprintln(stdout, Version)
		return 0
	}

	runtimeCfg, err := runtimeconfig.Load(*runtimeConfigPath)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "load runtime config: %v\n", err)
		return 2
	}

	if err := validateRotateFlags(*logRotate, *logMaxSizeMB, *logMaxBackups, *logMaxAgeDays); err != nil {
		_, _ = fmt.Fprintf(stderr, "init logger: %v\n", err)
		return 2
	}

	loggingCfg := mcpLoggerConfigFromRuntimeAndFlags(runtimeCfg, loggingFlagSet{
		level: *logLevel, format: *logFormat, output: *logOutput, file: *logFile,
		rotate: *logRotate, maxMB: *logMaxSizeMB, maxBackups: *logMaxBackups,
		maxAge: *logMaxAgeDays, compress: *logCompress,
	}, flags)

	slogLogger, slogErr := logger.NewLogger(loggingCfg, "mcp")
	if slogErr != nil {
		_, _ = fmt.Fprintf(stderr, "init logger: %v\n", slogErr)
		return 2
	}

	metadataTimeout, metadataErr := parseMCPMetadataConnectTimeout(runtimeCfg)
	if metadataErr != nil {
		_, _ = fmt.Fprintf(stderr, "init metadata: %v\n", metadataErr)
		return 2
	}

	server := newMCPServer(mcpapi.Config{
		Version:                Version,
		ConnectionsPath:        *connectionsPath,
		Logger:                 slogLogger,
		MetadataConnectTimeout: metadataTimeout,
	})
	if err := runMCPServer(server); err != nil {
		_, _ = fmt.Fprintf(stderr, "serve mcp: %v\n", err)
		return 3
	}
	return 0
}

func mcpLoggerConfig(level, format, output, file string, rotate bool, maxMB, maxBackups, maxAge int, compress bool) logger.Config {
	cfg := logger.Config{
		Level:    level,
		Format:   format,
		Output:   output,
		FilePath: file,
	}
	if rotate {
		cfg.Rotate = &logger.RotateConfig{
			Enabled:    true,
			MaxSizeMB:  maxMB,
			MaxBackups: maxBackups,
			MaxAgeDays: maxAge,
			Compress:   &compress,
		}
	}
	return cfg
}

type loggingFlagSet struct {
	level      string
	format     string
	output     string
	file       string
	rotate     bool
	maxMB      int
	maxBackups int
	maxAge     int
	compress   bool
}

func mcpLoggerConfigFromRuntimeAndFlags(runtime runtimeconfig.Config, flags loggingFlagSet, fs *flag.FlagSet) logger.Config {
	cfg := logger.Config{
		Level:    resolveStringOverride(runtime.Logging.Level, flags.level, "info", fs, "log-level"),
		Format:   resolveStringOverride(runtime.Logging.Format, flags.format, "json", fs, "log-format"),
		Output:   resolveStringOverride(runtime.Logging.Output, flags.output, "stderr", fs, "log-output"),
		FilePath: resolveStringOverride(runtime.Logging.File, flags.file, "", fs, "log-file"),
	}

	if rotateEnabled := mcpResolveRotateEnabled(runtime, flags, fs); rotateEnabled {
		cfg.Rotate = &logger.RotateConfig{
			Enabled:    true,
			MaxSizeMB:  resolveIntOverride(runtime.Logging.Rotate.MaxSizeMB, flags.maxMB, 100, fs, "log-max-size-mb"),
			MaxBackups: resolveIntOverride(runtime.Logging.Rotate.MaxBackups, flags.maxBackups, 3, fs, "log-max-backups"),
			MaxAgeDays: resolveIntOverride(runtime.Logging.Rotate.MaxAgeDays, flags.maxAge, 30, fs, "log-max-age-days"),
			Compress:   resolveBoolPtrOverride(runtime.Logging.Rotate.Compress, flags.compress, true, fs, "log-compress"),
		}
	}

	return cfg
}

func mcpResolveRotateEnabled(runtime runtimeconfig.Config, flags loggingFlagSet, fs *flag.FlagSet) bool {
	if isExplicitFlag(fs, "log-rotate") {
		return flags.rotate
	}
	if runtime.Logging.Rotate.Enabled != nil {
		return *runtime.Logging.Rotate.Enabled
	}
	return false
}

func resolveStringOverride(runtimeVal, flagVal, defaultVal string, fs *flag.FlagSet, flagName string) string {
	if isExplicitFlag(fs, flagName) {
		return flagVal
	}
	if runtimeVal != "" {
		return runtimeVal
	}
	return defaultVal
}

func resolveIntOverride(runtimeVal *int, flagVal, defaultVal int, fs *flag.FlagSet, flagName string) int {
	if isExplicitFlag(fs, flagName) {
		return flagVal
	}
	if runtimeVal != nil {
		return *runtimeVal
	}
	return defaultVal
}

func resolveBoolPtrOverride(runtimeVal *bool, flagVal bool, defaultVal bool, fs *flag.FlagSet, flagName string) *bool {
	if isExplicitFlag(fs, flagName) {
		return &flagVal
	}
	if runtimeVal != nil {
		return runtimeVal
	}
	return &defaultVal
}

func isExplicitFlag(fs *flag.FlagSet, name string) bool {
	found := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == name {
			found = true
		}
	})
	return found
}

func validateRotateFlags(rotate bool, maxMB, maxBackups, maxAge int) error {
	if !rotate {
		return nil
	}
	if maxMB <= 0 {
		return fmt.Errorf("log-max-size-mb must be > 0 when rotation is enabled, got %d", maxMB)
	}
	if maxBackups < 0 {
		return fmt.Errorf("log-max-backups must be >= 0 when rotation is enabled, got %d", maxBackups)
	}
	if maxAge < 0 {
		return fmt.Errorf("log-max-age-days must be >= 0 when rotation is enabled, got %d", maxAge)
	}
	return nil
}

func parseMCPMetadataConnectTimeout(runtime runtimeconfig.Config) (time.Duration, error) {
	d, set, err := runtimeconfig.ParseConnectTimeout(runtime.Metadata.ConnectTimeout)
	if err != nil {
		return 0, fmt.Errorf("metadata.connect_timeout: %w", err)
	}
	if !set {
		return 0, nil
	}
	if d <= 0 {
		return 0, fmt.Errorf("metadata.connect_timeout %q: must be positive", runtime.Metadata.ConnectTimeout)
	}
	return d, nil
}
