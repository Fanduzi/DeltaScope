// Package queryaccess owns the immutable MySQL/TiDB builtin semantic manifest.
// input: application-owned profile and parser-shape entries
// output: copy-safe semantic manifests and the session-only service capability
// pos: private semantic proof root, independent from PostgreSQL catalog trust
package queryaccess

import (
	"errors"
	"fmt"
	"strings"
)

var ErrBuiltinSemanticManifestInvalid = errors.New("invalid builtin semantic manifest")

type BuiltinSemanticCallClass string

const (
	BuiltinSemanticAggregate BuiltinSemanticCallClass = "aggregate"
	BuiltinSemanticWindow    BuiltinSemanticCallClass = "window"
	BuiltinSemanticScalar    BuiltinSemanticCallClass = "scalar"
)

// BuiltinSemanticEntry describes one exact native call shape. It contains no
// server identity, trust bit, catalog identity, or caller-provided evidence.
type BuiltinSemanticEntry struct {
	Dialect      string
	Profile      AnalysisProfile
	Name         string
	CallClass    BuiltinSemanticCallClass
	Arity        int
	OperandKinds []string

	// MinArity and MaxArity support variable-arity functions (e.g. COALESCE).
	// When MinArity > 0, the entry uses range-based arity matching:
	// candidate.Arity >= MinArity && (MaxArity == 0 || candidate.Arity <= MaxArity).
	// MaxArity == 0 means unlimited. Fixed-arity entries leave both at zero
	// and use Arity for exact match.
	MinArity int
	MaxArity int

	AllowFilter          bool
	AllowDistinct        bool
	AllowAggOrder        bool
	AllowWithinGroup     bool
	AllowFrame           bool
	AllowNamedWindow     bool
	AllowWindowPartition bool
	AllowWindowOrder     bool

	// RequireWindowPartition and RequireWindowOrder enforce that the parser
	// observed the corresponding clause with direct column operands. This is
	// stricter than MySQL's syntax contract (which accepts ranking windows
	// without ORDER BY): the design's "strict partition/order dependencies"
	// boundary deliberately fails closed when either clause is absent.
	RequireWindowPartition bool
	RequireWindowOrder     bool
}

// BuiltinSemanticManifest is immutable after construction. Entries are
// returned only through deep-copy accessors.
type BuiltinSemanticManifest struct {
	entries []BuiltinSemanticEntry
}

// NewBuiltinSemanticManifest validates and deep-copies application-owned data.
func NewBuiltinSemanticManifest(entries []BuiltinSemanticEntry) (*BuiltinSemanticManifest, error) {
	copied := make([]BuiltinSemanticEntry, len(entries))
	seen := make(map[string]struct{}, len(entries))
	for i, entry := range entries {
		if err := validateBuiltinSemanticEntry(entry); err != nil {
			return nil, fmt.Errorf("%w: entry %d: %v", ErrBuiltinSemanticManifestInvalid, i, err)
		}
		key := builtinSemanticEntryKey(entry)
		if _, exists := seen[key]; exists {
			return nil, fmt.Errorf("%w: duplicate entry", ErrBuiltinSemanticManifestInvalid)
		}
		seen[key] = struct{}{}
		copied[i] = cloneBuiltinSemanticEntry(entry)
	}
	return &BuiltinSemanticManifest{entries: copied}, nil
}

// Entries returns a deep copy of the manifest entries.
func (m *BuiltinSemanticManifest) Entries() []BuiltinSemanticEntry {
	if m == nil {
		return nil
	}
	entries := make([]BuiltinSemanticEntry, len(m.entries))
	for i, entry := range m.entries {
		entries[i] = cloneBuiltinSemanticEntry(entry)
	}
	return entries
}

