// Package audit runs the frozen T07 contract matrix offline.
package audit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"

	domainpolicy "github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type t07GoldenCase struct {
	ID       string          `json:"id"`
	Dialect  string          `json:"dialect"`
	Anchor   string          `json:"anchor"`
	Policy   string          `json:"policy"`
	SQL      string          `json:"sql"`
	Args     []string        `json:"args"`
	Expected map[string]any  `json:"expect"`
}

type t07Golden struct {
	Anchors map[string]struct {
		Product         string `json:"product"`
		VersionContains string `json:"version_contains"`
	} `json:"anchors"`
	Policy struct {
		Profiles map[string]struct {
			Enable map[string]struct {
				Enabled bool           `json:"enabled"`
				Level   string         `json:"level"`
				Params  map[string]any `json:"params"`
			} `json:"enable"`
		} `json:"profiles"`
	} `json:"policy"`
	CLICases     []t07GoldenCase `json:"cli_cases"`
	MetadataCase []t07GoldenCase `json:"metadata_cases"`
}

// t07GoldenTargetVersion resolves the case's target version: CLI cases carry
// --target-version in args; metadata cases resolve their anchor's observed
// version (the "v" prefix on TiDB banners is stripped).
func (c t07GoldenCase) t07TargetVersion(g t07Golden) string {
	for i := 0; i+1 < len(c.Args); i++ {
		if c.Args[i] == "--target-version" {
			return c.Args[i+1]
		}
	}
	if anchor, ok := g.Anchors[c.Anchor]; ok {
		return strings.TrimPrefix(anchor.VersionContains, "v")
	}
	return ""
}

