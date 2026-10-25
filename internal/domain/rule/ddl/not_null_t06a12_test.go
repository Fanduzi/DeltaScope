// Package ddl verifies the T06-A12 ordinary CREATE TABLE non-PK-column
// NOT NULL rule boundaries on synthetic statements.
package ddl

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func t06A12NotNullRule(t *testing.T, cfg policy.RulePolicy) rule.StatementRule {
	t.Helper()
	statementRule, err := newColumnNotNullRequiredRule(cfg)
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}
	return statementRule
}

var t06A12Strict = policy.RulePolicy{
	Enabled: true, Level: rule.LevelBlocker,
	Params: map[string]any{"required": true, "allow_time_null": false}}

// TestT06A12RequiredFalseAndInvalidParameters pins required=false as a
// distinct control from a disabled rule — the rule still constructs but
// applies to nothing — and pins boolParam's strict typing on both params:
// a non-bool required or allow_time_null fails construction, never coerces.
func TestT06A12RequiredFalseAndInvalidParameters(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	statement := createTableWithColumns("t", spec.Column{Name: "c", Type: "int(11)"})

	notRequired := t06A12NotNullRule(t, policy.RulePolicy{
		Enabled: true, Level: rule.LevelBlocker,
		Params: map[string]any{"required": false, "allow_time_null": false}})
	if notRequired.AppliesTo(statement) {
		t.Fatal("required=false rule must not apply")
	}
	if findings, err := notRequired.Evaluate(ctx, statement); err != nil || len(findings) != 0 {
		t.Fatalf("required=false findings = %v, err = %v; want none", findings, err)
	}

	for _, params := range []map[string]any{
		{"required": "yes"},
		{"required": true, "allow_time_null": "yes"},
	} {
		if _, err := newColumnNotNullRequiredRule(
			policy.RulePolicy{Enabled: true, Params: params}); err == nil {
			t.Fatalf("params %v must fail rule construction", params)
		}
	}
}

// TestT06A12DefaultLevelAndParams pins the shipped defaults: omitted params
// mean required=true and allow_time_null=true, and an omitted level falls
// back to warning — never the isolated profiles' blocker.
func TestT06A12DefaultLevelAndParams(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	statement := createTableWithColumns("t",
		spec.Column{Name: "c", Type: "int(11)"},
		spec.Column{Name: "time_c", Type: "datetime"})

	defaulted := t06A12NotNullRule(t, policy.RulePolicy{Enabled: true})
	findings, err := defaulted.Evaluate(ctx, statement)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 || findings[0].Level != rule.LevelWarning {
		t.Fatalf("findings = %+v, want one warning under the default level", findings)
	}
	if findings[0].Metadata["column"] != "c" {
		t.Fatalf("finding metadata = %+v, want column c — time_c is exempt by default",
			findings[0].Metadata)
	}
}

// TestT06A12AllowTimeNullSwitch pins the parameter's two positions on one
// datetime column: allow_time_null=true exempts the time-like nullable
// column, allow_time_null=false reports it — the same declaration under two
// policies is never an exemption borrowed across profiles.
func TestT06A12AllowTimeNullSwitch(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	statement := createTableWithColumns("t", spec.Column{Name: "c", Type: "datetime"})

	allowed := t06A12NotNullRule(t, policy.RulePolicy{
		Enabled: true, Level: rule.LevelBlocker,
		Params: map[string]any{"required": true, "allow_time_null": true}})
	if findings, err := allowed.Evaluate(ctx, statement); err != nil || len(findings) != 0 {
		t.Fatalf("allow_time_null=true findings = %v, err = %v; want none", findings, err)
	}

	strict := t06A12NotNullRule(t, t06A12Strict)
	findings, err := strict.Evaluate(ctx, statement)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 ||
		findings[0].Message != `column "c" should be declared NOT NULL` ||
		findings[0].Metadata["type"] != "datetime" {
		t.Fatalf("allow_time_null=false findings = %+v, want one datetime blocker", findings)
	}
}

// TestT06A12PerColumnAttribution pins per-column ownership on a mixed table:
// the ordinary nullable column reports, while the time-like and BLOB/TEXT-like
// columns stay silent — and findings bind the column name and rendered type,
// never a table-wide flag.
func TestT06A12PerColumnAttribution(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	statement := createTableWithColumns("t",
		spec.Column{Name: "ordinary", Type: "int(11)"},
		spec.Column{Name: "time_c", Type: "datetime"},
		spec.Column{Name: "payload", Type: "text"})

	allowed := t06A12NotNullRule(t, policy.RulePolicy{
		Enabled: true, Level: rule.LevelBlocker,
		Params: map[string]any{"required": true, "allow_time_null": true}})
	findings, err := allowed.Evaluate(ctx, statement)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("time-allowed findings = %+v, want exactly one on ordinary", findings)
	}
	finding := findings[0]
	if finding.Metadata["column"] != "ordinary" ||
		finding.Metadata["table"] != "t" || finding.Metadata["type"] != "int(11)" {
		t.Fatalf("finding metadata = %+v, want table=t column=ordinary type=int(11)",
			finding.Metadata)
	}

	// Under the strict profile the time column joins the report while the
	// BLOB/TEXT-like column stays exempt — each column is judged on its own
	// declared facts, in declaration order.
	strict := t06A12NotNullRule(t, t06A12Strict)
	findings, err = strict.Evaluate(ctx, statement)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 2 {
		t.Fatalf("strict findings = %+v, want ordinary+time_c", findings)
	}
	for i, want := range []string{"ordinary", "time_c"} {
		if findings[i].Metadata["column"] != want {
			t.Fatalf("finding[%d] column = %v, want %s", i, findings[i].Metadata["column"], want)
		}
	}
}

// TestT06A12CreateTableOnlySeam pins that the rule's applicability is the
// CREATE TABLE columns seam: an ALTER statement with a nullable column does
// not fall under this rule — ALTER-time nullability stays out of scope.
func TestT06A12CreateTableOnlySeam(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	alter := spec.Statement{
		Kind: spec.KindDDL,
		DDL: &spec.DDL{
			Operation: spec.DDLOperationAlterTable,
			Table:     &spec.Table{Name: "t"},
			Alter: []spec.Alter{{Action: "modify_column", Name: "c",
				Column: &spec.AlterColumn{Definition: &spec.Column{Name: "c", Type: "int(11)"}}}},
		},
	}
	strict := t06A12NotNullRule(t, t06A12Strict)
	if strict.AppliesTo(alter) {
		t.Fatal("not-null rule must not apply to ALTER statements")
	}
	if findings, err := strict.Evaluate(ctx, alter); err != nil || len(findings) != 0 {
		t.Fatalf("alter findings = %v, err = %v; want none", findings, err)
	}
}
