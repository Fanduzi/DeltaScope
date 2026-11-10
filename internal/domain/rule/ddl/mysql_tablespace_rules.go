// Package ddl: T25 standalone MySQL tablespace-family rules.
//
// CREATE/ALTER/DROP TABLESPACE (and CREATE UNDO TABLESPACE) are adjudicated
// on the MySQL dialect: the DROP forbids at blocker level on the authored
// action — it destroys a server-side storage object — while CREATE/ALTER
// warn on the storage mutation. One applicability rule binds the native
// gates: an authored non-InnoDB/non-NDB engine refuses outright, and the
// documented capability floors refuse on a proven-below version — the
// UNDO SQL surface and the memberless InnoDB create arrive in 8.0.14,
// tablespace ENCRYPTION in 8.0.13, the 8.0 general-alter members (RENAME
// TO, AUTOEXTEND_SIZE) have no 5.7 counterpart, InnoDB refuses datafile
// moves (an NDB-only member), a proven-absent object refuses ALTER/DROP,
// a proven-present object refuses CREATE, a proven kind mismatch refuses
// the cross-family form, a proven-occupied general tablespace refuses
// DROP, and an undo DROP keeps its runtime-eligibility premise owed —
// every unproven premise owes its fact through EvidenceGaps rather than
// a fabricated verdict. The NDB/logfile-group family and every
// TiDB-dialect form are engine/vendor boundaries marked by coverage
// before these rules ever run.
package ddl

import (
	"context"
	"fmt"
	"strings"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

const (
	tablespaceStandaloneReasonEngine       = "tablespace_engine_inapplicable"
	tablespaceStandaloneReasonVersion      = "tablespace_version_inapplicable"
	tablespaceStandaloneReasonEngineAction = "tablespace_engine_action_inapplicable"
	tablespaceStandaloneReasonAbsent       = "tablespace_absent"
	tablespaceStandaloneReasonPresent      = "tablespace_present"
	tablespaceStandaloneReasonOccupied     = "tablespace_occupied"
	tablespaceStandaloneReasonKindMismatch = "tablespace_kind_mismatch"
)

func appliesToMySQLTablespace(statement spec.Statement, operation spec.DDLOperation) bool {
	return statement.Kind == spec.KindDDL && statement.Dialect == spec.DialectMySQL &&
		statement.DDL != nil && statement.DDL.Operation == operation &&
		statement.DDL.Tablespace != nil
}

func tablespaceMetadata(decl *spec.TablespaceDeclaration) map[string]any {
	metadata := map[string]any{"tablespace": decl.Name, "kind": decl.Kind}
	if decl.Engine != "" {
		metadata["engine"] = decl.Engine
	}
	if decl.AlterAction != "" {
		metadata["alter_action"] = decl.AlterAction
	}
	if decl.FileCount > 0 {
		metadata["file_count"] = decl.FileCount
	}
	return metadata
}

// tablespacePolicyRule emits one policy finding per authored operation —
// the CREATE/ALTER storage-mutation warnings and the DROP forbid.
type tablespacePolicyRule struct {
	ruleID     string
	operation  spec.DDLOperation
	label      string
	risk       string
	suggestion string
	level      rule.Level
	forbid     *bool // non-nil when the rule honours a forbid switch
}

func newTablespacePolicyRule(ruleID string, operation spec.DDLOperation, label, risk, suggestion string, fallback rule.Level, forbidParam bool, cfg policy.RulePolicy) (rule.StatementRule, error) {
	r := tablespacePolicyRule{
		ruleID: ruleID, operation: operation, label: label,
		risk: risk, suggestion: suggestion, level: configuredLevel(cfg, fallback),
	}
	if forbidParam {
		forbid, err := boolParam(ruleID, cfg, "forbid", true)
		if err != nil {
			return nil, err
		}
		r.forbid = &forbid
	}
	return r, nil
}

func newTablespaceDropForbidRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newTablespacePolicyRule(ruleIDMySQLTablespaceDropForbid,
		spec.DDLOperationDropTablespace, "drop tablespace",
		"destroys a server-side storage object and detaches its data file",
		"verify no tables remain bound and take a restorable copy of the data before dropping", rule.LevelBlocker, true, cfg)
}

func newTablespaceCreateWarnRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newTablespacePolicyRule(ruleIDMySQLTablespaceCreateWarn,
		spec.DDLOperationCreateTablespace, "create tablespace",
		"creates a new storage object with an on-disk data file on the server",
		"verify the declared engine, file placement, and capacity options before shipping", rule.LevelWarning, false, cfg)
}

func newTablespaceAlterWarnRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newTablespacePolicyRule(ruleIDMySQLTablespaceAlterWarn,
		spec.DDLOperationAlterTablespace, "alter tablespace",
		"mutates a server-side storage object's files or attributes",
		"verify the engine-allowed members and the maintenance window before shipping", rule.LevelWarning, false, cfg)
}

