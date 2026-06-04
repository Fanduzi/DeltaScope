package audit

import (
	"strings"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

const (
	// DiagnosticParserError classifies parser-error outcomes.
	DiagnosticParserError = "parser_error"
	// DiagnosticUnsupportedStatement classifies structured unsupported outcomes.
	DiagnosticUnsupportedStatement = "unsupported_statement"

	// ParserErrorActionHint is the generic safe next step for parser-error diagnostics.
	ParserErrorActionHint = "verify the selected dialect and syntax, split multi-statement input if needed, or upgrade DeltaScope when parser support becomes available"
	// UnsupportedActionHint is the generic safe next step for unsupported-statement diagnostics.
	UnsupportedActionHint = "treat this statement as not covered by DeltaScope policy checks; review it manually or track support in a future DeltaScope release"

	// parserErrorReason is the v0.220.0 standard parser-error diagnostic reason.
	parserErrorReason = "statement was not audited because the selected dialect parser could not parse it; no audit findings were inferred"

	// DiagnosticGuidanceParserUpgradeCandidate identifies parser-error cases that would become
	// parseable after an upstream parser/library upgrade.
	DiagnosticGuidanceParserUpgradeCandidate = "parser_upgrade_candidate"

	// ParserUpgradeCandidateEvidenceRef is the stable GitHub documentation URL for parser-upgrade
	// candidate evidence.
	ParserUpgradeCandidateEvidenceRef = "https://github.com/Fanduzi/DeltaScope/blob/main/docs/reference/cli.md#parser-upgrade-candidate-evidence-v02500"
)

// mysqlParserUpgradePrefixes lists fixed SQL keyword prefixes that are parser-upgrade candidates
// for MySQL/TiDB dialects. Only the leading keyword sequence is matched; no object names or body
// content are examined.
var mysqlParserUpgradePrefixes = []string{
	"ALTER VIEW ",
	"ALTER PROCEDURE ",
	"CREATE FUNCTION ",
	"ALTER FUNCTION ",
	"DROP FUNCTION ",
	"CREATE TABLESPACE ",
	"ALTER TABLESPACE ",
	"DROP TABLESPACE ",
	"CREATE RESOURCE GROUP ",
	"ALTER RESOURCE GROUP ",
}

// pgParserUpgradePrefixes lists fixed SQL keyword prefixes that are parser-upgrade candidates
// for the PostgreSQL dialect.
var pgParserUpgradePrefixes = []string{
	"DROP SUBSCRIPTION ",
}

// pgParserUpgradeContains lists fixed keyword substrings within ALTER TABLE statements that are
// parser-upgrade candidates for PostgreSQL. These appear as sub-clauses, not statement prefixes.
var pgParserUpgradeContains = []string{
	"NOT VALID",
	"NOT ENFORCED",
	" NO INHERIT",
	" INHERIT",
}

func newParserErrorDiagnostic(dialect spec.Dialect) spec.Diagnostic {
	return spec.Diagnostic{
		Classification: DiagnosticParserError,
		Reason:         parserErrorReason,
		ActionHint:     ParserErrorActionHint,
		Audited:        false,
		Dialect:        string(dialect),
	}
}

// newParserErrorDiagnosticWithGuidance creates a parser-error diagnostic and attempts to classify
// the unsupported boundary using safe keyword matching on the SQL form. sql is used only for
// classification; no SQL content is stored in the returned diagnostic.
func newParserErrorDiagnosticWithGuidance(dialect spec.Dialect, sql string) spec.Diagnostic {
	d := newParserErrorDiagnostic(dialect)
	d.GuidanceCode, d.EvidenceRef = classifyParserErrorGuidance(sql, dialect)
	return d
}

func newUnsupportedStatementDiagnostic(dialect spec.Dialect) spec.Diagnostic {
	return spec.Diagnostic{
		Classification: DiagnosticUnsupportedStatement,
		Reason:         "DeltaScope recognized this statement or feature but does not audit it yet",
		ActionHint:     UnsupportedActionHint,
		Audited:        false,
		Dialect:        string(dialect),
	}
}

// classifyParserErrorGuidance returns a guidance code and evidence ref for the parser-error SQL
// using safe, fixed-keyword matching. It never embeds SQL content, object names, or parser
// internals into the returned values.
func classifyParserErrorGuidance(sql string, dialect spec.Dialect) (string, string) {
	upper := strings.ToUpper(sql)

	switch dialect {
	case spec.DialectMySQL, spec.DialectTiDB:
		for _, prefix := range mysqlParserUpgradePrefixes {
			if strings.HasPrefix(upper, prefix) {
				return DiagnosticGuidanceParserUpgradeCandidate, ParserUpgradeCandidateEvidenceRef
			}
		}
	case spec.DialectPostgreSQL:
		for _, prefix := range pgParserUpgradePrefixes {
			if strings.HasPrefix(upper, prefix) {
				return DiagnosticGuidanceParserUpgradeCandidate, ParserUpgradeCandidateEvidenceRef
			}
		}
		for _, substr := range pgParserUpgradeContains {
			if strings.Contains(upper, substr) {
				return DiagnosticGuidanceParserUpgradeCandidate, ParserUpgradeCandidateEvidenceRef
			}
		}
	}

	return "", ""
}
