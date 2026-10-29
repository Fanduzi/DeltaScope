// alter_online_ddl_rules.go — T15 (#94): online-DDL modifier rules,
// maintenance-action rules, charset-conversion rules, and the
// declaration-level table-option rules. Every rule reads the shared
// spec.OnlineDDLOutcomeFor / spec.AlterTableOptionOutcome adjudicators so
// findings, evidence gaps, coverage aspects, and ordered state never
// disagree about the same authored declaration.
package ddl

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

const gapReasonIncompleteTableFact = "incomplete_table_fact"

// ---- shared helpers ----------------------------------------------------

// hasOnlineModifier reports whether the statement authors an ALGORITHM or
// LOCK clause.
func hasOnlineModifier(ddl *spec.DDL) bool {
	for i := range ddl.Alter {
		if ddl.Alter[i].Action == "algorithm" || ddl.Alter[i].Action == "lock" {
			return true
		}
	}
	return false
}

// hasPartitionAlterSpec reports whether the statement carries a
// partition-scoped action — modifier combinations there are adjudicated
// by the T14 partition classifier, not the online-DDL matrix.
func hasPartitionAlterSpec(ddl *spec.DDL) bool {
	for i := range ddl.Alter {
		if spec.IsPartitionAlterAction(ddl.Alter[i].Action) {
			return true
		}
	}
	return false
}

// statementOnlineOutcome resolves the classifier inputs from the
// statement's own metadata: the observed target version and the observed
// pre-state engine. Callers gate on AppliesTo first, so the statement is
// always an online-DDL candidate here (modifier present, no partition
// action).
func statementOnlineOutcome(statement spec.Statement) spec.OnlineDDLOutcome {
	ddl := statement.DDL
	if ddl == nil || !hasOnlineModifier(ddl) || hasPartitionAlterSpec(ddl) {
		return spec.OnlineDDLOutcome{}
	}
	var version *spec.VersionIdentity
	var engine string
	if statement.Metadata != nil {
		version = statement.Metadata.Version
		if statement.Metadata.TargetTable != nil {
			engine = statement.Metadata.TargetTable.Options["engine"]
		}
	}
	return spec.OnlineDDLOutcomeFor(statement.Dialect, version, engine, ddl.Alter)
}

// authoredModifierValues folds the authored algorithm/lock declarations
// with last-wins semantics for finding metadata.
func authoredModifierValues(ddl *spec.DDL) (algorithm, lock string) {
	algorithm, lock, _, _ = spec.OnlineDDLModifierValues(ddl.Alter)
	return algorithm, lock
}

// structuralActionList returns the non-modifier actions in authored
// order for finding metadata.
func structuralActionList(ddl *spec.DDL) []string {
	_, _, actions, _ := spec.OnlineDDLModifierValues(ddl.Alter)
	return actions
}

// tableOptionAlterSpecs returns the alter specs that declare table
// options.
func tableOptionAlterSpecs(statement spec.Statement) []spec.Alter {
	var specs []spec.Alter
	if statement.DDL == nil {
		return nil
	}
	for _, alter := range statement.DDL.Alter {
		if alter.Action == "table_option" {
			specs = append(specs, alter)
		}
	}
	return specs
}

// statementHasAction reports whether any alter spec carries the action.
func statementHasAction(statement spec.Statement, actions ...string) bool {
	if statement.DDL == nil {
		return false
	}
	for _, alter := range statement.DDL.Alter {
		for _, action := range actions {
			if alter.Action == action {
				return true
			}
		}
	}
	return false
}

