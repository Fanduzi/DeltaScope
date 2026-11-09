package cli

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// T24-R6: --deployment-evidence-file wires a bounded local evidence
// document through the shared spec.DeploymentEvidence binding — the CLI
// only reads and forwards; adjudication stays downstream.
func writeEvidenceFile(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "evidence.json")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write evidence file: %v", err)
	}
	return path
}

func runEvidenceAudit(t *testing.T, args ...string) (map[string]any, int) {
	t.Helper()
	stdout := &strings.Builder{}
	stderr := &strings.Builder{}
	code := Execute(context.Background(), args, strings.NewReader(""), stdout, stderr)
	var decoded map[string]any
	if len(stdout.String()) > 0 {
		_ = json.Unmarshal([]byte(stdout.String()), &decoded)
	}
	return decoded, code
}

func evidenceGapCodes(decoded map[string]any) []string {
	var codes []string
	stmts, _ := decoded["statements"].([]any)
	for _, s := range stmts {
		first, _ := s.(map[string]any)
		gaps, _ := first["evidence_gaps"].([]any)
		for _, g := range gaps {
			entry, _ := g.(map[string]any)
			if code, ok := entry["reason_code"].(string); ok {
				codes = append(codes, code)
			}
		}
	}
	return codes
}

// A bound, sourced bundle fills the host-VCPU premise the offline
// channel cannot observe — the VCPU endpoint gap disappears.
func TestT24CLIDeploymentEvidenceFileFillsPremise(t *testing.T) {
	path := writeEvidenceFile(t, `{"source":"capacity-report","product":"mysql","version":"8.0.36","host_vcpus":8}`)
	decoded, code := runEvidenceAudit(t,
		"audit", "--sql", "CREATE RESOURCE GROUP rg TYPE=USER VCPU=7;",
		"--dialect", "mysql", "--target-version", "8.0.36",
		"--format", "json", "--fail-on", "none",
		"--deployment-evidence-file", path)
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	for _, gap := range evidenceGapCodes(decoded) {
		if gap == "unknown_host_vcpu_count" {
			t.Fatalf("gaps = %v, want the evidence-bound host fact applied", evidenceGapCodes(decoded))
		}
	}
}

// A version-mismatched or foreign-target bundle is ignored — the gaps
// stay bounded; an unreadable or invalid file is an input error, never
// a silent fact change; raw evidence contents never reach the output.
func TestT24CLIDeploymentEvidenceFileGuards(t *testing.T) {
	sql := []string{
		"audit", "--sql", "CREATE RESOURCE GROUP rg TYPE=USER VCPU=7;",
		"--dialect", "mysql", "--target-version", "8.0.36",
		"--format", "json", "--fail-on", "none",
	}

	// Version mismatch — bundle ignored, host gap stays.
	mismatched := writeEvidenceFile(t, `{"source":"capacity-report","product":"mysql","version":"8.0.35","host_vcpus":8}`)
	decoded, _ := runEvidenceAudit(t, append(append([]string{}, sql...), "--deployment-evidence-file", mismatched)...)
	found := false
	for _, gap := range evidenceGapCodes(decoded) {
		if gap == "unknown_host_vcpu_count" {
			found = true
		}
	}
	if !found {
		t.Fatal("version-mismatched bundle must not fill the host premise")
	}

	// Foreign target — same-version bundle still ignored.
	foreign := writeEvidenceFile(t, `{"source":"capacity-report","product":"mysql","version":"8.0.36","target":"host-a","host_vcpus":8}`)
	decoded, _ = runEvidenceAudit(t, append(append([]string{}, sql...), "--deployment-evidence-file", foreign, "--target-identity", "host-b")...)
	found = false
	for _, gap := range evidenceGapCodes(decoded) {
		if gap == "unknown_host_vcpu_count" {
			found = true
		}
	}
	if !found {
		t.Fatal("a foreign-target bundle must not fill the host premise")
	}

	// Matching declared target binds the bundle.
	decoded, _ = runEvidenceAudit(t, append(append([]string{}, sql...), "--deployment-evidence-file", foreign, "--target-identity", "host-a")...)
	for _, gap := range evidenceGapCodes(decoded) {
		if gap == "unknown_host_vcpu_count" {
			t.Fatal("a matching declared target must admit the bundle")
		}
	}

	// Malformed, oversized, missing-anchor, and nonexistent files are
	// input errors (exit 2), never silently adjusted.
	bad := writeEvidenceFile(t, `{"source":"capacity-report","product":"mysql","version":`)
	if _, code := runEvidenceAudit(t, append(append([]string{}, sql...), "--deployment-evidence-file", bad)...); code != 2 {
		t.Fatalf("malformed JSON: exit = %d, want 2", code)
	}
	noAnchor := writeEvidenceFile(t, `{"product":"mysql","version":"8.0.36"}`)
	if _, code := runEvidenceAudit(t, append(append([]string{}, sql...), "--deployment-evidence-file", noAnchor)...); code != 2 {
		t.Fatalf("missing anchors: exit = %d, want 2", code)
	}
	unknown := writeEvidenceFile(t, `{"source":"s","product":"mysql","version":"8.0.36","secret_dsn":"s3cret"}`)
	if _, code := runEvidenceAudit(t, append(append([]string{}, sql...), "--deployment-evidence-file", unknown)...); code != 2 {
		t.Fatalf("unknown field: exit = %d, want 2 — schema stays bounded", code)
	}
	if _, code := runEvidenceAudit(t, append(append([]string{}, sql...), "--deployment-evidence-file", filepath.Join(t.TempDir(), "absent.json"))...); code != 2 {
		t.Fatalf("missing file: exit = %d, want 2", code)
	}

	// Output privacy: the evidence values never surface in stdout.
	stdout := &strings.Builder{}
	code := Execute(context.Background(), append(append([]string{}, sql...),
		"--deployment-evidence-file", foreign, "--target-identity", "host-a"),
		strings.NewReader(""), stdout, &strings.Builder{})
	if code != 0 {
		t.Fatalf("exit = %d", code)
	}
	if strings.Contains(stdout.String(), "host-a") || strings.Contains(stdout.String(), "capacity-report") {
		t.Fatal("evidence contents must never leak into CLI output")
	}
}
