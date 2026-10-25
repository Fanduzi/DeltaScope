// Package tidbparser recovers bounded grammar gaps the locked TiDB grammar rejects.
package tidbparser

import (
	"strings"

	"github.com/pingcap/tidb/pkg/parser"
	"github.com/pingcap/tidb/pkg/parser/ast"
	"github.com/pingcap/tidb/pkg/parser/charset"
)

// supplementedStatement is the adapter-owned carrier for facts the locked
// grammar cannot represent. The embedded node is a real ast.StmtNode —
// parsed, or for the visibility setter a statically validated ALTER identity —
// and the supplements are typed adapter data bound to successfully parsed
// table/column identities, never fabricated option enums.
type supplementedStatement struct {
	ast.StmtNode
	supp *statementSupplements
}

// statementSupplements carries the typed facts one statement's supplement
// sites recovered: declared column visibility keyed by parsed column name,
// authored `DEFAULT (<expr>)` paren presence keyed by column name, and the
// visibility value of a narrow `ALTER COLUMN c SET VISIBLE|INVISIBLE`
// statement.
type statementSupplements struct {
	visibility       map[string]string
	setterVisibility string
	defaultParens    map[string]bool
}

type sqlTokKind byte

const (
	tokWord   sqlTokKind = iota
	tokQIdent            // `backtick` identifier
	tokStr               // 'single' or "double" string literal
	tokPunct             // one punctuation byte
)

// sqlTok is one lexical token. Comments and whitespace never become tokens,
// so `DEFAULT /* c */ (` reads as adjacent DEFAULT + '('.
type sqlTok struct {
	kind       sqlTokKind
	start, end int
}

// tokenizeSQL lexes the input without interpreting it: strings and quoted
// identifiers are opaque runs, comments are dropped, and every other run is a
// word or a single punctuation token.
func tokenizeSQL(sqlText string) []sqlTok {
	var toks []sqlTok
	for i := 0; i < len(sqlText); {
		switch sqlText[i] {
		case '\'', '"', '`':
			end := skipQuoted(sqlText, i)
			kind := tokQIdent
			if sqlText[i] != '`' {
				kind = tokStr
			}
			toks = append(toks, sqlTok{kind: kind, start: i, end: end})
			i = end
		case '#':
			i = skipLineComment(sqlText, i)
		case '-':
			if i+2 < len(sqlText) && sqlText[i+1] == '-' && isSQLSpace(sqlText[i+2]) {
				i = skipLineComment(sqlText, i)
			} else {
				toks = append(toks, sqlTok{kind: tokPunct, start: i, end: i + 1})
				i++
			}
		case '/':
			if i+1 < len(sqlText) && sqlText[i+1] == '*' {
				i = skipBlockComment(sqlText, i)
			} else {
				toks = append(toks, sqlTok{kind: tokPunct, start: i, end: i + 1})
				i++
			}
		default:
			if isSQLWordByte(sqlText[i]) {
				end := i
				for end < len(sqlText) && isSQLWordByte(sqlText[end]) {
					end++
				}
				toks = append(toks, sqlTok{kind: tokWord, start: i, end: end})
				i = end
				continue
			}
			if isSQLSpace(sqlText[i]) {
				i++
				continue
			}
			toks = append(toks, sqlTok{kind: tokPunct, start: i, end: i + 1})
			i++
		}
	}
	return toks
}

func isSQLSpace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r' || b == '\f' || b == '\v'
}

func (t sqlTok) text(sqlText string) string { return sqlText[t.start:t.end] }

func (t sqlTok) wordIs(sqlText, word string) bool {
	return t.kind == tokWord && strings.EqualFold(t.text(sqlText), word)
}

// identText returns the identifier's bare name lowercased: backticks are
// stripped and doubled backticks unescaped, matching ColumnName.Name.L.
func (t sqlTok) identText(sqlText string) string {
	raw := t.text(sqlText)
	if t.kind == tokQIdent {
		raw = strings.ReplaceAll(raw[1:len(raw)-1], "``", "`")
	}
	return strings.ToLower(raw)
}

// defaultExprSite marks one `DEFAULT (<expr>)` occurrence in a column-option
// or SET DEFAULT position. open is the `(` offset and innerStart/innerEnd
// bound the inner expression bytes; bound tracks one-to-one placeholder
// consumption after the rewrite parse. column is the declared column name
// the site belongs to — the authored parens fact rides to the extractor on
// this identity because the grammar absorbs the outer wrapper.
type defaultExprSite struct {
	open       int
	innerStart int
	innerEnd   int
	column     string
	bound      bool
}

