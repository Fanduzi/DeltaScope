//go:build postgresql

package postgresql

import (
	"strings"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	pg_query "github.com/pganalyze/pg_query_go/v6"
)

func pgCheckLiteral(expr *pg_query.Node) string {
	constant := expr.GetAConst()
	switch {
	case constant.GetIval() != nil:
		return "integer"
	case constant.GetSval() != nil:
		// PostgreSQL boolin trims ASCII whitespace before parsing the value.
		value := strings.Trim(constant.GetSval().GetSval(), " \t\n\r\v\f")
		if pgReloptionBooleanText(value) == spec.PGTableReloptionInvalidBoolean {
			return "invalid_boolean"
		}
	}
	return ""
}