func validateBuiltinSemanticEntry(entry BuiltinSemanticEntry) error {
	if entry.Dialect != "mysql" && entry.Dialect != "tidb" {
		return fmt.Errorf("unsupported dialect")
	}
	if entry.Profile == AnalysisProfileEmpty {
		return fmt.Errorf("missing profile")
	}
	if err := ValidateAnalysisProfile(entry.Profile, entry.Dialect); err != nil {
		return err
	}
	if entry.Name == "" || entry.Name != strings.ToLower(entry.Name) || strings.TrimSpace(entry.Name) != entry.Name {
		return fmt.Errorf("invalid native name")
	}
	if entry.CallClass != BuiltinSemanticAggregate && entry.CallClass != BuiltinSemanticWindow && entry.CallClass != BuiltinSemanticScalar {
		return fmt.Errorf("unsupported call class")
	}
	if entry.Arity < 0 {
		return fmt.Errorf("negative arity")
	}
	if entry.MinArity < 0 {
		return fmt.Errorf("negative min arity")
	}
	if entry.MaxArity < 0 {
		return fmt.Errorf("negative max arity")
	}
	if entry.MinArity > 0 && entry.MaxArity > 0 && entry.MinArity > entry.MaxArity {
		return fmt.Errorf("min arity exceeds max arity")
	}
	for _, kind := range entry.OperandKinds {
		if !validBuiltinOperandKind(kind) {
			return fmt.Errorf("unsupported operand kind")
		}
	}
	if entry.CallClass == BuiltinSemanticAggregate && (entry.AllowWindowPartition || entry.AllowWindowOrder || entry.AllowFrame || entry.AllowNamedWindow || entry.RequireWindowPartition || entry.RequireWindowOrder) {
		return fmt.Errorf("window modifier on aggregate")
	}
	if entry.CallClass == BuiltinSemanticScalar && (entry.AllowWindowPartition || entry.AllowWindowOrder || entry.AllowFrame || entry.AllowNamedWindow || entry.RequireWindowPartition || entry.RequireWindowOrder || entry.AllowFilter || entry.AllowDistinct || entry.AllowAggOrder || entry.AllowWithinGroup) {
		return fmt.Errorf("aggregate/window modifier on scalar")
	}
	if entry.RequireWindowPartition && !entry.AllowWindowPartition {
		return fmt.Errorf("require without allow: window partition")
	}
	if entry.RequireWindowOrder && !entry.AllowWindowOrder {
		return fmt.Errorf("require without allow: window order")
	}
	return nil
}

func validBuiltinOperandKind(kind string) bool {
	switch kind {
	case "column", "const", "param", "star", "expr", "subquery":
		return true
	default:
		return false
	}
}

func builtinSemanticEntryKey(entry BuiltinSemanticEntry) string {
	return fmt.Sprintf("%s|%s|%s|%s|%d|%v|%d|%d|%t%t%t%t%t%t%t%t%t%t", entry.Dialect, entry.Profile, entry.Name, entry.CallClass, entry.Arity, entry.OperandKinds, entry.MinArity, entry.MaxArity, entry.AllowFilter, entry.AllowDistinct, entry.AllowAggOrder, entry.AllowWithinGroup, entry.AllowFrame, entry.AllowNamedWindow, entry.AllowWindowPartition, entry.AllowWindowOrder, entry.RequireWindowPartition, entry.RequireWindowOrder)
}

func cloneBuiltinSemanticEntry(entry BuiltinSemanticEntry) BuiltinSemanticEntry {
	entry.OperandKinds = append([]string(nil), entry.OperandKinds...)
	return entry
}

type builtinSemanticRegistry struct {
	manifests map[AnalysisProfile]*BuiltinSemanticManifest
}

func newBuiltinSemanticRegistry(manifests map[AnalysisProfile]*BuiltinSemanticManifest) (*builtinSemanticRegistry, error) {
	registry := &builtinSemanticRegistry{manifests: make(map[AnalysisProfile]*BuiltinSemanticManifest, len(manifests))}
	for profile, manifest := range manifests {
		if profile == AnalysisProfileEmpty || manifest == nil {
			return nil, fmt.Errorf("%w: invalid registry entry", ErrBuiltinSemanticManifestInvalid)
		}
		entries := manifest.Entries()
		for _, entry := range entries {
			if entry.Profile != profile {
				return nil, fmt.Errorf("%w: profile registry mismatch", ErrBuiltinSemanticManifestInvalid)
			}
		}
		copyManifest, err := NewBuiltinSemanticManifest(entries)
		if err != nil {
			return nil, err
		}
		registry.manifests[profile] = copyManifest
	}
	return registry, nil
}

func (r *builtinSemanticRegistry) manifest(profile AnalysisProfile) *BuiltinSemanticManifest {
	if r == nil {
		return nil
	}
	return cloneBuiltinSemanticManifest(r.manifests[profile])
}