// columnVisibilityFact binds one declared VISIBLE/INVISIBLE attribute to the
// column name its column definition declares. tok is the keyword token index
// inside the statement span, blanked for the rewrite.
type columnVisibilityFact struct {
	column     string
	visibility string
	tok        int
}

// stmtSupplementPlan is one statement span's supplement work. byteEnd reaches
// through the statement delimiter so recovered statements carry the same raw
// text as base-parsed ones; contentEnd stops at the last content token so the
// same-width rewrite never erases the `;` separator.
type stmtSupplementPlan struct {
	byteStart  int
	byteEnd    int
	contentEnd int
	defaults   []defaultExprSite
	visibility []columnVisibilityFact
	setter     *visibilitySetterFact
	invalid    bool
}

func (p *stmtSupplementPlan) hasWork() bool {
	return len(p.defaults) > 0 || len(p.visibility) > 0 || p.setter != nil
}

// visibilitySetterFact is the narrow whole-statement form
// `ALTER TABLE qname ALTER [COLUMN] name SET VISIBLE|INVISIBLE` with
// statically validated table and column identifiers.
type visibilitySetterFact struct {
	schema     string
	table      string
	column     string
	visibility string
}

// supplementPlan is the whole input's supplement work: one entry per
// top-level statement span, in order.
type supplementPlan struct {
	toks    []sqlTok
	stmts   []stmtSupplementPlan
	hasWork bool
}

// planAdvancedSupplements runs the structural supplement analysis before the
// base parse. It recognizes column-option positions in CREATE TABLE and ALTER
// ADD/MODIFY/CHANGE column definitions, the `ALTER [COLUMN] name SET
// DEFAULT (<expr>)` site, and the narrow `SET VISIBLE|INVISIBLE` setter —
// nothing else. Sites outside those positions and shapes the narrow grammar
// cannot prove stay untouched for the base grammar to accept or reject.
func planAdvancedSupplements(sqlText string) supplementPlan {
	toks := tokenizeSQL(sqlText)
	plan := supplementPlan{toks: toks}
	for lo := 0; lo < len(toks); {
		hi := lo
		for hi < len(toks) && !(toks[hi].kind == tokPunct && toks[hi].text(sqlText) == ";") {
			hi++
		}
		if hi > lo {
			stmt := analyzeSupplementStatement(toks, sqlText, lo, hi)
			stmt.byteStart, stmt.byteEnd = toks[lo].start, toks[hi-1].end
			stmt.contentEnd = toks[hi-1].end
			if hi < len(toks) {
				// The parser keeps the statement delimiter in its raw text;
				// supplemented statements must carry the same span.
				stmt.byteEnd = toks[hi].end
			}
			plan.stmts = append(plan.stmts, stmt)
			if stmt.hasWork() {
				plan.hasWork = true
			}
		}
		lo = hi + 1
	}
	return plan
}

// analyzeStatement recognizes the statement shapes the supplement covers and
// collects same-width rewrites plus typed facts for one span.
func analyzeSupplementStatement(toks []sqlTok, sqlText string, lo, hi int) stmtSupplementPlan {
	plan := stmtSupplementPlan{}
	i := lo
	switch {
	case toks[i].wordIs(sqlText, "create"):
		i++
		for i < hi && (toks[i].wordIs(sqlText, "temporary") || toks[i].wordIs(sqlText, "global") || toks[i].wordIs(sqlText, "local")) {
			i++
		}
		if i >= hi || !toks[i].wordIs(sqlText, "table") {
			return plan
		}
		i++
		if i+2 < hi && toks[i].wordIs(sqlText, "if") && toks[i+1].wordIs(sqlText, "not") && toks[i+2].wordIs(sqlText, "exists") {
			i += 3
		}
		// Qualified table name, then the column-definition list must open
		// immediately; CREATE TABLE ... AS/LIKE forms carry none.
		i = skipQualifiedName(toks, sqlText, i, hi)
		if i < hi && toks[i].kind == tokPunct && toks[i].text(sqlText) == "(" {
			scanColumnList(toks, sqlText, i, hi, &plan)
		}
	case toks[i].wordIs(sqlText, "alter"):
		i++
		if i >= hi || !toks[i].wordIs(sqlText, "table") {
			return plan
		}
		i++
		// The visibility setter is only the narrow whole-statement form; a
		// multi-spec statement containing SET VISIBLE is never supplemented.
		if setter, ok := matchVisibilitySetter(toks, sqlText, i, hi); ok {
			plan.setter = setter
			return plan
		}
		i = skipQualifiedName(toks, sqlText, i, hi)
		for i < hi {
			specEnd := i
			depth := 0
			for specEnd < hi {
				if toks[specEnd].kind == tokPunct {
					switch toks[specEnd].text(sqlText) {
					case "(":
						depth++
					case ")":
						depth--
					case ",":
						if depth == 0 {
							goto specDone
						}
					}
				}
				specEnd++
			}
		specDone:
			analyzeAlterSpec(toks, sqlText, i, specEnd, &plan)
			i = specEnd + 1
		}
	}
	return plan
}