// onlineGapEvidence folds classifier GapFacts into rule evidence gaps —
// one gap per bounded reason code.
func onlineGapEvidence(ruleID string, outcome spec.OnlineDDLOutcome) []rule.EvidenceGap {
	if outcome.Verdict != spec.OnlineDDLUnproven || len(outcome.GapFacts) == 0 {
		return nil
	}
	var gaps []rule.EvidenceGap
	for _, fact := range outcome.GapFacts {
		switch fact {
		case "target.version":
			gaps = append(gaps, rule.EvidenceGap{
				RuleID:        ruleID,
				ReasonCode:    gapReasonMissingTargetVersion,
				RequiredFacts: []string{"target.version"},
			})
		case "target.version.validated_range":
			gaps = append(gaps, rule.EvidenceGap{
				RuleID:        ruleID,
				ReasonCode:    gapReasonTargetVersionOutOfRange,
				RequiredFacts: []string{"target.version"},
			})
		default:
			gaps = append(gaps, rule.EvidenceGap{
				RuleID:        ruleID,
				ReasonCode:    gapReasonIncompleteTableFact,
				RequiredFacts: []string{fact},
			})
		}
	}
	return gaps
}

// ---- ddl.alter.online_ddl.applicability --------------------------------

// onlineDDLApplicabilityRule is a blocker for modifier combinations the
// probed matrix proves refused — on either dialect.
type onlineDDLApplicabilityRule struct {
	level rule.Level
}

func newOnlineDDLApplicabilityRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return onlineDDLApplicabilityRule{level: configuredLevel(cfg, rule.LevelBlocker)}, nil
}

func (r onlineDDLApplicabilityRule) ID() string { return ruleIDAlterOnlineDDLApplicability }

func (r onlineDDLApplicabilityRule) AppliesTo(statement spec.Statement) bool {
	if statement.DDL == nil || statement.DDL.Operation != spec.DDLOperationAlterTable {
		return false
	}
	if statement.Dialect != spec.DialectMySQL && statement.Dialect != spec.DialectTiDB {
		return false
	}
	return hasOnlineModifier(statement.DDL) && !hasPartitionAlterSpec(statement.DDL)
}

func (r onlineDDLApplicabilityRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	outcome := statementOnlineOutcome(statement)
	if outcome.Verdict != spec.OnlineDDLRefused {
		return nil, nil
	}
	algorithm, lock := authoredModifierValues(statement.DDL)
	return []rule.Finding{{
		RuleID:     r.ID(),
		Level:      r.level,
		Message:    fmt.Sprintf("alter table modifier combination is refused (algorithm=%q lock=%q)", algorithm, lock),
		Suggestion: "drop the modifier clause or choose an algorithm/lock pair the target operation supports",
		Metadata: map[string]any{
			"table":     statement.DDL.Table.Name,
			"algorithm": algorithm,
			"lock":      lock,
			"actions":   structuralActionList(statement.DDL),
			"refusal":   outcome.Refusal,
		},
	}}, nil
}

func (r onlineDDLApplicabilityRule) EvidenceGaps(statement spec.Statement) []rule.EvidenceGap {
	if !r.AppliesTo(statement) {
		return nil
	}
	outcome := statementOnlineOutcome(statement)
	return onlineGapEvidence(r.ID(), outcome)
}

// ---- ddl.alter.online_ddl.warn -----------------------------------------

// onlineDDLWarnRule warns on combinable-but-risky modifier declarations:
// COPY rebuilds through a copy of the table with writes blocked,
// LOCK=EXCLUSIVE denies concurrent access, LOCK=SHARED denies concurrent
// writes, and INPLACE is not an online guarantee.
type onlineDDLWarnRule struct {
	level rule.Level
}

func newOnlineDDLWarnRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return onlineDDLWarnRule{level: configuredLevel(cfg, rule.LevelWarning)}, nil
}

func (r onlineDDLWarnRule) ID() string { return ruleIDAlterOnlineDDLWarn }

func (r onlineDDLWarnRule) AppliesTo(statement spec.Statement) bool {
	if statement.DDL == nil || statement.DDL.Operation != spec.DDLOperationAlterTable {
		return false
	}
	if statement.Dialect != spec.DialectMySQL {
		return false
	}
	return hasOnlineModifier(statement.DDL) && !hasPartitionAlterSpec(statement.DDL)
}

