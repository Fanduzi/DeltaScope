//go:build postgresql

package postgresql

import (
	"strconv"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	pg_query "github.com/pganalyze/pg_query_go/v6"
)

func pgGenerationFromDef(column *pg_query.ColumnDef) *spec.PGColumnGeneration {
	facts := &spec.PGColumnGeneration{TypeWidth: pgIdentityTypeWidth(column.GetTypeName())}
	for _, item := range column.GetConstraints() {
		c := item.GetConstraint()
		if c == nil {
			continue
		}
		switch c.GetContype() {
		case pg_query.ConstrType_CONSTR_NULL:
			facts.ExplicitNull = true
		case pg_query.ConstrType_CONSTR_IDENTITY, pg_query.ConstrType_CONSTR_GENERATED:
			if facts.IdentityCount+facts.StoredCount == 0 {
				facts.Mode = c.GetGeneratedWhen()
			}
			if c.GetContype() == pg_query.ConstrType_CONSTR_GENERATED {
				facts.StoredCount++
				continue
			}
			facts.IdentityCount++
			for _, node := range c.GetOptions() {
				option, ok := pgIdentityOption(node.GetDefElem())
				facts.Options = append(facts.Options, option)
				facts.Unmodeled = facts.Unmodeled || !ok
			}
		}
	}
	if facts.IdentityCount+facts.StoredCount == 0 {
		return nil
	}
	return facts
}

func pgIdentityTypeWidth(t *pg_query.TypeName) int {
	if t == nil {
		return 0
	}
	if len(t.GetArrayBounds()) > 0 {
		return -1
	}
	names := stringValuesFromNodes(t.GetNames())
	if len(names) == 2 && names[0] == "pg_catalog" {
		switch names[1] {
		case "int2":
			return 16
		case "int4":
			return 32
		case "int8":
			return 64
		default:
			return -1
		}
	}
	// Unqualified/custom names require catalog resolution, never a guessed base type.
	return 0
}

func pgIdentityOption(d *pg_query.DefElem) (spec.SequenceOption, bool) {
	unknown := spec.SequenceOption{Kind: spec.SequenceOptionUnknown}
	if d == nil {
		return unknown, false
	}
	arg := d.GetArg()
	if d.GetDefname() == "cycle" {
		if arg == nil || arg.GetBoolean() == nil {
			return unknown, false
		}
		kind := spec.SequenceOptionNoCycle
		if arg.GetBoolean().GetBoolval() {
			kind = spec.SequenceOptionCycle
		}
		return spec.SequenceOption{Kind: kind}, true
	}
	kinds := map[string]spec.SequenceOptionKind{
		"increment": spec.SequenceOptionIncrementBy, "start": spec.SequenceOptionStartWith,
		"minvalue": spec.SequenceOptionMinValue, "maxvalue": spec.SequenceOptionMaxValue,
		"cache": spec.SequenceOptionCache,
	}
	kind, ok := kinds[d.GetDefname()]
	if !ok {
		return unknown, false
	}
	if arg == nil {
		switch kind {
		case spec.SequenceOptionMinValue:
			return spec.SequenceOption{Kind: spec.SequenceOptionNoMinValue}, true
		case spec.SequenceOptionMaxValue:
			return spec.SequenceOption{Kind: spec.SequenceOptionNoMaxValue}, true
		}
		return unknown, false
	}
	var value int64
	switch {
	case arg.GetInteger() != nil:
		value = int64(arg.GetInteger().GetIval())
	case arg.GetFloat() != nil:
		var err error
		value, err = strconv.ParseInt(arg.GetFloat().GetFval(), 10, 64)
		if err != nil {
			return unknown, false
		}
	default:
		return unknown, false
	}
	return spec.SequenceOption{Kind: kind, Value: &value}, true
}
