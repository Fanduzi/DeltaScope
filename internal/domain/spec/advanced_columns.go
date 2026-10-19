// Package spec defines parser-neutral advanced-column facts and assessments.
// input: normalized statements carrying generated/default-expression/visibility facts plus version and pre-state metadata
// output: internal GeneratedColumn/ExpressionFacts/ColumnReference model, shared ColumnDependents and AssessAdvancedColumns answers
// pos: T07 advanced-column domain model consumed by rules and the ordered-state pass
// note: if this file changes, update this header and module README.md.
package spec

import "strings"

// Generated storage values. AST Stored=false means effective VIRTUAL,
// including omitted storage; the model never claims VIRTUAL was written.
const (
	GeneratedStorageVirtual = "virtual"
	GeneratedStorageStored  = "stored"
	GeneratedStorageUnknown = "unknown"
)

// Effective column visibility values; empty means withheld, never visible.
const (
	ColumnVisibilityVisible   = "visible"
	ColumnVisibilityInvisible = "invisible"
)

// Advanced-column feature names — the compatibility features parameter
// vocabulary and the finding metadata values.
const (
	AdvancedFeatureGenerated         = "generated"
	AdvancedFeatureExpressionDefault = "expression_default"
	AdvancedFeatureVisibility        = "visibility"
)

// AdvancedDependencyKind values: which advanced fact carries the dependency.
const (
	AdvancedDependencyGenerated         = "generated"
	AdvancedDependencyExpressionDefault = "expression_default"
)

// Dependency relation labels for finding metadata.
const (
	AdvancedDependencyDirect     = "direct"
	AdvancedDependencyTransitive = "transitive"
)

// Advanced-column reason codes; every code is a bounded product/version/
// structure responsibility named by the T07 contract.
const (
	AdvancedReasonVersionUnsupported           = "version_unsupported"
	AdvancedReasonMissingTargetVersion         = "missing_target_version"
	AdvancedReasonOutOfValidatedRange          = "target_version_out_of_validated_range"
	AdvancedReasonIncompleteSourceColumn       = "incomplete_source_column"
	AdvancedReasonUnknownTableState            = "unknown_table_state"
	AdvancedReasonIncompleteAlterDepEffects    = "incomplete_alter_dependency_effects"
	AdvancedReasonGeneratedStorageChange       = "generated_storage_change"
	AdvancedReasonTiDBAddStored                = "tidb_add_stored"
	AdvancedReasonTiDBStoredExpressionChange   = "tidb_stored_expression_change"
	AdvancedReasonTiDBOrdinaryToStored         = "tidb_ordinary_to_stored"
	AdvancedReasonTiDBDefaultExpression        = "tidb_default_expression"
	AdvancedReasonTiDBAlterJSONDefault         = "tidb_alter_json_default"
	AdvancedReasonNoVisibleColumns             = "no_visible_columns"
	AdvancedReasonExpressionConstructForbidden = "expression_construct_forbidden"
	// R1-1: declaration-time generated reference premise negatives. All
	// observed on every pinned anchor (3107/1054/3107), product-shared.
	AdvancedReasonGeneratedSelfReference = "generated_self_reference"
	AdvancedReasonGeneratedMissingColumn = "generated_missing_column"
	AdvancedReasonGeneratedColumnOrder   = "generated_column_order"
	// R1-2: non-literal defaults written without parens are a determined
	// negative on both products (1064 on CREATE and ALTER SET DEFAULT).
	AdvancedReasonExpressionDefaultShape     = "expression_default_shape"
	AdvancedReasonGeneratedFunctionForbidden = "generated_function_forbidden"
)

// Required-fact names shared with rule evidence gaps.
const (
	AdvancedFactTargetVersion            = "target.version"
	AdvancedFactTargetVersionRange       = "target.version.validated_range"
	AdvancedFactTargetVersionStoredToOrd = "target.version.stored_to_ordinary"
	AdvancedFactTargetTableColumns       = "target_table.columns"
	AdvancedFactSourceExprDependencies   = "source_column.expression_dependencies"
	AdvancedFactSourceGeneration         = "source_column.generation"
	AdvancedFactSourceVisibility         = "source_column.visibility"
	AdvancedFactSourceType               = "source_column.type"
	AdvancedFactAlterColumnDepEffects    = "alter.column_dependency_effects"
	AdvancedFactSourceDefaultFunction    = "source_column.default_function"
	AdvancedFactSourceDefaultComposite   = "source_column.default_composite"
	AdvancedFactAlterColumnPosition      = "alter.column_position"
)

// Expression feature markers recorded in ExpressionFacts.Features; they are
// normalized structure names, never raw SQL.
const (
	ExpressionFeatureSubquery     = "subquery"
	ExpressionFeatureAggregate    = "aggregate"
	ExpressionFeatureArithmetic   = "arithmetic"
	ExpressionFeatureUnknownFunc  = "unknown_function"
	ExpressionFeatureUnknownShape = "unknown_expression_feature"
)

// Minimum supported versions for MySQL-family advanced column features. TiDB
// column visibility is a vendor boundary rather than a version gate.
var (
	advancedMinimumGeneratedMySQL         = [3]int{5, 7, 6}
	advancedMinimumExpressionDefaultMySQL = [3]int{8, 0, 13}
	advancedMinimumVisibilityMySQL        = [3]int{8, 0, 23}
)

const (
	AdvancedMinimumGeneratedMySQL         = "5.7.6"
	AdvancedMinimumExpressionDefaultMySQL = "8.0.13"
	AdvancedMinimumVisibilityMySQL        = "8.0.23"
	// AdvancedTiDBStoredToOrdinaryVersion is the single observed TiDB patch
	// where stored-to-ordinary MODIFY succeeds; other patches stay unresolved.
	AdvancedTiDBStoredToOrdinaryVersion = "8.5.0"
)

// Dependency traversal bounds from the frozen contract; a limit hit is
// explicit unknown, never an empty complete graph.
const (
	advancedMaxColumns    = 1024
	advancedMaxPathEdges  = 64
	advancedMaxDependents = 64
)

// ColumnReference is one column reference inside an expression AST. Schema
// and Table retain the written qualifiers; only an unqualified reference can
// bind a local column name, except a qualifier that provably names the owning
// table itself.
type ColumnReference struct {
	Schema string
	Table  string
	Name   string
}

// Qualified reports whether the reference carries any qualifier.
func (r ColumnReference) Qualified() bool {
	return strings.TrimSpace(r.Schema) != "" || strings.TrimSpace(r.Table) != ""
}

// ExpressionFacts is the structured, evaluation-free fact record for one
// parsed column expression. References are AST column references in
// declaration order, deduplicated by identity, never literal contents.
// Functions are normalized lowercase callee names. Features carry normalized
// construct markers. Fingerprint is the SHA-256 of the restored expression
// structure — equal fingerprints prove sameness; missing or unequal ones
// prove nothing. Known=false means the expression semantics are unknown,
// not that the reference set is empty. TopForm is the paren-transparent
// root shape — a structural fact computed even when Known=false, like the
// construct markers.
type ExpressionFacts struct {
	References  []ColumnReference
	Known       bool
	Functions   []string
	Features    []string
	Fingerprint string
	TopForm     string
	// RootParenthesized records that the authored expression's outermost
	// node is a parentheses wrapper. TopForm stays paren-transparent, so
	// callers see both facts: `(uuid())` is TopForm=call with the root
	// wrapped, `uuid()` is TopForm=call unwrapped — the TiDB setter stores
	// the first as an evaluated literal and the second verbatim.
	RootParenthesized bool
}

// Expression root-shape names recorded in ExpressionFacts.TopForm: the
// paren-transparent outermost node class. An empty TopForm means the shape
// could not be computed — never a positive.
const (
	ExpressionTopCall       = "call"
	ExpressionTopLiteral    = "literal"
	ExpressionTopColumn     = "column"
	ExpressionTopArithmetic = "arithmetic"
	ExpressionTopUnary      = "unary"
	ExpressionTopCase       = "case"
	ExpressionTopSubquery   = "subquery"
	ExpressionTopAggregate  = "aggregate"
	ExpressionTopOther      = "other"
)

// LocalReferences returns unqualified references — the only form that binds
// a column of the owning table without a qualifier.
func (f ExpressionFacts) LocalReferences() []ColumnReference {
	if !f.Known {
		return nil
	}
	refs := make([]ColumnReference, 0, len(f.References))
	for _, ref := range f.References {
		if ref.Qualified() || strings.TrimSpace(ref.Name) == "" {
			continue
		}
		refs = append(refs, ref)
	}
	return refs
}

// GeneratedColumn carries the generated-column attribute: effective storage
// class plus structured expression facts.
type GeneratedColumn struct {
	Storage    string
	Expression ExpressionFacts
}

// CloneExpressionFacts deep-copies one ExpressionFacts value so request-local
// ownership never aliases slices between parser, provider, and state layers.
func CloneExpressionFacts(in *ExpressionFacts) *ExpressionFacts {
	if in == nil {
		return nil
	}
	out := *in
	out.References = append([]ColumnReference(nil), in.References...)
	out.Functions = append([]string(nil), in.Functions...)
	out.Features = append([]string(nil), in.Features...)
	return &out
}

// CloneGeneratedColumn deep-copies one GeneratedColumn fact including its
// embedded expression facts.
func CloneGeneratedColumn(in *GeneratedColumn) *GeneratedColumn {
	if in == nil {
		return nil
	}
	out := *in
	out.Expression = *CloneExpressionFacts(&in.Expression)
	return &out
}

// ColumnDependencyAssessment is the dependency answer for one candidate
// column name. Dependents keep column declaration order. Known=false means
// the column set or some expression was not provable — never an empty
// complete graph.
type ColumnDependencyAssessment struct {
	Dependents []ColumnDependent
	Known      bool
}

