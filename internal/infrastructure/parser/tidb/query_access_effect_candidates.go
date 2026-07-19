// Package tidbparser defines bounded internal effect-candidate facts.
// input: TiDB AST function-like nodes and resolved parser scope
// output: ordered, untrusted candidate facts for application-internal gateways
// pos: parser candidate contract; never a public result contract
package tidbparser

import (
	"fmt"
	"strings"

	"github.com/pingcap/tidb/pkg/parser/ast"
)

type EffectCandidateKind string

const (
	EffectCandidateOperator EffectCandidateKind = "operator"
	EffectCandidateFunction EffectCandidateKind = "function"
	EffectCandidateCast     EffectCandidateKind = "cast"
	EffectCandidateUnknown  EffectCandidateKind = "unknown"
)

type OperandKindHint string

const (
	OperandKindColumn   OperandKindHint = "column"
	OperandKindConst    OperandKindHint = "const"
	OperandKindParam    OperandKindHint = "param"
	OperandKindStar     OperandKindHint = "star"
	OperandKindExpr     OperandKindHint = "expr"
	OperandKindSubquery OperandKindHint = "subquery"
	OperandKindUnknown  OperandKindHint = "unknown"
)

type OperandColumnRef struct {
	Schema string
	Table  string
	Column string
}

type EffectCandidate struct {
	Kind                      EffectCandidateKind
	Ordinal                   int
	NamePath                  []string
	OriginalNamePath          []string
	ExplicitSchema            bool
	IsQuoted                  bool
	Canonical                 bool
	Ambiguous                 bool
	ParserClassification      string
	UnqualifiedRelation       bool
	Arity                     int
	OperandKinds              []OperandKindHint
	OperandColumnRefs         []OperandColumnRef
	IsAggregate               bool
	HasWindow                 bool
	HasFilter                 bool
	HasDistinct               bool
	HasAggOrder               bool
	HasWithinGroup            bool
	HasFrame                  bool
	HasNamedWindow            bool
	HasWindowPartition        bool
	HasWindowOrder            bool
	WindowPartitionKinds      []OperandKindHint
	WindowOrderKinds          []OperandKindHint
	WindowFrameKinds          []OperandKindHint
	WindowPartitionColumnRefs []OperandColumnRef
	WindowOrderColumnRefs     []OperandColumnRef
	TargetTypePath            []string
}

type effectCandidateCollector struct {
	defaultSchema          string
	sql                    string
	candidates             []EffectCandidate
	nextOrdinal            int
	hasUnqualifiedRelation bool
}

func newEffectCandidateCollector(defaultSchema, sql string) *effectCandidateCollector {
	return &effectCandidateCollector{
		defaultSchema: defaultSchema,
		sql:           sql,
		candidates:    make([]EffectCandidate, 0),
	}
}

func (c *effectCandidateCollector) append(candidate EffectCandidate) {
	candidate.Ordinal = c.nextOrdinal
	c.nextOrdinal++
	candidate.UnqualifiedRelation = candidate.UnqualifiedRelation || c.hasUnqualifiedRelation
	candidate.NamePath = append([]string(nil), candidate.NamePath...)
	candidate.OriginalNamePath = append([]string(nil), candidate.OriginalNamePath...)
	candidate.OperandKinds = append([]OperandKindHint(nil), candidate.OperandKinds...)
	candidate.OperandColumnRefs = append([]OperandColumnRef(nil), candidate.OperandColumnRefs...)
	candidate.WindowPartitionKinds = append([]OperandKindHint(nil), candidate.WindowPartitionKinds...)
	candidate.WindowOrderKinds = append([]OperandKindHint(nil), candidate.WindowOrderKinds...)
	candidate.WindowFrameKinds = append([]OperandKindHint(nil), candidate.WindowFrameKinds...)
	candidate.WindowPartitionColumnRefs = append([]OperandColumnRef(nil), candidate.WindowPartitionColumnRefs...)
	candidate.WindowOrderColumnRefs = append([]OperandColumnRef(nil), candidate.WindowOrderColumnRefs...)
	candidate.TargetTypePath = append([]string(nil), candidate.TargetTypePath...)
	c.candidates = append(c.candidates, candidate)
}

