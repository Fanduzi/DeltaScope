// Package cli exposes the command-line adapter for DeltaScope.
// input: process control from cmd/deltascope and Cobra command execution requests
// output: executable CLI behavior and stable process exit codes for local audits
// pos: interface adapter between process entrypoint and application services
// note: if this file changes, update this header and module README.md.
package cli

import (
	"context"
	"fmt"
	"io"
	"os"

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
	cmd.SetArgs(args)
	if err := cmd.ExecuteContext(ctx); err != nil {
		if exitCode == exitOK {
			exitCode = exitInternal
		}
		_, _ = fmt.Fprintln(stderr, err)
	}
	return exitCode
}
