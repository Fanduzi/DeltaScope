// online_ddl.go — bounded adjudication for ALTER TABLE ALGORITHM=/LOCK=
// modifiers and extracted table-option names. Every verdict cites the
// probed cell it stands on; cells outside the probed matrix answer
// Unproven, never a guess. The classifier is shared by rule evaluation,
// coverage aspects, and ordered state so the three layers never disagree
// about the same modifier combination (T15 / #94).
package spec

import "strings"

// OnlineDDLVerdict is the three-way adjudication of a modifier-bearing
// ALTER statement: proven combinable, proven refused, or outside the
// probed matrix.
type OnlineDDLVerdict int

const (
	// OnlineDDLCombinable means the modifier combination provably
	// executes on the resolved arm.
	OnlineDDLCombinable OnlineDDLVerdict = iota
	// OnlineDDLRefused means the combination provably refuses — the
	// statement fails atomically and the pre-state is the post-state.
	OnlineDDLRefused
	// OnlineDDLUnproven means the combination cannot be decided: a
	// required fact (version, engine) is missing or the cell sits
	// outside the probed matrix.
	OnlineDDLUnproven
)

// OnlineDDLOutcome is the statement-level answer for authored
// ALGORITHM=/LOCK= modifiers against the probed matrix.
type OnlineDDLOutcome struct {
	Verdict OnlineDDLVerdict
	// Refusal names the bounded native refusal class for refused
	// outcomes: "1800" unknown ALGORITHM, "1221" INSTANT+explicit LOCK,
	// "1845" algorithm unsupported for the operation, "1846" lock
	// unsupported for the algorithm.
	Refusal string
	// GapFacts lists the missing premise facts an unproven outcome
	// needs — "target.version", "target.version.validated_range",
	// "target_table.engine".
	GapFacts []string
}

// OnlineDDLModifierValues folds the authored algorithm/lock declarations
// out of an alter list with last-wins semantics (probed: duplicate
// declarations resolve to the last value on both MySQL and TiDB).
// Non-modifier actions are returned in authored order.
func OnlineDDLModifierValues(alters []Alter) (algorithm string, lock string, actions []string, tableOptionKeys map[string]struct{}) {
	tableOptionKeys = map[string]struct{}{}
	for i := range alters {
		switch alters[i].Action {
		case "algorithm":
			algorithm = alters[i].Options["algorithm"]
		case "lock":
			lock = alters[i].Options["lock"]
		default:
			actions = append(actions, alters[i].Action)
			if alters[i].Action == "table_option" {
				for key := range alters[i].Options {
					tableOptionKeys[key] = struct{}{}
				}
			}
		}
	}
	return algorithm, lock, actions, tableOptionKeys
}

// instantCapableFloors maps a MySQL action name to the documented
// 8.0 patch floor for ALGORITHM=INSTANT (8.0.12 ADD COLUMN,
// 8.0.3 RENAME COLUMN, 8.0.29 DROP COLUMN).
var instantCapableFloors = map[string]int{
	"add_columns":   12,
	"drop_column":   29,
	"rename_column": 3,
}

// instantRefusedActions is the probed 1845-class refusal set for
// ALGORITHM=INSTANT on MySQL 8.x.
var instantRefusedActions = map[string]struct{}{
	"add_index":      {},
	"add_constraint": {},
	"modify_column":  {},
	"change_column":  {},
}

// lockNoneCapableActions is the probed LOCK=NONE execution set on InnoDB
// (with ALGORITHM=INPLACE or server-chosen). Every other action under
// LOCK=NONE stays unproven.
var lockNoneCapableActions = map[string]struct{}{
	"add_columns": {},
	"add_index":   {},
}

// inplaceCapableActions is the probed ALGORITHM=INPLACE execution set
// (single and multi-action forms). table_option joins only through the
// engine-option path — see OnlineDDLOutcomeFor.
var inplaceCapableActions = map[string]struct{}{
	"add_columns":   {},
	"add_index":     {},
	"modify_column": {},
	"drop_column":   {},
}

// tidbInstantCapableActions is the probed ALGORITHM=INSTANT acceptance
// set on TiDB v8.5; add_index is a proven 1846 refusal instead.
var tidbInstantCapableActions = map[string]struct{}{
	"add_columns":   {},
	"drop_column":   {},
	"modify_column": {},
	"rename_column": {},
}

// OnlineDDLOutcomeFor adjudicates the statement's effective ALGORITHM/LOCK
// declarations against the probed matrix. alters must be the statement's
// full spec list (the function folds modifier values itself). engine is
// the observed pre-state engine, already lowercased-insensitive compared
// here.
func OnlineDDLOutcomeFor(dialect Dialect, version *VersionIdentity, engine string, alters []Alter) OnlineDDLOutcome {
	algorithm, lock, actions, tableOptionKeys := OnlineDDLModifierValues(alters)
	if !knownAlgorithmValue(algorithm) || !knownLockValue(lock) {
		return OnlineDDLOutcome{Verdict: OnlineDDLUnproven}
	}
	switch dialect {
	case DialectTiDB:
		return tidbOnlineDDLOutcome(algorithm, actions)
	case DialectMySQL:
		return mysqlOnlineDDLOutcome(version, engine, algorithm, lock, actions, tableOptionKeys)
	default:
		return OnlineDDLOutcome{Verdict: OnlineDDLUnproven}
	}
}

