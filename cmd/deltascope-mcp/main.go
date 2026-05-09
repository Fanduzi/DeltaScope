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

	"github.com/Fanduzi/DeltaScope/internal/infrastructure/logger"
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
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *showVersion {
		_, _ = fmt.Fprintln(stdout, Version)
		return 0
	}

	slogLogger, slogErr := logger.NewLogger(logger.Config{
		Level:    *logLevel,
		Format:   *logFormat,
		Output:   *logOutput,
		FilePath: *logFile,
	}, "mcp")
	if slogErr != nil {
		_, _ = fmt.Fprintf(stderr, "init logger: %v\n", slogErr)
		return 2
	}

	server := newMCPServer(mcpapi.Config{
		Version:         Version,
		ConnectionsPath: *connectionsPath,
		Logger:          slogLogger,
	})
	if err := runMCPServer(server); err != nil {
		_, _ = fmt.Fprintf(stderr, "serve mcp: %v\n", err)
		return 3
	}
	return 0
}
