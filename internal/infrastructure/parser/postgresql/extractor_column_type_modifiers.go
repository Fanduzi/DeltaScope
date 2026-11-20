//go:build postgresql

package postgresql

import (
	pg_query "github.com/pganalyze/pg_query_go/v6"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func pgColumnTypeModifiers(nodes []*pg_query.Node) *spec.PGColumnTypeModifiers {
	if len(nodes) == 0 {
		return nil
	}
	result := &spec.PGColumnTypeModifiers{Count: len(nodes), Known: true}
	for i, node := range nodes {
		constant := node.GetAConst()
		value := constant.GetIval()
		if constant.GetIsnull() || value == nil {
			result.Known = false
		} else if i < len(result.Values) {
			result.Values[i] = value.GetIval()
		}
	}
	return result
}
