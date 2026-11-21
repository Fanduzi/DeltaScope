//go:build postgresql

package postgresql

import (
	"strings"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	pg_query "github.com/pganalyze/pg_query_go/v6"
)

func pgTableReloptions(items []*pg_query.Node, reset bool) []spec.PGTableReloption {
	options := make([]spec.PGTableReloption, 0, len(items))
	for _, item := range items {
		d := item.GetDefElem()
		arg := d.GetArg()
		o := spec.PGTableReloption{Name: d.GetDefname(), Namespace: d.GetDefnamespace(), HasValue: arg != nil}
		if !reset && o.Namespace == "" {
			switch o.Name {
			case "fillfactor":
				if n := arg.GetInteger(); n != nil {
					o.Value = spec.PGTableReloptionInteger
					o.Integer = n.GetIval()
				}
			case "autovacuum_enabled":
				o.Value = pgBooleanReloptionValue(arg)
			}
		}
		options = append(options, o)
	}
	return options
}

func pgBooleanReloptionValue(arg *pg_query.Node) spec.PGTableReloptionValue {
	switch {
	case arg == nil || arg.GetBoolean() != nil:
		return spec.PGTableReloptionBoolean
	case arg.GetInteger() != nil:
		n := arg.GetInteger().GetIval()
		if n == 0 || n == 1 {
			return spec.PGTableReloptionBoolean
		}
		return spec.PGTableReloptionInvalidBoolean
	case arg.GetFloat() != nil:
		if arg.GetFloat().GetFval() == "1.0" {
			return spec.PGTableReloptionInvalidBoolean
		}
	case arg.GetString_() != nil:
		return pgReloptionBooleanText(arg.GetString_().GetSval())
	case arg.GetTypeName() != nil:
		t := arg.GetTypeName()
		if len(t.GetNames()) == 1 && t.GetNames()[0].GetString_() != nil && len(t.GetArrayBounds()) == 0 && !t.GetPctType() && len(t.GetTypmods()) == 0 {
			return pgReloptionBooleanText(t.GetNames()[0].GetString_().GetSval())
		}
	}
	return spec.PGTableReloptionUnknown
}

func pgReloptionBooleanText(value string) spec.PGTableReloptionValue {
	// PostgreSQL's reloption parser calls parse_bool directly, so whitespace
	// stays significant and the ambiguous prefix "o" is refused.
	for _, c := range value {
		if c == 0 || c > 127 {
			return spec.PGTableReloptionUnknown
		}
	}
	switch strings.ToLower(value) {
	case "t", "tr", "tru", "true", "f", "fa", "fal", "fals", "false", "y", "ye", "yes", "n", "no", "on", "of", "off", "1", "0":
		return spec.PGTableReloptionBoolean
	default:
		return spec.PGTableReloptionInvalidBoolean
	}
}
