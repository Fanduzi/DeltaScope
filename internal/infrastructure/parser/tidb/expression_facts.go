// Package tidbparser extracts evaluation-free expression facts from parsed expressions.
// input: TiDB AST expression nodes from column declarations and catalog generation text
// output: spec.ExpressionFacts (references/functions/features/fingerprint/known) plus ParseCatalogExpressionFacts
// pos: shared expression-fact collector used by the column extractor and the MySQL metadata provider
// note: if this file changes, update this header and module README.md.
package tidbparser

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	"github.com/pingcap/tidb/pkg/parser"
	"github.com/pingcap/tidb/pkg/parser/ast"
	"github.com/pingcap/tidb/pkg/parser/format"
	"github.com/pingcap/tidb/pkg/parser/mysql"
	_ "github.com/pingcap/tidb/pkg/parser/test_driver"
)

// expressionFactNodeLimit bounds AST traversal so one pathological expression
// cannot exhaust the audit budget; a limit hit degrades to unknown facts.
const expressionFactNodeLimit = 4096

// recognizedExpressionFunctions is the bounded built-in vocabulary the T07
// audit surface classifies. Anything outside it is an unknown/tool boundary
// (spec.ExpressionFeatureUnknownFunc), never an empty fact.
var recognizedExpressionFunctions = map[string]bool{
	"abs": true, "concat": true, "lower": true, "upper": true,
	"coalesce": true, "ifnull": true,
	"json_extract": true, "json_unquote": true, "json_object": true,
	"json_array": true, "json_quote": true,
	"unhex": true, "sha2": true, "hex": true,
	"uuid": true, "rand": true, "now": true, "current_timestamp": true,
	"sysdate": true, "curdate": true, "curtime": true,
	"utc_timestamp": true, "utc_date": true, "utc_time": true,
	"connection_id": true, "last_insert_id": true,
	"user": true, "current_user": true, "database": true, "schema": true,
	"version": true, "found_rows": true, "row_count": true,
	"benchmark": true, "sleep": true,
	"get_lock": true, "release_lock": true, "load_file": true,
	"uuid_short": true,
}

// expressionFacts computes the structured facts for one parsed expression
// node. It never evaluates the expression; it records what the AST proves.
func expressionFacts(expr ast.ExprNode) spec.ExpressionFacts {
	if expr == nil {
		return spec.ExpressionFacts{Known: false}
	}
	v := &expressionFactVisitor{
		features:  map[string]bool{},
		functions: map[string]bool{},
	}
	v.known = true
	expr.Accept(v)
	facts := spec.ExpressionFacts{
		References: v.references,
		Functions:  sortedKeys(v.functions),
		Features:   sortedKeys(v.features),
	}
	facts.Known = v.known && v.nodes <= expressionFactNodeLimit && !v.limitHit
	if v.limitHit {
		facts.Known = false
	}
	// The fingerprint is only meaningful for a fully traversed expression;
	// unknown or truncated facts never carry one (missing fingerprint is an
	// identity gap, not proof of change).
	if facts.Known {
		facts.Fingerprint = expressionFingerprint(expr)
	}
	return facts
}

// expressionFactVisitor collects column references, callee names, and
// construct markers in declaration order.
type expressionFactVisitor struct {
	references []spec.ColumnReference
	seenRefs   map[spec.ColumnReference]struct{}
	functions  map[string]bool
	features   map[string]bool
	known      bool
	limitHit   bool
	nodes      int
}

