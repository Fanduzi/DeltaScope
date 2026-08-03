// Package queryaccess defines the Phase-1 PG17 trusted effect manifest.
// input: T2 ledger plus PG17 pure aggregate/window expansion
// output: immutable TrustedEffectManifest for PG17
// pos: T8 Phase-1 manifest; compile-time owned, versioned, deterministically hashed
// note: if this file changes, update this header and module README.md.
package queryaccess

// pg17ManifestData is the Phase-1 trusted effect manifest for PostgreSQL 17.
//
// It contains the T2 ledger's audited minimum closed set:
//   - 54 comparison operators (9 types × 6 ops)
//   - 61 functions: count variants, common typed aggregates, ranking windows,
//     direct-column scalar overloads, COALESCE (variadic any), and NULLIF (any, any)
//
// All OIDs are stable across PostgreSQL 14.23, 16.14, and 17.10.
// Phase-1 runtime claim: PostgreSQL 17 only (ServerVersionNum 170000–179999).
//
// Casts are omitted by default. Function-backed casts are indeterminate.
//
// Semantic audit for all entries:
//   - Unique data dependency is left/right AST operands only (operators) or
//     query row sources (aggregates)
//   - Pure comparison C builtins or standard aggregates
//   - No table/GUC/role/file/network reads
//   - Cannot omit permission-bearing columns already required by operand extraction
var pg17ManifestData = TrustedEffectManifest{
	SchemaVersion:      "1.0",
	PostgreSQLMajorMin: 17,
	PostgreSQLMajorMax: 17,
	Entries:            pg17Entries,
	Hash:               ComputeManifestHash(pg17Entries),
}

// NewPG17Manifest returns a deep copy of the PG17 manifest (immutable safe).
// Entries and nested OperandTypeOIDs slices are copied so callers cannot
// mutate the compile-time manifest backing store.
func NewPG17Manifest() TrustedEffectManifest {
	copied := pg17ManifestData
	copied.Entries = make([]TrustedEffectEntry, len(pg17ManifestData.Entries))
	for i, e := range pg17ManifestData.Entries {
		copied.Entries[i] = e
		if len(e.OperandTypeOIDs) > 0 {
			copied.Entries[i].OperandTypeOIDs = append([]uint32(nil), e.OperandTypeOIDs...)
		}
	}
	return copied
}

// PG17Manifest is deprecated: use NewPG17Manifest() for immutable copies.
// Retained for backward compatibility with existing tests.
var PG17Manifest = NewPG17Manifest()

