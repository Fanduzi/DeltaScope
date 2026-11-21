package spec

import "strconv"

// PGOperatorChange preserves the authored operator signature and destination.
type PGOperatorChange struct {
	Source              PGDestructiveTableTarget
	Left, Right         PGColumnTypeTarget
	Unary, Signature    bool
	Action, Destination string
	Owner               PGTableOwnerTarget
	Restrict, Join      Table
}

// PGOperatorEstimatorOption preserves one authored option. An empty Function denotes NONE.
type PGOperatorEstimatorOption struct {
	Kind     string
	Function Table
}

// PGOperatorChangeState contains the exact, observed change premises.
type PGOperatorChangeState struct {
	Target                                  PGOperatorChange
	OperandSchema, OperandTypes             bool
	SourceSchema, SourceExists, OwnerExists bool
	DestinationSchema, NamespaceIneligible  bool
	DestinationOccupied                     bool
	BooleanResult                           bool
	RestrictMatches, JoinMatches            int
	RestrictReturnsFloat, JoinReturnsFloat  bool
}

// PGOperatorChangeLookup returns a request only for an exact qualified signature.
func PGOperatorChangeLookup(s Statement) *ObjectLookupRequest {
	if s.Dialect != DialectPostgreSQL || s.Kind != KindDDL || s.Unsupported != nil || s.DDL == nil ||
		s.DDL.Operation != DDLOperationAlterOperator || s.DDL.PGOperatorChange == nil {
		return nil
	}
	t := s.DDL.PGOperatorChange
	if !t.Signature || !t.Source.Qualified() || t.Source.Table.Schema == "pg_toast_temp" {
		return nil
	}
	switch t.Action {
	case "set_owner":
		if !t.Owner.Valid() {
			return nil
		}
	case "set_estimators":
		for _, estimator := range []Table{t.Restrict, t.Join} {
			if estimator.Name != "" && (estimator.Schema == "pg_toast_temp" || !(PGDestructiveTableTarget{Table: estimator}).Qualified()) {
				return nil
			}
		}
	case "set_schema":
		if t.Destination == "" || t.Destination == "pg_temp" || t.Destination == "pg_toast_temp" {
			return nil
		}
	default:
		return nil
	}
	return &ObjectLookupRequest{Type: "pg_operator_change", Schema: t.Source.Table.Schema, Name: t.Source.Table.Name,
		Qualifiers: map[string]string{"action": t.Action, "destination": t.Destination, "role_kind": t.Owner.Kind, "role_name": t.Owner.Name,
			"left_schema": t.Left.Schema, "left_type": t.Left.Name, "left_array": strconv.FormatBool(t.Left.Array), "unary": strconv.FormatBool(t.Unary),
			"right_schema": t.Right.Schema, "right_type": t.Right.Name, "right_array": strconv.FormatBool(t.Right.Array),
			"restrict_schema": t.Restrict.Schema, "restrict_name": t.Restrict.Name, "join_schema": t.Join.Schema, "join_name": t.Join.Name}}
}