func cloneBuiltinSemanticManifest(manifest *BuiltinSemanticManifest) *BuiltinSemanticManifest {
	if manifest == nil {
		return nil
	}
	copyManifest, err := NewBuiltinSemanticManifest(manifest.Entries())
	if err != nil {
		return nil
	}
	return copyManifest
}

func cloneBuiltinSemanticRegistry(registry *builtinSemanticRegistry) *builtinSemanticRegistry {
	if registry == nil {
		return nil
	}
	cloned, err := newBuiltinSemanticRegistry(registry.manifests)
	if err != nil {
		return nil
	}
	return cloned
}

// builtinSemanticProductionRegistry is the immutable, evidence-backed production
// registry. Each entry is backed by primary documentation and live Docker probes
// against the exact server image for its profile:
//
//   - mysql-5.7: aggregates only (MySQL 5.7 has no native ranking-window support).
//   - mysql-8.0: aggregates plus ROW_NUMBER/RANK/DENSE_RANK with direct partition
//     and order columns.
//   - mysql-8.4: aggregates plus ROW_NUMBER/RANK/DENSE_RANK with direct partition
//     and order columns.
//   - tidb-8.5: aggregates plus ROW_NUMBER/RANK/DENSE_RANK with direct partition
//     and order columns, independently evidenced.
//
// Live Docker evidence: MySQL 5.7.44, 8.0.46, 8.4.10, TiDB v8.5.7
// (observed via SELECT VERSION() under docker/query-access-builtin-compose.yaml).
// Boundary probes verify stored-function collision, UDF rejection, qualified/
// quoted builtin rejection, noncanonical spacing rejection without IGNORE_SPACE,
// and IGNORE_SPACE comment-form rejection for every profile.
//
// A profile may ship with fewer entries than another profile. No profile aliases
// another. The registry is constructed once at package init; callers cannot
// mutate it. The session-only service deep-clones it at assembly time.
var builtinSemanticProductionRegistry = mustBuiltinSemanticProductionRegistry()