// matchVisibilitySetter validates the whole span as
// `ALTER TABLE qname ALTER [COLUMN] name SET VISIBLE|INVISIBLE` — the only
// visibility setter form the supplement owns.
func matchVisibilitySetter(toks []sqlTok, sqlText string, i, hi int) (*visibilitySetterFact, bool) {
	parts := make([]string, 0, 2)
	for i < hi && isIdentToken(toks[i]) {
		parts = append(parts, toks[i].identText(sqlText))
		i++
		if i+1 < hi && toks[i].kind == tokPunct && toks[i].text(sqlText) == "." && isIdentToken(toks[i+1]) {
			i++
			continue
		}
		break
	}
	if len(parts) < 1 || len(parts) > 2 {
		return nil, false
	}
	fact := &visibilitySetterFact{table: parts[len(parts)-1]}
	if len(parts) == 2 {
		fact.schema = parts[0]
	}
	if i >= hi || !toks[i].wordIs(sqlText, "alter") {
		return nil, false
	}
	i++
	if i < hi && toks[i].wordIs(sqlText, "column") {
		i++
	}
	if i >= hi || !isIdentToken(toks[i]) {
		return nil, false
	}
	fact.column = toks[i].identText(sqlText)
	i++
	if i >= hi || !toks[i].wordIs(sqlText, "set") {
		return nil, false
	}
	i++
	if i+1 != hi || !(toks[i].wordIs(sqlText, "visible") || toks[i].wordIs(sqlText, "invisible")) {
		return nil, false
	}
	fact.visibility = strings.ToLower(toks[i].text(sqlText))
	return fact, true
}

// skipQualifiedName consumes `name[.name[.name]]` and returns the index after
// the chain.
func skipQualifiedName(toks []sqlTok, sqlText string, i, hi int) int {
	for i < hi && isIdentToken(toks[i]) {
		i++
		if i+1 < hi && toks[i].kind == tokPunct && toks[i].text(sqlText) == "." && isIdentToken(toks[i+1]) {
			i++
			continue
		}
		return i
	}
	return i
}

