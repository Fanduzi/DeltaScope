//go:build postgresql

package postgresql

import (
	"strconv"
	"strings"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	pg_query "github.com/pganalyze/pg_query_go/v6"
)

func pgCheckLiteral(expr *pg_query.Node) string {
	constant := expr.GetAConst()
	switch {
	case constant.GetIval() != nil:
		return "integer"
	case constant.GetFval() != nil:
		value := strings.ReplaceAll(constant.GetFval().GetFval(), "_", "")
		digits := strings.TrimPrefix(value, "-")
		base := 10
		if len(digits) >= 2 && digits[0] == '0' && strings.ContainsRune("xXoObB", rune(digits[1])) {
			base = 0
		}
		if integer, err := strconv.ParseInt(value, base, 64); err == nil {
			if int64(int32(integer)) == integer {
				return "integer"
			}
			return "bigint"
		}
		if base == 10 && len(value) <= 128 {
			// Keep numeric_in overflow and scale limits outside the Boolean check.
			if _, exponent, found := strings.Cut(strings.ToLower(value), "e"); found {
				power, err := strconv.ParseInt(exponent, 10, 16)
				if err != nil || power < -1000 || power > 1000 {
					return ""
				}
			}
			return "numeric"
		}
	case constant.GetSval() != nil:
		// PostgreSQL boolin trims ASCII whitespace before parsing the value.
		value := strings.Trim(constant.GetSval().GetSval(), " \t\n\r\v\f")
		if pgReloptionBooleanText(value) == spec.PGTableReloptionInvalidBoolean {
			return "invalid_boolean"
		}
	}
	return ""
}