func (r tablespacePolicyRule) ID() string { return r.ruleID }

func (r tablespacePolicyRule) AppliesTo(statement spec.Statement) bool {
	if !appliesToMySQLTablespace(statement, r.operation) {
		return false
	}
	return r.forbid == nil || *r.forbid
}

func (r tablespacePolicyRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	decl := statement.DDL.Tablespace
	return []rule.Finding{{
		Level:      r.level,
		Message:    fmt.Sprintf("DDL %s %s", r.label, r.risk),
		Suggestion: r.suggestion,
		Metadata:   tablespaceMetadata(decl),
	}}, nil
}

// tablespaceApplicabilityRule binds the native premise gates to the
// proven ordered state — every refusal cites a proven premise, every
// unproven premise is owed through EvidenceGaps.
type tablespaceApplicabilityRule struct {
	level rule.Level
}

func newTablespaceApplicabilityRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return tablespaceApplicabilityRule{level: configuredLevel(cfg, rule.LevelBlocker)}, nil
}

func (r tablespaceApplicabilityRule) ID() string { return ruleIDMySQLTablespaceApplicability }

func (r tablespaceApplicabilityRule) AppliesTo(statement spec.Statement) bool {
	if statement.Kind != spec.KindDDL || statement.Dialect != spec.DialectMySQL ||
		statement.DDL == nil || statement.DDL.Tablespace == nil {
		return false
	}
	switch statement.DDL.Operation {
	case spec.DDLOperationCreateTablespace, spec.DDLOperationAlterTablespace,
		spec.DDLOperationDropTablespace:
		return true
	}
	return false
}

// missingTablespaceFacts reports the premises the rule consumed but the
// projection never answered.
func (r tablespaceApplicabilityRule) missingTablespaceFacts(statement spec.Statement) []string {
	decl := statement.DDL.Tablespace
	missing := make([]string, 0, 5)
	if decl.Engine == "" && (decl.AlterAction == "add_datafile" || decl.AlterAction == "drop_datafile" ||
		spec.TablespaceMemberlessCreate(statement.DDL.Operation, decl)) {
		missing = append(missing, "tablespace.engine")
	}
	if spec.TablespaceVersionOutcomeFor(statement.DDL.Operation, decl, tablespaceStatementVersion(statement)) ==
		spec.TablespaceVersionUnproven {
		missing = append(missing, "target.version")
	}
	state := tablespaceStateFor(statement, decl.Name)
	if !state.Proven() {
		missing = append(missing, "tablespace.existence")
	}
	if (statement.DDL.Operation == spec.DDLOperationAlterTablespace ||
		statement.DDL.Operation == spec.DDLOperationDropTablespace) &&
		state.Proven() && state.Exists && state.Kind == "" {
		// The catalog answered existence but not the object family — the
		// undo/general distinction decides the outcome and stays owed.
		missing = append(missing, "tablespace.kind")
	}
	if statement.DDL.Operation == spec.DDLOperationDropTablespace {
		switch decl.Kind {
		case spec.TablespaceKindUndo:
			// Dropping an undo tablespace needs the runtime STATE=empty
			// fact plus the not-a-default-object fact — neither is
			// modeled, so eligibility stays owed whenever the object is
			// not already proven absent.
			if !state.Proven() || state.Exists {
				missing = append(missing, "tablespace.drop_eligibility")
			}
		default:
			if !state.Proven() || !state.Exists || state.Occupied == nil {
				missing = append(missing, "tablespace.occupancy")
			}
		}
	}
	return missing
}

func (r tablespaceApplicabilityRule) EvidenceGaps(statement spec.Statement) []rule.EvidenceGap {
	if !r.AppliesTo(statement) {
		return nil
	}
	missing := r.missingTablespaceFacts(statement)
	if len(missing) == 0 {
		return nil
	}
	return []rule.EvidenceGap{{
		ReasonCode:    "unknown_tablespace_state",
		RequiredFacts: missing,
	}}
}