func (c *effectCandidateCollector) appendUnsupported() {
	c.append(EffectCandidate{
		Kind:                 EffectCandidateUnknown,
		OperandKinds:         []OperandKindHint{OperandKindUnknown},
		ParserClassification: "unsupported_traversal",
		Ambiguous:            true,
	})
}

func candidateOperandKind(expr ast.ExprNode) OperandKindHint {
	if expr == nil {
		return OperandKindUnknown
	}
	if column, ok := expr.(*ast.ColumnNameExpr); ok {
		if column.Name != nil && column.Name.Name.L == "*" {
			return OperandKindStar
		}
		return OperandKindColumn
	}
	if _, ok := expr.(ast.ParamMarkerExpr); ok {
		return OperandKindParam
	}
	if _, ok := expr.(ast.ValueExpr); ok {
		if value, ok := expr.(ast.ValueExpr); ok && fmt.Sprint(value.GetValue()) == "*" {
			return OperandKindStar
		}
		if value, ok := expr.(ast.ValueExpr); ok && value.GetString() == "*" {
			return OperandKindStar
		}
		if expr.Text() == "*" {
			return OperandKindStar
		}
		return OperandKindConst
	}
	if _, ok := expr.(*ast.SubqueryExpr); ok {
		return OperandKindSubquery
	}
	return OperandKindExpr
}

func candidateNamePath(schema, name ast.CIStr) ([]string, []string, bool) {
	path := make([]string, 0, 2)
	original := make([]string, 0, 2)
	if schema.L != "" {
		path = append(path, schema.L)
		original = append(original, schema.O)
	}
	if name.L != "" {
		path = append(path, name.L)
		original = append(original, name.O)
	}
	return path, original, schema.L != ""
}

func candidateCallFacts(originalText string, originalPath []string, explicitSchema bool, parserClass string) (bool, bool, bool) {
	quoted := false
	open := strings.Index(originalText, "(")
	if open >= 0 {
		quoted = strings.Contains(originalText[:open], "`")
	}
	canonical := originalText != "" && !quoted && !explicitSchema
	ambiguous := originalText == "" || quoted || explicitSchema
	if open <= 0 || strings.TrimSpace(originalText[:open]) != originalText[:open] || strings.ContainsAny(originalText[:open], "/*") {
		canonical = false
		ambiguous = true
	}
	if parserClass == "keyword" {
		canonical = false
		ambiguous = true
	}
	if len(originalPath) == 0 {
		ambiguous = true
		canonical = false
	}
	if parserClass == "unsupported_traversal" {
		ambiguous = true
		canonical = false
	}
	return quoted, canonical, ambiguous
}

func (c *effectCandidateCollector) quotedCall(node ast.Node) bool {
	if c == nil || node == nil {
		return false
	}
	position := node.OriginTextPosition()
	if position <= 0 || position >= len(c.sql) {
		return false
	}
	rest := c.sql[position:]
	end := strings.IndexByte(rest, '(')
	if end < 0 {
		return false
	}
	prefix := rest[:end]
	return strings.ContainsAny(prefix, "`\"")
}

func (c *effectCandidateCollector) callSourceText(node ast.Node) string {
	if node == nil {
		return ""
	}
	if text := node.OriginalText(); text != "" {
		return text
	}
	if c == nil {
		return ""
	}
	position := node.OriginTextPosition()
	if position < 0 || position >= len(c.sql) {
		return ""
	}
	rest := c.sql[position:]
	open := strings.IndexByte(rest, '(')
	if open < 0 {
		return ""
	}
	return rest[:open+1]
}
