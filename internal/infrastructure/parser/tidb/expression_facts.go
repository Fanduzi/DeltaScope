// Package tidbparser extracts evaluation-free expression facts from parsed expressions.
// input: TiDB AST expression nodes from column declarations and catalog generation text
// output: spec.ExpressionFacts (references/functions/call-count/features/fingerprint/known) plus ParseCatalogExpressionFacts and MultiValuedCastFacts (the CAST(… AS <T> ARRAY) element family + authored-length bit the member-legality check consumes — FieldType.GetType reports JSON for the array wrapper, so the element byte type/length come from Tp.ArrayType())
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
	"github.com/pingcap/tidb/pkg/parser/opcode"
	_ "github.com/pingcap/tidb/pkg/parser/test_driver"
	tidbtypes "github.com/pingcap/tidb/pkg/parser/types"
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
		TopForm:    expressionTopForm(expr),
		CallCount:  v.calls,
	}
	_, facts.RootParenthesized = expr.(*ast.ParenthesesExpr)
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

// expressionTopForm classifies the paren-transparent root node of one
// expression into the shared spec root-shape vocabulary. The shape is a
// structural fact computed even when the fact record is incomplete —
// subqueries and aggregates classify before their unknown markers do.
func expressionTopForm(expr ast.ExprNode) string {
	switch node := unwrapParenthesesExpr(expr).(type) {
	case *ast.FuncCallExpr:
		return spec.ExpressionTopCall
	case ast.ValueExpr:
		return spec.ExpressionTopLiteral
	case *ast.ColumnNameExpr:
		return spec.ExpressionTopColumn
	case *ast.BinaryOperationExpr:
		// Only the numeric-coercing operators are "arithmetic": TiDB
		// evaluates function-bearing numeric composites and rejects them at
		// the coercion boundary (observed uuid()+1 -> 1292), while
		// comparison/logical roots fold to a boolean literal whose outcome
		// is not the observed negative.
		if numericCoerceBinaryOp(node.Op) {
			return spec.ExpressionTopArithmetic
		}
		return spec.ExpressionTopOther
	case *ast.UnaryOperationExpr:
		return spec.ExpressionTopUnary
	case *ast.CaseExpr:
		return spec.ExpressionTopCase
	case *ast.SubqueryExpr, *ast.ExistsSubqueryExpr, *ast.CompareSubqueryExpr:
		return spec.ExpressionTopSubquery
	case *ast.AggregateFuncExpr:
		return spec.ExpressionTopAggregate
	case nil:
		return ""
	default:
		_ = node
		return spec.ExpressionTopOther
	}
}

// numericCoerceBinaryOp reports whether a binary operator coerces its
// operands to numbers — the family the TiDB SET DEFAULT evaluation path
// rejects for non-numeric function results.
func numericCoerceBinaryOp(op opcode.Op) bool {
	switch op {
	case opcode.Plus, opcode.Minus, opcode.Mul, opcode.Div, opcode.Mod,
		opcode.IntDiv, opcode.LeftShift, opcode.RightShift,
		opcode.And, opcode.Or, opcode.Xor:
		return true
	}
	return false
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
	calls      int
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
		v.calls++
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

// CatalogExpressionIsMultiValued reports whether stored catalog expression
// text is the MySQL multi-valued member form CAST(… AS … ARRAY). The same
// bare-expression grammar as ParseCatalogExpressionFacts applies; anything
// outside it is not a multi-valued member.
func CatalogExpressionIsMultiValued(text string) bool {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return false
	}
	expr, ok := parseSelectExpressionAST(trimmed)
	if !ok {
		decoded, decOK := decodeCatalogEscapes(trimmed)
		if !decOK {
			return false
		}
		expr, ok = parseSelectExpressionAST(decoded)
		if !ok {
			return false
		}
	}
	cast, isCast := unwrapParenthesesExpr(expr).(*ast.FuncCastExpr)
	return isCast && cast.Tp != nil && cast.Tp.IsArray()
}

// MultiValuedCastFacts normalizes a multi-valued member's
// CAST(… AS <T> ARRAY) target into the element family the product
// legality check consumes plus whether the declaration authored a
// length (char/binary targets require one — the unspecified form is a
// product rejection). FieldType.GetType reports JSON for the array
// wrapper, so the element byte type comes from Tp.ArrayType(); flen
// comes off the same element. Element types outside the pinned matrix
// answer "" — the member check turns that into a bounded gap rather
// than guessing legality.
func MultiValuedCastFacts(cast *ast.FuncCastExpr) (family string, lengthSet bool) {
	if cast == nil || cast.Tp == nil || !cast.Tp.IsArray() {
		return "", false
	}
	elem := cast.Tp.ArrayType()
	if elem == nil {
		return "", false
	}
	lengthSet = elem.GetFlen() != tidbtypes.UnspecifiedLength
	switch elem.GetType() {
	case mysql.TypeTiny, mysql.TypeShort, mysql.TypeInt24, mysql.TypeLong, mysql.TypeLonglong:
		if elem.GetFlag()&mysql.UnsignedFlag != 0 {
			return "unsigned", lengthSet
		}
		return "signed", lengthSet
	case mysql.TypeVarchar, mysql.TypeVarString, mysql.TypeString:
		// CAST AS BINARY surfaces as a var-string element with the binary
		// charset/flag; every other character target is the char family.
		if elem.GetFlag()&mysql.BinaryFlag != 0 || strings.EqualFold(elem.GetCharset(), "binary") {
			return "binary", lengthSet
		}
		return "char", lengthSet
	case mysql.TypeDate, mysql.TypeNewDate:
		return "date", lengthSet
	case mysql.TypeDuration:
		return "time", lengthSet
	case mysql.TypeDatetime:
		return "datetime", lengthSet
	case mysql.TypeNewDecimal:
		return "decimal", lengthSet
	case mysql.TypeDouble:
		return "double", lengthSet
	case mysql.TypeFloat:
		return "float", lengthSet
	case mysql.TypeJSON:
		return "json", lengthSet
	case mysql.TypeYear:
		return "year", lengthSet
	}
	return "", false
}

// CatalogMultiValuedCastFacts is the catalog-text twin of
// MultiValuedCastFacts: it parses stored STATISTICS.EXPRESSION text and
// answers the same normalized cast-target facts for a multi-valued member.
// Non-multi-valued or unparseable text answers an empty family — the
// caller already treats that shape as unknown.
func CatalogMultiValuedCastFacts(text string) (family string, lengthSet bool) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return "", false
	}
	expr, ok := parseSelectExpressionAST(trimmed)
	if !ok {
		if decoded, decOK := decodeCatalogEscapes(trimmed); decOK {
			expr, ok = parseSelectExpressionAST(decoded)
		}
	}
	if !ok {
		return "", false
	}
	cast, isCast := unwrapParenthesesExpr(expr).(*ast.FuncCastExpr)
	if !isCast {
		return "", false
	}
	return MultiValuedCastFacts(cast)
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
	// Strip exactly the parentheses layer this function added around the
	// authored text: deeper authored parens stay part of the expression
	// structure so `uuid()` and `(uuid())` keep their distinct roots.
	expr := field.Expr
	if paren, ok := expr.(*ast.ParenthesesExpr); ok && paren.Expr != nil {
		expr = paren.Expr
	}
	inner := expr
	if paren, ok := inner.(*ast.ParenthesesExpr); ok && paren.Expr != nil {
		inner = paren.Expr
	}
	if _, row := inner.(*ast.RowExpr); row {
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