func (r tablespaceApplicabilityRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}
	decl := statement.DDL.Tablespace
	metadata := func(reason string) map[string]any {
		m := tablespaceMetadata(decl)
		m["reason_code"] = reason
		return m
	}
	refuse := func(reason, message, suggestion string) []rule.Finding {
		return []rule.Finding{{
			Level: r.level, Message: message, Suggestion: suggestion, Metadata: metadata(reason),
		}}
	}
	// Authored engine boundary: only InnoDB and the NDB family (NDB and
	// its NDBCLUSTER alias) are legal engines — anything else refuses.
	if decl.Engine != "" && !strings.EqualFold(decl.Engine, "innodb") && !spec.TablespaceEngineNDB(decl.Engine) {
		return refuse(tablespaceStandaloneReasonEngine,
			fmt.Sprintf("%s tablespace %q is not supported on the %s storage engine", statement.DDL.Operation, decl.Name, decl.Engine),
			"the tablespace family supports only InnoDB and NDB — verify the declared engine"), ctx.Err()
	}
	// Capability floors are exact-release: the UNDO SQL surface and the
	// memberless InnoDB create arrive in 8.0.14, tablespace ENCRYPTION in
	// 8.0.13, and the 8.0 general-tablespace members (RENAME TO,
	// AUTOEXTEND_SIZE, ENGINE_ATTRIBUTE) have no 5.7 counterpart. A
	// proven-below-floor version refuses; an unproven version owes
	// target.version through EvidenceGaps, never a refusal.
	floor, feature, gated := spec.TablespaceCapabilityFloor(statement.DDL.Operation, decl)
	if gated && spec.TablespaceVersionOutcomeFor(statement.DDL.Operation, decl, tablespaceStatementVersion(statement)) ==
		spec.TablespaceVersionRefused {
		return refuse(tablespaceStandaloneReasonVersion,
			fmt.Sprintf("%s tablespace %q requires MySQL %s or later — the %s surface does not exist", statement.DDL.Operation, decl.Name, floor, feature),
			"the declared member postdates the proven target version — review the target version"), ctx.Err()
	}
	// InnoDB refuses datafile moves on ALTER TABLESPACE — ADD/DROP
	// DATAFILE is an NDB member; InnoDB accepts only ENCRYPTION,
	// RENAME TO, and AUTOEXTEND_SIZE.
	if (decl.AlterAction == "add_datafile" || decl.AlterAction == "drop_datafile") &&
		strings.EqualFold(decl.Engine, "innodb") {
		return refuse(tablespaceStandaloneReasonEngineAction,
			fmt.Sprintf("alter tablespace %q %s is not supported on InnoDB", decl.Name, strings.ReplaceAll(decl.AlterAction, "_", " ")),
			"datafile moves are an NDB member — InnoDB alters only encryption, rename, and autoextend"), ctx.Err()
	}
	// Ordered-state premises: existence, kind, and occupancy.
	state := tablespaceStateFor(statement, decl.Name)
	if state.Proven() {
		switch statement.DDL.Operation {
		case spec.DDLOperationCreateTablespace:
			if state.Exists {
				return refuse(tablespaceStandaloneReasonPresent,
					fmt.Sprintf("create tablespace %q refuses — the object already exists", decl.Name),
					"verify the intended object identity or drop the existing tablespace first"), ctx.Err()
			}
		case spec.DDLOperationAlterTablespace, spec.DDLOperationDropTablespace:
			if !state.Exists {
				return refuse(tablespaceStandaloneReasonAbsent,
					fmt.Sprintf("%s tablespace %q refuses — the object does not exist", statement.DDL.Operation, decl.Name),
					"verify the object identity and the catalog state before shipping"), ctx.Err()
			}
			// The declared form addresses one object family — a proven
			// different catalog kind refuses (DROP TABLESPACE never
			// deletes an undo object; ALTER TABLESPACE never alters one,
			// and DROP UNDO TABLESPACE never deletes a general one).
			if state.Kind != "" && state.Kind != decl.Kind {
				return refuse(tablespaceStandaloneReasonKindMismatch,
					fmt.Sprintf("%s tablespace %q refuses — the object is a %s tablespace", statement.DDL.Operation, decl.Name, state.Kind),
					"verify the object family matches the statement form before shipping"), ctx.Err()
			}
		}
		if statement.DDL.Operation == spec.DDLOperationDropTablespace &&
			decl.Kind != spec.TablespaceKindUndo &&
			state.Exists && state.Occupied != nil && *state.Occupied {
			return refuse(tablespaceStandaloneReasonOccupied,
				fmt.Sprintf("drop tablespace %q refuses — tables are still bound to it", decl.Name),
				"move or drop the bound tables before dropping the tablespace"), ctx.Err()
		}
	}
	return nil, ctx.Err()
}

// tablespaceStateFor reads the bound ordered state from statement
// metadata; an unbound name reports unproven. The key keeps authored
// case — tablespace names are case-sensitive identifiers.
func tablespaceStateFor(statement spec.Statement, name string) *spec.TablespaceState {
	if statement.Metadata == nil || statement.Metadata.Tablespaces == nil {
		return nil
	}
	return statement.Metadata.Tablespaces[strings.TrimSpace(name)]
}

// statementVersionIdentity reports the proven version identity, or nil
// when the fact never answered — capability floors need the full
// MAJOR.MINOR.PATCH precision, not just the series.
func tablespaceStatementVersion(statement spec.Statement) *spec.VersionIdentity {
	if statement.Metadata == nil || statement.Metadata.Version == nil ||
		!statement.Metadata.Version.ValidatedRange {
		return nil
	}
	return statement.Metadata.Version
}