func knownAlgorithmValue(v string) bool {
	switch v {
	case "", "default", "copy", "inplace", "instant":
		return true
	default:
		return false
	}
}

func knownLockValue(v string) bool {
	switch v {
	case "", "default", "none", "shared", "exclusive":
		return true
	default:
		return false
	}
}

// tidbOnlineDDLOutcome is TiDB's own modifier semantics: every authored
// combination is accepted except the probed INSTANT-for-index refusal —
// TiDB decides executability per operation, never through the MySQL
// lock-vs-algorithm matrix. Unprobed INSTANT operations stay unproven.
func tidbOnlineDDLOutcome(algorithm string, actions []string) OnlineDDLOutcome {
	if algorithm != "instant" {
		return OnlineDDLOutcome{Verdict: OnlineDDLCombinable}
	}
	unproven := false
	for _, action := range actions {
		if action == "add_index" {
			return OnlineDDLOutcome{Verdict: OnlineDDLRefused, Refusal: "1846"}
		}
		if _, ok := tidbInstantCapableActions[action]; !ok {
			unproven = true
		}
	}
	if unproven {
		return OnlineDDLOutcome{Verdict: OnlineDDLUnproven}
	}
	return OnlineDDLOutcome{Verdict: OnlineDDLCombinable}
}

func mysqlOnlineDDLOutcome(version *VersionIdentity, engine, algorithm, lock string, actions []string, tableOptionKeys map[string]struct{}) OnlineDDLOutcome {
	switch algorithm {
	case "instant":
		// INSTANT gates on the version fact first — an unobserved or
		// unvalidated version leaves every cell undecidable.
		if version == nil || version.Product != VersionProductMySQL || version.Major == 0 {
			return OnlineDDLOutcome{Verdict: OnlineDDLUnproven, GapFacts: []string{"target.version"}}
		}
		if !version.ValidatedRange {
			return OnlineDDLOutcome{Verdict: OnlineDDLUnproven, GapFacts: []string{"target.version.validated_range"}}
		}
		// INSTANT permits only LOCK=DEFAULT/absent — any explicit lock
		// refuses 1221 on 8.x (probed). On 5.x the algorithm itself is
		// already unknown, and 1800 is the earlier refusal.
		if version.Major == 5 {
			return OnlineDDLOutcome{Verdict: OnlineDDLRefused, Refusal: "1800"}
		}
		if lock == "none" || lock == "shared" || lock == "exclusive" {
			return OnlineDDLOutcome{Verdict: OnlineDDLRefused, Refusal: "1221"}
		}
		unproven := false
		for _, action := range actions {
			if floor, ok := instantCapableFloors[action]; ok {
				if version.Minor == 0 && version.Patch < floor {
					return OnlineDDLOutcome{Verdict: OnlineDDLRefused, Refusal: "1845"}
				}
				continue
			}
			if _, refused := instantRefusedActions[action]; refused {
				return OnlineDDLOutcome{Verdict: OnlineDDLRefused, Refusal: "1845"}
			}
			unproven = true
		}
		if unproven {
			return OnlineDDLOutcome{Verdict: OnlineDDLUnproven}
		}
		return OnlineDDLOutcome{Verdict: OnlineDDLCombinable}
	case "copy":
		// COPY is the universal fallback — every action accepts it — but
		// it can never satisfy LOCK=NONE (probed 1846).
		if lock == "none" {
			return OnlineDDLOutcome{Verdict: OnlineDDLRefused, Refusal: "1846"}
		}
		return OnlineDDLOutcome{Verdict: OnlineDDLCombinable}
	case "inplace":
		if lock == "none" {
			return mysqlLockNoneOutcome(engine, actions)
		}
		for _, action := range actions {
			if _, ok := inplaceCapableActions[action]; ok {
				continue
			}
			// table_option under INPLACE is probed only for the engine
			// declaration; other option kinds stay unproven.
			if action == "table_option" {
				if onlyEngineOptions(tableOptionKeys) {
					continue
				}
				return OnlineDDLOutcome{Verdict: OnlineDDLUnproven}
			}
			return OnlineDDLOutcome{Verdict: OnlineDDLUnproven}
		}
		return OnlineDDLOutcome{Verdict: OnlineDDLCombinable}
	case "", "default":
		if lock == "none" {
			return mysqlLockNoneOutcome(engine, actions)
		}
		// Absent/DEFAULT algorithm with a satisfiable lock request is
		// accepted for every action — a stronger lock request is always
		// satisfiable.
		return OnlineDDLOutcome{Verdict: OnlineDDLCombinable}
	default:
		return OnlineDDLOutcome{Verdict: OnlineDDLUnproven}
	}
}

