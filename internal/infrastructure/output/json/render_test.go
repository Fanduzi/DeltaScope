// Package jsonrender verifies JSON rendering behavior.
package jsonrender

import (
	"encoding/json"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
)

func TestRenderProducesStableJSONShape(t *testing.T) {
	t.Parallel()
	rendered, err := Render(report.Result{
		Verdict: report.VerdictReject,
		Summary: report.Summary{
			Statements: 1,
			Blockers:   1,
		},
		Statements: []report.StatementResult{{
			Index: 0,
			Kind:  "dml",
			Findings: []rule.Finding{{
				RuleID:  "dml.where.require",
				Level:   rule.LevelBlocker,
				Message: "UPDATE and DELETE statements must include a WHERE clause",
			}},
		}},
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(rendered, &decoded); err != nil {
		t.Fatalf("unmarshal rendered json: %v", err)
	}

	if decoded["verdict"] != "reject" {
		t.Fatalf("expected verdict reject, got %#v", decoded["verdict"])
	}
	if _, ok := decoded["summary"]; !ok {
		t.Fatal("expected summary field")
	}
	if _, ok := decoded["statements"]; !ok {
		t.Fatal("expected statements field")
	}
}
