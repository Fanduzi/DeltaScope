// Package cli exposes the command-line adapter for DeltaScope.
// input: audit command flags including whether --sql was explicitly provided, SQL text from flags/files/stdin, password source/prompt dependencies, and application audit services
// output: rendered audit results, dialect-aware connection-option normalization with MySQL/TiDB catalog aliases and PostgreSQL schema/database validation, password resolution, offline existence caveats on default surfaces, and user-vs-runtime exit-code mapping for CLI audit invocations
// pos: CLI audit command implementation above the application service and output renderers
// note: if this file changes, update this header and module README.md.
package cli

import (
	"bufio"
	"context"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	appaudit "github.com/Fanduzi/DeltaScope/internal/application/audit"
	auditmeta "github.com/Fanduzi/DeltaScope/internal/application/auditmeta"
	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	"github.com/Fanduzi/DeltaScope/internal/infrastructure/output/githubactions"
	"github.com/Fanduzi/DeltaScope/internal/infrastructure/output/githubsummary"
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

type runtimeError struct {
	message string
}

func (e runtimeError) Error() string { return e.message }

func newRuntimeError(message string) error {
	return runtimeError{message: message}
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
			sql, err := resolveAuditSQL(cmd.Context(), cmd.InOrStdin(), inlineSQL, filePath, cmd.ErrOrStderr(), stdinIsTerminal(cmd), cmd.Flags().Changed("sql"))
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
				Note:          existenceNotCheckedNote,
				Unproven:      offlineExistenceUnproven(),
			}
			if connection.Enabled() {
				client, resolvedDialect, resolvedSchema, metadataContext, err := prepareMetadataAudit(cmd.Context(), sql, connection, dialect, cmd.Flags().Changed("dialect"))
				if err != nil {
					mapped := mapMetadataPrepareError(err, connection)
					*exitCode = exitCodeForCLIError(mapped)
					return mapped
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
	cmd.Flags().IntVarP(&options.Port, "port", "P", options.Port, "database port for metadata-aware audit (3306 for MySQL/TiDB/auto-detect; 5432 for explicit PostgreSQL)")
	cmd.Flags().StringVarP(&options.User, "user", "u", "", "database user for metadata-aware audit")
	cmd.Flags().StringVar(&options.PasswordEnv, "password-env", "", "environment variable that contains the database password for metadata-aware audit")
	cmd.Flags().StringVar(&options.PasswordFile, "password-file", "", "file path that contains the database password for metadata-aware audit")
	cmd.Flags().BoolVar(&options.AskPassword, "ask-password", false, "prompt for a database password without echo")
	cmd.Flags().StringVarP(&options.Schema, "schema", "D", "", "schema for metadata-aware audit (MySQL/TiDB catalog; PostgreSQL schema)")
	cmd.Flags().StringVarP(&options.Socket, "socket", "S", "", "database Unix socket for metadata-aware audit")
	cmd.Flags().StringVar(&options.Database, "database", "", "database/catalog for metadata-aware audit (MySQL/TiDB alias of --schema; PostgreSQL database)")
	cmd.Flags().StringVar(&options.MetadataConnectTimeout, "metadata-connect-timeout", "", "metadata connection timeout for metadata-aware audit, for example 5s or 500ms")
	cmd.Flags().StringVar(&options.TLSMode, "tls-mode", "disabled", "TLS mode for database connection: disabled or enabled")
	cmd.Flags().StringVar(&options.TLSCAFile, "tls-ca-file", "", "path to TLS CA certificate PEM file (requires tls-mode=enabled)")
	return cmd
}

type auditConnectionOptions struct {
	Host              string
	Port              int
	PortSet           bool
	User              string
	Password          string
	PasswordEnv       string
	PasswordFile      string
	Schema            string
	Database          string
	Socket            string
	Dialect           string
	ConnectTimeout    time.Duration
	TLSMode           string
	TLSCAFile         string
	CACert            *x509.CertPool
	passwordSourceSet bool
}

var passwordPrompt = promptPassword

func resolveConnectionOptions(cmd *cobra.Command, options *cliOptions) (auditConnectionOptions, error) {
	portSet := cmd.Flags().Changed("port")
	port := options.Port
	if !portSet && cmd.Flags().Changed("dialect") && parseDialect(options.Dialect) == spec.DialectPostgreSQL {
		port = 5432
	}
	resolved := auditConnectionOptions{
		Host:         strings.TrimSpace(options.Host),
		Port:         port,
		PortSet:      portSet,
		User:         strings.TrimSpace(options.User),
		PasswordEnv:  strings.TrimSpace(options.PasswordEnv),
		PasswordFile: strings.TrimSpace(options.PasswordFile),
		Schema:       strings.TrimSpace(options.Schema),
		Database:     strings.TrimSpace(options.Database),
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
		return auditConnectionOptions{}, newUserError("--password-env, --password-file, and --ask-password are mutually exclusive")
	}
	if resolved.Socket != "" && (resolved.Host != "" || resolved.PortSet) {
		return auditConnectionOptions{}, newUserError("--socket cannot be combined with host/port TCP options")
	}

	// TLS validation
	tlsMode := strings.TrimSpace(options.TLSMode)
	if tlsMode == "" {
		tlsMode = "disabled"
	}
	if tlsMode != "disabled" && tlsMode != "enabled" {
		return auditConnectionOptions{}, newUserError("invalid --tls-mode: must be disabled or enabled")
	}
	resolved.TLSMode = tlsMode

	caFile := strings.TrimSpace(options.TLSCAFile)
	if caFile != "" && tlsMode != "enabled" {
		return auditConnectionOptions{}, newUserError("--tls-ca-file requires --tls-mode=enabled")
	}
	if tlsMode == "enabled" {
		if resolved.Host == "" {
			return auditConnectionOptions{}, newUserError("--tls-mode=enabled requires --host")
		}
		if resolved.User == "" {
			return auditConnectionOptions{}, newUserError("--tls-mode=enabled requires --user")
		}
		if resolved.Socket != "" {
			return auditConnectionOptions{}, newUserError("--tls-mode=enabled cannot be used with --socket")
		}
	}

	if caFile != "" {
		expanded, err := ifaceconn.ExpandHome(caFile)
		if err != nil {
			return auditConnectionOptions{}, newUserError("invalid TLS CA file path")
		}
		pemBytes, err := os.ReadFile(expanded)
		if err != nil {
			return auditConnectionOptions{}, newUserError("cannot read TLS CA file")
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pemBytes) {
			return auditConnectionOptions{}, newUserError("invalid TLS CA certificate")
		}
		resolved.CACert = pool
	}

	if options.AskPassword {
		password, err := passwordPrompt(cmd.ErrOrStderr())
		if err != nil {
			return auditConnectionOptions{}, newUserError(fmt.Sprintf("prompt password: %v", err))
		}
		resolved.Password = password
		resolved.passwordSourceSet = true
		return resolved, nil
	}

	password, err := ifaceconn.ResolvePassword(ifaceconn.ConnectionInput{
		Password:     resolved.Password,
		PasswordEnv:  resolved.PasswordEnv,
		PasswordFile: resolved.PasswordFile,
	}, ifaceconn.ResolveConnectionOptions{})
	if err != nil {
		return auditConnectionOptions{}, newUserError("invalid password source")
	}
	resolved.Password = password
	resolved.passwordSourceSet = hasConfiguredPasswordSource(resolved) || resolved.Password != ""
	return resolved, nil
}

func hasConfiguredPasswordSource(options auditConnectionOptions) bool {
	return options.PasswordEnv != "" || options.PasswordFile != ""
}

func (o auditConnectionOptions) Enabled() bool {
	return o.Host != "" || o.PortSet || o.User != "" || o.Password != "" || o.Schema != "" || o.Socket != ""
}

func resolveAuditSQL(ctx context.Context, stdin io.Reader, inlineSQL string, filePath string, stderr io.Writer, interactive bool, sqlProvided bool) (string, error) {
	if sqlProvided && strings.TrimSpace(filePath) != "" {
		return "", newUserError("use either --sql or --file, not both")
	}
	if sqlProvided {
		if strings.TrimSpace(inlineSQL) == "" {
			return "", newUserError("SQL input must not be empty")
		}
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
	return len(result.Statements) > 0 || len(result.GlobalFindings) > 0 || len(result.Unsupported) > 0 || result.RuleSummary != nil || result.Explanation != nil || result.Verdict != "" || result.Summary != (report.Summary{}) || len(result.Diagnostics) > 0
}

func renderResult(format string, quiet bool, result report.Result, runContext *auditRunContext, sourcePath string) ([]byte, error) {
	switch format {
	case "json":
		return renderJSONResult(result, runContext)
	case "github-actions":
		return githubactions.Render(result, githubactions.Options{Path: sourcePath})
	case "github-summary":
		return githubsummary.Render(result)
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
		if runContext.Note != "" {
			fmt.Fprintf(&b, "- %s\n", runContext.Note)
		}
		b.WriteString("\n")
	}
	body = insertActionSummaryNote(body, runContext)
	b.Write(body)
	if len(result.Unsupported) > 0 {
		b.WriteString("\n\n## Unsupported Statements\n")
		for _, item := range result.Unsupported {
			fmt.Fprintf(&b, "- Statement %d: `%s` — %s\n", item.Index+1, item.Feature, item.Reason)
		}
	}
	if len(result.Diagnostics) > 0 {
		b.WriteString("\n\n## Diagnostics\n")
		for _, d := range result.Diagnostics {
			fmt.Fprintf(&b, "- classification: %s\n  action_hint: %s\n  reason: %s\n  audited: %v\n  dialect: %s\n", d.Classification, d.ActionHint, d.Reason, d.Audited, d.Dialect)
			if d.GuidanceCode != "" {
				fmt.Fprintf(&b, "  guidance_code: %s\n", d.GuidanceCode)
			}
			if d.EvidenceRef != "" {
				fmt.Fprintf(&b, "  evidence_ref: %s\n", d.EvidenceRef)
			}
		}
	}
	return []byte(b.String()), nil
}

func insertActionSummaryNote(body []byte, runContext *auditRunContext) []byte {
	if runContext == nil || runContext.Note == "" {
		return body
	}
	const heading = "## Action Summary\n\n"
	text := string(body)
	idx := strings.Index(text, heading)
	if idx < 0 {
		return body
	}
	insertAt := idx + len(heading)
	return []byte(text[:insertAt] + runContext.Note + "\n\n" + text[insertAt:])
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
	for _, d := range result.Diagnostics {
		line := fmt.Sprintf("[diagnostic] classification=%s action_hint=%s reason=%s audited=%v dialect=%s", d.Classification, d.ActionHint, d.Reason, d.Audited, d.Dialect)
		if d.GuidanceCode != "" {
			line += fmt.Sprintf(" guidance_code=%s", d.GuidanceCode)
		}
		if d.EvidenceRef != "" {
			line += fmt.Sprintf(" evidence_ref=%s", d.EvidenceRef)
		}
		lines = append(lines, line)
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
		if runContext.Note != "" {
			contextLine += " " + runContext.Note
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
	case strings.Contains(err.Error(), "statement was not audited because the selected dialect parser could not parse it"):
		*exitCode = exitUser
	case strings.Contains(err.Error(), "load policy:"):
		*exitCode = exitUser
	case isOnlineConnectionError(err):
		*exitCode = exitInternal
		return mapOnlineCLIBoundaryError(err)
	default:
		*exitCode = exitInternal
	}
	return err
}

func isBoundedApplicationError(err error) bool {
	var ue userError
	if errors.As(err, &ue) {
		return true
	}
	var capabilityErr *appaudit.PostgreSQLCapabilityBoundaryError
	if errors.As(err, &capabilityErr) {
		return true
	}
	var auditMetaErr *auditmeta.Error
	if errors.As(err, &auditMetaErr) {
		return isUserFacingAuditMetaErrorKind(auditMetaErr.Kind)
	}
	return false
}

func isUserFacingAuditMetaErrorKind(kind auditmeta.ErrorKind) bool {
	switch kind {
	case auditmeta.ErrorDialectMismatch, auditmeta.ErrorSchemaHintRequired, auditmeta.ErrorPostgreSQLDatabaseRequired, auditmeta.ErrorMySQLDatabaseSchemaConflict:
		return true
	default:
		return false
	}
}

func mapMetadataPrepareError(err error, connection auditConnectionOptions) error {
	if isBoundedApplicationError(err) {
		return err
	}
	var auditMetaErr *auditmeta.Error
	if errors.As(err, &auditMetaErr) {
		return applyPasswordSourceHint(mapAuditMetaErrorToBounded(auditMetaErr), connection)
	}
	return applyPasswordSourceHint(mapOnlineCLIBoundaryError(err), connection)
}

func exitCodeForCLIError(err error) int {
	var ue userError
	if errors.As(err, &ue) {
		return exitUser
	}
	var re runtimeError
	if errors.As(err, &re) {
		return exitInternal
	}
	var auditMetaErr *auditmeta.Error
	if errors.As(err, &auditMetaErr) {
		if isUserFacingAuditMetaErrorKind(auditMetaErr.Kind) {
			return exitUser
		}
	}
	var capabilityErr *appaudit.PostgreSQLCapabilityBoundaryError
	if errors.As(err, &capabilityErr) {
		return exitUser
	}
	return exitInternal
}

func mapAuditMetaErrorToBounded(err *auditmeta.Error) error {
	switch err.Kind {
	case auditmeta.ErrorConnectionOpen:
		return classifyConnectionError(err)
	case auditmeta.ErrorDialectDetect:
		return newUserError("server identity error")
	case auditmeta.ErrorSchemaLookupFailed:
		return newUserError("metadata analysis failed")
	case auditmeta.ErrorInvalidSQL:
		return newUserError("invalid SQL input")
	default:
		return newRuntimeError("connection failed")
	}
}

func classifyConnectionError(err error) error {
	msg := connectionErrorText(err)
	switch {
	case isAuthenticationFailure(msg):
		return newRuntimeError("authentication failed")
	case strings.Contains(msg, "certificate") || strings.Contains(msg, "x509"):
		return newRuntimeError("TLS certificate verification failed")
	case strings.Contains(msg, "tls"):
		return newRuntimeError("TLS handshake failed")
	case strings.Contains(msg, "timeout"):
		return newRuntimeError("connection timed out")
	default:
		return newRuntimeError("connection failed")
	}
}

func connectionErrorText(err error) string {
	if err == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString(strings.ToLower(err.Error()))
	if inner := errors.Unwrap(err); inner != nil {
		b.WriteByte('\n')
		b.WriteString(strings.ToLower(inner.Error()))
	}
	return b.String()
}

func isAuthenticationFailure(msg string) bool {
	return strings.Contains(msg, "access denied") ||
		strings.Contains(msg, "authentication failed") ||
		strings.Contains(msg, "password authentication") ||
		strings.Contains(msg, "invalid authorization")
}

func applyPasswordSourceHint(err error, connection auditConnectionOptions) error {
	if err != nil && err.Error() == "authentication failed" && !connection.passwordSourceSet {
		return newUserError("password source required: use --password-env, --password-file, or --ask-password")
	}
	return err
}

func isOnlineConnectionError(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "connection refused") ||
		strings.Contains(msg, "no route to host") ||
		strings.Contains(msg, "network is unreachable") ||
		strings.Contains(msg, "host unreachable") ||
		strings.Contains(msg, "certificate") ||
		strings.Contains(msg, "timeout") ||
		strings.Contains(msg, "dial tcp") ||
		strings.Contains(msg, "dial unix") ||
		strings.Contains(msg, "tls:") ||
		strings.Contains(msg, "x509:") ||
		strings.Contains(msg, "connection failed") ||
		strings.Contains(msg, "pgpass") ||
		isAuthenticationFailure(msg)
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

func mapOnlineCLIBoundaryError(err error) error {
	msg := err.Error()
	switch {
	case isAuthenticationFailure(strings.ToLower(msg)):
		return newRuntimeError("authentication failed")
	case strings.Contains(msg, "certificate"):
		return newRuntimeError("TLS handshake failed")
	case strings.Contains(msg, "x509:"):
		return newRuntimeError("TLS certificate verification failed")
	case strings.Contains(msg, "tls:"):
		return newRuntimeError("TLS handshake failed")
	case strings.Contains(msg, "timeout"):
		return newRuntimeError("connection timed out")
	case strings.Contains(msg, "context canceled"):
		return newRuntimeError("request canceled")
	default:
		return newRuntimeError("connection failed")
	}
}