func mustBuiltinSemanticProductionRegistry() *builtinSemanticRegistry {
	entries := []BuiltinSemanticEntry{
		// MySQL 5.7 aggregates (MySQL 5.7 has no native ranking-window support).
		mysqlAggregateEntry(AnalysisProfileMySQL57, "count", 0, []string{"star"}),
		mysqlAggregateEntry(AnalysisProfileMySQL57, "count", 1, []string{"column"}),
		mysqlAggregateEntry(AnalysisProfileMySQL57, "sum", 1, []string{"column"}),
		mysqlAggregateEntry(AnalysisProfileMySQL57, "avg", 1, []string{"column"}),
		mysqlAggregateEntry(AnalysisProfileMySQL57, "min", 1, []string{"column"}),
		mysqlAggregateEntry(AnalysisProfileMySQL57, "max", 1, []string{"column"}),

		// MySQL 8.0 aggregates (independently evidenced from 5.7).
		mysqlAggregateEntry(AnalysisProfileMySQL80, "count", 0, []string{"star"}),
		mysqlAggregateEntry(AnalysisProfileMySQL80, "count", 1, []string{"column"}),
		mysqlAggregateEntry(AnalysisProfileMySQL80, "sum", 1, []string{"column"}),
		mysqlAggregateEntry(AnalysisProfileMySQL80, "avg", 1, []string{"column"}),
		mysqlAggregateEntry(AnalysisProfileMySQL80, "min", 1, []string{"column"}),
		mysqlAggregateEntry(AnalysisProfileMySQL80, "max", 1, []string{"column"}),
		// MySQL 8.0 ranking windows (direct partition/order columns only).
		mysqlRankingWindowEntry(AnalysisProfileMySQL80, "row_number"),
		mysqlRankingWindowEntry(AnalysisProfileMySQL80, "rank"),
		mysqlRankingWindowEntry(AnalysisProfileMySQL80, "dense_rank"),

		// MySQL 8.4 aggregates (independently evidenced from 5.7 and 8.0).
		mysqlAggregateEntry(AnalysisProfileMySQL84, "count", 0, []string{"star"}),
		mysqlAggregateEntry(AnalysisProfileMySQL84, "count", 1, []string{"column"}),
		mysqlAggregateEntry(AnalysisProfileMySQL84, "sum", 1, []string{"column"}),
		mysqlAggregateEntry(AnalysisProfileMySQL84, "avg", 1, []string{"column"}),
		mysqlAggregateEntry(AnalysisProfileMySQL84, "min", 1, []string{"column"}),
		mysqlAggregateEntry(AnalysisProfileMySQL84, "max", 1, []string{"column"}),
		// MySQL 8.4 ranking windows (direct partition/order columns only).
		mysqlRankingWindowEntry(AnalysisProfileMySQL84, "row_number"),
		mysqlRankingWindowEntry(AnalysisProfileMySQL84, "rank"),
		mysqlRankingWindowEntry(AnalysisProfileMySQL84, "dense_rank"),

		// TiDB 8.5 aggregates (independently evidenced; not copied from MySQL).
		tidbAggregateEntry("count", 0, []string{"star"}),
		tidbAggregateEntry("count", 1, []string{"column"}),
		tidbAggregateEntry("sum", 1, []string{"column"}),
		tidbAggregateEntry("avg", 1, []string{"column"}),
		tidbAggregateEntry("min", 1, []string{"column"}),
		tidbAggregateEntry("max", 1, []string{"column"}),
		// TiDB 8.5 ranking windows (independently evidenced; direct partition/order columns only).
		tidbRankingWindowEntry("row_number"),
		tidbRankingWindowEntry("rank"),
		tidbRankingWindowEntry("dense_rank"),

		// MySQL 5.7 scalar functions (direct-column operands only).
		mysqlScalarEntry(AnalysisProfileMySQL57, "lower", 1, []string{"column"}),
		mysqlScalarEntry(AnalysisProfileMySQL57, "upper", 1, []string{"column"}),
		mysqlScalarEntry(AnalysisProfileMySQL57, "length", 1, []string{"column"}),
		mysqlScalarEntry(AnalysisProfileMySQL57, "char_length", 1, []string{"column"}),
		mysqlScalarEntry(AnalysisProfileMySQL57, "abs", 1, []string{"column"}),
		mysqlScalarEntry(AnalysisProfileMySQL57, "ceil", 1, []string{"column"}),
		mysqlScalarEntry(AnalysisProfileMySQL57, "ceiling", 1, []string{"column"}),
		mysqlScalarEntry(AnalysisProfileMySQL57, "floor", 1, []string{"column"}),
		mysqlVariableArityScalarEntry(AnalysisProfileMySQL57, "coalesce", 2, []string{"column", "column"}),
		mysqlScalarEntry(AnalysisProfileMySQL57, "ifnull", 2, []string{"column", "column"}),
		mysqlScalarEntry(AnalysisProfileMySQL57, "ifnull", 2, []string{"column", "const"}),
		mysqlScalarEntry(AnalysisProfileMySQL57, "nullif", 2, []string{"column", "column"}),

		// MySQL 8.0 scalar functions (independently evidenced from 5.7).
		mysqlScalarEntry(AnalysisProfileMySQL80, "lower", 1, []string{"column"}),
		mysqlScalarEntry(AnalysisProfileMySQL80, "upper", 1, []string{"column"}),
		mysqlScalarEntry(AnalysisProfileMySQL80, "length", 1, []string{"column"}),
		mysqlScalarEntry(AnalysisProfileMySQL80, "char_length", 1, []string{"column"}),
		mysqlScalarEntry(AnalysisProfileMySQL80, "abs", 1, []string{"column"}),
		mysqlScalarEntry(AnalysisProfileMySQL80, "ceil", 1, []string{"column"}),
		mysqlScalarEntry(AnalysisProfileMySQL80, "ceiling", 1, []string{"column"}),
		mysqlScalarEntry(AnalysisProfileMySQL80, "floor", 1, []string{"column"}),
		mysqlVariableArityScalarEntry(AnalysisProfileMySQL80, "coalesce", 2, []string{"column", "column"}),
		mysqlScalarEntry(AnalysisProfileMySQL80, "ifnull", 2, []string{"column", "column"}),
		mysqlScalarEntry(AnalysisProfileMySQL80, "ifnull", 2, []string{"column", "const"}),
		mysqlScalarEntry(AnalysisProfileMySQL80, "nullif", 2, []string{"column", "column"}),

		// MySQL 8.4 scalar functions (independently evidenced from 5.7 and 8.0).
		mysqlScalarEntry(AnalysisProfileMySQL84, "lower", 1, []string{"column"}),
		mysqlScalarEntry(AnalysisProfileMySQL84, "upper", 1, []string{"column"}),
		mysqlScalarEntry(AnalysisProfileMySQL84, "length", 1, []string{"column"}),
		mysqlScalarEntry(AnalysisProfileMySQL84, "char_length", 1, []string{"column"}),
		mysqlScalarEntry(AnalysisProfileMySQL84, "abs", 1, []string{"column"}),
		mysqlScalarEntry(AnalysisProfileMySQL84, "ceil", 1, []string{"column"}),
		mysqlScalarEntry(AnalysisProfileMySQL84, "ceiling", 1, []string{"column"}),
		mysqlScalarEntry(AnalysisProfileMySQL84, "floor", 1, []string{"column"}),
		mysqlVariableArityScalarEntry(AnalysisProfileMySQL84, "coalesce", 2, []string{"column", "column"}),
		mysqlScalarEntry(AnalysisProfileMySQL84, "ifnull", 2, []string{"column", "column"}),
		mysqlScalarEntry(AnalysisProfileMySQL84, "ifnull", 2, []string{"column", "const"}),
		mysqlScalarEntry(AnalysisProfileMySQL84, "nullif", 2, []string{"column", "column"}),

		// TiDB 8.5 scalar functions (independently evidenced).
		tidbScalarEntry("lower", 1, []string{"column"}),
		tidbScalarEntry("upper", 1, []string{"column"}),
		tidbScalarEntry("length", 1, []string{"column"}),
		tidbScalarEntry("char_length", 1, []string{"column"}),
		tidbScalarEntry("abs", 1, []string{"column"}),
		tidbScalarEntry("ceil", 1, []string{"column"}),
		tidbScalarEntry("ceiling", 1, []string{"column"}),
		tidbScalarEntry("floor", 1, []string{"column"}),
		tidbVariableArityScalarEntry("coalesce", 2, []string{"column", "column"}),
		tidbScalarEntry("ifnull", 2, []string{"column", "column"}),
		tidbScalarEntry("ifnull", 2, []string{"column", "const"}),
		tidbScalarEntry("nullif", 2, []string{"column", "column"}),
	}
	manifest, err := NewBuiltinSemanticManifest(entries)
	if err != nil {
		panic(fmt.Errorf("internal: production builtin semantic manifest: %w", err))
	}
	mysql57, err := manifestForProfile(manifest, AnalysisProfileMySQL57)
	if err != nil {
		panic(err)
	}
	mysql80, err := manifestForProfile(manifest, AnalysisProfileMySQL80)
	if err != nil {
		panic(err)
	}
	mysql84, err := manifestForProfile(manifest, AnalysisProfileMySQL84)
	if err != nil {
		panic(err)
	}
	tidb85, err := manifestForProfile(manifest, AnalysisProfileTiDB85)
	if err != nil {
		panic(err)
	}
	registry, err := newBuiltinSemanticRegistry(map[AnalysisProfile]*BuiltinSemanticManifest{
		AnalysisProfileMySQL57: mysql57,
		AnalysisProfileMySQL80: mysql80,
		AnalysisProfileMySQL84: mysql84,
		AnalysisProfileTiDB85:  tidb85,
	})
	if err != nil {
		panic(err)
	}
	return registry
}

