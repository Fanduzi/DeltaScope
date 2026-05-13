// Package cli exposes the command-line adapter for DeltaScope.
// input: audit command flags, SQL text from flags/files/stdin, password source/prompt dependencies, and application audit services
// output: rendered audit results, connection-option validation, password resolution, and exit-code mapping for CLI audit invocations
// pos: CLI audit command implementation above the application service and output renderers
// note: if this file changes, update this header and module README.md.
package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	appaudit "github.com/Fanduzi/DeltaScope/internal/application/audit"
	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	"github.com/Fanduzi/DeltaScope/internal/infrastructure/output/githubactions"
	"github.com/Fanduzi/DeltaScope/internal/infrastructure/output/gitlabcodequality"
	"github.com/Fanduzi/DeltaScope/internal/infrastructure/output/markdown"
	"github.com/Fanduzi/DeltaScope/internal/infrastructure/output/sarif"
	ifaceconn "github.com/Fanduzi/DeltaScope/internal/interfaces/metadata"
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
		Long: "Audit SQL in offline mode or enrich the same audit engine with live metadata.\n" +
			"When connection flags are present, DeltaScope uses metadata-aware mode, auto-detects the dialect, and infers schema when possible for mysql, tidb, and postgresql connections.",
		Example: "Offline example:\n" +
			"  deltascope audit --sql \"delete from users\" --dialect mysql\n" +
			"  deltascope audit --sql \"drop index idx_name;\" --dialect postgresql\n\n" +
			"Metadata-aware example:\n" +
			"  deltascope audit --sql \"alter table users add column email varchar(255)\" --host 127.0.0.1 --port 3306 --user root --ask-password --schema app",
		RunE: func(cmd *cobra.Command, args []string) error {
			sql, err := resolveAuditSQL(cmd.Context(), cmd.InOrStdin(), inlineSQL, filePath, cmd.ErrOrStderr(), stdinIsTerminal(cmd))
			if err != nil {
				*exitCode = exitUser
				return err
			}

			connection, err := resolveConnectionOptions(cmd, options)
			if err != nil {
				*exitCode = exitUser
				return err
			}

			dialect := parseDialect(options.Dialect)
			var metadataProvider appaudit.MetadataProvider
			var schema string
			runContext := &auditRunContext{
				Mode:          "offline",
				Dialect:       string(dialect),
				DialectSource: dialectSource(cmd.Flags().Changed("dialect")),
			}
			if connection.Enabled() {
				client, resolvedDialect, resolvedSchema, metadataContext, err := prepareMetadataAudit(cmd.Context(), sql, connection, dialect, cmd.Flags().Changed("dialect"))
				if err != nil {
					*exitCode = exitUser
					return err
				}
				defer client.Close()
				dialect = resolvedDialect
				schema = resolvedSchema
				metadataProvider = cliMetadataProvider{client: client}
				runContext = metadataContext
			}

			result, auditErr := appaudit.AuditSQL(cmd.Context(), appaudit.Request{
				SQL:              sql,
				Dialect:          dialect,
				ConfigPath:       options.ConfigPath,
				Schema:           schema,
				MetadataProvider: metadataProvider,
			})
			if auditErr != nil && !errors.Is(auditErr, appaudit.ErrUnsupportedStatement) && !hasRenderableAuditResult(result) {
				return mapAuditError(exitCode, auditErr)
			}

			output, err := renderResult(options.Format, options.Quiet, result, runContext, filePath)
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

			if auditErr != nil && !errors.Is(auditErr, appaudit.ErrUnsupportedStatement) {
				return mapAuditError(exitCode, auditErr)
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
	cmd.Flags().StringVar(&options.PasswordEnv, "password-env", "", "environment variable that contains the database password for metadata-aware audit")
	cmd.Flags().StringVar(&options.PasswordFile, "password-file", "", "file path that contains the database password for metadata-aware audit")
	cmd.Flags().BoolVar(&options.AskPassword, "ask-password", false, "prompt for a database password without echo")
	cmd.Flags().StringVarP(&options.Schema, "schema", "D", "", "database schema for metadata-aware audit")
	cmd.Flags().StringVarP(&options.Socket, "socket", "S", "", "database Unix socket for metadata-aware audit")
	cmd.Flags().StringVar(&options.MetadataConnectTimeout, "metadata-connect-timeout", "", "metadata connection timeout for metadata-aware audit, for example 5s or 500ms")
	return cmd
}

type auditConnectionOptions struct {
	Host           string
	Port           int
	PortSet        bool
	User           string
	Password       string
	PasswordEnv    string
	PasswordFile   string
	Schema         string
	Socket         string
	Dialect        string
	ConnectTimeout time.Duration
}

var passwordPrompt = promptPassword

func resolveConnectionOptions(cmd *cobra.Command, options *cliOptions) (auditConnectionOptions, error) {
	resolved := auditConnectionOptions{
		Host:         strings.TrimSpace(options.Host),
		Port:         options.Port,
		PortSet:      cmd.Flags().Changed("port"),
		User:         strings.TrimSpace(options.User),
		Password:     options.Password,
		PasswordEnv:  strings.TrimSpace(options.PasswordEnv),
		PasswordFile: strings.TrimSpace(options.PasswordFile),
		Schema:       strings.TrimSpace(options.Schema),
		Socket:       strings.TrimSpace(options.Socket),
	}

	if timeout := strings.TrimSpace(options.MetadataConnectTimeout); timeout != "" {
		d, err := time.ParseDuration(timeout)
		if err != nil {
			return auditConnectionOptions{}, newUserError(fmt.Sprintf("invalid --metadata-connect-timeout: %v", err))
		}
		if d < 0 {
			return auditConnectionOptions{}, newUserError("--metadata-connect-timeout must be a non-negative duration such as 5s")
		}
		resolved.ConnectTimeout = d
	}

	if options.AskPassword && hasConfiguredPasswordSource(resolved) {
		return auditConnectionOptions{}, newUserError("--password, --password-env, --password-file, and --ask-password are mutually exclusive")
	}
	if resolved.Socket != "" && (resolved.Host != "" || resolved.PortSet) {
		return auditConnectionOptions{}, newUserError("--socket cannot be combined with host/port TCP options")
	}
	if options.AskPassword {
		password, err := passwordPrompt(cmd.ErrOrStderr())
		if err != nil {
			return auditConnectionOptions{}, newUserError(fmt.Sprintf("prompt password: %v", err))
		}
		resolved.Password = password
		return resolved, nil
	}

	password, err := ifaceconn.ResolvePassword(ifaceconn.ConnectionInput{
		Password:     resolved.Password,
		PasswordEnv:  resolved.PasswordEnv,
		PasswordFile: resolved.PasswordFile,
	}, ifaceconn.ResolveConnectionOptions{})
	if err != nil {
		return auditConnectionOptions{}, newUserError(err.Error())
	}
	resolved.Password = password
	return resolved, nil
}

func hasConfiguredPasswordSource(options auditConnectionOptions) bool {
	return strings.TrimSpace(options.Password) != "" || options.PasswordEnv != "" || options.PasswordFile != ""
}

func (o auditConnectionOptions) Enabled() bool {
	return o.Host != "" || o.PortSet || o.User != "" || o.Password != "" || o.Schema != "" || o.Socket != ""
}

func resolveAuditSQL(ctx context.Context, stdin io.Reader, inlineSQL string, filePath string, stderr io.Writer, interactive bool) (string, error) {
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

func stdinIsTerminal(cmd *cobra.Command) bool {
	file, ok := cmd.InOrStdin().(*os.File)
	if !ok {
		return false
	}
	return term.IsTerminal(int(file.Fd()))
}

func parseDialect(value string) spec.Dialect {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "mysql":
		return spec.DialectMySQL
	case "tidb":
		return spec.DialectTiDB
	case "postgresql":
		return spec.DialectPostgreSQL
	default:
		return spec.Dialect(strings.ToLower(strings.TrimSpace(value)))
	}
}

func dialectSource(explicit bool) string {
	if explicit {
		return "flag"
	}
	return "default"
}

func hasRenderableAuditResult(result report.Result) bool {
	return len(result.Statements) > 0 || len(result.GlobalFindings) > 0 || len(result.Unsupported) > 0 || result.RuleSummary != nil || result.Explanation != nil || result.Verdict != "" || result.Summary != (report.Summary{})
}

func renderResult(format string, quiet bool, result report.Result, runContext *auditRunContext, sourcePath string) ([]byte, error) {
	switch format {
	case "json":
		return renderJSONResult(result, runContext)
	case "github-actions":
		return githubactions.Render(result, githubactions.Options{Path: sourcePath})
	case "sarif":
		return sarif.Render(result, sarif.Options{Path: sourcePath})
	case "gitlab-codequality":
		return gitlabcodequality.Render(result, gitlabcodequality.Options{Path: sourcePath})
	case "markdown":
		if quiet {
			return renderQuietResult(result, runContext), nil
		}
		return renderMarkdownResult(result, runContext)
	default:
		if quiet {
			return renderQuietResult(result, runContext), nil
		}
		return renderMarkdownResult(result, runContext)
	}
}

func renderMarkdownResult(result report.Result, runContext *auditRunContext) ([]byte, error) {
	body, err := markdown.Render(result)
	if err != nil {
		return nil, err
	}

	var b strings.Builder
	if runContext != nil {
		b.WriteString("## Audit Context\n")
		fmt.Fprintf(&b, "- Mode: `%s`\n", runContext.Mode)
		fmt.Fprintf(&b, "- Dialect: `%s` (%s)\n", runContext.Dialect, runContext.DialectSource)
		if runContext.Schema != "" {
			fmt.Fprintf(&b, "- Schema: `%s` (%s)\n", runContext.Schema, runContext.SchemaSource)
		}
		if hasPostgreSQLSyntaxNotice(result) {
			fmt.Fprintf(&b, "- Trust Note: Dialect remains `%s` (%s). DeltaScope did not auto-switch dialect.\n", runContext.Dialect, runContext.DialectSource)
		}
		b.WriteString("\n")
	}
	b.Write(body)
	if len(result.Unsupported) > 0 {
		b.WriteString("\n\n## Unsupported Statements\n")
		for _, item := range result.Unsupported {
			fmt.Fprintf(&b, "- Statement %d: `%s` — %s\n", item.Index+1, item.Feature, item.Reason)
		}
	}
	return []byte(b.String()), nil
}

func renderJSONResult(result report.Result, runContext *auditRunContext) ([]byte, error) {
	payload := struct {
		report.Result
		Context *auditRunContext `json:"context,omitempty"`
	}{
		Result:  resultWithoutAggregateExplanations(result),
		Context: runContext,
	}
	return json.Marshal(payload)
}

func resultWithoutAggregateExplanations(result report.Result) report.Result {
	result.Explanation = nil
	for i := range result.Statements {
		result.Statements[i].Explanation = nil
	}
	return result
}

func renderQuietResult(result report.Result, runContext *auditRunContext) []byte {
	lines := make([]string, 0)
	for _, statement := range result.Statements {
		for _, finding := range statement.Findings {
			lines = append(lines, formatQuietFinding(finding))
		}
	}
	for _, finding := range result.GlobalFindings {
		lines = append(lines, formatQuietFinding(finding))
	}
	for _, item := range result.Unsupported {
		lines = append(lines, fmt.Sprintf("[unsupported] %s: %s", item.Feature, item.Reason))
	}
	if result.RuleSummary != nil {
		lines = append(lines, fmt.Sprintf("[summary] loaded=%d applicable=%d skipped=%d",
			result.RuleSummary.Loaded,
			result.RuleSummary.Applicable,
			len(result.RuleSummary.Skipped)))
	}
	if len(lines) == 0 {
		return []byte("pass")
	}
	if runContext != nil {
		contextLine := fmt.Sprintf("[context] mode=%s dialect=%s dialect_source=%s", runContext.Mode, runContext.Dialect, runContext.DialectSource)
		if runContext.Schema != "" {
			contextLine += fmt.Sprintf(" schema=%s schema_source=%s", runContext.Schema, runContext.SchemaSource)
		}
		lines = append(lines, contextLine)
	}
	return []byte(strings.Join(lines, "\n"))
}

func hasPostgreSQLSyntaxNotice(result report.Result) bool {
	for _, finding := range result.GlobalFindings {
		if finding.RuleID == "dialect.postgresql.syntax.detected.notice" {
			return true
		}
	}
	return false
}

func formatQuietFinding(finding rule.Finding) string {
	return fmt.Sprintf("[%s] %s: %s", finding.Level, finding.RuleID, finding.Message)
}

func exitCodeForResult(result report.Result, threshold string) int {
	if len(result.Unsupported) > 0 {
		return exitAudit
	}
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
	var capabilityErr *appaudit.PostgreSQLCapabilityBoundaryError
	switch {
	case errors.As(err, &inputErr):
		*exitCode = exitUser
	case errors.Is(err, appaudit.ErrEmptySQL), errors.Is(err, appaudit.ErrUnknownDialect), errors.Is(err, appaudit.ErrUnsupportedStatement):
		*exitCode = exitUser
	case errors.As(err, &fileNotFoundErr), errors.As(err, &configParseErr):
		*exitCode = exitUser
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		*exitCode = exitInternal
	case errors.As(err, &capabilityErr):
		*exitCode = exitUser
	case strings.Contains(err.Error(), "parse sql:"):
		*exitCode = exitUser
	case strings.Contains(err.Error(), "load policy:"):
		*exitCode = exitUser
	default:
		*exitCode = exitInternal
	}
	return err
}

func promptPassword(stderr io.Writer) (string, error) {
	if _, err := io.WriteString(stderr, "Password: "); err != nil {
		return "", err
	}

	file, err := os.Open("/dev/tty")
	if err != nil {
		return "", errors.New("interactive password prompt requires a TTY")
	}
	defer file.Close()

	if term.IsTerminal(int(file.Fd())) {
		bytes, err := term.ReadPassword(int(file.Fd()))
		if _, writeErr := io.WriteString(stderr, "\n"); writeErr != nil && err == nil {
			err = writeErr
		}
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(string(bytes)), nil
	}

	line, err := bufio.NewReader(file).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	if _, writeErr := io.WriteString(stderr, "\n"); writeErr != nil && err == nil {
		err = writeErr
	}
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(line), nil
}
