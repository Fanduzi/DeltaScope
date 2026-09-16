// Package cli exposes the command-line adapter for DeltaScope.
// input: --sql/--file/stdin text plus command-specific empty-input copy
// output: SQL text or a user error that names the command
// pos: shared CLI SQL loader used by audit and query-access
// note: if this file changes, update this header and module README.md.
package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
)

func resolveCLISQL(ctx context.Context, stdin io.Reader, inlineSQL string, filePath string, stderr io.Writer, interactive bool, sqlProvided bool, emptyMsg string, mapFileErr func(error) error) (string, error) {
	if sqlProvided && strings.TrimSpace(filePath) != "" {
		return "", newUserError("use either --sql or --file, not both")
	}
	if sqlProvided {
		if strings.TrimSpace(inlineSQL) == "" {
			return "", newUserError(emptyMsg)
		}
		return inlineSQL, nil
	}
	if strings.TrimSpace(filePath) != "" {
		content, err := os.ReadFile(filePath)
		if err != nil {
			return "", mapFileErr(err)
		}
		if err := ctx.Err(); err != nil {
			return "", err
		}
		if strings.TrimSpace(string(content)) == "" {
			return "", newUserError(emptyMsg)
		}
		return string(content), nil
	}

	if interactive {
		if _, err := io.WriteString(stderr, "Waiting for SQL from stdin. Press Ctrl+D to finish.\n"); err != nil {
			return "", newUserError(fmt.Sprintf("write stdin hint: %v", err))
		}
	}

	content, err := io.ReadAll(stdin)
	if err != nil {
		return "", newUserError(fmt.Sprintf("read stdin: %v", err))
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if strings.TrimSpace(string(content)) == "" {
		return "", newUserError(emptyMsg)
	}
	return string(content), nil
}