// analyzeAlterSpec scans one ALTER TABLE spec. ADD/MODIFY/CHANGE column
// definitions share the column-option scanner; the ALTER COLUMN setter is
// matched on its token shape; every other spec is ignored.
func analyzeAlterSpec(toks []sqlTok, sqlText string, lo, hi int, plan *stmtSupplementPlan) {
	if lo >= hi {
		return
	}
	i := lo
	switch {
	case toks[i].wordIs(sqlText, "add"):
		i++
		if i < hi && toks[i].wordIs(sqlText, "column") {
			i++
		}
		if i >= hi {
			return
		}
		if toks[i].kind == tokPunct && toks[i].text(sqlText) == "(" {
			scanColumnList(toks, sqlText, i, hi, plan)
			return
		}
		if constraintKeyword(toks[i], sqlText) {
			return
		}
		scanColumnDef(toks, sqlText, i+1, hi, toks[i].identText(sqlText), plan)
	case toks[i].wordIs(sqlText, "modify"):
		i++
		if i < hi && toks[i].wordIs(sqlText, "column") {
			i++
		}
		if i >= hi || !isIdentToken(toks[i]) {
			return
		}
		scanColumnDef(toks, sqlText, i+1, hi, toks[i].identText(sqlText), plan)
	case toks[i].wordIs(sqlText, "change"):
		i++
		if i < hi && toks[i].wordIs(sqlText, "column") {
			i++
		}
		if i+1 >= hi || !isIdentToken(toks[i]) || !isIdentToken(toks[i+1]) {
			return
		}
		scanColumnDef(toks, sqlText, i+2, hi, toks[i+1].identText(sqlText), plan)
	case toks[i].wordIs(sqlText, "alter"):
		i++
		if i < hi && toks[i].wordIs(sqlText, "column") {
			i++
		}
		if i >= hi || !isIdentToken(toks[i]) {
			return
		}
		column := toks[i].identText(sqlText)
		i++
		if i >= hi || !toks[i].wordIs(sqlText, "set") {
			return
		}
		i++
		if i < hi && toks[i].wordIs(sqlText, "default") {
			i++
			if i < hi && toks[i].kind == tokPunct && toks[i].text(sqlText) == "(" {
				if site, ok := parenSite(toks, sqlText, i, hi); ok {
					site.column = column
					plan.defaults = append(plan.defaults, site)
				}
			}
		}
		// SET VISIBLE|INVISIBLE never reaches here as the sole spec —
		// matchVisibilitySetter claimed whole-statement forms — so any other
		// SET shape inside a multi-spec statement stays for the base grammar.
	}
}

// scanColumnList scans a parenthesized column/constraint definition list.
// Items starting with constraint or index keywords keep the old grammar path;
// anything else is a column definition whose first token names the column.
func scanColumnList(toks []sqlTok, sqlText string, openTok, hi int, plan *stmtSupplementPlan) {
	depth := 1
	itemStart := openTok + 1
	for i := openTok + 1; i < hi; i++ {
		if toks[i].kind != tokPunct {
			continue
		}
		switch toks[i].text(sqlText) {
		case "(":
			depth++
		case ")":
			depth--
			if depth == 0 {
				scanColumnListItem(toks, sqlText, itemStart, i, plan)
				return
			}
		case ",":
			if depth == 1 {
				scanColumnListItem(toks, sqlText, itemStart, i, plan)
				itemStart = i + 1
			}
		}
	}
}

// scanColumnListItem classifies one comma-separated list item: constraint and
// index definitions stay on the old path (index-level VISIBLE/INVISIBLE is a
// grammar-native index option, never a column fact), column definitions get
// the option scanner.
func scanColumnListItem(toks []sqlTok, sqlText string, lo, hi int, plan *stmtSupplementPlan) {
	if lo >= hi || !isIdentToken(toks[lo]) {
		return
	}
	if constraintKeyword(toks[lo], sqlText) {
		// `key INT`/`index BIGINT` are column definitions named `key`/`index`
		// — disambiguate on a column-type second token.
		if (toks[lo].wordIs(sqlText, "key") || toks[lo].wordIs(sqlText, "index")) &&
			lo+1 < hi && isIdentToken(toks[lo+1]) && columnTypeWord(toks[lo+1], sqlText) {
			scanColumnDef(toks, sqlText, lo+1, hi, toks[lo].identText(sqlText), plan)
		}
		return
	}
	scanColumnDef(toks, sqlText, lo+1, hi, toks[lo].identText(sqlText), plan)
}

// scanColumnDef scans one column definition's option region — [lo,hi) is the
// region after the declared name — for `DEFAULT (<expr>)` and
// VISIBLE|INVISIBLE attribute sites at depth zero only. Nested groups are
// never option positions.
func scanColumnDef(toks []sqlTok, sqlText string, lo, hi int, column string, plan *stmtSupplementPlan) {
	depth := 0
	seenVisibility := false
	for i := lo; i < hi; i++ {
		tok := toks[i]
		if tok.kind == tokPunct {
			switch tok.text(sqlText) {
			case "(":
				depth++
			case ")":
				depth--
			}
			continue
		}
		if tok.kind != tokWord || depth != 0 {
			continue
		}
		switch {
		case tok.wordIs(sqlText, "default"):
			if i+1 < hi && toks[i+1].kind == tokPunct && toks[i+1].text(sqlText) == "(" {
				if site, ok := parenSite(toks, sqlText, i+1, hi); ok {
					site.column = column
					plan.defaults = append(plan.defaults, site)
				}
			}
		case tok.wordIs(sqlText, "visible"), tok.wordIs(sqlText, "invisible"):
			if seenVisibility {
				// Duplicated or contradictory visibility attributes on one
				// column definition are not a provable shape.
				plan.invalid = true
				return
			}
			seenVisibility = true
			plan.visibility = append(plan.visibility, columnVisibilityFact{
				column:     column,
				visibility: strings.ToLower(tok.text(sqlText)),
				tok:        i,
			})
		}
	}
}

