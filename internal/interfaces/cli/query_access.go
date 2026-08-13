// Package cli exposes the command-line adapter for DeltaScope.
// input: query-access command flags including connection flags, SQL text from flags/files/stdin, and the unified public online query access API
// output: rendered offline or identity-routed online query access results in JSON format, exit-code mapping for CI integration
// pos: CLI query-access command implementation above offline analysis and the opaque unified online session boundary
// note: if this file changes, update this header and module README.md.
package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/Fanduzi/DeltaScope/internal/application/online"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	"github.com/Fanduzi/DeltaScope/pkg/deltascope"
	"github.com/spf13/cobra"
)

var (
	openOnlineSession                   = online.OpenSession
	newOnlineQueryAccessSessionFromConn = deltascope.NewOnlineQueryAccessSessionFromConn
	analyzeOnlineQueryAccessWithSession = deltascope.AnalyzeOnlineQueryAccessWithSession
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
		Long: "Analyze SQL to determine read classification, admission, and permission requirements.\n" +
			"When connection flags are present, DeltaScope uses online mode with server-identity-derived capability.",
		Example: "Offline example:\n" +
			"  deltascope query-access analyze --sql \"SELECT id, name FROM users WHERE id = 1\" --dialect mysql\n" +
			"  deltascope query-access analyze --file ./query.sql --dialect postgresql --mode projection_only\n\n" +
			"Online example:\n" +
			"  deltascope query-access analyze --sql \"SELECT id, name FROM users WHERE id = 1\" --host 127.0.0.1 --port 3306 --user root --ask-password --schema app",
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

			connection, err := resolveConnectionOptions(cmd, options)
			if err != nil {
				*exitCode = exitQueryAccessUsageError
				return err
			}

			if !connection.Enabled() {
				return runQueryAccessOffline(cmd, sql, dialect, accessMode, defaultSchema, exitCode)
			}
			return runQueryAccessOnline(cmd, sql, dialect, accessMode, defaultSchema, connection, exitCode)
		},
	}

	cmd.Flags().StringVar(&inlineSQL, "sql", "", "inline SQL text to analyze")
	cmd.Flags().StringVar(&filePath, "file", "", "path to a SQL file to analyze")
	cmd.Flags().StringVar(&mode, "mode", "", "analysis mode: strict or projection_only (default strict)")
	cmd.Flags().StringVar(&defaultSchema, "default-schema", "", "default schema for unqualified references")
	cmd.Flags().BoolP("help", "", false, "help for analyze")
	cmd.Flags().StringVarP(&options.Host, "host", "h", "", "database host for online query access")
	cmd.Flags().IntVarP(&options.Port, "port", "P", options.Port, "database port for online query access")
	cmd.Flags().StringVarP(&options.User, "user", "u", "", "database user for online query access")
	cmd.Flags().StringVar(&options.PasswordEnv, "password-env", "", "environment variable that contains the database password for online query access")
	cmd.Flags().StringVar(&options.PasswordFile, "password-file", "", "file path that contains the database password for online query access")
	cmd.Flags().BoolVar(&options.AskPassword, "ask-password", false, "prompt for a database password without echo")
	cmd.Flags().StringVarP(&options.Schema, "schema", "D", "", "database schema for online query access")
	cmd.Flags().StringVar(&options.Database, "database", "", "database name for online query access (PostgreSQL)")
	cmd.Flags().StringVarP(&options.Socket, "socket", "S", "", "database Unix socket for online query access")
	cmd.Flags().StringVar(&options.MetadataConnectTimeout, "metadata-connect-timeout", "", "connection timeout for online query access, for example 5s or 500ms")
	cmd.Flags().StringVar(&options.TLSMode, "tls-mode", "disabled", "TLS mode for database connection: disabled or enabled")
	cmd.Flags().StringVar(&options.TLSCAFile, "tls-ca-file", "", "path to TLS CA certificate PEM file (requires tls-mode=enabled)")
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
		return exitQueryAccessIndeterminate
	}
}

func runQueryAccessOffline(cmd *cobra.Command, sql string, dialect spec.Dialect, accessMode deltascope.QueryAccessMode, defaultSchema string, exitCode *int) error {
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

	return writeQueryAccessResult(cmd, result, exitCode)
}

func buildOnlineSessionConfig(connection auditConnectionOptions, dialect spec.Dialect) online.SessionConfig {
	cfg := online.SessionConfig{
		Host:           connection.Host,
		Port:           connection.Port,
		Socket:         connection.Socket,
		User:           connection.User,
		Password:       connection.Password,
		Schema:         connection.Schema,
		Dialect:        string(dialect),
		ConnectTimeout: connection.ConnectTimeout,
		TLSMode:        connection.TLSMode,
		CACert:         connection.CACert,
	}
	if dialect == spec.DialectPostgreSQL {
		cfg.Database = connection.Database
	}
	return cfg
}

func runQueryAccessOnline(cmd *cobra.Command, sql string, dialect spec.Dialect, accessMode deltascope.QueryAccessMode, defaultSchema string, connection auditConnectionOptions, exitCode *int) error {
	sessionCfg := buildOnlineSessionConfig(connection, dialect)

	session, err := openOnlineSession(cmd.Context(), sessionCfg)
	if err != nil {
		*exitCode = exitQueryAccessUsageError
		return mapOnlineCLIBoundaryError(err)
	}
	defer session.Close()

	queryAccessSession, err := newOnlineQueryAccessSessionFromConn(cmd.Context(), session.Conn)
	if err != nil {
		*exitCode = exitQueryAccessUsageError
		return mapQueryAccessSessionConstructorError(cmd.Context(), dialect, session.Conn, err)
	}

	result, err := analyzeOnlineQueryAccessWithSession(cmd.Context(), queryAccessSession, deltascope.QueryAccessRequest{
		SQL:           sql,
		Mode:          accessMode,
		DefaultSchema: strings.TrimSpace(defaultSchema),
	})
	if err != nil {
		*exitCode = exitQueryAccessUsageError
		return err
	}

	return writeQueryAccessResult(cmd, result, exitCode)
}

func mapQueryAccessSessionConstructorError(ctx context.Context, dialect spec.Dialect, conn *sql.Conn, err error) error {
	if dialect != spec.DialectPostgreSQL {
		return deltascope.ErrMySQLTiDBQueryAccessSessionUnavailable
	}
	if errors.Is(err, deltascope.ErrOnlineQueryAccessCapabilityUnsupported) {
		return errors.New("server identity is not PostgreSQL 17")
	}
	if conn == nil || conn.PingContext(ctx) != nil {
		return errors.New("postgresql session: connection is not alive")
	}
	return errors.New("server identity is not PostgreSQL 17")
}

func writeQueryAccessResult(cmd *cobra.Command, result *deltascope.QueryAccessResult, exitCode *int) error {
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
}
