package deltascope

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
)

func t18R1Policy(t *testing.T, enabled ...string) string {
	t.Helper()
	allowed := map[string]bool{}
	for _, id := range enabled {
		allowed[id] = true
	}
	ids := make([]string, 0, len(policy.Default().Rules))
	for id := range policy.Default().Rules {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	var config strings.Builder
	config.WriteString("rules:\n")
	for _, id := range ids {
		fmt.Fprintf(&config, "  %s:\n    enabled: %t\n", strconv.Quote(id), allowed[id])
	}
	path := filepath.Join(t.TempDir(), "t18r1.yaml")
	if err := os.WriteFile(path, []byte(config.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func findingsForRule(result Result, statementIndex int, ruleID string) []Finding {
	var findings []Finding
	for _, finding := range result.Statements[statementIndex].Findings {
		if finding.RuleID == ruleID {
			findings = append(findings, finding)
		}
	}
	return findings
}

func TestT18R1DroppedSchemaRefusesCreateView(t *testing.T) {
	result, err := Audit(context.Background(), Request{
		SQL:     "DROP DATABASE IF EXISTS g;\nCREATE VIEW g.v AS SELECT id FROM src.t;\nCREATE VIEW g.v AS SELECT id FROM src.t;",
		Dialect: DialectMySQL, Schema: "g",
		ConfigPath:       t18R1Policy(t, "ddl.view.target.valid", "ddl.view.source.valid"),
		MetadataProvider: t18FirstProvider{},
	})
	if err != nil || result.Verdict != VerdictReject || len(result.Statements) != 3 {
		t.Fatalf("err=%v result=%#v", err, result)
	}
	for i := 1; i <= 2; i++ {
		findings := findingsForRule(result, i, "ddl.view.target.valid")
		if len(findings) != 1 || findings[0].Level != LevelBlocker {
			t.Fatalf("schema premise did not refuse statement %d: %#v", i, result.Statements[i])
		}
	}
}

func TestT18R1CheckOptionRejectsStaticallyNonUpdatableViews(t *testing.T) {
	for _, tc := range []struct {
		name string
		sql  string
	}{
		{"computed_only", "CREATE VIEW calc_v AS SELECT id + 1 AS x FROM t WITH CHECK OPTION;"},
		{"outer_join", "CREATE VIEW join_v AS SELECT t.id, u.id FROM t LEFT JOIN u ON t.id = u.id WITH LOCAL CHECK OPTION;"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result, err := Audit(context.Background(), Request{
				SQL: tc.sql, Dialect: DialectMySQL, Schema: "g",
				ConfigPath:       t18R1Policy(t, "ddl.view.check_option.valid"),
				MetadataProvider: t18FirstProvider{},
			})
			if err != nil || result.Verdict != VerdictReject || result.Coverage.Status != CoverageComplete ||
				len(result.Statements) != 1 {
				t.Fatalf("err=%v result=%#v", err, result)
			}
			findings := findingsForRule(result, 0, "ddl.view.check_option.valid")
			if len(findings) != 1 || findings[0].Level != LevelBlocker {
				t.Fatalf("check option finding=%#v", result.Statements[0].Findings)
			}
		})
	}
}

func TestT18R1CheckOptionKeepsUpdatableControls(t *testing.T) {
	for _, tc := range []struct {
		name string
		sql  string
	}{
		{"direct", "CREATE VIEW direct_v AS SELECT id FROM t WITH CHECK OPTION;"},
		{"mixed_projection", "CREATE VIEW mixed_v AS SELECT id, id + 1 AS next_id FROM t WITH LOCAL CHECK OPTION;"},
		{"inner_join", "CREATE VIEW inner_v AS SELECT t.id, u.id FROM t JOIN u ON t.id = u.id WITH LOCAL CHECK OPTION;"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result, err := Audit(context.Background(), Request{
				SQL: tc.sql, Dialect: DialectMySQL, Schema: "g",
				ConfigPath:       t18R1Policy(t, "ddl.view.check_option.valid"),
				MetadataProvider: t18FirstProvider{},
			})
			if err != nil || result.Verdict != VerdictPass || result.Coverage.Status != CoverageComplete ||
				len(result.Statements) != 1 || len(result.Statements[0].Findings) != 0 {
				t.Fatalf("err=%v result=%#v", err, result)
			}
		})
	}
}

func TestT18R1InheritedAlgorithmWarnsAndFallsBack(t *testing.T) {
	result, err := Audit(context.Background(), Request{
		SQL:     "ALTER VIEW v AS SELECT COUNT(*) AS n FROM t;",
		Dialect: DialectMySQL, Schema: "g",
		ConfigPath:       t18R1Policy(t, "ddl.view.algorithm.warn"),
		MetadataProvider: t18FirstProvider{targetMode: "merge"},
	})
	if err != nil || result.Verdict != VerdictReview || result.Coverage.Status != CoverageComplete ||
		len(result.Statements) != 1 {
		t.Fatalf("err=%v result=%#v", err, result)
	}
	findings := findingsForRule(result, 0, "ddl.view.algorithm.warn")
	if len(findings) != 1 || findings[0].Level != LevelWarning {
		t.Fatalf("inherited merge fallback finding=%#v", result.Statements[0].Findings)
	}
}

func TestT18R1CorrelatedSubqueryAcceptsNearestOuterScope(t *testing.T) {
	result, err := Audit(context.Background(), Request{
		SQL: "CREATE VIEW cv AS SELECT a.id FROM a WHERE EXISTS (\n" +
			"  SELECT 1 FROM u AS b WHERE EXISTS (\n" +
			"    SELECT 1 WHERE id > 0\n" +
			"  )\n" +
			");",
		Dialect: DialectMySQL, Schema: "g",
		ConfigPath:       t18R1Policy(t, "ddl.view.source.valid"),
		MetadataProvider: t18FirstProvider{extraTable: "a"},
	})
	if err != nil || result.Verdict != VerdictPass || result.Coverage.Status != CoverageComplete ||
		len(result.Statements) != 1 || len(result.Statements[0].Findings) != 0 {
		t.Fatalf("err=%v result=%#v", err, result)
	}
}