// parenSite returns the byte span of the balanced '(' group opened by the
// token at openTok, or fails when the group never closes inside the span.
func parenSite(toks []sqlTok, sqlText string, openTok, hi int) (defaultExprSite, bool) {
	depth := 0
	for i := openTok; i < hi; i++ {
		if toks[i].kind != tokPunct {
			continue
		}
		switch toks[i].text(sqlText) {
		case "(":
			depth++
		case ")":
			depth--
			if depth == 0 {
				return defaultExprSite{
					open:       toks[openTok].start,
					innerStart: toks[openTok].start + 1,
					innerEnd:   toks[i].start,
				}, true
			}
		}
	}
	return defaultExprSite{}, false
}

func isIdentToken(tok sqlTok) bool {
	return tok.kind == tokWord || tok.kind == tokQIdent
}

// constraintKeyword reports whether the word starts a table constraint or
// index definition rather than a column name.
func constraintKeyword(tok sqlTok, sqlText string) bool {
	if tok.kind != tokWord {
		return false
	}
	for _, kw := range []string{"primary", "foreign", "unique", "check", "constraint", "key", "index", "fulltext", "spatial"} {
		if tok.wordIs(sqlText, kw) {
			return true
		}
	}
	return false
}

// columnTypeWord reports whether the token is a column-type keyword — it
// keeps `key INT`/`index BIGINT` column definitions out of the index skip.
func columnTypeWord(tok sqlTok, sqlText string) bool {
	if tok.kind != tokWord {
		return false
	}
	for _, kw := range []string{
		"int", "integer", "tinyint", "smallint", "mediumint", "bigint",
		"varchar", "char", "text", "tinytext", "mediumtext", "longtext",
		"blob", "tinyblob", "mediumblob", "longblob", "json",
		"decimal", "numeric", "float", "double", "real",
		"date", "datetime", "timestamp", "time", "year",
		"binary", "varbinary", "bit", "bool", "boolean", "enum", "set",
		"serial", "geometry", "point",
	} {
		if tok.wordIs(sqlText, kw) {
			return true
		}
	}
	return false
}

// applySupplementPlan produces the same-width rewritten text: every DEFAULT
// site inner becomes a `0` literal placeholder padded with spaces, every
// column visibility keyword is blanked, and every visibility-setter span is
// blanked whole so the base parse never sees the un-grammatical form.
func applySupplementPlan(sqlText string, plan supplementPlan) []byte {
	rewritten := []byte(sqlText)
	for si := range plan.stmts {
		stmt := &plan.stmts[si]
		if stmt.setter != nil {
			for i := stmt.byteStart; i < stmt.contentEnd; i++ {
				if rewritten[i] != '\n' && rewritten[i] != '\r' {
					rewritten[i] = ' '
				}
			}
			continue
		}
		for _, site := range stmt.defaults {
			rewritten[site.innerStart] = '0'
			for i := site.innerStart + 1; i < site.innerEnd; i++ {
				rewritten[i] = ' '
			}
		}
		for _, fact := range stmt.visibility {
			for i := plan.toks[fact.tok].start; i < plan.toks[fact.tok].end; i++ {
				rewritten[i] = ' '
			}
		}
	}
	return rewritten
}

