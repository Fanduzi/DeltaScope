// Package cli exposes the command-line adapter for DeltaScope.
// input: audit command flags, SQL text from flags/files/stdin, password prompt dependencies, and application audit services
// output: rendered audit results, connection-option validation, and exit-code mapping for CLI audit invocations
// pos: CLI audit command implementation above the application service and output renderers
// note: if this file changes, update this header and module README.md.
package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	appaudit "github.com/Fanduzi/DeltaScope/internal/application/audit"
	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	jsonrender "github.com/Fanduzi/DeltaScope/internal/infrastructure/output/json"
	"github.com/Fanduzi/DeltaScope/internal/infrastructure/output/markdown"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/term"
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
			if _, err := resolveConnectionOptions(cmd, options); err != nil {
				*exitCode = exitUser
				return err
			}

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

			output, err := renderResult(options.Format, options.Quiet, result)
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

	cmd.Flags().BoolP("help", "", false, "help for audit")
	cmd.Flags().StringVar(&inlineSQL, "sql", "", "inline SQL text to audit")
	cmd.Flags().StringVar(&filePath, "file", "", "path to a SQL file to audit")
	cmd.Flags().StringVarP(&options.Host, "host", "h", "", "database host for metadata-aware audit")
	cmd.Flags().IntVarP(&options.Port, "port", "P", options.Port, "database port for metadata-aware audit")
	cmd.Flags().StringVarP(&options.User, "user", "u", "", "database user for metadata-aware audit")
	cmd.Flags().StringVarP(&options.Password, "password", "p", "", "database password for metadata-aware audit")
	cmd.Flags().BoolVar(&options.AskPassword, "ask-password", false, "prompt for a database password without echo")
	cmd.Flags().StringVarP(&options.Schema, "schema", "D", "", "database schema for metadata-aware audit")
	cmd.Flags().StringVarP(&options.Socket, "socket", "S", "", "database Unix socket for metadata-aware audit")
	return cmd
}

type auditConnectionOptions struct {
	Host     string
	Port     int
	User     string
	Password string
	Schema   string
	Socket   string
}

var passwordPrompt = promptPassword

func resolveConnectionOptions(cmd *cobra.Command, options *cliOptions) (auditConnectionOptions, error) {
	resolved := auditConnectionOptions{
		Host:     strings.TrimSpace(options.Host),
		Port:     options.Port,
		User:     strings.TrimSpace(options.User),
		Password: options.Password,
		Schema:   strings.TrimSpace(options.Schema),
		Socket:   strings.TrimSpace(options.Socket),
	}

	if options.AskPassword && strings.TrimSpace(options.Password) != "" {
		return auditConnectionOptions{}, newUserError("--password and --ask-password are mutually exclusive")
	}
	if resolved.Socket != "" && (resolved.Host != "" || cmd.Flags().Changed("port")) {
		return auditConnectionOptions{}, newUserError("--socket cannot be combined with host/port TCP options")
	}
	if options.AskPassword {
		password, err := passwordPrompt(cmd.InOrStdin(), cmd.ErrOrStderr())
		if err != nil {
			return auditConnectionOptions{}, newUserError(fmt.Sprintf("prompt password: %v", err))
		}
		resolved.Password = password
	}

	return resolved, nil
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

func renderResult(format string, quiet bool, result report.Result) ([]byte, error) {
	switch format {
	case "json":
		return jsonrender.Render(result)
	case "markdown":
		if quiet {
			return renderQuietResult(result), nil
		}
		fallthrough
	default:
		return markdown.Render(result)
	}
}

func renderQuietResult(result report.Result) []byte {
	lines := make([]string, 0)
	for _, statement := range result.Statements {
		for _, finding := range statement.Findings {
			lines = append(lines, formatQuietFinding(finding))
		}
	}
	for _, finding := range result.GlobalFindings {
		lines = append(lines, formatQuietFinding(finding))
	}
	if len(lines) == 0 {
		return []byte("pass")
	}
	return []byte(strings.Join(lines, "\n"))
}

func formatQuietFinding(finding rule.Finding) string {
	return fmt.Sprintf("[%s] %s: %s", finding.Level, finding.RuleID, finding.Message)
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
	var fileNotFoundErr viper.ConfigFileNotFoundError
	var configParseErr viper.ConfigParseError
	switch {
	case errors.As(err, &inputErr):
		*exitCode = exitUser
	case errors.Is(err, appaudit.ErrEmptySQL), errors.Is(err, appaudit.ErrUnknownDialect):
		*exitCode = exitUser
	case errors.As(err, &fileNotFoundErr), errors.As(err, &configParseErr):
		*exitCode = exitUser
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		*exitCode = exitInternal
	case strings.Contains(err.Error(), "parse sql:"):
		*exitCode = exitUser
	case strings.Contains(err.Error(), "load policy:"):
		*exitCode = exitUser
	default:
		*exitCode = exitInternal
	}
	return err
}

func promptPassword(stdin io.Reader, stderr io.Writer) (string, error) {
	if _, err := io.WriteString(stderr, "Password: "); err != nil {
		return "", err
	}

	file, ok := stdin.(*os.File)
	if ok && term.IsTerminal(int(file.Fd())) {
		bytes, err := term.ReadPassword(int(file.Fd()))
		if _, writeErr := io.WriteString(stderr, "\n"); writeErr != nil && err == nil {
			err = writeErr
		}
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(string(bytes)), nil
	}

	line, err := bufio.NewReader(stdin).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	return strings.TrimSpace(line), nil
}