func (r onlineDDLWarnRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	outcome := statementOnlineOutcome(statement)
	if outcome.Verdict == spec.OnlineDDLRefused {
		return nil, nil
	}
	algorithm, lock := authoredModifierValues(statement.DDL)
	var risks []string
	switch algorithm {
	case "copy":
		risks = append(risks, "algorithm=copy rebuilds through a full table copy with writes blocked")
	case "inplace":
		risks = append(risks, "algorithm=inplace is not an online guarantee")
	}
	switch lock {
	case "exclusive":
		risks = append(risks, "lock=exclusive denies concurrent reads and writes")
	case "shared":
		risks = append(risks, "lock=shared denies concurrent writes")
	}
	if len(risks) == 0 {
		return nil, nil
	}
	return []rule.Finding{{
		RuleID:     r.ID(),
		Level:      r.level,
		Message:    fmt.Sprintf("alter table declares %s", strings.Join(risks, "; ")),
		Suggestion: "prefer the server-default algorithm/lock or a reviewed instant/inplace combination",
		Metadata: map[string]any{
			"table":     statement.DDL.Table.Name,
			"algorithm": algorithm,
			"lock":      lock,
			"actions":   structuralActionList(statement.DDL),
		},
	}}, nil
}

// ---- ddl.tidb.alter.compat_options.notice ------------------------------

// tidbAlterCompatOptionsNoticeRule notices TiDB-only compatibility
// declarations: authored ALGORITHM/LOCK clauses, ORDER BY, and
// ENABLE/DISABLE KEYS are accepted parse-level syntax that TiDB does not
// apply the MySQL way. The notice only fires when the statement's own
// classifier adjudicated the combination combinable — a refused or
// unproven cell is owned elsewhere.
type tidbAlterCompatOptionsNoticeRule struct {
	level rule.Level
}

func newTiDBAlterCompatOptionsNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return tidbAlterCompatOptionsNoticeRule{level: configuredLevel(cfg, rule.LevelNotice)}, nil
}

func (r tidbAlterCompatOptionsNoticeRule) ID() string { return ruleIDTiDBAlterCompatOptionsNotice }

func (r tidbAlterCompatOptionsNoticeRule) AppliesTo(statement spec.Statement) bool {
	if statement.DDL == nil || statement.DDL.Operation != spec.DDLOperationAlterTable {
		return false
	}
	if statement.Dialect != spec.DialectTiDB {
		return false
	}
	return statementHasAction(statement, "algorithm", "lock", "order_by_columns", "enable_keys", "disable_keys") &&
		!hasPartitionAlterSpec(statement.DDL)
}

func (r tidbAlterCompatOptionsNoticeRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	// A refused or unproven modifier combination owns the statement
	// elsewhere — a compat-acceptance notice would fabricate acceptance.
	if hasOnlineModifier(statement.DDL) {
		outcome := statementOnlineOutcome(statement)
		if outcome.Verdict != spec.OnlineDDLCombinable {
			return nil, nil
		}
	}
	// One finding per statement: the authored compat declarations fold
	// into a single notice — each spec is a clause of the same accepted
	// syntax, not its own statement result.
	declarations := make([]string, 0, 2)
	for _, alter := range statement.DDL.Alter {
		switch alter.Action {
		case "algorithm", "lock":
			declarations = append(declarations, fmt.Sprintf("%s=%q", alter.Action, alter.Options[alter.Action]))
		case "order_by_columns":
			declarations = append(declarations, "order by")
		case "enable_keys":
			declarations = append(declarations, "enable keys")
		case "disable_keys":
			declarations = append(declarations, "disable keys")
		}
	}
	if len(declarations) == 0 {
		return nil, nil
	}
	return []rule.Finding{{
		RuleID:     r.ID(),
		Level:      r.level,
		Message:    fmt.Sprintf("alter table declares TiDB compatibility clauses: %s", strings.Join(declarations, ", ")),
		Suggestion: "the clauses are accepted for syntax compatibility; TiDB applies its own online semantics",
		Metadata: map[string]any{
			"table":        statement.DDL.Table.Name,
			"declarations": declarations,
		},
	}}, nil
}

// ---- ddl.alter.rebuild.warn --------------------------------------------

// alterRebuildWarnRule warns on MySQL reordering/rebuild declarations:
// ORDER BY physically reorders rows without adding an access path and
// FORCE rebuilds the table wholesale.
type alterRebuildWarnRule struct {
	level rule.Level
}

func newAlterRebuildWarnRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return alterRebuildWarnRule{level: configuredLevel(cfg, rule.LevelWarning)}, nil
}

func (r alterRebuildWarnRule) ID() string { return ruleIDAlterRebuildWarn }

func (r alterRebuildWarnRule) AppliesTo(statement spec.Statement) bool {
	if statement.DDL == nil || statement.DDL.Operation != spec.DDLOperationAlterTable {
		return false
	}
	return statement.Dialect == spec.DialectMySQL &&
		statementHasAction(statement, "order_by_columns", "force")
}

func (r alterRebuildWarnRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}
	findings := make([]rule.Finding, 0, 1)
	for _, alter := range statement.DDL.Alter {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		var message string
		var metadata map[string]any
		switch alter.Action {
		case "order_by_columns":
			columns := make([]string, 0, len(alter.OrderBy))
			for _, item := range alter.OrderBy {
				name := item.Column
				if item.Desc {
					name += " desc"
				}
				columns = append(columns, name)
			}
			message = fmt.Sprintf("alter table order by %s reorders rows physically without adding an access path", strings.Join(columns, ", "))
			metadata = map[string]any{"table": statement.DDL.Table.Name, "action": alter.Action, "columns": columns}
		case "force":
			message = "alter table force rebuilds the whole table"
			metadata = map[string]any{"table": statement.DDL.Table.Name, "action": alter.Action}
		default:
			continue
		}
		findings = append(findings, rule.Finding{
			RuleID:     r.ID(),
			Level:      r.level,
			Message:    message,
			Suggestion: "confirm the rebuild is reviewed and scheduled — it is not an online operation",
			Metadata:   metadata,
		})
	}
	return findings, nil
}

// ---- ddl.alter.keys_toggle.notice --------------------------------------

// alterKeysToggleNoticeRule notices MyISAM's real ENABLE/DISABLE KEYS
// maintenance; other engines accept the clause but it is inert.
type alterKeysToggleNoticeRule struct {
	level rule.Level
}

func newAlterKeysToggleNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return alterKeysToggleNoticeRule{level: configuredLevel(cfg, rule.LevelNotice)}, nil
}

func (r alterKeysToggleNoticeRule) ID() string { return ruleIDAlterKeysToggleNotice }

func (r alterKeysToggleNoticeRule) AppliesTo(statement spec.Statement) bool {
	if statement.DDL == nil || statement.DDL.Operation != spec.DDLOperationAlterTable {
		return false
	}
	return statement.Dialect == spec.DialectMySQL &&
		statementHasAction(statement, "enable_keys", "disable_keys")
}

func (r alterKeysToggleNoticeRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	snapshot, hasSnapshot := targetTableSnapshot(statement)
	if hasSnapshot && snapshot != nil && !snapshot.Exists {
		return nil, nil
	}
	engine := ""
	if hasSnapshot && snapshot != nil {
		engine = snapshot.Options["engine"]
	}
	findings := make([]rule.Finding, 0, 1)
	for _, alter := range statement.DDL.Alter {
		if alter.Action != "enable_keys" && alter.Action != "disable_keys" {
			continue
		}
		var message string
		switch {
		case strings.EqualFold(engine, "myisam"):
			message = fmt.Sprintf("alter table %s performs real MyISAM key maintenance — indexes rebuild while the table is locked", alter.Action)
		case strings.TrimSpace(engine) == "":
			message = fmt.Sprintf("alter table %s is only effective on MyISAM; the target engine is unobserved", alter.Action)
		default:
			message = fmt.Sprintf("alter table %s is accepted but inert on %s — the clause only affects MyISAM", alter.Action, engine)
		}
		findings = append(findings, rule.Finding{
			RuleID:     r.ID(),
			Level:      r.level,
			Message:    message,
			Suggestion: "the clause is a no-op outside MyISAM — drop it or confirm the target engine",
			Metadata: map[string]any{
				"table":  statement.DDL.Table.Name,
				"action": alter.Action,
				"engine": engine,
			},
		})
	}
	return findings, nil
}