// bindSupplementPlan verifies the rewritten parse and binds every recovered
// fact to a parsed table/column identity one-to-one. Any mismatch — an
// unconsumed site, a duplicate binding, a parsed statement outside its span,
// a visibility fact without exactly one named column, or an unexpected count —
// fails closed so the caller can fall back to the original text's own result.
func bindSupplementPlan(sqlText string, plan supplementPlan, parsed []ast.StmtNode) ([]ast.StmtNode, bool) {
	out := make([]ast.StmtNode, 0, len(plan.stmts))
	pi := 0
	for si := range plan.stmts {
		stmt := &plan.stmts[si]
		if stmt.invalid {
			return nil, false
		}
		if stmt.setter != nil {
			text := strings.TrimSpace(sqlText[stmt.byteStart:stmt.byteEnd])
			node := buildVisibilitySetterNode(stmt.setter, text)
			out = append(out, &supplementedStatement{
				StmtNode: node,
				supp:     &statementSupplements{setterVisibility: stmt.setter.visibility},
			})
			continue
		}
		if pi >= len(parsed) {
			return nil, false
		}
		node := parsed[pi]
		pi++
		if pos := node.OriginTextPosition(); pos < stmt.byteStart || pos > stmt.byteEnd {
			return nil, false
		}
		supp := &statementSupplements{}
		if !bindDefaultSites(sqlText, stmt, node) {
			return nil, false
		}
		for _, site := range stmt.defaults {
			if site.column == "" {
				continue
			}
			if supp.defaultParens == nil {
				supp.defaultParens = map[string]bool{}
			}
			supp.defaultParens[site.column] = true
		}
		if len(stmt.visibility) > 0 {
			supp.visibility = map[string]string{}
			for _, fact := range stmt.visibility {
				if countParsedColumnDefs(node, fact.column) != 1 {
					return nil, false
				}
				supp.visibility[fact.column] = fact.visibility
			}
		}
		out = append(out, &supplementedStatement{StmtNode: node, supp: supp})
	}
	if pi != len(parsed) {
		return nil, false
	}
	return out, true
}

// bindDefaultSites substitutes the placeholder option expressions with the
// real inner-expression ASTs. A parsed DEFAULT option that anchors inside a
// planned site is rebound to the authored inner expression; options outside
// every site (bare literal defaults such as `DEFAULT 2` that the base
// grammar already produced correctly) are left untouched. Two options
// landing in the same site, or a planned site left unbound, fails the
// binding.
func bindDefaultSites(sqlText string, stmt *stmtSupplementPlan, node ast.StmtNode) bool {
	for _, option := range defaultOptionNodes(node) {
		pos := option.Expr.OriginTextPosition()
		var site *defaultExprSite
		for i := range stmt.defaults {
			candidate := &stmt.defaults[i]
			if pos >= candidate.open && pos <= candidate.innerEnd {
				if site != nil {
					return false
				}
				site = candidate
			}
		}
		if site == nil {
			continue
		}
		if site.bound {
			return false
		}
		expr, ok := parseSelectExpressionAST(sqlText[site.innerStart:site.innerEnd])
		if !ok {
			return false
		}
		// Rebind the original bytes and position so consumers see the
		// authored expression, never the placeholder literal.
		expr.SetText(charset.FindEncoding(""), strings.TrimSpace(sqlText[site.innerStart:site.innerEnd]))
		expr.SetOriginTextPosition(site.innerStart)
		site.bound = true
		option.Expr = expr
	}
	for i := range stmt.defaults {
		if !stmt.defaults[i].bound {
			return false
		}
	}
	return true
}

// countParsedColumnDefs counts column definitions named name inside the
// parsed node — CREATE TABLE columns plus ALTER ADD/MODIFY/CHANGE
// definitions. Visibility facts are recorded only for those positions.
func countParsedColumnDefs(node ast.StmtNode, name string) int {
	count := 0
	match := func(def *ast.ColumnDef) {
		if def != nil && def.Name != nil && def.Name.Name.L == name {
			count++
		}
	}
	switch stmt := node.(type) {
	case *ast.CreateTableStmt:
		for _, def := range stmt.Cols {
			match(def)
		}
	case *ast.AlterTableStmt:
		for _, spec := range stmt.Specs {
			switch spec.Tp {
			case ast.AlterTableAddColumns, ast.AlterTableModifyColumn, ast.AlterTableChangeColumn:
				for _, def := range spec.NewColumns {
					match(def)
				}
			}
		}
	}
	return count
}