// ColumnDependent is one column whose expression binds the queried name.
type ColumnDependent struct {
	Column     string
	Kind       string
	Dependency string
}

// ColumnDependents computes the ordered direct and transitive dependents of
// name inside the column set owned by table. Only unqualified references and
// references qualified by the owning table's own identity bind local columns;
// external or unresolvable qualifiers mark the answer unknown. Empty or
// duplicate column names, self-references, cycles reachable from the dropped
// name, and traversal-limit hits also mark the answer unknown. The walk is a
// single bounded reverse-adjacency pass from the dropped name — never a
// per-column graph traversal.
func ColumnDependents(table Table, columns []Column, name string) ColumnDependencyAssessment {
	if columns == nil {
		return ColumnDependencyAssessment{Known: false}
	}
	target := strings.ToLower(strings.TrimSpace(name))
	if len(columns) > advancedMaxColumns {
		return ColumnDependencyAssessment{Known: false}
	}
	known := true
	indexByName := make(map[string]int, len(columns))
	for i, column := range columns {
		key := strings.ToLower(strings.TrimSpace(column.Name))
		if key == "" {
			known = false
			continue
		}
		if _, duplicate := indexByName[key]; duplicate {
			known = false
			continue
		}
		indexByName[key] = i
	}
	targetIndex, targetKnown := indexByName[target]

	refs := make([][]int, len(columns))
	reverse := make([][]int, len(columns))
	for i, column := range columns {
		seen := map[int]struct{}{}
		for _, facts := range columnExpressions(column) {
			if !facts.Known {
				known = false
				continue
			}
			for _, ref := range facts.References {
				bound, ok := bindColumnReference(table, indexByName, ref)
				if !ok {
					known = false
					continue
				}
				if bound == i {
					// A column that provably references itself is not a
					// computable edge — the expression state is unknown.
					known = false
					continue
				}
				if _, dup := seen[bound]; !dup {
					seen[bound] = struct{}{}
					refs[i] = append(refs[i], bound)
					reverse[bound] = append(reverse[bound], i)
				}
			}
		}
	}
	if !targetKnown {
		return ColumnDependencyAssessment{Known: known}
	}

	// One reverse-adjacency walk from the dropped name: depth 1 is a direct
	// dependent, deeper is transitive. Reachable work is bounded at
	// advancedMaxDependents nodes and advancedMaxPathEdges depth; a bound hit
	// or a reachable cycle marks the answer unknown instead of claiming a
	// complete (possibly truncated) dependent set.
	depth := make([]int, len(columns))
	for i := range depth {
		depth[i] = -1
	}
	depth[targetIndex] = 0
	queue := []int{targetIndex}
	reached := 0
	for head := 0; head < len(queue); head++ {
		current := queue[head]
		if depth[current] >= advancedMaxPathEdges {
			known = false
			continue
		}
		for _, dep := range reverse[current] {
			if depth[dep] >= 0 {
				continue
			}
			reached++
			if reached > advancedMaxDependents {
				known = false
				break
			}
			depth[dep] = depth[current] + 1
			queue = append(queue, dep)
		}
	}
	// A reachable node that can return to itself is a cycle: the expression
	// set is degenerate and the answer is unknown rather than complete.
	for i := 0; i < len(columns) && known; i++ {
		if i == targetIndex || depth[i] < 0 {
			continue
		}
		if reachableCycle(refs, i) {
			known = false
		}
	}
	var dependents []ColumnDependent
	for i, column := range columns {
		if i == targetIndex || depth[i] < 0 {
			continue
		}
		relation := AdvancedDependencyTransitive
		if depth[i] == 1 {
			relation = AdvancedDependencyDirect
		}
		dependents = append(dependents, ColumnDependent{
			Column:     column.Name,
			Kind:       dependencyKind(column),
			Dependency: relation,
		})
	}
	return ColumnDependencyAssessment{Dependents: dependents, Known: known}
}

// reachableCycle reports whether the column at start can reach itself through
// the forward reference edges within the traversal bound.
func reachableCycle(refs [][]int, start int) bool {
	visited := map[int]struct{}{}
	stack := []int{start}
	steps := 0
	for len(stack) > 0 {
		current := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		for _, next := range refs[current] {
			if next == start {
				return true
			}
			steps++
			if steps > advancedMaxPathEdges*advancedMaxPathEdges {
				return true
			}
			if _, seen := visited[next]; !seen {
				visited[next] = struct{}{}
				stack = append(stack, next)
			}
		}
	}
	return false
}

// bindColumnReference resolves one reference to a local column index. A
// qualified reference binds only when it provably names the owning table
// itself; anything else — external schema, unmatched table qualifier, or a
// name absent from the declared set — is unresolvable.
func bindColumnReference(table Table, indexByName map[string]int, ref ColumnReference) (int, bool) {
	name := strings.ToLower(strings.TrimSpace(ref.Name))
	if name == "" {
		return -1, false
	}
	if ref.Qualified() {
		if !strings.EqualFold(strings.TrimSpace(ref.Table), strings.TrimSpace(table.Name)) {
			return -1, false
		}
		schema := strings.TrimSpace(ref.Schema)
		own := strings.TrimSpace(table.Schema)
		if schema != "" && (own == "" || !strings.EqualFold(schema, own)) {
			return -1, false
		}
	}
	idx, ok := indexByName[name]
	return idx, ok
}

// columnExpressions returns the expression fact records a column carries:
// the generated expression and the expression default, in that order.
func columnExpressions(column Column) []ExpressionFacts {
	var exprs []ExpressionFacts
	if column.Generated != nil {
		exprs = append(exprs, column.Generated.Expression)
	}
	if column.DefaultExpression != nil {
		exprs = append(exprs, *column.DefaultExpression)
	}
	return exprs
}

func dependencyKind(column Column) string {
	if column.Generated != nil {
		return AdvancedDependencyGenerated
	}
	return AdvancedDependencyExpressionDefault
}

// AdvancedColumnIssue is one known product/version/structure negative on an
// advanced column fact. MinimumVersion is empty for non-version checks.
type AdvancedColumnIssue struct {
	Column         string
	Feature        string
	Action         string
	ReasonCode     string
	MinimumVersion string
}

// AdvancedColumnGap is one bounded evidence hole. Feature scopes the gap to
// one feature family; an empty Feature means the gap is shared (version).
type AdvancedColumnGap struct {
	ReasonCode    string
	RequiredFacts []string
	Feature       string
}

// ColumnDependencyEffect is the dependency-side answer for one
// dependency-scope alter action: the resolved dependents when the answer is
// provable, or Known=false with a matching gap when it is not.
type ColumnDependencyEffect struct {
	Column     string
	Action     string
	Dependents []ColumnDependent
	Known      bool
}

// AdvancedColumnAssessment is the shared spec-owned answer for one
// statement's advanced-column facts. Applies marks that the statement
// carries compatibility-relevant advanced features. Issues are determined
// negatives; Gaps are bounded compatibility/version uncertainty;
// DependencyEffects and DependencyGaps carry the dependency-side answer.
// InvalidTransition marks a provably rejected operation that must not publish
// a post-state; UnresolvedTransition marks one whose post-state cannot be
// honestly derived. Features is the set of advanced features the statement
// carries; VersionGated is the subset that consults the target version.
type AdvancedColumnAssessment struct {
	Applies              bool
	Issues               []AdvancedColumnIssue
	Gaps                 []AdvancedColumnGap
	DependencyEffects    []ColumnDependencyEffect
	DependencyGaps       []AdvancedColumnGap
	InvalidTransition    bool
	UnresolvedTransition bool
	Features             map[string]bool
	VersionGated         map[string]bool
}

// HasFeature reports whether the statement carries the named feature.
func (a AdvancedColumnAssessment) HasFeature(feature string) bool {
	return a.Features[feature]
}

func (a *AdvancedColumnAssessment) appendGap(feature, reason string, facts ...string) {
	for _, gap := range a.Gaps {
		if gap.ReasonCode == reason && gap.Feature == feature && sameStringSet(gap.RequiredFacts, facts) {
			return
		}
	}
	a.Gaps = append(a.Gaps, AdvancedColumnGap{ReasonCode: reason, RequiredFacts: append([]string(nil), facts...), Feature: feature})
}

func (a *AdvancedColumnAssessment) appendDependencyGap(reason string, facts ...string) {
	for _, gap := range a.DependencyGaps {
		if gap.ReasonCode == reason && sameStringSet(gap.RequiredFacts, facts) {
			return
		}
	}
	a.DependencyGaps = append(a.DependencyGaps, AdvancedColumnGap{ReasonCode: reason, RequiredFacts: append([]string(nil), facts...)})
}

func (a *AdvancedColumnAssessment) appendIssue(column, feature, action, reason, minimum string) {
	a.Issues = append(a.Issues, AdvancedColumnIssue{
		Column:         column,
		Feature:        feature,
		Action:         action,
		ReasonCode:     reason,
		MinimumVersion: minimum,
	})
	// Every issue is a determined negative: the statement cannot succeed, so
	// a post-state must never be published for it — independently of whether
	// a finding rule is enabled.
	a.InvalidTransition = true
}

func sameStringSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// dependencyScopeAction returns the source column name for a
// dependency-scope alter action. DROP COLUMN is always in scope;
// CHANGE/RENAME COLUMN only when the identity actually changes.
func dependencyScopeAction(alter Alter) (string, bool) {
	switch alter.Action {
	case "drop_column":
		name := strings.TrimSpace(alter.Name)
		if alter.Column != nil && strings.TrimSpace(alter.Column.OldName) != "" {
			name = strings.TrimSpace(alter.Column.OldName)
		}
		return name, name != ""
	case "rename_column":
		if alter.Column == nil {
			return "", false
		}
		oldName := strings.TrimSpace(alter.Column.OldName)
		newName := ""
		if alter.Column.Definition != nil {
			newName = strings.TrimSpace(alter.Column.Definition.Name)
		}
		if oldName == "" || strings.EqualFold(oldName, newName) {
			return "", false
		}
		return oldName, true
	case "change_column":
		if alter.Column == nil || alter.Column.Definition == nil {
			return "", false
		}
		oldName := strings.TrimSpace(alter.Column.OldName)
		newName := strings.TrimSpace(alter.Column.Definition.Name)
		if oldName == "" || newName == "" || strings.EqualFold(oldName, newName) {
			return "", false
		}
		return oldName, true
	default:
		return "", false
	}
}

