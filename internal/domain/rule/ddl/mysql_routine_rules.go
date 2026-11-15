// Package ddl defines Tier-1 DDL rules.
package ddl

import (
	"context"
	"fmt"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// mysql_routine_rules.go — stored-routine body rules (T28/#107).
// Routine declarations carry the statically parsed body; rules here
// reason over routine-local facts — never over runtime effects.

// mysqlRoutineVariableShadowingRule reports local variables declared
// with the same name as a routine parameter — reads of the name bind
// to the local after declaration, so shadowing silently changes which
// value SQL sees (T28/#107).
type mysqlRoutineVariableShadowingRule struct {
	level rule.Level
}

func newMySQLTiDBRoutineVariableShadowingRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return mysqlRoutineVariableShadowingRule{level: configuredLevel(cfg, rule.LevelWarning)}, nil
}

func (r mysqlRoutineVariableShadowingRule) ID() string {
	return ruleIDRoutineVariableShadowingWarn
}

func (r mysqlRoutineVariableShadowingRule) AppliesTo(statement spec.Statement) bool {
	if statement.Dialect != spec.DialectMySQL {
		return false // TiDB routines stay behind the vendor boundary
	}
	if statement.Kind != spec.KindDDL || statement.DDL == nil || statement.DDL.Routine == nil {
		return false
	}
	switch statement.DDL.Operation {
	case spec.DDLOperationCreateProcedure, spec.DDLOperationCreateFunction:
	default:
		return false
	}
	decl := statement.DDL.Routine
	return len(decl.Params) > 0 && decl.Body != nil
}

func (r mysqlRoutineVariableShadowingRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}
	decl := statement.DDL.Routine
	params := make(map[string]bool, len(decl.Params))
	for _, p := range decl.Params {
		params[p.Name] = true
	}
	var out []rule.Finding
	err := eachRoutineMember(ctx, decl.Body, func(bs *spec.RoutineBodyStatement, memberIdx int) {
		if bs.Declare == nil || bs.Declare.Kind != "variable" {
			return
		}
		for _, name := range bs.Declare.Names {
			if params[name] {
				out = append(out, rule.Finding{
					Level:      r.level,
					Message:    fmt.Sprintf("local variable %q shadows routine parameter %q", name, name),
					Suggestion: "rename the local variable or the parameter so name references stay unambiguous",
					Location:   rule.ChildLocation(statement, bs.Statement.Line, bs.Statement.Column),
					Metadata: map[string]any{
						"routine":        decl.Name.Name,
						"routine_member": memberIdx,
						"variable":       name,
					},
				})
			}
		}
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// eachRoutineMember visits every member of a routine body in source
// order — recursing into compound arms, loop bodies, and declare
// members — so routine rules share one traversal. memberIdx is the
// enclosing top-level member's index; nested members report their
// containing member's index (T28).
func eachRoutineMember(ctx context.Context, body *spec.RoutineBody, visit func(bs *spec.RoutineBodyStatement, memberIdx int)) error {
	var walkMember func(bs *spec.RoutineBodyStatement, memberIdx int) error
	var walkBody func(body *spec.RoutineBody, memberIdx int) error
	walkMember = func(bs *spec.RoutineBodyStatement, memberIdx int) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		visit(bs, memberIdx)
		switch {
		case bs.Compound != nil:
			for _, br := range bs.Compound.Branches {
				if err := walkBody(br, memberIdx); err != nil {
					return err
				}
			}
			if bs.Compound.Body != nil {
				if err := walkBody(bs.Compound.Body, memberIdx); err != nil {
					return err
				}
			}
		case bs.Declare != nil && bs.Declare.Member != nil:
			if err := walkMember(bs.Declare.Member, memberIdx); err != nil {
				return err
			}
		}
		return nil
	}
	walkBody = func(body *spec.RoutineBody, memberIdx int) error {
		for i := range body.Statements {
			idx := memberIdx
			if idx < 0 {
				idx = i
			}
			if err := walkMember(&body.Statements[i], idx); err != nil {
				return err
			}
		}
		return nil
	}
	return walkBody(body, -1)
}
