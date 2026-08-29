// Package cli exposes the command-line adapter for DeltaScope.
// input: process control from cmd/deltascope, Cobra command execution requests, and legacy audit flag placement
// output: executable CLI behavior and stable process exit codes, including legacy audit flag normalization and Cobra usage-error mapping for audit and query-access command paths
// pos: interface adapter between process entrypoint and application services
// note: if this file changes, update this header and module README.md.
package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	publicapi "github.com/Fanduzi/DeltaScope/pkg/deltascope"
)

const (
	exitOK       = 0
	exitAudit    = 1
	exitUser     = 2
	exitInternal = 3
)

// Version is the build version printed by the version command.
var Version = publicapi.DefaultVersion

// Run executes the CLI against the current process environment and exits.
func Run() {
	os.Exit(Execute(context.Background(), os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

// Execute runs the CLI using the supplied process-like dependencies.
func Execute(ctx context.Context, args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) int {
	exitCode := exitOK
	cmd := newRootCmd(&exitCode, stdin, stdout, stderr)
	normalizedArgs := normalizeLegacyAuditFlags(args)
	cmd.SetArgs(normalizedArgs)
	if err := cmd.ExecuteContext(ctx); err != nil {
		if exitCode == exitOK {
			exitCode = classifyUnsetExecuteError(normalizedArgs, err)
		}
		_, _ = fmt.Fprintln(stderr, err)
	}
	return exitCode
}

func classifyUnsetExecuteError(args []string, err error) int {
	if !isCLIUsageError(err) {
		return exitInternal
	}
	if firstPositionalArg(args) == "query-access" {
		return exitQueryAccessUsageError
	}
	return exitUser
}

func isCLIUsageError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "unknown flag:") ||
		strings.Contains(msg, "unknown shorthand flag:") ||
		strings.Contains(msg, "unknown command") ||
		strings.Contains(msg, "flag needs an argument")
}

func firstPositionalArg(args []string) string {
	index := firstPositionalArgIndex(args)
	if index < 0 {
		return ""
	}
	return args[index]
}

func firstPositionalArgIndex(args []string) int {
	skipValue := false
	for i, arg := range args {
		if skipValue {
			skipValue = false
			continue
		}
		if arg == "--" {
			return -1
		}
		if strings.HasPrefix(arg, "-") {
			if !strings.Contains(arg, "=") {
				switch arg {
				case "--config", "--dialect", "--format", "--fail-on":
					skipValue = true
				}
			}
			continue
		}
		return i
	}
	return -1
}

func normalizeLegacyAuditFlags(args []string) []string {
	auditIndex := firstPositionalArgIndex(args)
	if auditIndex < 0 || args[auditIndex] != "audit" {
		return args
	}

	prefix := make([]string, 0, auditIndex)
	auditFlags := make([]string, 0, 2)
	for i := 0; i < auditIndex; i++ {
		arg := args[i]
		if arg != "--format" && arg != "--fail-on" && !strings.HasPrefix(arg, "--format=") && !strings.HasPrefix(arg, "--fail-on=") {
			prefix = append(prefix, arg)
			continue
		}

		auditFlags = append(auditFlags, arg)
		if (arg == "--format" || arg == "--fail-on") && i+1 < auditIndex {
			i++
			auditFlags = append(auditFlags, args[i])
		}
	}
	if len(auditFlags) == 0 {
		return args
	}

	normalized := append(prefix, "audit")
	normalized = append(normalized, auditFlags...)
	return append(normalized, args[auditIndex+1:]...)
}