// buildVisibilitySetterNode builds the statically validated real
// AlterTableStmt identity for a `SET VISIBLE|INVISIBLE` statement — the same
// statement the base grammar produces for `ALTER COLUMN c SET DEFAULT`,
// carrying no fabricated option facts.
func buildVisibilitySetterNode(fact *visibilitySetterFact, text string) ast.StmtNode {
	table := &ast.TableName{Name: ast.NewCIStr(fact.table)}
	if fact.schema != "" {
		table.Schema = ast.NewCIStr(fact.schema)
	}
	col := &ast.ColumnDef{Name: &ast.ColumnName{Name: ast.NewCIStr(fact.column)}}
	specNode := &ast.AlterTableSpec{Tp: ast.AlterTableAlterColumn, NewColumns: []*ast.ColumnDef{col}}
	node := &ast.AlterTableStmt{Table: table, Specs: []*ast.AlterTableSpec{specNode}}
	node.SetText(charset.FindEncoding(""), text)
	return node
}

// runSupplementedParse executes the pre-planned supplement path: same-width
// rewrite, base parse of the rewritten text, then one-to-one binding. A false
// result means the caller must run the original text through the base parser
// so failures keep their original diagnostics.
func runSupplementedParse(sqlText string) ([]ast.StmtNode, []error, bool) {
	plan := planAdvancedSupplements(sqlText)
	if !plan.hasWork {
		return nil, nil, false
	}
	rewritten := applySupplementPlan(sqlText, plan)
	parsed, warns, err := parser.New().Parse(string(rewritten), "", "")
	if err != nil {
		return nil, nil, false
	}
	bound, ok := bindSupplementPlan(sqlText, plan, parsed)
	if !ok {
		return nil, nil, false
	}
	if !restoreStatementText(parsed, string(rewritten), sqlText) {
		return nil, nil, false
	}
	return bound, warns, true
}

// defaultOptionNodes enumerates the column default-value options a statement
// exposes: CREATE TABLE columns and every ALTER column spec that carries a
// ColumnDef (ADD/MODIFY/CHANGE and the SET/DROP DEFAULT setter). The
// SET/DROP DEFAULT setter carries its value in a ColumnOptionNoOption whose
// Expr is set, so those options are enumerated too.
func defaultOptionNodes(stmt ast.StmtNode) []*ast.ColumnOption {
	var options []*ast.ColumnOption
	appendDefs := func(defs []*ast.ColumnDef, setSetter bool) {
		for _, def := range defs {
			if def == nil {
				continue
			}
			for _, option := range def.Options {
				if option == nil {
					continue
				}
				if option.Tp == ast.ColumnOptionDefaultValue || (setSetter && option.Expr != nil) {
					options = append(options, option)
				}
			}
		}
	}
	switch node := stmt.(type) {
	case *ast.CreateTableStmt:
		appendDefs(node.Cols, false)
	case *ast.AlterTableStmt:
		for _, alterSpec := range node.Specs {
			appendDefs(alterSpec.NewColumns, alterSpec.Tp == ast.AlterTableAlterColumn)
		}
	}
	return options
}

// restoreStatementText rebinds each parsed statement's text to the original
// source. The placeholder rewrite preserves byte width, so the rewritten
// statement text locates the identical span in the original input; a miss
// fails closed.
func restoreStatementText(stmts []ast.StmtNode, rewritten, original string) bool {
	search := 0
	for _, stmt := range stmts {
		text := stmt.Text()
		if text == "" {
			continue
		}
		index := strings.Index(rewritten[search:], text)
		if index < 0 {
			return false
		}
		offset := search + index
		stmt.SetText(charset.FindEncoding(""), original[offset:offset+len(text)])
		search = offset + len(text)
	}
	return true
}

func isSQLWordByte(b byte) bool {
	return b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9' || b == '_' || b == '$'
}

// skipQuoted returns the index after a quoted literal starting at i. Quote
// doubling and backslash escapes are honored; unterminated quotes consume the
// rest of the input.
func skipQuoted(sqlText string, i int) int {
	quote := sqlText[i]
	i++
	for i < len(sqlText) {
		if sqlText[i] == '\\' && quote != '`' {
			i += 2
			continue
		}
		if sqlText[i] == quote {
			if i+1 < len(sqlText) && sqlText[i+1] == quote {
				i += 2
				continue
			}
			return i + 1
		}
		i++
	}
	return len(sqlText)
}

func skipLineComment(sqlText string, i int) int {
	for i < len(sqlText) && sqlText[i] != '\n' {
		i++
	}
	return i
}

func skipBlockComment(sqlText string, i int) int {
	i += 2
	for i+1 < len(sqlText) {
		if sqlText[i] == '*' && sqlText[i+1] == '/' {
			return i + 2
		}
		i++
	}
	return len(sqlText)
}
