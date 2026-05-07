// Package main verifies MCP top-level panic recovery.
// input: subprocess environment triggering panic during server construction
// output: regression coverage ensuring top-level recover logs and exits with code 4
// pos: subprocess-based test for top-level panic recovery in the MCP entrypoint
// note: if this file changes, update this header and module README.md.
package main

import (
	"bytes"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestRunPanicRecoveryExitsWithCode4(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("resolve test executable: %v", err)
	}

	var stderr bytes.Buffer
	cmd := exec.Command(exe)
	cmd.Env = append(os.Environ(), runAsMCPPanicTest+"=1")
	cmd.Stderr = &stderr

	err = cmd.Run()
	if err == nil {
		t.Fatal("expected non-zero exit from panic test subprocess")
	}

	if exitErr, ok := err.(*exec.ExitError); ok {
		if exitErr.ExitCode() != 4 {
			t.Fatalf("expected exit code 4, got %d", exitErr.ExitCode())
		}
	} else {
		t.Fatalf("unexpected error type: %v", err)
	}

	output := stderr.String()
	if !strings.Contains(output, "FATAL: MCP server panic") {
		t.Errorf("stderr should contain FATAL message, got:\n%s", output)
	}
	if !strings.Contains(output, "test construction panic") {
		t.Errorf("stderr should contain panic message, got:\n%s", output)
	}
	if !strings.Contains(output, "Stack trace:") {
		t.Errorf("stderr should contain stack trace, got:\n%s", output)
	}
}
