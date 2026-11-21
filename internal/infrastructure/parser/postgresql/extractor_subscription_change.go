//go:build postgresql

package postgresql

import (
	"strconv"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	pg_query "github.com/pganalyze/pg_query_go/v6"
)

func pgSubscriptionChange(stmt *pg_query.AlterSubscriptionStmt, action string) *spec.PGSubscriptionChange {
	change := &spec.PGSubscriptionChange{Name: stmt.GetSubname(), Action: action}
	for _, node := range stmt.GetPublication() {
		change.Publications = append(change.Publications, stringNodeValue(node))
	}
	for _, node := range stmt.GetOptions() {
		def := node.GetDefElem()
		if def == nil {
			continue
		}
		option := spec.PGReplicationOption{Name: def.GetDefname()}
		switch option.Name {
		case "binary", "copy_data", "disable_on_error", "failover", "password_required", "refresh", "run_as_owner",
			"lsn", "origin", "slot_name", "streaming", "synchronous_commit":
			arg := def.GetArg()
			option.Text, option.Known = pgReplicationOptionText(arg)
			option.Bare, option.Integer = arg == nil, arg.GetInteger() != nil
		}
		change.Options = append(change.Options, option)
	}
	return change
}

func pgReplicationOptionText(node *pg_query.Node) (string, bool) {
	switch {
	case node == nil:
		return "", false
	case node.GetString_() != nil:
		return stringNodeValue(node), true
	case node.GetInteger() != nil:
		return strconv.FormatInt(int64(node.GetInteger().GetIval()), 10), true
	case node.GetBoolean() != nil:
		return strconv.FormatBool(node.GetBoolean().GetBoolval()), true
	case node.GetFloat() != nil:
		return node.GetFloat().GetFval(), true
	}
	return "", false
}