// t07PolicyConfigPath renders the frozen policy profile into the YAML config
// shape the audit consumes: every default rule disabled, the profile's
// enable-map entries enabled at their frozen level and params.
func t07PolicyConfigPath(t *testing.T, g t07Golden, profile string) string {
	t.Helper()
	var builder strings.Builder
	builder.WriteString("rules:\n")
	enables := map[string]struct {
		level  string
		params map[string]any
	}{}
	if p, ok := g.Policy.Profiles[profile]; ok {
		for id, entry := range p.Enable {
			if !entry.Enabled {
				continue
			}
			level := entry.Level
			if level == "" {
				level = "blocker"
			}
			enables[id] = struct {
				level  string
				params map[string]any
			}{level: level, params: entry.Params}
		}
	}
	ruleIDs := make([]string, 0, len(domainpolicy.Default().Rules))
	for id := range domainpolicy.Default().Rules {
		ruleIDs = append(ruleIDs, id)
	}
	sort.Strings(ruleIDs)
	for _, id := range ruleIDs {
		if _, keep := enables[id]; keep {
			continue
		}
		fmt.Fprintf(&builder, "  %s:\n    enabled: false\n", strconv.Quote(id))
	}
	enabledIDs := make([]string, 0, len(enables))
	for id := range enables {
		enabledIDs = append(enabledIDs, id)
	}
	sort.Strings(enabledIDs)
	for _, id := range enabledIDs {
		if _, known := domainpolicy.Default().Rules[id]; !known {
			t.Fatalf("profile %q enables unregistered rule %q", profile, id)
		}
		entry := enables[id]
		fmt.Fprintf(&builder, "  %s:\n    enabled: true\n    level: %s\n", strconv.Quote(id), entry.level)
		if len(entry.params) > 0 {
			builder.WriteString("    params:\n")
			keys := make([]string, 0, len(entry.params))
			for key := range entry.params {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			for _, key := range keys {
				fmt.Fprintf(&builder, "      %s: %s\n", key, t07YAMLScalar(entry.params[key]))
			}
		}
	}
	path := filepath.Join(t.TempDir(), "t07-policy.yaml")
	if err := os.WriteFile(path, []byte(builder.String()), 0o600); err != nil {
		t.Fatalf("write policy: %v", err)
	}
	return path
}

// t07YAMLScalar renders a frozen policy param value into YAML.
func t07YAMLScalar(value any) string {
	switch v := value.(type) {
	case bool:
		return strconv.FormatBool(v)
	case string:
		return strconv.Quote(v)
	case []any:
		parts := make([]string, 0, len(v))
		for _, item := range v {
			parts = append(parts, t07YAMLScalar(item))
		}
		return "[" + strings.Join(parts, ", ") + "]"
	default:
		return fmt.Sprintf("%v", v)
	}
}

// t07ExpectEqual asserts one frozen expectation key against the actual value.
// The actual side is JSON-normalized first so []string/map[string]any shapes
// compare byte-faithfully against the decoded oracle.
func t07ExpectEqual(t *testing.T, id, key string, want, got any) {
	t.Helper()
	var normalized any
	if raw, err := json.Marshal(got); err == nil {
		_ = json.Unmarshal(raw, &normalized)
	} else {
		normalized = got
	}
	if !reflect.DeepEqual(want, normalized) {
		t.Fatalf("%s: %s = %#v, want %#v", id, key, normalized, want)
	}
}

func t07ExpectFloat(t *testing.T, id, key string, want any, got int) {
	t.Helper()
	t07ExpectEqual(t, id, key, want, float64(got))
}

// t07ActualEntries projects the report into the frozen entry shapes.
func t07ActualEntries(result report.Result) (findings, metadata, locations, gaps, unsupported []any, exact []any) {
	for _, stmt := range result.Statements {
		for _, f := range stmt.Findings {
			findings = append(findings, map[string]any{
				"index": float64(stmt.Index), "rule_id": f.RuleID, "level": string(f.Level),
			})
			metadata = append(metadata, map[string]any{
				"index": float64(stmt.Index), "rule_id": f.RuleID, "metadata": f.Metadata,
			})
			entry := map[string]any{
				"index": float64(stmt.Index),
				"line":  float64(0), "column": float64(0),
			}
			if f.Location != nil {
				entry["line"] = float64(f.Location.Line)
				entry["column"] = float64(f.Location.Column)
			}
			locations = append(locations, entry)
			exact = append(exact, map[string]any{
				"index": float64(stmt.Index), "rule_id": f.RuleID,
				"level": string(f.Level), "message": f.Message, "metadata": f.Metadata,
			})
		}
		for _, gap := range stmt.EvidenceGaps {
			gaps = append(gaps, map[string]any{
				"index":          float64(stmt.Index),
				"rule_id":        gap.RuleID,
				"reason_code":    gap.ReasonCode,
				"required_facts": gap.RequiredFacts,
			})
		}
	}
	for _, item := range result.Unsupported {
		unsupported = append(unsupported, map[string]any{
			"index":    float64(item.Index),
			"feature":  item.Feature,
			"reason":   item.Reason,
			"metadata": item.Metadata,
		})
	}
	return findings, metadata, locations, gaps, unsupported, exact
}

// TestT07FrozenMatrix drives every non-provider contract cell in the frozen
// oracle through the real shared audit seam — no derived expectations. The 11
// provider-* cells need a live catalog and stay out of the offline run.
func TestT07FrozenMatrix(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "ddl-golden", "T07.json"))
	if err != nil {
		t.Fatalf("read T07 oracle: %v", err)
	}
	var golden t07Golden
	if err := json.Unmarshal(raw, &golden); err != nil {
		t.Fatalf("decode T07 oracle: %v", err)
	}
	cases := append(append([]t07GoldenCase{}, golden.CLICases...), golden.MetadataCase...)
	skipped := 0
	for _, tc := range cases {
		if strings.Contains(tc.ID, "provider-") {
			skipped++
			continue
		}
		tc := tc
		t.Run(tc.ID, func(t *testing.T) {
			t.Parallel()
			result, err := AuditSQL(context.Background(), Request{
				SQL:           tc.SQL,
				Dialect:       spec.Dialect(tc.Dialect),
				Schema:        "golden",
				TargetVersion: tc.t07TargetVersion(golden),
				ConfigPath:    t07PolicyConfigPath(t, golden, tc.Policy),
			})
			if err != nil && !errors.Is(err, ErrUnsupportedStatement) && !errors.Is(err, errParserUnsupported) {
				t.Fatalf("audit: %v", err)
			}
			expect := tc.Expected
			actualExit := 0
			if err != nil || result.Verdict == report.VerdictReject {
				actualExit = 1
			}
			t07ExpectFloat(t, tc.ID, "exit", expect["exit"], actualExit)
			if v, ok := expect["verdict"]; ok {
				t07ExpectEqual(t, tc.ID, "verdict", v, string(result.Verdict))
			}
			t07ExpectFloat(t, tc.ID, "statements", expect["statements"], len(result.Statements))
			totalFindings := 0
			totalGaps := 0
			statementCoverage := make([]any, 0, len(result.Statements))
			statementSQL := make([]any, 0, len(result.Statements))
			indices := make([]any, 0, len(result.Statements))
			for _, stmt := range result.Statements {
				totalFindings += len(stmt.Findings)
				totalGaps += len(stmt.EvidenceGaps)
				statementCoverage = append(statementCoverage, string(stmt.Coverage.Status))
				statementSQL = append(statementSQL, stmt.RawSQL)
				indices = append(indices, float64(stmt.Index))
			}
			t07ExpectFloat(t, tc.ID, "findings", expect["findings"], totalFindings)
			t07ExpectFloat(t, tc.ID, "diagnostics", expect["diagnostics"], len(result.Diagnostics))
			t07ExpectFloat(t, tc.ID, "unsupported", expect["unsupported"], len(result.Unsupported))
			t07ExpectFloat(t, tc.ID, "evidence_gaps", expect["evidence_gaps"], totalGaps)
			if v, ok := expect["coverage"]; ok {
				t07ExpectEqual(t, tc.ID, "coverage", v, string(result.Coverage.Status))
			}
			if v, ok := expect["statement_coverage"]; ok {
				t07ExpectEqual(t, tc.ID, "statement_coverage", v, []any(statementCoverage))
			}
			if v, ok := expect["statement_sql"]; ok {
				t07ExpectEqual(t, tc.ID, "statement_sql", v, []any(statementSQL))
			}
			if v, ok := expect["statement_indices"]; ok {
				t07ExpectEqual(t, tc.ID, "statement_indices", v, []any(indices))
			}
			findings, metadata, locations, gaps, unsupported, exact := t07ActualEntries(result)
			empty := []any{}
			for _, key := range []string{"finding_entries", "finding_metadata", "finding_locations", "evidence_gap_entries", "unsupported_entries", "t07_exact_findings"} {
				want, present := expect[key]
				if !present {
					continue
				}
				var got any
				switch key {
				case "finding_entries":
					got = findings
				case "finding_metadata":
					got = metadata
				case "finding_locations":
					got = locations
				case "evidence_gap_entries":
					got = gaps
				case "unsupported_entries":
					got = unsupported
				case "t07_exact_findings":
					got = exact
				}
				if got == nil || reflect.ValueOf(got).Len() == 0 {
					got = empty
				}
				t07ExpectEqual(t, tc.ID, key, want, got)
			}
			if v, ok := expect["rule_summary_loaded"]; ok {
				loaded := 0
				if result.RuleSummary != nil {
					loaded = result.RuleSummary.Loaded
				}
				t07ExpectFloat(t, tc.ID, "rule_summary_loaded", v, loaded)
			}
			if v, ok := expect["rule_summary_applicable"]; ok {
				applicable := 0
				if result.RuleSummary != nil {
					applicable = result.RuleSummary.Applicable
				}
				t07ExpectFloat(t, tc.ID, "rule_summary_applicable", v, applicable)
			}
		})
	}
	t.Logf("provider cells skipped (live catalog required): %d", skipped)
}
