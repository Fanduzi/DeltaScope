//go:build postgresql

package postgresql

import (
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	pg_query "github.com/pganalyze/pg_query_go/v6"
	"google.golang.org/protobuf/reflect/protopath"
	"google.golang.org/protobuf/reflect/protorange"
)

func pgColumnTransitionFromCmd(cmd *pg_query.AlterTableCmd) *spec.PGColumnTransition {
	facts := &spec.PGColumnTransition{IfExists: cmd.GetMissingOk()}
	var options []*pg_query.Node
	switch cmd.GetSubtype() {
	case pg_query.AlterTableType_AT_AddIdentity:
		c := cmd.GetDef().GetConstraint()
		if c == nil || c.GetContype() != pg_query.ConstrType_CONSTR_IDENTITY {
			facts.Unmodeled = true
			return facts
		}
		facts.Mode, facts.ModeCount = c.GetGeneratedWhen(), 1
		facts.Unmodeled = facts.Mode != "a" && facts.Mode != "d"
		options = c.GetOptions()
	case pg_query.AlterTableType_AT_SetIdentity:
		list := cmd.GetDef().GetList()
		if list == nil || len(list.GetItems()) == 0 {
			facts.Unmodeled = true
			return facts
		}
		options = list.GetItems()
	case pg_query.AlterTableType_AT_ColumnDefault:
		if expression := cmd.GetDef(); expression != nil {
			facts.DefaultHasDisallowedReference = pgExpressionHasReference(expression, true)
			facts.DefaultHasParameter = expression.GetParamRef() != nil
			for _, arg := range expression.GetCoalesceExpr().GetArgs() {
				if arg.GetParamRef() != nil {
					facts.DefaultHasParameter = true
					break
				}
			}
			constant := expression.GetAConst()
			switch {
			case constant.GetBoolval() != nil:
				facts.DefaultLiteralType = "bool"
			case constant.GetIval() != nil:
				facts.DefaultLiteralType = "int4"
			}
		}
	}
	for _, node := range options {
		d := node.GetDefElem()
		if d != nil && d.GetDefname() == "generated" {
			facts.ModeCount++
			arg := d.GetArg().GetInteger()
			if arg == nil || (arg.GetIval() != 'a' && arg.GetIval() != 'd') {
				facts.Unmodeled = true
				continue
			}
			if facts.ModeCount == 1 {
				facts.Mode = string(arg.GetIval())
			}
			continue
		}
		option, known := pgIdentityOption(d)
		facts.Options = append(facts.Options, option)
		facts.Unmodeled = facts.Unmodeled || !known
	}
	return facts
}

func pgExpressionHasReference(expression *pg_query.Node, includeColumn bool) bool {
	if expression == nil {
		return false
	}
	found := false
	_ = protorange.Range(expression.ProtoReflect(), func(path protopath.Values) error {
		field := path.Index(-1)
		if descriptor := field.Step.FieldDescriptor(); descriptor != nil && !descriptor.IsList() && descriptor.Message() != nil {
			switch field.Value.Message().Interface().(type) {
			case *pg_query.TypeName:
				// PostgreSQL accepts simple identifiers as type modifiers.
				return protorange.Break
			case *pg_query.ColumnRef:
				if includeColumn {
					found = true
					return protorange.Terminate
				}
			case *pg_query.SubLink:
				found = true
				return protorange.Terminate
			}
		}
		return nil
	})
	return found
}