func (r alterKeysToggleNoticeRule) EvidenceGaps(statement spec.Statement) []rule.EvidenceGap {
	if !r.AppliesTo(statement) {
		return nil
	}
	snapshot, ok := targetTableSnapshot(statement)
	if !ok || snapshot == nil || !snapshot.Exists {
		return nil
	}
	if strings.TrimSpace(snapshot.Options["engine"]) != "" {
		return nil
	}
	return []rule.EvidenceGap{{
		RuleID:        r.ID(),
		ReasonCode:    gapReasonIncompleteTableFact,
		RequiredFacts: []string{"target_table.engine"},
	}}
}

// ---- ddl.alter.charset_conversion.warn ---------------------------------

// charsetConversionTargets folds every convert_to_* declaration out of
// the statement's table-option specs.
func charsetConversionTargets(statement spec.Statement) (charset, collate string, declared bool) {
	for _, alter := range tableOptionAlterSpecs(statement) {
		if value, ok := alter.Options["convert_to_charset"]; ok {
			charset, declared = value, true
		}
		if value, ok := alter.Options["convert_to_collate"]; ok {
			collate = value
		}
	}
	return charset, collate, declared
}

// stringColumnNames returns the names of snapshot columns whose type
// family carries a charset — the columns a CONVERT TO actually rewrites.
func stringColumnNames(snapshot *spec.TableSnapshot) []string {
	if snapshot == nil {
		return nil
	}
	names := make([]string, 0, len(snapshot.Columns))
	for _, column := range snapshot.Columns {
		if columnTypeFamily(column) == "string" {
			names = append(names, column.Name)
		}
	}
	return names
}

type alterCharsetConversionWarnRule struct {
	level rule.Level
}

func newAlterCharsetConversionWarnRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return alterCharsetConversionWarnRule{level: configuredLevel(cfg, rule.LevelWarning)}, nil
}

func (r alterCharsetConversionWarnRule) ID() string { return ruleIDAlterCharsetConversionWarn }

func (r alterCharsetConversionWarnRule) AppliesTo(statement spec.Statement) bool {
	if statement.DDL == nil || statement.DDL.Operation != spec.DDLOperationAlterTable {
		return false
	}
	if statement.Dialect != spec.DialectMySQL && statement.Dialect != spec.DialectTiDB {
		return false
	}
	_, _, declared := charsetConversionTargets(statement)
	return declared
}

func (r alterCharsetConversionWarnRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	charset, collate, _ := charsetConversionTargets(statement)
	if statement.Dialect == spec.DialectTiDB && spec.TiDBCharsetConversionOutcome(charset) != spec.OnlineDDLCombinable {
		return nil, nil
	}
	snapshot, hasSnapshot := targetTableSnapshot(statement)
	columns := []string(nil)
	if hasSnapshot && snapshot != nil && snapshot.Exists {
		columns = stringColumnNames(snapshot)
	}
	metadata := map[string]any{
		"table":              statement.DDL.Table.Name,
		"action":             "table_option",
		"convert_to_charset": charset,
	}
	if collate != "" {
		metadata["convert_to_collate"] = collate
	}
	if columns != nil {
		metadata["string_columns"] = columns
		metadata["string_column_count"] = len(columns)
	}
	return []rule.Finding{{
		RuleID:     r.ID(),
		Level:      r.level,
		Message:    fmt.Sprintf("alter table convert to character set %s rewrites every string column", charset),
		Suggestion: "the conversion rebuilds all string data — review column-level charsets and the rebuild window explicitly",
		Metadata:   metadata,
	}}, nil
}

// ---- ddl.alter.charset_conversion.applicability -------------------------

// alterCharsetConversionApplicabilityRule blocks proven TiDB
// charset-conversion refusals — targets outside the utf8mb4 family.
type alterCharsetConversionApplicabilityRule struct {
	level rule.Level
}

func newAlterCharsetConversionApplicabilityRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return alterCharsetConversionApplicabilityRule{level: configuredLevel(cfg, rule.LevelBlocker)}, nil
}

func (r alterCharsetConversionApplicabilityRule) ID() string {
	return ruleIDAlterCharsetConversionApplicability
}

