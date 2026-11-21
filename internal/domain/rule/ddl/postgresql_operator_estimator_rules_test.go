package ddl

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestPGOperatorEstimatorAssessment(t *testing.T) {
	fn := spec.Table{Schema: "app", Name: "estimator"}
	for _, tc := range []struct {
		name string
		set  func(*spec.PGOperatorChangeState)
		want string
	}{
		{name: "both absent", set: func(s *spec.PGOperatorChangeState) {
			s.RestrictMatches, s.JoinMatches = 0, 0
		}, want: "alter_operator_restrict_absent"},
		{name: "RESTRICT return type before JOIN", set: func(s *spec.PGOperatorChangeState) {
			s.RestrictReturnsFloat, s.JoinMatches = false, 2
		}, want: "alter_operator_restrict_result"},
		{name: "both JOIN signatures", set: func(s *spec.PGOperatorChangeState) {
			s.JoinMatches = 2
		}, want: "alter_operator_join_ambiguous"},
		{name: "JOIN result type", set: func(s *spec.PGOperatorChangeState) {
			s.JoinReturnsFloat = false
		}, want: "alter_operator_join_result"},
		{name: "unary JOIN", set: func(s *spec.PGOperatorChangeState) {
			s.Target.Unary = true
		}, want: "alter_operator_join_requires_binary"},
		{name: "non-Boolean operand result", set: func(s *spec.PGOperatorChangeState) {
			s.BooleanResult = false
		}, want: "alter_operator_estimator_requires_boolean"},
		{name: "clear on unary non-Boolean", set: func(s *spec.PGOperatorChangeState) {
			s.Target = spec.PGOperatorChange{Unary: true}
			s.BooleanResult = false
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			state := spec.PGOperatorChangeState{Target: spec.PGOperatorChange{Restrict: fn, Join: fn}, BooleanResult: true,
				RestrictMatches: 1, JoinMatches: 1, RestrictReturnsFloat: true, JoinReturnsFloat: true}
			tc.set(&state)
			if actual := assessPGOperatorEstimators(&state); actual != tc.want {
				t.Fatalf("assessment = %q, want %q", actual, tc.want)
			}
		})
	}
}
