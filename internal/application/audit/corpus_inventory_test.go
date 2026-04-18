package audit

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"go.yaml.in/yaml/v3"

	domainpolicy "github.com/Fanduzi/DeltaScope/internal/domain/policy"
)

func TestSQLCorpusPrintSupportedRuleCoverageInventory(t *testing.T) {
	corpusRoot := filepath.Join("..", "..", "..", "testdata", "sql-corpus")
	files, err := corpusExpectedFiles(corpusRoot)
	if err != nil {
		t.Fatalf("walk corpus: %v", err)
	}

	covered, expectedFilesByDialect, err := corpusCoveredRuleDialects(files)
	if err != nil {
		t.Fatal(err)
	}

	ruleIDs := corpusDefaultRuleIDs()

	supportedTargets := 0
	coveredTargets := 0
	missing := make([]string, 0)
	deferred := make([]string, 0)
	corpusFilesByDialect := map[string]int{
		"mysql":      0,
		"tidb":       0,
		"postgresql": 0,
	}
	for dialect, count := range expectedFilesByDialect {
		corpusFilesByDialect[dialect] = count
	}

	for _, ruleID := range ruleIDs {
		if isDeferredCorpusCoverageRule(ruleID) {
			deferred = append(deferred, ruleID)
			continue
		}
		targets := corpusRuleDialectTargets(ruleID)
		for _, dialect := range targets {
			supportedTargets++
			if covered[ruleID][dialect] != "" {
				coveredTargets++
				continue
			}
			missing = append(missing, ruleID+"@"+dialect)
		}
	}

	coveragePercent := 100.0
	if supportedTargets > 0 {
		coveragePercent = float64(coveredTargets) * 100 / float64(supportedTargets)
	}

	fmt.Printf("SQL Corpus Supported-Rule Coverage Inventory\n")
	fmt.Printf("policy_rule_ids=%d\n", len(ruleIDs))
	fmt.Printf("supported_rule_dialect_targets=%d\n", supportedTargets)
	fmt.Printf("covered_rule_dialect_targets=%d\n", coveredTargets)
	fmt.Printf("coverage_percent=%.1f\n", coveragePercent)
	fmt.Printf("expected_yaml_files_total=%d\n", len(files))
	fmt.Printf("expected_yaml_files_by_dialect=mysql:%d tidb:%d postgresql:%d\n",
		corpusFilesByDialect["mysql"], corpusFilesByDialect["tidb"], corpusFilesByDialect["postgresql"])
	fmt.Printf("deferred_rule_ids=%d\n", len(deferred))
	for _, ruleID := range deferred {
		fmt.Printf("deferred=%s\n", ruleID)
	}
	if len(missing) == 0 {
		fmt.Printf("missing_rule_dialect_targets=0\n")
	} else {
		fmt.Printf("missing_rule_dialect_targets=%d\n", len(missing))
		for _, item := range missing {
			fmt.Printf("missing=%s\n", item)
		}
	}
}

func corpusCoveredRuleDialects(files []string) (map[string]map[string]string, map[string]int, error) {
	covered := map[string]map[string]string{}
	expectedFilesByDialect := map[string]int{}
	for _, path := range files {
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, nil, fmt.Errorf("read %s: %w", path, err)
		}
		var tc corpusExpected
		if err := yaml.Unmarshal(raw, &tc); err != nil {
			return nil, nil, fmt.Errorf("parse %s: %w", path, err)
		}
		expectedFilesByDialect[tc.Dialect]++
		if tc.Expect.Findings == nil {
			continue
		}
		for _, ruleID := range tc.Expect.Findings.Include {
			if covered[ruleID] == nil {
				covered[ruleID] = map[string]string{}
			}
			covered[ruleID][tc.Dialect] = path
		}
	}
	return covered, expectedFilesByDialect, nil
}

func corpusDefaultRuleIDs() []string {
	defaults := domainpolicy.Default()
	ruleIDs := make([]string, 0, len(defaults.Rules))
	for ruleID := range defaults.Rules {
		ruleIDs = append(ruleIDs, ruleID)
	}
	sort.Strings(ruleIDs)
	return ruleIDs
}