func (v *expressionFactVisitor) Enter(in ast.Node) (ast.Node, bool) {
	v.nodes++
	if v.nodes > expressionFactNodeLimit {
		v.limitHit = true
		return in, true
	}
	switch node := in.(type) {
	case *ast.ColumnNameExpr:
		if node.Name == nil || strings.TrimSpace(node.Name.Name.O) == "" {
			v.features[spec.ExpressionFeatureUnknownShape] = true
			v.known = false
			return in, true
		}
		v.addReference(spec.ColumnReference{
			Schema: node.Name.Schema.O,
			Table:  node.Name.Table.O,
			Name:   node.Name.Name.O,
		})
		return in, true
	case *ast.ColumnName:
		return in, true
	case *ast.SubqueryExpr, *ast.ExistsSubqueryExpr, *ast.CompareSubqueryExpr:
		v.features[spec.ExpressionFeatureSubquery] = true
		v.known = false
		return in, true
	case *ast.AggregateFuncExpr:
		v.features[spec.ExpressionFeatureAggregate] = true
		v.known = false
		return in, true
	case *ast.BinaryOperationExpr, *ast.UnaryOperationExpr:
		v.features[spec.ExpressionFeatureArithmetic] = true
	case *ast.FuncCallExpr:
		name := strings.ToLower(node.FnName.O)
		v.functions[name] = true
		if node.Schema.O != "" || !recognizedExpressionFunctions[name] {
			// Schema-qualified callees and functions outside the recognized
			// vocabulary are unverifiable: an unknown/tool boundary, never a
			// silent built-in.
			v.features[spec.ExpressionFeatureUnknownFunc] = true
			v.known = false
		}
	case ast.ParamMarkerExpr, *ast.VariableExpr, *ast.ValuesExpr, *ast.DefaultExpr:
		v.features[spec.ExpressionFeatureUnknownShape] = true
		v.known = false
		return in, true
	case ast.ValueExpr:
		return in, true
	case *ast.ParenthesesExpr, *ast.FuncCastExpr, *ast.CaseExpr, *ast.WhenClause,
		*ast.PatternInExpr, *ast.IsNullExpr, *ast.IsTruthExpr, *ast.BetweenExpr,
		*ast.PatternLikeOrIlikeExpr, *ast.PatternRegexpExpr, *ast.PositionExpr,
		*ast.RowExpr, *ast.SetCollationExpr:
		return in, false
	default:
		// Any expression node outside the recognized allowlist is an
		// unknown shape, not an empty fact.
		v.features[spec.ExpressionFeatureUnknownShape] = true
		v.known = false
		return in, true
	}
	return in, false
}

func (v *expressionFactVisitor) Leave(in ast.Node) (ast.Node, bool) {
	return in, true
}

func (v *expressionFactVisitor) addReference(ref spec.ColumnReference) {
	if v.seenRefs == nil {
		v.seenRefs = map[spec.ColumnReference]struct{}{}
	}
	key := spec.ColumnReference{
		Schema: strings.ToLower(ref.Schema),
		Table:  strings.ToLower(ref.Table),
		Name:   strings.ToLower(ref.Name),
	}
	if _, dup := v.seenRefs[key]; dup {
		return
	}
	v.seenRefs[key] = struct{}{}
	v.references = append(v.references, ref)
}