func mysqlAggregateEntry(profile AnalysisProfile, name string, arity int, operandKinds []string) BuiltinSemanticEntry {
	return BuiltinSemanticEntry{
		Dialect:      "mysql",
		Profile:      profile,
		Name:         name,
		CallClass:    BuiltinSemanticAggregate,
		Arity:        arity,
		OperandKinds: operandKinds,
	}
}

func tidbAggregateEntry(name string, arity int, operandKinds []string) BuiltinSemanticEntry {
	return BuiltinSemanticEntry{
		Dialect:      "tidb",
		Profile:      AnalysisProfileTiDB85,
		Name:         name,
		CallClass:    BuiltinSemanticAggregate,
		Arity:        arity,
		OperandKinds: operandKinds,
	}
}

// mysqlRankingWindowEntry permits ROW_NUMBER/RANK/DENSE_RANK with direct
// partition and order columns. The design's "strict partition/order
// dependencies" boundary requires both clauses to be present and direct.
// Frames, named windows, FILTER, DISTINCT, aggregate-local ORDER BY, and
// nested operands remain indeterminate.
func mysqlRankingWindowEntry(profile AnalysisProfile, name string) BuiltinSemanticEntry {
	return BuiltinSemanticEntry{
		Dialect:                "mysql",
		Profile:                profile,
		Name:                   name,
		CallClass:              BuiltinSemanticWindow,
		Arity:                  0,
		AllowWindowPartition:   true,
		AllowWindowOrder:       true,
		RequireWindowPartition: true,
		RequireWindowOrder:     true,
	}
}

