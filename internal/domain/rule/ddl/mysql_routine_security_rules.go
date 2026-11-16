package ddl

import (
	"context"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type mysqlRoutineSecurityDefinerRule struct {
	level rule.Level
}

func newMySQLRoutineSecurityDefinerRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return mysqlRoutineSecurityDefinerRule{level: configuredLevel(cfg, rule.LevelWarning)}, nil
}

func (r mysqlRoutineSecurityDefinerRule) ID() string { return ruleIDRoutineSecurityDefinerWarn }

func (r mysqlRoutineSecurityDefinerRule) AppliesTo(statement spec.Statement) bool {
	if statement.Dialect != spec.DialectMySQL || statement.Kind != spec.KindDDL ||
		statement.DDL == nil || statement.DDL.Routine == nil {
		return false
	}
	decl := statement.DDL.Routine
	if decl.Kind != spec.RoutineKindProcedure && decl.Kind != spec.RoutineKindFunction {
		return false
	}
	for _, c := range decl.Characteristics {
		if c.Key == "soname" {
			return false
		}
	}
	switch statement.DDL.Operation {
	case spec.DDLOperationCreateProcedure, spec.DDLOperationCreateFunction,
		spec.DDLOperationAlterProcedure, spec.DDLOperationAlterFunction:
		return true
	}
	return false
}

// routineSecurityMode never chooses a last-wins interpretation of repeated characteristics.
func routineSecurityMode(statement spec.Statement) (mode, source string, known bool) {
	count := 0
	for _, c := range statement.DDL.Routine.Characteristics {
		if c.Key == "sql_security" {
			count++
			mode = c.Value
		}
	}
	if count > 0 {
		return mode, "authored", count == 1 && (mode == "definer" || mode == "invoker")
	}
	if statement.DDL.Operation == spec.DDLOperationCreateProcedure || statement.DDL.Operation == spec.DDLOperationCreateFunction {
		return "definer", "create_default", true
	}
	return "", "", true // ALTER omission makes no claim about the existing mode.
}

func (r mysqlRoutineSecurityDefinerRule) EvidenceGaps(statement spec.Statement) []rule.EvidenceGap {
	if !r.AppliesTo(statement) {
		return nil
	}
	if _, _, known := routineSecurityMode(statement); !known {
		return []rule.EvidenceGap{{ReasonCode: "unproven_routine_security", RequiredFacts: []string{"routine.sql_security"}}}
	}
	return nil
}

func (r mysqlRoutineSecurityDefinerRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !r.AppliesTo(statement) {
		return nil, nil
	}
	mode, source, known := routineSecurityMode(statement)
	if !known || mode != "definer" {
		return nil, nil
	}
	decl := statement.DDL.Routine
	metadata := map[string]any{
		"routine": decl.Name.Name, "kind": string(decl.Kind), "operation": string(statement.DDL.Operation),
		"security": mode, "security_source": source,
	}
	if decl.Name.Schema != "" {
		metadata["schema"] = decl.Name.Schema
	}
	message := "routine declares SQL SECURITY DEFINER; execution uses the definer security context"
	if source == "create_default" {
		message = "routine omits SQL SECURITY; MySQL defaults CREATE routines to the definer security context"
	}
	return []rule.Finding{{
		Level: r.level, Message: message,
		Suggestion: "review whether definer-context execution is intended; use SQL SECURITY INVOKER when caller-context execution is appropriate",
		Metadata:   metadata,
	}}, nil
}