func sortedKeys(set map[string]bool) []string {
	if len(set) == 0 {
		return nil
	}
	keys := make([]string, 0, len(set))
	for key := range set {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// expressionFingerprint restores the expression through the AST's own
// Restore with a normalized flag set (lowercase identifiers/keywords, single
// quoted escaped literals, stable operator spacing), then hashes the result.
// Only fully traversed, known facts receive a fingerprint; equal
// fingerprints prove structural sameness, missing or unequal ones prove
// nothing.
var expressionFingerprintFlags = format.RestoreStringSingleQuotes |
	format.RestoreStringEscapeBackslash |
	format.RestoreKeyWordLowercase |
	format.RestoreNameLowercase |
	format.RestoreNameBackQuotes |
	format.RestoreSpacesAroundBinaryOperation |
	format.RestoreBracketAroundBinaryOperation

func expressionFingerprint(expr ast.ExprNode) string {
	expr = unwrapParenthesesExpr(expr)
	if expr == nil {
		return ""
	}
	var builder strings.Builder
	ctx := format.NewRestoreCtx(expressionFingerprintFlags, &builder)
	if err := expr.Restore(ctx); err != nil {
		return ""
	}
	sum := sha256.Sum256([]byte(builder.String()))
	return hex.EncodeToString(sum[:])
}

// ParseCatalogExpressionFacts parses a stored catalog expression (the
// information_schema GENERATION_EXPRESSION / expression-default text) into
// the same structured facts as declaration parsing. It tries the raw text
// first; only a syntactically valid escaped SQL-string representation is
// decoded and reparsed. Unparseable input returns unknown facts — never an
// empty known record.
func ParseCatalogExpressionFacts(text string) spec.ExpressionFacts {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return spec.ExpressionFacts{Known: false}
	}
	if facts, ok := parseCatalogExpression(trimmed); ok {
		return facts
	}
	if decoded, ok := decodeCatalogEscapes(trimmed); ok && decoded != trimmed {
		if facts, ok := parseCatalogExpression(decoded); ok {
			return facts
		}
	}
	return spec.ExpressionFacts{Known: false}
}

// parseCatalogExpression requires exactly one bare expression: no FROM,
// WHERE, GROUP/HAVING, ORDER, LIMIT, aliases, wildcards, or set operations.
// Anything broader is not a stored catalog expression.
func parseCatalogExpression(text string) (spec.ExpressionFacts, bool) {
	expr, ok := parseSelectExpressionAST(text)
	if !ok {
		return spec.ExpressionFacts{}, false
	}
	return expressionFacts(expr), true
}

// parseSelectExpressionAST parses text as exactly one bare expression field:
// it is wrapped as `SELECT (<text>)` so trailing clause tokens, aliases, and
// second fields cannot parse, then the SELECT itself must be a single plain
// unqualified projection — no modifiers (ALL/DISTINCT/priority/SQL_*),
// hints, FROM/WHERE/GROUP/HAVING/ORDER/LIMIT, locks, set operations, WITH, or
// window specs.
func parseSelectExpressionAST(text string) (ast.ExprNode, bool) {
	stmts, _, err := parser.New().Parse("SELECT ("+text+")", "", "")
	if err != nil || len(stmts) != 1 {
		return nil, false
	}
	selectStmt, ok := stmts[0].(*ast.SelectStmt)
	if !ok || selectStmt.Kind != ast.SelectStmtKindSelect || selectStmt.IsInBraces ||
		selectStmt.AfterSetOperator != nil || selectStmt.With != nil {
		return nil, false
	}
	if opts := selectStmt.SelectStmtOpts; opts != nil {
		// SQLCache is the parser's default-on bit, not an authored modifier.
		if opts.Distinct || opts.ExplicitAll || opts.SQLBigResult || opts.SQLBufferResult ||
			opts.SQLSmallResult || opts.CalcFoundRows || opts.StraightJoin ||
			opts.Priority != mysql.NoPriority || len(opts.TableHints) > 0 {
			return nil, false
		}
	}
	if selectStmt.From != nil || selectStmt.Where != nil || selectStmt.GroupBy != nil ||
		selectStmt.Having != nil || selectStmt.OrderBy != nil || selectStmt.Limit != nil ||
		selectStmt.LockInfo != nil || selectStmt.SelectIntoOpt != nil ||
		len(selectStmt.WindowSpecs) > 0 || len(selectStmt.Lists) > 0 || len(selectStmt.TableHints) > 0 {
		return nil, false
	}
	if selectStmt.Fields == nil || len(selectStmt.Fields.Fields) != 1 {
		return nil, false
	}
	field := selectStmt.Fields.Fields[0]
	if field == nil || field.Expr == nil || field.WildCard != nil || field.AsName.O != "" {
		return nil, false
	}
	expr := unwrapParenthesesExpr(field.Expr)
	if _, row := expr.(*ast.RowExpr); row {
		// A row constructor smuggles a second field — not one scalar
		// expression.
		return nil, false
	}
	return expr, true
}

// decodeCatalogEscapes decodes the recognized catalog escaped-string layer:
// MySQL writes stored expression text with quote delimiters and backslashes
// backslash-escaped (`json_object(\'k\',\'v\')`). The prescan accepts only
// the \' \" \\ family and rejects raw single quotes; decoding itself runs
// through the parser's own string-literal grammar — the text wrapped as one
// single-quoted literal must parse as exactly one bare expression that is a
// string ValueExpr. Any other shape aborts the decode.
func decodeCatalogEscapes(text string) (string, bool) {
	if !strings.ContainsRune(text, '\\') {
		return "", false
	}
	for i := 0; i < len(text); i++ {
		switch text[i] {
		case '\'':
			// An unescaped single quote would terminate the wrapper
			// literal early — not the recognized encoding.
			return "", false
		case '\\':
			if i+1 >= len(text) {
				return "", false
			}
			switch text[i+1] {
			case '\'', '"', '\\':
				i++
			default:
				return "", false
			}
		}
	}
	expr, ok := parseSelectExpressionAST("'" + text + "'")
	if !ok {
		return "", false
	}
	literal, ok := expr.(ast.ValueExpr)
	if !ok {
		return "", false
	}
	decoded, ok := literal.GetValue().(string)
	if !ok || decoded == text {
		return "", false
	}
	return decoded, true
}
