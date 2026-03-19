// Package cli exposes the command-line adapter for DeltaScope.
// input: audit command flags, SQL text from flags/files/stdin, and application audit services
// output: rendered audit results and exit-code mapping for CLI audit invocations
// pos: CLI audit command implementation above the application service and output renderers
// note: if this file changes, update this header and module README.md.
package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	appaudit "github.com/Fanduzi/DeltaScope/internal/application/audit"
	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	jsonrender "github.com/Fanduzi/DeltaScope/internal/infrastructure/output/json"
	"github.com/Fanduzi/DeltaScope/internal/infrastructure/output/markdown"
	"github.com/spf13/cobra"
)

type userError struct {
	message string
}

func (e userError) Error() string { return e.message }

func newUserError(message string) error {
	return userError{message: message}
}

func newAuditCmd(options *cliOptions, exitCode *int) *cobra.Command {
	var inlineSQL string
	var filePath string

	cmd := &cobra.Command{
		Use:   "audit",
		Short: "Audit SQL from flags, files, or stdin",
		RunE: func(cmd *cobra.Command, args []string) error {
			sql, err := resolveAuditSQL(cmd.Context(), cmd.InOrStdin(), inlineSQL, filePath)
			if err != nil {
				*exitCode = exitUser
				return err
			}

			result, err := appaudit.AuditSQL(cmd.Context(), appaudit.Request{
				SQL:        sql,
				Dialect:    parseDialect(options.Dialect),
				ConfigPath: options.ConfigPath,
			})
			if err != nil {
				return mapAuditError(exitCode, err)
			}

			output, err := renderResult(options.Format, result)
			if err != nil {
				*exitCode = exitInternal
				return err
			}

			if _, err := cmd.OutOrStdout().Write(output); err != nil {
				*exitCode = exitInternal
				return err
			}
			if len(output) == 0 || output[len(output)-1] != '\n' {
				if _, err := io.WriteString(cmd.OutOrStdout(), "\n"); err != nil {
					*exitCode = exitInternal
					return err
				}
			}

			*exitCode = exitCodeForResult(result, options.FailOn)
			return nil
		},
	}

	cmd.Flags().StringVar(&inlineSQL, "sql", "", "inline SQL text to audit")
	cmd.Flags().StringVar(&filePath, "file", "", "path to a SQL file to audit")
	return cmd
}

func resolveAuditSQL(ctx context.Context, stdin io.Reader, inlineSQL string, filePath string) (string, error) {
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

func parseDialect(value string) spec.Dialect {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "mysql":
		return spec.DialectMySQL
	case "tidb":
		return spec.DialectTiDB
	default:
		return spec.Dialect(strings.ToLower(strings.TrimSpace(value)))
	}
}

func renderResult(format string, result report.Result) ([]byte, error) {
	switch format {
	case "json":
		return jsonrender.Render(result)
	case "markdown":
		fallthrough
	default:
		return markdown.Render(result)
	}
}

func exitCodeForResult(result report.Result, threshold string) int {
	switch threshold {
	case "none":
		return exitOK
	case "notice":
		if result.Summary.Notices > 0 || result.Summary.Warnings > 0 || result.Summary.Blockers > 0 {
			return exitAudit
		}
	case "warning":
		if result.Summary.Warnings > 0 || result.Summary.Blockers > 0 {
			return exitAudit
		}
	default:
		if result.Summary.Blockers > 0 {
			return exitAudit
		}
	}
	return exitOK
}

func mapAuditError(exitCode *int, err error) error {
	var inputErr userError
	switch {
	case errors.As(err, &inputErr):
		*exitCode = exitUser
	case errors.Is(err, appaudit.ErrEmptySQL), errors.Is(err, appaudit.ErrUnknownDialect):
		*exitCode = exitUser
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		*exitCode = exitInternal
	case strings.Contains(err.Error(), "parse sql:"):
		*exitCode = exitUser
	default:
		*exitCode = exitInternal
	}
	return err
}
