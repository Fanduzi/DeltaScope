//go:build postgresql

package postgresql

import (
	"math"
	"strconv"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	pg_query "github.com/pganalyze/pg_query_go/v6"
)

func pgColumnTuningFromCmd(cmd *pg_query.AlterTableCmd) *spec.PGColumnTuning {
	facts := &spec.PGColumnTuning{}
	if cmd.GetSubtype() == pg_query.AlterTableType_AT_SetStatistics {
		if cmd.GetDef() == nil {
			facts.StatisticsDefault = true
		} else if n := cmd.GetDef().GetInteger(); n != nil {
			value := int64(n.GetIval())
			facts.StatisticsValue = &value
		} else {
			facts.Unmodeled = true
		}
		return facts
	}
	list := cmd.GetDef().GetList()
	if list == nil || len(list.GetItems()) == 0 {
		facts.Unmodeled = true
		return facts
	}
	facts.Options = make([]spec.PGColumnOption, 0, len(list.GetItems()))
	for _, item := range list.GetItems() {
		d := item.GetDefElem()
		if d == nil || d.GetDefname() == "" {
			facts.Options = append(facts.Options, spec.PGColumnOption{ValueKind: spec.PGColumnOptionUnmodeled})
			facts.Unmodeled = true
			continue
		}
		option := spec.PGColumnOption{Name: d.GetDefname(), Namespace: d.GetDefnamespace(), ValueKind: spec.PGColumnOptionUnmodeled}
		arg := d.GetArg()
		switch {
		case arg == nil:
			option.ValueKind = spec.PGColumnOptionAbsent
		case arg.GetString_() != nil:
			option.ValueKind = spec.PGColumnOptionQuoted
		case arg.GetInteger() != nil:
			value := float64(arg.GetInteger().GetIval())
			option.Numeric = &value
			option.ValueKind = spec.PGColumnOptionNumeric
		case arg.GetFloat() != nil:
			value, err := strconv.ParseFloat(arg.GetFloat().GetFval(), 64)
			if err == nil && !math.IsInf(value, 0) && !math.IsNaN(value) {
				option.Numeric = &value
				option.ValueKind = spec.PGColumnOptionNumeric
			}
		}
		facts.Options = append(facts.Options, option)
	}
	return facts
}
