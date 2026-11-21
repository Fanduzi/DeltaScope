//go:build postgresql

package postgresql

import (
	"strings"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	pg_query "github.com/pganalyze/pg_query_go/v6"
)

func pgViewOptions(nodes []*pg_query.Node) []spec.PGViewOption {
	if len(nodes) == 0 {
		return nil
	}
	options := make([]spec.PGViewOption, 0, len(nodes))
	for _, node := range nodes {
		d := node.GetDefElem()
		option := spec.PGViewOption{Name: d.GetDefname(), Namespace: d.GetDefnamespace()}
		arg := d.GetArg()
		option.HasValue = arg != nil
		switch option.Name {
		case "security_barrier", "security_invoker":
			value := pgBooleanReloptionValue(arg)
			option.ValueKnown = value != spec.PGTableReloptionUnknown
			option.ValueValid = value == spec.PGTableReloptionBoolean
		case "check_option":
			if value := arg.GetString_(); value != nil {
				option.ValueKnown = true
				text := strings.ToLower(value.GetSval())
				option.ValueValid = text == "local" || text == "cascaded"
			} else if arg == nil || arg.GetInteger() != nil || arg.GetBoolean() != nil {
				option.ValueKnown = true
			}
		}
		options = append(options, option)
	}
	return options
}