// mysqlLockNoneOutcome adjudicates a LOCK=NONE request (with no explicit
// algorithm or INPLACE) — the only lock value whose satisfaction depends
// on the engine and the operation set. Probed on InnoDB {add_columns,
// add_index}; every other cell is unproven, and a missing engine fact is
// the gap the caller owes.
func mysqlLockNoneOutcome(engine string, actions []string) OnlineDDLOutcome {
	if !strings.EqualFold(strings.TrimSpace(engine), "innodb") {
		if strings.TrimSpace(engine) == "" {
			return OnlineDDLOutcome{Verdict: OnlineDDLUnproven, GapFacts: []string{"target_table.engine"}}
		}
		return OnlineDDLOutcome{Verdict: OnlineDDLUnproven}
	}
	for _, action := range actions {
		if _, ok := lockNoneCapableActions[action]; !ok {
			return OnlineDDLOutcome{Verdict: OnlineDDLUnproven}
		}
	}
	return OnlineDDLOutcome{Verdict: OnlineDDLCombinable}
}

func onlyEngineOptions(keys map[string]struct{}) bool {
	if len(keys) == 0 {
		return false
	}
	for key := range keys {
		if key != "engine" {
			return false
		}
	}
	return true
}

// TiDBCharsetConversionOutcome adjudicates a TiDB CONVERT TO CHARACTER
// SET target: utf8mb4 is the probed accepted family, latin1 and utf8 the
// probed 8200 refusals; every other pair stays unproven. Rules, coverage,
// and ordered state share this one arm.
func TiDBCharsetConversionOutcome(charset string) OnlineDDLVerdict {
	switch strings.ToLower(strings.TrimSpace(charset)) {
	case "utf8mb4":
		return OnlineDDLCombinable
	case "latin1", "utf8":
		return OnlineDDLRefused
	default:
		return OnlineDDLUnproven
	}
}

// TableOptionOutcome is the bounded per-option adjudication for an
// extracted table-option name on ALTER — consumed by audited semantics,
// a proven vendor refusal, or extracted-but-unaudited.
type TableOptionOutcome int

const (
	// TableOptionConsumed means the option name has audited semantics on
	// this dialect's ALTER (catalog comparison, conversion, or a
	// rule-owned declaration path).
	TableOptionConsumed TableOptionOutcome = iota
	// TableOptionVendorRefused means the option provably refuses on this
	// dialect's ALTER (TiDB 8200/8233/8232, probed).
	TableOptionVendorRefused
	// TableOptionUnaudited means the name is extracted but carries no
	// audited semantics on this dialect's ALTER — a bounded aspect gap.
	TableOptionUnaudited
)

// mysqlAlterConsumedOptions are extracted ALTER option names consumed by
// rule/state semantics on MySQL: catalog-compared declarations, charset
// conversion, and the warn/applicability-owned declarations.
var mysqlAlterConsumedOptions = map[string]struct{}{
	"engine":             {},
	"charset":            {},
	"collate":            {},
	"row_format":         {},
	"auto_increment":     {},
	"comment":            {},
	"convert_to_charset": {},
	"convert_to_collate": {},
	"key_block_size":     {},
	"compression":        {},
	"encryption":         {},
	"tablespace":         {},
	"insert_method":      {},
}

// tidbAlterConsumedOptions are extracted ALTER option names TiDB accepts
// (probed rc=0). Accepted catalog options publish their mapped effect;
// the rest are compat declarations TiDB does not apply.
var tidbAlterConsumedOptions = map[string]struct{}{
	"engine":             {},
	"charset":            {},
	"collate":            {},
	"row_format":         {},
	"auto_increment":     {},
	"comment":            {},
	"convert_to_charset": {},
	"convert_to_collate": {},
}

// tidbAlterRefusedOptions is the probed TiDB ALTER refusal set — every
// name refused 8200 (8233 insert_method, 8232 union) on the v8.5.0 anchor.
var tidbAlterRefusedOptions = map[string]struct{}{
	"key_block_size":     {},
	"compression":        {},
	"encryption":         {},
	"tablespace":         {},
	"insert_method":      {},
	"stats_persistent":   {},
	"stats_auto_recalc":  {},
	"stats_sample_pages": {},
	"pack_keys":          {},
	"delay_key_write":    {},
	"avg_row_length":     {},
	"checksum":           {},
	"table_checksum":     {},
	"max_rows":           {},
	"min_rows":           {},
	"data_directory":     {},
	"index_directory":    {},
	"connection":         {},
	"password":           {},
	"storage_media":      {},
	"secondary_engine":   {},
	"union":              {},
}

// AlterTableOptionOutcome classifies one extracted option name under an
// ALTER on the dialect. Names outside every probed set stay unaudited.
func AlterTableOptionOutcome(dialect Dialect, name string) TableOptionOutcome {
	switch dialect {
	case DialectTiDB:
		if _, ok := tidbAlterConsumedOptions[name]; ok {
			return TableOptionConsumed
		}
		if _, refused := tidbAlterRefusedOptions[name]; refused {
			return TableOptionVendorRefused
		}
		return TableOptionUnaudited
	case DialectMySQL:
		if _, ok := mysqlAlterConsumedOptions[name]; ok {
			return TableOptionConsumed
		}
		return TableOptionUnaudited
	default:
		return TableOptionUnaudited
	}
}
