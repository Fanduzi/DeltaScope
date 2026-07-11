// Package cli exposes the command-line adapter for DeltaScope.
// input: query-access command flags, SQL text from flags/files/stdin, and the public query access API
// output: rendered query access results in JSON format, exit-code mapping for CI integration
// pos: CLI query-access command implementation above the public DeltaScope query access API
// note: if this file changes, update this header and module README.md.
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/Fanduzi/DeltaScope/pkg/deltascope"
	"github.com/spf13/cobra"
)

const (
	exitQueryAccessAdmissible    = 0
	exitQueryAccessRejected      = 1
	exitQueryAccessIndeterminate = 2
	exitQueryAccessUsageError    = 3
)

func newQueryAccessCmd(options *cliOptions, exitCode *int) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "query-access",
		Short: "Query access analysis",
	}
	cmd.AddCommand(newQueryAccessAnalyzeCmd(options, exitCode))
	return cmd
}

func newQueryAccessAnalyzeCmd(options *cliOptions, exitCode *int) *cobra.Command {
	var inlineSQL string
	var filePath string
	var mode string
	var defaultSchema string

	cmd := &cobra.Command{
		Use:   "analyze",
		Short: "Analyze SQL for query access requirements",
		Long:  "Analyze SQL to determine read classification, admission, and permission requirements.",
		Example: "  deltascope query-access analyze --sql \"SELECT id, name FROM users WHERE id = 1\" --dialect mysql\n" +
			"  deltascope query-access analyze --file ./query.sql --dialect postgresql --mode projection_only",
		RunE: func(cmd *cobra.Command, args []string) error {
			sql, err := resolveQueryAccessSQL(cmd.Context(), cmd.InOrStdin(), inlineSQL, filePath, cmd.ErrOrStderr(), stdinIsTerminal(cmd))
			if err != nil {
				*exitCode = exitQueryAccessUsageError
				return err
			}

			dialect := parseDialect(options.Dialect)
			accessMode := deltascope.QueryAccessMode(strings.TrimSpace(mode))
			if accessMode == "" {
				accessMode = deltascope.QueryAccessModeStrict
			}
			if accessMode != deltascope.QueryAccessModeStrict && accessMode != deltascope.QueryAccessModeProjectionOnly {
				*exitCode = exitQueryAccessUsageError
				return fmt.Errorf("invalid mode %q: must be strict or projection_only", accessMode)
			}

			result, err := deltascope.AnalyzeQueryAccess(cmd.Context(), deltascope.QueryAccessRequest{
				SQL:           sql,
				Dialect:       deltascope.Dialect(dialect),
				Mode:          accessMode,
				DefaultSchema: strings.TrimSpace(defaultSchema),
			})
			if err != nil {
				*exitCode = exitQueryAccessUsageError
				return err
			}

			output, err := json.MarshalIndent(result, "", "  ")
			if err != nil {
				*exitCode = exitInternal
				return fmt.Errorf("marshal result: %w", err)
			}

			if _, err := cmd.OutOrStdout().Write(output); err != nil {
				*exitCode = exitInternal
				return err
			}
			if _, err := io.WriteString(cmd.OutOrStdout(), "\n"); err != nil {
				*exitCode = exitInternal
				return err
			}

			*exitCode = exitCodeForQueryAccess(result)
			return nil
		},
	}

	cmd.Flags().StringVar(&inlineSQL, "sql", "", "inline SQL text to analyze")
	cmd.Flags().StringVar(&filePath, "file", "", "path to a SQL file to analyze")
	cmd.Flags().StringVar(&mode, "mode", "", "analysis mode: strict or projection_only (default strict)")
	cmd.Flags().StringVar(&defaultSchema, "default-schema", "", "default schema for unqualified references")
	return cmd
}

func resolveQueryAccessSQL(ctx context.Context, stdin io.Reader, inlineSQL string, filePath string, stderr io.Writer, interactive bool) (string, error) {
	if strings.TrimSpace(inlineSQL) != "" && strings.TrimSpace(filePath) != "" {
		return "", newUserError("use either --sql or --file, not both")
	}
	if strings.TrimSpace(inlineSQL) != "" {
		return inlineSQL, nil
	}
	if strings.TrimSpace(filePath) != "" {
		content, err := os.ReadFile(filePath)
		if err != nil {
			return "", newUserError(fmt.Sprintf("read SQL file: %v", err))
		}
		if err := ctx.Err(); err != nil {
			return "", err
		}
		if strings.TrimSpace(string(content)) == "" {
			return "", newUserError("SQL input must not be empty")
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
		return "", newUserError("SQL input must not be empty")
	}
	return string(content), nil
}

func exitCodeForQueryAccess(result *deltascope.QueryAccessResult) int {
	switch result.Admission {
	case deltascope.QueryAccessAdmissible:
		return exitQueryAccessAdmissible
	case deltascope.QueryAccessRejected:
		return exitQueryAccessRejected
	case deltascope.QueryAccessIndeterminateAdmission:
		return exitQueryAccessIndeterminate
	default:
		return exitQueryAccessAdmissible
	}
}
