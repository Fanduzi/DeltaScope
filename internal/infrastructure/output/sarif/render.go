// Package sarif renders audit results as SARIF 2.1.0 JSON.
// input: internal report results from the audit application flow
// output: SARIF 2.1.0 JSON payloads for CI pipeline integration
// pos: infrastructure output adapter for the SARIF CI-native renderer
// note: if this file changes, update this header and module README.md.
package sarif

import (
	"encoding/json"
	"sort"
	"strings"

	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
)

const sarifSchema = "https://docs.oasis-open.org/sarif/sarif/v2.1.0/errata01/os/schemas/sarif-schema-2.1.0.json"

// Options carries renderer configuration.
type Options struct {
	// Path is the source file URI. When empty, locations omit artifactLocation.
	Path string
}

// Render formats an audit result into SARIF 2.1.0 JSON.
// Unsupported statements are not included in SARIF output.
func Render(result report.Result, options Options) ([]byte, error) {
	ruleMeta := make(map[string]*sarifRule, 8)
	var results []sarifResult

	collectFinding := func(finding rule.Finding) {
		if _, exists := ruleMeta[finding.RuleID]; !exists {
			ruleMeta[finding.RuleID] = buildRuleMeta(finding)
		}
		results = append(results, toSARIFResult(finding, options))
	}

	for _, stmt := range result.Statements {
		for _, finding := range stmt.Findings {
			collectFinding(finding)
		}
	}
	for _, finding := range result.GlobalFindings {
		collectFinding(finding)
	}

	rules := make([]sarifRule, 0, len(ruleMeta))
	for _, r := range ruleMeta {
		rules = append(rules, *r)
	}
	sort.Slice(rules, func(i, j int) bool { return rules[i].ID < rules[j].ID })

	if results == nil {
		results = []sarifResult{}
	}
	if rules == nil {
		rules = []sarifRule{}
	}

	doc := sarifDocument{
		Schema:  sarifSchema,
		Version: "2.1.0",
		Runs: []sarifRun{{
			Tool: sarifTool{
				Driver: sarifDriver{
					Name:           "DeltaScope",
					Version:        "",
					InformationURI: "https://github.com/Fanduzi/DeltaScope",
					Rules:          rules,
				},
			},
			Results: results,
		}},
	}

	return json.Marshal(doc)
}

func buildRuleMeta(finding rule.Finding) *sarifRule {
	sr := &sarifRule{ID: finding.RuleID}
	if finding.Explanation == nil {
		return sr
	}
	if finding.Explanation.Suggestion != "" {
		sr.Help = &sarifMessage{Text: finding.Explanation.Suggestion}
	}
	return sr
}

func toSARIFResult(finding rule.Finding, options Options) sarifResult {
	sr := sarifResult{
		RuleID:  finding.RuleID,
		Level:   mapLevel(finding.Level),
		Message: sarifMessage{Text: finding.Message},
	}

	if finding.Explanation != nil {
		var parts []string
		if finding.Explanation.Why != "" {
			parts = append(parts, "Why: "+finding.Explanation.Why)
		}
		if finding.Explanation.Risk != "" {
			parts = append(parts, "Risk: "+finding.Explanation.Risk)
		}
		if finding.Explanation.Suggestion != "" {
			parts = append(parts, "Suggestion: "+finding.Explanation.Suggestion)
		}
		if len(parts) > 0 {
			sr.Message = sarifMessage{Text: finding.Message + "\n" + joinParts(parts)}
		}
	}

	if finding.Location != nil {
		loc := sarifLocation{
			PhysicalLocation: sarifPhysicalLocation{
				Region: sarifRegion{
					StartLine:   finding.Location.Line,
					StartColumn: finding.Location.Column,
				},
			},
		}
		if options.Path != "" {
			loc.PhysicalLocation.ArtifactLocation = &sarifArtifactLocation{
				URI: options.Path,
			}
		}
		sr.Locations = []sarifLocation{loc}
	}

	return sr
}

func joinParts(parts []string) string {
	return strings.Join(parts, "\n")
}

func mapLevel(level rule.Level) string {
	switch level {
	case rule.LevelBlocker:
		return "error"
	case rule.LevelWarning:
		return "warning"
	case rule.LevelNotice:
		return "note"
	default:
		return "none"
	}
}

type sarifDocument struct {
	Schema  string     `json:"$schema"`
	Version string     `json:"version"`
	Runs    []sarifRun `json:"runs"`
}

type sarifRun struct {
	Tool    sarifTool     `json:"tool"`
	Results []sarifResult `json:"results"`
}

type sarifTool struct {
	Driver sarifDriver `json:"driver"`
}

type sarifDriver struct {
	Name           string      `json:"name"`
	Version        string      `json:"version,omitempty"`
	InformationURI string      `json:"informationUri"`
	Rules          []sarifRule `json:"rules"`
}

type sarifRule struct {
	ID   string        `json:"id"`
	Help *sarifMessage `json:"help,omitempty"`
}

type sarifResult struct {
	RuleID    string          `json:"ruleId"`
	Level     string          `json:"level"`
	Message   sarifMessage    `json:"message"`
	Locations []sarifLocation `json:"locations,omitempty"`
}

type sarifMessage struct {
	Text string `json:"text"`
}

type sarifLocation struct {
	PhysicalLocation sarifPhysicalLocation `json:"physicalLocation"`
}

type sarifPhysicalLocation struct {
	ArtifactLocation *sarifArtifactLocation `json:"artifactLocation,omitempty"`
	Region           sarifRegion            `json:"region"`
}

type sarifArtifactLocation struct {
	URI string `json:"uri"`
}

type sarifRegion struct {
	StartLine   int `json:"startLine,omitempty"`
	StartColumn int `json:"startColumn,omitempty"`
}
