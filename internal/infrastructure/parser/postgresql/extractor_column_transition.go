//go:build postgresql

package postgresql

import (
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	pg_query "github.com/pganalyze/pg_query_go/v6"
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