// statementColumnFacts collects the (column, action) pairs carrying
// potentially advanced definitions, in statement order.
type advancedColumnFact struct {
	column     *Column
	action     string
	oldName    string
	positioned bool
}

func statementColumnFacts(ddl *DDL) []advancedColumnFact {
	var out []advancedColumnFact
	if ddl == nil {
		return out
	}
	switch ddl.Operation {
	case DDLOperationCreateTable:
		for i := range ddl.Columns {
			out = append(out, advancedColumnFact{column: &ddl.Columns[i], action: "create_table"})
		}
	case DDLOperationAlterTable:
		for i := range ddl.Alter {
			alter := &ddl.Alter[i]
			if alter.Column == nil || alter.Column.Definition == nil {
				continue
			}
			switch alter.Action {
			case "add_columns", "modify_column", "change_column",
				"set_column_default", "set_column_visibility":
				out = append(out, advancedColumnFact{column: alter.Column.Definition, action: alter.Action, oldName: alter.Column.OldName, positioned: alter.HasColumnPosition})
			}
		}
	}
	return out
}

// columnFeatures returns the advanced features one declared column carries.
func columnFeatures(column *Column, action string) []string {
	var features []string
	if column.Generated != nil {
		features = append(features, AdvancedFeatureGenerated)
	}
	// An authored `DEFAULT (<expr>)` wrapper is expression-default surface
	// even when the inner expression is a literal: TiDB's parens grammar
	// rejects the literal form while MySQL accepts it — the feature must be
	// present for the dialect-specific outcome to attach.
	if column.DefaultExpression != nil || column.DefaultParenthesized {
		features = append(features, AdvancedFeatureExpressionDefault)
	}
	if action == "set_column_visibility" || column.VisibilityExplicit {
		features = append(features, AdvancedFeatureVisibility)
	}
	return features
}

// versionAtLeast compares the already-parsed identity against a numeric
// minimum triple.
func versionAtLeast(version *VersionIdentity, minimum [3]int) bool {
	got := [3]int{version.Major, version.Minor, version.Patch}
	for i := range got {
		if got[i] != minimum[i] {
			return got[i] > minimum[i]
		}
	}
	return true
}

