package deltascope

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestAuditRoutineSecurityRetainsPartialResultAndPrivacy(t *testing.T) {
	for _, sql := range []string{
		"CREATE DEFINER='account_sentinel'@'host_sentinel' PROCEDURE p() SQL SECURITY DEFINER COMMENT 'comment_sentinel' SET @body_sentinel=1",
		"CREATE DEFINER='account_sentinel'@'host_sentinel' FUNCTION f() RETURNS INT SQL SECURITY DEFINER COMMENT 'comment_sentinel' RETURN 'body_sentinel'",
	} {
		res, err := Audit(context.Background(), Request{SQL: sql, Dialect: DialectMySQL})
		partial := strings.Contains(sql, "FUNCTION")
		if partial != errors.Is(err, ErrUnsupportedStatement) || (err != nil && !partial) {
			t.Fatalf("partial=%t err=%v", partial, err)
		}
		if res.Verdict != "review" || (len(res.Unsupported) > 0) != partial {
			t.Fatalf("lost verdict or body boundary: %+v", res)
		}
		found := false
		for _, f := range res.Statements[0].Findings {
			if f.RuleID != "ddl.routine.security.definer.warn" {
				continue
			}
			found = true
			if f.Level != "warning" || f.Metadata["security_source"] != "authored" {
				t.Fatalf("finding: %+v", f)
			}
			raw, err := json.Marshal(f)
			if err != nil {
				t.Fatal(err)
			}
			for _, marker := range []string{"account_sentinel", "host_sentinel", "comment_sentinel", "body_sentinel"} {
				if strings.Contains(string(raw), marker) {
					t.Fatalf("new finding leaked %s: %s", marker, raw)
				}
			}
		}
		if !found {
			t.Fatal("missing security warning")
		}
	}
}