func tidbRankingWindowEntry(name string) BuiltinSemanticEntry {
	return BuiltinSemanticEntry{
		Dialect:                "tidb",
		Profile:                AnalysisProfileTiDB85,
		Name:                   name,
		CallClass:              BuiltinSemanticWindow,
		Arity:                  0,
		AllowWindowPartition:   true,
		AllowWindowOrder:       true,
		RequireWindowPartition: true,
		RequireWindowOrder:     true,
	}
}

func mysqlScalarEntry(profile AnalysisProfile, name string, arity int, operandKinds []string) BuiltinSemanticEntry {
	return BuiltinSemanticEntry{
		Dialect:      "mysql",
		Profile:      profile,
		Name:         name,
		CallClass:    BuiltinSemanticScalar,
		Arity:        arity,
		OperandKinds: operandKinds,
	}
}

func mysqlVariableArityScalarEntry(profile AnalysisProfile, name string, minArity int, operandKinds []string) BuiltinSemanticEntry {
	return BuiltinSemanticEntry{
		Dialect:      "mysql",
		Profile:      profile,
		Name:         name,
		CallClass:    BuiltinSemanticScalar,
		MinArity:     minArity,
		OperandKinds: operandKinds,
	}
}

func tidbScalarEntry(name string, arity int, operandKinds []string) BuiltinSemanticEntry {
	return BuiltinSemanticEntry{
		Dialect:      "tidb",
		Profile:      AnalysisProfileTiDB85,
		Name:         name,
		CallClass:    BuiltinSemanticScalar,
		Arity:        arity,
		OperandKinds: operandKinds,
	}
}

func tidbVariableArityScalarEntry(name string, minArity int, operandKinds []string) BuiltinSemanticEntry {
	return BuiltinSemanticEntry{
		Dialect:      "tidb",
		Profile:      AnalysisProfileTiDB85,
		Name:         name,
		CallClass:    BuiltinSemanticScalar,
		MinArity:     minArity,
		OperandKinds: operandKinds,
	}
}

func manifestForProfile(all *BuiltinSemanticManifest, profile AnalysisProfile) (*BuiltinSemanticManifest, error) {
	filtered := make([]BuiltinSemanticEntry, 0)
	for _, entry := range all.Entries() {
		if entry.Profile == profile {
			filtered = append(filtered, entry)
		}
	}
	return NewBuiltinSemanticManifest(filtered)
}

type builtinSemanticBundle struct {
	schemaResolver SchemaResolver
	registry       *builtinSemanticRegistry
}

func (b *builtinSemanticBundle) validate() error {
	if b == nil || b.schemaResolver == nil || b.registry == nil {
		return fmt.Errorf("%w: incomplete semantic capability", ErrBuiltinSemanticManifestInvalid)
	}
	return nil
}

func newBuiltinSemanticService(schemaResolver SchemaResolver, registry *builtinSemanticRegistry) (*Service, error) {
	clonedRegistry := cloneBuiltinSemanticRegistry(registry)
	bundle := &builtinSemanticBundle{schemaResolver: schemaResolver, registry: clonedRegistry}
	if err := bundle.validate(); err != nil {
		return nil, err
	}
	return &Service{builtinSemantic: bundle}, nil
}

// NewMySQLTiDBSemanticService is the only production constructor for the
// private semantic capability. It accepts only the session-owned resolver;
// manifests remain application-owned. The production registry is populated
// for mysql-5.7, mysql-8.0, mysql-8.4, and tidb-8.5 and is session-only.
func NewMySQLTiDBSemanticService(schemaResolver SchemaResolver) (*Service, error) {
	return newBuiltinSemanticService(schemaResolver, builtinSemanticProductionRegistry)
}
