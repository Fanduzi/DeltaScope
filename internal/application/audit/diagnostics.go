package audit

import "github.com/Fanduzi/DeltaScope/internal/domain/spec"

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
)

func newParserErrorDiagnostic(dialect spec.Dialect) spec.Diagnostic {
	return spec.Diagnostic{
		Classification: DiagnosticParserError,
		Reason:         parserErrorReason,
		ActionHint:     ParserErrorActionHint,
		Audited:        false,
		Dialect:        string(dialect),
	}
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