func (r alterCharsetConversionApplicabilityRule) AppliesTo(statement spec.Statement) bool {
	if statement.DDL == nil || statement.DDL.Operation != spec.DDLOperationAlterTable {
		return false
	}
	if statement.Dialect != spec.DialectTiDB {
		return false
	}
	_, _, declared := charsetConversionTargets(statement)
	return declared
}

func (r alterCharsetConversionApplicabilityRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	charset, collate, _ := charsetConversionTargets(statement)
	if spec.TiDBCharsetConversionOutcome(charset) != spec.OnlineDDLRefused {
		return nil, nil
	}
	return []rule.Finding{{
		RuleID:     r.ID(),
		Level:      r.level,
		Message:    fmt.Sprintf("alter table convert to character set %s is refused on TiDB (probed 8200)", charset),
		Suggestion: "TiDB only accepts the utf8mb4 family for table charset conversion",
		Metadata: map[string]any{
			"table":              statement.DDL.Table.Name,
			"action":             "table_option",
			"convert_to_charset": charset,
			"convert_to_collate": collate,
			"refusal":            "8200",
		},
	}}, nil
}

func (r alterCharsetConversionApplicabilityRule) EvidenceGaps(statement spec.Statement) []rule.EvidenceGap {
	if !r.AppliesTo(statement) {
		return nil
	}
	charset, _, _ := charsetConversionTargets(statement)
	if spec.TiDBCharsetConversionOutcome(charset) != spec.OnlineDDLUnproven {
		return nil
	}
	return []rule.EvidenceGap{{
		RuleID:        r.ID(),
		ReasonCode:    gapReasonUnprovenStatementFact,
		RequiredFacts: []string{"statement.convert_to_charset"},
	}}
}

// ---- ddl.alter.table_option.applicability -------------------------------

// alterTableOptionApplicabilityRule blocks the probed MySQL refusal:
// KEY_BLOCK_SIZE on a non-COMPRESSED InnoDB table (1478).
type alterTableOptionApplicabilityRule struct {
	level rule.Level
}

func newAlterTableOptionApplicabilityRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return alterTableOptionApplicabilityRule{level: configuredLevel(cfg, rule.LevelBlocker)}, nil
}

func (r alterTableOptionApplicabilityRule) ID() string {
	return ruleIDAlterTableOptionApplicability
}

func (r alterTableOptionApplicabilityRule) AppliesTo(statement spec.Statement) bool {
	if statement.DDL == nil || statement.DDL.Operation != spec.DDLOperationAlterTable {
		return false
	}
	if statement.Dialect != spec.DialectMySQL {
		return false
	}
	for _, alter := range tableOptionAlterSpecs(statement) {
		if _, ok := alter.Options["key_block_size"]; ok {
			return true
		}
	}
	return false
}

func (r alterTableOptionApplicabilityRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	snapshot, hasSnapshot := targetTableSnapshot(statement)
	if !hasSnapshot || snapshot == nil || !snapshot.Exists {
		return nil, nil
	}
	engine := snapshot.Options["engine"]
	if !strings.EqualFold(engine, "innodb") {
		return nil, nil
	}
	rowFormat := snapshot.Options["row_format"]
	if strings.TrimSpace(rowFormat) == "" || strings.EqualFold(rowFormat, "compressed") {
		return nil, nil
	}
	for _, alter := range tableOptionAlterSpecs(statement) {
		value, ok := alter.Options["key_block_size"]
		if !ok {
			continue
		}
		return []rule.Finding{{
			RuleID:     r.ID(),
			Level:      r.level,
			Message:    fmt.Sprintf("table option key_block_size=%s is refused on a %s InnoDB table (probed 1478)", value, rowFormat),
			Suggestion: "key_block_size only takes effect with ROW_FORMAT=COMPRESSED — set the row format first or drop the option",
			Metadata: map[string]any{
				"table":      statement.DDL.Table.Name,
				"action":     alter.Action,
				"option":     "key_block_size",
				"value":      value,
				"engine":     engine,
				"row_format": rowFormat,
				"refusal":    "1478",
			},
		}}, nil
	}
	return nil, nil
}

