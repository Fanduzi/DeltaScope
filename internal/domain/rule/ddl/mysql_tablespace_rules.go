// Package ddl: T25 standalone MySQL tablespace-family rules.
//
// CREATE/ALTER/DROP TABLESPACE (and CREATE UNDO TABLESPACE) are adjudicated
// on the MySQL dialect: the DROP forbids at blocker level on the authored
// action — it destroys a server-side storage object — while CREATE/ALTER
// warn on the storage mutation. One applicability rule binds the native
// gates: an authored non-InnoDB/non-NDB engine refuses outright, undo
// tablespaces and ENCRYPTION refuse before MySQL 8.0, InnoDB refuses
// datafile moves (an NDB-only member), a proven-absent object refuses
// ALTER/DROP, a proven-present object refuses CREATE, and a proven-occupied
// tablespace refuses DROP — every unproven premise owes its fact through
// EvidenceGaps rather than a fabricated verdict. The NDB/logfile-group
// family and every TiDB-dialect form are engine/vendor boundaries marked
// by coverage before these rules ever run.
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
	missing := make([]string, 0, 4)
	if decl.Engine == "" && (decl.AlterAction == "add_datafile" || decl.AlterAction == "drop_datafile") {
		missing = append(missing, "tablespace.engine")
	}
	if (decl.Kind == spec.TablespaceKindUndo || decl.Encryption != nil) &&
		statementVersionMajor(statement) == nil {
		missing = append(missing, "target.version")
	}
	state := tablespaceStateFor(statement, decl.Name)
	if !state.Proven() {
		missing = append(missing, "tablespace.existence")
	}
	if statement.DDL.Operation == spec.DDLOperationDropTablespace &&
		(!state.Proven() || !state.Exists || state.Occupied == nil) {
		missing = append(missing, "tablespace.occupancy")
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
	// Authored engine boundary: only InnoDB and NDB are legal engines —
	// anything else is refused when the engine name resolves.
	if decl.Engine != "" && !strings.EqualFold(decl.Engine, "innodb") && !strings.EqualFold(decl.Engine, "ndb") {
		return refuse(tablespaceStandaloneReasonEngine,
			fmt.Sprintf("%s tablespace %q is not supported on the %s storage engine", statement.DDL.Operation, decl.Name, decl.Engine),
			"the tablespace family supports only InnoDB and NDB — verify the declared engine"), ctx.Err()
	}
	version := statementVersionMajor(statement)
	if version != nil && *version < 8 {
		// MySQL 5.7 has no UNDO TABLESPACE form and no tablespace
		// ENCRYPTION member — both are 8.0 surface.
		if decl.Kind == spec.TablespaceKindUndo {
			return refuse(tablespaceStandaloneReasonVersion,
				fmt.Sprintf("create undo tablespace %q requires MySQL 8.0 or later", decl.Name),
				"undo tablespaces do not exist before MySQL 8.0 — review the target version"), ctx.Err()
		}
		if decl.Encryption != nil {
			return refuse(tablespaceStandaloneReasonVersion,
				fmt.Sprintf("%s tablespace %q encryption requires MySQL 8.0 or later", statement.DDL.Operation, decl.Name),
				"the ENCRYPTION member does not exist before MySQL 8.0 — review the target version"), ctx.Err()
		}
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
	// Ordered-state premises: existence and occupancy.
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
		}
		if statement.DDL.Operation == spec.DDLOperationDropTablespace &&
			state.Exists && state.Occupied != nil && *state.Occupied {
			return refuse(tablespaceStandaloneReasonOccupied,
				fmt.Sprintf("drop tablespace %q refuses — tables are still bound to it", decl.Name),
				"move or drop the bound tables before dropping the tablespace"), ctx.Err()
		}
	}
	return nil, ctx.Err()
}

// tablespaceStateFor reads the bound ordered state from statement
// metadata; an unbound name reports unproven.
func tablespaceStateFor(statement spec.Statement, name string) *spec.TablespaceState {
	if statement.Metadata == nil || statement.Metadata.Tablespaces == nil {
		return nil
	}
	return statement.Metadata.Tablespaces[strings.ToLower(name)]
}

// statementVersionMajor reports the proven major version, or nil when the
// version fact never answered.
func statementVersionMajor(statement spec.Statement) *int {
	if statement.Metadata == nil || statement.Metadata.Version == nil ||
		!statement.Metadata.Version.ValidatedRange {
		return nil
	}
	v := statement.Metadata.Version.Major
	return &v
}