// pg17Entries is the Phase-1 trusted effect entries for PG17.
// Sorted by (Kind, ObjectOID, NamespaceOID, CanonicalSignature).
var pg17Entries = []TrustedEffectEntry{
	// =========================================================================
	// Common typed aggregates and ranking windows.
	// =========================================================================
	{
		Kind: EffectCandidateFunction, ObjectOID: 2100, NamespaceOID: 11, OperandTypeOIDs: []uint32{20}, ResultTypeOID: 1700, Volatility: EffectVolatilityImmutable,
		CanonicalSignature: "pg_catalog.avg(20)", AuditNotes: "avg(bigint); immutable built-in aggregate; dependency is the input column",
	},
	{
		Kind: EffectCandidateFunction, ObjectOID: 2101, NamespaceOID: 11, OperandTypeOIDs: []uint32{23}, ResultTypeOID: 1700, Volatility: EffectVolatilityImmutable,
		CanonicalSignature: "pg_catalog.avg(23)", AuditNotes: "avg(integer); immutable built-in aggregate; dependency is the input column",
	},
	{
		Kind: EffectCandidateFunction, ObjectOID: 2102, NamespaceOID: 11, OperandTypeOIDs: []uint32{21}, ResultTypeOID: 1700, Volatility: EffectVolatilityImmutable,
		CanonicalSignature: "pg_catalog.avg(21)", AuditNotes: "avg(smallint); immutable built-in aggregate; dependency is the input column",
	},
	{
		Kind: EffectCandidateFunction, ObjectOID: 2103, NamespaceOID: 11, OperandTypeOIDs: []uint32{1700}, ResultTypeOID: 1700, Volatility: EffectVolatilityImmutable,
		CanonicalSignature: "pg_catalog.avg(1700)", AuditNotes: "avg(numeric); immutable built-in aggregate; dependency is the input column",
	},
	{
		Kind: EffectCandidateFunction, ObjectOID: 2104, NamespaceOID: 11, OperandTypeOIDs: []uint32{700}, ResultTypeOID: 701, Volatility: EffectVolatilityImmutable,
		CanonicalSignature: "pg_catalog.avg(700)", AuditNotes: "avg(real); immutable built-in aggregate; dependency is the input column",
	},
	{
		Kind: EffectCandidateFunction, ObjectOID: 2105, NamespaceOID: 11, OperandTypeOIDs: []uint32{701}, ResultTypeOID: 701, Volatility: EffectVolatilityImmutable,
		CanonicalSignature: "pg_catalog.avg(701)", AuditNotes: "avg(double precision); immutable built-in aggregate; dependency is the input column",
	},
	{
		Kind: EffectCandidateFunction, ObjectOID: 2106, NamespaceOID: 11, OperandTypeOIDs: []uint32{1186}, ResultTypeOID: 1186, Volatility: EffectVolatilityImmutable,
		CanonicalSignature: "pg_catalog.avg(1186)", AuditNotes: "avg(interval); immutable built-in aggregate; dependency is the input column",
	},
	{
		Kind: EffectCandidateFunction, ObjectOID: 2107, NamespaceOID: 11, OperandTypeOIDs: []uint32{20}, ResultTypeOID: 1700, Volatility: EffectVolatilityImmutable,
		CanonicalSignature: "pg_catalog.sum(20)", AuditNotes: "sum(bigint); immutable built-in aggregate; dependency is the input column",
	},
	{
		Kind: EffectCandidateFunction, ObjectOID: 2108, NamespaceOID: 11, OperandTypeOIDs: []uint32{23}, ResultTypeOID: 20, Volatility: EffectVolatilityImmutable,
		CanonicalSignature: "pg_catalog.sum(23)", AuditNotes: "sum(integer); immutable built-in aggregate; dependency is the input column",
	},
	{
		Kind: EffectCandidateFunction, ObjectOID: 2109, NamespaceOID: 11, OperandTypeOIDs: []uint32{21}, ResultTypeOID: 20, Volatility: EffectVolatilityImmutable,
		CanonicalSignature: "pg_catalog.sum(21)", AuditNotes: "sum(smallint); immutable built-in aggregate; dependency is the input column",
	},
	{
		Kind: EffectCandidateFunction, ObjectOID: 2110, NamespaceOID: 11, OperandTypeOIDs: []uint32{700}, ResultTypeOID: 700, Volatility: EffectVolatilityImmutable,
		CanonicalSignature: "pg_catalog.sum(700)", AuditNotes: "sum(real); immutable built-in aggregate; dependency is the input column",
	},
	{
		Kind: EffectCandidateFunction, ObjectOID: 2111, NamespaceOID: 11, OperandTypeOIDs: []uint32{701}, ResultTypeOID: 701, Volatility: EffectVolatilityImmutable,
		CanonicalSignature: "pg_catalog.sum(701)", AuditNotes: "sum(double precision); immutable built-in aggregate; dependency is the input column",
	},
	{
		Kind: EffectCandidateFunction, ObjectOID: 2112, NamespaceOID: 11, OperandTypeOIDs: []uint32{790}, ResultTypeOID: 790, Volatility: EffectVolatilityImmutable,
		CanonicalSignature: "pg_catalog.sum(790)", AuditNotes: "sum(money); immutable built-in aggregate; dependency is the input column",
	},
	{
		Kind: EffectCandidateFunction, ObjectOID: 2113, NamespaceOID: 11, OperandTypeOIDs: []uint32{1186}, ResultTypeOID: 1186, Volatility: EffectVolatilityImmutable,
		CanonicalSignature: "pg_catalog.sum(1186)", AuditNotes: "sum(interval); immutable built-in aggregate; dependency is the input column",
	},
	{
		Kind: EffectCandidateFunction, ObjectOID: 2114, NamespaceOID: 11, OperandTypeOIDs: []uint32{1700}, ResultTypeOID: 1700, Volatility: EffectVolatilityImmutable,
		CanonicalSignature: "pg_catalog.sum(1700)", AuditNotes: "sum(numeric); immutable built-in aggregate; dependency is the input column",
	},
	{
		Kind: EffectCandidateFunction, ObjectOID: 2115, NamespaceOID: 11, OperandTypeOIDs: []uint32{20}, ResultTypeOID: 20, Volatility: EffectVolatilityImmutable,
		CanonicalSignature: "pg_catalog.max(20)", AuditNotes: "max(bigint); immutable built-in aggregate; dependency is the input column",
	},
	{
		Kind: EffectCandidateFunction, ObjectOID: 2116, NamespaceOID: 11, OperandTypeOIDs: []uint32{23}, ResultTypeOID: 23, Volatility: EffectVolatilityImmutable,
		CanonicalSignature: "pg_catalog.max(23)", AuditNotes: "max(integer); immutable built-in aggregate; dependency is the input column",
	},
	{
		Kind: EffectCandidateFunction, ObjectOID: 2117, NamespaceOID: 11, OperandTypeOIDs: []uint32{21}, ResultTypeOID: 21, Volatility: EffectVolatilityImmutable,
		CanonicalSignature: "pg_catalog.max(21)", AuditNotes: "max(smallint); immutable built-in aggregate; dependency is the input column",
	},
	{
		Kind: EffectCandidateFunction, ObjectOID: 2119, NamespaceOID: 11, OperandTypeOIDs: []uint32{700}, ResultTypeOID: 700, Volatility: EffectVolatilityImmutable,
		CanonicalSignature: "pg_catalog.max(700)", AuditNotes: "max(real); immutable built-in aggregate; dependency is the input column",
	},
	{
		Kind: EffectCandidateFunction, ObjectOID: 2120, NamespaceOID: 11, OperandTypeOIDs: []uint32{701}, ResultTypeOID: 701, Volatility: EffectVolatilityImmutable,
		CanonicalSignature: "pg_catalog.max(701)", AuditNotes: "max(double precision); immutable built-in aggregate; dependency is the input column",
	},
	{
		Kind: EffectCandidateFunction, ObjectOID: 2122, NamespaceOID: 11, OperandTypeOIDs: []uint32{1082}, ResultTypeOID: 1082, Volatility: EffectVolatilityImmutable,
		CanonicalSignature: "pg_catalog.max(1082)", AuditNotes: "max(date); immutable built-in aggregate; dependency is the input column",
	},
	{
		Kind: EffectCandidateFunction, ObjectOID: 2126, NamespaceOID: 11, OperandTypeOIDs: []uint32{1114}, ResultTypeOID: 1114, Volatility: EffectVolatilityImmutable,
		CanonicalSignature: "pg_catalog.max(1114)", AuditNotes: "max(timestamp); immutable built-in aggregate; dependency is the input column",
	},
	{
		Kind: EffectCandidateFunction, ObjectOID: 2127, NamespaceOID: 11, OperandTypeOIDs: []uint32{1184}, ResultTypeOID: 1184, Volatility: EffectVolatilityImmutable,
		CanonicalSignature: "pg_catalog.max(1184)", AuditNotes: "max(timestamptz); immutable built-in aggregate; dependency is the input column",
	},
	{
		Kind: EffectCandidateFunction, ObjectOID: 2129, NamespaceOID: 11, OperandTypeOIDs: []uint32{25}, ResultTypeOID: 25, Volatility: EffectVolatilityImmutable,
		CanonicalSignature: "pg_catalog.max(25)", AuditNotes: "max(text); immutable built-in aggregate; dependency is the input column",
	},
	{
		Kind: EffectCandidateFunction, ObjectOID: 2130, NamespaceOID: 11, OperandTypeOIDs: []uint32{1700}, ResultTypeOID: 1700, Volatility: EffectVolatilityImmutable,
		CanonicalSignature: "pg_catalog.max(1700)", AuditNotes: "max(numeric); immutable built-in aggregate; dependency is the input column",
	},
	{
		Kind: EffectCandidateFunction, ObjectOID: 2131, NamespaceOID: 11, OperandTypeOIDs: []uint32{20}, ResultTypeOID: 20, Volatility: EffectVolatilityImmutable,
		CanonicalSignature: "pg_catalog.min(20)", AuditNotes: "min(bigint); immutable built-in aggregate; dependency is the input column",
	},
	{
		Kind: EffectCandidateFunction, ObjectOID: 2132, NamespaceOID: 11, OperandTypeOIDs: []uint32{23}, ResultTypeOID: 23, Volatility: EffectVolatilityImmutable,
		CanonicalSignature: "pg_catalog.min(23)", AuditNotes: "min(integer); immutable built-in aggregate; dependency is the input column",
	},
	{
		Kind: EffectCandidateFunction, ObjectOID: 2133, NamespaceOID: 11, OperandTypeOIDs: []uint32{21}, ResultTypeOID: 21, Volatility: EffectVolatilityImmutable,
		CanonicalSignature: "pg_catalog.min(21)", AuditNotes: "min(smallint); immutable built-in aggregate; dependency is the input column",
	},
	{
		Kind: EffectCandidateFunction, ObjectOID: 2135, NamespaceOID: 11, OperandTypeOIDs: []uint32{700}, ResultTypeOID: 700, Volatility: EffectVolatilityImmutable,
		CanonicalSignature: "pg_catalog.min(700)", AuditNotes: "min(real); immutable built-in aggregate; dependency is the input column",
	},
	{
		Kind: EffectCandidateFunction, ObjectOID: 2136, NamespaceOID: 11, OperandTypeOIDs: []uint32{701}, ResultTypeOID: 701, Volatility: EffectVolatilityImmutable,
		CanonicalSignature: "pg_catalog.min(701)", AuditNotes: "min(double precision); immutable built-in aggregate; dependency is the input column",
	},
	{
		Kind: EffectCandidateFunction, ObjectOID: 2138, NamespaceOID: 11, OperandTypeOIDs: []uint32{1082}, ResultTypeOID: 1082, Volatility: EffectVolatilityImmutable,
		CanonicalSignature: "pg_catalog.min(1082)", AuditNotes: "min(date); immutable built-in aggregate; dependency is the input column",
	},
	{
		Kind: EffectCandidateFunction, ObjectOID: 2142, NamespaceOID: 11, OperandTypeOIDs: []uint32{1114}, ResultTypeOID: 1114, Volatility: EffectVolatilityImmutable,
		CanonicalSignature: "pg_catalog.min(1114)", AuditNotes: "min(timestamp); immutable built-in aggregate; dependency is the input column",
	},
	{
		Kind: EffectCandidateFunction, ObjectOID: 2143, NamespaceOID: 11, OperandTypeOIDs: []uint32{1184}, ResultTypeOID: 1184, Volatility: EffectVolatilityImmutable,
		CanonicalSignature: "pg_catalog.min(1184)", AuditNotes: "min(timestamptz); immutable built-in aggregate; dependency is the input column",
	},
	{
		Kind: EffectCandidateFunction, ObjectOID: 2145, NamespaceOID: 11, OperandTypeOIDs: []uint32{25}, ResultTypeOID: 25, Volatility: EffectVolatilityImmutable,
		CanonicalSignature: "pg_catalog.min(25)", AuditNotes: "min(text); immutable built-in aggregate; dependency is the input column",
	},
	{
		Kind: EffectCandidateFunction, ObjectOID: 2146, NamespaceOID: 11, OperandTypeOIDs: []uint32{1700}, ResultTypeOID: 1700, Volatility: EffectVolatilityImmutable,
		CanonicalSignature: "pg_catalog.min(1700)", AuditNotes: "min(numeric); immutable built-in aggregate; dependency is the input column",
	},
	{
		Kind: EffectCandidateFunction, ObjectOID: 2244, NamespaceOID: 11, OperandTypeOIDs: []uint32{1042}, ResultTypeOID: 1042, Volatility: EffectVolatilityImmutable,
		CanonicalSignature: "pg_catalog.max(1042)", AuditNotes: "max(bpchar); immutable built-in aggregate; dependency is the input column",
	},
	{
		Kind: EffectCandidateFunction, ObjectOID: 2245, NamespaceOID: 11, OperandTypeOIDs: []uint32{1042}, ResultTypeOID: 1042, Volatility: EffectVolatilityImmutable,
		CanonicalSignature: "pg_catalog.min(1042)", AuditNotes: "min(bpchar); immutable built-in aggregate; dependency is the input column",
	},
	{
		Kind: EffectCandidateFunction, ObjectOID: 3100, NamespaceOID: 11, ResultTypeOID: 20, Volatility: EffectVolatilityImmutable,
		CanonicalSignature: "pg_catalog.row_number()", AuditNotes: "row_number window function; immutable ranking over query row sources",
	},
	{
		Kind: EffectCandidateFunction, ObjectOID: 3101, NamespaceOID: 11, ResultTypeOID: 20, Volatility: EffectVolatilityImmutable,
		CanonicalSignature: "pg_catalog.rank()", AuditNotes: "rank window function; immutable ranking over query row sources",
	},
	{
		Kind: EffectCandidateFunction, ObjectOID: 3102, NamespaceOID: 11, ResultTypeOID: 20, Volatility: EffectVolatilityImmutable,
		CanonicalSignature: "pg_catalog.dense_rank()", AuditNotes: "dense_rank window function; immutable ranking over query row sources",
	},

	// Existing count aggregates.
	{
		Kind:               EffectCandidateFunction,
		ObjectOID:          2803,
		NamespaceOID:       11, // pg_catalog
		OperandTypeOIDs:    nil,
		ResultTypeOID:      20, // int8
		ImplementationOID:  0,
		Volatility:         EffectVolatilityImmutable,
		CanonicalSignature: "pg_catalog.count()",
		AuditNotes:         "count(*) aggregate; unique data dep = query row sources; no hidden reads",
	},
	{
		Kind:               EffectCandidateFunction,
		ObjectOID:          2147,
		NamespaceOID:       11,             // pg_catalog
		OperandTypeOIDs:    []uint32{2276}, // anyelement (pseudo-type)
		ResultTypeOID:      20,             // int8
		ImplementationOID:  0,
		Volatility:         EffectVolatilityImmutable,
		CanonicalSignature: "pg_catalog.count(2276)",
		AuditNotes:         "count(any) aggregate; catalog arg = any OID 2276; unique data dep = query row sources",
	},

	// Direct-column scalar functions. The catalog OID and argument OIDs bind each
	// overloaded call to its immutable pg_catalog implementation.
	{Kind: EffectCandidateFunction, ObjectOID: 870, NamespaceOID: 11, OperandTypeOIDs: []uint32{25}, ResultTypeOID: 25, Volatility: EffectVolatilityImmutable,
		CanonicalSignature: "pg_catalog.lower(25)", AuditNotes: "lower(text); immutable builtin; dependency is the input column"},
	{Kind: EffectCandidateFunction, ObjectOID: 871, NamespaceOID: 11, OperandTypeOIDs: []uint32{25}, ResultTypeOID: 25, Volatility: EffectVolatilityImmutable,
		CanonicalSignature: "pg_catalog.upper(25)", AuditNotes: "upper(text); immutable builtin; dependency is the input column"},
	{Kind: EffectCandidateFunction, ObjectOID: 1317, NamespaceOID: 11, OperandTypeOIDs: []uint32{25}, ResultTypeOID: 23, Volatility: EffectVolatilityImmutable,
		CanonicalSignature: "pg_catalog.length(25)", AuditNotes: "length(text); immutable builtin; dependency is the input column"},
	{Kind: EffectCandidateFunction, ObjectOID: 1369, NamespaceOID: 11, OperandTypeOIDs: []uint32{25}, ResultTypeOID: 23, Volatility: EffectVolatilityImmutable,
		CanonicalSignature: "pg_catalog.character_length(25)", AuditNotes: "character_length(text); immutable builtin; dependency is the input column"},
	{Kind: EffectCandidateFunction, ObjectOID: 1381, NamespaceOID: 11, OperandTypeOIDs: []uint32{25}, ResultTypeOID: 23, Volatility: EffectVolatilityImmutable,
		CanonicalSignature: "pg_catalog.char_length(25)", AuditNotes: "char_length(text); immutable builtin; dependency is the input column"},
	{Kind: EffectCandidateFunction, ObjectOID: 1394, NamespaceOID: 11, OperandTypeOIDs: []uint32{700}, ResultTypeOID: 700, Volatility: EffectVolatilityImmutable,
		CanonicalSignature: "pg_catalog.abs(700)", AuditNotes: "abs(real); immutable builtin; dependency is the input column"},
	{Kind: EffectCandidateFunction, ObjectOID: 1395, NamespaceOID: 11, OperandTypeOIDs: []uint32{701}, ResultTypeOID: 701, Volatility: EffectVolatilityImmutable,
		CanonicalSignature: "pg_catalog.abs(701)", AuditNotes: "abs(double precision); immutable builtin; dependency is the input column"},
	{Kind: EffectCandidateFunction, ObjectOID: 1396, NamespaceOID: 11, OperandTypeOIDs: []uint32{20}, ResultTypeOID: 20, Volatility: EffectVolatilityImmutable,
		CanonicalSignature: "pg_catalog.abs(20)", AuditNotes: "abs(bigint); immutable builtin; dependency is the input column"},
	{Kind: EffectCandidateFunction, ObjectOID: 1397, NamespaceOID: 11, OperandTypeOIDs: []uint32{23}, ResultTypeOID: 23, Volatility: EffectVolatilityImmutable,
		CanonicalSignature: "pg_catalog.abs(23)", AuditNotes: "abs(integer); immutable builtin; dependency is the input column"},
	{Kind: EffectCandidateFunction, ObjectOID: 1398, NamespaceOID: 11, OperandTypeOIDs: []uint32{21}, ResultTypeOID: 21, Volatility: EffectVolatilityImmutable,
		CanonicalSignature: "pg_catalog.abs(21)", AuditNotes: "abs(smallint); immutable builtin; dependency is the input column"},
	{Kind: EffectCandidateFunction, ObjectOID: 1705, NamespaceOID: 11, OperandTypeOIDs: []uint32{1700}, ResultTypeOID: 1700, Volatility: EffectVolatilityImmutable,
		CanonicalSignature: "pg_catalog.abs(1700)", AuditNotes: "abs(numeric); immutable builtin; dependency is the input column"},
	{Kind: EffectCandidateFunction, ObjectOID: 1711, NamespaceOID: 11, OperandTypeOIDs: []uint32{1700}, ResultTypeOID: 1700, Volatility: EffectVolatilityImmutable,
		CanonicalSignature: "pg_catalog.ceil(1700)", AuditNotes: "ceil(numeric); immutable builtin; dependency is the input column"},
	{Kind: EffectCandidateFunction, ObjectOID: 1712, NamespaceOID: 11, OperandTypeOIDs: []uint32{1700}, ResultTypeOID: 1700, Volatility: EffectVolatilityImmutable,
		CanonicalSignature: "pg_catalog.floor(1700)", AuditNotes: "floor(numeric); immutable builtin; dependency is the input column"},
	{Kind: EffectCandidateFunction, ObjectOID: 2167, NamespaceOID: 11, OperandTypeOIDs: []uint32{1700}, ResultTypeOID: 1700, Volatility: EffectVolatilityImmutable,
		CanonicalSignature: "pg_catalog.ceiling(1700)", AuditNotes: "ceiling(numeric); immutable builtin; dependency is the input column"},
	{Kind: EffectCandidateFunction, ObjectOID: 2308, NamespaceOID: 11, OperandTypeOIDs: []uint32{701}, ResultTypeOID: 701, Volatility: EffectVolatilityImmutable,
		CanonicalSignature: "pg_catalog.ceil(701)", AuditNotes: "ceil(double precision); immutable builtin; dependency is the input column"},
	{Kind: EffectCandidateFunction, ObjectOID: 2309, NamespaceOID: 11, OperandTypeOIDs: []uint32{701}, ResultTypeOID: 701, Volatility: EffectVolatilityImmutable,
		CanonicalSignature: "pg_catalog.floor(701)", AuditNotes: "floor(double precision); immutable builtin; dependency is the input column"},
	{Kind: EffectCandidateFunction, ObjectOID: 2320, NamespaceOID: 11, OperandTypeOIDs: []uint32{701}, ResultTypeOID: 701, Volatility: EffectVolatilityImmutable,
		CanonicalSignature: "pg_catalog.ceiling(701)", AuditNotes: "ceiling(double precision); immutable builtin; dependency is the input column"},

	// Null-handling scalar functions. COALESCE is variadic (provariadic = 2276 "any").
	// NULLIF takes 2 polymorphic args (proargtypes = 2276 2276 "any"). Both are
	// IMMUTABLE builtins whose unique data dependency is the input column(s).
	{Kind: EffectCandidateFunction, ObjectOID: 840, NamespaceOID: 11, OperandTypeOIDs: []uint32{2276}, ResultTypeOID: 2276, Volatility: EffectVolatilityImmutable,
		CanonicalSignature: "pg_catalog.coalesce(2276)", AuditNotes: "coalesce(variadic any); immutable builtin; polymorphic variadic; dependency is the input columns"},
	{Kind: EffectCandidateFunction, ObjectOID: 1706, NamespaceOID: 11, OperandTypeOIDs: []uint32{2276, 2276}, ResultTypeOID: 2276, Volatility: EffectVolatilityImmutable,
		CanonicalSignature: "pg_catalog.nullif(2276,2276)", AuditNotes: "nullif(any, any); immutable builtin; polymorphic 2-arg; dependency is the input columns"},

	// =========================================================================
	// Comparison operators — bool (OID 16) × 6
	// =========================================================================
	{
		Kind:               EffectCandidateOperator,
		ObjectOID:          91,
		NamespaceOID:       11,
		OperandTypeOIDs:    []uint32{16, 16},
		ResultTypeOID:      16,
		ImplementationOID:  60, // booleq
		Volatility:         EffectVolatilityImmutable,
		CanonicalSignature: "pg_catalog.=(16,16)",
		AuditNotes:         "bool = bool; impl booleq; pure C builtin",
	},
	{
		Kind:               EffectCandidateOperator,
		ObjectOID:          92,
		NamespaceOID:       11,
		OperandTypeOIDs:    []uint32{16, 16},
		ResultTypeOID:      16,
		ImplementationOID:  61, // boolne
		Volatility:         EffectVolatilityImmutable,
		CanonicalSignature: "pg_catalog.<>(16,16)",
		AuditNotes:         "bool <> bool; impl boolne; pure C builtin",
	},
	{
		Kind:               EffectCandidateOperator,
		ObjectOID:          58,
		NamespaceOID:       11,
		OperandTypeOIDs:    []uint32{16, 16},
		ResultTypeOID:      16,
		ImplementationOID:  58, // boollt
		Volatility:         EffectVolatilityImmutable,
		CanonicalSignature: "pg_catalog.<(16,16)",
		AuditNotes:         "bool < bool; impl boollt; pure C builtin",
	},
	{
		Kind:               EffectCandidateOperator,
		ObjectOID:          59,
		NamespaceOID:       11,
		OperandTypeOIDs:    []uint32{16, 16},
		ResultTypeOID:      16,
		ImplementationOID:  59, // boolgt
		Volatility:         EffectVolatilityImmutable,
		CanonicalSignature: "pg_catalog.>(16,16)",
		AuditNotes:         "bool > bool; impl boolgt; pure C builtin",
	},
	{
		Kind:               EffectCandidateOperator,
		ObjectOID:          1694,
		NamespaceOID:       11,
		OperandTypeOIDs:    []uint32{16, 16},
		ResultTypeOID:      16,
		ImplementationOID:  1694, // boolle
		Volatility:         EffectVolatilityImmutable,
		CanonicalSignature: "pg_catalog.<=(16,16)",
		AuditNotes:         "bool <= bool; impl boolle; pure C builtin",
	},
	{
		Kind:               EffectCandidateOperator,
		ObjectOID:          1695,
		NamespaceOID:       11,
		OperandTypeOIDs:    []uint32{16, 16},
		ResultTypeOID:      16,
		ImplementationOID:  1695, // boolge
		Volatility:         EffectVolatilityImmutable,
		CanonicalSignature: "pg_catalog.>=(16,16)",
		AuditNotes:         "bool >= bool; impl boolge; pure C builtin",
	},

	// =========================================================================
	// Comparison operators — int2 (OID 21) × 6
	// =========================================================================
	{
		Kind:               EffectCandidateOperator,
		ObjectOID:          412,
		NamespaceOID:       11,
		OperandTypeOIDs:    []uint32{21, 21},
		ResultTypeOID:      16,
		ImplementationOID:  77, // int2eq
		Volatility:         EffectVolatilityImmutable,
		CanonicalSignature: "pg_catalog.=(21,21)",
		AuditNotes:         "int2 = int2; impl int2eq; pure C builtin",
	},
	{
		Kind:               EffectCandidateOperator,
		ObjectOID:          414,
		NamespaceOID:       11,
		OperandTypeOIDs:    []uint32{21, 21},
		ResultTypeOID:      16,
		ImplementationOID:  79, // int2ne
		Volatility:         EffectVolatilityImmutable,
		CanonicalSignature: "pg_catalog.<>(21,21)",
		AuditNotes:         "int2 <> int2; impl int2ne; pure C builtin",
	},
	{
		Kind:               EffectCandidateOperator,
		ObjectOID:          410,
		NamespaceOID:       11,
		OperandTypeOIDs:    []uint32{21, 21},
		ResultTypeOID:      16,
		ImplementationOID:  76, // int2lt
		Volatility:         EffectVolatilityImmutable,
		CanonicalSignature: "pg_catalog.<(21,21)",
		AuditNotes:         "int2 < int2; impl int2lt; pure C builtin",
	},
	{
		Kind:               EffectCandidateOperator,
		ObjectOID:          411,
		NamespaceOID:       11,
		OperandTypeOIDs:    []uint32{21, 21},
		ResultTypeOID:      16,
		ImplementationOID:  78, // int2gt
		Volatility:         EffectVolatilityImmutable,
		CanonicalSignature: "pg_catalog.>(21,21)",
		AuditNotes:         "int2 > int2; impl int2gt; pure C builtin",
	},
	{
		Kind:               EffectCandidateOperator,
		ObjectOID:          413,
		NamespaceOID:       11,
		OperandTypeOIDs:    []uint32{21, 21},
		ResultTypeOID:      16,
		ImplementationOID:  80, // int2le
		Volatility:         EffectVolatilityImmutable,
		CanonicalSignature: "pg_catalog.<=(21,21)",
		AuditNotes:         "int2 <= int2; impl int2le; pure C builtin",
	},
	{
		Kind:               EffectCandidateOperator,
		ObjectOID:          415,
		NamespaceOID:       11,
		OperandTypeOIDs:    []uint32{21, 21},
		ResultTypeOID:      16,
		ImplementationOID:  81, // int2ge
		Volatility:         EffectVolatilityImmutable,
		CanonicalSignature: "pg_catalog.>=(21,21)",
		AuditNotes:         "int2 >= int2; impl int2ge; pure C builtin",
	},

	// =========================================================================
	// Comparison operators — int4 (OID 23) × 6
	// =========================================================================
	{
		Kind:               EffectCandidateOperator,
		ObjectOID:          96,
		NamespaceOID:       11,
		OperandTypeOIDs:    []uint32{23, 23},
		ResultTypeOID:      16,
		ImplementationOID:  65, // int4eq
		Volatility:         EffectVolatilityImmutable,
		CanonicalSignature: "pg_catalog.=(23,23)",
		AuditNotes:         "int4 = int4; impl int4eq; pure C builtin",
	},
	{
		Kind:               EffectCandidateOperator,
		ObjectOID:          518,
		NamespaceOID:       11,
		OperandTypeOIDs:    []uint32{23, 23},
		ResultTypeOID:      16,
		ImplementationOID:  1852, // int4ne
		Volatility:         EffectVolatilityImmutable,
		CanonicalSignature: "pg_catalog.<>(23,23)",
		AuditNotes:         "int4 <> int4; impl int4ne; pure C builtin",
	},
	{
		Kind:               EffectCandidateOperator,
		ObjectOID:          97,
		NamespaceOID:       11,
		OperandTypeOIDs:    []uint32{23, 23},
		ResultTypeOID:      16,
		ImplementationOID:  66, // int4lt
		Volatility:         EffectVolatilityImmutable,
		CanonicalSignature: "pg_catalog.<(23,23)",
		AuditNotes:         "int4 < int4; impl int4lt; pure C builtin",
	},
	{
		Kind:               EffectCandidateOperator,
		ObjectOID:          520,
		NamespaceOID:       11,
		OperandTypeOIDs:    []uint32{23, 23},
		ResultTypeOID:      16,
		ImplementationOID:  67, // int4gt
		Volatility:         EffectVolatilityImmutable,
		CanonicalSignature: "pg_catalog.>(23,23)",
		AuditNotes:         "int4 > int4; impl int4gt; pure C builtin",
	},
	{
		Kind:               EffectCandidateOperator,
		ObjectOID:          521,
		NamespaceOID:       11,
		OperandTypeOIDs:    []uint32{23, 23},
		ResultTypeOID:      16,
		ImplementationOID:  1850, // int4le
		Volatility:         EffectVolatilityImmutable,
		CanonicalSignature: "pg_catalog.<=(23,23)",
		AuditNotes:         "int4 <= int4; impl int4le; pure C builtin",
	},
	{
		Kind:               EffectCandidateOperator,
		ObjectOID:          522,
		NamespaceOID:       11,
		OperandTypeOIDs:    []uint32{23, 23},
		ResultTypeOID:      16,
		ImplementationOID:  1851, // int4ge
		Volatility:         EffectVolatilityImmutable,
		CanonicalSignature: "pg_catalog.>=(23,23)",
		AuditNotes:         "int4 >= int4; impl int4ge; pure C builtin",
	},

	// =========================================================================
	// Comparison operators — int8 (OID 20) × 6
	// =========================================================================
	{
		Kind:               EffectCandidateOperator,
		ObjectOID:          410,
		NamespaceOID:       11,
		OperandTypeOIDs:    []uint32{20, 20},
		ResultTypeOID:      16,
		ImplementationOID:  467, // int8eq
		Volatility:         EffectVolatilityImmutable,
		CanonicalSignature: "pg_catalog.=(20,20)",
		AuditNotes:         "int8 = int8; impl int8eq; pure C builtin",
	},
	{
		Kind:               EffectCandidateOperator,
		ObjectOID:          414,
		NamespaceOID:       11,
		OperandTypeOIDs:    []uint32{20, 20},
		ResultTypeOID:      16,
		ImplementationOID:  469, // int8ne
		Volatility:         EffectVolatilityImmutable,
		CanonicalSignature: "pg_catalog.<>(20,20)",
		AuditNotes:         "int8 <> int8; impl int8ne; pure C builtin",
	},
	{
		Kind:               EffectCandidateOperator,
		ObjectOID:          412,
		NamespaceOID:       11,
		OperandTypeOIDs:    []uint32{20, 20},
		ResultTypeOID:      16,
		ImplementationOID:  466, // int8lt
		Volatility:         EffectVolatilityImmutable,
		CanonicalSignature: "pg_catalog.<(20,20)",
		AuditNotes:         "int8 < int8; impl int8lt; pure C builtin",
	},
	{
		Kind:               EffectCandidateOperator,
		ObjectOID:          413,
		NamespaceOID:       11,
		OperandTypeOIDs:    []uint32{20, 20},
		ResultTypeOID:      16,
		ImplementationOID:  468, // int8gt
		Volatility:         EffectVolatilityImmutable,
		CanonicalSignature: "pg_catalog.>(20,20)",
		AuditNotes:         "int8 > int8; impl int8gt; pure C builtin",
	},
	{
		Kind:               EffectCandidateOperator,
		ObjectOID:          842,
		NamespaceOID:       11,
		OperandTypeOIDs:    []uint32{20, 20},
		ResultTypeOID:      16,
		ImplementationOID:  1854, // int8le
		Volatility:         EffectVolatilityImmutable,
		CanonicalSignature: "pg_catalog.<=(20,20)",
		AuditNotes:         "int8 <= int8; impl int8le; pure C builtin",
	},
	{
		Kind:               EffectCandidateOperator,
		ObjectOID:          843,
		NamespaceOID:       11,
		OperandTypeOIDs:    []uint32{20, 20},
		ResultTypeOID:      16,
		ImplementationOID:  1855, // int8ge
		Volatility:         EffectVolatilityImmutable,
		CanonicalSignature: "pg_catalog.>=(20,20)",
		AuditNotes:         "int8 >= int8; impl int8ge; pure C builtin",
	},

	// =========================================================================
	// Comparison operators — float4 (OID 700) × 6
	// =========================================================================
	{
		Kind:               EffectCandidateOperator,
		ObjectOID:          622,
		NamespaceOID:       11,
		OperandTypeOIDs:    []uint32{700, 700},
		ResultTypeOID:      16,
		ImplementationOID:  631, // float4eq
		Volatility:         EffectVolatilityImmutable,
		CanonicalSignature: "pg_catalog.=(700,700)",
		AuditNotes:         "float4 = float4; impl float4eq; pure C builtin",
	},
	{
		Kind:               EffectCandidateOperator,
		ObjectOID:          624,
		NamespaceOID:       11,
		OperandTypeOIDs:    []uint32{700, 700},
		ResultTypeOID:      16,
		ImplementationOID:  633, // float4ne
		Volatility:         EffectVolatilityImmutable,
		CanonicalSignature: "pg_catalog.<>(700,700)",
		AuditNotes:         "float4 <> float4; impl float4ne; pure C builtin",
	},
	{
		Kind:               EffectCandidateOperator,
		ObjectOID:          620,
		NamespaceOID:       11,
		OperandTypeOIDs:    []uint32{700, 700},
		ResultTypeOID:      16,
		ImplementationOID:  630, // float4lt
		Volatility:         EffectVolatilityImmutable,
		CanonicalSignature: "pg_catalog.<(700,700)",
		AuditNotes:         "float4 < float4; impl float4lt; pure C builtin",
	},
	{
		Kind:               EffectCandidateOperator,
		ObjectOID:          621,
		NamespaceOID:       11,
		OperandTypeOIDs:    []uint32{700, 700},
		ResultTypeOID:      16,
		ImplementationOID:  632, // float4gt
		Volatility:         EffectVolatilityImmutable,
		CanonicalSignature: "pg_catalog.>(700,700)",
		AuditNotes:         "float4 > float4; impl float4gt; pure C builtin",
	},
	{
		Kind:               EffectCandidateOperator,
		ObjectOID:          623,
		NamespaceOID:       11,
		OperandTypeOIDs:    []uint32{700, 700},
		ResultTypeOID:      16,
		ImplementationOID:  634, // float4le
		Volatility:         EffectVolatilityImmutable,
		CanonicalSignature: "pg_catalog.<=(700,700)",
		AuditNotes:         "float4 <= float4; impl float4le; pure C builtin",
	},
	{
		Kind:               EffectCandidateOperator,
		ObjectOID:          625,
		NamespaceOID:       11,
		OperandTypeOIDs:    []uint32{700, 700},
		ResultTypeOID:      16,
		ImplementationOID:  635, // float4ge
		Volatility:         EffectVolatilityImmutable,
		CanonicalSignature: "pg_catalog.>=(700,700)",
		AuditNotes:         "float4 >= float4; impl float4ge; pure C builtin",
	},

	// =========================================================================
	// Comparison operators — float8 (OID 701) × 6
	// =========================================================================
	{
		Kind:               EffectCandidateOperator,
		ObjectOID:          672,
		NamespaceOID:       11,
		OperandTypeOIDs:    []uint32{701, 701},
		ResultTypeOID:      16,
		ImplementationOID:  670, // float8eq
		Volatility:         EffectVolatilityImmutable,
		CanonicalSignature: "pg_catalog.=(701,701)",
		AuditNotes:         "float8 = float8; impl float8eq; pure C builtin",
	},
	{
		Kind:               EffectCandidateOperator,
		ObjectOID:          673,
		NamespaceOID:       11,
		OperandTypeOIDs:    []uint32{701, 701},
		ResultTypeOID:      16,
		ImplementationOID:  671, // float8ne
		Volatility:         EffectVolatilityImmutable,
		CanonicalSignature: "pg_catalog.<>(701,701)",
		AuditNotes:         "float8 <> float8; impl float8ne; pure C builtin",
	},
	{
		Kind:               EffectCandidateOperator,
		ObjectOID:          674,
		NamespaceOID:       11,
		OperandTypeOIDs:    []uint32{701, 701},
		ResultTypeOID:      16,
		ImplementationOID:  668, // float8lt
		Volatility:         EffectVolatilityImmutable,
		CanonicalSignature: "pg_catalog.<(701,701)",
		AuditNotes:         "float8 < float8; impl float8lt; pure C builtin",
	},
	{
		Kind:               EffectCandidateOperator,
		ObjectOID:          675,
		NamespaceOID:       11,
		OperandTypeOIDs:    []uint32{701, 701},
		ResultTypeOID:      16,
		ImplementationOID:  669, // float8gt
		Volatility:         EffectVolatilityImmutable,
		CanonicalSignature: "pg_catalog.>(701,701)",
		AuditNotes:         "float8 > float8; impl float8gt; pure C builtin",
	},
	{
		Kind:               EffectCandidateOperator,
		ObjectOID:          676,
		NamespaceOID:       11,
		OperandTypeOIDs:    []uint32{701, 701},
		ResultTypeOID:      16,
		ImplementationOID:  672, // float8le
		Volatility:         EffectVolatilityImmutable,
		CanonicalSignature: "pg_catalog.<=(701,701)",
		AuditNotes:         "float8 <= float8; impl float8le; pure C builtin",
	},
	{
		Kind:               EffectCandidateOperator,
		ObjectOID:          677,
		NamespaceOID:       11,
		OperandTypeOIDs:    []uint32{701, 701},
		ResultTypeOID:      16,
		ImplementationOID:  673, // float8ge
		Volatility:         EffectVolatilityImmutable,
		CanonicalSignature: "pg_catalog.>=(701,701)",
		AuditNotes:         "float8 >= float8; impl float8ge; pure C builtin",
	},

	// =========================================================================
	// Comparison operators — numeric (OID 1700) × 6
	// =========================================================================
	{
		Kind:               EffectCandidateOperator,
		ObjectOID:          1752,
		NamespaceOID:       11,
		OperandTypeOIDs:    []uint32{1700, 1700},
		ResultTypeOID:      16,
		ImplementationOID:  1718, // numeric_eq
		Volatility:         EffectVolatilityImmutable,
		CanonicalSignature: "pg_catalog.=(1700,1700)",
		AuditNotes:         "numeric = numeric; impl numeric_eq; pure C builtin",
	},
	{
		Kind:               EffectCandidateOperator,
		ObjectOID:          1754,
		NamespaceOID:       11,
		OperandTypeOIDs:    []uint32{1700, 1700},
		ResultTypeOID:      16,
		ImplementationOID:  1720, // numeric_ne
		Volatility:         EffectVolatilityImmutable,
		CanonicalSignature: "pg_catalog.<>(1700,1700)",
		AuditNotes:         "numeric <> numeric; impl numeric_ne; pure C builtin",
	},
	{
		Kind:               EffectCandidateOperator,
		ObjectOID:          1756,
		NamespaceOID:       11,
		OperandTypeOIDs:    []uint32{1700, 1700},
		ResultTypeOID:      16,
		ImplementationOID:  1716, // numeric_lt
		Volatility:         EffectVolatilityImmutable,
		CanonicalSignature: "pg_catalog.<(1700,1700)",
		AuditNotes:         "numeric < numeric; impl numeric_lt; pure C builtin",
	},
	{
		Kind:               EffectCandidateOperator,
		ObjectOID:          1753,
		NamespaceOID:       11,
		OperandTypeOIDs:    []uint32{1700, 1700},
		ResultTypeOID:      16,
		ImplementationOID:  1719, // numeric_gt
		Volatility:         EffectVolatilityImmutable,
		CanonicalSignature: "pg_catalog.>(1700,1700)",
		AuditNotes:         "numeric > numeric; impl numeric_gt; pure C builtin",
	},
	{
		Kind:               EffectCandidateOperator,
		ObjectOID:          1755,
		NamespaceOID:       11,
		OperandTypeOIDs:    []uint32{1700, 1700},
		ResultTypeOID:      16,
		ImplementationOID:  1721, // numeric_le
		Volatility:         EffectVolatilityImmutable,
		CanonicalSignature: "pg_catalog.<=(1700,1700)",
		AuditNotes:         "numeric <= numeric; impl numeric_le; pure C builtin",
	},
	{
		Kind:               EffectCandidateOperator,
		ObjectOID:          1757,
		NamespaceOID:       11,
		OperandTypeOIDs:    []uint32{1700, 1700},
		ResultTypeOID:      16,
		ImplementationOID:  1722, // numeric_ge
		Volatility:         EffectVolatilityImmutable,
		CanonicalSignature: "pg_catalog.>=(1700,1700)",
		AuditNotes:         "numeric >= numeric; impl numeric_ge; pure C builtin",
	},

	// =========================================================================
	// Comparison operators — text (OID 25) × 6
	// =========================================================================
	{
		Kind:               EffectCandidateOperator,
		ObjectOID:          98,
		NamespaceOID:       11,
		OperandTypeOIDs:    []uint32{25, 25},
		ResultTypeOID:      16,
		ImplementationOID:  67, // texteq
		Volatility:         EffectVolatilityImmutable,
		CanonicalSignature: "pg_catalog.=(25,25)",
		AuditNotes:         "text = text; impl texteq; pure C builtin",
	},
	{
		Kind:               EffectCandidateOperator,
		ObjectOID:          531,
		NamespaceOID:       11,
		OperandTypeOIDs:    []uint32{25, 25},
		ResultTypeOID:      16,
		ImplementationOID:  86, // textne
		Volatility:         EffectVolatilityImmutable,
		CanonicalSignature: "pg_catalog.<>(25,25)",
		AuditNotes:         "text <> text; impl textne; pure C builtin",
	},
	{
		Kind:               EffectCandidateOperator,
		ObjectOID:          664,
		NamespaceOID:       11,
		OperandTypeOIDs:    []uint32{25, 25},
		ResultTypeOID:      16,
		ImplementationOID:  87, // text_lt
		Volatility:         EffectVolatilityImmutable,
		CanonicalSignature: "pg_catalog.<(25,25)",
		AuditNotes:         "text < text; impl text_lt; pure C builtin",
	},
	{
		Kind:               EffectCandidateOperator,
		ObjectOID:          665,
		NamespaceOID:       11,
		OperandTypeOIDs:    []uint32{25, 25},
		ResultTypeOID:      16,
		ImplementationOID:  88, // text_gt
		Volatility:         EffectVolatilityImmutable,
		CanonicalSignature: "pg_catalog.>(25,25)",
		AuditNotes:         "text > text; impl text_gt; pure C builtin",
	},
	{
		Kind:               EffectCandidateOperator,
		ObjectOID:          666,
		NamespaceOID:       11,
		OperandTypeOIDs:    []uint32{25, 25},
		ResultTypeOID:      16,
		ImplementationOID:  89, // text_le
		Volatility:         EffectVolatilityImmutable,
		CanonicalSignature: "pg_catalog.<=(25,25)",
		AuditNotes:         "text <= text; impl text_le; pure C builtin",
	},
	{
		Kind:               EffectCandidateOperator,
		ObjectOID:          667,
		NamespaceOID:       11,
		OperandTypeOIDs:    []uint32{25, 25},
		ResultTypeOID:      16,
		ImplementationOID:  90, // text_ge
		Volatility:         EffectVolatilityImmutable,
		CanonicalSignature: "pg_catalog.>=(25,25)",
		AuditNotes:         "text >= text; impl text_ge; pure C builtin",
	},

	// =========================================================================
	// Comparison operators — oid (OID 26) × 6
	// =========================================================================
	{
		Kind:               EffectCandidateOperator,
		ObjectOID:          608,
		NamespaceOID:       11,
		OperandTypeOIDs:    []uint32{26, 26},
		ResultTypeOID:      16,
		ImplementationOID:  1856, // oideq
		Volatility:         EffectVolatilityImmutable,
		CanonicalSignature: "pg_catalog.=(26,26)",
		AuditNotes:         "oid = oid; impl oideq; pure C builtin",
	},
	{
		Kind:               EffectCandidateOperator,
		ObjectOID:          609,
		NamespaceOID:       11,
		OperandTypeOIDs:    []uint32{26, 26},
		ResultTypeOID:      16,
		ImplementationOID:  1857, // oidne
		Volatility:         EffectVolatilityImmutable,
		CanonicalSignature: "pg_catalog.<>(26,26)",
		AuditNotes:         "oid <> oid; impl oidne; pure C builtin",
	},
	{
		Kind:               EffectCandidateOperator,
		ObjectOID:          604,
		NamespaceOID:       11,
		OperandTypeOIDs:    []uint32{26, 26},
		ResultTypeOID:      16,
		ImplementationOID:  1858, // oidlt
		Volatility:         EffectVolatilityImmutable,
		CanonicalSignature: "pg_catalog.<(26,26)",
		AuditNotes:         "oid < oid; impl oidlt; pure C builtin",
	},
	{
		Kind:               EffectCandidateOperator,
		ObjectOID:          605,
		NamespaceOID:       11,
		OperandTypeOIDs:    []uint32{26, 26},
		ResultTypeOID:      16,
		ImplementationOID:  1859, // oidgt
		Volatility:         EffectVolatilityImmutable,
		CanonicalSignature: "pg_catalog.>(26,26)",
		AuditNotes:         "oid > oid; impl oidgt; pure C builtin",
	},
	{
		Kind:               EffectCandidateOperator,
		ObjectOID:          606,
		NamespaceOID:       11,
		OperandTypeOIDs:    []uint32{26, 26},
		ResultTypeOID:      16,
		ImplementationOID:  1860, // oidle
		Volatility:         EffectVolatilityImmutable,
		CanonicalSignature: "pg_catalog.<=(26,26)",
		AuditNotes:         "oid <= oid; impl oidle; pure C builtin",
	},
	{
		Kind:               EffectCandidateOperator,
		ObjectOID:          607,
		NamespaceOID:       11,
		OperandTypeOIDs:    []uint32{26, 26},
		ResultTypeOID:      16,
		ImplementationOID:  1861, // oidge
		Volatility:         EffectVolatilityImmutable,
		CanonicalSignature: "pg_catalog.>=(26,26)",
		AuditNotes:         "oid >= oid; impl oidge; pure C builtin",
	},
}