func (r alterTableOptionApplicabilityRule) EvidenceGaps(statement spec.Statement) []rule.EvidenceGap {
	if !r.AppliesTo(statement) {
		return nil
	}
	snapshot, hasSnapshot := targetTableSnapshot(statement)
	if !hasSnapshot || snapshot == nil {
		return []rule.EvidenceGap{{
			RuleID:        r.ID(),
			ReasonCode:    gapReasonUnknownTableState,
			RequiredFacts: []string{"target_table.engine"},
		}}
	}
	if !snapshot.Exists {
		return nil
	}
	engine := snapshot.Options["engine"]
	if strings.TrimSpace(engine) == "" {
		return []rule.EvidenceGap{{
			RuleID:        r.ID(),
			ReasonCode:    gapReasonIncompleteTableFact,
			RequiredFacts: []string{"target_table.engine"},
		}}
	}
	if !strings.EqualFold(engine, "innodb") {
		// A known non-InnoDB engine is outside the probed matrix — the
		// declaration stays unproven rather than silently accepted.
		return []rule.EvidenceGap{{
			RuleID:        r.ID(),
			ReasonCode:    gapReasonUnprovenStatementFact,
			RequiredFacts: []string{"statement.key_block_size"},
		}}
	}
	if strings.TrimSpace(snapshot.Options["row_format"]) == "" {
		return []rule.EvidenceGap{{
			RuleID:        r.ID(),
			ReasonCode:    gapReasonIncompleteTableFact,
			RequiredFacts: []string{"target_table.row_format"},
		}}
	}
	return nil
}

// ---- ddl.alter.table_option.warn ----------------------------------------

// alterTableOptionWarnRule warns on accepted-but-risky option
// declarations the state layer records as declared: page compression,
// runtime-dependent encryption and tablespace bindings, key block size,
// and MERGE-only insert methods.
type alterTableOptionWarnRule struct {
	level rule.Level
}

func newAlterTableOptionWarnRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return alterTableOptionWarnRule{level: configuredLevel(cfg, rule.LevelWarning)}, nil
}

func (r alterTableOptionWarnRule) ID() string { return ruleIDAlterTableOptionWarn }

func (r alterTableOptionWarnRule) AppliesTo(statement spec.Statement) bool {
	if statement.DDL == nil || statement.DDL.Operation != spec.DDLOperationAlterTable {
		return false
	}
	if statement.Dialect != spec.DialectMySQL {
		return false
	}
	for _, alter := range tableOptionAlterSpecs(statement) {
		for _, name := range []string{"compression", "encryption", "tablespace", "insert_method", "key_block_size"} {
			if _, ok := alter.Options[name]; ok {
				return true
			}
		}
	}
	return false
}

func (r alterTableOptionWarnRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}
	findings := make([]rule.Finding, 0, 2)
	for _, alter := range tableOptionAlterSpecs(statement) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		names := make([]string, 0, len(alter.Options))
		for name := range alter.Options {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			value := alter.Options[name]
			var message string
			switch name {
			case "compression":
				message = "table option compression is accepted but only applies to a compressed InnoDB row format"
			case "encryption":
				message = "table option encryption depends on a runtime keyring the audit cannot observe"
			case "tablespace":
				message = fmt.Sprintf("table option tablespace=%q requires a pre-existing general tablespace", value)
			case "insert_method":
				message = fmt.Sprintf("table option insert_method=%q only takes effect on MERGE tables", value)
			case "key_block_size":
				message = fmt.Sprintf("table option key_block_size=%s only takes effect with ROW_FORMAT=COMPRESSED or on MyISAM", value)
			default:
				continue
			}
			findings = append(findings, rule.Finding{
				RuleID:     r.ID(),
				Level:      r.level,
				Message:    message,
				Suggestion: "verify the option actually took effect against the live engine before relying on it",
				Metadata: map[string]any{
					"table":  statement.DDL.Table.Name,
					"action": alter.Action,
					"option": name,
					"value":  value,
				},
			})
		}
	}
	return findings, nil
}
