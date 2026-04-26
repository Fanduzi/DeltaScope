// Package gitlabcodequality renders audit results as GitLab Code Quality JSON.
// input: internal report results from the audit application flow
// output: GitLab Code Quality JSON array for CI pipeline integration
// pos: infrastructure output adapter for the GitLab Code Quality CI-native renderer
// note: if this file changes, update this header and module README.md.
package gitlabcodequality

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"

	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
)

// Options carries renderer configuration.
type Options struct {
	// Path is the source file path. When empty, the renderer uses "deltascope.sql".
	Path string
}

// Render formats an audit result into a GitLab Code Quality JSON array.
// Unsupported statements are not included.
func Render(result report.Result, options Options) ([]byte, error) {
	path := resolvePath(options.Path)
	var issues []issue

	for _, stmt := range result.Statements {
		for _, finding := range stmt.Findings {
			issues = append(issues, buildIssue(finding, path))
		}
	}
	for _, finding := range result.GlobalFindings {
		issues = append(issues, buildIssue(finding, path))
	}

	if issues == nil {
		issues = []issue{}
	}

	return json.Marshal(issues)
}

type issue struct {
	Description string   `json:"description"`
	CheckName   string   `json:"check_name"`
	Fingerprint string   `json:"fingerprint"`
	Severity    string   `json:"severity"`
	Location    location `json:"location"`
}

type location struct {
	Path  string `json:"path"`
	Lines lines  `json:"lines"`
}

type lines struct {
	Begin int `json:"begin"`
}

func buildIssue(finding rule.Finding, path string) issue {
	return issue{
		Description: buildDescription(finding),
		CheckName:   finding.RuleID,
		Fingerprint: computeFingerprint(finding.RuleID, path, resolveLine(finding), finding.StatementIndex, finding.Message),
		Severity:    mapSeverity(finding.Level),
		Location: location{
			Path:  path,
			Lines: lines{Begin: resolveLine(finding)},
		},
	}
}

func buildDescription(finding rule.Finding) string {
	msg := finding.Message
	if finding.Explanation != nil && finding.Explanation.Suggestion != "" {
		msg += " Suggestion: " + finding.Explanation.Suggestion
	}
	return msg
}

func resolvePath(filePath string) string {
	if filePath == "" {
		return "deltascope.sql"
	}
	return strings.TrimPrefix(filePath, "./")
}

func resolveLine(finding rule.Finding) int {
	if finding.Location != nil && finding.Location.Line > 0 {
		return finding.Location.Line
	}
	if finding.StatementIndex > 0 {
		return finding.StatementIndex + 1
	}
	return 1
}

func mapSeverity(level rule.Level) string {
	switch level {
	case rule.LevelBlocker:
		return "major"
	case rule.LevelWarning:
		return "minor"
	case rule.LevelNotice:
		return "info"
	default:
		return "minor"
	}
}

func computeFingerprint(ruleID, path string, line, statementIndex int, message string) string {
	h := sha256.New()
	h.Write([]byte(ruleID))
	h.Write([]byte{0})
	h.Write([]byte(path))
	h.Write([]byte{0})
	h.Write([]byte{byte(line >> 24), byte(line >> 16), byte(line >> 8), byte(line)})
	h.Write([]byte{0})
	h.Write([]byte{byte(statementIndex >> 24), byte(statementIndex >> 16), byte(statementIndex >> 8), byte(statementIndex)})
	h.Write([]byte{0})
	h.Write([]byte(message))
	return hex.EncodeToString(h.Sum(nil))
}