// nondeterministicGeneratedFunctions are recognized built-ins a generated
// expression must never call.
var nondeterministicGeneratedFunctions = map[string]bool{
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

// tidbCreateDefaultFunctions is the observed TiDB expression-default function
// set for CREATE-family declarations. uuid() joined the observed positives in
// the R1 evidence round (accepted on CREATE, stored verbatim).
var tidbCreateDefaultFunctions = map[string]bool{
	"uuid": true, "json_object": true, "json_array": true, "json_quote": true,
}

// tidbAlterDefaultFunctions is the observed TiDB ALTER SET DEFAULT
// expression-preserving set: bare calls of these callees store the
// expression verbatim (uuid, json_object and json_array are the
// anchor-observed acceptances).
var tidbAlterDefaultFunctions = map[string]bool{
	"uuid": true, "json_object": true, "json_array": true,
}

// tidbAlterFoldableFunctions are TiDB ALTER SET DEFAULT callees the setter
// evaluates rather than preserves: current_timestamp folds to a timestamp
// literal (accepted on temporal/DATE/VARCHAR targets, 1067 on numeric).
var tidbAlterFoldableFunctions = map[string]bool{
	"current_timestamp": true,
}

// tidbCreateDisallowedDefaultFunctions are observed TiDB CREATE-family
// expression-default rejections (3770 disallowed function).
var tidbCreateDisallowedDefaultFunctions = map[string]bool{
	"uuid_short": true, "lower": true,
}

// tidbAlterDisallowedDefaultFunctions are observed TiDB ALTER SET DEFAULT
// rejections under the disallowed-function check (3770) — now() included,
// which is disallowed on the setter even where CREATE accepts it.
var tidbAlterDisallowedDefaultFunctions = map[string]bool{
	"lower": true, "concat": true, "uuid_short": true, "now": true,
}

// tidbTemporalDefaultFunctions are the temporal default callees: a bare
// pure call of one of these is the classic temporal default accepted on
// temporal columns by both products (now/current_timestamp/localtime
// observed; localtimestamp is the documented synonym family), and a
// parenthesized pure call is the type-gated cell on the TiDB create path.
var tidbTemporalDefaultFunctions = map[string]bool{
	"now": true, "current_timestamp": true, "localtime": true, "localtimestamp": true,
}

func allFunctionsIn(functions []string, allowed map[string]bool) bool {
	for _, fn := range functions {
		if !allowed[strings.ToLower(fn)] {
			return false
		}
	}
	return true
}

// AssessAdvancedColumns computes the shared T07 assessment for one
// statement. It reads bound metadata (target version, pre-state snapshot) so
// the same answer feeds the compatibility rule, the dependency rule, and the
// ordered-state publication gate. PostgreSQL and non-DDL never apply.
func AssessAdvancedColumns(statement Statement) AdvancedColumnAssessment {
	assessment := AdvancedColumnAssessment{Features: map[string]bool{}, VersionGated: map[string]bool{}}
	if statement.Dialect != DialectMySQL && statement.Dialect != DialectTiDB {
		return assessment
	}
	ddl := statement.DDL
	if ddl == nil || (ddl.Operation != DDLOperationCreateTable && ddl.Operation != DDLOperationAlterTable) {
		return assessment
	}
	var version *VersionIdentity
	var snapshot *TableSnapshot
	if statement.Metadata != nil {
		version = statement.Metadata.Version
		snapshot = statement.Metadata.TargetTable
	}

	for _, item := range statementColumnFacts(ddl) {
		assessColumnFacts(&assessment, statement, ddl, item, snapshot, version)
	}
	assessGeneratedRemovals(&assessment, ddl, snapshot, version)
	assessGeneratedReferencePremises(&assessment, ddl, snapshot)
	assessDependencyScope(&assessment, ddl, snapshot)

	assessment.Applies = len(assessment.Features) > 0
	return assessment
}

// assessColumnFacts evaluates one declared column for the three advanced
// features. Version gates run before product negatives.
func assessColumnFacts(assessment *AdvancedColumnAssessment, statement Statement, ddl *DDL, item advancedColumnFact, snapshot *TableSnapshot, version *VersionIdentity) {
	column := item.column
	if column == nil {
		return
	}
	dialect := statement.Dialect
	features := columnFeatures(column, item.action)
	for _, feature := range features {
		assessment.Features[feature] = true
	}
	if len(features) == 0 {
		return
	}

	// Version gating: one gap per statement when any feature consults the
	// version. TiDB visibility is a vendor boundary, not a version gate.
	var gated []string
	for _, feature := range features {
		if feature == AdvancedFeatureVisibility && dialect == DialectTiDB {
			continue
		}
		gated = append(gated, feature)
	}
	for _, feature := range gated {
		assessment.VersionGated[feature] = true
	}
	if len(gated) > 0 {
		switch {
		case version == nil:
			assessment.appendGap("", AdvancedReasonMissingTargetVersion, AdvancedFactTargetVersion)
		case !version.ValidatedRange:
			assessment.appendGap("", AdvancedReasonOutOfValidatedRange, AdvancedFactTargetVersionRange)
		}
	}
	versionKnown := version != nil && version.ValidatedRange

	for _, feature := range features {
		switch feature {
		case AdvancedFeatureGenerated:
			assessGeneratedFeature(assessment, item, snapshot, version, versionKnown, dialect)
		case AdvancedFeatureExpressionDefault:
			assessExpressionDefaultFeature(assessment, item, snapshot, version, versionKnown, dialect)
		case AdvancedFeatureVisibility:
			assessVisibilityFeature(assessment, ddl, item, snapshot, version, versionKnown, dialect)
		}
	}
}

// assessGeneratedFeature checks the generated feature against the
// product/version responsibility table and generated-specific constructs.
func assessGeneratedFeature(assessment *AdvancedColumnAssessment, item advancedColumnFact, snapshot *TableSnapshot, version *VersionIdentity, versionKnown bool, dialect Dialect) {
	column := item.column
	name := column.Name
	if versionKnown && dialect == DialectMySQL && !versionAtLeast(version, advancedMinimumGeneratedMySQL) {
		assessment.appendIssue(name, AdvancedFeatureGenerated, item.action, AdvancedReasonVersionUnsupported, AdvancedMinimumGeneratedMySQL)
		return
	}
	expr := column.Generated.Expression
	// Construct and callee facts are structural: they are valid evidence even
	// when the full traversal is unknown.
	for _, feature := range expr.Features {
		if feature == ExpressionFeatureSubquery || feature == ExpressionFeatureAggregate {
			assessment.appendIssue(name, AdvancedFeatureGenerated, item.action, AdvancedReasonExpressionConstructForbidden, "")
			return
		}
	}
	for _, fn := range expr.Functions {
		if nondeterministicGeneratedFunctions[strings.ToLower(fn)] {
			assessment.appendIssue(name, AdvancedFeatureGenerated, item.action, AdvancedReasonGeneratedFunctionForbidden, "")
			return
		}
	}
	if !expr.Known {
		assessment.appendGap(AdvancedFeatureGenerated, AdvancedReasonIncompleteSourceColumn, AdvancedFactSourceExprDependencies)
		assessment.UnresolvedTransition = true
	}
	switch item.action {
	case "add_columns":
		if dialect == DialectTiDB && column.Generated.Storage == GeneratedStorageStored {
			assessment.appendIssue(name, AdvancedFeatureGenerated, item.action, AdvancedReasonTiDBAddStored, "")
		}
	case "modify_column", "change_column":
		priorName := alterSourceName(column.Name, item.oldName)
		prior, priorOK := priorColumn(snapshot, priorName)
		switch {
		case snapshot == nil || snapshot.Columns == nil:
			// Withheld column set: the prior generation state is not provable.
			assessment.appendGap(AdvancedFeatureGenerated, AdvancedReasonIncompleteSourceColumn, AdvancedFactSourceGeneration)
			assessment.UnresolvedTransition = true
		case !priorOK:
			// A missing source member stays owned by the existence checks.
		case prior.Generated != nil && prior.Generated.Storage == GeneratedStorageUnknown:
			assessment.appendGap(AdvancedFeatureGenerated, AdvancedReasonIncompleteSourceColumn, AdvancedFactSourceGeneration)
			assessment.UnresolvedTransition = true
		case column.Generated.Storage == GeneratedStorageUnknown:
			assessment.appendGap(AdvancedFeatureGenerated, AdvancedReasonIncompleteSourceColumn, AdvancedFactSourceGeneration)
			assessment.UnresolvedTransition = true
		case prior.Generated != nil &&
			prior.Generated.Storage != column.Generated.Storage:
			// Storage-mode conversion in either direction is a documented
			// negative: VIRTUAL→STORED is a recorded anchor negative and
			// STORED→VIRTUAL hits the same documented restriction.
			assessment.appendIssue(name, AdvancedFeatureGenerated, item.action, AdvancedReasonGeneratedStorageChange, "")
		case dialect == DialectTiDB && prior.Generated == nil &&
			column.Generated.Storage == GeneratedStorageStored:
			assessment.appendIssue(name, AdvancedFeatureGenerated, item.action, AdvancedReasonTiDBOrdinaryToStored, "")
		case dialect == DialectTiDB && prior.Generated == nil &&
			column.Generated.Storage == GeneratedStorageVirtual:
			// Ordinary→VIRTUAL on TiDB is outside the frozen native
			// evidence; withhold the post-state instead of asserting an
			// unobserved outcome.
			assessment.appendGap(AdvancedFeatureGenerated, AdvancedReasonIncompleteSourceColumn, AdvancedFactSourceGeneration)
			assessment.UnresolvedTransition = true
		case dialect == DialectTiDB && prior.Generated != nil &&
			prior.Generated.Storage == GeneratedStorageStored &&
			column.Generated.Storage == GeneratedStorageStored:
			// STORED→STORED: identical fingerprints are the recorded positive;
			// missing or unequal identities cannot prove sameness.
			if prior.Generated.Expression.Fingerprint == "" || column.Generated.Expression.Fingerprint == "" {
				assessment.appendGap(AdvancedFeatureGenerated, AdvancedReasonIncompleteSourceColumn, AdvancedFactSourceGeneration)
				assessment.UnresolvedTransition = true
			} else if prior.Generated.Expression.Fingerprint != column.Generated.Expression.Fingerprint {
				assessment.appendIssue(name, AdvancedFeatureGenerated, item.action, AdvancedReasonTiDBStoredExpressionChange, "")
			}
		}
	}
}

// assessGeneratedRemovals handles a MODIFY/CHANGE definition that drops the
// generated attribute entirely. TiDB pins the observed 8.5.0 positive; other
// validated patches are unresolved with the feature-specific range gap.
func assessGeneratedRemovals(assessment *AdvancedColumnAssessment, ddl *DDL, snapshot *TableSnapshot, version *VersionIdentity) {
	if ddl == nil || ddl.Operation != DDLOperationAlterTable {
		return
	}
	for i := range ddl.Alter {
		alter := &ddl.Alter[i]
		if alter.Action != "modify_column" && alter.Action != "change_column" {
			continue
		}
		if alter.Column == nil || alter.Column.Definition == nil || alter.Column.Definition.Generated != nil {
			continue
		}
		prior, ok := priorColumn(snapshot, alterSourceName(alter.Column.Definition.Name, alter.Column.OldName))
		if !ok || prior.Generated == nil {
			continue
		}
		assessment.Features[AdvancedFeatureGenerated] = true
		assessment.VersionGated[AdvancedFeatureGenerated] = true
		switch {
		case version == nil:
			assessment.appendGap("", AdvancedReasonMissingTargetVersion, AdvancedFactTargetVersion)
			assessment.UnresolvedTransition = true
		case !version.ValidatedRange:
			assessment.appendGap("", AdvancedReasonOutOfValidatedRange, AdvancedFactTargetVersionRange)
			assessment.UnresolvedTransition = true
		case version.Product == VersionProductTiDB &&
			!(version.Version == AdvancedTiDBStoredToOrdinaryVersion && prior.Generated.Storage == GeneratedStorageStored):
			// Only stored→ordinary at the observed 8.5.0 build is a TiDB
			// positive; every other TiDB removal transition is unresolved.
			assessment.appendGap("", AdvancedReasonOutOfValidatedRange, AdvancedFactTargetVersionStoredToOrd)
			assessment.UnresolvedTransition = true
		}
	}
}

// alterSourceName resolves the prior-state column name for a MODIFY or
// CHANGE: CHANGE carries the old identity in Column.OldName.
func alterSourceName(newName, oldName string) string {
	if trimmed := strings.TrimSpace(oldName); trimmed != "" {
		return trimmed
	}
	return strings.TrimSpace(newName)
}

// priorColumn finds a column in the pre-state snapshot by name.
func priorColumn(snapshot *TableSnapshot, name string) (*Column, bool) {
	if snapshot == nil || snapshot.Columns == nil {
		return nil, false
	}
	for i := range snapshot.Columns {
		if strings.EqualFold(strings.TrimSpace(snapshot.Columns[i].Name), strings.TrimSpace(name)) {
			return &snapshot.Columns[i], true
		}
	}
	return nil, false
}

// assessExpressionDefaultFeature applies the expression-default version and
// product responsibility table. (Expression-default production facts are
// wired in the default-expression family; the checks live here so the
// shared assessment shape is complete.) The TiDB table adjudicates the
// whole expression shape, not just the callee set — the R1 evidence shows
// TiDB evaluates SET DEFAULT composites and restricts CREATE parens defaults
// to a bare call, while both products reject non-literal defaults written
// without parens.
func assessExpressionDefaultFeature(assessment *AdvancedColumnAssessment, item advancedColumnFact, snapshot *TableSnapshot, version *VersionIdentity, versionKnown bool, dialect Dialect) {
	column := item.column
	if versionKnown && dialect == DialectMySQL && !versionAtLeast(version, advancedMinimumExpressionDefaultMySQL) {
		assessment.appendIssue(column.Name, AdvancedFeatureExpressionDefault, item.action, AdvancedReasonVersionUnsupported, AdvancedMinimumExpressionDefaultMySQL)
		return
	}
	if item.action == "set_column_default" {
		// A default setter landing on a prior generated column is a
		// documented negative on both products: generated columns cannot
		// carry defaults. Declare-side DEFAULT-on-generated is rejected
		// by the grammar before it reaches this seam.
		if prior, ok := priorColumn(snapshot, item.oldName); ok && prior.Generated != nil {
			assessment.appendIssue(column.Name, AdvancedFeatureExpressionDefault, item.action, AdvancedReasonExpressionConstructForbidden, "")
			return
		}
	}
	facts := column.DefaultExpression
	if facts == nil {
		// SET DEFAULT (current_timestamp) folds to a timestamp literal at
		// the TiDB setter — the same outcome table as the foldable-call
		// expression path, just arriving through the literal-class fact:
		// temporal/DATE targets store verbatim CURRENT_TIMESTAMP, the
		// positive bases store a folded literal, the issue bases answer
		// 1067, and anything else is an unproven coercion.
		if item.action == "set_column_default" && dialect == DialectTiDB &&
			column.DefaultParenthesized && column.DefaultIsCurrentTimestamp {
			if snapshot == nil || snapshot.Columns == nil {
				assessment.appendGap(AdvancedFeatureExpressionDefault, AdvancedReasonUnknownTableState, AdvancedFactTargetTableColumns)
				assessment.UnresolvedTransition = true
				return
			}
			prior, priorOK := priorColumn(snapshot, item.oldName)
			if !priorOK {
				// A missing source member stays owned by the existence
				// checks.
				return
			}
			switch base := baseColumnTypeName(prior.Type); {
			case base == "":
				assessment.appendGap(AdvancedFeatureExpressionDefault, AdvancedReasonIncompleteSourceColumn, AdvancedFactSourceType)
				assessment.UnresolvedTransition = true
			case tidbTimestampFoldTemporalTargets[base] || tidbTimestampFoldPositiveTargets[base]:
				// Observed accepted: verbatim temporal or folded literal.
			case tidbTimestampFoldIssueTargets[base]:
				assessment.appendIssue(column.Name, AdvancedFeatureExpressionDefault, item.action, AdvancedReasonTiDBDefaultExpression, "")
			default:
				assessment.appendGap(AdvancedFeatureExpressionDefault, AdvancedReasonIncompleteSourceColumn, AdvancedFactSourceDefaultComposite)
				assessment.UnresolvedTransition = true
			}
			return
		}
		// An authored `DEFAULT (<expr>)` wrapper around a literal/NULL is
		// outside the expression fact but inside the surface: only TiDB's
		// create-family parens grammar rejects it (1064); MySQL and the
		// ALTER setter fold it to a literal default.
		if column.DefaultParenthesized && dialect == DialectTiDB && createFamilyAlterAction(item.action) {
			assessment.appendIssue(column.Name, AdvancedFeatureExpressionDefault, item.action, AdvancedReasonTiDBDefaultExpression, "")
		}
		return
	}
	for _, feature := range facts.Features {
		if feature == ExpressionFeatureSubquery || feature == ExpressionFeatureAggregate {
			assessment.appendIssue(column.Name, AdvancedFeatureExpressionDefault, item.action, AdvancedReasonExpressionConstructForbidden, "")
			return
		}
	}
	if !facts.Known {
		assessment.appendGap(AdvancedFeatureExpressionDefault, AdvancedReasonIncompleteSourceColumn, AdvancedFactSourceExprDependencies)
		assessment.UnresolvedTransition = true
		return
	}
	if !column.DefaultParenthesized {
		if item.action == "set_column_default" {
			// The SET path requires parens for every non-literal default:
			// bare function calls — temporal ones included — are a grammar
			// rejection on both products (observed 1064).
			if dialect == DialectMySQL {
				assessment.appendIssue(column.Name, AdvancedFeatureExpressionDefault, item.action, AdvancedReasonExpressionDefaultShape, "")
			} else {
				assessment.appendIssue(column.Name, AdvancedFeatureExpressionDefault, item.action, AdvancedReasonTiDBDefaultExpression, "")
			}
			return
		}
		if facts.TopForm == ExpressionTopCall && len(facts.Functions) > 0 && allFunctionsIn(facts.Functions, tidbTemporalDefaultFunctions) {
			// A bare pure temporal call is the classic temporal default,
			// accepted on temporal columns by both products and a
			// determined invalid-default rejection (observed 1067) on a
			// proven non-temporal declared type.
			base := baseColumnTypeName(column.Type)
			if base == "" {
				assessment.appendGap(AdvancedFeatureExpressionDefault, AdvancedReasonIncompleteSourceColumn, AdvancedFactSourceType)
				assessment.UnresolvedTransition = true
			} else if !temporalFamilyType(base) {
				if dialect == DialectMySQL {
					assessment.appendIssue(column.Name, AdvancedFeatureExpressionDefault, item.action, AdvancedReasonExpressionDefaultShape, "")
				} else {
					assessment.appendIssue(column.Name, AdvancedFeatureExpressionDefault, item.action, AdvancedReasonTiDBDefaultExpression, "")
				}
			}
			return
		}
		if dialect == DialectMySQL {
			// Every other bare non-literal default is a grammar rejection
			// (observed 1064 for uuid()/sysdate() and composite forms).
			assessment.appendIssue(column.Name, AdvancedFeatureExpressionDefault, item.action, AdvancedReasonExpressionDefaultShape, "")
			return
		}
		// TiDB's create family additionally accepts a bare function call;
		// it falls through to the create-side table.
	}
	if dialect != DialectTiDB {
		return
	}
	switch item.action {
	case "create_table", "add_columns", "modify_column", "change_column":
		assessTiDBCreateExpressionDefault(assessment, item, facts)
	case "set_column_default":
		assessTiDBAlterExpressionDefault(assessment, item, snapshot, facts)
	}
}

// createFamilyAlterAction reports whether the action is a declare-side
// column definition path (CREATE-family) as opposed to the ALTER setter.
func createFamilyAlterAction(action string) bool {
	switch action {
	case "create_table", "add_columns", "modify_column", "change_column":
		return true
	}
	return false
}

// assessTiDBCreateExpressionDefault adjudicates a TiDB CREATE-family
// expression default. TiDB's parens grammar accepts only a bare function
// call — literals, NULL, arithmetic, CASE, and composites are 1064 — then
// applies the function allowlist over the whole expression tree, with the
// temporal cell gated on the declared column type.
func assessTiDBCreateExpressionDefault(assessment *AdvancedColumnAssessment, item advancedColumnFact, facts *ExpressionFacts) {
	column := item.column
	if column.DefaultParenthesized && facts.TopForm != ExpressionTopCall {
		// A parens default whose unwrapped root is not a bare call cannot be
		// expressed by the TiDB grammar (observed 1064 for literals,
		// arithmetic, CASE, and subqueries).
		assessment.appendIssue(column.Name, AdvancedFeatureExpressionDefault, item.action, AdvancedReasonTiDBDefaultExpression, "")
		return
	}
	if facts.TopForm != ExpressionTopCall {
		// Unreachable for authored bare forms — the grammar only parses a
		// bare call there — kept as a fail-closed guard.
		assessment.appendIssue(column.Name, AdvancedFeatureExpressionDefault, item.action, AdvancedReasonTiDBDefaultExpression, "")
		return
	}
	for _, fn := range facts.Functions {
		name := strings.ToLower(fn)
		switch {
		case tidbCreateDisallowedDefaultFunctions[name]:
			assessment.appendIssue(column.Name, AdvancedFeatureExpressionDefault, item.action, AdvancedReasonTiDBDefaultExpression, "")
			return
		case !tidbCreateDefaultFunctions[name] && !tidbTemporalDefaultFunctions[name]:
			assessment.appendGap(AdvancedFeatureExpressionDefault, AdvancedReasonIncompleteSourceColumn, AdvancedFactSourceDefaultFunction)
			assessment.UnresolvedTransition = true
			return
		}
	}
	temporal := 0
	for _, fn := range facts.Functions {
		if tidbTemporalDefaultFunctions[strings.ToLower(fn)] {
			temporal++
		}
	}
	if temporal > 0 {
		// A temporal callee mixes with nothing else observably; a pure
		// temporal default is the type-gated cell.
		if temporal != len(facts.Functions) {
			assessment.appendGap(AdvancedFeatureExpressionDefault, AdvancedReasonIncompleteSourceColumn, AdvancedFactSourceDefaultComposite)
			assessment.UnresolvedTransition = true
			return
		}
		switch base := baseColumnTypeName(column.Type); {
		case base == "":
			assessment.appendGap(AdvancedFeatureExpressionDefault, AdvancedReasonIncompleteSourceColumn, AdvancedFactSourceType)
			assessment.UnresolvedTransition = true
		case !temporalFamilyType(base):
			// Observed 1067: a temporal default on a non-temporal column.
			assessment.appendIssue(column.Name, AdvancedFeatureExpressionDefault, item.action, AdvancedReasonTiDBDefaultExpression, "")
		}
		return
	}
	// All functions are inside the observed allowed set and the root is a
	// bare call — the observed positive family.
}

// assessTiDBAlterExpressionDefault adjudicates a TiDB ALTER SET DEFAULT
// expression. TiDB evaluates the default: a bare allowed call stores the
// expression, foldable composites store an unprovable literal, and
// disallowed functions (3770), column references (1054), function-bearing
// arithmetic (1292), and any expression on a BLOB/TEXT/JSON target (1101)
// are determined rejections. Unobserved function families and unproven
// composite roots stay bounded gaps.
func assessTiDBAlterExpressionDefault(assessment *AdvancedColumnAssessment, item advancedColumnFact, snapshot *TableSnapshot, facts *ExpressionFacts) {
	column := item.column
	if len(facts.LocalReferences()) > 0 {
		// Column references inside a default are rejected whether or not
		// the named column exists (observed 1054 both ways).
		assessment.appendIssue(column.Name, AdvancedFeatureExpressionDefault, item.action, AdvancedReasonTiDBDefaultExpression, "")
		return
	}
	if snapshot == nil || snapshot.Columns == nil {
		assessment.appendGap(AdvancedFeatureExpressionDefault, AdvancedReasonUnknownTableState, AdvancedFactTargetTableColumns)
		assessment.UnresolvedTransition = true
		return
	}
	prior, priorOK := priorColumn(snapshot, item.oldName)
	if !priorOK {
		// A missing source member stays owned by the existence checks.
		return
	}
	switch base := baseColumnTypeName(prior.Type); {
	case base == "":
		assessment.appendGap(AdvancedFeatureExpressionDefault, AdvancedReasonIncompleteSourceColumn, AdvancedFactSourceType)
		assessment.UnresolvedTransition = true
		return
	case blobTextJSONFamilyType(base):
		// BLOB/TEXT/JSON targets cannot carry a default at all (observed
		// 1101), independent of the function family — the rescoped
		// tidb_alter_json_default cell.
		assessment.appendIssue(column.Name, AdvancedFeatureExpressionDefault, item.action, AdvancedReasonTiDBAlterJSONDefault, "")
		return
	}
	// The disallowed/unobserved callee gate applies to the authored bare
	// call path only: a root parentheses wrapper routes the setter through
	// pure evaluation, where `((lower('x')))` provably evaluates and stores
	// a literal (observed) — no 3770 allowlist check runs.
	if !facts.RootParenthesized {
		for _, fn := range facts.Functions {
			name := strings.ToLower(fn)
			switch {
			case tidbAlterDisallowedDefaultFunctions[name]:
				assessment.appendIssue(column.Name, AdvancedFeatureExpressionDefault, item.action, AdvancedReasonTiDBDefaultExpression, "")
				return
			case !tidbAlterDefaultFunctions[name] && !tidbAlterFoldableFunctions[name]:
				assessment.appendGap(AdvancedFeatureExpressionDefault, AdvancedReasonIncompleteSourceColumn, AdvancedFactSourceDefaultFunction)
				assessment.UnresolvedTransition = true
				return
			}
		}
	}
	priorBase := baseColumnTypeName(prior.Type)
	switch {
	case facts.TopForm == ExpressionTopCall && !facts.RootParenthesized:
		if allFunctionsIn(facts.Functions, tidbAlterDefaultFunctions) {
			if len(facts.Functions) != 1 {
				// Nested allowed calls are an unobserved composite — a
				// bare call positive never covers them.
				assessment.appendGap(AdvancedFeatureExpressionDefault, AdvancedReasonIncompleteSourceColumn, AdvancedFactSourceDefaultComposite)
				assessment.UnresolvedTransition = true
				return
			}
			// A single bare allowed call stores the expression verbatim
			// on every non-BLOB/TEXT/JSON target (the 1101 gate ran
			// above): uuid() is observed verbatim on varchar, char, int,
			// bigint, decimal, double, float, year, bit, enum, set, time,
			// date, datetime, timestamp, and varbinary; json_object and
			// json_array are observed verbatim on varchar, int, decimal,
			// date, and timestamp. Evaluation is deferred to INSERT, so
			// no target coercion runs at the setter.
			return
		}
		if allFunctionsIn(facts.Functions, tidbAlterFoldableFunctions) {
			// A bare (current_timestamp) call folds to a timestamp
			// literal: temporal/DATE targets store the verbatim
			// CURRENT_TIMESTAMP default, the positive bases store the
			// folded literal (BIGINT/DOUBLE take the numeric
			// YYYYMMDDHHMMSS form, TIME takes the time part), and the
			// observed rejections answer 1067.
			switch {
			case tidbTimestampFoldTemporalTargets[priorBase] || tidbTimestampFoldPositiveTargets[priorBase]:
				return
			case tidbTimestampFoldIssueTargets[priorBase]:
				assessment.appendIssue(column.Name, AdvancedFeatureExpressionDefault, item.action, AdvancedReasonTiDBDefaultExpression, "")
				return
			default:
				assessment.appendGap(AdvancedFeatureExpressionDefault, AdvancedReasonIncompleteSourceColumn, AdvancedFactSourceDefaultComposite)
				assessment.UnresolvedTransition = true
				return
			}
		}
		// A call mixing expression-preserving and foldable callees is
		// unobserved.
		assessment.appendGap(AdvancedFeatureExpressionDefault, AdvancedReasonIncompleteSourceColumn, AdvancedFactSourceDefaultComposite)
		assessment.UnresolvedTransition = true
	case facts.TopForm == ExpressionTopCall:
		// A root-wrapped call takes the evaluation path — the callee
		// allowlist does not apply (observed ((lower('x'))) folds to
		// 'x'). The folded literal's coercion decides the cell; only the
		// uuid and lower literal folds are observed. ((current_timestamp))
		// and ((now())) are observed acceptances with a NULL catalog
		// default — an unprovable stored shape, so every other callee
		// composite stays a bounded gap.
		if len(facts.Functions) == 1 {
			switch strings.ToLower(facts.Functions[0]) {
			case "uuid":
				switch {
				case tidbStringFoldTargets[priorBase]:
					return
				case tidbUUIDFoldRejects(priorBase):
					assessment.appendIssue(column.Name, AdvancedFeatureExpressionDefault, item.action, AdvancedReasonTiDBDefaultExpression, "")
					return
				default:
					assessment.appendGap(AdvancedFeatureExpressionDefault, AdvancedReasonIncompleteSourceColumn, AdvancedFactSourceDefaultComposite)
					assessment.UnresolvedTransition = true
					return
				}
			case "lower":
				switch priorBase {
				case "varchar":
					return
				case "int":
					assessment.appendIssue(column.Name, AdvancedFeatureExpressionDefault, item.action, AdvancedReasonTiDBDefaultExpression, "")
					return
				default:
					assessment.appendGap(AdvancedFeatureExpressionDefault, AdvancedReasonIncompleteSourceColumn, AdvancedFactSourceDefaultComposite)
					assessment.UnresolvedTransition = true
					return
				}
			}
		}
		assessment.appendGap(AdvancedFeatureExpressionDefault, AdvancedReasonIncompleteSourceColumn, AdvancedFactSourceDefaultComposite)
		assessment.UnresolvedTransition = true
	case facts.TopForm == ExpressionTopCase && !facts.RootParenthesized:
		// A ref-free CASE of the one observed callee folds to the same
		// uuid string literal as a wrapped uuid() call, so it shares the
		// folded-literal outcome table.
		if allFunctionsIn(facts.Functions, tidbAlterCaseCallees) {
			switch {
			case tidbStringFoldTargets[priorBase]:
				return
			case tidbUUIDFoldRejects(priorBase):
				assessment.appendIssue(column.Name, AdvancedFeatureExpressionDefault, item.action, AdvancedReasonTiDBDefaultExpression, "")
				return
			}
		}
		assessment.appendGap(AdvancedFeatureExpressionDefault, AdvancedReasonIncompleteSourceColumn, AdvancedFactSourceDefaultComposite)
		assessment.UnresolvedTransition = true
	case facts.TopForm == ExpressionTopArithmetic || facts.TopForm == ExpressionTopUnary:
		if len(facts.Functions) == 0 {
			// Pure constant arithmetic folds to a numeric literal —
			// observed stored on string, numeric, time, enum, set, bit,
			// and year targets. DATE/DATETIME/TIMESTAMP accept but store
			// a NULL catalog default, an unprovable shape.
			if tidbArithFoldTargets(priorBase) {
				return
			}
			assessment.appendGap(AdvancedFeatureExpressionDefault, AdvancedReasonIncompleteSourceColumn, AdvancedFactSourceDefaultComposite)
			assessment.UnresolvedTransition = true
			return
		}
		if facts.RootParenthesized {
			// A wrapped function-bearing composite is an unobserved eval
			// cell.
			assessment.appendGap(AdvancedFeatureExpressionDefault, AdvancedReasonIncompleteSourceColumn, AdvancedFactSourceDefaultComposite)
			assessment.UnresolvedTransition = true
			return
		}
		// A function-bearing numeric composite is eagerly evaluated and
		// rejected at the coercion boundary (observed uuid()+1 -> 1292).
		assessment.appendIssue(column.Name, AdvancedFeatureExpressionDefault, item.action, AdvancedReasonTiDBDefaultExpression, "")
	default:
		assessment.appendGap(AdvancedFeatureExpressionDefault, AdvancedReasonIncompleteSourceColumn, AdvancedFactSourceDefaultComposite)
		assessment.UnresolvedTransition = true
	}
}

// TiDB SET DEFAULT publication kinds: the catalog-visible shape a positive
// setter outcome stores. The ordered-state seam publishes exactly the
// matching post-state — never a computed value.
const (
	// TiDBSetDefaultVerbatim keeps the declaration's expression facts.
	TiDBSetDefaultVerbatim = "verbatim"
	// TiDBSetDefaultTemporal publishes a CURRENT_TIMESTAMP temporal
	// default (the catalog-visible shape of the foldable-timestamp cell
	// on temporal/DATE targets).
	TiDBSetDefaultTemporal = "temporal"
	// TiDBSetDefaultLiteral publishes literal default presence only: no
	// expression facts, no computed value.
	TiDBSetDefaultLiteral = "literal"
)

// TiDBSetDefaultPublication mirrors the assessTiDBAlterExpressionDefault
// positive cells so the ordered-state seam publishes the same catalog shape
// the native setter stored: verbatim expression facts, a verbatim temporal
// CURRENT_TIMESTAMP default, or folded literal presence. Any cell the
// assessment did not mark positive returns "" — those identities are
// tombstoned before publication anyway, so the empty result is unreachable
// on the gated path.
func TiDBSetDefaultPublication(facts *ExpressionFacts, priorType string) string {
	if facts == nil || !facts.Known || len(facts.LocalReferences()) > 0 {
		return ""
	}
	base := baseColumnTypeName(priorType)
	switch {
	case facts.TopForm == ExpressionTopCall && !facts.RootParenthesized:
		if allFunctionsIn(facts.Functions, tidbAlterDefaultFunctions) {
			// A single bare allowed call stores the expression verbatim
			// on every non-BLOB/TEXT/JSON target.
			if len(facts.Functions) == 1 {
				return TiDBSetDefaultVerbatim
			}
			return ""
		}
		if allFunctionsIn(facts.Functions, tidbAlterFoldableFunctions) {
			switch {
			case tidbTimestampFoldTemporalTargets[base]:
				return TiDBSetDefaultTemporal
			case tidbTimestampFoldPositiveTargets[base]:
				return TiDBSetDefaultLiteral
			}
		}
		return ""
	case facts.TopForm == ExpressionTopCall:
		// Root-wrapped calls fold: literal presence on the observed
		// folded-literal target cells.
		if len(facts.Functions) != 1 {
			return ""
		}
		switch strings.ToLower(facts.Functions[0]) {
		case "uuid":
			if tidbStringFoldTargets[base] {
				return TiDBSetDefaultLiteral
			}
		case "lower":
			if base == "varchar" {
				return TiDBSetDefaultLiteral
			}
		}
		return ""
	case facts.TopForm == ExpressionTopCase && !facts.RootParenthesized:
		if allFunctionsIn(facts.Functions, tidbAlterCaseCallees) && tidbStringFoldTargets[base] {
			return TiDBSetDefaultLiteral
		}
		return ""
	case facts.TopForm == ExpressionTopArithmetic || facts.TopForm == ExpressionTopUnary:
		if len(facts.Functions) == 0 && tidbArithFoldTargets(base) {
			return TiDBSetDefaultLiteral
		}
	}
	return ""
}

// TiDBLiteralSetDefaultPublication mirrors the literal-class
// SET DEFAULT (current_timestamp) cell for the ordered-state seam: the
// setter stores a verbatim temporal default on the temporal bases and a
// folded literal on the positive bases — every other target is a
// non-positive cell the assessment already gated.
func TiDBLiteralSetDefaultPublication(priorType string) string {
	base := baseColumnTypeName(priorType)
	switch {
	case tidbTimestampFoldTemporalTargets[base]:
		return TiDBSetDefaultTemporal
	case tidbTimestampFoldPositiveTargets[base]:
		return TiDBSetDefaultLiteral
	}
	return ""
}

// tidbAlterCaseCallees are the callees observed folding inside a CASE
// default on the TiDB setter path — uuid() only.
var tidbAlterCaseCallees = map[string]bool{"uuid": true}

// tidbStringFoldTargets are the normalized prior-type bases where a folded
// string literal stores verbatim — observed for the wrapped-uuid fold on
// varchar, char, binary, and varbinary, and the uuid-CASE fold on varchar
// and char (the folded literal is identical, so the table is shared).
var tidbStringFoldTargets = map[string]bool{
	"varchar": true, "char": true, "binary": true, "varbinary": true,
}

// tidbUUIDFoldRejects reports whether the normalized prior-type base is an
// observed rejection for the folded uuid string literal: numeric families
// and YEAR/BIT answer 1067, value-restricted ENUM/SET answer 1067, and TIME
// answers 1292 — all observed. DATE/DATETIME/TIMESTAMP accept but store a
// NULL catalog default and stay bounded gaps.
func tidbUUIDFoldRejects(base string) bool {
	return numericFamilyType(base) || base == "enum" || base == "set" || base == "time"
}

// tidbArithFoldTargets reports whether the normalized prior-type base is an
// observed positive for a folded constant-numeric literal: string and
// numeric families (including YEAR and BIT) plus TIME, ENUM, and SET — all
// observed storing the folded value. DATE/DATETIME/TIMESTAMP accept but
// store a NULL catalog default and stay bounded gaps.
func tidbArithFoldTargets(base string) bool {
	if numericFamilyType(base) {
		return true
	}
	switch base {
	case "varchar", "char", "binary", "varbinary", "time", "enum", "set":
		return true
	}
	return false
}

// tidbTimestampFoldTemporalTargets are the normalized prior-type bases
// where a folded current_timestamp default stores the verbatim temporal
// CURRENT_TIMESTAMP shape (observed TIMESTAMP, DATETIME, DATE).
var tidbTimestampFoldTemporalTargets = map[string]bool{
	"timestamp": true, "datetime": true, "date": true,
}

// tidbTimestampFoldPositiveTargets are the normalized prior-type bases
// where a folded current_timestamp default stores a literal: VARCHAR and
// CHAR take the string form, BIGINT and DOUBLE take the numeric
// YYYYMMDDHHMMSS form, and TIME takes the time part (all observed).
var tidbTimestampFoldPositiveTargets = map[string]bool{
	"varchar": true, "char": true, "bigint": true, "double": true, "time": true,
}

// tidbTimestampFoldIssueTargets are the normalized prior-type bases where a
// folded current_timestamp default is a determined 1067 rejection —
// observed on INT, DECIMAL, and YEAR (the numeric YYYYMMDDHHMMSS form
// overflows INT and YEAR and DECIMAL rejects the coercion).
var tidbTimestampFoldIssueTargets = map[string]bool{
	"int": true, "decimal": true, "year": true,
}

// numericFamilyType reports whether the normalized base type is a numeric
// family — the observed 1067 rejection cell for folded non-numeric literals
// (uuid/CASE and current_timestamp on INT).
func numericFamilyType(typ string) bool {
	switch typ {
	case "tinyint", "smallint", "mediumint", "int", "integer", "bigint",
		"decimal", "numeric", "float", "double", "real", "bit", "year":
		return true
	}
	return false
}

// baseColumnTypeName normalizes a declared or catalog column type to its
// lowercase base name without parameters.
func baseColumnTypeName(typ string) string {
	base := strings.ToLower(strings.TrimSpace(typ))
	if i := strings.IndexByte(base, '('); i >= 0 {
		base = strings.TrimSpace(base[:i])
	}
	return base
}

// blobTextJSONFamilyType reports whether the normalized base type cannot
// carry any default on the TiDB ALTER path (observed 1101).
func blobTextJSONFamilyType(typ string) bool {
	switch typ {
	case "json", "blob", "tinyblob", "mediumblob", "longblob",
		"text", "tinytext", "mediumtext", "longtext":
		return true
	}
	return false
}

// temporalFamilyType reports whether the normalized base type accepts a
// temporal expression default on the TiDB CREATE path.
func temporalFamilyType(typ string) bool {
	return typ == "timestamp" || typ == "datetime"
}

// assessVisibilityFeature applies the MySQL visibility version gate and the
// all-invisible structural check. TiDB visibility is a vendor boundary owned
// by the coverage layer, not a compatibility finding — but it is still a
// determined negative for publication.
func assessVisibilityFeature(assessment *AdvancedColumnAssessment, ddl *DDL, item advancedColumnFact, snapshot *TableSnapshot, version *VersionIdentity, versionKnown bool, dialect Dialect) {
	column := item.column
	if dialect == DialectTiDB {
		assessment.InvalidTransition = true
		return
	}
	if versionKnown && !versionAtLeast(version, advancedMinimumVisibilityMySQL) {
		assessment.appendIssue(column.Name, AdvancedFeatureVisibility, item.action, AdvancedReasonVersionUnsupported, AdvancedMinimumVisibilityMySQL)
		return
	}
	if !versionKnown || column.Visibility != ColumnVisibilityInvisible {
		return
	}
	switch item.action {
	case "create_table":
		if allColumnsInvisible(ddl.Columns) {
			assessment.appendIssue(column.Name, AdvancedFeatureVisibility, item.action, AdvancedReasonNoVisibleColumns, "")
		}
	case "set_column_visibility":
		switch {
		case snapshot == nil || snapshot.Columns == nil:
			assessment.appendGap(AdvancedFeatureVisibility, AdvancedReasonUnknownTableState, AdvancedFactTargetTableColumns)
			assessment.UnresolvedTransition = true
		default:
			allInvisible, withheld := allButInvisible(snapshot.Columns, column.Name)
			switch {
			case withheld:
				assessment.appendGap(AdvancedFeatureVisibility, AdvancedReasonIncompleteSourceColumn, AdvancedFactSourceVisibility)
				assessment.UnresolvedTransition = true
			case allInvisible:
				assessment.appendIssue(column.Name, AdvancedFeatureVisibility, item.action, AdvancedReasonNoVisibleColumns, "")
			}
		}
	}
}

// effectiveVisibility resolves a declaration's own visibility fact: explicit
// invisible wins, everything else on a declaration resolves to visible.
func effectiveVisibility(column *Column) string {
	if column != nil && column.Visibility == ColumnVisibilityInvisible {
		return ColumnVisibilityInvisible
	}
	return ColumnVisibilityVisible
}

func allColumnsInvisible(columns []Column) bool {
	if len(columns) == 0 {
		return false
	}
	for i := range columns {
		if effectiveVisibility(&columns[i]) != ColumnVisibilityInvisible {
			return false
		}
	}
	return true
}

// allButInvisible reports whether every column other than name is already
// effectively invisible — detecting a SET INVISIBLE that removes the last
// visible column. Withheld catalog visibility (empty) never resolves to
// visible; it reports withheld=true instead of a provable answer.
func allButInvisible(columns []Column, name string) (allInvisible, withheld bool) {
	seen := false
	for i := range columns {
		if strings.EqualFold(strings.TrimSpace(columns[i].Name), strings.TrimSpace(name)) {
			seen = true
			continue
		}
		if columns[i].Visibility == ColumnVisibilityInvisible {
			continue
		}
		if columns[i].Visibility == "" && !columns[i].VisibilityExplicit {
			return false, true
		}
		return false, false
	}
	return seen, false
}

// assessGeneratedReferencePremises validates the declaration-time reference
// premises of generated columns — the check ColumnDependents cannot cover
// because it only fires at destructive ALTER time. The candidate column set
// is the provable post-statement ordering: the complete CREATE declaration
// set, or the prior set plus this statement's column declarations for a
// provable single-column-clause ALTER. Native semantics are identical on
// all four anchors: a self-reference, an absent name, or a reference to a
// generated column not ordered earlier is a determined rejection; an
// ordinary-column reference at any ordinal is legal. Same-statement binds,
// positioned declarations, and name-space-touching clauses are unprovable
// and stay bounded gaps — never silent passes.
func assessGeneratedReferencePremises(assessment *AdvancedColumnAssessment, ddl *DDL, snapshot *TableSnapshot) {
	if ddl == nil {
		return
	}
	var table Table
	if snapshot != nil && snapshot.Table != nil {
		table = *snapshot.Table
	}
	if table.Name == "" && ddl.Table != nil {
		table = *ddl.Table
	}

	// generatedDecl is one generated column definition plus its position
	// inside the candidate ordering.
	type generatedDecl struct {
		column     *Column
		index      int
		positioned bool
		located    bool // false when the self position is not provable
	}

	// premiseContext is the provable candidate view for one statement.
	type premiseContext struct {
		candidates []*Column
		declared   map[int]bool // candidates (re)declared by this statement
		touched    map[int]bool // prior candidates dropped/renamed this statement
		tainted    bool         // name-space-changing clause present
		priorKnown bool
	}

	ctx := premiseContext{declared: map[int]bool{}, touched: map[int]bool{}}
	var decls []generatedDecl

	switch ddl.Operation {
	case DDLOperationCreateTable:
		ctx.priorKnown = true
		for i := range ddl.Columns {
			ctx.candidates = append(ctx.candidates, &ddl.Columns[i])
			if ddl.Columns[i].Generated != nil {
				decls = append(decls, generatedDecl{column: &ddl.Columns[i], index: i, located: true})
			}
		}
	case DDLOperationAlterTable:
		ctx.priorKnown = snapshot != nil && snapshot.Columns != nil
		if ctx.priorKnown {
			for i := range snapshot.Columns {
				ctx.candidates = append(ctx.candidates, &snapshot.Columns[i])
			}
		}
		for i := range ddl.Alter {
			alter := &ddl.Alter[i]
			switch alter.Action {
			case "drop_column", "rename_column":
				ctx.tainted = true
				if ctx.priorKnown {
					if idx, ok := priorIndexByName(snapshot.Columns, alter.Name); ok {
						ctx.touched[idx] = true
					}
				}
			case "change_column":
				// CHANGE can rename: the name space moves even when the
				// old name stays, so absence is not provable around it.
				ctx.tainted = true
				fallthrough
			case "add_columns", "modify_column":
				if alter.Column == nil || alter.Column.Definition == nil {
					continue
				}
				def := alter.Column.Definition
				if alter.Action == "add_columns" {
					idx := len(ctx.candidates)
					ctx.candidates = append(ctx.candidates, def)
					ctx.declared[idx] = true
					if def.Generated != nil {
						decls = append(decls, generatedDecl{column: def, index: idx, positioned: alter.HasColumnPosition, located: ctx.priorKnown})
					}
					continue
				}
				// modify/change substitute the definition at the prior
				// position; an unresolvable prior name stays owned by the
				// existence checks and contributes no premise verdict.
				if ctx.priorKnown {
					source := alterSourceName(def.Name, alter.Column.OldName)
					if idx, ok := priorIndexByName(snapshot.Columns, source); ok {
						ctx.candidates[idx] = def
						ctx.declared[idx] = true
						if alter.Action == "change_column" && !strings.EqualFold(def.Name, source) {
							ctx.touched[idx] = true
						}
						if def.Generated != nil {
							decls = append(decls, generatedDecl{column: def, index: idx, positioned: alter.HasColumnPosition, located: true})
						}
					}
					// Prior set known but name absent: the existence checks
					// own the outcome — no premise verdict from this seam.
					continue
				}
				if def.Generated != nil {
					decls = append(decls, generatedDecl{column: def, positioned: alter.HasColumnPosition})
				}
			}
		}
	default:
		return
	}
	if len(decls) == 0 {
		return
	}

	indexByName, resolved := columnIndexByName(ctx.candidates)
	for _, decl := range decls {
		expr := &decl.column.Generated.Expression
		if !expr.Known {
			// The expression completeness gap is already reported per
			// column; the premise check needs the full reference set, so an
			// unknown fact cannot produce either a negative or a positive.
			continue
		}
		if len(expr.References) == 0 {
			continue
		}
		declaredOnce := false
		gap := func(reason, fact string) {
			if !declaredOnce {
				assessment.appendGap(AdvancedFeatureGenerated, reason, fact)
				assessment.UnresolvedTransition = true
				declaredOnce = true
			}
		}
		issued := false
		for _, ref := range expr.References {
			if issued {
				break
			}
			if ref.Qualified() {
				if _, ok := bindColumnReference(table, indexByName, ref); !ok && !qualifierMatchesTable(table, ref) {
					// The qualifier provably does not name the owning
					// table — the reference cannot be bound locally.
					gap(AdvancedReasonIncompleteSourceColumn, AdvancedFactSourceExprDependencies)
					continue
				}
			}
			name := strings.ToLower(strings.TrimSpace(ref.Name))
			idx, bound := indexByName[name]
			action := ddlActionName(ddl, decl.column)
			switch {
			case !resolved || !decl.located:
				// The candidate set or the self position is unprovable —
				// bounded gap, never a guessed membership answer.
				gap(AdvancedReasonIncompleteSourceColumn, AdvancedFactSourceExprDependencies)
			case !bound:
				if ctx.tainted || !ctx.priorKnown {
					// A name-space-changing clause or an unproven prior set
					// makes absence unprovable.
					gap(AdvancedReasonIncompleteSourceColumn, AdvancedFactSourceExprDependencies)
					continue
				}
				assessment.appendIssue(decl.column.Name, AdvancedFeatureGenerated, action, AdvancedReasonGeneratedMissingColumn, "")
				issued = true
			case idx == decl.index:
				assessment.appendIssue(decl.column.Name, AdvancedFeatureGenerated, action, AdvancedReasonGeneratedSelfReference, "")
				issued = true
			case ctx.declared[idx] || ctx.touched[idx]:
				// A bind to a column (re)declared, dropped, or renamed in
				// the same statement is a net effect no single-action
				// semantics can prove.
				gap(AdvancedReasonIncompleteAlterDepEffects, AdvancedFactAlterColumnDepEffects)
			case decl.positioned && ctx.candidates[idx].Generated != nil:
				// A FIRST|AFTER clause moves the declaration's position, so
				// "referenced generated column ordered earlier" is not
				// provable.
				gap(AdvancedReasonIncompleteAlterDepEffects, AdvancedFactAlterColumnPosition)
			case ctx.candidates[idx].Generated != nil && idx > decl.index:
				assessment.appendIssue(decl.column.Name, AdvancedFeatureGenerated, action, AdvancedReasonGeneratedColumnOrder, "")
				issued = true
			}
		}
	}
}

// columnIndexByName builds the name → position index for a candidate column
// set; resolved=false marks a set whose member names cannot be trusted
// (empty or duplicated names), matching ColumnDependents' convention.
func columnIndexByName(columns []*Column) (map[string]int, bool) {
	index := make(map[string]int, len(columns))
	resolved := true
	for i, column := range columns {
		name := strings.ToLower(strings.TrimSpace(column.Name))
		if name == "" {
			resolved = false
			continue
		}
		if _, dup := index[name]; dup {
			resolved = false
			continue
		}
		index[name] = i
	}
	return index, resolved
}

// priorIndexByName resolves a prior snapshot column name to its position.
func priorIndexByName(columns []Column, name string) (int, bool) {
	lower := strings.ToLower(strings.TrimSpace(name))
	for i := range columns {
		if strings.ToLower(strings.TrimSpace(columns[i].Name)) == lower {
			return i, true
		}
	}
	return -1, false
}

// qualifierMatchesTable reports whether a qualified reference's schema/table
// qualifiers provably name the owning table — independent of whether the
// column name itself resolves.
func qualifierMatchesTable(table Table, ref ColumnReference) bool {
	if !ref.Qualified() {
		return true
	}
	if !strings.EqualFold(strings.TrimSpace(ref.Table), strings.TrimSpace(table.Name)) {
		return false
	}
	schema := strings.TrimSpace(ref.Schema)
	own := strings.TrimSpace(table.Schema)
	return schema == "" || (own != "" && strings.EqualFold(schema, own))
}

// ddlActionName returns the action label attached to premise issues: the
// statement operation for CREATE, or the clause action for ALTER.
func ddlActionName(ddl *DDL, column *Column) string {
	if ddl.Operation == DDLOperationAlterTable {
		for i := range ddl.Alter {
			alter := &ddl.Alter[i]
			if alter.Column != nil && alter.Column.Definition == column {
				return alter.Action
			}
		}
		return "alter_table"
	}
	return "create_table"
}

// assessDependencyScope computes the dependency-side answer for DROP and
// identity-changing CHANGE/RENAME actions, one effect per action in
// statement order. A known break is an invalid transition independently of
// rule switches; unknown facts and unbounded multi-action net effects stay
// unresolved. Confirmed-absent tables stay owned by the existence checks.
func assessDependencyScope(assessment *AdvancedColumnAssessment, ddl *DDL, snapshot *TableSnapshot) {
	if ddl == nil || ddl.Operation != DDLOperationAlterTable {
		return
	}
	type scopeAction struct {
		action string
		name   string
	}
	var scope []scopeAction
	for i := range ddl.Alter {
		if name, ok := dependencyScopeAction(ddl.Alter[i]); ok {
			scope = append(scope, scopeAction{action: ddl.Alter[i].Action, name: name})
		}
	}
	if len(scope) == 0 {
		return
	}

	var table Table
	if snapshot != nil && snapshot.Table != nil {
		table = *snapshot.Table
	}
	if table.Name == "" && ddl.Table != nil {
		table = *ddl.Table
	}

	// Only clauses that can create, remove, or rebind column dependencies
	// make a multi-clause statement's net effect unprovable; index,
	// constraint, table-option and visibility clauses cannot.
	depAffecting := 0
	for i := range ddl.Alter {
		if depAffectingAlterAction(ddl.Alter[i].Action) {
			depAffecting++
		}
	}
	multi := depAffecting > 1
	for _, item := range scope {
		effect := ColumnDependencyEffect{Column: item.name, Action: item.action}
		switch {
		case snapshot != nil && !snapshot.Exists:
			// Confirmed absence: existence rules own the outcome.
			effect.Known = true
		case snapshot == nil || snapshot.Columns == nil:
			effect.Known = false
			assessment.appendDependencyGap(AdvancedReasonUnknownTableState, AdvancedFactTargetTableColumns)
			assessment.UnresolvedTransition = true
		case multi:
			// A multi-action statement can remove or repair dependents in
			// the same clause, so a per-action break is not a provable net
			// effect — answer unknown without a determined negative.
			effect.Known = false
			assessment.appendDependencyGap(AdvancedReasonIncompleteAlterDepEffects, AdvancedFactAlterColumnDepEffects)
			assessment.UnresolvedTransition = true
		default:
			dep := ColumnDependents(table, snapshot.Columns, item.name)
			effect.Dependents = dep.Dependents
			effect.Known = dep.Known
			if !dep.Known {
				assessment.appendDependencyGap(AdvancedReasonIncompleteSourceColumn, AdvancedFactSourceExprDependencies)
				assessment.UnresolvedTransition = true
			}
			if len(dep.Dependents) > 0 {
				assessment.InvalidTransition = true
			}
		}
		assessment.DependencyEffects = append(assessment.DependencyEffects, effect)
	}
}

// depAffectingAlterAction reports whether an ALTER clause can change the
// dependency answer: column add/modify/change/drop/rename, default setters
// and an owning-table rename can repair or remove dependents in the same
// statement, while index, constraint, table-option and visibility clauses
// cannot. A dependency-scope action co-occurring only with non-affecting
// clauses keeps its provable per-action answer.
func depAffectingAlterAction(action string) bool {
	switch action {
	case "add_columns", "modify_column", "change_column", "drop_column", "rename_column", "rename_table", "set_column_default", "drop_column_default":
		return true
	}
	return false
}
